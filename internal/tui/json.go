package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	jNum = lipgloss.NewStyle().Foreground(lipgloss.Color("173")) // soft amber
	jLit = lipgloss.NewStyle().Foreground(lipgloss.Color("103")) // muted blue-grey

	reStr = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	reLit = regexp.MustCompile(`\b(?:true|false|null)\b`)
	reNum = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?`)
)

// object is a JSON object that remembers its key order, which json.Unmarshal into
// a map would lose. Column and row order follow the original payload.
type object struct {
	keys []string
	vals map[string]any
}

// decodeOrdered parses one JSON value, preserving object key order. Scalars come
// back as string, json.Number, bool, or nil; objects as *object; arrays as []any.
func decodeOrdered(raw string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	v, err := readValue(dec)
	if err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("trailing data after JSON value")
	}
	return v, nil
}

func readValue(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return t, nil // string, json.Number, bool, or nil
	}
	switch delim {
	case '{':
		o := &object{vals: map[string]any{}}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key := kt.(string)
			val, err := readValue(dec)
			if err != nil {
				return nil, err
			}
			o.keys = append(o.keys, key)
			o.vals[key] = val
		}
		_, err = dec.Token() // closing }
		return o, err
	case '[':
		var arr []any
		for dec.More() {
			val, err := readValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		_, err = dec.Token() // closing ]
		return arr, err
	}
	return t, nil
}

// cell kinds drive both alignment and color.
const (
	kEmpty = iota
	kStr
	kNum
	kLit
	kNested
)

type cell struct {
	text string
	kind int
}

func toCell(v any, present bool) cell {
	if !present {
		return cell{"", kEmpty}
	}
	switch x := v.(type) {
	case nil:
		return cell{"null", kLit}
	case bool:
		return cell{strconv.FormatBool(x), kLit}
	case json.Number:
		return cell{x.String(), kNum}
	case string:
		return cell{x, kStr}
	default:
		return cell{compact(v), kNested}
	}
}

func styleCell(text string, kind int) string {
	switch kind {
	case kNum:
		return jNum.Render(text)
	case kLit:
		return jLit.Render(text)
	case kStr:
		return green.Render(text)
	case kNested:
		return dim.Render(text)
	default:
		return text
	}
}

// renderTable lays an array of objects out as columns. It declines (ok=false)
// when the array is empty, holds non-objects, or cannot be made to fit, so the
// caller falls back to indented JSON.
func renderTable(arr []any, width int) (string, bool) {
	if len(arr) == 0 {
		return "", false
	}
	rows := make([]*object, len(arr))
	for i, el := range arr {
		o, ok := el.(*object)
		if !ok {
			return "", false
		}
		rows[i] = o
	}
	const maxTableRows = 1000
	extra := 0
	if len(rows) > maxTableRows {
		extra = len(rows) - maxTableRows
		rows = rows[:maxTableRows]
	}

	cols := orderedColumns(rows)
	if len(cols) == 0 {
		return "", false
	}
	headers := append([]string{"#"}, cols...)

	grid := make([][]cell, len(rows))
	for i, o := range rows {
		line := make([]cell, len(headers))
		line[0] = cell{strconv.Itoa(i + 1), kNum}
		for j, c := range cols {
			v, present := o.vals[c]
			line[j+1] = toCell(v, present)
		}
		grid[i] = line
	}

	numeric := make([]bool, len(headers))
	colW := make([]int, len(headers))
	for j, h := range headers {
		colW[j] = cellWidth(h)
		num, hasVal := true, false
		for i := range grid {
			cl := grid[i][j]
			if cl.kind != kEmpty {
				hasVal = true
				if cl.kind != kNum {
					num = false
				}
			}
			if w := cellWidth(cl.text); w > colW[j] {
				colW[j] = w
			}
		}
		numeric[j] = num && hasVal
	}
	numeric[0] = true // the index column

	if !fitColumns(colW, width) {
		return "", false
	}

	var b strings.Builder
	b.WriteString(renderHeader(headers, colW, numeric))
	b.WriteString("\n")
	b.WriteString(dim.Render(strings.Repeat("─", rowWidth(colW))))
	b.WriteString("\n")
	for i := range rows {
		texts := make([]string, len(headers))
		kinds := make([]int, len(headers))
		for j := range headers {
			texts[j] = grid[i][j].text
			kinds[j] = grid[i][j].kind
		}
		// the index column reads as a quiet gutter, not data
		kinds[0] = kEmpty
		b.WriteString(renderRowCells(texts, kinds, colW, numeric))
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	if extra > 0 {
		b.WriteString("\n" + dim.Render(fmt.Sprintf("… %d more rows", extra)))
	}
	return b.String(), true
}

// orderedColumns is the union of the rows' keys in first-seen order.
func orderedColumns(rows []*object) []string {
	var cols []string
	seen := map[string]bool{}
	for _, o := range rows {
		for _, k := range o.keys {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	return cols
}

// fitColumns shrinks the widest columns (capping any single column at 40) until
// the row fits the width, truncating cells later. It returns false if the row
// cannot fit even with every column at the minimum.
func fitColumns(colW []int, width int) bool {
	const cap, min = 40, 3
	for i := range colW {
		if colW[i] > cap {
			colW[i] = cap
		}
	}
	for rowWidth(colW) > width {
		widest := -1
		for i := range colW {
			if colW[i] > min && (widest == -1 || colW[i] > colW[widest]) {
				widest = i
			}
		}
		if widest == -1 {
			return false
		}
		colW[widest]--
	}
	return true
}

func rowWidth(colW []int) int {
	total := 0
	for _, w := range colW {
		total += w
	}
	return total + 2*(len(colW)-1)
}

func renderRowCells(texts []string, kinds []int, colW []int, numeric []bool) string {
	cells := make([]string, len(texts))
	for j := range texts {
		t := ellipsize(texts[j], colW[j])
		if numeric[j] {
			t = padLeft(t, colW[j])
		} else {
			t = padRight(t, colW[j])
		}
		cells[j] = styleCell(t, kinds[j])
	}
	return strings.Join(cells, "  ")
}

// renderHeader styles the column titles in one accent style, aligned the same
// way as their data cells.
func renderHeader(texts []string, colW []int, numeric []bool) string {
	cells := make([]string, len(texts))
	for j := range texts {
		t := ellipsize(texts[j], colW[j])
		if numeric[j] {
			t = padLeft(t, colW[j])
		} else {
			t = padRight(t, colW[j])
		}
		cells[j] = accent.Render(t)
	}
	return strings.Join(cells, "  ")
}

// compact renders any decoded value back to minified JSON for a table cell that
// holds nested data.
func compact(v any) string {
	switch x := v.(type) {
	case *object:
		parts := make([]string, len(x.keys))
		for i, k := range x.keys {
			parts[i] = strconv.Quote(k) + ":" + compact(x.vals[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = compact(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(x)
	case json.Number:
		return x.String()
	case string:
		return strconv.Quote(x)
	default:
		return ""
	}
}

func cellWidth(s string) int { return utf8.RuneCountInString(s) }

func padRight(s string, w int) string {
	if d := w - cellWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

func padLeft(s string, w int) string {
	if d := w - cellWidth(s); d > 0 {
		return strings.Repeat(" ", d) + s
	}
	return s
}

func ellipsize(s string, w int) string {
	if cellWidth(s) <= w {
		return s
	}
	r := []rune(s)
	if w <= 1 {
		return string(r[:max(0, w)])
	}
	return string(r[:w-1]) + "…"
}

// indentJSON indents a JSON payload and lightly colors keys, strings, numbers,
// and literals: the fallback for nested data that no flat layout fits.
func indentJSON(raw string) (string, bool) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(raw), "", "  "); err != nil {
		return "", false
	}
	lines := strings.Split(buf.String(), "\n")
	for i, line := range lines {
		lines[i] = colorJSONLine(line)
	}
	return strings.Join(lines, "\n"), true
}

// colorJSONLine colors one line of indented JSON. A line that opens with a
// quoted string immediately followed by a colon is an object key; everything
// else is treated as a value.
func colorJSONLine(line string) string {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	rest := line[len(indent):]
	if loc := reStr.FindStringIndex(rest); loc != nil && loc[0] == 0 {
		if after := strings.TrimLeft(rest[loc[1]:], " "); strings.HasPrefix(after, ":") {
			key := rest[:loc[1]]
			tail := rest[loc[1]:]
			colon := strings.IndexByte(tail, ':')
			return indent + accent.Render(key) + dim.Render(tail[:colon+1]) + colorJSONValue(tail[colon+1:])
		}
	}
	return indent + colorJSONValue(rest)
}

// colorJSONValue colors the value portion of a line, walking string tokens whole
// so their contents are never recolored as scalars.
func colorJSONValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '"' {
			if loc := reStr.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
				b.WriteString(green.Render(s[i : i+loc[1]]))
				i += loc[1]
				continue
			}
		}
		next := strings.IndexByte(s[i:], '"')
		if next < 0 {
			b.WriteString(colorJSONScalars(s[i:]))
			break
		}
		b.WriteString(colorJSONScalars(s[i : i+next]))
		i += next
	}
	return b.String()
}

// colorJSONScalars colors numbers and literals in a chunk that contains no
// strings. Numbers run first because literals carry no digits, so coloring them
// afterward cannot corrupt the escape codes the numbers introduced.
func colorJSONScalars(s string) string {
	s = reNum.ReplaceAllStringFunc(s, func(m string) string { return jNum.Render(m) })
	s = reLit.ReplaceAllStringFunc(s, func(m string) string { return jLit.Render(m) })
	return s
}
