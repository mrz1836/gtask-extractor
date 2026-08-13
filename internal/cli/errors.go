package cli

// codedError attaches a process exit code to an error so Execute can map typed
// failures to the documented exit codes.
type codedError struct {
	code int
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

// coded wraps err with an exit code.
func coded(code int, err error) error {
	return &codedError{code: code, err: err}
}
