// ---------------------------------------------------------------------------------------
//
//	capture.go
//	----------
//
//	Capture mode. Runs a child command (invoked like the `time` command), forwards stdin
//	to it, and writes its stdout, stderr, and exit code to monopipe's stdout as a single
//	ordered JSONL stream. A start record carries the child argv; a retcode record carries
//	the exit code.
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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"sync"
)

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

// readBufSize is the buffer size used when reading from the child's output pipes.
const readBufSize = 32 * 1024

// maxRecordBytes caps the output carried by a single JSONL record. A record holds one
// text line (up to and including its newline) or up to this many bytes, whichever comes
// first — so the stream stays readable line-by-line while memory stays bounded.
const maxRecordBytes = 512

// ---------------------------------------------------------------------------------------
//
//	Capture
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// runCapture runs argv, streams its output as JSONL to stdout, and returns monopipe's own
// exit code. The child's exit code travels inside the stream (as a retcode record), so a
// successful capture returns 0; a non-zero return here means monopipe itself failed (for
// example, the command could not be started).
func runCapture(argv []string) int {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = os.Stdin

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "monopipe: %v\n", err)
		return 1
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "monopipe: %v\n", err)
		return 1
	}

	// Single writer goroutine serialises records to stdout so the JSONL is always
	// well-formed (no interleaved partial lines) and is flushed line-by-line. The first
	// write error is recorded so a truncated stream is reported rather than passed off as
	// a successful capture; the loop keeps draining afterwards so senders never block.
	events := make(chan []byte, 64)
	writerDone := make(chan struct{})
	var writeErr error
	go func() {
		w := bufio.NewWriter(os.Stdout)
		for rec := range events {
			if writeErr != nil {
				continue
			}
			if _, err := w.Write(rec); err != nil {
				writeErr = err
			} else if err := w.WriteByte('\n'); err != nil {
				writeErr = err
			} else if err := w.Flush(); err != nil {
				writeErr = err
			}
		}
		close(writerDone)
	}()

	sendRecord := func(key string, value any) {
		if rec, err := marshalRecord(key, value); err == nil {
			events <- rec
		}
	}

	if err := cmd.Start(); err != nil {
		close(events)
		<-writerDone
		fmt.Fprintf(os.Stderr, "monopipe: cannot run %q: %v\n", argv[0], err)
		if errors.Is(err, fs.ErrPermission) {
			return 126 // found but not executable
		}
		return 127 // not found (or other start failure)
	}

	// The start record only appears for commands that actually started.
	sendRecord(keyStart, argv)

	var wg sync.WaitGroup
	wg.Add(2)
	go pumpStream(stdoutPipe, newStreamEncoder(stdoutKeys), events, &wg)
	go pumpStream(stderrPipe, newStreamEncoder(stderrKeys), events, &wg)
	wg.Wait()

	waitErr := cmd.Wait()
	code := processExitCode(cmd.ProcessState, waitErr)
	sendRecord(keyRetcode, code)

	close(events)
	<-writerDone
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "monopipe: error writing output: %v\n", writeErr)
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------------------
// pumpStream reads r, frames its bytes into records (one text line, or up to
// maxRecordBytes when a line is longer or no newline has arrived yet), encodes each via
// enc, and sends them to the events channel until end of stream.
func pumpStream(r io.Reader, enc *streamEncoder, events chan<- []byte, wg *sync.WaitGroup) {
	defer wg.Done()

	send := func(frame []byte) {
		if rec, e := enc.encode(frame); e == nil && rec != nil {
			events <- rec
		}
	}

	buf := make([]byte, readBufSize)
	var pending []byte
	for {
		n, err := r.Read(buf)
		if n > 0 {
			pending = emitFrames(append(pending, buf[:n]...), send)
		}
		if err != nil {
			break
		}
	}

	// Emit whatever is left at EOF, still respecting the record size cap.
	for len(pending) > 0 {
		k := min(len(pending), maxRecordBytes)
		send(pending[:k])
		pending = pending[k:]
	}
	if rec, e := enc.flush(); e == nil && rec != nil {
		events <- rec
	}
}

// ---------------------------------------------------------------------------------------
// emitFrames sends every complete record at the front of data — a line up to and including
// its newline, or a maxRecordBytes-sized chunk when a line is longer or carries no newline
// yet — and returns the unconsumed remainder to carry into the next read.
func emitFrames(data []byte, send func([]byte)) []byte {
	for {
		if nl := bytes.IndexByte(data, '\n'); nl >= 0 && nl < maxRecordBytes {
			send(data[:nl+1])
			data = data[nl+1:]
			continue
		}
		if len(data) >= maxRecordBytes {
			send(data[:maxRecordBytes])
			data = data[maxRecordBytes:]
			continue
		}
		return data
	}
}
