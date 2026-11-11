package highlight

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

const (
	SourceOverride = "override"
	SourceFilename = "filename"
	SourceShebang  = "shebang"
	SourceMIME     = "mime"
	SourceContent  = "content"
	SourceNone     = "none"
)

// headBytes is how much of a document detection reads.
const headBytes = 1024

// minAnalyseScore keeps weak hints such as "package " in a note from turning prose into Go.
const minAnalyseScore = 0.1

var interpreters = map[string]string{
	"node": "javascript", "nodejs": "javascript", "deno": "javascript", "bun": "javascript",
	"ts-node": "typescript", "tsx": "typescript",
	"sh": "bash", "dash": "bash", "ash": "bash", "ksh": "bash", "mksh": "bash", "busybox": "bash",
	"pwsh": "powershell", "tclsh": "tcl", "wish": "tcl", "rscript": "r",
	"runghc": "haskell", "runhaskell": "haskell", "escript": "erlang", "guile": "scheme",
	"sbcl": "common lisp", "osascript": "applescript", "gawk": "awk", "mawk": "awk", "nawk": "awk",
}

// detect picks a lexer for a document without an override.
func detect(filePath, head string) (chroma.Lexer, string) {
	if filePath != "" {
		if l := Match(filepath.Base(filePath)); l != nil {
			return l, SourceFilename
		}
	}
	if l := shebangLexer(head); l != nil {
		return l, SourceShebang
	}
	if mime := sniffMIME(head); mime != "" {
		if l := lexers.MatchMimeType(mime); l != nil {
			return l, SourceMIME
		}
	}
	if l := analyse(head); l != nil {
		return l, SourceContent
	}
	return lexers.Get("plaintext"), SourceNone
}

func shebangLexer(head string) chroma.Lexer {
	line, _, _ := strings.Cut(head, "\n")
	rest, ok := strings.CutPrefix(line, "#!")
	if !ok {
		return nil
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil
	}
	interp := path.Base(fields[0])
	if interp == "env" {
		interp = envCommand(fields[1:])
	}
	name := strings.ToLower(strings.TrimRight(interp, "0123456789."))
	if name == "" {
		return nil
	}
	if mapped, ok := interpreters[name]; ok {
		name = mapped
	}
	return Lexer(name)
}

// envCommand finds the program in `env [-S] [-i] [NAME=value...] program`.
func envCommand(args []string) string {
	for _, arg := range args {
		switch {
		case arg == "-S" || arg == "--split-string" || arg == "--":
		case strings.HasPrefix(arg, "-S"):
			return path.Base(arg[2:])
		case strings.HasPrefix(arg, "-"), strings.Contains(arg, "="):
		default:
			return path.Base(arg)
		}
	}
	return ""
}

// sniffMIME recognizes the few formats whose first bytes say what they are.
func sniffMIME(head string) string {
	t := strings.TrimLeft(head, " \t\r\n\ufeff")
	lower := strings.ToLower(t[:min(len(t), 16)])
	switch {
	case strings.HasPrefix(lower, "<?xml"):
		return "application/xml"
	case strings.HasPrefix(lower, "<!doctype html"), strings.HasPrefix(lower, "<html"):
		return "text/html"
	case strings.HasPrefix(lower, "<svg"):
		return "image/svg+xml"
	case looksLikeJSON(t):
		return "application/json"
	}
	return ""
}

func looksLikeJSON(t string) bool {
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return false
	}
	next := strings.TrimLeft(t[1:], " \t\r\n")
	if next == "" {
		return false
	}
	if t[0] == '{' {
		return next[0] == '"' || next[0] == '}'
	}
	return next[0] == '{' || next[0] == '"' || next[0] == ']'
}

func analyse(head string) chroma.Lexer {
	var best chroma.Lexer
	score := float32(minAnalyseScore)
	for _, l := range lexers.GlobalLexerRegistry.Lexers {
		a, ok := l.(chroma.Analyser)
		if !ok {
			continue
		}
		if w := a.AnalyseText(head); w > score {
			best, score = l, w
		}
	}
	return best
}
