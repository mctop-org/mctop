package tui

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func decodeOK(t *testing.T, raw string) any {
	t.Helper()
	v, err := decodeOrdered(raw)
	if err != nil {
		t.Fatalf("decodeOrdered(%s): %v", raw, err)
	}
	return v
}

func TestDecodeOrderedPreservesKeyOrder(t *testing.T) {
	o, ok := decodeOK(t, `{"z":1,"a":2,"m":3}`).(*object)
	if !ok {
		t.Fatal("want *object")
	}
	if want := []string{"z", "a", "m"}; !reflect.DeepEqual(o.keys, want) {
		t.Fatalf("key order = %v, want %v", o.keys, want)
	}
}

func TestDecodeOrderedRejectsNonJSON(t *testing.T) {
	for _, raw := range []string{"hello world", "", "{not json", "12 34"} {
		if _, err := decodeOrdered(raw); err == nil {
			t.Fatalf("non-JSON accepted: %q", raw)
		}
	}
}

func TestRenderTableArrayOfObjects(t *testing.T) {
	raw := `[{"name":"openai","models":3,"enabled":true},{"name":"anthropic","models":5,"enabled":false}]`
	got, ok := renderTable(decodeOK(t, raw).([]any), 80, -1)
	if !ok {
		t.Fatal("array of objects should render as a table")
	}
	plain := stripANSI(got)
	for _, want := range []string{"#", "name", "models", "enabled", "openai", "anthropic", "─"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("table missing %q:\n%s", want, plain)
		}
	}
	header := strings.SplitN(plain, "\n", 2)[0]
	if !(strings.Index(header, "name") < strings.Index(header, "models") &&
		strings.Index(header, "models") < strings.Index(header, "enabled")) {
		t.Fatalf("columns out of order: %q", header)
	}
}

func TestRenderTableCapsRows(t *testing.T) {
	var sb strings.Builder
	sb.WriteByte('[')
	for i := 0; i < 1500; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"id":` + strconv.Itoa(i) + `}`)
	}
	sb.WriteByte(']')
	got, ok := renderTable(decodeOK(t, sb.String()).([]any), 80, -1)
	if !ok {
		t.Fatal("should render")
	}
	if !strings.Contains(stripANSI(got), "500 more rows") {
		t.Fatal("expected the row-cap note")
	}
}

func TestRenderTableTruncatesWideColumns(t *testing.T) {
	raw := `[{"note":"` + strings.Repeat("x", 200) + `"}]`
	got, ok := renderTable(decodeOK(t, raw).([]any), 40, -1)
	if !ok {
		t.Fatal("should render")
	}
	for _, line := range strings.Split(stripANSI(got), "\n") {
		if len([]rune(line)) > 40 {
			t.Fatalf("line exceeds width 40 (%d): %q", len([]rune(line)), line)
		}
	}
}

func TestIndentJSONMatchesStdlib(t *testing.T) {
	for _, raw := range []string{`{"a":1,"b":[1,2]}`, `[1,2,3]`, `"x : y"`, `42`} {
		got, ok := indentJSON(raw)
		if !ok {
			t.Fatalf("indentJSON declined valid JSON: %s", raw)
		}
		var want bytes.Buffer
		if err := json.Indent(&want, []byte(raw), "", "  "); err != nil {
			t.Fatal(err)
		}
		if stripANSI(got) != want.String() {
			t.Fatalf("indent mismatch for %s:\n got %q\nwant %q", raw, stripANSI(got), want.String())
		}
	}
}
