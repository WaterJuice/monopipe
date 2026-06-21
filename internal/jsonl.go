// ---------------------------------------------------------------------------------------
//
//	jsonl.go
//	--------
//
//	The shared JSONL record format used by both modes. Each record is a single JSON object
//	on its own line. The capture side always emits exactly one key per object; the split
//	side is lenient and accepts extra keys, multiple keys, or none.
//
//	Recognised keys:
//	    start        — array of strings, the child argv (program + args)
//	    stdout       — string, a chunk of standard output (valid UTF-8)
//	    stdout_b64   — string, base64 of a chunk of standard output (non-UTF-8 bytes)
//	    stderr       — string, a chunk of standard error (valid UTF-8)
//	    stderr_b64   — string, base64 of a chunk of standard error (non-UTF-8 bytes)
//	    retcode      — integer, the child's exit code
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
	"encoding/base64"
	"encoding/json"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

// Recognised JSONL record keys.
const (
	keyStart     = "start"
	keyStdout    = "stdout"
	keyStdoutB64 = "stdout_b64"
	keyStderr    = "stderr"
	keyStderrB64 = "stderr_b64"
	keyRetcode   = "retcode"
)

// ---------------------------------------------------------------------------------------
//
//	Record Marshalling
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// marshalRecord serialises a single-key JSONL record. The result has no trailing newline.
func marshalRecord(key string, value any) ([]byte, error) {
	return json.Marshal(map[string]any{key: value})
}

// ---------------------------------------------------------------------------------------
//
//	Stream Encoder
//
// ---------------------------------------------------------------------------------------

// streamKeys pairs the text and base64 record keys used for one output channel. It is the
// single source of truth for those key names, shared by capture (which produces records)
// and split (which consumes them).
type streamKeys struct {
	text string
	b64  string
}

// Record key pairs for the child's two output channels.
var (
	stdoutKeys = streamKeys{text: keyStdout, b64: keyStdoutB64}
	stderrKeys = streamKeys{text: keyStderr, b64: keyStderrB64}
)

// streamEncoder turns successive byte chunks from a single output stream into JSONL
// records. A chunk that is valid UTF-8 is emitted as text under the channel's text key;
// otherwise it is base64-encoded under its b64 key. To avoid spuriously splitting a
// multi-byte rune across a read boundary (which would force base64 on otherwise-readable
// text), an incomplete trailing UTF-8 sequence is carried over and prepended to the next
// chunk.
type streamEncoder struct {
	keys  streamKeys
	carry []byte
}

// ---------------------------------------------------------------------------------------
// newStreamEncoder creates an encoder for a channel identified by its key pair.
func newStreamEncoder(keys streamKeys) *streamEncoder {
	return &streamEncoder{keys: keys}
}

// ---------------------------------------------------------------------------------------
// encode consumes a chunk and returns a marshalled JSONL record, or (nil, nil) if the
// chunk produced no output (all bytes held back as an incomplete trailing rune).
func (e *streamEncoder) encode(chunk []byte) ([]byte, error) {
	buf := chunk
	if len(e.carry) > 0 {
		buf = append(e.carry, chunk...)
		e.carry = nil
	}

	// Hold back an incomplete trailing UTF-8 sequence for the next chunk.
	if n := incompleteTrailing(buf); n > 0 {
		e.carry = append(e.carry, buf[len(buf)-n:]...)
		buf = buf[:len(buf)-n]
	}

	return e.emit(buf)
}

// ---------------------------------------------------------------------------------------
// flush emits any bytes still held in the carry buffer. Called once at end of stream.
func (e *streamEncoder) flush() ([]byte, error) {
	if len(e.carry) == 0 {
		return nil, nil
	}
	buf := e.carry
	e.carry = nil
	return e.emit(buf)
}

// ---------------------------------------------------------------------------------------
// emit marshals buf as a text record when it is valid UTF-8, otherwise as base64.
func (e *streamEncoder) emit(buf []byte) ([]byte, error) {
	if len(buf) == 0 {
		return nil, nil
	}
	if utf8.Valid(buf) {
		return marshalRecord(e.keys.text, string(buf))
	}
	return marshalRecord(e.keys.b64, base64.StdEncoding.EncodeToString(buf))
}

// ---------------------------------------------------------------------------------------
// incompleteTrailing returns the number of trailing bytes that form a valid but unfinished
// prefix of a multi-byte UTF-8 sequence (0 if the buffer ends on a rune boundary). Such
// bytes are held back so a rune split across a read boundary is not forced into base64.
func incompleteTrailing(buf []byte) int {
	// Scan back to the start of the last rune (at most UTFMax bytes away), then ask whether
	// it is fully present. utf8.FullRune is false only for a valid-but-truncated prefix —
	// exactly the bytes worth carrying over; invalid bytes count as "full" and are emitted
	// as-is (emit then routes them to base64).
	for i := len(buf) - 1; i >= 0 && i >= len(buf)-utf8.UTFMax; i-- {
		if utf8.RuneStart(buf[i]) {
			if utf8.FullRune(buf[i:]) {
				return 0
			}
			return len(buf) - i
		}
	}
	return 0
}
