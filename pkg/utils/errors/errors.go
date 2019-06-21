package errors

import (
	"fmt"
	"io"
)

type withSuppressed struct {
	cause      error
	suppressed error
}

func (w *withSuppressed) Error() string {
	return fmt.Sprintf("%s, suppressed: %s", w.cause.Error(), w.suppressed.Error())
}

func (w *withSuppressed) Cause() error {
	return w.cause
}

func (w *withSuppressed) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "%+v\nsuppressed: %+v", w.Cause(), w.suppressed)
			return
		}
		fallthrough
	case 's', 'q':
		_, _ = io.WriteString(s, w.Error())
	}
}

func (w *withSuppressed) Suppressed() error {
	return w.suppressed
}

// Suppressed retrieves the suppressed error of the given error, if any.
// An error has a suppressed error if it implements the following interface:
//
//     type suppressor interface {
//            Suppressed() error
//     }
// If the error does not implement the interface, nil is returned.
func Suppressed(err error) error {
	type suppressor interface {
		Suppressed() error
	}
	if w, ok := err.(suppressor); ok {
		return w.Suppressed()
	}
	return nil
}

// WithSuppressed annotates err with a suppressed error.
// If err is nil, WithSuppressed returns nil.
// If suppressed is nil, WithSuppressed returns err.
func WithSuppressed(err, suppressed error) error {
	if err == nil || suppressed == nil {
		return err
	}

	return &withSuppressed{
		cause:      err,
		suppressed: suppressed,
	}
}
