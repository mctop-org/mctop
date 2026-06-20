package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// timeNow is overridable in tests so relative dates are deterministic.
var timeNow = time.Now

var urlStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Underline(true)

// humanValue renders a decoded result for people rather than developers: objects
// become labelled fields and sections, arrays of objects become tables, and
// values are formatted by what they are (dates, yes/no, status, links, lists).
func humanValue(v any, width, depth int) string {
	switch x := v.(type) {
	case *object:
		return humanObject(x, width, depth)
	case []any:
		return humanArray(x, width, depth)
	default:
		return formatField("", v)
	}
}

// field is one short label/value pair in the aligned block.
type field struct {
	label string
	value string // already styled
}

func humanObject(o *object, width, depth int) string {
	var shorts []field
	type section struct {
		key  string
		v    any
		text string // set when the section is a long string
	}
	var sections []section
	for _, k := range o.keys {
		v := o.vals[k]
		switch {
		case isShortScalar(v):
			shorts = append(shorts, field{humanLabel(k), formatField(k, v)})
		case isString(v):
			sections = append(sections, section{key: k, text: v.(string)})
		default:
			sections = append(sections, section{key: k, v: v})
		}
	}

	var parts []string
	if len(shorts) > 0 {
		parts = append(parts, renderFields(shorts, width))
	}
	for _, s := range sections {
		title := accent.Bold(true).Render(humanLabel(s.key))
		var body string
		if s.text != "" {
			lines := strings.Split(wrapPlain(s.text, width-2), "\n")
			for i := range lines {
				lines[i] = green.Render(lines[i])
			}
			body = strings.Join(lines, "\n")
		} else {
			body = humanValue(s.v, width-2, depth+1)
		}
		parts = append(parts, title+"\n"+indentBy(body, 2))
	}
	return strings.Join(parts, "\n\n")
}

func humanArray(arr []any, width, depth int) string {
	if len(arr) == 0 {
		return dim.Render("(empty)")
	}
	allObjects := true
	for _, e := range arr {
		if _, ok := e.(*object); !ok {
			allObjects = false
			break
		}
	}
	if allObjects {
		if t, ok := renderTable(arr, width, -1); ok {
			return t
		}
		blocks := make([]string, len(arr))
		for i, e := range arr {
			blocks[i] = dim.Render(fmt.Sprintf("[%d]", i+1)) + "\n" +
				indentBy(humanObject(e.(*object), width-2, depth+1), 2)
		}
		return strings.Join(blocks, "\n\n")
	}
	lines := make([]string, len(arr))
	for i, e := range arr {
		lines[i] = "• " + formatField("", e)
	}
	return strings.Join(lines, "\n")
}

// renderFields lays short label/value pairs out, flowing them into multiple
// columns when the values are short enough that the width has room to spare.
func renderFields(fields []field, width int) string {
	labelW := 0
	for _, f := range fields {
		if w := cellWidth(f.label); w > labelW {
			labelW = w
		}
	}
	if labelW > 24 {
		labelW = 24
	}

	maxValW, multi := 0, true
	for _, f := range fields {
		w := ansi.StringWidth(f.value)
		if w > 36 {
			multi = false
		}
		if w > maxValW {
			maxValW = w
		}
	}
	fieldW := labelW + 2 + maxValW
	gap := 4
	cols := 1
	if multi && fieldW > 0 {
		cols = clamp((width+gap)/(fieldW+gap), 1, 3)
		if cols > len(fields) {
			cols = len(fields)
		}
	}

	perCol := (len(fields) + cols - 1) / cols
	var lines []string
	for r := 0; r < perCol; r++ {
		var cells []string
		for c := 0; c < cols; c++ {
			idx := c*perCol + r
			if idx >= len(fields) {
				break
			}
			f := fields[idx]
			cell := accent.Render(padRight(f.label, labelW)) + "  " + f.value
			if c < cols-1 {
				cell += strings.Repeat(" ", max(0, fieldW-ansi.StringWidth(cell)))
			}
			cells = append(cells, cell)
		}
		lines = append(lines, strings.Join(cells, strings.Repeat(" ", gap)))
	}
	return strings.Join(lines, "\n")
}

// formatField renders a single scalar by what it represents.
func formatField(key string, v any) string {
	switch x := v.(type) {
	case nil:
		return dim.Render("—")
	case bool:
		if x {
			return green.Render("✓ yes")
		}
		return dim.Render("✗ no")
	case json.Number:
		return jNum.Render(humanNumber(x))
	case string:
		switch {
		case x == "":
			return dim.Render("—")
		case isStatusKey(key):
			return formatStatus(x)
		}
		if t, ok := parseTime(x); ok {
			return formatTime(t)
		}
		if isURL(x) {
			return urlStyle.Render(x)
		}
		return green.Render(x)
	default:
		return dim.Render(compact(v))
	}
}

func isShortScalar(v any) bool {
	switch x := v.(type) {
	case nil, bool, json.Number:
		return true
	case string:
		return !strings.Contains(x, "\n") && utf8.RuneCountInString(x) <= 48
	default:
		return false
	}
}

func isString(v any) bool {
	_, ok := v.(string)
	return ok
}

// humanLabel turns a field key into a sentence-case label: snake_case, kebab,
// and camelCase all split into words, with a few acronyms kept uppercase.
func humanLabel(key string) string {
	spaced := strings.NewReplacer("_", " ", "-", " ").Replace(key)
	var words []string
	var cur strings.Builder
	prevLower := false
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range spaced {
		if r == ' ' {
			flush()
			continue
		}
		if unicode.IsUpper(r) && prevLower {
			flush()
		}
		cur.WriteRune(r)
		prevLower = unicode.IsLower(r)
	}
	flush()
	if len(words) == 0 {
		return key
	}
	for i, w := range words {
		lw := strings.ToLower(w)
		if up, ok := acronyms[lw]; ok {
			words[i] = up
		} else if i == 0 {
			words[i] = capitalize(lw)
		} else {
			words[i] = lw
		}
	}
	return strings.Join(words, " ")
}

var acronyms = map[string]string{
	"id": "ID", "url": "URL", "uri": "URI", "api": "API",
	"ip": "IP", "sdk": "SDK", "ssl": "SSL", "dns": "DNS",
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func isStatusKey(key string) bool {
	k := strings.ToLower(key)
	return k == "status" || k == "state" || k == "health" ||
		strings.HasSuffix(k, "_status") || strings.HasSuffix(k, "_state")
}

func formatStatus(v string) string {
	style := jNum // amber default for pending/unknown
	switch strings.ToLower(v) {
	case "active", "ok", "success", "succeeded", "healthy", "up", "enabled", "running", "online", "ready", "pass", "passed":
		style = green
	case "error", "failed", "failure", "down", "unhealthy", "disabled", "offline", "inactive", "stopped", "fail":
		style = red
	}
	return style.Render("● " + v)
}

// parseTime recognizes the common machine date formats so they can be humanized.
func parseTime(s string) (time.Time, bool) {
	for _, layout := range []string{
		time.RFC3339Nano, time.RFC3339,
		"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func formatTime(t time.Time) string {
	date := t.Format("Jan 2, 2006")
	if t.Hour() != 0 || t.Minute() != 0 {
		date += " " + t.Format("15:04")
	}
	s := green.Render(date)
	if rel := relativeTime(timeNow(), t); rel != "" {
		s += dim.Render("  (" + rel + ")")
	}
	return s
}

func relativeTime(now, t time.Time) string {
	d := now.Sub(t)
	future := d < 0
	if future {
		d = -d
	}
	var s string
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		s = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		s = fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		s = fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		s = fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	default:
		s = fmt.Sprintf("%dy", int(d.Hours()/24/365))
	}
	if future {
		return "in " + s
	}
	return s + " ago"
}

// humanNumber groups integer thousands for readability and leaves floats and
// exponents untouched.
func humanNumber(n json.Number) string {
	s := n.String()
	if strings.ContainsAny(s, ".eE") {
		return s
	}
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	if len(s) <= 4 {
		return sign + s
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}

func indentBy(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}
