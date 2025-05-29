package application

import (
	"bytes"
	"html/template"
	"reflect"
	"strings"

	"github.com/Jeffail/gabs/v2"
)

type CFormat struct {
	Attachment       string
	AttachmentBase64 string
	AttachmentType   string
	Device           string
	HTML             string
	Message          string
	Priority         string
	TTL              string
	Timestamp        string
	Title            string
	URL              string
	URLTitle         string
}

func (cf *CFormat) GetLocationAndPath(str string) (string, string) {
	loc, path, found := strings.Cut(str, ".")
	if !found {
		return "body", str
	}
	return loc, path
}

func (cf *CFormat) GetValue(
	locations map[string]*gabs.Container,
	tmplstr string,
) (string, bool) {
	if tmplstr == "" {
		return "", false
	}

	funcs := template.FuncMap{
		"webhook": func(fullpath string) any {
			loc, path := cf.GetLocationAndPath(fullpath)

			location, ok := locations[loc]
			if !ok {
				return ""
			}

			locctr := location.Path(path)
			if locctr == nil {
				return ""
			}

			locctrData := locctr.Data()
			if locctrData == nil {
				return ""
			}
			locctrType := reflect.TypeOf(locctrData).Kind()
			switch locctrType {
			case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
				if reflect.ValueOf(locctrData).IsNil() {
					return ""
				}
			}

			if locctrType == reflect.String {
				return locctrData.(string)
			}

			return locctr.String()
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

type Application struct {
	Token        string
	Name         string
	IconPath     string
	Format       string
	CustomFormat CFormat

	Target string
}
