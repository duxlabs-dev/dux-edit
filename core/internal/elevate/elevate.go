// Package elevate saves and reads files the user cannot touch, through pkexec and the hidden helper command.
package elevate

import (
	"errors"
	"io"
	"path/filepath"

	"github.com/duxlabs-dev/dux-go/atomicfile"
	"github.com/duxlabs-dev/dux-go/elevate"

	"github.com/duxlabs-dev/dux-edit/core/internal/helper"
)

const PolicyName = "dev.duxlabs.edit.policy"

// Matches document.MaxOpenBytes; a larger read stops the helper instead of filling memory.
const maxRead = 32 << 20

var ErrTooLarge = errors.New("file too large")

type Client struct {
	helper *elevate.Client
}

func New() *Client { return &Client{helper: &elevate.Client{Policy: PolicyName}} }

// Available returns "" when an elevated save can be offered, otherwise the reason it cannot.
func (c *Client) Available() string { return c.helper.Available() }

// Read returns the bytes and the identity of a file the user cannot read.
func (c *Client) Read(path string) ([]byte, string, error) {
	r, err := c.helper.Reader(helper.OpRead, resolve(path))
	if err != nil {
		return nil, "", err
	}
	data, err := io.ReadAll(io.LimitReader(r, maxRead+1))
	if len(data) > maxRead {
		_ = r.Close()
		return nil, "", ErrTooLarge
	}
	if closeErr := r.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, "", err
	}
	var done helper.Success
	_ = r.Result(&done)
	return data, done.Identity, nil
}

// Replace fails unless path still has identity; an empty one skips the check.
func (c *Client) Replace(path string, data []byte, identity string) (string, error) {
	return c.write(data, helper.OpReplace, resolve(path), identity)
}

// Create writes a new root-owned file and fails when path exists.
func (c *Client) Create(path string, data []byte) (string, error) {
	return c.write(data, helper.OpWrite, resolve(path))
}

func (c *Client) write(data []byte, op string, args ...string) (string, error) {
	w, err := c.helper.Writer(op, args...)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	var done helper.Success
	_ = w.Result(&done)
	return done.Identity, nil
}

// The helper refuses every symlink, so a link like /etc/resolv.conf is resolved here, as the user.
func resolve(path string) string {
	if resolved, err := atomicfile.Resolve(path); err == nil {
		return resolved
	}
	// The user may not see into the file's directory; a link left in the path is refused by the helper.
	if dir, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		return filepath.Join(dir, filepath.Base(path))
	}
	return filepath.Clean(path)
}
