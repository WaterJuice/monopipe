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

// chunkSize is the read buffer size for draining the child's output pipes. One JSONL
// record is emitted per non-empty read, so memory stays bounded for large output.
const chunkSize = 32 * 1024

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
// pumpStream reads chunks from r, encodes each as a JSONL record via enc, and sends them
// to the events channel until end of stream.
func pumpStream(r io.Reader, enc *streamEncoder, events chan<- []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, chunkSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if rec, e := enc.encode(buf[:n]); e == nil && rec != nil {
				events <- rec
			}
		}
		if err != nil {
			break
		}
	}
	if rec, e := enc.flush(); e == nil && rec != nil {
		events <- rec
	}
}
