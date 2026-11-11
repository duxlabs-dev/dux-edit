package paths

import (
	"os"
	"path/filepath"

	duxpaths "github.com/duxlabs-dev/dux-go/paths"
)

// The socket and pid files stay under the shellapp ID "dux-edit"; only config, state and cache live under dux/edit.
var app = duxpaths.New("dux/edit")

func ConfigDir() (string, error) { return app.ConfigDir() }

func StateDir() (string, error) { return app.StateDir() }

func CacheDir() (string, error) { return app.CacheDir() }

// Swap files hold document contents, so the directory is private.
func SwapDir() (string, error) {
	state, err := StateDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(state, "swap")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, os.Chmod(dir, 0o700)
}

func CustomDictionary() (string, error) {
	config, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "custom_dictionary.txt"), nil
}

// UIEnv hands the resolved directories to the QML side so it never recomputes them.
func UIEnv(prefix string) []string {
	var env []string
	for name, resolve := range map[string]func() (string, error){
		"_CONFIG_DIR": ConfigDir,
		"_STATE_DIR":  StateDir,
		"_CACHE_DIR":  CacheDir,
	} {
		dir, err := resolve()
		if err != nil {
			continue
		}
		env = append(env, prefix+name+"="+dir)
	}
	return env
}
