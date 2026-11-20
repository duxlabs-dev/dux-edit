package helper

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-go/atomicfile"
	dhelper "github.com/duxlabs-dev/dux-go/elevate/helper"
	"github.com/duxlabs-dev/dux-go/files"
)

var errConflict = &files.Error{Code: files.CodeConflict, Err: files.ErrConflict}

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

func replace(path, identity string, in io.Reader, ready io.Writer) (string, error) {
	if identity != "" && !validIdentity(identity) {
		return "", dhelper.Invalid("not an identity: " + identity)
	}
	parent, name, err := dhelper.OpenParent(path)
	if err != nil {
		return "", err
	}
	defer unix.Close(parent)

	handle, st, err := dhelper.OpenRegular(parent, name, path)
	switch {
	case errors.Is(err, unix.ENOENT) && identity != "":
		return "", errConflict
	case err != nil:
		return "", err
	}
	defer unix.Close(handle)

	if identity != "" && atomicfile.StatIdentity(&st) != identity {
		return "", errConflict
	}
	if err := dhelper.Announce(ready); err != nil {
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
	fd, err := dhelper.OpenNoSymlinks(parent, tmpName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	file := os.NewFile(uintptr(fd), filepath.Join(filepath.Dir(path), tmpName))
	err = dhelper.CopyFrames(file, in)
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
	return identity, dhelper.SyncDir(parent, path)
}

func restoreAttributes(fd, original int, st *unix.Stat_t) error {
	if err := unix.Fchown(fd, int(st.Uid), int(st.Gid)); err != nil {
		return &os.PathError{Op: "chown", Path: dhelper.ProcPath(fd), Err: err}
	}
	// chown clears setuid and setgid, so the mode goes on after it.
	if err := unix.Fchmod(fd, st.Mode&0o7777); err != nil {
		return &os.PathError{Op: "chmod", Path: dhelper.ProcPath(fd), Err: err}
	}
	copyXattrs(original, fd)
	return nil
}

// Best effort: a filesystem without xattrs, or a policy that refuses the label, still gets the save.
func copyXattrs(from, to int) {
	src := dhelper.ProcPath(from)
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
	if err := dhelper.CopyFrames(&data, in); err != nil {
		return "", err
	}
	fd, err := unix.Open(dhelper.ProcPath(handle), unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOCTTY, 0)
	if err != nil {
		return "", &os.PathError{Op: "open", Path: path, Err: err}
	}
	file := os.NewFile(uintptr(fd), path)
	return syncClose(file, atomicfile.WriteInPlace(file, data.Bytes()))
}

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

func randomHex() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
