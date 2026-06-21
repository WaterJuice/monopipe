# monopipe

Multiplex a command's stdout, stderr, and exit code into a single ordered JSONL stream — and demultiplex it back into the original channels.

## Why?

When you capture a program's output to a file or send it down a pipe, you lose information: stdout and stderr collapse into one undifferentiated blob (or one of them is thrown away), the interleaving order is muddied, and the exit code vanishes entirely. monopipe records all of it — both channels, in arrival order, plus the return code — as one line-delimited JSON stream you can store, transport, or process, then replays it faithfully into real stdout and stderr later.

It runs like the `time` command: put `monopipe` in front of any command.

## Quick Start

### Install

```bash
pip install monopipe
```

Or run directly with uv:

```bash
uvx monopipe -- your-command --flag
```

### Capture

```bash
monopipe ./build.sh --verbose > run.jsonl
```

This runs `./build.sh --verbose` and writes a JSONL record stream to stdout:

```json
{"start":["./build.sh","--verbose"]}
{"stdout":"Compiling...\n"}
{"stderr":"warning: unused variable\n"}
{"stdout":"Done\n"}
{"retcode":0}
```

### Split

Feed that stream back in on stdin with `-s` to demultiplex it:

```bash
monopipe -s < run.jsonl        # stdout → stdout, stderr → stderr
echo $?                        # → the original command's exit code
```

## The Format

One JSON object per line, each (when produced by monopipe) carrying exactly one key:

| Key                         | Value                                                  |
| --------------------------- | ------------------------------------------------------ |
| `start`                     | array of strings — the command and its arguments       |
| `stdout` / `stderr`         | string — a chunk of output, when it is valid UTF-8     |
| `stdout_b64` / `stderr_b64` | string — base64 of the raw bytes, for non-UTF-8 output |
| `retcode`                   | integer — the command's exit code                      |

Text is kept human-readable in the common case; arbitrary binary output falls back to base64 so the round-trip is always byte-exact. Split mode is lenient: it ignores unrecognised keys, accepts multiple recognised keys on one line, and skips lines that carry none.

## How It Works

In capture mode, monopipe runs the child with its own stdin forwarded through (so interactive programs and filters still work), reads stdout and stderr concurrently, and emits one record per read chunk through a single serialised writer — so the JSONL is always well-formed and streamed in real time. The child's exit code is emitted last as a `retcode` record.

In split mode, monopipe reads the JSONL on stdin, writes each chunk to the matching channel, and exits with the recorded `retcode` so it stays transparent in a pipeline.

The binary is a statically-linked Go executable with zero external dependencies — distributed as platform-specific Python wheels via [bin2whl](https://pypi.org/project/bin2whl/), so `pip install monopipe` needs no Python runtime at execution time.

## Licence

Released under the [Unlicense](https://unlicense.org/) — public domain.
