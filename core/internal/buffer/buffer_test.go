package buffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceCountsUTF16Units(t *testing.T) {
	b := New("a😀b\nsecond")

	_, err := b.Replace(2, 1, "x")
	assert.ErrorIs(t, err, ErrRange, "offset 2 is inside the surrogate pair")

	change, err := b.Replace(3, 2, "Z\nnew\n")
	require.NoError(t, err)
	assert.Equal(t, Change{StartLine: 0, OldLines: 2, NewLines: 3}, change)
	assert.Equal(t, "a😀Z\nnew\nsecond", b.Text())
	assert.Equal(t, Len16(b.Text()), b.Len16())
	assert.Equal(t, 5, b.Offset(1))
}

func TestReplaceRejectsOutOfRange(t *testing.T) {
	b := New("abc")
	_, err := b.Replace(2, 5, "")
	assert.ErrorIs(t, err, ErrRange)
	assert.Equal(t, "abc", b.Text())
}
