// Package mcp wraps the official MCP Go SDK into the small surface mctop needs:
// connect to a server, list its tools/resources/prompts, and call a tool. It
// hides the SDK so the rest of mctop (subcommands, TUI) depends on one seam.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client is a connected, initialized MCP session.
type Client struct {
	sess *sdk.ClientSession
}

// Connect dials a target and returns an initialized client. A target is either
// an http(s):// URL served over the streamable HTTP transport or a command to
// spawn over stdio (for example "uvx mcp-server-time").
func Connect(ctx context.Context, target string) (*Client, error) {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return connectHTTP(ctx, target)
	}
	return connectStdio(ctx, target)
}

func connectHTTP(ctx context.Context, endpoint string) (*Client, error) {
	client := sdk.NewClient(&sdk.Implementation{Name: "mctop", Version: "dev"}, nil)
	sess, err := client.Connect(ctx, &sdk.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %q: %w", endpoint, err)
	}
	return &Client{sess: sess}, nil
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
