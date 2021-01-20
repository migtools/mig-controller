package errorutil

import "errors"

func Unwrap(err error) error {
	if err1 := errors.Unwrap(err); err1 != nil {
		return err1
	}
	return err
}
