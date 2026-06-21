// ---------------------------------------------------------------------------------------
//
//	exitcode_unix.go
//	----------------
//
//	Exit-code derivation for Unix-like platforms (macOS, Linux). When the child is killed
//	by a signal it has no ordinary exit code, so the POSIX 128+signal convention is used.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Jun 2026 - Created
//
// ---------------------------------------------------------------------------------------

//go:build !windows

package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"os"
	"syscall"
)

// ---------------------------------------------------------------------------------------
//
//	Functions
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// processExitCode returns the exit code for a finished child process. A normal exit yields
// its status; a signal-terminated child yields 128+signal; anything else yields 1.
func processExitCode(state *os.ProcessState, waitErr error) int {
	if state != nil {
		if code := state.ExitCode(); code >= 0 {
			return code
		}
		if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal())
		}
	}
	if waitErr != nil {
		return 1
	}
	return 0
}
