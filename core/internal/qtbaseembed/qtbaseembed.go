package qtbaseembed

import "github.com/duxlabs-dev/dux-go/qtbase"

// Locator extracts the runtime a withqtbase build carries; "" otherwise.
var Locator = qtbase.Embedded(libFS, "lib", "dux-edit")
