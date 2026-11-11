// Package watch reports changes to open files. It watches parent directories, because editors that
// save by rename kill a watch on the file itself.
package watch

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const coalesce = 100 * time.Millisecond

// Notifier is the part of fsnotify the watcher uses, so tests can inject events.
type Notifier interface {
	Add(dir string) error
	Remove(dir string) error
	Close() error
}

// Source starts a notifier that calls onEvent with the full path of every event in a watched directory.
type Source func(onEvent func(path string)) (Notifier, error)

func FSNotify(onEvent func(path string)) (Notifier, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			select {
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				onEvent(ev.Name)
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return w, nil
}

// Watcher holds one notifier while at least one file is watched.
type Watcher struct {
	source   Source
	onChange func(path string)

	mu      sync.Mutex
	n       Notifier
	dirs    map[string]int
	files   map[string]int
	pending map[string]struct{}
	armed   bool
}

// New calls onChange, at most once per coalescing window and path, after a watched path changes.
func New(source Source, onChange func(path string)) *Watcher {
	return &Watcher{
		source:   source,
		onChange: onChange,
		dirs:     make(map[string]int),
		files:    make(map[string]int),
		pending:  make(map[string]struct{}),
	}
}

func (w *Watcher) Add(path string) error {
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.n == nil {
		n, err := w.source(w.event)
		if err != nil {
			return err
		}
		w.n = n
	}
	if w.dirs[dir] == 0 {
		if err := w.n.Add(dir); err != nil {
			w.closeIfIdle()
			return err
		}
	}
	w.dirs[dir]++
	w.files[path]++
	return nil
}

func (w *Watcher) Remove(path string) {
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.files[path] == 0 {
		return
	}
	if w.files[path]--; w.files[path] == 0 {
		delete(w.files, path)
		delete(w.pending, path)
	}
	if w.dirs[dir]--; w.dirs[dir] == 0 {
		delete(w.dirs, dir)
		_ = w.n.Remove(dir)
	}
	w.closeIfIdle()
}

func (w *Watcher) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	clear(w.dirs)
	clear(w.files)
	w.closeIfIdle()
}

// fsnotify's Close waits for its reader, which may be blocked delivering to event, so it runs unlocked.
func (w *Watcher) closeIfIdle() {
	if len(w.dirs) > 0 || w.n == nil {
		return
	}
	n := w.n
	w.n = nil
	clear(w.pending)
	go n.Close()
}

func (w *Watcher) event(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.files[path] == 0 {
		return
	}
	w.pending[path] = struct{}{}
	if w.armed {
		return
	}
	w.armed = true
	time.AfterFunc(coalesce, w.fire)
}

func (w *Watcher) fire() {
	w.mu.Lock()
	paths := make([]string, 0, len(w.pending))
	for path := range w.pending {
		paths = append(paths, path)
	}
	clear(w.pending)
	w.armed = false
	w.mu.Unlock()
	for _, path := range paths {
		w.onChange(path)
	}
}
