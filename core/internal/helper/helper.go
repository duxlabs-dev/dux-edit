// Package helper is the root side of an elevated read or save, run by pkexec as `dux-edit helper <op> ...`.
package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/syslog"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	OpRead    = "read"
	OpWrite   = "write"
	OpReplace = "replace"
)

// Ready is the line a write or replace prints on stdout once it is authorized and the file checks out.
const Ready = "ready"

const (
	ExitFailed  = 1
	ExitRefused = 2
)

type Code string

const (
	CodeInvalid  Code = "INVALID"
	CodeConflict Code = "CONFLICT"
	CodeExists   Code = "EXISTS"
	CodeNotFound Code = "NOTFOUND"
	CodeAccess   Code = "EACCES"
	CodeIO       Code = "EIO"
)

// ErrConflict means the file no longer has the identity the save was based on.
var ErrConflict = errors.New("the file changed since it was loaded")

type Error struct {
	Code Code
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }

func (e *Error) Unwrap() error { return e.Err }

// Is lets callers match a code that crossed the process boundary with the usual sentinels.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrConflict:
		return e.Code == CodeConflict
	case fs.ErrExist:
		return e.Code == CodeExists
	case fs.ErrNotExist:
		return e.Code == CodeNotFound
	case fs.ErrPermission:
		return e.Code == CodeAccess
	}
	return false
}

func CodeOf(err error) Code {
	var coded *Error
	switch {
	case errors.As(err, &coded):
		return coded.Code
	case errors.Is(err, fs.ErrExist):
		return CodeExists
	case errors.Is(err, fs.ErrNotExist):
		return CodeNotFound
	case errors.Is(err, fs.ErrPermission):
		return CodeAccess
	}
	return CodeIO
}

// Failure is the last stderr line of a failed helper run.
type Failure struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

// Success is the last stderr line of a helper run that worked; stdout carries only file bytes or Ready.
type Success struct {
	Identity string `json:"identity"`
}

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	callerUID, err := strconv.Atoi(os.Getenv("PKEXEC_UID"))
	if err != nil {
		report(stderr, invalid("dux-edit helper only runs under pkexec"))
		return ExitRefused
	}
	if os.Getuid() != 0 || os.Geteuid() != 0 {
		report(stderr, invalid("dux-edit helper must be started as root"))
		return ExitRefused
	}
	if err := confine(callerUID); err != nil {
		report(stderr, err)
		return ExitRefused
	}
	unix.Umask(0)

	identity, err := Run(args, stdin, stdout)
	journal(callerUID, args, err)
	if err != nil {
		report(stderr, err)
		return ExitFailed
	}
	line, _ := json.Marshal(Success{Identity: identity})
	fmt.Fprintln(stderr, string(line))
	return 0
}

// Run returns the identity of the file it read or wrote.
func Run(args []string, stdin io.Reader, stdout io.Writer) (string, error) {
	if len(args) == 0 {
		return "", invalid("missing operation")
	}
	op, rest := args[0], args[1:]

	switch op {
	case OpRead:
		if len(rest) != 1 {
			return "", invalid("read takes one path")
		}
		return read(rest[0], stdout)
	case OpWrite:
		if len(rest) != 1 {
			return "", invalid("write takes one path")
		}
		return create(rest[0], stdin, stdout)
	case OpReplace:
		if len(rest) != 2 {
			return "", invalid("replace takes a path and an identity")
		}
		return replace(rest[0], rest[1], stdin, stdout)
	default:
		return "", invalid("unknown operation: " + op)
	}
}

func invalid(msg string) error {
	return &Error{Code: CodeInvalid, Err: errors.New(msg)}
}

func report(stderr io.Writer, err error) {
	line, _ := json.Marshal(Failure{Code: string(CodeOf(err)), Error: err.Error()})
	fmt.Fprintln(stderr, string(line))
}

func journal(uid int, args []string, err error) {
	writer, openErr := syslog.New(syslog.LOG_AUTHPRIV|syslog.LOG_INFO, "dux-edit-helper")
	if openErr != nil {
		return
	}
	defer writer.Close()
	outcome := "ok"
	if err != nil {
		outcome = "failed: " + err.Error()
	}
	_ = writer.Info(fmt.Sprintf("uid=%d %s %s", uid, strings.Join(args, " "), outcome))
}
