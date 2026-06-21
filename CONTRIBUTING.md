# Contributing to mctop

Thanks for helping out. mctop is a single Go binary; the loop is small.

## Build and test

```
go build ./...
go test ./...
go vet ./...
```

`go run . <target>` runs your working copy against a server. The `examples/`
directory has a demo server to point it at:

```
go run ./examples/demoserver        # serves :8080
go run . http://localhost:8080/mcp
```

## Layout

- `internal/mcp` wraps the official Go SDK into the surface mctop needs.
- `internal/tui` is the interactive client.
- `internal/cli` holds the headless subcommands (`ls`, `call`, `test`, ...).
- `internal/spec` parses and runs CI contracts.
- `DESIGN.md` is the why: the problem, the non-goals, and the shape. Read it
  before a large change so a feature lands where it belongs.

## Pull requests

- Keep each PR to one concern, with a clear title that says the intent.
- Add a test for behavior you change, especially in `internal/mcp` and
  `internal/spec` where a wrong result would lie to someone's CI.
- `go test ./...` and `go vet ./...` must pass. CI runs both on every PR.
- Match the surrounding style. No new dependency without a reason in the PR.

## Reporting bugs

Open an issue with the version (`mctop version`), the target, and the protocol
trace (press `T` in the TUI) when a call behaves unexpectedly. Redact any tokens.
