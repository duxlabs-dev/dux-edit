// Package swap keeps unsaved text on disk: <id>.txt holds the mirror, <id>.json what it belongs to.
package swap

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-edit/core/internal/atomicfile"
)

var (
	ErrLocked = errors.New("swap is locked by another instance")
	ErrBadID  = errors.New("invalid swap id")
)

type Sidecar struct {
	Path     string `json:"path"`
	Title    string `json:"title"`
	Encoding string `json:"encoding"`
	BOM      bool   `json:"bom"`
	EOL      string `json:"eol"`
	Identity string `json:"identity"`
	// SavedAt is in Unix milliseconds.
	SavedAt int64 `json:"savedAt"`
}

type Entry struct {
	ID string
	Sidecar
	Size int64
}

// Dir is the swap directory; paths.SwapDir creates it 0700.
type Dir string

func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ValidID keeps ids from the socket inside the swap directory.
func ValidID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

func (d Dir) file(id, ext string) string { return filepath.Join(string(d), id+ext) }

func (d Dir) Exists(id string) bool {
	if !ValidID(id) {
		return false
	}
	_, err := os.Stat(d.file(id, ".json"))
	return err == nil
}

// Write stores the text before the sidecar, so a listed sidecar always has its text.
func (d Dir) Write(id, text string, side Sidecar) error {
	if !ValidID(id) {
		return ErrBadID
	}
	if _, err := atomicfile.Save(d.file(id, ".txt"), []byte(text), atomicfile.Options{Mode: 0o600}); err != nil {
		return err
	}
	raw, err := json.Marshal(side)
	if err != nil {
		return err
	}
	_, err = atomicfile.Save(d.file(id, ".json"), raw, atomicfile.Options{Mode: 0o600})
	return err
}

func (d Dir) Read(id string) (string, Sidecar, error) {
	if !ValidID(id) {
		return "", Sidecar{}, ErrBadID
	}
	side, err := d.sidecar(id)
	if err != nil {
		return "", Sidecar{}, err
	}
	text, err := os.ReadFile(d.file(id, ".txt"))
	if err != nil {
		return "", Sidecar{}, err
	}
	return string(text), side, nil
}

func (d Dir) sidecar(id string) (Sidecar, error) {
	var side Sidecar
	raw, err := os.ReadFile(d.file(id, ".json"))
	if err != nil {
		return side, err
	}
	return side, json.Unmarshal(raw, &side)
}

func (d Dir) Remove(id string) {
	if !ValidID(id) {
		return
	}
	os.Remove(d.file(id, ".json"))
	os.Remove(d.file(id, ".txt"))
}

// List returns every swap with readable text and sidecar, locked or not.
func (d Dir) List() ([]Entry, error) {
	names, err := filepath.Glob(filepath.Join(string(d), "*.json"))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, name := range names {
		id := strings.TrimSuffix(filepath.Base(name), ".json")
		if !ValidID(id) {
			continue
		}
		side, err := d.sidecar(id)
		if err != nil {
			continue
		}
		info, err := os.Stat(d.file(id, ".txt"))
		if err != nil {
			continue
		}
		entries = append(entries, Entry{ID: id, Sidecar: side, Size: info.Size()})
	}
	return entries, nil
}

// Lock is an flock on <id>.lock, held while a document uses the swap.
type Lock struct {
	path string
	f    *os.File
}

func (d Dir) Lock(id string) (*Lock, error) {
	if !ValidID(id) {
		return nil, ErrBadID
	}
	path := d.file(id, ".lock")
	// A holder that releases unlinks the file; retry when we locked an inode that is no longer at path.
	for range 3 {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return nil, err
		}
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			f.Close()
			if errors.Is(err, unix.EWOULDBLOCK) {
				return nil, ErrLocked
			}
			return nil, err
		}
		held, err1 := f.Stat()
		current, err2 := os.Stat(path)
		if err1 == nil && err2 == nil && os.SameFile(held, current) {
			return &Lock{path: path, f: f}, nil
		}
		f.Close()
	}
	return nil, ErrLocked
}

func (l *Lock) Release() {
	if l == nil {
		return
	}
	os.Remove(l.path)
	l.f.Close()
}
