// Package elevate saves and reads files the user cannot touch, through pkexec and the hidden helper command.
package elevate

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/duxlabs-dev/dux-edit/core/internal/helper"
	"github.com/duxlabs-dev/dux-edit/core/internal/xdg"
)

const (
	PolicyName = "dev.duxlabs.edit.policy"
	ActionID   = "dev.duxlabs.edit.save"
)

const (
	ReasonSandboxed = "SANDBOXED"
	ReasonNoPkexec  = "NOPKEXEC"
	ReasonNoPolicy  = "NOPOLICY"
	ReasonNoBinary  = "NOBINARY"
)

// Matches document.MaxOpenBytes; a larger read stops the helper instead of filling memory.
const maxRead = 32 << 20

const frameSize = 1 << 20

var (
	ErrUnavailable = errors.New("elevation unavailable")
	ErrTooLarge    = errors.New("file too large")
	// ErrConflict comes back from Replace when the file no longer has the given identity.
	ErrConflict = helper.ErrConflict
)

// Error carries a helper result code; errors.Is matches it against fs.ErrExist, fs.ErrPermission,
// fs.ErrNotExist and ErrConflict.
type Error = helper.Error

type Client struct {
	pkexec string
	exe    string
	env    []string
	once   sync.Once
	exeErr error
}

func New() *Client { return &Client{} }

// Available returns "" when an elevated save can be offered, otherwise the reason it cannot.
func (c *Client) Available() string {
	if inFlatpak() {
		return ReasonSandboxed
	}
	if _, err := c.pkexecPath(); err != nil {
		return ReasonNoPkexec
	}
	exe, err := c.executable()
	if err != nil {
		return ReasonNoBinary
	}
	if !policyCovers(exe, actionDirs()) {
		return ReasonNoPolicy
	}
	return ""
}

func inFlatpak() bool {
	if os.Getenv("FLATPAK_ID") != "" {
		return true
	}
	_, err := os.Stat("/.flatpak-info")
	return err == nil
}

func (c *Client) pkexecPath() (string, error) {
	if c.pkexec == "" {
		return exec.LookPath("pkexec")
	}
	if _, err := os.Stat(c.pkexec); err != nil {
		return "", err
	}
	return c.pkexec, nil
}

func (c *Client) executable() (string, error) {
	c.once.Do(func() {
		if c.exe != "" {
			return
		}
		exe, err := os.Executable()
		if err == nil {
			exe, err = filepath.EvalSymlinks(exe)
		}
		c.exe, c.exeErr = exe, err
	})
	return c.exe, c.exeErr
}

func actionDirs() []string {
	dirs := []string{"/usr/share/polkit-1/actions"}
	for _, dir := range xdg.DataDirs() {
		dirs = append(dirs, filepath.Join(dir, "polkit-1", "actions"))
	}
	return dirs
}

func policyCovers(exe string, dirs []string) bool {
	for _, dir := range dirs {
		body, err := os.ReadFile(filepath.Join(dir, PolicyName))
		if err != nil {
			continue
		}
		if annotatedPath(string(body)) == exe {
			return true
		}
	}
	return false
}

func annotatedPath(policy string) string {
	const key = `key="org.freedesktop.policykit.exec.path">`
	_, after, ok := strings.Cut(policy, key)
	if !ok {
		return ""
	}
	value, _, ok := strings.Cut(after, "<")
	if !ok {
		return ""
	}
	value = strings.TrimSpace(value)
	resolved, err := filepath.EvalSymlinks(value)
	if err != nil {
		return value
	}
	return resolved
}

// Read returns the bytes and the identity of a file the user cannot read.
func (c *Client) Read(path string) ([]byte, string, error) {
	cmd, err := c.command(helper.OpRead, resolve(path))
	if err != nil {
		return nil, "", err
	}
	out := &capped{limit: maxRead}
	var stderr bytes.Buffer
	cmd.Stdout = out
	cmd.Stderr = &stderr
	err = cmd.Run()
	switch {
	case out.over:
		return nil, "", ErrTooLarge
	case err != nil:
		return nil, "", failure(err, stderr.Bytes())
	}
	return out.buf.Bytes(), identity(stderr.Bytes()), nil
}

// Replace writes data over path while it still has identity, and returns the new one. An empty identity
// skips the check.
func (c *Client) Replace(path string, data []byte, identity string) (string, error) {
	return c.stream(data, helper.OpReplace, resolve(path), identity)
}

// Create writes a new root-owned file and fails when path exists.
func (c *Client) Create(path string, data []byte) (string, error) {
	return c.stream(data, helper.OpWrite, resolve(path))
}

func (c *Client) stream(data []byte, op string, args ...string) (string, error) {
	cmd, err := c.command(op, args...)
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stdin = framed(data)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", failure(err, stderr.Bytes())
	}
	return identity(stderr.Bytes()), nil
}

func identity(stderr []byte) string {
	var reported helper.Success
	_ = json.Unmarshal(lastLine(stderr), &reported)
	return reported.Identity
}

func (c *Client) command(op string, args ...string) (*exec.Cmd, error) {
	pkexec, err := c.pkexecPath()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, ReasonNoPkexec)
	}
	exe, err := c.executable()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, ReasonNoBinary)
	}
	cmd := exec.Command(pkexec, append([]string{exe, "helper", op}, args...)...)
	if len(c.env) > 0 {
		cmd.Env = append(os.Environ(), c.env...)
	}
	return cmd, nil
}

// The helper refuses every symlink, so a link like /etc/resolv.conf is resolved here, as the user.
func resolve(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	dir, base := filepath.Split(path)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Join(resolved, base)
	}
	return path
}

// The zero-length frame at the end commits; a stream cut short before it changes nothing.
func framed(data []byte) io.Reader {
	parts := make([]io.Reader, 0, 2*(len(data)/frameSize+1)+1)
	for len(data) > 0 {
		n := min(len(data), frameSize)
		parts = append(parts, bytes.NewReader(binary.BigEndian.AppendUint32(nil, uint32(n))), bytes.NewReader(data[:n]))
		data = data[n:]
	}
	return io.MultiReader(append(parts, bytes.NewReader(make([]byte, 4)))...)
}

func failure(err error, stderr []byte) error {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return &Error{Code: helper.CodeIO, Err: err}
	}

	var reported helper.Failure
	if json.Unmarshal(lastLine(stderr), &reported) == nil && reported.Code != "" {
		return &Error{Code: helper.Code(reported.Code), Err: errors.New(reported.Error)}
	}

	switch exit.ExitCode() {
	case 126:
		return &Error{Code: helper.CodeAccess, Err: errors.New("administrator authorization was not granted")}
	case 127:
		if bytes.Contains(stderr, []byte("Not authorized")) {
			return &Error{Code: helper.CodeAccess, Err: errors.New("administrator authorization failed")}
		}
		return &Error{Code: helper.CodeIO, Err: errors.New("pkexec failed: " + firstLine(stderr))}
	}
	return &Error{Code: helper.CodeIO, Err: errors.New("helper exited " + strconv.Itoa(exit.ExitCode()) + ": " + firstLine(stderr))}
}

func lastLine(out []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(out), []byte("\n"))
	return lines[len(lines)-1]
}

func firstLine(out []byte) string {
	line, _, _ := bytes.Cut(bytes.TrimSpace(out), []byte("\n"))
	return string(line)
}

// capped fails the copy past limit, which closes the pipe and stops the helper.
type capped struct {
	buf   bytes.Buffer
	limit int
	over  bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.buf.Len()+len(p) > c.limit {
		c.over = true
		return 0, ErrTooLarge
	}
	return c.buf.Write(p)
}
