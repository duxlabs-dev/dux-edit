package backend

import (
	"context"
	"errors"
	"sync"

	"github.com/duxlabs-dev/dux-edit/core/internal/markdown"
	"github.com/duxlabs-dev/dux-edit/core/internal/search"
	"github.com/duxlabs-dev/dux-edit/core/internal/spell"
	"github.com/duxlabs-dev/dux-go/ipc/params"
)

func query(p map[string]any) search.Query {
	return search.Query{
		Text:          params.StringOpt(p, "query", ""),
		Regex:         params.BoolOpt(p, "regex", false),
		CaseSensitive: params.BoolOpt(p, "caseSensitive", false),
		WholeWord:     params.BoolOpt(p, "wholeWord", false),
		Limit:         params.IntOpt(p, "limit", search.DefaultLimit),
	}
}

func searchFailure(err error) any {
	switch {
	case errors.Is(err, search.ErrBadRegex):
		return fail("BADREGEX", err.Error())
	case errors.Is(err, search.ErrNoMatch):
		return fail("NOMATCH", "")
	}
	return fail("EIO", err.Error())
}

// latestFinds holds the newest search.find seq per document while its finds run. The UI numbers its finds
// because requests on one connection start in any order.
var latestFinds = findSeqs{latest: map[string]int{}}

type findSeqs struct {
	mu     sync.Mutex
	latest map[string]int
}

func (f *findSeqs) begin(docID string, seq int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.latest[docID] = max(f.latest[docID], seq)
}

func (f *findSeqs) superseded(docID string, seq int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.latest[docID] > seq
}

func (f *findSeqs) end(docID string, seq int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.latest[docID] == seq {
		delete(f.latest, docID)
	}
}

func (b *Backend) searchFind(ctx context.Context, p map[string]any) any {
	find := func(text string, _ int64) any {
		matches, err := search.Find(text, query(p))
		if err != nil {
			return searchFailure(err)
		}
		return matches
	}
	docID, seq := params.StringOpt(p, "docId", ""), params.IntOpt(p, "seq", 0)
	if seq <= 0 {
		return b.withText(ctx, p, find)
	}
	latestFinds.begin(docID, seq)
	defer latestFinds.end(docID, seq)
	return b.withTextUnless(ctx, p, func() bool { return latestFinds.superseded(docID, seq) }, find)
}

func (b *Backend) searchReplaceAll(ctx context.Context, p map[string]any) any {
	return b.withText(ctx, p, func(text string, _ int64) any {
		replacement, err := search.ReplaceAll(text, query(p), params.StringOpt(p, "replacement", ""))
		if err != nil {
			return searchFailure(err)
		}
		return replacement
	})
}

func (b *Backend) searchExpand(ctx context.Context, p map[string]any) any {
	return b.withText(ctx, p, func(text string, _ int64) any {
		expanded, err := search.Expand(text, query(p), params.StringOpt(p, "replacement", ""), params.IntOpt(p, "start", 0), params.IntOpt(p, "length", 0))
		if err != nil {
			return searchFailure(err)
		}
		return map[string]any{"ok": true, "text": expanded}
	})
}

func (b *Backend) spellConfigure(_ context.Context, p map[string]any) any {
	return b.spell.Configure(spell.Config{
		Enabled:  params.BoolOpt(p, "enabled", false),
		Language: params.StringOpt(p, "language", "auto"),
		Strings:  params.BoolOpt(p, "strings", false),
	})
}

func (b *Backend) spellLanguages(context.Context, map[string]any) any {
	languages := b.spell.Languages()
	if languages == nil {
		languages = []spell.Language{}
	}
	return map[string]any{"languages": languages}
}

func (b *Backend) spellView(_ context.Context, p map[string]any) any {
	b.spell.View(
		params.StringOpt(p, "docId", ""),
		params.IntOpt(p, "first", 0),
		params.IntOpt(p, "last", 0),
		params.IntOpt(p, "cursorLine", -1),
		params.IntOpt(p, "cursorColumn", -1),
	)
	return ok()
}

func (b *Backend) spellSuggest(_ context.Context, p map[string]any) any {
	suggestions := b.spell.Suggest(params.StringOpt(p, "word", ""), params.StringOpt(p, "language", ""))
	if suggestions == nil {
		suggestions = []string{}
	}
	return map[string]any{"suggestions": suggestions}
}

func (b *Backend) spellAdd(_ context.Context, p map[string]any) any {
	if err := b.spell.Add(params.StringOpt(p, "word", "")); err != nil {
		return fail("EIO", err.Error())
	}
	return ok()
}

func (b *Backend) spellIgnore(_ context.Context, p map[string]any) any {
	b.spell.Ignore(params.StringOpt(p, "word", ""))
	return ok()
}

func (b *Backend) markdownRender(ctx context.Context, p map[string]any) any {
	return b.withText(ctx, p, func(source string, version int64) any {
		if len(source) > markdown.MaxBytes {
			return fail("TOOLARGE", "")
		}
		html, err := markdown.Render(source, markdown.Options{
			Palette:        params.StringMapOpt(p, "palette"),
			CodeBackground: params.StringOpt(p, "codeBackground", ""),
			LinkColor:      params.StringOpt(p, "linkColor", ""),
		})
		if err != nil {
			return fail("EIO", err.Error())
		}
		return map[string]any{"ok": true, "html": html, "version": version}
	})
}
