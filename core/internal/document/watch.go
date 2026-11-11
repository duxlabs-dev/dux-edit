package document

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/duxlabs-dev/dux-edit/core/internal/atomicfile"
	"github.com/duxlabs-dev/dux-edit/core/internal/watch"
)

// watchDoc watches the file d.path resolves to. Files read through elevation are never watched.
func (s *Store) watchDoc(d *Doc) bool {
	d.mu.RLock()
	path, skip := d.path, d.readElevated || d.closed || d.watchPath != ""
	d.mu.RUnlock()
	if path == "" || skip {
		return false
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		target = path
	}

	s.mu.Lock()
	if s.watcher == nil {
		s.watcher = watch.New(s.watchSource, s.fileChanged)
	}
	w := s.watcher
	s.mu.Unlock()
	if w.Add(target) != nil {
		return false
	}
	d.mu.Lock()
	d.watchPath = target
	d.mu.Unlock()
	return true
}

func (s *Store) unwatch(target string) {
	if target == "" {
		return
	}
	s.mu.Lock()
	w := s.watcher
	s.mu.Unlock()
	if w != nil {
		w.Remove(target)
	}
}

func (s *Store) fileChanged(target string) {
	s.mu.Lock()
	var docs []*Doc
	for _, d := range s.docs {
		d.mu.RLock()
		if d.watchPath == target {
			docs = append(docs, d)
		}
		d.mu.RUnlock()
	}
	s.mu.Unlock()
	for _, d := range docs {
		// A save in progress holds saveMu for as long as pkexec takes; other documents must not wait on it.
		go s.checkDisk(d)
	}
}

// checkDisk publishes a change unless the file still has the identity of our last load or save.
func (s *Store) checkDisk(d *Doc) {
	d.saveMu.Lock()
	defer d.saveMu.Unlock()
	d.mu.RLock()
	path, reported, closed := d.path, d.reported, d.closed
	d.mu.RUnlock()
	if closed || path == "" {
		return
	}

	current, err := atomicfile.Stat(path)
	kind := "modified"
	switch {
	case errors.Is(err, os.ErrNotExist):
		if reported == "" {
			return
		}
		kind, current = "deleted", ""
	case err != nil, current == reported:
		return
	}
	d.mu.Lock()
	d.reported = current
	d.mu.Unlock()
	s.publish("docs", map[string]any{"type": "changed", "docId": d.ID, "kind": kind})
}
