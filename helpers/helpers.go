package helpers

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
)

type Errors map[string]error

func ErrorsToError(errs Errors) error {
	if len(errs) == 0 {
		return nil
	}

	errstr := ""

	for key, err := range errs {
		fmt.Sprintf("%s[%s] \n", errstr, key, err.Error())
	}
	return errors.New(errstr)
}

func GetFieldValue(tmplstr string, args map[string]interface{}) (string, bool) {
	funcs := template.FuncMap{
		"arg": func(arg string) any {
			val, ok := args[arg].(string)
			if !ok {
				return ""
			}

			return val
		},
	}

	tmpl, err := template.New("field").Funcs(funcs).Parse(tmplstr)
	if err != nil {
		return "", false
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		return "", false
	}

	return buf.String(), true
}
