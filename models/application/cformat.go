package application

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"text/template/parse"

	"github.com/Jeffail/gabs/v2"
)

const MaxFieldOutput = 64 * 1024

var errOutputLimit = fmt.Errorf(
	"rendered output exceeds %d bytes", MaxFieldOutput)

var errTemplateRestricted = errors.New(
	`only text and {{ webhook "path" }} actions are allowed`)

type limitedBuffer struct {
	buf bytes.Buffer
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	if lb.buf.Len()+len(p) > MaxFieldOutput {
		return 0, errOutputLimit
	}
	return lb.buf.Write(p)
}

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

func validateAction(n *parse.ActionNode) error {
	pipe := n.Pipe
	if pipe == nil || len(pipe.Decl) != 0 || len(pipe.Cmds) != 1 {
		return errTemplateRestricted
	}
	cmd := pipe.Cmds[0]
	if len(cmd.Args) != 2 {
		return errTemplateRestricted
	}
	ident, ok := cmd.Args[0].(*parse.IdentifierNode)
	if !ok || ident.Ident != "webhook" {
		return errTemplateRestricted
	}
	if _, ok := cmd.Args[1].(*parse.StringNode); !ok {
		return errTemplateRestricted
	}
	return nil
}

func validateTemplate(tmplstr string) error {
	trees, err := parse.Parse("field", tmplstr, "{{", "}}",
		map[string]any{"webhook": func(string) any { return nil }})
	if err != nil {
		return err
	}
	for _, node := range trees["field"].Root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
		case *parse.ActionNode:
			if err := validateAction(n); err != nil {
				return err
			}
		default:
			return errTemplateRestricted
		}
	}
	return nil
}

func (cf *CFormat) GetValue(
	locations map[string]*gabs.Container,
	tmplstr string,
) (string, bool, error) {
	if tmplstr == "" {
		return "", false, nil
	}

	if err := validateTemplate(tmplstr); err != nil {
		return "", false, err
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
		return "", false, err
	}

	lb := new(limitedBuffer)
	if err := tmpl.Execute(lb, nil); err != nil {
		return "", false, err
	}

	return lb.buf.String(), true, nil
}
