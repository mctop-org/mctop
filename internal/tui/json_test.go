package tui

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func indented(t *testing.T, raw string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(raw), "", "  "); err != nil {
		t.Fatalf("json.Indent(%s): %v", raw, err)
	}
	return buf.String()
}

func TestDecodeOrderedPreservesKeyOrder(t *testing.T) {
	v, err := decodeOrdered(`{"z":1,"a":2,"m":3}`)
	if err != nil {
		t.Fatal(err)
	}
	o, ok := v.(*object)
	if !ok {
		t.Fatalf("want *object, got %T", v)
	}
	if want := []string{"z", "a", "m"}; !reflect.DeepEqual(o.keys, want) {
		t.Fatalf("key order = %v, want %v", o.keys, want)
	}
}

func TestPrettyJSONArrayOfObjectsIsTable(t *testing.T) {
	raw := `[{"name":"openai","models":3,"enabled":true},{"name":"anthropic","models":5,"enabled":false}]`
	got, ok := prettyJSON(raw, 80)
	if !ok {
		t.Fatal("array of objects should pretty-print")
	}
	plain := stripANSI(got)
	for _, want := range []string{"#", "name", "models", "enabled", "openai", "anthropic", "true", "false", "─"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("table missing %q:\n%s", want, plain)
		}
	}
	// columns follow the payload's key order, not alphabetical
	header := strings.SplitN(plain, "\n", 2)[0]
	if !(strings.Index(header, "name") < strings.Index(header, "models") &&
		strings.Index(header, "models") < strings.Index(header, "enabled")) {
		t.Fatalf("columns out of order: %q", header)
	}
}

func TestPrettyJSONFlatObjectIsAligned(t *testing.T) {
	raw := `{"name":"add","version":"1.0.0","count":42}`
	got, ok := prettyJSON(raw, 80)
	if !ok {
		t.Fatal("flat object should pretty-print")
	}
	plain := stripANSI(got)
	for _, want := range []string{"name", "add", "version", "1.0.0", "count", "42"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("aligned view missing %q:\n%s", want, plain)
		}
	}
	if strings.HasPrefix(strings.TrimSpace(plain), "{") {
		t.Fatalf("flat object should not fall back to indented JSON:\n%s", plain)
	}
	if strings.Contains(plain, "#") {
		t.Fatalf("flat object should not render as a table:\n%s", plain)
	}
}

func TestPrettyJSONNestedFallsBackToIndent(t *testing.T) {
	cases := []string{
		`{"a":{"b":1},"c":[1,2]}`, // nested object value
		`[1,2,3]`,                 // array of scalars, not objects
		`42`,                      // top-level scalar
		`"a string : with \" quote"`,
	}
	for _, raw := range cases {
		got, ok := prettyJSON(raw, 80)
		if !ok {
			t.Fatalf("valid JSON not recognized: %s", raw)
		}
		if stripANSI(got) != indented(t, raw) {
			t.Fatalf("nested/scalar should match indented JSON for %s:\n got %q\nwant %q",
				raw, stripANSI(got), indented(t, raw))
		}
	}
}

func TestPrettyJSONRejectsNonJSON(t *testing.T) {
	for _, raw := range []string{"hello world", "", "{not json", "12 34"} {
		if _, ok := prettyJSON(raw, 80); ok {
			t.Fatalf("non-JSON accepted: %q", raw)
		}
	}
}

func TestTableTruncatesWideColumns(t *testing.T) {
	raw := `[{"note":"` + strings.Repeat("x", 200) + `"}]`
	got, ok := prettyJSON(raw, 40)
	if !ok {
		t.Fatal("should still render")
	}
	for _, line := range strings.Split(stripANSI(got), "\n") {
		if len([]rune(line)) > 40 {
			t.Fatalf("line exceeds width 40 (%d): %q", len([]rune(line)), line)
		}
	}
}
