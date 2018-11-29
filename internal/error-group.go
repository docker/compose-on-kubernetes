package internal

import (
	"fmt"
)

// ErrorGroup is a type of error that aggregates several errors.
type ErrorGroup struct {
	errors []error
}

func (err ErrorGroup) Error() string {
	res := "multiple errors: "
	for i, e := range err.errors {
		if i != 0 {
			res += ", "
		}
		res += fmt.Sprintf("%q", e)
	}
	return res
}

// GroupErrors returns an aggregation of several errors applying
// simplifications: ignore nils, avoid groups of one, flatten
// subgroups of errors, etc.
func GroupErrors(errs ...error) error {
	// Collect all (sub) errors.
	errors := []error{}
	for _, e := range errs {
		if e == nil {
			continue
		}
		if g, ok := e.(ErrorGroup); ok {
			errors = append(errors, g.errors...)
		} else {
			errors = append(errors, e)
		}
	}
	switch len(errors) {
	case 0:
		return nil
	case 1:
		return errors[0]
	default:
		return ErrorGroup{errors}
	}
}
