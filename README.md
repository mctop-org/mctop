# mctop

[![release](https://img.shields.io/github/v/release/mctop-org/mctop?label=release)](https://github.com/mctop-org/mctop/releases)
[![ci](https://img.shields.io/github/actions/workflow/status/mctop-org/mctop/ci.yml?branch=main&label=ci)](https://github.com/mctop-org/mctop/actions/workflows/ci.yml)
[![license](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

A terminal client for MCP servers. Connect to any server, browse its tools,
resources, and prompts, call them, and watch the result, without leaving the
shell. Then assert the server's contract in CI so a renamed tool or a drifted
schema fails the build instead of breaking an agent in production.

Think `curl` and `k9s`, but for the Model Context Protocol.

![mctop browsing a server, calling a tool, and showing the protocol trace](./docs/demo.gif)

## What it does

- **Explore** a server interactively: browse tools, resources, and prompts, fill
  a tool's arguments in a schema-driven form, and read the result.
- **Script** it headless: `mctop ls` to list, `mctop call` for one-shot calls.
- **Test** it in CI: `mctop test spec.yaml` runs a contract and exits non-zero
  when it breaks.

## Install

```
curl -fsSL https://mctop.org/install | sh
```

Or with Homebrew, or the Go toolchain:

```
brew install mctop-org/tap/mctop
go install github.com/mctop-org/mctop@latest
```

Then `mctop upgrade` keeps it current, however you installed it.

## Usage

```
mctop <target>                 open the interactive client against a server
mctop ls <target>              list tools, resources, and prompts
mctop call <target> <tool>     call one tool and print the result
mctop login <url>              log in to an OAuth-protected server
mctop test <spec.yaml>         run a contract, exit 0 on pass, 1 on fail
mctop record <target>          browse a server and save the calls as a spec
mctop upgrade                  update to the latest release
```

A target is either a command to spawn (`"uvx mcp-server-time"`) or an
`http(s)://` URL. For an older server that needs the legacy SSE transport, add
`--sse`.

```
mctop call "uvx mcp-server-time" get_current_time timezone=UTC
```

Arguments are `key=value` pairs (values that look like JSON, such as numbers,
booleans, arrays, and objects, are typed; anything else is a string), or a
single `--json '{...}'` object.

## Interactive mode

Run `mctop <target>` with no subcommand to open the full-screen client: browse
tools, resources, and prompts; press enter on a tool to fill its arguments in a
schema-driven form and run it; read the result, then go again.

```
↑↓ move    enter open    / search    tab switch section    T trace    ? keys    q quit
```

Results render as readable fields and tables instead of raw JSON. When the
result is a list of records, `↑`/`↓` select a row and `enter` expands it into a
full, untruncated view; `esc` collapses back to the list. Press `t` for the raw
JSON, `y` to copy, `r` to re-run, `e` to edit the arguments, and `esc` (or `←`)
to go back.

Press `T` to see the raw protocol: every JSON-RPC frame that crossed the wire,
each tagged with its direction, method, and time, above its JSON. It reads like
the web Inspector's network pane, without leaving the terminal, so you can see
exactly what a server sent back when a call surprises you.

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
  # sse: true                          # for a legacy HTTP+SSE server
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

You don't have to write a spec by hand: `mctop record <target> -o spec.yaml`
opens the interactive client and saves every tool call you make as a spec step,
asserting the error status it observed. Headers passed with `-H` are written as
`$NAME` environment references, so the file is safe to commit. Sharpen the
recorded steps with `contains` assertions where it strengthens the contract.

## Examples

The `examples/` directory has runnable specs and a small demo server you can
point mctop at:

```
go run ./examples/demoserver
mctop ls http://localhost:8080/mcp
```

## License

MIT, see [LICENSE](./LICENSE).
