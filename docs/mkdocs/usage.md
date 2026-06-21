# Usage

monopipe has two modes: **capture** (run a command and emit JSONL) and **split** (read JSONL and demultiplex it).

```
--8<-- "_include/help_main.txt"
```

monopipe's own options must come before the command. Everything from the first non-option token onward is treated as the command and its arguments. Use `--` to end monopipe's option parsing explicitly — needed when the command itself begins with `-`.

## Capture mode

Put `monopipe` in front of any command, just like `time`:

```bash
monopipe ./build.sh --verbose > run.jsonl
```

monopipe runs the command, forwarding its own stdin through to the child, and writes a JSONL record stream to stdout. The child's stdin is **not** recorded.

```bash
# Capture to a file
monopipe ./job.sh > run.jsonl

# Pipe straight into another tool
monopipe ./job.sh | my-log-processor

# Wrap a command that starts with a dash
monopipe -- --weird-program

# Feed a filter its input; the echoed output is captured
echo "data" | monopipe ./filter.sh > run.jsonl
```

A successful capture exits `0` regardless of the child's exit code — the child's code is carried inside the stream as a `retcode` record. monopipe returns non-zero only when it fails itself (for example, exit `127` when the command cannot be started).

## Split mode

Pass `-s` (or `--split`) and feed the JSONL on stdin:

```bash
monopipe -s < run.jsonl
```

monopipe writes each `stdout` record to stdout and each `stderr` record to stderr, in order, then exits with the recorded `retcode`:

```bash
cat run.jsonl | monopipe -s
echo $?                # → the original command's exit code
```

This makes monopipe transparent: a captured-then-replayed command behaves like the original in a pipeline.

## The JSONL format

One JSON object per line. When **produced** by capture mode, each object carries exactly one key:

| Key                         | Value                                                    |
|-----------------------------|----------------------------------------------------------|
| `start`                     | array of strings — the command and its arguments         |
| `stdout` / `stderr`         | string — a chunk of output, when it is valid UTF-8       |
| `stdout_b64` / `stderr_b64` | string — base64 of the raw bytes, for non-UTF-8 output   |
| `retcode`                   | integer — the command's exit code                        |

Records appear in this order: `start` first, then `stdout` and `stderr` chunks interleaved in arrival order, then `retcode` last. One record is emitted per read chunk, so memory stays constant for large output and the stream is real-time.

### Text vs base64

Output that is valid UTF-8 is stored as a readable string under `stdout` / `stderr`. Output that is not (arbitrary binary) is base64-encoded under `stdout_b64` / `stderr_b64`, so the round-trip is always byte-exact. An incomplete multi-byte UTF-8 sequence that lands on a read boundary is carried over to the next chunk, so genuine text never spuriously falls back to base64.

### Lenient split parsing

When **consuming** JSONL, split mode is permissive — this makes it easy to hand-author or post-process streams:

- Unrecognised keys are ignored.
- An object with several recognised keys has each applied, in the fixed order above.
- An object (or any well-formed JSON value that is not an object) carrying none of the recognised keys is skipped.

Malformed JSON, or invalid base64 in a `*_b64` value, is reported as an error and monopipe exits non-zero.

## Exit codes

| Situation                          | monopipe exit code            |
|------------------------------------|-------------------------------|
| Capture, command ran              | `0` (child's code is in the stream) |
| Capture, command not found        | `127`                         |
| Capture, command not executable   | `126`                         |
| Capture, output stream broke mid-capture | `1` (the JSONL was truncated) |
| Capture, child killed by a signal | child's code is `128+signal` in the `retcode` record (Unix) |
| Split, stream consumed            | the recorded `retcode` (or `0` if none) |
| Split, malformed input            | `1`                           |

## Running with uv

```bash
# Run without installing
uvx monopipe -- ./job.sh

# Split a recorded stream
uvx monopipe -s < run.jsonl
```
