package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHumanLabel(t *testing.T) {
	cases := map[string]string{
		"name":       "Name",
		"created_at": "Created at",
		"latencyMs":  "Latency ms",
		"userID":     "User ID",
		"api_url":    "API URL",
		"enabled":    "Enabled",
	}
	for in, want := range cases {
		if got := humanLabel(in); got != want {
			t.Errorf("humanLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatFieldByType(t *testing.T) {
	if got := stripANSI(formatField("x", true)); !strings.Contains(got, "yes") {
		t.Errorf("bool true = %q", got)
	}
	if got := stripANSI(formatField("x", false)); !strings.Contains(got, "no") {
		t.Errorf("bool false = %q", got)
	}
	if got := stripANSI(formatField("x", nil)); got != "—" {
		t.Errorf("nil = %q, want —", got)
	}
	if got := stripANSI(formatField("status", "active")); !strings.Contains(got, "● active") {
		t.Errorf("status = %q, want a dot", got)
	}
	if got := stripANSI(formatField("link", "https://x.io")); got != "https://x.io" {
		t.Errorf("url = %q", got)
	}
}

func TestHumanNumberGroups(t *testing.T) {
	cases := map[string]string{"1234567": "1,234,567", "12345": "12,345", "1000": "1000", "-12345": "-12,345", "3.14": "3.14"}
	for in, want := range cases {
		if got := humanNumber(json.Number(in)); got != want {
			t.Errorf("humanNumber(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTime(t *testing.T) {
	if _, ok := parseTime("2026-01-15T10:30:00Z"); !ok {
		t.Error("RFC3339 should parse")
	}
	if _, ok := parseTime("2026-01-15"); !ok {
		t.Error("date should parse")
	}
	for _, s := range []string{"1.0.0", "active", "42", ""} {
		if _, ok := parseTime(s); ok {
			t.Errorf("%q should not parse as a date", s)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	past := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	if got := relativeTime(now, past); got != "5mo ago" {
		t.Errorf("relativeTime = %q, want 5mo ago", got)
	}
	future := now.Add(48 * time.Hour)
	if got := relativeTime(now, future); !strings.HasPrefix(got, "in ") {
		t.Errorf("future = %q, want 'in ...'", got)
	}
}

func TestHumanValueRoutes(t *testing.T) {
	// flat object: humanized labels and formatted values, no JSON braces
	obj := stripANSI(humanValue(decodeMust(`{"created_at":"2026-01-15","enabled":true}`), 80, 0))
	if !strings.Contains(obj, "Created at") || !strings.Contains(obj, "yes") {
		t.Fatalf("flat object not humanized:\n%s", obj)
	}
	if strings.Contains(obj, "{") {
		t.Fatalf("should not show raw JSON:\n%s", obj)
	}

	// nested object becomes a titled, indented section
	nested := stripANSI(humanValue(decodeMust(`{"config":{"model":"gpt-5"}}`), 80, 0))
	if !strings.Contains(nested, "Config") || !strings.Contains(nested, "  Model") {
		t.Fatalf("nested object not sectioned:\n%s", nested)
	}

	// array of scalars becomes a bullet list
	bullets := stripANSI(humanValue(decodeMust(`["a","b","c"]`), 80, 0))
	if strings.Count(bullets, "•") != 3 {
		t.Fatalf("array not bulleted:\n%s", bullets)
	}
}

func TestRenderFieldsUsesColumns(t *testing.T) {
	fields := []field{
		{"alpha", "1"}, {"bravo", "2"}, {"charlie", "3"},
		{"delta", "4"}, {"echo", "5"}, {"foxtrot", "6"},
	}
	out := renderFields(fields, 100)
	if lines := strings.Count(out, "\n") + 1; lines >= len(fields) {
		t.Fatalf("wide layout should pack fields into columns, got %d lines for %d fields", lines, len(fields))
	}
}

func decodeMust(raw string) any {
	v, err := decodeOrdered(raw)
	if err != nil {
		panic(err)
	}
	return v
}
