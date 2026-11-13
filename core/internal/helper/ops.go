package helper

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-edit/core/internal/atomicfile"
)

var errSymlink = &Error{Code: CodeInvalid, Err: errors.New("path goes through a symlink")}

func validPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != "/" && !strings.ContainsRune(path, 0)
}

func validIdentity(identity string) bool {
	parts := strings.Split(identity, ":")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.ParseUint(part, 10, 64); err != nil {
			return false
		}
	}
	return true
}

func openParent(path string) (int, string, error) {
	if !validPath(path) {
		return -1, "", invalid("not an absolute, clean path: " + path)
	}
	fd, err := openNoSymlinks(unix.AT_FDCWD, filepath.Dir(path), unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return -1, "", err
	}
	return fd, filepath.Base(path), nil
}

func openNoSymlinks(dirfd int, path string, flags int, mode uint32) (int, error) {
	how := unix.OpenHow{Flags: uint64(flags | unix.O_CLOEXEC), Mode: uint64(mode), Resolve: unix.RESOLVE_NO_SYMLINKS}
	fd, err := unix.Openat2(dirfd, path, &how)
	switch {
	case errors.Is(err, unix.ELOOP):
		return -1, errSymlink
	case err != nil:
		return -1, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return fd, nil
}

func procPath(fd int) string { return "/proc/self/fd/" + strconv.Itoa(fd) }

// openRegular returns an O_PATH handle, so a FIFO or device is refused before anything opens it for I/O.
func openRegular(parent int, name, path string) (int, unix.Stat_t, error) {
	var st unix.Stat_t
	handle, err := openNoSymlinks(parent, name, unix.O_PATH, 0)
	if err != nil {
		return -1, st, err
	}
	if err := unix.Fstat(handle, &st); err != nil {
		unix.Close(handle)
		return -1, st, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFREG {
		unix.Close(handle)
		return -1, st, invalid("not a regular file: " + path)
	}
	return handle, st, nil
}

// Every operation returns the identity of the file it read or wrote, which the caller may be unable to stat.
func read(path string, out io.Writer) (string, error) {
	parent, name, err := openParent(path)
	if err != nil {
		return "", err
	}
	defer unix.Close(parent)

	handle, st, err := openRegular(parent, name, path)
	if err != nil {
		return "", err
	}
	defer unix.Close(handle)

	fd, err := unix.Open(procPath(handle), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOCTTY, 0)
	if err != nil {
		return "", &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return atomicfile.StatIdentity(&st), nil
}

func create(path string, in io.Reader, ready io.Writer) (string, error) {
	parent, name, err := openParent(path)
	if err != nil {
		return "", err
	}
	defer unix.Close(parent)

	fd, err := openNoSymlinks(parent, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(fd), path)
	err = announce(ready)
	if err == nil {
		err = copyFrames(file, in)
	}
	identity, err := syncClose(file, err)
	if err != nil {
		_ = unix.Unlinkat(parent, name, 0)
		return "", err
	}
	return identity, syncDir(parent, path)
}

// The caller waits for this before it streams, so the time spent in the password prompt is not bounded.
func announce(ready io.Writer) error {
	_, err := io.WriteString(ready, Ready+"\n")
	return err
}

func replace(path, identity string, in io.Reader, ready io.Writer) (string, error) {
	if identity != "" && !validIdentity(identity) {
		return "", invalid("not an identity: " + identity)
	}
	parent, name, err := openParent(path)
	if err != nil {
		return "", err
	}
	defer unix.Close(parent)

	handle, st, err := openRegular(parent, name, path)
	switch {
	case errors.Is(err, unix.ENOENT) && identity != "":
		return "", &Error{Code: CodeConflict, Err: ErrConflict}
	case err != nil:
		return "", err
	}
	defer unix.Close(handle)

	if identity != "" && atomicfile.StatIdentity(&st) != identity {
		return "", &Error{Code: CodeConflict, Err: ErrConflict}
	}
	if err := announce(ready); err != nil {
		return "", err
	}
	// A rename would split the hard links.
	if st.Nlink > 1 {
		return replaceInPlace(handle, path, in)
	}
	return replaceByRename(parent, name, path, handle, &st, in)
}

func replaceByRename(parent int, name, path string, original int, st *unix.Stat_t, in io.Reader) (string, error) {
	tmpName := ".dux-edit-" + randomHex() + ".tmp"
	fd, err := openNoSymlinks(parent, tmpName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(filepath.Dir(path), tmpName))
	err = copyFrames(file, in)
	if err == nil {
		err = restoreAttributes(fd, original, st)
	}
	identity, err := syncClose(file, err)
	if err != nil {
		_ = unix.Unlinkat(parent, tmpName, 0)
		return "", err
	}
	if err := unix.Renameat(parent, tmpName, parent, name); err != nil {
		_ = unix.Unlinkat(parent, tmpName, 0)
		return "", &os.LinkError{Op: "rename", Old: tmpName, New: path, Err: err}
	}
	return identity, syncDir(parent, path)
}

func restoreAttributes(fd, original int, st *unix.Stat_t) error {
	if err := unix.Fchown(fd, int(st.Uid), int(st.Gid)); err != nil {
		return &os.PathError{Op: "chown", Path: procPath(fd), Err: err}
	}
	// chown clears setuid and setgid, so the mode goes on after it.
	if err := unix.Fchmod(fd, st.Mode&0o7777); err != nil {
		return &os.PathError{Op: "chmod", Path: procPath(fd), Err: err}
	}
	copyXattrs(original, fd)
	return nil
}

// Best effort: a filesystem without xattrs, or a policy that refuses the label, still gets the save.
func copyXattrs(from, to int) {
	src := procPath(from)
	for _, name := range atomicfile.KeptXattrs() {
		size, err := unix.Getxattr(src, name, nil)
		if err != nil || size <= 0 {
			continue
		}
		value := make([]byte, size)
		n, err := unix.Getxattr(src, name, value)
		if err != nil {
			continue
		}
		_ = unix.Fsetxattr(to, name, value[:n], 0)
	}
}

// The whole stream is read before the file is touched and the old tail is cut last, so an abort leaves it intact.
func replaceInPlace(handle int, path string, in io.Reader) (string, error) {
	var data bytes.Buffer
	if err := copyFrames(&data, in); err != nil {
		return "", err
	}
	fd, err := unix.Open(procPath(handle), unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOCTTY, 0)
	if err != nil {
		return "", &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	return syncClose(file, atomicfile.WriteInPlace(file, data.Bytes()))
}

// syncClose always closes the file. It returns the identity of what was written, or the first error,
// including writeErr.
func syncClose(file *os.File, writeErr error) (string, error) {
	err := writeErr
	if err == nil {
		err = file.Sync()
	}
	var st unix.Stat_t
	if err == nil {
		err = unix.Fstat(int(file.Fd()), &st)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	return atomicfile.StatIdentity(&st), nil
}

func syncDir(parent int, path string) error {
	if err := unix.Fsync(parent); err != nil {
		return &os.PathError{Op: "fsync", Path: filepath.Dir(path), Err: err}
	}
	return nil
}

// Frames are a big-endian uint32 length and that many bytes; a zero length commits the write.
func copyFrames(dst io.Writer, src io.Reader) error {
	var header [4]byte
	for {
		if _, err := io.ReadFull(src, header[:]); err != nil {
			return errors.New("write aborted before it was committed")
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == 0 {
			return nil
		}
		if _, err := io.CopyN(dst, src, int64(length)); err != nil {
			return err
		}
	}
}

func randomHex() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
