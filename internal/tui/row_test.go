package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	enterKey = tea.KeyMsg{Type: tea.KeyEnter}
	escKey   = tea.KeyMsg{Type: tea.KeyEsc}
)

func tableModel() model {
	m := model{screen: result, width: 80, height: 24, spin: newSpinner(), vim: true}
	m.vp = viewport.New(80, 10)
	m.output = `[{"name":"alpha","n":1},{"name":"beta","n":2},{"name":"gamma","n":3}]`
	m.rows = asObjectRows(decodeMust(m.output))
	m.vp.SetContent(m.resultBody())
	return m
}

func TestAsObjectRows(t *testing.T) {
	if rows := asObjectRows(decodeMust(`[{"a":1},{"b":2}]`)); len(rows) != 2 {
		t.Fatalf("array of objects should yield rows, got %d", len(rows))
	}
	for _, raw := range []string{`[1,2,3]`, `[]`, `{"a":1}`, `[{"a":1},2]`, `"x"`} {
		if asObjectRows(decodeMust(raw)) != nil {
			t.Fatalf("%s should not be rows", raw)
		}
	}
}

func TestTableMarksSelectedRow(t *testing.T) {
	rows := asObjectRows(decodeMust(`[{"a":1},{"a":2}]`))
	got, ok := renderObjectTable(rows, 80, 1)
	if !ok {
		t.Fatal("should render")
	}
	lines := strings.Split(stripANSI(got), "\n") // header, divider, row0, row1
	if strings.Contains(lines[2], "▌") {
		t.Fatalf("row 0 should not be marked:\n%s", got)
	}
	if !strings.Contains(lines[3], "▌") {
		t.Fatalf("row 1 should carry the selection marker:\n%s", got)
	}
}

func TestRenderObjectTableNoMarkerWhenUnselected(t *testing.T) {
	rows := asObjectRows(decodeMust(`[{"a":1},{"a":2}]`))
	got, _ := renderObjectTable(rows, 80, -1)
	if strings.Contains(got, "▌") {
		t.Fatal("an unselected table should have no marker gutter")
	}
}

func TestCallResultDetectsTable(t *testing.T) {
	m := model{width: 80, height: 24, spin: newSpinner()}
	m.vp = viewport.New(80, 10)
	m, _ = send(m, callResultMsg{output: `[{"a":1},{"a":2}]`, elapsed: "1ms"})
	if len(m.rows) != 2 {
		t.Fatalf("array-of-objects result should be row-navigable, got %d rows", len(m.rows))
	}
	m, _ = send(m, callResultMsg{output: `{"a":1}`, elapsed: "1ms"})
	if m.rows != nil {
		t.Fatal("a single object result should not be row-navigable")
	}
	m, _ = send(m, callResultMsg{err: errors.New("boom"), output: `[{"a":1}]`, elapsed: "1ms"})
	if m.rows != nil {
		t.Fatal("an errored result should not be row-navigable")
	}
}

func TestDetectRowsEnvelope(t *testing.T) {
	rows, env, key := detectRows(decodeMust(`{"status":"ok","total":2,"records":[{"a":1},{"a":2}]}`))
	if len(rows) != 2 || env == nil || key != "records" {
		t.Fatalf("envelope not detected: rows=%d env=%v key=%q", len(rows), env, key)
	}
	if rows, env, _ := detectRows(decodeMust(`[{"a":1}]`)); len(rows) != 1 || env != nil {
		t.Fatal("a bare array should have rows and no envelope")
	}
	if r, _, _ := detectRows(decodeMust(`{"a":[{"x":1}],"b":[{"y":2}]}`)); r != nil {
		t.Fatal("two record arrays are ambiguous and should not be navigable")
	}
	if r, _, _ := detectRows(decodeMust(`{"a":1,"b":"x"}`)); r != nil {
		t.Fatal("a plain object should not be navigable")
	}
}

func TestEnvelopeRowNavigation(t *testing.T) {
	m := model{width: 100, height: 24, spin: newSpinner(), vim: true}
	m.vp = viewport.New(100, 12)
	out := `{"status":"ok","total":3,"records":[{"id":1,"name":"a"},{"id":2,"name":"b"},{"id":3,"name":"c"}]}`
	m, _ = send(m, callResultMsg{output: out, elapsed: "1ms"})
	if len(m.rows) != 3 || m.rowEnvelope == nil {
		t.Fatal("an envelope result should be row-navigable")
	}
	if !strings.Contains(stripANSI(m.resultBody()), "Records") {
		t.Fatal("the list should show the records section title above the table")
	}
	m, _ = send(m, key("down"))
	if m.rowCursor != 1 {
		t.Fatalf("down should select row 1, got %d", m.rowCursor)
	}
	m, _ = send(m, enterKey)
	if !m.rowOpen {
		t.Fatal("enter should expand a nested-envelope row")
	}
}

func TestTableCapsColumns(t *testing.T) {
	var fields []string
	for i := 0; i < 20; i++ {
		fields = append(fields, fmt.Sprintf(`"f%d":%d`, i, i))
	}
	raw := `[{` + strings.Join(fields, ",") + `}]`
	got, ok := renderObjectTable(asObjectRows(decodeMust(raw)), 100, 0)
	if !ok {
		t.Fatal("should render")
	}
	if !strings.Contains(stripANSI(got), "more fields per record") {
		t.Fatalf("a wide record should note the hidden fields:\n%s", stripANSI(got))
	}
}

// A wide record buries its identifying fields among filler (the shape of a
// crowded analytics payload); the capped table must surface them anyway.
func TestWideTableSurfacesIdentifyingColumns(t *testing.T) {
	var fields []string
	for i := 0; i < 10; i++ {
		fields = append(fields, fmt.Sprintf(`"total_metric_%d":%d`, i, i))
	}
	fields = append(fields, `"call_id":"c-1"`, `"status":"done"`, `"created_at":"2026-07-12"`)
	raw := `[{` + strings.Join(fields, ",") + `}]`
	got, ok := renderObjectTable(asObjectRows(decodeMust(raw)), 100, 0)
	if !ok {
		t.Fatal("should render")
	}
	plain := stripANSI(got)
	for _, want := range []string{"call_id", "status", "created_at"} {
		if !strings.Contains(plain, want) {
			t.Errorf("identifying column %q should survive the cap:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "more fields per record") {
		t.Fatal("the hidden-fields note should still show")
	}
}

func TestPickColumnsKeepsDeclaredOrder(t *testing.T) {
	cols := []string{"zeta", "status", "id", "alpha"}
	got := pickColumns(cols, 2)
	if len(got) != 2 || got[0] != "status" || got[1] != "id" {
		t.Fatalf("want the ranked survivors in declared order [status id], got %v", got)
	}
	all := pickColumns(cols, 4)
	if len(all) != 4 || all[0] != "zeta" {
		t.Fatalf("no cap means untouched order, got %v", all)
	}
}

func TestRowNavigationMovesSelectsAndExpands(t *testing.T) {
	m := tableModel() // three rows, vim on
	m, _ = send(m, key("down"))
	if m.rowCursor != 1 {
		t.Fatalf("down: want row 1, got %d", m.rowCursor)
	}
	m, _ = send(m, key("G"))
	if m.rowCursor != 2 {
		t.Fatalf("G: want last row, got %d", m.rowCursor)
	}
	m, _ = send(m, key("g"))
	if m.rowCursor != 0 {
		t.Fatalf("g: want first row, got %d", m.rowCursor)
	}
	m, _ = send(m, enterKey)
	if !m.rowOpen {
		t.Fatal("enter should expand the selected row")
	}
	m, _ = send(m, escKey)
	if m.rowOpen {
		t.Fatal("esc should collapse the row back to the list")
	}
	if m.screen != result {
		t.Fatal("collapsing should stay on the result screen")
	}
	m, _ = send(m, escKey)
	if m.screen != browse {
		t.Fatal("esc from the list should return to browse")
	}
}

func TestExpandedRowKeysScrollNotSelect(t *testing.T) {
	m := tableModel()
	m, _ = send(m, enterKey) // expand row 0
	m, _ = send(m, key("j")) // should scroll the detail, not move the row cursor
	if m.rowCursor != 0 {
		t.Fatalf("j in an expanded row should not move the row cursor, got %d", m.rowCursor)
	}
}

func TestRowDetailShowsFullValue(t *testing.T) {
	note := strings.Repeat("ab", 80) // 160 chars, far over the table cell cap
	m := model{screen: result, width: 80, height: 24, spin: newSpinner(), vim: true}
	m.vp = viewport.New(80, 10)
	m.output = `[{"name":"alpha","note":"` + note + `"}]`
	m.rows = asObjectRows(decodeMust(m.output))
	m.vp.SetContent(m.resultBody())

	if strings.Contains(stripANSI(m.resultBody()), note) {
		t.Fatal("the table cell should be truncated, not show the whole note")
	}
	m, _ = send(m, enterKey)
	flat := strings.NewReplacer("\n", "", " ", "").Replace(stripANSI(m.resultBody()))
	if !strings.Contains(flat, note) {
		t.Fatal("the expanded row should show the note in full")
	}
}
