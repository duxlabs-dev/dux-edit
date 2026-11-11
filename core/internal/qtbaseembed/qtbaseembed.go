package qtbaseembed

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/duxlabs-dev/dux-go/qtbase"
	"github.com/duxlabs-dev/dux-go/shellapp/shellfs"
)

const (
	libRoot  = "lib"
	cacheDir = "dux-edit"
)

func Available() bool {
	info, err := fs.Stat(libFS, path.Join(libRoot, qtbase.Soname))
	return err == nil && !info.IsDir()
}

func Extract() (string, error) {
	if !Available() {
		return "", nil
	}
	sub, err := fs.Sub(libFS, libRoot)
	if err != nil {
		return "", err
	}
	userCache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	// the cache dir rather than the runtime tmpfs keeps the mapped code evictable
	baseDir := filepath.Join(userCache, cacheDir, "qtbase")
	dir, err := shellfs.Extract(sub, baseDir)
	if err != nil {
		return "", err
	}
	// dlopen maps the whole file, so a running older app keeps its copy after the unlink
	shellfs.Prune(baseDir, dir)
	return filepath.Join(dir, qtbase.Soname), nil
}
