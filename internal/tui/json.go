package tui

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	jNum = lipgloss.NewStyle().Foreground(lipgloss.Color("173")) // soft amber
	jLit = lipgloss.NewStyle().Foreground(lipgloss.Color("103")) // muted blue-grey

	reStr = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	reLit = regexp.MustCompile(`\b(?:true|false|null)\b`)
	reNum = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?`)
)

// prettyJSON indents a JSON payload and lightly colors keys, strings, numbers,
// and literals so a result is readable instead of a single dense line. It
// returns ok=false when the input is not valid JSON, so callers fall back to the
// raw text untouched.
func prettyJSON(raw string) (string, bool) {
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
