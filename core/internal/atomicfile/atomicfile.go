// Package atomicfile saves files without losing their mode, owner, labels, ACLs, symlinks or hard links.
package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type Options struct {
	// Mode for a file that does not exist yet; 0 means 0666 minus the umask.
	Mode os.FileMode
}

var copiedXattrs = []string{"security.selinux", "system.posix_acl_access"}

// Stat returns the "dev:ino:size:mtimeNs" identity of the file path resolves to.
func Stat(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return Identity(info), nil
}

// Identity is what Stat returns, for a FileInfo already at hand.
func Identity(info os.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Sprintf("0:0:%d:%d", info.Size(), info.ModTime().UnixNano())
	}
	return fmt.Sprintf("%d:%d:%d:%d", st.Dev, st.Ino, st.Size, st.Mtim.Nano())
}

// Writable reports whether the effective user may write path, or create it when it does not exist.
func Writable(path string) bool {
	err := unix.Faccessat(unix.AT_FDCWD, path, unix.W_OK, unix.AT_EACCESS)
	if errors.Is(err, unix.ENOENT) {
		err = unix.Faccessat(unix.AT_FDCWD, filepath.Dir(path), unix.W_OK|unix.X_OK, unix.AT_EACCESS)
	}
	return err == nil
}

// Resolve follows symlinks, including a dangling last one, so a save writes the target and keeps the link.
func Resolve(path string) (string, error) {
	for range 40 {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			dir, derr := filepath.EvalSymlinks(filepath.Dir(path))
			if derr != nil {
				return "", derr
			}
			return filepath.Join(dir, filepath.Base(path)), nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return filepath.EvalSymlinks(path)
		}
		link, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		path = link
	}
	return "", unix.ELOOP
}

// Save writes data to path and returns the new identity.
func Save(path string, data []byte, opts Options) (string, error) {
	target, err := Resolve(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	switch {
	case errors.Is(err, os.ErrNotExist):
		mode := opts.Mode
		if mode == 0 {
			mode = 0o666 &^ umask()
		}
		if err := replace(target, data, mode, nil); err != nil {
			return "", err
		}
	case err != nil:
		return "", err
	case info.IsDir():
		return "", unix.EISDIR
	case inPlace(target, info):
		if err := overwrite(target, data); err != nil {
			return "", err
		}
	default:
		if err := replace(target, data, info.Mode(), info); err != nil {
			return "", err
		}
	}
	return Stat(target)
}

// A rename would split a hard link, hand the file to us, or is impossible in a read-only directory.
func inPlace(target string, info os.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return true
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if ok && (st.Nlink > 1 || int(st.Uid) != os.Geteuid()) {
		return true
	}
	dirWritable := unix.Faccessat(unix.AT_FDCWD, filepath.Dir(target), unix.W_OK|unix.X_OK, unix.AT_EACCESS) == nil
	fileWritable := unix.Faccessat(unix.AT_FDCWD, target, unix.W_OK, unix.AT_EACCESS) == nil
	return !dirWritable && fileWritable
}

// The blocks are reserved and written before the old tail is cut, so a full disk fails with the file intact.
func overwrite(target string, data []byte) error {
	f, err := os.OpenFile(target, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	err = reserve(f, int64(len(data)))
	if err == nil {
		_, err = f.WriteAt(data, 0)
	}
	// A device or FIFO has no length to set.
	if info, statErr := f.Stat(); err == nil && statErr == nil && info.Mode().IsRegular() {
		err = f.Truncate(int64(len(data)))
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Filesystems without fallocate fall back to the plain write.
func reserve(f *os.File, size int64) error {
	if size == 0 {
		return nil
	}
	for {
		err := unix.Fallocate(int(f.Fd()), unix.FALLOC_FL_KEEP_SIZE, 0, size)
		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case errors.Is(err, unix.ENOSPC), errors.Is(err, unix.EDQUOT), errors.Is(err, unix.EFBIG):
			return &os.PathError{Op: "fallocate", Path: f.Name(), Err: err}
		}
		return nil
	}
}

func replace(target string, data []byte, mode os.FileMode, original os.FileInfo) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(target)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := fill(tmp, data, mode, target, original); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	syncDir(dir)
	return nil
}

func fill(tmp *os.File, data []byte, mode os.FileMode, target string, original os.FileInfo) error {
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if original != nil {
		if st, ok := original.Sys().(*syscall.Stat_t); ok {
			// Only root or a member of the group can do this; the mode below still applies.
			_ = tmp.Chown(int(st.Uid), int(st.Gid))
		}
	}
	if err := tmp.Chmod(mode & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)); err != nil {
		return err
	}
	// The ACL goes after chmod, which would otherwise rewrite its mask.
	if original != nil {
		copyXattrs(target, tmp)
	}
	return tmp.Sync()
}

// Best effort: a filesystem without labels or ACLs, or a label we may not set, must not fail the save.
func copyXattrs(target string, tmp *os.File) {
	for _, name := range copiedXattrs {
		size, err := unix.Lgetxattr(target, name, nil)
		if err != nil || size <= 0 {
			continue
		}
		buf := make([]byte, size)
		size, err = unix.Lgetxattr(target, name, buf)
		if err != nil {
			continue
		}
		_ = unix.Fsetxattr(int(tmp.Fd()), name, buf[:size], 0)
	}
}

// Some filesystems refuse fsync on a directory; the rename has happened either way.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	d.Close()
}

// Reading the umask through /proc avoids setting it, which would race with other goroutines creating files.
func umask() os.FileMode {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0o022
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		value, found := strings.CutPrefix(line, "Umask:")
		if !found {
			continue
		}
		mask, err := strconv.ParseUint(strings.TrimSpace(value), 8, 32)
		if err != nil {
			return 0o022
		}
		return os.FileMode(mask)
	}
	return 0o022
}
