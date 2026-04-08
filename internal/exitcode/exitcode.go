package exitcode

import "errors"

// BSD sysexits-compatible exit codes for structured error reporting.
const (
	Usage       = 64 // command-line usage error (bad flags, missing args)
	Auth        = 65 // authentication/authorization error
	Unavailable = 69 // network/service unavailable
)

// Error wraps an error with a specific exit code.
type Error struct {
	Err  error
	Code int
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Wrap returns an error that carries the given exit code.
// If err is nil, returns nil.
func Wrap(err error, code int) error {
	if err == nil {
		return nil
	}
	return &Error{Err: err, Code: code}
}

// Of extracts the exit code from an error chain.
// Returns 1 (generic failure) if no exit code is found, or 0 if err is nil.
func Of(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *Error
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return 1
}
