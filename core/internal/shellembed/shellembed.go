package shellembed

import "github.com/duxlabs-dev/dux-go/shellapp/shellfs"

// UI is the QML tree a withshell build carries.
var UI = shellfs.Embedded{FS: distFS, Root: "dist", Entry: "shell.qml"}
