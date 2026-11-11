package shellembed

import (
	"io/fs"
	"path"

	"github.com/duxlabs-dev/dux-go/shellapp/shellfs"
)

const (
	distRoot   = "dist"
	shellEntry = "shell.qml"
)

func Available() bool {
	info, err := fs.Stat(distFS, path.Join(distRoot, shellEntry))
	return err == nil && !info.IsDir()
}

func Extract(baseDir string) (string, error) {
	sub, err := fs.Sub(distFS, distRoot)
	if err != nil {
		return "", err
	}
	return shellfs.Extract(sub, baseDir)
}

func Prune(baseDir, keep string) {
	shellfs.Prune(baseDir, keep)
}
