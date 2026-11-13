// Package textcodec detects, decodes and encodes document bytes. Decoded text uses "\n" only.
package textcodec

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
)

const (
	UTF8    = "utf-8"
	UTF16LE = "utf-16le"
	UTF16BE = "utf-16be"
	Latin1  = "iso-8859-1"
)

type EOL uint8

const (
	LF EOL = iota
	CRLF
	CR
)

func (e EOL) String() string {
	switch e {
	case CRLF:
		return "crlf"
	case CR:
		return "cr"
	}
	return "lf"
}

func (e EOL) Bytes() string {
	switch e {
	case CRLF:
		return "\r\n"
	case CR:
		return "\r"
	}
	return "\n"
}

func ParseEOL(name string) (EOL, bool) {
	switch name {
	case "lf":
		return LF, true
	case "crlf":
		return CRLF, true
	case "cr":
		return CR, true
	}
	return LF, false
}

type Info struct {
	Encoding string
	BOM      bool
	EOL      EOL
	Mixed    bool
	Binary   bool
}

var (
	ErrUnknownEncoding = errors.New("unknown encoding")
	ErrInvalid         = errors.New("the file is not valid in this encoding")
)

// UnencodableError names the first character the target encoding cannot hold.
type UnencodableError struct {
	Rune     rune
	Encoding string
}

func (e *UnencodableError) Error() string {
	return fmt.Sprintf("%q (U+%04X) cannot be written as %s", e.Rune, e.Rune, e.Encoding)
}

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
	bomUTF32LE = []byte{0xFF, 0xFE, 0x00, 0x00}
	bomUTF32BE = []byte{0x00, 0x00, 0xFE, 0xFF}
)

// Doc is a decoded file: the "\n" text and, for a file with mixed line endings, the ending after every line.
type Doc struct {
	Info
	Text string
	Ends []EOL
	// Fallback is set when ISO-8859-1 was assumed for invalid UTF-8, so the text may be UTF-8 shown as mojibake.
	// Converting it would double-encode every non-ASCII character.
	Fallback bool
	// Lossy is set when writing the text back gives other bytes, so a save would change untouched parts.
	Lossy bool
}

// Load detects (or, with a named encoding, applies) the encoding and decodes raw.
func Load(raw []byte, encodingName string) (Doc, error) {
	info, err := detectEncoding(raw, encodingName)
	if err != nil {
		return Doc{}, err
	}
	if info.Binary {
		return Doc{Info: info}, nil
	}
	text, lossy, err := decodeRaw(raw, info)
	if err != nil {
		return Doc{}, err
	}
	if strings.IndexByte(text, 0) >= 0 {
		info.Binary = true
		return Doc{Info: info}, nil
	}
	text, ends, counts := splitEOL(text, true)
	info.EOL, info.Mixed = dominant(counts)
	doc := Doc{Info: info, Text: text, Lossy: lossy, Fallback: encodingName == "" && info.Encoding == Latin1}
	if info.Mixed {
		doc.Ends = ends
	}
	return doc, nil
}

func detectEncoding(raw []byte, name string) (Info, error) {
	if name != "" {
		canonical, err := Canonical(name)
		if err != nil {
			return Info{}, err
		}
		bom := bomFor(canonical)
		return Info{Encoding: canonical, BOM: bom != nil && bytes.HasPrefix(raw, bom)}, nil
	}
	switch {
	case bytes.HasPrefix(raw, bomUTF32LE), bytes.HasPrefix(raw, bomUTF32BE):
		return Info{Encoding: UTF8, Binary: true}, nil
	case bytes.HasPrefix(raw, bomUTF8):
		return Info{Encoding: UTF8, BOM: true}, nil
	case bytes.HasPrefix(raw, bomUTF16LE):
		return Info{Encoding: UTF16LE, BOM: true}, nil
	case bytes.HasPrefix(raw, bomUTF16BE):
		return Info{Encoding: UTF16BE, BOM: true}, nil
	case bytes.IndexByte(raw, 0) >= 0:
		return Info{Encoding: UTF8, Binary: true}, nil
	case utf8.Valid(raw):
		return Info{Encoding: UTF8}, nil
	}
	return Info{Encoding: Latin1}, nil
}

// Canonical maps an encoding name to the name Info uses.
func Canonical(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "utf-8", "utf8":
		return UTF8, nil
	case "utf-16", "utf-16le", "utf16le":
		return UTF16LE, nil
	case "utf-16be", "utf16be":
		return UTF16BE, nil
	// WHATWG maps these labels to windows-1252, which does not round-trip every byte.
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1", "l1":
		return Latin1, nil
	}
	enc, err := htmlindex.Get(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnknownEncoding, name)
	}
	canonical, err := htmlindex.Name(enc)
	if err != nil || canonical == "replacement" {
		return "", fmt.Errorf("%w: %s", ErrUnknownEncoding, name)
	}
	return canonical, nil
}

// Unicode encodings can carry a BOM; the rest never do.
func bomFor(encodingName string) []byte {
	switch encodingName {
	case UTF8:
		return bomUTF8
	case UTF16LE:
		return bomUTF16LE
	case UTF16BE:
		return bomUTF16BE
	}
	return nil
}

func HasBOM(encodingName string) bool { return bomFor(encodingName) != nil }

func decodeRaw(raw []byte, info Info) (string, bool, error) {
	if bom := bomFor(info.Encoding); info.BOM && bytes.HasPrefix(raw, bom) {
		raw = raw[len(bom):]
	}
	switch info.Encoding {
	case UTF8:
		if !utf8.Valid(raw) {
			return "", false, ErrInvalid
		}
		return string(raw), false, nil
	case UTF16LE, UTF16BE:
		text, err := decodeUTF16(raw, info.Encoding == UTF16BE)
		return text, false, err
	case Latin1:
		return decodeLatin1(raw), false, nil
	}
	enc, err := named(info.Encoding)
	if err != nil {
		return "", false, err
	}
	out, err := enc.NewDecoder().Bytes(raw)
	if err != nil {
		return "", false, ErrInvalid
	}
	back, err := enc.NewEncoder().Bytes(out)
	switch {
	case err == nil && bytes.Equal(back, raw):
		return string(out), false, nil
	// These decoders turn bytes the encoding does not define into U+FFFD without an error.
	case bytes.ContainsRune(out, utf8.RuneError):
		return "", false, ErrInvalid
	}
	return string(out), true, nil
}

func decodeLatin1(raw []byte) string {
	high := 0
	for _, b := range raw {
		if b >= utf8.RuneSelf {
			high++
		}
	}
	var sb strings.Builder
	sb.Grow(len(raw) + high)
	for _, b := range raw {
		if b < utf8.RuneSelf {
			sb.WriteByte(b)
			continue
		}
		sb.WriteRune(rune(b))
	}
	return sb.String()
}

// Lone surrogates cannot live in a Go string, so a file with one is refused rather than silently changed.
func decodeUTF16(raw []byte, bigEndian bool) (string, error) {
	if len(raw)%2 != 0 {
		return "", ErrInvalid
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		lo, hi := raw[2*i], raw[2*i+1]
		if bigEndian {
			lo, hi = hi, lo
		}
		units[i] = uint16(lo) | uint16(hi)<<8
	}
	var sb strings.Builder
	sb.Grow(len(units))
	for i := 0; i < len(units); i++ {
		u := units[i]
		switch {
		case utf16.IsSurrogate(rune(u)):
			if i+1 >= len(units) {
				return "", ErrInvalid
			}
			r := utf16.DecodeRune(rune(u), rune(units[i+1]))
			if r == utf8.RuneError {
				return "", ErrInvalid
			}
			sb.WriteRune(r)
			i++
		default:
			sb.WriteRune(rune(u))
		}
	}
	return sb.String(), nil
}

func named(name string) (encoding.Encoding, error) {
	if name == Latin1 {
		return charmap.ISO8859_1, nil
	}
	enc, err := htmlindex.Get(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEncoding, name)
	}
	return enc, nil
}

func splitEOL(text string, keepEnds bool) (string, []EOL, [3]int) {
	var counts [3]int
	if !strings.ContainsRune(text, '\r') {
		counts[LF] = strings.Count(text, "\n")
		return text, nil, counts
	}
	var sb strings.Builder
	sb.Grow(len(text))
	var ends []EOL
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c != '\r' && c != '\n' {
			sb.WriteByte(c)
			continue
		}
		kind := LF
		switch {
		case c == '\r' && i+1 < len(text) && text[i+1] == '\n':
			kind = CRLF
			i++
		case c == '\r':
			kind = CR
		}
		counts[kind]++
		if keepEnds {
			ends = append(ends, kind)
		}
		sb.WriteByte('\n')
	}
	return sb.String(), ends, counts
}

// Ties go to LF, then CRLF.
func dominant(counts [3]int) (EOL, bool) {
	best, kinds := LF, 0
	for _, kind := range []EOL{LF, CRLF, CR} {
		if counts[kind] == 0 {
			continue
		}
		kinds++
		if counts[kind] > counts[best] {
			best = kind
		}
	}
	return best, kinds > 1
}

// Encode converts "\n" to info.EOL and encodes text, with a BOM when info asks for one.
func Encode(text string, info Info) ([]byte, error) {
	if info.EOL != LF {
		text = strings.ReplaceAll(text, "\n", info.EOL.Bytes())
	}
	return EncodeExact(text, info)
}

// EncodeLines writes every line followed by its own ending; ends has one entry per line break.
func EncodeLines(lines []string, ends []EOL, info Info) ([]byte, error) {
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(line)
		if i == len(lines)-1 {
			break
		}
		end := info.EOL
		if i < len(ends) {
			end = ends[i]
		}
		sb.WriteString(end.Bytes())
	}
	return EncodeExact(sb.String(), info)
}

// EncodeExact encodes text without touching its line endings.
func EncodeExact(text string, info Info) ([]byte, error) {
	var out []byte
	if info.BOM {
		out = append(out, bomFor(info.Encoding)...)
	}
	switch info.Encoding {
	case UTF8:
		return append(out, text...), nil
	case UTF16LE, UTF16BE:
		out = slices.Grow(out, 2*len(text))
		for _, u := range utf16.Encode([]rune(text)) {
			if info.Encoding == UTF16BE {
				out = append(out, byte(u>>8), byte(u))
				continue
			}
			out = append(out, byte(u), byte(u>>8))
		}
		return out, nil
	}
	enc, err := named(info.Encoding)
	if err != nil {
		return nil, err
	}
	encoded, err := enc.NewEncoder().Bytes([]byte(text))
	if err != nil {
		return nil, unencodable(enc, text, info.Encoding)
	}
	return append(out, encoded...), nil
}

// Only runs after a failed encode, to find the character to name.
func unencodable(enc encoding.Encoding, text, name string) error {
	for _, r := range text {
		if _, err := enc.NewEncoder().String(string(r)); err != nil {
			return &UnencodableError{Rune: r, Encoding: name}
		}
	}
	return &UnencodableError{Rune: utf8.RuneError, Encoding: name}
}
