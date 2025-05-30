package helpers

import (
	"errors"
	"fmt"
)

type Errors map[string]error

func ErrorsToError(errs Errors) error {
	if len(errs) == 0 {
		return nil
	}

	var errstr = ""

	for key, err := range errs {
		fmt.Sprintf("%s[%s] \n", errstr, key, err.Error())
	}
	return errors.New(errstr)
}
