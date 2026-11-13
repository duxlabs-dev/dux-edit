// Package spell finds misspelled prose with short-lived hunspell runs and a per-language word cache.
package spell

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/document"
	"github.com/duxlabs-dev/dux-edit/core/internal/highlight"
)

const (
	TopicEvents = "spell"

	ReasonNoHunspell = "NOHUNSPELL"
	ReasonNoDict     = "NODICT"

	maxDocUnits = highlight.MaxBytes
	// A cache this large means many documents were scrolled through; starting over is cheaper than keeping it.
	maxCacheWords = 200_000
	runTimeout    = 10 * time.Second
)

type Config struct {
	Enabled bool
	// Language is a dictionary code such as en_US, or "auto" to follow the locale.
	Language string
	// Strings also checks spaced string literals in code.
	Strings bool
}

type Status struct {
	OK        bool   `json:"ok"`
	Available bool   `json:"available"`
	Language  string `json:"language"`
	// Reason is NOHUNSPELL or NODICT when Available is false.
	Reason string `json:"reason,omitempty"`
}

type Language struct {
	Code string `json:"code"`
	Path string `json:"path"`
}

type textSource interface {
	// read returns lines [first, last) of a document with its version and UTF-16 length.
	read(docID string, first, last int) (lines []string, version int64, size int, ok bool)
}

type proseSource interface {
	Lines(ctx context.Context, docID string, first, last int) ([]highlight.Line, int64, error)
}

type storeText struct{ store *document.Store }

func (s storeText) read(docID string, first, last int) (lines []string, version int64, size int, ok bool) {
	d, ok := s.store.Get(docID)
	if !ok {
		return nil, 0, 0, false
	}
	d.Read(func(b *buffer.Buffer, v int64) {
		lines, version, size = b.Lines(first, last), v, b.Len16()
	})
	return lines, version, size, true
}

type view struct {
	first, last           int
	cursorLine, cursorCol int

	sent        bool
	sentVersion int64
	sentFirst   int
	sentLines   [][]int
}

type Service struct {
	text       textSource
	prose      proseSource
	publish    document.Publisher
	dictionary string
	run        runner
	getenv     func(string) string
	home       string
	lookPath   func(string) (string, error)

	mu     sync.Mutex
	active bool
	cfg    Config
	bin    string
	lang   Language
	// gen changes whenever cached results stop being valid, so a batch that finishes late is dropped.
	gen      int
	cache    map[string]bool
	personal map[string]bool
	ignored  map[string]bool
	views    map[string]*view
	open     map[string]bool
	dirty    map[string]bool
	working  bool
	stop     context.CancelFunc
	closed   bool
	worker   sync.WaitGroup
}

// dictionary is the path of custom_dictionary.txt.
func New(store *document.Store, tokens *highlight.Service, publish document.Publisher, dictionary string) *Service {
	home, _ := os.UserHomeDir()
	s := newService(storeText{store}, nil, publish, dictionary, execRunner{}, os.Getenv, home, exec.LookPath)
	if tokens != nil {
		s.prose = tokens
	}
	return s
}

func newService(text textSource, prose proseSource, publish document.Publisher, dictionary string, run runner,
	getenv func(string) string, home string, lookPath func(string) (string, error)) *Service {
	return &Service{
		text: text, prose: prose, publish: publish, dictionary: dictionary, run: run,
		getenv: getenv, home: home, lookPath: lookPath,
		ignored: map[string]bool{}, views: map[string]*view{}, open: map[string]bool{}, dirty: map[string]bool{},
	}
}

func normalize(w string) string { return strings.ReplaceAll(w, "’", "'") }

func (s *Service) DocOpened(d *document.Doc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open[d.ID] = true
}

func (s *Service) DocChanged(d *document.Doc, _ buffer.Change) { s.recheck(d.ID) }

func (s *Service) DocReset(d *document.Doc) { s.recheck(d.ID) }

// DocSaved rechecks because a save as can change the lexer and with it the prose.
func (s *Service) DocSaved(d *document.Doc) { s.recheck(d.ID) }

func (s *Service) DocClosed(docID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.views, docID)
	delete(s.open, docID)
	delete(s.dirty, docID)
	if len(s.open) == 0 {
		s.resetLocked()
	}
}

func (s *Service) recheck(docID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.views[docID]; ok {
		s.kickLocked(docID)
	}
}

func (s *Service) Configure(cfg Config) Status {
	status, bin, lang := s.resolve(cfg.Language)
	s.mu.Lock()
	defer s.mu.Unlock()
	active := cfg.Enabled && status.Available
	if lang != s.lang || !active {
		s.resetLocked()
	}
	s.cfg, s.bin, s.lang, s.active = cfg, bin, lang, active
	for id := range s.views {
		s.kickLocked(id)
	}
	return status
}

func (s *Service) resolve(language string) (Status, string, Language) {
	status := Status{OK: true}
	bin, err := s.lookPath("hunspell")
	if err != nil {
		status.Reason = ReasonNoHunspell
		return status, "", Language{}
	}
	candidates := localeCandidates(s.getenv)
	if language != "" && language != "auto" {
		candidates = []string{normalizeLocale(language)}
	}
	lang, ok := pickLanguage(candidates, s.Languages())
	if !ok {
		status.Reason = ReasonNoDict
		return status, bin, Language{}
	}
	status.Available, status.Language = true, lang.Code
	return status, bin, lang
}

func (s *Service) Languages() []Language {
	return listDictionaries(dictionaryDirs(s.getenv, s.home))
}

// View sets the visible line range [first, last] of a document. cursorLine and cursorCol (UTF-16) mark the
// word being typed, which is left alone; pass -1 for none.
func (s *Service) View(docID string, first, last, cursorLine, cursorCol int) {
	if docID == "" {
		return
	}
	first = max(first, 0)
	last = max(last, first)
	s.mu.Lock()
	defer s.mu.Unlock()
	// A view sent just before doc.close can land after DocClosed; it must not bring the document back.
	if !s.open[docID] {
		return
	}
	v, ok := s.views[docID]
	if !ok {
		v = &view{}
		s.views[docID] = v
	}
	if ok && v.first == first && v.last == last && v.cursorLine == cursorLine && v.cursorCol == cursorCol {
		return
	}
	v.first, v.last, v.cursorLine, v.cursorCol = first, last, cursorLine, cursorCol
	s.kickLocked(docID)
}

func (s *Service) Suggest(word, language string) []string {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil
	}
	s.mu.Lock()
	bin, lang := s.bin, s.lang
	s.mu.Unlock()
	if language != "" && language != lang.Code {
		var ok bool
		if lang, ok = pickLanguage([]string{normalizeLocale(language)}, s.Languages()); !ok {
			return nil
		}
	}
	if lang.Path == "" {
		return nil
	}
	if bin == "" {
		var err error
		if bin, err = s.lookPath("hunspell"); err != nil {
			return nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	suggestions, err := s.run.Suggest(ctx, bin, lang.Path, word)
	if err != nil {
		return nil
	}
	return suggestions
}

var errBadWord = errors.New("not a single word")

func (s *Service) Add(word string) error {
	word = strings.TrimSpace(word)
	if word == "" || strings.ContainsAny(word, " \t\r\n") {
		return errBadWord
	}
	if err := appendWord(s.dictionary, word); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := normalize(word)
	if s.personal != nil {
		s.personal[key] = true
	}
	if s.cache != nil {
		s.cache[key] = true
	}
	for id := range s.views {
		s.kickLocked(id)
	}
	return nil
}

func (s *Service) Ignore(word string) {
	word = strings.TrimSpace(word)
	if word == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ignored[normalize(word)] = true
	for id := range s.views {
		s.kickLocked(id)
	}
}

func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	if s.stop != nil {
		s.stop()
	}
	s.mu.Unlock()
	s.worker.Wait()
}

// resetLocked drops every cached word and what was last published, and stops a running batch.
func (s *Service) resetLocked() {
	s.gen++
	s.cache, s.personal = nil, nil
	if s.stop != nil {
		s.stop()
	}
	for _, v := range s.views {
		v.sent, v.sentLines = false, nil
	}
}

// kickLocked marks a document for checking and starts the worker if none runs. The worker exits when no
// document is left, so nothing stays resident between edits.
func (s *Service) kickLocked(docID string) {
	if !s.active || s.closed {
		return
	}
	s.dirty[docID] = true
	if s.working {
		return
	}
	s.working = true
	s.worker.Add(1)
	go s.work()
}

type job struct {
	docID                 string
	first, last           int
	cursorLine, cursorCol int
}

type scan struct {
	docID   string
	version int64
	first   int
	words   [][]word
}

type event struct {
	topic string
	data  any
}

func (s *Service) work() {
	defer s.worker.Done()
	for {
		s.mu.Lock()
		if !s.active || s.closed || len(s.dirty) == 0 {
			s.working = false
			s.mu.Unlock()
			return
		}
		jobs := make([]job, 0, len(s.dirty))
		for id := range s.dirty {
			if v, ok := s.views[id]; ok {
				jobs = append(jobs, job{docID: id, first: v.first, last: v.last, cursorLine: v.cursorLine, cursorCol: v.cursorCol})
			}
		}
		clear(s.dirty)
		s.loadPersonalLocked()
		gen, bin, dict, checkStrings := s.gen, s.bin, s.lang.Path, s.cfg.Strings
		ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
		s.stop = cancel
		s.mu.Unlock()

		s.batch(ctx, jobs, gen, bin, dict, checkStrings)
		cancel()
	}
}

// batch checks the words of every job with at most one hunspell run, then publishes what changed.
func (s *Service) batch(ctx context.Context, jobs []job, gen int, bin, dict string, checkStrings bool) {
	scans := make([]scan, 0, len(jobs))
	for _, j := range jobs {
		if sc, ok := s.scan(ctx, j, checkStrings); ok {
			scans = append(scans, sc)
		}
	}

	unknown := s.unknown(scans, gen)
	var bad map[string]bool
	if len(unknown) > 0 {
		var err error
		if bad, err = s.run.Check(ctx, bin, dict, unknown); err != nil {
			// Uncached words count as correct until the next pass tries again.
			unknown = nil
		}
	}

	s.mu.Lock()
	if s.gen != gen || !s.active {
		s.mu.Unlock()
		return
	}
	if s.cache == nil || len(s.cache) > maxCacheWords {
		s.cache = map[string]bool{}
	}
	for _, w := range unknown {
		s.cache[w] = !bad[w]
	}
	var events []event
	for _, sc := range scans {
		if e, ok := s.rangesLocked(sc); ok {
			events = append(events, e)
		}
	}
	s.mu.Unlock()
	for _, e := range events {
		s.publish(e.topic, e.data)
	}
}

func (s *Service) scan(ctx context.Context, j job, checkStrings bool) (scan, bool) {
	if s.prose == nil {
		return scan{}, false
	}
	prose, proseVersion, err := s.prose.Lines(ctx, j.docID, j.first, j.last+1)
	if err != nil {
		return scan{}, false
	}
	text, version, size, ok := s.text.read(j.docID, j.first, j.last+1)
	// A newer edit is on its way to DocChanged, which queues this document again.
	if !ok || version != proseVersion {
		return scan{}, false
	}
	sc := scan{docID: j.docID, version: version, first: j.first, words: make([][]word, len(text))}
	if size > maxDocUnits {
		return sc, true
	}
	for i, line := range text {
		if i >= len(prose) {
			break
		}
		cursor := -1
		if j.first+i == j.cursorLine {
			cursor = j.cursorCol
		}
		sc.words[i] = lineWords(line, prose[i], checkStrings, cursor)
	}
	return sc, true
}

func (s *Service) unknown(scans []scan, gen int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, sc := range scans {
		for _, line := range sc.words {
			for _, w := range line {
				if seen[w.key] || s.acceptedLocked(w.key) {
					continue
				}
				if _, ok := s.cache[w.key]; ok {
					continue
				}
				seen[w.key] = true
				out = append(out, w.key)
			}
		}
	}
	return out
}

func (s *Service) acceptedLocked(key string) bool {
	return s.ignored[key] || s.personal[key] || s.personal[strings.ToLower(key)]
}

func (s *Service) misspelledLocked(key string) bool {
	if s.acceptedLocked(key) {
		return false
	}
	correct, ok := s.cache[key]
	return ok && !correct
}

func (s *Service) rangesLocked(sc scan) (event, bool) {
	v, ok := s.views[sc.docID]
	if !ok {
		return event{}, false
	}
	lines := make([][]int, len(sc.words))
	for i, words := range sc.words {
		row := []int{}
		for _, w := range words {
			if s.misspelledLocked(w.key) {
				row = append(row, w.col, w.len)
			}
		}
		lines[i] = row
	}
	same := v.sent && v.sentVersion == sc.version && v.sentFirst == sc.first &&
		slices.EqualFunc(v.sentLines, lines, slices.Equal[[]int])
	if same {
		return event{}, false
	}
	v.sent, v.sentVersion, v.sentFirst, v.sentLines = true, sc.version, sc.first, lines
	return event{topic: TopicEvents, data: map[string]any{
		"type": "ranges", "docId": sc.docID, "version": sc.version, "first": sc.first, "lines": lines,
	}}, true
}

func (s *Service) loadPersonalLocked() {
	if s.personal != nil {
		return
	}
	s.personal = map[string]bool{}
	for _, path := range personalFiles(s.dictionary, s.lang.Code, s.getenv, s.home) {
		readWords(path, s.personal)
	}
}
