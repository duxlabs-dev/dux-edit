package highlight

import (
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Lexer finds a lexer by name or alias, nil when there is none.
func Lexer(name string) chroma.Lexer {
	if name == "" {
		return nil
	}
	return lexers.Get(name)
}

// Match finds a lexer for a file name, nil when there is none.
func Match(filename string) chroma.Lexer { return lexers.Match(filename) }

// lexerID is what State reports and SetLexer accepts back.
func lexerID(l chroma.Lexer) string { return strings.ToLower(l.Config().Name) }

func lexerInfos() []LexerInfo {
	reg := lexers.GlobalLexerRegistry
	out := make([]LexerInfo, 0, len(reg.Lexers))
	for _, l := range reg.Lexers {
		cfg := l.Config()
		aliases := cfg.Aliases
		if aliases == nil {
			aliases = []string{}
		}
		out = append(out, LexerInfo{Name: cfg.Name, Aliases: aliases})
	}
	slices.SortFunc(out, func(a, b LexerInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}
