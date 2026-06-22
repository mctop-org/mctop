package tui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// renderMarkdown styles a resource or prompt body for the result view: headings,
// emphasis, lists, links, and fenced code become readable terminal output
// instead of raw source. It wraps to width and falls back to plain wrapped text
// if the renderer cannot be built, so a result is never lost.
func renderMarkdown(src string, width int) string {
	if width < 1 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return wrapPlain(src, width)
	}
	out, err := r.Render(src)
	if err != nil {
		return wrapPlain(src, width)
	}
	return strings.Trim(out, "\n")
}
