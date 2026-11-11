package watch

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fake struct {
	mu      sync.Mutex
	dirs    map[string]bool
	closed  bool
	onEvent func(string)
}

func (f *fake) Add(dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dirs[dir] = true
	return nil
}

func (f *fake) Remove(dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.dirs, dir)
	return nil
}

func (f *fake) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func TestWatcherCoalescesFiltersAndReleases(t *testing.T) {
	var notifiers []*fake
	source := func(on func(string)) (Notifier, error) {
		f := &fake{dirs: map[string]bool{}, onEvent: on}
		notifiers = append(notifiers, f)
		return f, nil
	}
	changed := make(chan string, 8)
	w := New(source, func(path string) { changed <- path })

	require.NoError(t, w.Add("/d/a.txt"))
	require.NoError(t, w.Add("/d/a.txt"))
	require.NoError(t, w.Add("/d/b.txt"))
	require.Len(t, notifiers, 1, "one notifier for the process")
	n := notifiers[0]

	n.onEvent("/d/a.txt")
	n.onEvent("/d/a.txt")
	n.onEvent("/d/other.txt")
	assert.Equal(t, "/d/a.txt", <-changed)
	select {
	case extra := <-changed:
		t.Fatalf("unexpected change %s", extra)
	case <-time.After(2 * coalesce):
	}

	w.Remove("/d/a.txt")
	w.Remove("/d/b.txt")
	n.mu.Lock()
	assert.True(t, n.dirs["/d"], "a.txt is still open once")
	n.mu.Unlock()
	w.Remove("/d/a.txt")
	assert.Eventually(t, func() bool {
		n.mu.Lock()
		defer n.mu.Unlock()
		return n.closed && len(n.dirs) == 0
	}, time.Second, 5*time.Millisecond)
}
