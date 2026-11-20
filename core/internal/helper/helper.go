// Package helper is the root side of an elevated read or save, run by pkexec as `dux-edit helper <op> ...`.
package helper

import (
	"io"

	"golang.org/x/sys/unix"

	"github.com/duxlabs-dev/dux-go/atomicfile"
	dhelper "github.com/duxlabs-dev/dux-go/elevate/helper"
)

const (
	OpRead    = "read"
	OpWrite   = "write"
	OpReplace = "replace"
)

// Success carries the identity of the file an operation read or wrote, which the caller may be unable to stat.
type Success struct {
	Identity string `json:"identity"`
}

// UnderPkexec runs "dux-edit helper <op> ..." and exits when pkexec started this process.
func UnderPkexec() {
	dhelper.UnderPkexec("dux-edit", Run)
}

func Run(args []string, stdin io.Reader, stdout io.Writer) (any, error) {
	if len(args) == 0 {
		return nil, dhelper.Invalid("missing operation")
	}
	op, rest := args[0], args[1:]

	switch op {
	case OpRead:
		if len(rest) != 1 {
			return nil, dhelper.Invalid("read takes one path")
		}
		return success(dhelper.Read(rest[0], stdout))
	case OpWrite:
		if len(rest) != 1 {
			return nil, dhelper.Invalid("write takes one path")
		}
		return success(dhelper.Create(rest[0], 0o644, true, stdin, stdout))
	case OpReplace:
		if len(rest) != 2 {
			return nil, dhelper.Invalid("replace takes a path and an identity")
		}
		identity, err := replace(rest[0], rest[1], stdin, stdout)
		if err != nil {
			return nil, err
		}
		return Success{Identity: identity}, nil
	default:
		return nil, dhelper.Invalid("unknown operation: " + op)
	}
}

func success(st unix.Stat_t, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return Success{Identity: atomicfile.StatIdentity(&st)}, nil
}
