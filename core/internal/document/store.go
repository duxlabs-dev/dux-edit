// Package document owns every open document: the text mirror, file I/O, save, swap and the file watch.
package document

import (
	"context"
	"errors"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/paths"
	"github.com/duxlabs-dev/dux-edit/core/internal/swap"
	"github.com/duxlabs-dev/dux-edit/core/internal/textcodec"
	"github.com/duxlabs-dev/dux-edit/core/internal/watch"
)

const (
	resyncAfter  = 2 * time.Second
	swapAfter    = time.Second
	swapAfterMax = 15 * time.Second
	// staleIdentity matches no file, so the next save of a document marked with it is a conflict.
	staleIdentity = "stale"
)

// swapDelay grows with the document, since every swap write is the whole text. Hide and quit flush at once.
func swapDelay(len16 int) time.Duration {
	return min(max(swapAfter, swapAfter*time.Duration(len16>>20)), swapAfterMax)
}

type Publisher func(topic string, data any)

type Store struct {
	publish     Publisher
	elevator    Elevator
	watchSource watch.Source

	mu        sync.Mutex
	docs      map[string]*Doc
	listeners []Listener
	nextID    int
	swapDir   swap.Dir
	watcher   *watch.Watcher
	// parked holds the locks of swaps whose documents closed with unsaved edits, so no other instance
	// offers them for recovery while this one is only hidden.
	parked map[string]*swap.Lock
}

func NewStore(publish Publisher) *Store {
	return &Store{publish: publish, docs: make(map[string]*Doc), parked: make(map[string]*swap.Lock), watchSource: watch.FSNotify}
}

func (s *Store) SetElevator(e Elevator) { s.elevator = e }

// SetSwapDir overrides paths.SwapDir, which is otherwise resolved on the first swap.
func (s *Store) SetSwapDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.swapDir = swap.Dir(dir)
}

// AddListener must be called before the server starts accepting requests.
func (s *Store) AddListener(l Listener) { s.listeners = append(s.listeners, l) }

func (s *Store) Get(docID string) (*Doc, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[docID]
	return d, ok
}

func (s *Store) swaps() (swap.Dir, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.swapDir != "" {
		return s.swapDir, nil
	}
	dir, err := paths.SwapDir()
	if err != nil {
		return "", err
	}
	s.swapDir = swap.Dir(dir)
	return s.swapDir, nil
}

type noticeKind uint8

const (
	noticeChanged noticeKind = iota
	noticeReset
	noticeSaved
	noticeClosed
)

type notice struct {
	kind   noticeKind
	change buffer.Change
}

type Doc struct {
	ID string

	// saveMu serializes save, reload, close and the on-disk check of a file event.
	saveMu sync.Mutex
	// swapMu guards lock and released, and serializes swap writes and deletes.
	swapMu   sync.Mutex
	lock     *swap.Lock
	released bool

	mu       sync.RWMutex
	buf      *buffer.Buffer
	version  int64
	pending  map[int64]edit
	changed  chan struct{}
	desynced bool
	gapTimer *time.Timer

	path     string
	title    string
	info     textcodec.Info
	ends     []textcodec.EOL
	identity string
	// reported is the on-disk identity the UI last heard about, so one change is published once.
	reported  string
	elevated  bool
	writable  bool
	watchPath string

	// lossy: the encoding does not write this file's bytes back, so a save in it would change text nobody touched.
	lossy bool

	swapID       string
	readElevated bool
	dirty        bool
	swapStale    bool
	swapFailed   bool
	swapTimer    *time.Timer
	closed       bool

	outbox    []notice
	notifying bool
}

type edit struct {
	start, removed, length int
	text                   string
}

func (d *Doc) Path() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.path
}

func (d *Doc) Version() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.version
}

// Read runs fn with the mirror read-locked. fn must not keep the buffer.
func (d *Doc) Read(fn func(b *buffer.Buffer, version int64)) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	fn(d.buf, d.version)
}

var ErrBehind = errors.New("mirror did not reach the requested version")

// Await blocks until the mirror is at version or later. Requests race on the socket, so a read tagged
// with version N can arrive before edit N.
func (d *Doc) Await(ctx context.Context, version int64) error {
	for {
		d.mu.RLock()
		current, changed := d.version, d.changed
		d.mu.RUnlock()
		if current >= version {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ErrBehind
		}
	}
}

func (d *Doc) bumpLocked() {
	close(d.changed)
	d.changed = make(chan struct{})
}

// queueLocked reports whether the caller must drain the outbox once it has unlocked.
func (d *Doc) queueLocked(n ...notice) bool {
	d.outbox = append(d.outbox, n...)
	if d.notifying {
		return false
	}
	d.notifying = true
	return true
}

// drain delivers notices in order without the doc lock; one goroutine drains at a time.
func (s *Store) drain(d *Doc) {
	for {
		d.mu.Lock()
		batch := d.outbox
		d.outbox = nil
		if len(batch) == 0 {
			d.notifying = false
			d.mu.Unlock()
			return
		}
		d.mu.Unlock()
		for _, n := range batch {
			for _, l := range s.listeners {
				switch n.kind {
				case noticeChanged:
					l.DocChanged(d, n.change)
				case noticeReset:
					l.DocReset(d)
				case noticeSaved:
					l.DocSaved(d)
				case noticeClosed:
					l.DocClosed(d.ID)
				}
			}
		}
	}
}

func (s *Store) Open(req OpenRequest) any {
	path := req.Path
	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return Fail(CodeIO, err.Error())
		}
		path = abs
	}

	swapID := req.SwapID
	var restored *restoredSwap
	switch {
	case swapID == "":
		swapID = swap.NewID()
	case !swap.ValidID(swapID):
		return Fail(CodeNotFound, "")
	default:
		if fail := s.releaseHolder(swapID); fail != nil {
			return *fail
		}
		if req.Elevated {
			break
		}
		r, fail := s.restoreSwap(swapID)
		if fail != nil {
			return *fail
		}
		restored = r
	}
	if restored != nil && restored.side.Path != "" {
		path = restored.side.Path
	}
	if restored == nil && req.SwapID != "" && path == "" {
		return Fail(CodeNotFound, "")
	}

	d := &Doc{
		swapID:       swapID,
		path:         path,
		pending:      make(map[int64]edit),
		changed:      make(chan struct{}),
		writable:     true,
		info:         textcodec.Info{Encoding: textcodec.UTF8},
		readElevated: req.Elevated,
		elevated:     req.Elevated,
	}
	result := OpenResult{OK: true, SwapID: swapID, Path: path, Elevated: req.Elevated, Recoverable: !req.Elevated}
	text := ""
	unreadable := false
	if path != "" {
		ld, fail := s.load(path, req.Encoding, req.Elevated, req.Force)
		switch {
		case fail == nil:
			text, d.info, d.ends = ld.doc.Text, ld.doc.Info, ld.doc.Ends
			d.identity, d.reported, d.lossy = ld.identity, ld.identity, ld.doc.Lossy
			result.Size, result.Missing = ld.size, ld.missing
			result.ReadOnly, result.EncodingGuessed = ld.doc.Lossy, ld.doc.Fallback
		case restored == nil || fail.Code == CodeConfirmLarge:
			restored.release()
			return *fail
		default:
			unreadable = true
		}
		d.writable = ld.writable
		d.title = filepath.Base(path)
	}
	if restored != nil {
		if size := int64(len(restored.text)); size > MaxOpenBytes || (!req.Force && size > ConfirmLargeBytes) {
			restored.release()
			return largeFailure(size)
		}
		result.Conflict = restored.apply(d, req.Encoding == "")
		if unreadable {
			result.Conflict = "modified"
		}
		fresh := buffer.New(restored.text)
		switch ends, ok := decodeEnds(restored.side.Ends, fresh.LineCount()); {
		case ok:
			d.ends, d.info.Mixed = ends, true
		case d.ends != nil:
			d.ends = carryEnds(buffer.New(text), fresh, d.ends, d.info.EOL)
		}
		text = restored.text
		result.Recovered, result.Dirty = true, true
		if path == "" {
			result.Size = int64(len(text))
		}
	}
	d.buf = buffer.New(text)

	s.mu.Lock()
	s.nextID++
	d.ID = "d" + strconv.Itoa(s.nextID)
	s.docs[d.ID] = d
	s.mu.Unlock()

	result.Watched = s.watchDoc(d)
	for _, l := range s.listeners {
		l.DocOpened(d)
	}

	result.DocID = d.ID
	result.Title = d.title
	result.Text = text
	result.Encoding = d.info.Encoding
	result.BOM = d.info.BOM
	result.EOL = d.info.EOL.String()
	result.MixedEOL = d.info.Mixed
	result.Writable = d.writable
	result.setIndent(path, text)
	return result
}

// Edit applies edits in version order and parks the ones that arrive early.
func (s *Store) Edit(docID string, version int64, start, removed int, text string, length int) {
	d, ok := s.Get(docID)
	if !ok {
		return
	}
	d.mu.Lock()
	if d.closed || version <= d.version {
		d.mu.Unlock()
		return
	}
	d.pending[version] = edit{start: start, removed: removed, text: text, length: length}
	if d.desynced {
		d.mu.Unlock()
		return
	}
	desync, drain := s.applyPendingLocked(d)
	d.mu.Unlock()

	if drain {
		s.drain(d)
	}
	if desync {
		s.resync(d)
	}
}

// applyPendingLocked applies the parked edits that follow the mirror's version.
func (s *Store) applyPendingLocked(d *Doc) (desync, drain bool) {
	applied := false
	for {
		next, ready := d.pending[d.version+1]
		if !ready {
			break
		}
		delete(d.pending, d.version+1)
		change, err := d.buf.Replace(next.start, next.removed, next.text)
		if err != nil || d.buf.Len16() != next.length {
			desync = true
			break
		}
		if d.ends != nil {
			d.ends = spliceEnds(d.ends, change, d.info.EOL)
		}
		d.version++
		applied = true
		drain = d.queueLocked(notice{kind: noticeChanged, change: change}) || drain
	}
	if applied {
		d.bumpLocked()
		s.touchLocked(d)
		d.stopGapLocked()
	}
	if desync {
		d.desynced = true
		clear(d.pending)
	}
	if len(d.pending) > 0 && d.gapTimer == nil {
		s.armGapLocked(d)
	}
	return desync, drain
}

func (s *Store) resync(d *Doc) {
	s.publish("docs", map[string]any{"type": "resync", "docId": d.ID})
}

func (s *Store) armGapLocked(d *Doc) {
	var t *time.Timer
	t = time.AfterFunc(resyncAfter, func() {
		d.mu.Lock()
		if d.gapTimer != t || d.closed || len(d.pending) == 0 {
			d.mu.Unlock()
			return
		}
		d.gapTimer = nil
		d.desynced = true
		clear(d.pending)
		d.mu.Unlock()
		s.resync(d)
	})
	d.gapTimer = t
}

func (d *Doc) stopGapLocked() {
	if d.gapTimer == nil {
		return
	}
	d.gapTimer.Stop()
	d.gapTimer = nil
}

// touchLocked marks the mirror ahead of disk and of the swap, and arms the swap write.
func (s *Store) touchLocked(d *Doc) {
	d.dirty = true
	if d.readElevated || d.closed {
		return
	}
	d.swapStale = true
	delay := swapDelay(d.buf.Len16())
	if d.swapTimer != nil {
		d.swapTimer.Reset(delay)
		return
	}
	d.swapTimer = time.AfterFunc(delay, func() { _ = s.flushSwap(d) })
}

// Reset replaces the mirror with the UI's text at version; conflict marks the file as changed under it.
func (s *Store) Reset(docID string, version int64, text string, conflict bool) any {
	d, ok := s.Get(docID)
	if !ok {
		return Fail(CodeUnknownDoc, "")
	}
	d.mu.Lock()
	switch {
	case d.closed:
		d.mu.Unlock()
		return Fail(CodeUnknownDoc, "")
	case version < d.version && !d.desynced:
		if conflict {
			d.identity = staleIdentity
		}
		d.mu.Unlock()
		return map[string]any{"ok": true}
	}
	fresh := buffer.New(mergeReset(d.buf.Text(), text))
	if d.ends != nil {
		d.ends = carryEnds(d.buf, fresh, d.ends, d.info.EOL)
	}
	d.buf = fresh
	d.version = version
	for v := range d.pending {
		if v <= version {
			delete(d.pending, v)
		}
	}
	d.desynced = false
	if conflict {
		d.identity = staleIdentity
	}
	d.stopGapLocked()
	d.bumpLocked()
	s.touchLocked(d)
	drain := d.queueLocked(notice{kind: noticeReset})
	desync, more := s.applyPendingLocked(d)
	d.mu.Unlock()

	if drain || more {
		s.drain(d)
	}
	if desync {
		s.resync(d)
	}
	return map[string]any{"ok": true}
}

func (s *Store) elevation() string {
	if s.elevator == nil {
		return "NOBINARY"
	}
	if reason := s.elevator.Available(); reason != "" {
		return reason
	}
	return "available"
}

// Elevation reports "available" or the reason elevated saves cannot be offered.
func (s *Store) Elevation() string { return s.elevation() }

// Close closes a document. A kept one first proves its swap reached disk; otherwise it stays open.
func (s *Store) Close(ctx context.Context, req CloseRequest) any {
	d, ok := s.Get(req.DocID)
	if !ok {
		return Fail(CodeUnknownDoc, "")
	}
	if !req.Discard && d.Await(ctx, req.Version) != nil {
		return Fail(CodeDesync, "")
	}
	return s.closeDoc(d, req.Discard, req.MD5, true)
}

func (s *Store) closeDoc(d *Doc, discard bool, sum string, verify bool) any {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()

	d.mu.Lock()
	switch {
	case d.closed:
		d.mu.Unlock()
		return Fail(CodeUnknownDoc, "")
	case discard:
	case verify && (d.desynced || len(d.pending) > 0):
		d.mu.Unlock()
		return Fail(CodeDesync, "")
	case verify && sum != "" && !strings.EqualFold(sum, plainTextMD5(d.buf.Text())):
		d.mu.Unlock()
		return Fail(CodeDesync, "")
	case d.dirty && d.readElevated:
		d.mu.Unlock()
		return Fail(CodeIO, "a document read as administrator has no swap")
	}
	d.closed = true
	d.stopGapLocked()
	if d.swapTimer != nil {
		d.swapTimer.Stop()
	}
	d.mu.Unlock()

	d.swapMu.Lock()
	if discard {
		s.removeSwapLocked(d)
	} else if err := s.writeSwapLocked(d); err != nil {
		d.swapMu.Unlock()
		d.mu.Lock()
		d.closed = false
		d.mu.Unlock()
		return Fail(CodeIO, err.Error())
	}
	s.parkLocked(d)
	d.released = true
	d.swapMu.Unlock()

	s.mu.Lock()
	delete(s.docs, d.ID)
	last := len(s.docs) == 0
	s.mu.Unlock()

	d.mu.Lock()
	watched := d.watchPath
	d.watchPath = ""
	drain := d.queueLocked(notice{kind: noticeClosed})
	d.mu.Unlock()
	s.unwatch(watched)
	if drain {
		s.drain(d)
	}
	if last {
		// A large document's mirror would otherwise stay resident under a hidden window until the next GC.
		go debug.FreeOSMemory()
	}
	return map[string]any{"ok": true}
}

// releaseHolder closes the document still open on swapID after a UI restart, so its swap carries the text.
func (s *Store) releaseHolder(swapID string) *Failure {
	s.mu.Lock()
	var holder *Doc
	for _, d := range s.docs {
		if d.swapID == swapID {
			holder = d
			break
		}
	}
	s.mu.Unlock()
	if holder == nil {
		return nil
	}
	if failure, failed := s.closeDoc(holder, false, "", false).(Failure); failed && failure.Code != CodeUnknownDoc {
		return &Failure{Code: CodeLocked, Message: failure.Message}
	}
	return nil
}

// SetTitle names a draft; the title reaches recovery through the swap sidecar.
func (s *Store) SetTitle(docID, title string) any {
	d, ok := s.Get(docID)
	if !ok {
		return Fail(CodeUnknownDoc, "")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.title = title
	if d.dirty {
		s.touchLocked(d)
	}
	return map[string]any{"ok": true}
}

func (s *Store) Shutdown() {
	s.Flush()
	s.mu.Lock()
	docs := make([]*Doc, 0, len(s.docs))
	for _, d := range s.docs {
		docs = append(docs, d)
	}
	watcher := s.watcher
	s.watcher = nil
	s.mu.Unlock()

	for _, d := range docs {
		d.mu.Lock()
		if d.swapTimer != nil {
			d.swapTimer.Stop()
		}
		d.stopGapLocked()
		d.mu.Unlock()
		d.swapMu.Lock()
		d.lock.Release()
		d.lock = nil
		d.released = true
		d.swapMu.Unlock()
	}
	s.mu.Lock()
	parked := s.parked
	s.parked = make(map[string]*swap.Lock)
	s.mu.Unlock()
	for _, lock := range parked {
		lock.Release()
	}
	if watcher != nil {
		watcher.Close()
	}
}
