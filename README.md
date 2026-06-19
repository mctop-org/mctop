# mctop

A terminal client for MCP servers. Connect to any server, browse its tools,
resources, and prompts, call them, and watch the result, without leaving the
shell. Then assert the server's contract in CI so a renamed tool or a drifted
schema fails the build instead of breaking an agent in production.

Think `curl` and `k9s`, but for the Model Context Protocol.

> Status: early. Building in the open. See [DESIGN.md](./DESIGN.md).

## What it does

- **Explore** a server in a TUI: tools, resources, and prompts, with their
  schemas, and call any tool with real arguments.
- **Script** it headless: `mctop ls` to list, `mctop call` for one-shot calls.
- **Test** it in CI: `mctop test spec.yaml` runs a contract and exits non-zero
  when it breaks.

Works over stdio (it spawns the server) and Streamable HTTP (it connects to a
URL).

## Install

```
curl -fsSL https://raw.githubusercontent.com/aloki-alok/mctop/main/install.sh | sh
```

Or with Homebrew, or the Go toolchain:

```
brew install aloki-alok/tap/mctop
go install github.com/aloki-alok/mctop@latest
```

## Usage

```
mctop <target>                 open the TUI against a server
mctop ls <target>              list tools, resources, and prompts
mctop call <target> <tool>     call one tool and print the result
mctop test <spec.yaml>         run a contract, exit 0 on pass, 1 on fail
```

A target is either a command to spawn (`"uvx mcp-server-time"`) or an
`http(s)://` URL.

### Interactive mode

Run `mctop <target>` with no subcommand to open the TUI: browse tools,
resources, and prompts; press enter on a tool to fill its arguments in a
schema-driven form and run it; read the result, then go again.

```
↑↓ move    enter open    / search    tab switch section    q quit
```

In the result view: `r` re-runs, `e` edits the arguments, `esc` goes back.

### Updating

```
mctop upgrade
```

Replaces the binary with the latest release (checksum-verified). However you
installed, this keeps it current.

For OAuth-protected servers, log in once and mctop handles the token after that:

```
mctop login https://api.example.com/mcp   # opens the browser, caches the token
mctop ls    https://api.example.com/mcp   # uses it automatically
mctop logout https://api.example.com/mcp  # forgets it
```

The token is cached per host and refreshed as needed, so you log in once per
server, not per command. For servers that take a static token instead, pass it
with `-H` (repeatable):

```
mctop ls https://api.example.com/mcp -H "Authorization: Bearer $TOKEN"
```

In a test spec, headers live under the server and expand from the
environment, so a token stays out of the committed file:

```yaml
server:
  url: "https://api.example.com/mcp"
  headers:
    Authorization: "Bearer ${TOKEN}"
expect:
  tools: [search]
```

The `examples/` directory has runnable specs and a small demo server you can
point mctop at (`go run ./examples/demoserver`, then `mctop ls
http://localhost:8080/mcp`).

## License

MIT.
