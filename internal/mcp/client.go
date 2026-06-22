// Package mcp wraps the official MCP Go SDK into the small surface mctop needs:
// connect to a server, list its tools/resources/prompts, and call a tool. It
// hides the SDK so the rest of mctop (subcommands, TUI) depends on one seam.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client is a connected, initialized MCP session.
type Client struct {
	sess *sdk.ClientSession
	rec  *recorder
}

// Options tunes how Connect dials a target.
type Options struct {
	// Headers are sent on every HTTP request, the place a bearer token or other
	// auth lives. They are ignored for stdio targets, which have no request
	// headers.
	Headers map[string]string

	// SSE selects the legacy HTTP+SSE transport instead of streamable HTTP for an
	// http(s):// target. It is for older servers that only speak SSE; modern
	// servers use streamable HTTP, the default. Ignored for stdio targets.
	SSE bool
}

// Connect dials a target and returns an initialized client. A target is either
// an http(s):// URL served over the streamable HTTP transport or a command to
// spawn over stdio (for example "uvx mcp-server-time"). Every JSON-RPC frame is
// recorded so the trace view can show the protocol after the fact.
func Connect(ctx context.Context, target string, opts Options) (*Client, error) {
	transport, err := transportFor(target, opts)
	if err != nil {
		return nil, err
	}
	rec := &recorder{}
	client := sdk.NewClient(&sdk.Implementation{Name: "mctop", Version: "dev"}, nil)
	sess, err := client.Connect(ctx, &sdk.LoggingTransport{Transport: transport, Writer: rec}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %q: %w", target, err)
	}
	return &Client{sess: sess, rec: rec}, nil
}

// transportFor picks the transport for a target: an http(s):// URL gets
// streamable HTTP, or legacy HTTP+SSE when opts.SSE is set, both carrying any
// fixed headers; anything else is spawned over stdio.
func transportFor(target string, opts Options) (sdk.Transport, error) {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		var httpClient *http.Client
		if len(opts.Headers) > 0 {
			httpClient = &http.Client{Transport: headerRoundTripper{headers: opts.Headers}}
		}
		if opts.SSE {
			return &sdk.SSEClientTransport{Endpoint: target, HTTPClient: httpClient}, nil
		}
		return &sdk.StreamableClientTransport{Endpoint: target, HTTPClient: httpClient}, nil
	}
	argv := strings.Fields(target)
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	return &sdk.CommandTransport{Command: exec.Command(argv[0], argv[1:]...)}, nil
}

// Trace returns the JSON-RPC frames recorded so far, oldest first.
func (c *Client) Trace() []Frame { return c.rec.snapshot() }

// headerRoundTripper adds fixed headers to every request before delegating to
// the default transport.
type headerRoundTripper struct {
	headers map[string]string
}

func (h headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// Close ends the session and stops the spawned server.
func (c *Client) Close() error { return c.sess.Close() }

// Server reports the connected server's name and version, if it advertised them.
func (c *Client) Server() (name, version string) {
	if r := c.sess.InitializeResult(); r != nil && r.ServerInfo != nil {
		return r.ServerInfo.Name, r.ServerInfo.Version
	}
	return "", ""
}

// HasResources reports whether the server advertised the resources capability.
// Calling resources/list on a server that lacks it can error and, over HTTP,
// tear down the session, so callers gate on this.
func (c *Client) HasResources() bool {
	r := c.sess.InitializeResult()
	return r != nil && r.Capabilities != nil && r.Capabilities.Resources != nil
}

// HasPrompts reports whether the server advertised the prompts capability.
func (c *Client) HasPrompts() bool {
	r := c.sess.InitializeResult()
	return r != nil && r.Capabilities != nil && r.Capabilities.Prompts != nil
}

// Tools lists the server's tools.
func (c *Client) Tools(ctx context.Context) ([]*sdk.Tool, error) {
	res, err := c.sess.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	return res.Tools, nil
}

// Resources lists the server's resources.
func (c *Client) Resources(ctx context.Context) ([]*sdk.Resource, error) {
	res, err := c.sess.ListResources(ctx, nil)
	if err != nil {
		return nil, err
	}
	return res.Resources, nil
}

// Prompts lists the server's prompts.
func (c *Client) Prompts(ctx context.Context) ([]*sdk.Prompt, error) {
	res, err := c.sess.ListPrompts(ctx, nil)
	if err != nil {
		return nil, err
	}
	return res.Prompts, nil
}

// Call invokes a tool by name with JSON-able arguments and returns the result.
func (c *Client) Call(ctx context.Context, name string, args map[string]any) (*sdk.CallToolResult, error) {
	return c.sess.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
}

// ReadResource fetches a resource's contents by URI.
func (c *Client) ReadResource(ctx context.Context, uri string) (*sdk.ReadResourceResult, error) {
	return c.sess.ReadResource(ctx, &sdk.ReadResourceParams{URI: uri})
}

// GetPrompt renders a prompt by name with the given arguments.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*sdk.GetPromptResult, error) {
	return c.sess.GetPrompt(ctx, &sdk.GetPromptParams{Name: name, Arguments: args})
}

// RenderResource flattens a resource's contents to text, showing binary blobs
// as a size note rather than dumping bytes.
func RenderResource(r *sdk.ReadResourceResult) string {
	var b strings.Builder
	for i, c := range r.Contents {
		if i > 0 {
			b.WriteString("\n")
		}
		switch {
		case c.Text != "":
			b.WriteString(c.Text)
		case len(c.Blob) > 0:
			fmt.Fprintf(&b, "<%d bytes of %s>", len(c.Blob), c.MIMEType)
		}
	}
	return b.String()
}

// RenderPrompt flattens a prompt's messages into markdown: each message's role
// becomes a heading above its content, so the result view can render it with the
// message bodies (which are often markdown themselves) formatted.
func RenderPrompt(r *sdk.GetPromptResult) string {
	var b strings.Builder
	for i, m := range r.Messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "## %s\n\n%s", m.Role, RenderContent([]sdk.Content{m.Content}))
	}
	return b.String()
}

// RenderResult turns a tool result into the text mctop displays, choosing the
// most structured form available so the insight view can lay it out. Text that
// already is JSON (optionally inside a markdown fence) passes through; otherwise
// the server's structured payload is used when present, falling back to the
// plain text blocks.
func RenderResult(res *sdk.CallToolResult) string {
	text := RenderContent(res.Content)
	if s := unwrapJSON(text); json.Valid([]byte(s)) {
		return s
	}
	if res.StructuredContent != nil {
		if raw, err := json.Marshal(res.StructuredContent); err == nil && json.Valid(raw) {
			return string(raw)
		}
	}
	return text
}

// unwrapJSON strips a surrounding markdown code fence so a JSON payload a server
// wrapped for display can still be parsed. It leaves unfenced text untouched
// beyond trimming surrounding whitespace.
func unwrapJSON(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	if nl := strings.IndexByte(t, '\n'); nl >= 0 {
		t = t[nl+1:] // drop the opening ``` or ```json line
	}
	t = strings.TrimSpace(t)
	t = strings.TrimSuffix(t, "```")
	return strings.TrimSpace(t)
}

// RenderContent flattens a tool result's content blocks into readable text.
// Text blocks pass through; other block types are shown as their JSON so nothing
// is silently dropped.
func RenderContent(content []sdk.Content) string {
	var b strings.Builder
	for i, block := range content {
		if i > 0 {
			b.WriteString("\n")
		}
		switch v := block.(type) {
		case *sdk.TextContent:
			b.WriteString(v.Text)
		default:
			if raw, err := json.Marshal(block); err == nil {
				b.Write(raw)
			} else {
				fmt.Fprintf(&b, "%v", block)
			}
		}
	}
	return b.String()
}
