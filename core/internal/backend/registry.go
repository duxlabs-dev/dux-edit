package backend

import "slices"

type ParamSpec struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	// List params take a list of strings; the CLI wraps a single value.
	List bool   `json:"list,omitempty"`
	Desc string `json:"desc,omitempty"`
}

type MethodSpec struct {
	Name   string      `json:"name"`
	Group  string      `json:"group"`
	Desc   string      `json:"desc,omitempty"`
	Params []ParamSpec `json:"params,omitempty"`
}

func req(name, desc string) ParamSpec { return ParamSpec{Name: name, Required: true, Desc: desc} }
func opt(name, desc string) ParamSpec { return ParamSpec{Name: name, Desc: desc} }
func optList(name, desc string) ParamSpec {
	return ParamSpec{Name: name, List: true, Desc: desc}
}

var docVersion = []ParamSpec{req("docId", "document id"), req("version", "mirror version the request was made at")}

func queryParams(extra ...ParamSpec) []ParamSpec {
	base := append([]ParamSpec{}, docVersion...)
	base = append(base, req("query", "text or pattern"), opt("regex", "query is an RE2 pattern"), opt("caseSensitive", "match case"), opt("wholeWord", "match whole words"))
	return append(base, extra...)
}

// Methods drives the `describe` method and shell completion.
var Methods = []MethodSpec{
	{Name: "ping", Group: "core", Desc: "Liveness check"},
	{Name: "version", Group: "core", Desc: "Backend and API version"},
	{Name: "describe", Group: "core", Desc: "List all IPC methods and their params"},
	{Name: "subscribe", Group: "core", Desc: "Subscribe to event topics", Params: []ParamSpec{optList("topics", "ui,docs,highlight,spell,colorScheme; default ui")}},
	{Name: "unsubscribe", Group: "core", Desc: "Unsubscribe from event topics", Params: []ParamSpec{optList("topics", "topics to drop")}},

	{Name: "ui.show", Group: "ui", Desc: "Show the window, creating it if needed"},
	{Name: "ui.hide", Group: "ui", Desc: "Close the window and stay resident"},
	{Name: "ui.toggle", Group: "ui", Desc: "Show or hide the window"},
	{Name: "ui.open", Group: "ui", Desc: "Open files as tabs", Params: []ParamSpec{optList("paths", "absolute paths; empty opens a new draft"), opt("positions", "one {line, column} object per path; nonnegative integers, 1-based or 0 for unset; INVALID on malformed input"), opt("line", "1-based fallback line for the first path"), opt("column", "1-based fallback column for the first path"), opt("simple", "show the window in Simple mode")}},
	{Name: "ui.quit", Group: "ui", Desc: "Quit the app"},

	{Name: "doc.open", Group: "doc", Desc: "Open a file, a swap draft, or a new draft when both are empty; a file's result carries indentStyle and indentWidth from .editorconfig or a modeline when known", Params: []ParamSpec{opt("path", "absolute path"), opt("swapId", "draft or recovery swap to load"), opt("encoding", "force an encoding instead of detecting"), opt("elevated", "read through polkit"), opt("force", "skip the large file confirmation")}},
	{Name: "doc.edit", Group: "doc", Desc: "Apply one text change to the mirror", Params: append(append([]ParamSpec{}, docVersion...), req("start", "UTF-16 offset"), req("removed", "UTF-16 units removed"), req("text", "inserted text"), req("length", "UTF-16 length of the document afterwards"), opt("undoSteps", "Qt undo stack position after the edit; restores mixed line endings on undo"), opt("redo", "this delta is a redo"))},
	{Name: "doc.reset", Group: "doc", Desc: "Replace the mirror after a resync event", Params: append(append([]ParamSpec{}, docVersion...), req("text", "full text"), opt("conflict", "the file changed on disk under this text; the next save asks"))},
	{Name: "doc.save", Group: "doc", Desc: "Save the mirror to its file", Params: append(append([]ParamSpec{}, docVersion...), opt("md5", "md5 of the UI text as UTF-8, checked against the mirror"), opt("path", "save as this path"), opt("encoding", "convert to this encoding"), opt("bom", "write a byte order mark"), opt("eol", "lf|crlf|cr"), opt("elevated", "write through polkit"), opt("force", "overwrite a file that changed on disk"), opt("overwrite", "replace an existing file on save as"))},
	{Name: "doc.reload", Group: "doc", Desc: "Re-read the file from disk, dropping unsaved edits; answers like doc.open", Params: []ParamSpec{req("docId", "document id"), opt("encoding", "decode with this encoding")}},
	{Name: "doc.close", Group: "doc", Desc: "Close a document; a kept one answers DESYNC or EIO and stays open when its swap would not hold the UI's text", Params: []ParamSpec{req("docId", "document id"), opt("discard", "delete its swap; otherwise unsaved edits stay recoverable"), opt("version", "mirror version to wait for before keeping"), opt("md5", "md5 of the UI text as UTF-8, checked against the mirror")}},
	{Name: "doc.setTitle", Group: "doc", Desc: "Name a draft", Params: []ParamSpec{req("docId", "document id"), req("title", "tab title")}},
	{Name: "doc.text", Group: "doc", Desc: "Current mirror text", Params: []ParamSpec{req("docId", "document id")}},
	{Name: "doc.matchBracket", Group: "doc", Desc: "Match the bracket beside the cursor, ignoring strings and comments; scans at most 20000 UTF-16 units in the line window", Params: append(append([]ParamSpec{}, docVersion...), req("offset", "cursor UTF-16 offset; bracket before it takes priority"), req("first", "first line, 0-based"), req("last", "last line, inclusive"))},
	{Name: "doc.flush", Group: "doc", Desc: "Write pending swaps now; EIO lists the docIds that failed"},
	{Name: "recovery.list", Group: "doc", Desc: "Swaps no open document or live instance owns"},
	{Name: "recovery.discard", Group: "doc", Desc: "Delete a swap", Params: []ParamSpec{req("swapId", "swap id")}},

	{Name: "highlight.view", Group: "highlight", Desc: "Set the visible line range; spans follow on the highlight topic", Params: []ParamSpec{req("docId", "document id"), req("first", "first line, 0-based"), req("last", "last line, inclusive")}},
	{Name: "highlight.setLexer", Group: "highlight", Desc: "Override the detected language", Params: []ParamSpec{req("docId", "document id"), opt("lexer", "lexer name; empty for automatic"), opt("force", "highlight past the size caps")}},
	{Name: "highlight.state", Group: "highlight", Desc: "Lexer and tier of a document", Params: []ParamSpec{req("docId", "document id")}},
	{Name: "highlight.lexers", Group: "highlight", Desc: "Every lexer, for the language picker"},

	{Name: "search.find", Group: "search", Desc: "Find matches", Params: queryParams(opt("limit", "most ranges to return (default 10000)"), opt("seq", "UI find counter; an older find answers SUPERSEDED"))},
	{Name: "search.replaceAll", Group: "search", Desc: "One edit that replaces every match", Params: queryParams(req("replacement", "replacement, $1 expands in regex mode"))},
	{Name: "search.expand", Group: "search", Desc: "Replacement text for one match", Params: queryParams(req("replacement", "replacement"), req("start", "match offset"), req("length", "match length"))},

	{Name: "spell.configure", Group: "spell", Desc: "Turn spellcheck on or off and pick the language", Params: []ParamSpec{req("enabled", "check spelling"), opt("language", "dictionary code or auto"), opt("strings", "also check spaced string literals")}},
	{Name: "spell.languages", Group: "spell", Desc: "Installed hunspell dictionaries"},
	{Name: "spell.view", Group: "spell", Desc: "Set the visible line range; ranges follow on the spell topic", Params: []ParamSpec{req("docId", "document id"), req("first", "first line"), req("last", "last line, inclusive"), opt("cursorLine", "line of the word being typed"), opt("cursorColumn", "its UTF-16 column")}},
	{Name: "spell.suggest", Group: "spell", Desc: "Suggestions for a word", Params: []ParamSpec{req("word", "misspelled word"), opt("language", "dictionary code")}},
	{Name: "spell.add", Group: "spell", Desc: "Add a word to the user dictionary", Params: []ParamSpec{req("word", "word")}},
	{Name: "spell.ignore", Group: "spell", Desc: "Ignore a word until the app exits", Params: []ParamSpec{req("word", "word")}},

	{Name: "markdown.render", Group: "markdown", Desc: "Render a Markdown document to Qt rich text HTML", Params: append(append([]ParamSpec{}, docVersion...), opt("palette", "role name to #rrggbb"), opt("codeBackground", "#rrggbb"), opt("linkColor", "#rrggbb"))},

	{Name: "plugins.list", Group: "plugins", Desc: "Scan the plugin directories"},

	{Name: "dialog.openFiles", Group: "system", Desc: "Portal open dialog", Params: []ParamSpec{opt("title", "dialog title"), opt("dir", "starting directory"), opt("multiple", "allow several files (default true)")}},
	{Name: "dialog.saveFile", Group: "system", Desc: "Portal save dialog", Params: []ParamSpec{opt("title", "dialog title"), opt("dir", "starting directory"), opt("name", "suggested file name")}},
	{Name: "system.colorScheme.get", Group: "system", Desc: "Desktop light/dark preference"},
	{Name: "system.elevation", Group: "system", Desc: "Whether saving as administrator can be offered"},
	{Name: "import.dms", Group: "system", Desc: "Copy DMS notepad tabs and settings", Params: []ParamSpec{opt("dryRun", "report without copying"), opt("apply", "also send the result to the UI as a ui import action")}},
}

func MethodNames() []string {
	names := make([]string, len(Methods))
	for i, m := range Methods {
		names[i] = m.Name
	}
	slices.Sort(names)
	return names
}

func FindMethod(name string) (MethodSpec, bool) {
	i := slices.IndexFunc(Methods, func(m MethodSpec) bool { return m.Name == name })
	if i < 0 {
		return MethodSpec{}, false
	}
	return Methods[i], true
}
