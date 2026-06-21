# CLAUDE.md

This file provides guidance for AI agents working on this project.

## Project Overview

monopipe is a command-line tool with two modes of operation:

1. **Capture** — invoked like the `time` command (`monopipe COMMAND [ARGS...]`). It runs the command, forwarding its own stdin through to the child, captures the child's stdout and stderr, and writes a single ordered JSONL record stream to monopipe's stdout. Each record carries the command (`start`), a chunk of output (`stdout`/`stderr`), or the exit code (`retcode`).
2. **Split** — invoked as `monopipe -s` / `--split`. It reads that JSONL stream on stdin and demultiplexes it back into the real stdout and stderr, then exits with the recorded `retcode` so monopipe is transparent in a pipeline.

The project is written in Go and distributed as platform-specific Python wheels via PyPI (using bin2whl). Users install it with `pip install monopipe` or `uvx monopipe`, but the binary is a statically-linked Go executable — no Python runtime required at execution time.

## Language and Spelling

Use **Australian English** throughout:

- colour (not color)
- initialise (not initialize)
- serialise (not serialize)
- licence (noun)

## Code Style

### Go Files

Every Go file should have:

1. A file header block with description and version history
2. Section headers separating major sections (Imports, Constants, Functions, etc.)
3. Horizontal separators (87 dashes after `// `, 90 chars total) above each function definition

Example structure:

```go
// ---------------------------------------------------------------------------------------
//
//	filename.go
//	-----------
//
//	Brief description of what this module does.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Jun 2026 - Created
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"fmt"
)

// ---------------------------------------------------------------------------------------
//
//	Functions
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// MyFunction does something.
func MyFunction() {
}
```

### General

- Go 1.25+
- Zero external dependencies — stdlib only
- Use `gofmt` for formatting, `go vet` for linting
- Run `make format` to auto-fix formatting
- Run `make check` to verify formatting and lint
- CLI uses manual argument parsing (no flag package, no external CLI libs)
- TTY-aware ANSI colours for terminal output

## Common Commands

```bash
make help       # Show all available targets
make check      # Run gofmt check + go vet
make format     # Auto-format Go source with gofmt
make go-build   # Cross-compile for all 6 platforms
make build      # Full build: check, go-build, docs, platform wheels
make docs       # Build HTML documentation into html/
make clean      # Remove build artefacts
make dev        # Set up .venv + a go-run launcher (runs from source, no rebuild)
make run ARGS="sh -c 'echo hi'"   # Run from source via `go run` with arguments
```

## Project Structure

```
main.go                     # Entry point, calls internal.Run(Version)
go.mod                      # Go module definition
internal/
├── cli.go                  # CLI argument parsing, help text, mode dispatch, TTY helpers
├── capture.go              # Capture mode: run child, stream JSONL to stdout
├── split.go                # Split mode: read JSONL on stdin, demultiplex to stdout/stderr
├── jsonl.go                # Shared record keys + per-stream text/base64 encoder
├── exitcode_unix.go        # Exit-code derivation (POSIX 128+signal) for non-Windows
└── exitcode_windows.go     # Exit-code derivation for Windows
wheel.json                  # bin2whl configuration for platform wheels
pyproject.toml              # Minimal — just for uv dev dependencies
Makefile                    # Build orchestration
```

## Architecture

### JSONL Format

One JSON object per line. The capture side always emits exactly one key per object; the split side is lenient and accepts extra keys, multiple recognised keys, or none.

| Key                         | Value                                              |
| --------------------------- | -------------------------------------------------- |
| `start`                     | array of strings — the child argv (program + args) |
| `stdout` / `stderr`         | string — a chunk of output, when valid UTF-8       |
| `stdout_b64` / `stderr_b64` | string — base64 of the raw chunk bytes, otherwise  |
| `retcode`                   | integer — the child's exit code                    |

Order: `start` first, then `stdout`/`stderr` interleaved in arrival order, then `retcode`.

### Capture Mode (capture.go)

- Runs the child via `os/exec`, with `cmd.Stdin = os.Stdin` (forwarded, not recorded)
- One goroutine per output pipe reads from it and frames the bytes into records — one text line (up to its newline) or up to 512 bytes (`maxRecordBytes`), whichever comes first
- A single writer goroutine serialises records to stdout via a channel — guarantees well-formed JSONL with no interleaved partial lines, flushed line-by-line for streaming
- Line/512-byte framing keeps records readable while bounding memory for large output
- Exit code via `ProcessState.ExitCode()`; signal-terminated children yield 128+signal (Unix)
- monopipe itself exits 0 on a successful capture; non-zero only on its own failure

### Split Mode (split.go)

- Streams JSON values from stdin with `json.NewDecoder` (no line-length limit)
- Each recognised key is applied in a fixed order; output written directly to os.Stdout / os.Stderr for correct ordering and real-time replay
- Lenient: unrecognised keys ignored, non-object JSON values skipped, no-key objects skipped
- Malformed JSON or invalid base64 → error to stderr, non-zero exit
- Exits with the last seen `retcode` (0 if absent)

### Encoding (jsonl.go)

- `streamEncoder` chooses text (`utf8.Valid`) vs base64 per chunk
- Holds back an incomplete trailing UTF-8 sequence (carry buffer) so multi-byte runes straddling a read boundary do not spuriously trigger the base64 fallback; flushed at EOF

## Key Design Decisions

1. **Go binary, PyPI distribution** — statically-linked Go binary wrapped in platform wheels via bin2whl
2. **Zero dependencies** — Go stdlib only
3. **Single ordered stream** — both channels plus exit code in one JSONL stream, serialised through a single writer goroutine
4. **Readable when possible, faithful always** — UTF-8 text stored as strings, binary as base64, byte-exact round-trip
5. **Lenient split parsing** — easy to hand-author or post-process streams
6. **Manual CLI parsing** — no flag package; monopipe options precede the command, `--` ends them
7. **Platform build tags** — exitcode_unix.go / exitcode_windows.go for signal handling

## Build & Distribution

- Cross-compiled for 6 platforms: macOS (arm64/amd64), Linux (arm64/amd64), Windows (arm64/amd64)
- All binaries are statically linked (`CGO_ENABLED=0`)
- Version injected at build time via `-ldflags -X main.Version=...`
- Platform wheels built using `bin2whl` from `wheel.json` config
- Published to PyPI via `wj-publish`

## Testing Changes

After making changes:

1. Run `make check` to verify formatting and vet pass
2. Run `make go-build` to verify cross-compilation works
3. Test a round-trip: capture a command, then split the stream back and confirm stdout, stderr, and the exit code are reproduced

## Versioning

- Version is derived from git tags via `git describe --tags --always`
- Create a tag like `1.0.0` before running `make build` for a release (no `v` prefix)
- Version is injected at build time via `-ldflags`
- Falls back to "dev" if no tags exist

## Commits

When committing:

- Use clear, descriptive commit messages
- Include `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>` in commits made with AI assistance
- **Never rewrite git history** unless explicitly asked to

## Licence

Released under the [Unlicense](https://unlicense.org/) — public domain.
