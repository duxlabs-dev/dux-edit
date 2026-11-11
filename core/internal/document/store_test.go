package document

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/watch"
)

type fakeNotifier struct {
	mu      sync.Mutex
	onEvent func(string)
}

func (f *fakeNotifier) Add(string) error    { return nil }
func (f *fakeNotifier) Remove(string) error { return nil }
func (f *fakeNotifier) Close() error        { return nil }

func (f *fakeNotifier) emit(path string) {
	f.mu.Lock()
	on := f.onEvent
	f.mu.Unlock()
	on(path)
}

type harness struct {
	*Store
	notifier *fakeNotifier
	mu       sync.Mutex
	events   []map[string]any
}

func newHarness(t *testing.T, swapDir string) *harness {
	t.Helper()
	h := &harness{notifier: &fakeNotifier{}}
	h.Store = NewStore(func(topic string, data any) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.events = append(h.events, data.(map[string]any))
	})
	h.SetSwapDir(swapDir)
	h.watchSource = func(on func(string)) (watch.Notifier, error) {
		h.notifier.mu.Lock()
		h.notifier.onEvent = on
		h.notifier.mu.Unlock()
		return h.notifier, nil
	}
	t.Cleanup(h.Shutdown)
	return h
}

func (h *harness) changes() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var kinds []string
	for _, ev := range h.events {
		if ev["type"] == "changed" {
			kinds = append(kinds, ev["kind"].(string))
		}
	}
	return kinds
}

func (h *harness) open(t *testing.T, req OpenRequest) OpenResult {
	t.Helper()
	raw := h.Open(req)
	res, ok := raw.(OpenResult)
	require.True(t, ok, "open failed: %#v", raw)
	return res
}

func (h *harness) edit(t *testing.T, docID string, version int64, start, removed int, text string) {
	t.Helper()
	d, ok := h.Get(docID)
	require.True(t, ok)
	length := 0
	d.Read(func(b *buffer.Buffer, _ int64) { length = b.Len16() - removed + buffer.Len16(text) })
	h.Edit(docID, version, start, removed, text, length)
	require.Equal(t, version, d.Version())
}

func (h *harness) save(req SaveRequest) any {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return h.Save(ctx, req)
}

func qtMD5(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestMixedLineEndingsSurviveAnEdit(t *testing.T) {
	h := newHarness(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "mixed.txt")
	require.NoError(t, os.WriteFile(path, []byte("a\r\nb\nc\r\nd"), 0o644))

	doc := h.open(t, OpenRequest{Path: path})
	require.True(t, doc.MixedEOL)
	require.Equal(t, "crlf", doc.EOL)
	h.edit(t, doc.DocID, 1, 3, 0, "X")
	h.edit(t, doc.DocID, 2, 1, 0, "\nnew")

	res := h.save(SaveRequest{DocID: doc.DocID, Version: 2})
	require.IsType(t, SaveResult{}, res)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "a\r\nnew\r\nbX\nc\r\nd", string(data))

	h.save(SaveRequest{DocID: doc.DocID, Version: 2, EOL: "lf"})
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "a\nnew\nbX\nc\nd", string(data))
}

func TestSaveChecksTheFileAndTheText(t *testing.T) {
	h := newHarness(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("a b"), 0o644))
	doc := h.open(t, OpenRequest{Path: path})
	h.edit(t, doc.DocID, 1, 3, 0, "!")

	assert.Equal(t, Fail(CodeDesync, ""), h.save(SaveRequest{DocID: doc.DocID, Version: 1, MD5: qtMD5("a b!")}),
		"TextEdit reports a no-break space as a space")
	res := h.save(SaveRequest{DocID: doc.DocID, Version: 1, MD5: qtMD5("a b!")})
	require.IsType(t, SaveResult{}, res)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "a b!", string(data), "the file keeps the no-break space")

	require.NoError(t, os.WriteFile(path, []byte("changed elsewhere"), 0o644))
	assert.Equal(t, Fail(CodeConflict, ""), h.save(SaveRequest{DocID: doc.DocID, Version: 1}))
	assert.IsType(t, SaveResult{}, h.save(SaveRequest{DocID: doc.DocID, Version: 1, Force: true}))

	require.NoError(t, os.Remove(path))
	assert.IsType(t, SaveResult{}, h.save(SaveRequest{DocID: doc.DocID, Version: 1}), "a deleted file is recreated")

	other := filepath.Join(filepath.Dir(path), "other.txt")
	require.NoError(t, os.WriteFile(other, []byte("keep"), 0o644))
	assert.Equal(t, Fail(CodeExists, ""), h.save(SaveRequest{DocID: doc.DocID, Version: 1, Path: other}))
}

func TestOpenFailures(t *testing.T) {
	h := newHarness(t, t.TempDir())
	dir := t.TempDir()
	binary := filepath.Join(dir, "blob")
	require.NoError(t, os.WriteFile(binary, []byte("a\x00b"), 0o644))

	assert.Equal(t, CodeIsDir, h.Open(OpenRequest{Path: dir}).(Failure).Code)
	assert.Equal(t, CodeBinary, h.Open(OpenRequest{Path: binary}).(Failure).Code)
	assert.Equal(t, Fail(CodeNotFound, ""), h.Open(OpenRequest{SwapID: "0123456789abcdef"}))

	missing := h.open(t, OpenRequest{Path: filepath.Join(dir, "new.txt")})
	assert.Equal(t, "", missing.Text)
	assert.True(t, missing.Writable)
}

func TestSwapRecoversUnsavedEdits(t *testing.T) {
	swapDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

	first := newHarness(t, swapDir)
	doc := first.open(t, OpenRequest{Path: path})
	first.edit(t, doc.DocID, 1, 5, 0, " world")
	first.Shutdown()
	assert.FileExists(t, filepath.Join(swapDir, doc.SwapID+".txt"))

	second := newHarness(t, swapDir)
	restored := second.open(t, OpenRequest{Path: path, SwapID: doc.SwapID})
	assert.Equal(t, "hello world", restored.Text)
	assert.True(t, restored.Dirty)
	assert.True(t, restored.Recovered)
	assert.Empty(t, restored.Conflict)

	require.IsType(t, SaveResult{}, second.save(SaveRequest{DocID: restored.DocID}))
	assert.NoFileExists(t, filepath.Join(swapDir, doc.SwapID+".txt"))
	assert.NoFileExists(t, filepath.Join(swapDir, doc.SwapID+".json"))
}

func TestRecoverySkipsSwapsHeldByAnotherInstance(t *testing.T) {
	swapDir := t.TempDir()
	first := newHarness(t, swapDir)
	draft := first.open(t, OpenRequest{})
	first.edit(t, draft.DocID, 1, 0, 0, "draft text")
	first.SetTitle(draft.DocID, "Ideas")
	first.Flush()

	second := newHarness(t, swapDir)
	assert.Empty(t, second.Recovery())
	assert.Equal(t, Fail(CodeLocked, ""), second.Open(OpenRequest{SwapID: draft.SwapID}))

	first.Close(draft.DocID, false)
	items := second.Recovery()
	require.Len(t, items, 1)
	assert.Equal(t, "Ideas", items[0].Title)
	restored := second.open(t, OpenRequest{SwapID: draft.SwapID})
	assert.Equal(t, "draft text", restored.Text)
	assert.Equal(t, "Ideas", restored.Title)
}

func TestWatchReportsOnlyForeignChanges(t *testing.T) {
	h := newHarness(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))
	doc := h.open(t, OpenRequest{Path: path})
	require.True(t, doc.Watched)

	h.edit(t, doc.DocID, 1, 5, 0, "!")
	require.IsType(t, SaveResult{}, h.save(SaveRequest{DocID: doc.DocID, Version: 1}))
	h.notifier.emit(path)
	assert.Never(t, func() bool { return len(h.changes()) > 0 }, 300*time.Millisecond, 20*time.Millisecond)

	require.NoError(t, os.WriteFile(path, []byte("from elsewhere"), 0o644))
	h.notifier.emit(path)
	assert.Eventually(t, func() bool { return assert.ObjectsAreEqual([]string{"modified"}, h.changes()) }, time.Second, 10*time.Millisecond)

	require.NoError(t, os.Remove(path))
	h.notifier.emit(path)
	assert.Eventually(t, func() bool { return assert.ObjectsAreEqual([]string{"modified", "deleted"}, h.changes()) }, time.Second, 10*time.Millisecond)
}

func TestResetKeepsWhatTextEditCannotReport(t *testing.T) {
	h := newHarness(t, t.TempDir())
	path := filepath.Join(t.TempDir(), "note.txt")
	require.NoError(t, os.WriteFile(path, []byte("a b\nc d\ne "), 0o644))
	doc := h.open(t, OpenRequest{Path: path})

	h.Reset(doc.DocID, 1, "a b\nc\nd!\ne ")
	require.IsType(t, SaveResult{}, h.save(SaveRequest{DocID: doc.DocID, Version: 1, MD5: qtMD5("a b\nc\nd!\ne ")}))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "a b\nc d!\ne ", string(data))
}

type fakeElevator struct {
	identity string
	replaced []string
	created  int
}

func (f *fakeElevator) Available() string { return "" }

func (f *fakeElevator) Read(string) ([]byte, string, error) { return []byte("secret"), f.identity, nil }

func (f *fakeElevator) Replace(_ string, _ []byte, identity string) (string, error) {
	f.replaced = append(f.replaced, identity)
	return "1:2:9:9", nil
}

func (f *fakeElevator) Create(string, []byte) (string, error) {
	f.created++
	return "1:2:0:1", nil
}

func TestElevatedSaveChecksTheIdentityTheHelperReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can stat everything")
	}
	h := newHarness(t, t.TempDir())
	elevator := &fakeElevator{identity: "1:2:3:4"}
	h.SetElevator(elevator)
	locked := filepath.Join(t.TempDir(), "locked")
	require.NoError(t, os.Mkdir(locked, 0o700))
	path := filepath.Join(locked, "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("secret"), 0o600))
	require.NoError(t, os.Chmod(locked, 0))
	t.Cleanup(func() { os.Chmod(locked, 0o700) })

	doc := h.open(t, OpenRequest{Path: path, Elevated: true})
	assert.False(t, doc.Recoverable)
	require.IsType(t, SaveResult{}, h.save(SaveRequest{DocID: doc.DocID}))
	require.IsType(t, SaveResult{}, h.save(SaveRequest{DocID: doc.DocID}))
	assert.Equal(t, []string{"1:2:3:4", "1:2:9:9"}, elevator.replaced, "never the empty identity that skips the check")

	copyPath := filepath.Join(t.TempDir(), "copy.txt")
	res, ok := h.save(SaveRequest{DocID: doc.DocID, Path: copyPath}).(SaveResult)
	require.True(t, ok)
	assert.False(t, res.Elevated, "a copy in the user's directory is written as the user")
	assert.True(t, res.Recoverable)
	assert.Zero(t, elevator.created)
	data, err := os.ReadFile(copyPath)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(data))
}
