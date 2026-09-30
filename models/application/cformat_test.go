package application

import (
	"strings"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

func testLocations(t *testing.T, body map[string]interface{}) map[string]*gabs.Container {
	t.Helper()
	return map[string]*gabs.Container{"body": gabs.Wrap(body)}
}

func TestGetValueAllowedTemplates(t *testing.T) {
	cf := CFormat{}
	locations := testLocations(t, map[string]interface{}{
		"title": "Server Down",
		"alerts": []interface{}{
			map[string]interface{}{"message": "not responding"},
		},
		"count": 42.0,
	})

	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{"plain text", "hello", "hello"},
		{"single call", `{{ webhook "body.title" }}`, "Server Down"},
		{"nested array path", `{{ webhook "body.alerts.0.message" }}`, "not responding"},
		{"text and call", `CrowdSec: {{ webhook "body.title" }}!`, "CrowdSec: Server Down!"},
		{"two calls", `{{ webhook "body.title" }} {{ webhook "body.count" }}`, "Server Down 42"},
		{"missing path", `{{ webhook "body.nope" }}`, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found, err := cf.GetValue(locations, tc.tmpl)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !found {
				t.Fatalf("expected found")
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetValueEmptyTemplate(t *testing.T) {
	cf := CFormat{}
	got, found, err := cf.GetValue(testLocations(t, nil), "")
	if err != nil || found || got != "" {
		t.Fatalf("got %q found=%v err=%v, want empty/false/nil", got, found, err)
	}
}

func TestGetValueRejectedTemplates(t *testing.T) {
	cf := CFormat{}
	locations := testLocations(t, map[string]interface{}{"x": "y"})

	cases := []struct {
		name string
		tmpl string
	}{
		{"range", `{{ range 3000000 }}AAAA{{ end }}`},
		{"range empty body", `{{ range 1000000000 }}{{ end }}`},
		{"if", `{{ if true }}x{{ end }}`},
		{"with", `{{ with "a" }}x{{ end }}`},
		{"template", `{{ template "x" }}`},
		{"variable", `{{ $x := 1 }}`},
		{"pipeline", `{{ webhook "body.x" | printf "%s" }}`},
		{"builtin", `{{ printf "x" }}`},
		{"field access", `{{ .Field }}`},
		{"non-literal argument", `{{ webhook .Field }}`},
		{"extra argument", `{{ webhook "body.x" "body.y" }}`},
		{"no argument", `{{ webhook }}`},
		{"unknown function", `{{ nosuchfunc "x" }}`},
		{"unparseable", `{{ webhook "body.x" `},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found, err := cf.GetValue(locations, tc.tmpl)
			if err == nil {
				t.Fatalf("expected error, got %q found=%v", got, found)
			}
		})
	}
}

func TestGetValueOutputLimit(t *testing.T) {
	cf := CFormat{}

	under := strings.Repeat("a", MaxFieldOutput-16)
	got, found, err := cf.GetValue(
		testLocations(t, map[string]interface{}{"big": under}),
		`{{ webhook "body.big" }}`)
	if err != nil || !found || got != under {
		t.Fatalf("under limit: len=%d found=%v err=%v", len(got), found, err)
	}

	over := strings.Repeat("a", MaxFieldOutput+1)
	_, _, err = cf.GetValue(
		testLocations(t, map[string]interface{}{"big": over}),
		`{{ webhook "body.big" }}`)
	if err == nil {
		t.Fatal("over limit: expected error")
	}

	overText := strings.Repeat("b", MaxFieldOutput+1)
	_, _, err = cf.GetValue(testLocations(t, nil), overText)
	if err == nil {
		t.Fatal("over limit text: expected error")
	}
}

func TestGetValueKeepsEscaping(t *testing.T) {
	cf := CFormat{}
	got, _, err := cf.GetValue(
		testLocations(t, map[string]interface{}{"msg": "a&b"}),
		`{{ webhook "body.msg" }}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a&amp;b" {
		t.Fatalf("got %q, want current html escaping preserved", got)
	}
}
