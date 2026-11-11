package document

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/duxlabs-dev/dux-edit/core/internal/atomicfile"
	"github.com/duxlabs-dev/dux-edit/core/internal/buffer"
	"github.com/duxlabs-dev/dux-edit/core/internal/helper"
	"github.com/duxlabs-dev/dux-edit/core/internal/textcodec"
)

// ErrConflict is what Elevator.Replace returns when the file no longer has the expected identity. The
// helper's sentinel crosses the pkexec boundary as a code, so it is the one to match.
var ErrConflict = helper.ErrConflict

type loaded struct {
	doc      textcodec.Doc
	identity string
	size     int64
	writable bool
}

func largeFailure(size int64) Failure {
	if size > MaxOpenBytes {
		return Failure{Code: CodeTooLarge, Size: size}
	}
	return Failure{Code: CodeConfirmLarge, Size: size}
}

func (s *Store) accessFailure(err error) *Failure {
	return &Failure{Code: CodeAccess, Message: err.Error(), Elevation: s.elevation()}
}

// load reads and decodes path. A missing file is an empty document that the first save creates.
func (s *Store) load(path, encoding string, elevated, force bool) (loaded, *Failure) {
	ld := loaded{writable: atomicfile.Writable(path)}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		ld.doc.Encoding = textcodec.UTF8
		if encoding != "" {
			ld.doc.Encoding, err = textcodec.Canonical(encoding)
			if err != nil {
				return ld, &Failure{Code: CodeIO, Message: err.Error()}
			}
		}
		return ld, nil
	case errors.Is(err, os.ErrPermission) && !elevated:
		return ld, s.accessFailure(err)
	case err != nil && !elevated:
		return ld, &Failure{Code: CodeIO, Message: err.Error()}
	case err == nil && info.IsDir():
		return ld, &Failure{Code: CodeIsDir}
	case err == nil:
		ld.size = info.Size()
		if ld.size > MaxOpenBytes || (!force && ld.size > ConfirmLargeBytes) {
			return ld, ptr(largeFailure(ld.size))
		}
	}

	var raw []byte
	if elevated {
		if s.elevation() != "available" {
			return ld, &Failure{Code: CodeAccess, Elevation: s.elevation()}
		}
		raw, ld.identity, err = s.elevator.Read(path)
		switch {
		case errors.Is(err, os.ErrPermission):
			return ld, s.accessFailure(err)
		case err != nil:
			return ld, &Failure{Code: CodeIO, Message: err.Error()}
		}
	} else {
		raw, ld.identity, err = readFile(path)
		switch {
		case errors.Is(err, os.ErrPermission):
			return ld, s.accessFailure(err)
		case err != nil:
			return ld, &Failure{Code: CodeIO, Message: err.Error()}
		}
	}
	ld.size = int64(len(raw))
	if ld.size > MaxOpenBytes {
		return ld, ptr(largeFailure(ld.size))
	}

	ld.doc, err = textcodec.Load(raw, encoding)
	switch {
	case err != nil:
		return ld, &Failure{Code: CodeIO, Message: err.Error()}
	case ld.doc.Binary:
		return ld, &Failure{Code: CodeBinary, Size: ld.size}
	}
	return ld, nil
}

// The identity comes from the descriptor that was read, so a write racing the read is not mistaken for ours.
func readFile(path string) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, "", err
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxOpenBytes+1))
	if err != nil {
		return nil, "", err
	}
	return raw, atomicfile.Identity(info), nil
}

func ptr[T any](v T) *T { return &v }

// plainRune is what QTextDocument::toPlainText, and so TextEdit.text, reports for r.
func plainRune(r rune) rune {
	switch r {
	case '\u00a0':
		return ' '
	case '\u2028', '\u2029', '\ufdd0', '\ufdd1':
		return '\n'
	}
	return r
}

// plainTextMD5 hashes text the way Qt.md5 hashes TextEdit.text.
func plainTextMD5(text string) string {
	sum := md5.Sum([]byte(strings.Map(plainRune, text)))
	return hex.EncodeToString(sum[:])
}

// mergeReset returns the UI's text with everything it left alone taken from old, so the no-break spaces
// and separators that TextEdit reports as plain ones survive outside the region that really differs.
func mergeReset(old, text string) string {
	from, textFrom := 0, 0
	for from < len(old) && textFrom < len(text) {
		r, size := utf8.DecodeRuneInString(old[from:])
		t, textSize := utf8.DecodeRuneInString(text[textFrom:])
		if plainRune(r) != t {
			break
		}
		from, textFrom = from+size, textFrom+textSize
	}
	to, textTo := len(old), len(text)
	for to > from && textTo > textFrom {
		r, size := utf8.DecodeLastRuneInString(old[:to])
		t, textSize := utf8.DecodeLastRuneInString(text[:textTo])
		if plainRune(r) != t {
			break
		}
		to, textTo = to-size, textTo-textSize
	}
	return old[:from] + text[textFrom:textTo] + old[to:]
}

type saveSnapshot struct {
	version  int64
	text     string
	lines    []string
	path     string
	info     textcodec.Info
	ends     []textcodec.EOL
	identity string
	elevated bool
}

func (s *Store) Save(ctx context.Context, req SaveRequest) any {
	d, ok := s.Get(req.DocID)
	if !ok {
		return Fail(CodeUnknownDoc, "")
	}
	if d.Await(ctx, req.Version) != nil {
		return Fail(CodeDesync, "")
	}
	d.saveMu.Lock()
	defer d.saveMu.Unlock()

	var snap saveSnapshot
	d.mu.RLock()
	closed := d.closed
	snap.version, snap.text, snap.path = d.version, d.buf.Text(), d.path
	snap.info, snap.identity, snap.elevated = d.info, d.identity, d.elevated
	if d.ends != nil {
		snap.lines = d.buf.Lines(0, d.buf.LineCount())
		snap.ends = append([]textcodec.EOL(nil), d.ends...)
	}
	d.mu.RUnlock()
	if closed {
		return Fail(CodeUnknownDoc, "")
	}
	if req.MD5 != "" && !strings.EqualFold(req.MD5, plainTextMD5(snap.text)) {
		return Fail(CodeDesync, "")
	}

	target := snap.path
	if req.Path != "" {
		abs, err := filepath.Abs(req.Path)
		if err != nil {
			return Fail(CodeIO, err.Error())
		}
		target = abs
	}
	if target == "" {
		return Fail(CodeIO, "no path")
	}
	saveAs := !samePath(target, snap.path)
	if saveAs && !req.Overwrite {
		if _, err := os.Lstat(target); err == nil {
			return Fail(CodeExists, "")
		}
	}

	info, ends, fail := convert(snap.info, snap.ends, req)
	if fail != nil {
		return *fail
	}
	data, err := encode(snap, info, ends)
	var unencodable *textcodec.UnencodableError
	switch {
	case errors.As(err, &unencodable):
		return Failure{Code: CodeUnencodable, Message: unencodable.Error(), Char: string(unencodable.Rune), Encoding: unencodable.Encoding}
	case err != nil:
		return Fail(CodeIO, err.Error())
	}

	// Save As starts over as the user: a copy in the user's own directory must not end up owned by root.
	elevated := req.Elevated || (snap.elevated && !saveAs)
	var identity string
	if elevated {
		identity, fail = s.writeElevated(target, data, snap.identity, saveAs, req)
	} else {
		identity, fail = s.writeDirect(target, data, snap.identity, saveAs, req)
	}
	if fail != nil {
		if fail.Code == CodeIO {
			// The file may be cut short; the swap keeps a full copy and the UI will not auto-reload over it.
			d.mu.Lock()
			s.touchLocked(d)
			d.mu.Unlock()
		}
		return *fail
	}

	d.mu.Lock()
	d.path = target
	d.title = filepath.Base(target)
	d.info, d.identity, d.reported = info, identity, identity
	if req.EOL != "" {
		d.ends = nil
	}
	d.elevated = elevated
	if saveAs && !elevated {
		d.readElevated = false
	}
	recoverable := !d.readElevated
	d.writable = atomicfile.Writable(target)
	d.dirty = d.version != snap.version
	stillDirty := d.dirty
	if stillDirty {
		s.touchLocked(d)
	} else {
		d.swapStale = false
	}
	oldWatch := d.watchPath
	drain := d.queueLocked(notice{kind: noticeSaved})
	d.mu.Unlock()

	if !stillDirty {
		d.swapMu.Lock()
		s.removeSwapLocked(d)
		d.swapMu.Unlock()
	}
	if saveAs || (oldWatch == "" && !elevated) {
		d.mu.Lock()
		d.watchPath = ""
		d.mu.Unlock()
		s.unwatch(oldWatch)
		s.watchDoc(d)
	}
	if drain {
		s.drain(d)
	}
	return SaveResult{
		OK:          true,
		Path:        target,
		Title:       filepath.Base(target),
		Version:     snap.version,
		Encoding:    info.Encoding,
		BOM:         info.BOM,
		EOL:         info.EOL.String(),
		Elevated:    elevated,
		Recoverable: recoverable,
	}
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	ai, err1 := os.Stat(a)
	bi, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ai, bi)
}

// convert applies the encoding, BOM and line ending a save asks for. An explicit eol rewrites every line.
func convert(info textcodec.Info, ends []textcodec.EOL, req SaveRequest) (textcodec.Info, []textcodec.EOL, *Failure) {
	if req.Encoding != "" {
		enc, err := textcodec.Canonical(req.Encoding)
		if err != nil {
			return info, ends, &Failure{Code: CodeIO, Message: err.Error()}
		}
		if enc != info.Encoding {
			info.Encoding = enc
			// Without a BOM, UTF-16 reopens as binary.
			info.BOM = enc == textcodec.UTF16LE || enc == textcodec.UTF16BE
		}
	}
	if req.BOM != nil {
		info.BOM = *req.BOM
	}
	info.BOM = info.BOM && textcodec.HasBOM(info.Encoding)
	if req.EOL != "" {
		eol, ok := textcodec.ParseEOL(req.EOL)
		if !ok {
			return info, ends, &Failure{Code: CodeIO, Message: "unknown line ending " + req.EOL}
		}
		info.EOL, info.Mixed, ends = eol, false, nil
	}
	return info, ends, nil
}

func encode(snap saveSnapshot, info textcodec.Info, ends []textcodec.EOL) ([]byte, error) {
	if ends != nil {
		return textcodec.EncodeLines(snap.lines, ends, info)
	}
	return textcodec.Encode(snap.text, info)
}

// A file deleted under a plain save is recreated; any other change since load is a conflict.
func (s *Store) writeDirect(target string, data []byte, loaded string, saveAs bool, req SaveRequest) (string, *Failure) {
	current, err := atomicfile.Stat(target)
	if !saveAs && !req.Force && err == nil && current != loaded {
		return "", &Failure{Code: CodeConflict}
	}
	identity, err := atomicfile.Save(target, data, atomicfile.Options{})
	switch {
	case errors.Is(err, os.ErrPermission):
		return "", s.accessFailure(err)
	case err != nil:
		return "", &Failure{Code: CodeIO, Message: err.Error()}
	}
	return identity, nil
}

// The helper checks the file as root, so nothing here depends on what the user may stat.
func (s *Store) writeElevated(target string, data []byte, loaded string, saveAs bool, req SaveRequest) (string, *Failure) {
	if s.elevation() != "available" {
		return "", &Failure{Code: CodeAccess, Elevation: s.elevation()}
	}
	resolved := target
	if r, err := atomicfile.Resolve(target); err == nil {
		resolved = r
	}
	var identity string
	var err error
	switch {
	case saveAs && !req.Overwrite:
		identity, err = s.elevator.Create(resolved, data)
	case saveAs || req.Force:
		identity, err = s.elevator.Replace(resolved, data, "")
		if errors.Is(err, os.ErrNotExist) {
			identity, err = s.elevator.Create(resolved, data)
		}
	case loaded == "":
		// The file did not exist when it was opened; one that appeared since is someone else's.
		identity, err = s.elevator.Create(resolved, data)
		if errors.Is(err, os.ErrExist) {
			err = ErrConflict
		}
	default:
		identity, err = s.elevator.Replace(resolved, data, loaded)
	}
	switch {
	case errors.Is(err, ErrConflict):
		return "", &Failure{Code: CodeConflict}
	case errors.Is(err, os.ErrExist):
		return "", &Failure{Code: CodeExists}
	case errors.Is(err, os.ErrPermission):
		return "", s.accessFailure(err)
	case err != nil:
		return "", &Failure{Code: CodeIO, Message: err.Error()}
	}
	return identity, nil
}

func (s *Store) Reload(docID, encoding string) any {
	d, ok := s.Get(docID)
	if !ok {
		return Fail(CodeUnknownDoc, "")
	}
	d.saveMu.Lock()
	defer d.saveMu.Unlock()

	d.mu.RLock()
	path, elevated, closed := d.path, d.readElevated, d.closed
	d.mu.RUnlock()
	switch {
	case closed:
		return Fail(CodeUnknownDoc, "")
	case path == "":
		return Fail(CodeNotFound, "")
	}
	ld, fail := s.load(path, encoding, elevated, true)
	if fail != nil {
		return *fail
	}

	d.mu.Lock()
	d.buf = buffer.New(ld.doc.Text)
	d.version = 0
	clear(d.pending)
	d.desynced = false
	d.stopGapLocked()
	d.bumpLocked()
	d.info, d.ends = ld.doc.Info, ld.doc.Ends
	d.identity, d.reported, d.writable = ld.identity, ld.identity, ld.writable
	d.dirty, d.swapStale = false, false
	result := OpenResult{
		OK:       true,
		DocID:    d.ID,
		SwapID:   d.swapID,
		Path:     path,
		Title:    d.title,
		Text:     ld.doc.Text,
		Encoding: ld.doc.Encoding,
		BOM:      ld.doc.BOM,
		EOL:      ld.doc.EOL.String(),
		MixedEOL: ld.doc.Mixed,
		Writable: ld.writable,
		Elevated: d.elevated,
		Size:     ld.size,

		Recoverable: !d.readElevated,
	}
	watched := d.watchPath != ""
	drain := d.queueLocked(notice{kind: noticeReset})
	d.mu.Unlock()

	d.swapMu.Lock()
	s.removeSwapLocked(d)
	d.swapMu.Unlock()
	if !watched && !elevated {
		watched = s.watchDoc(d)
	}
	result.Watched = watched
	if drain {
		s.drain(d)
	}
	return result
}
