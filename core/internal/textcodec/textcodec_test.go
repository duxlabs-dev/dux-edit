package textcodec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectAndRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		info Info
		text string
	}{
		{"ascii", []byte("a\nb\n"), Info{Encoding: UTF8, EOL: LF}, "a\nb\n"},
		{"utf-8 bom", []byte("\xEF\xBB\xBFé\r\n"), Info{Encoding: UTF8, BOM: true, EOL: CRLF}, "é\n"},
		{"utf-16le bom", []byte{0xFF, 0xFE, 'h', 0, '\r', 0, '\n', 0, 0x3D, 0xD8, 0x00, 0xDE}, Info{Encoding: UTF16LE, BOM: true, EOL: CRLF}, "h\n😀"},
		{"utf-16be bom", []byte{0xFE, 0xFF, 0, 'h', 0, '\r'}, Info{Encoding: UTF16BE, BOM: true, EOL: CR}, "h\n"},
		{"latin1 fallback", []byte("caf\xE9\x81\xFF\n"), Info{Encoding: Latin1, EOL: LF}, "café\u0081ÿ\n"},
		{"mixed", []byte("a\r\nb\r\nc\nd\re"), Info{Encoding: UTF8, EOL: CRLF, Mixed: true}, "a\nb\nc\nd\ne"},
		{"no trailing newline", []byte("x"), Info{Encoding: UTF8, EOL: LF}, "x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc, err := Load(c.raw, "")
			require.NoError(t, err)
			assert.Equal(t, c.info, doc.Info)
			assert.Equal(t, c.text, doc.Text)

			var out []byte
			if doc.Mixed {
				out, err = EncodeLines(strings.Split(doc.Text, "\n"), doc.Ends, doc.Info)
			} else {
				out, err = Encode(doc.Text, doc.Info)
			}
			require.NoError(t, err)
			assert.Equal(t, c.raw, out)
		})
	}
}

func TestLoadRefusesBinaryAndUTF32(t *testing.T) {
	binary := func(raw []byte) bool {
		doc, err := Load(raw, "")
		require.NoError(t, err)
		return doc.Binary
	}
	assert.True(t, binary([]byte("ab\x00cd")))
	assert.True(t, binary([]byte{0xFF, 0xFE, 0, 0, 'a', 0, 0, 0}))
	assert.False(t, binary([]byte{0xFF, 0xFE, 'a', 0}))
}

func TestNamedEncodings(t *testing.T) {
	doc, err := Load([]byte("\x80"), "windows-1252")
	require.NoError(t, err)
	assert.Equal(t, "windows-1252", doc.Encoding)
	assert.Equal(t, "€", doc.Text)

	doc, err = Load([]byte("\xE9"), "latin1")
	require.NoError(t, err)
	assert.Equal(t, Latin1, doc.Encoding, "latin1 must not become windows-1252")

	_, err = Load([]byte("x"), "no-such-charset")
	assert.ErrorIs(t, err, ErrUnknownEncoding)

	_, err = Load([]byte("\xE9"), "utf-8")
	assert.ErrorIs(t, err, ErrInvalid)

	for name, raw := range map[string]string{"shift_jis": "a\x81", "gbk": "a\xFF", "euc-kr": "a\xFF\xFF"} {
		_, err = Load([]byte(raw), name)
		assert.ErrorIs(t, err, ErrInvalid, name)
	}
}

func TestEncodeNamesTheUnencodableRune(t *testing.T) {
	_, err := Encode("abc€d", Info{Encoding: Latin1})
	var unencodable *UnencodableError
	require.ErrorAs(t, err, &unencodable)
	assert.Equal(t, '€', unencodable.Rune)
}
