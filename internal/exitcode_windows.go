// ---------------------------------------------------------------------------------------
//
//	exitcode_windows.go
//	-------------------
//
//	Exit-code derivation for Windows, which has no POSIX signals. The process state's exit
//	code is used directly.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Jun 2026 - Created
//
// ---------------------------------------------------------------------------------------

//go:build windows

package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import "os"

// ---------------------------------------------------------------------------------------
//
//	Functions
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// processExitCode returns the exit code for a finished child process.
func processExitCode(state *os.ProcessState, waitErr error) int {
	if state != nil {
		if code := state.ExitCode(); code >= 0 {
			return code
		}
	}
	if waitErr != nil {
		return 1
	}
	return 0
}
