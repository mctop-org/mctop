# mctop

[![release](https://img.shields.io/github/v/release/aloki-alok/mctop?label=release)](https://github.com/aloki-alok/mctop/releases)
[![ci](https://img.shields.io/github/actions/workflow/status/aloki-alok/mctop/ci.yml?branch=main&label=ci)](https://github.com/aloki-alok/mctop/actions/workflows/ci.yml)
[![license](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

A terminal client for MCP servers. Connect to any server, browse its tools,
resources, and prompts, call them, and watch the result, without leaving the
shell. Then assert the server's contract in CI so a renamed tool or a drifted
schema fails the build instead of breaking an agent in production.

Think `curl` and `k9s`, but for the Model Context Protocol.

It works over stdio (it spawns the server) and Streamable HTTP (it connects to a
URL), with OAuth login for protected servers. Building in the open; see
[DESIGN.md](./DESIGN.md).

## What it does

- **Explore** a server interactively: browse tools, resources, and prompts, fill
  a tool's arguments in a schema-driven form, and read the result.
- **Script** it headless: `mctop ls` to list, `mctop call` for one-shot calls.
- **Test** it in CI: `mctop test spec.yaml` runs a contract and exits non-zero
  when it breaks.

## Install

```
curl -fsSL https://raw.githubusercontent.com/aloki-alok/mctop/main/install.sh | sh
```

Or with Homebrew, or the Go toolchain:

```
brew install aloki-alok/tap/mctop
go install github.com/aloki-alok/mctop@latest
```

Then `mctop upgrade` keeps it current, however you installed it.

## Usage

```
mctop <target>                 open the interactive client against a server
mctop ls <target>              list tools, resources, and prompts
mctop call <target> <tool>     call one tool and print the result
mctop login <url>              log in to an OAuth-protected server
mctop test <spec.yaml>         run a contract, exit 0 on pass, 1 on fail
mctop upgrade                  update to the latest release
```

A target is either a command to spawn (`"uvx mcp-server-time"`) or an
`http(s)://` URL.

```
mctop call "uvx mcp-server-time" get_current_time timezone=UTC
```

Arguments are `key=value` pairs (each parsed as JSON, falling back to a string),
or a single `--json '{...}'` object.

## Interactive mode

Run `mctop <target>` with no subcommand to open the full-screen client: browse
tools, resources, and prompts; press enter on a tool to fill its arguments in a
schema-driven form and run it; read the result, then go again.

```
↑↓ move    enter open    / search    tab switch section    ? keys    q quit
```

Results are shown as an insight view that reads the data: field names become
plain labels, values are formatted by what they are (dates, yes/no, status,
links, grouped numbers), nested objects become sections, arrays of objects
become tables, and short fields flow into columns to fill the screen. When the
result is a list of records (a bare array, or one wrapped in a `{status, total,
data: [...]}` envelope), `↑`/`↓` select a row and `enter` expands it into a full,
untruncated view of that record; `esc` collapses back to the list. A wide record
shows a scannable few columns in the table and the rest on expand. Press `t` for
the colored JSON, `y` to copy. `r` re-runs, `e` edits the arguments, `esc` (or
`←`) goes back.

Vim motions (`h`/`j`/`k`/`l`, `g`/`G`) are on by default; `V` toggles them and the
choice is remembered. The arrow keys always work either way. Press `?` for the
full list of keys.

## Authentication

For OAuth-protected servers, log in once and mctop handles the token after that:

```
mctop login  https://api.example.com/mcp   # opens the browser, caches the token
mctop ls     https://api.example.com/mcp   # uses it automatically
mctop logout https://api.example.com/mcp   # forgets it
```

The token is cached per host and refreshed as needed, so you log in once per
server, not per command. For servers that take a static token instead, pass it
with `-H` (repeatable):

```
mctop ls https://api.example.com/mcp -H "Authorization: Bearer $TOKEN"
```

## Testing in CI

A spec describes what a server must expose and how its calls must behave. `mctop
test` exits non-zero when the contract breaks, so it gates a build.

```yaml
server:
  url: "https://api.example.com/mcp"
  headers:
    Authorization: "Bearer ${TOKEN}"   # expanded from the environment
expect:
  tools: [search, fetch]
calls:
  - tool: search
    args: { query: "hello" }
    assert:
      not_error: true
      contains: "results"
```

```
mctop test spec.yaml --report json
```

## Examples

The `examples/` directory has runnable specs and a small demo server you can
point mctop at:

```
go run ./examples/demoserver
mctop ls http://localhost:8080/mcp
```

## License

MIT, see [LICENSE](./LICENSE).
