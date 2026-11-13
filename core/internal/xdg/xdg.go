package xdg

import (
	"cmp"
	"os"
	"strings"
)

func DataDirs() []string {
	var out []string
	for dir := range strings.SplitSeq(cmp.Or(os.Getenv("XDG_DATA_DIRS"), "/usr/local/share:/usr/share"), ":") {
		if dir = strings.TrimSpace(dir); dir != "" {
			out = append(out, dir)
		}
	}
	return out
}
