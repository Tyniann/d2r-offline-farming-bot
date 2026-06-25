package process

import "errors"

var (
	// ErrNotFound indicates the target process was not found.
	ErrNotFound = errors.New("process not found")

	// ErrMultipleInstances indicates more than one matching process was found.
	ErrMultipleInstances = errors.New("multiple instances")

	// ErrAccessDenied indicates OpenProcess failed due to insufficient privileges.
	ErrAccessDenied = errors.New("access denied")

	// ErrModuleNotFound indicates the target module could not be resolved.
	ErrModuleNotFound = errors.New("module not found")

	// ErrAlreadyAttached indicates Attach was called while already attached.
	ErrAlreadyAttached = errors.New("already attached")

	// ErrNotAttached indicates ReadAt was called while not in the attached state.
	ErrNotAttached = errors.New("not attached")

	// ErrPartialRead indicates the OS read returned fewer bytes than requested.
	ErrPartialRead = errors.New("partial read")

	// ErrInvalidRead indicates an invalid read request at the process layer (e.g. address 0 or empty buffer).
	ErrInvalidRead = errors.New("invalid read")

	// ErrReadFailed indicates a presumably transient or unexpected OS read failure.
	ErrReadFailed = errors.New("read failed")
)

// IsRetryable reports whether an attach error should be retried in the app wait loop.
func IsRetryable(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsFatal reports whether an attach error is operator-actionable and should be logged once at error level.
func IsFatal(err error) bool {
	return errors.Is(err, ErrMultipleInstances) ||
		errors.Is(err, ErrAccessDenied) ||
		errors.Is(err, ErrModuleNotFound)
}

// IsReadRetryable reports whether [memory.Reader] should retry a failed read.
// Only [ErrReadFailed] without structural sentinel errors in the chain is retryable.
func IsReadRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotAttached) ||
		errors.Is(err, ErrPartialRead) ||
		errors.Is(err, ErrInvalidRead) {
		return false
	}
	return errors.Is(err, ErrReadFailed)
}
