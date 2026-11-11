//go:build withshell

package shellembed

import "embed"

// all: keeps the .dankrev revision key, which go:embed would otherwise skip as a dotfile.
//
//go:embed all:dist
var distFS embed.FS
