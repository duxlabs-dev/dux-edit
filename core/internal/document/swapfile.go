package document

import (
	"errors"
	"os"
	"time"

	"github.com/duxlabs-dev/dux-edit/core/internal/swap"
	"github.com/duxlabs-dev/dux-edit/core/internal/textcodec"
	"github.com/duxlabs-dev/dux-go/log"
)

type restoredSwap struct {
	text string
	side swap.Sidecar
	lock *swap.Lock
}

// restoreSwap returns nil without a failure when the swap does not exist.
func (s *Store) restoreSwap(id string) (*restoredSwap, *Failure) {
	dir, err := s.swaps()
	if err != nil || !dir.Exists(id) {
		return nil, nil
	}
	lock, err := dir.Lock(id)
	switch {
	case errors.Is(err, swap.ErrLocked):
		return nil, &Failure{Code: CodeLocked}
	case err != nil:
		return nil, &Failure{Code: CodeIO, Message: err.Error()}
	}
	text, side, err := dir.Read(id)
	if err != nil {
		lock.Release()
		return nil, &Failure{Code: CodeIO, Message: err.Error()}
	}
	return &restoredSwap{text: text, side: side, lock: lock}, nil
}

func (r *restoredSwap) release() {
	if r != nil {
		r.lock.Release()
	}
}

// apply hands the swap to d, which already holds what is on disk now, and returns the conflict kind.
func (r *restoredSwap) apply(d *Doc, useEncoding bool) string {
	d.lock = r.lock
	d.dirty = true
	if d.path == "" {
		d.title = r.side.Title
	}
	if useEncoding {
		if enc, err := textcodec.Canonical(r.side.Encoding); err == nil {
			d.info.Encoding = enc
			d.info.BOM = r.side.BOM && textcodec.HasBOM(enc)
		}
	}
	if eol, ok := textcodec.ParseEOL(r.side.EOL); ok && !d.info.Mixed {
		d.info.EOL = eol
	}
	onDisk := d.identity
	d.identity = r.side.Identity
	switch {
	case d.path == "" || onDisk == r.side.Identity:
		return ""
	case onDisk == "":
		return "deleted"
	}
	return "modified"
}

func (s *Store) flushSwap(d *Doc) {
	d.swapMu.Lock()
	defer d.swapMu.Unlock()
	if d.released {
		return
	}
	s.writeSwapLocked(d)
}

// writeSwapLocked writes the swap when it is behind the mirror. The caller holds d.swapMu.
func (s *Store) writeSwapLocked(d *Doc) {
	d.mu.Lock()
	if !d.swapStale || d.readElevated {
		d.mu.Unlock()
		return
	}
	d.swapStale = false
	text := d.buf.Text()
	side := swap.Sidecar{
		Path:     d.path,
		Title:    d.title,
		Encoding: d.info.Encoding,
		BOM:      d.info.BOM,
		EOL:      d.info.EOL.String(),
		Identity: d.identity,
		SavedAt:  time.Now().UnixMilli(),
	}
	d.mu.Unlock()

	if err := s.storeSwap(d, text, side); err != nil {
		log.Warnf("swap %s: %v", d.swapID, err)
		d.mu.Lock()
		d.swapStale = true
		d.mu.Unlock()
	}
}

func (s *Store) storeSwap(d *Doc, text string, side swap.Sidecar) error {
	dir, err := s.swaps()
	if err != nil {
		return err
	}
	if d.lock == nil {
		lock, err := dir.Lock(d.swapID)
		if err != nil {
			return err
		}
		d.lock = lock
	}
	return dir.Write(d.swapID, text, side)
}

// removeSwapLocked deletes the swap and its lock. The caller holds d.swapMu.
func (s *Store) removeSwapLocked(d *Doc) {
	if d.lock == nil {
		return
	}
	if dir, err := s.swaps(); err == nil {
		dir.Remove(d.swapID)
	}
	d.lock.Release()
	d.lock = nil
}

// Flush writes every pending swap now.
func (s *Store) Flush() {
	s.mu.Lock()
	docs := make([]*Doc, 0, len(s.docs))
	for _, d := range s.docs {
		docs = append(docs, d)
	}
	s.mu.Unlock()
	for _, d := range docs {
		s.flushSwap(d)
	}
}

// Recovery lists swaps that no open document and no other instance holds.
func (s *Store) Recovery() []RecoveryItem {
	dir, err := s.swaps()
	if err != nil {
		return nil
	}
	entries, err := dir.List()
	if err != nil {
		return nil
	}
	open := s.openSwaps()
	var items []RecoveryItem
	for _, e := range entries {
		if open[e.ID] {
			continue
		}
		lock, err := dir.Lock(e.ID)
		if err != nil {
			continue
		}
		lock.Release()
		items = append(items, RecoveryItem{SwapID: e.ID, Path: e.Path, Title: e.Title, SavedAt: e.SavedAt, Size: e.Size})
	}
	return items
}

func (s *Store) openSwaps() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	open := make(map[string]bool, len(s.docs))
	for _, d := range s.docs {
		open[d.swapID] = true
	}
	return open
}

func (s *Store) DiscardRecovery(swapID string) any {
	if !swap.ValidID(swapID) {
		return Fail(CodeNotFound, "")
	}
	if s.openSwaps()[swapID] {
		return Fail(CodeLocked, "")
	}
	dir, err := s.swaps()
	if err != nil {
		return Fail(CodeIO, err.Error())
	}
	lock, err := dir.Lock(swapID)
	switch {
	case errors.Is(err, swap.ErrLocked):
		return Fail(CodeLocked, "")
	case errors.Is(err, os.ErrNotExist):
		return map[string]any{"ok": true}
	case err != nil:
		return Fail(CodeIO, err.Error())
	}
	dir.Remove(swapID)
	lock.Release()
	return map[string]any{"ok": true}
}
