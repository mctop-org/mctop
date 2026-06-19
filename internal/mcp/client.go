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
}

// Options tunes how Connect dials a target.
type Options struct {
	// Headers are sent on every HTTP request, the place a bearer token or other
	// auth lives. They are ignored for stdio targets, which have no request
	// headers.
	Headers map[string]string
}

// Connect dials a target and returns an initialized client. A target is either
// an http(s):// URL served over the streamable HTTP transport or a command to
// spawn over stdio (for example "uvx mcp-server-time").
func Connect(ctx context.Context, target string, opts Options) (*Client, error) {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return connectHTTP(ctx, target, opts.Headers)
	}
	return connectStdio(ctx, target)
}

func connectHTTP(ctx context.Context, endpoint string, headers map[string]string) (*Client, error) {
	client := sdk.NewClient(&sdk.Implementation{Name: "mctop", Version: "dev"}, nil)
	transport := &sdk.StreamableClientTransport{Endpoint: endpoint}
	if len(headers) > 0 {
		transport.HTTPClient = &http.Client{Transport: headerRoundTripper{headers: headers}}
	}
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %q: %w", endpoint, err)
	}
	return &Client{sess: sess}, nil
}

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

func connectStdio(ctx context.Context, command string) (*Client, error) {
	argv := strings.Fields(command)
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	client := sdk.NewClient(&sdk.Implementation{Name: "mctop", Version: "dev"}, nil)
	sess, err := client.Connect(ctx, &sdk.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %q: %w", command, err)
	}
	return &Client{sess: sess}, nil
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

// RenderPrompt flattens a prompt's messages into readable role-tagged text.
func RenderPrompt(r *sdk.GetPromptResult) string {
	var b strings.Builder
	for i, m := range r.Messages {
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "[%s]\n%s", m.Role, RenderContent([]sdk.Content{m.Content}))
	}
	return b.String()
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
