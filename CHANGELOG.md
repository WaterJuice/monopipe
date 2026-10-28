# monopipe 1.0.0 - 28 Oct 2026

First stable release, promoting 1.0.0 Beta 1 unchanged apart from the toolchain.

- Built with Go 1.27.3 to pick up the fix for the eMudura TLS issue.
- `go.mod` now requires Go 1.27.3 (was 1.25.0), so builds from source can't fall back to an older toolchain without the fix. This also brings in Go 1.27's default runtime (`GODEBUG`) settings.

# monopipe 1.0.0 Beta 1 - 21 Jun 2026

- Initial release
- Capture mode: run a command like `time`, multiplexing its stdout, stderr, and exit code into a single ordered JSONL stream on stdout
- Split mode (`-s`): read that JSONL stream on stdin and demultiplex it back into real stdout and stderr, mirroring the recorded return code so monopipe stays transparent in a pipeline
- One JSONL record per output line, or per 512 bytes when a line is longer, emitted through a single serialised writer — readable line-by-line, well-formed, streamed in real time, bounded memory for large output
- Output kept as human-readable text when valid UTF-8, with byte-exact base64 fallback (`stdout_b64` / `stderr_b64`) for arbitrary binary output
- Incomplete trailing UTF-8 sequences are carried across read boundaries, so multi-byte runes never spuriously trigger the base64 fallback
- Child stdin is forwarded in capture mode, so interactive programs and filters work
- Lenient split parsing: ignores unrecognised keys, applies multiple recognised keys per line, and skips lines carrying none
- Signal-terminated children report a POSIX 128+signal exit code (Unix)
- Written in Go with zero external dependencies — stdlib only
- Distributed as platform-specific Python wheels via bin2whl (`pip install monopipe`)
- Cross-compiled for macOS (arm64/amd64), Linux (arm64/amd64), Windows (arm64/amd64)
- Statically linked single binary — no Python runtime needed at execution time
