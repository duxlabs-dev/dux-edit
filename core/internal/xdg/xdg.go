package xdg

import (
	"os"
	"strings"
)

const DefaultDataDirs = "/usr/local/share:/usr/share"

func DataDirs() []string {
	return SplitPathList(EnvOr(os.Getenv, "XDG_DATA_DIRS", DefaultDataDirs))
}

func EnvOr(env func(string) string, key, fallback string) string {
	if v := env(key); v != "" {
		return v
	}
	return fallback
}

func SplitPathList(raw string) []string {
	var out []string
	for _, dir := range strings.Split(raw, ":") {
		if dir = strings.TrimSpace(dir); dir != "" {
			out = append(out, dir)
		}
	}
	return out
}
