package tui

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestPrettyJSONMatchesIndentStructure(t *testing.T) {
	cases := []string{
		`{"b":2,"a":[1,2,{"x":true}],"s":"hi","n":null}`,
		`[1,2,3]`,
		`"a string with : and \" escaped quote"`,
		`{"url":"http://example.com:8080/path"}`,
		`42.5`,
		`true`,
	}
	for _, raw := range cases {
		got, ok := prettyJSON(raw)
		if !ok {
			t.Fatalf("valid JSON not recognized: %s", raw)
		}
		var want bytes.Buffer
		if err := json.Indent(&want, []byte(raw), "", "  "); err != nil {
			t.Fatalf("json.Indent: %v", err)
		}
		if stripANSI(got) != want.String() {
			t.Fatalf("structure mismatch for %s:\n got %q\nwant %q", raw, stripANSI(got), want.String())
		}
	}
}

func TestPrettyJSONRejectsNonJSON(t *testing.T) {
	for _, raw := range []string{"hello world", "", "{not json", "12 34"} {
		if _, ok := prettyJSON(raw); ok {
			t.Fatalf("non-JSON accepted: %q", raw)
		}
	}
}

func TestPrettyJSONKeepsContent(t *testing.T) {
	got, ok := prettyJSON(`{"name":"add","value":42}`)
	if !ok {
		t.Fatal("valid JSON should pretty-print")
	}
	for _, want := range []string{"name", "add", "value", "42"} {
		if !strings.Contains(stripANSI(got), want) {
			t.Fatalf("pretty output dropped %q:\n%s", want, stripANSI(got))
		}
	}
	if !strings.Contains(got, "\n") {
		t.Fatal("pretty output should be multi-line")
	}
}
