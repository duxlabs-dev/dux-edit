package indent

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

const maxConfigBytes = 64 << 10

var errSkip = errors.New("not a small regular file")

// boundedParser reads only small regular files, so a FIFO or huge .editorconfig cannot stall an open.
// Anything unreadable or malformed counts as absent.
type boundedParser struct{}

func (p boundedParser) ParseIni(filename string) (*editorconfig.Editorconfig, error) {
	ec, _, err := p.ParseIniGraceful(filename)
	return ec, err
}

func (boundedParser) ParseIniGraceful(filename string) (*editorconfig.Editorconfig, error, error) {
	data, err := readSmall(filename)
	if err != nil {
		return nil, nil, fs.ErrNotExist
	}
	ec, _, err := editorconfig.ParseGraceful(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fs.ErrNotExist
	}
	return ec, nil, nil
}

func (boundedParser) FnmatchCase(pattern, name string) (bool, error) {
	return editorconfig.FnmatchCase(pattern, name)
}

func readSmall(name string) ([]byte, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxConfigBytes {
		return nil, errSkip
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxConfigBytes))
}

func editorConfig(path string) settings {
	cfg := editorconfig.Config{Parser: boundedParser{}}
	def, _, err := cfg.LoadGraceful(path)
	if err != nil || def == nil {
		return settings{}
	}
	var s settings
	switch def.IndentStyle {
	case Space, Tab:
		s.style = def.IndentStyle
	}
	s.indent = width(def.IndentSize)
	// tab_width defaults to indent_size; Raw keeps an "unset" that the library's TabWidth would fill from a parent section.
	tabWidth, set := def.Raw["tab_width"]
	if !set {
		tabWidth = def.IndentSize
	}
	s.tab = width(tabWidth)
	return s
}
