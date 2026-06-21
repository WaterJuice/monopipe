// ---------------------------------------------------------------------------------------
//
//	split.go
//	--------
//
//	Split mode. Reads the JSONL record stream produced by capture mode on stdin and
//	demultiplexes it back into the real stdout and stderr, then exits with the recorded
//	return code so monopipe is transparent in a pipeline.
//
//	Parsing is deliberately lenient: unrecognised keys are ignored, an object with several
//	recognised keys has each applied in turn, and an object (or line) with none of the
//	recognised keys is skipped.
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
	"errors"
	"fmt"
	"io"
	"os"
)

// ---------------------------------------------------------------------------------------
//
//	Split
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// runSplit reads JSONL on stdin and writes each chunk to the appropriate channel. It
// returns the last recorded retcode (0 if none was seen), or a non-zero monopipe error
// code if the input stream itself is malformed.
func runSplit() int {
	dec := json.NewDecoder(os.Stdin)
	retcode := 0

	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			fmt.Fprintf(os.Stderr, "monopipe: malformed JSONL input: %v\n", err)
			return 1
		}

		// A well-formed JSON value that is not an object (e.g. a number or array) is
		// simply skipped rather than treated as an error.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}

		if code, err := applyRecord(obj); err != nil {
			fmt.Fprintf(os.Stderr, "monopipe: %v\n", err)
			return 1
		} else if code != nil {
			retcode = *code
		}
	}

	return retcode
}

// splitChannels maps each output channel's key pair to the writer it replays to. Sharing
// streamKeys with capture keeps the producer and consumer in step.
var splitChannels = []struct {
	keys streamKeys
	w    io.Writer
}{
	{stdoutKeys, os.Stdout},
	{stderrKeys, os.Stderr},
}

// ---------------------------------------------------------------------------------------
// applyRecord writes the output carried by a single record's recognised keys, in a fixed
// order. It returns a non-nil pointer when the record carried a retcode.
func applyRecord(obj map[string]json.RawMessage) (*int, error) {
	for _, ch := range splitChannels {
		if v, ok := obj[ch.keys.text]; ok {
			if err := writeText(ch.w, v); err != nil {
				return nil, err
			}
		}
		if v, ok := obj[ch.keys.b64]; ok {
			if err := writeBase64(ch.w, v); err != nil {
				return nil, err
			}
		}
	}
	if v, ok := obj[keyRetcode]; ok {
		var code int
		if err := json.Unmarshal(v, &code); err == nil {
			return &code, nil
		}
	}
	return nil, nil
}

// ---------------------------------------------------------------------------------------
// writeText decodes a JSON string value and writes its bytes to w. A non-string value for
// a text key is leniently skipped.
func writeText(w io.Writer, raw json.RawMessage) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	_, err := io.WriteString(w, s)
	return err
}

// ---------------------------------------------------------------------------------------
// writeBase64 decodes a base64 JSON string value and writes the raw bytes to w. A decode
// failure is reported as an error, since it indicates corrupt input.
func writeBase64(w io.Writer, raw json.RawMessage) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("invalid base64 in input: %w", err)
	}
	_, err = w.Write(data)
	return err
}
