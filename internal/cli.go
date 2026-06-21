// ---------------------------------------------------------------------------------------
//
//	cli.go
//	------
//
//	CLI argument parsing, help text, and mode dispatch. monopipe has two modes: capture
//	(run a command and emit JSONL) and split (read JSONL on stdin and demultiplex it).
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
	"os"
	"strings"
)

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

const licenceText = `monopipe — Released under the Unlicense (public domain)

This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or
distribute this software, either in source code form or as a compiled
binary, for any purpose, commercial or non-commercial, and by any
means.

For more information, please refer to <https://unlicense.org/>
`

// ---------------------------------------------------------------------------------------
//
//	Run
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// Run is the main entry point called from main.go. It parses monopipe's own options
// (which must precede any command), then dispatches to capture or split mode.
func Run(version string) {
	args := os.Args[1:]

	split := false
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			// Explicit end of monopipe options; the command follows.
			i++
			break
		}
		switch arg {
		case "-h", "--help":
			printUsage()
			return
		case "--version":
			fmt.Printf("monopipe: %s\n", version)
			return
		case "--license":
			fmt.Print(licenceText)
			return
		case "-s", "--split":
			split = true
			i++
			continue
		}
		// First token that is not a monopipe option begins the command.
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintf(os.Stderr, "Unknown option: %s\n", arg)
			fmt.Fprintln(os.Stderr, "To run a command beginning with '-', precede it with '--'.")
			fmt.Fprintln(os.Stderr, "Run with --help for usage")
			os.Exit(1)
		}
		break
	}

	command := args[i:]

	if split {
		if len(command) > 0 {
			fmt.Fprintln(os.Stderr, "Error: --split takes no command (it reads JSONL on stdin)")
			os.Exit(1)
		}
		os.Exit(runSplit())
	}

	if len(command) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no command given to run")
		fmt.Fprintln(os.Stderr, "Run with --help for usage")
		os.Exit(1)
	}

	os.Exit(runCapture(command))
}

// ---------------------------------------------------------------------------------------
//
//	Help Text
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// printUsage prints the help text with TTY-aware ANSI colour codes.
func printUsage() {
	tty := isTerminal(os.Stdout)
	if tty {
		h := "\033[1;34m"
		p := "\033[1;35m"
		s := "\033[32m"
		l := "\033[36m"
		m := "\033[33m"
		S := "\033[1;32m"
		L := "\033[1;36m"
		M := "\033[1;33m"
		r := "\033[0m"

		fmt.Printf("%susage: %s%smonopipe%s [%s-h%s] [%s--version%s] [%s--license%s] (%s-s%s | %sCOMMAND%s [%sARGS%s...])\n",
			h, r, p, r, s, r, l, r, l, r, s, r, m, r, m, r)
		fmt.Println()
		fmt.Println("Multiplex a command's stdout, stderr, and exit code into a single JSONL")
		fmt.Println("stream — and demultiplex it back into the original channels.")
		fmt.Println()
		fmt.Printf("%smodes:%s\n", h, r)
		fmt.Printf("  %smonopipe%s %sCOMMAND%s [%sARGS%s...]   run COMMAND, write JSONL records to stdout\n", p, r, M, r, M, r)
		fmt.Printf("  %smonopipe%s %s-s%s                 read JSONL on stdin, split to stdout/stderr\n", p, r, S, r)
		fmt.Println()
		fmt.Printf("%soptions:%s\n", h, r)
		fmt.Printf("  %s-h%s, %s--help%s      show this help message and exit\n", S, r, L, r)
		fmt.Printf("  %s--version%s       show version and exit\n", L, r)
		fmt.Printf("  %s--license%s       show licence information and exit\n", L, r)
		fmt.Printf("  %s-s%s, %s--split%s     split mode: demultiplex JSONL from stdin\n", S, r, L, r)
		fmt.Println()
		fmt.Printf("  %s--%s              end monopipe options; everything after is the command\n", L, r)
	} else {
		fmt.Println("usage: monopipe [-h] [--version] [--license] (-s | COMMAND [ARGS...])")
		fmt.Println()
		fmt.Println("Multiplex a command's stdout, stderr, and exit code into a single JSONL")
		fmt.Println("stream — and demultiplex it back into the original channels.")
		fmt.Println()
		fmt.Println("modes:")
		fmt.Println("  monopipe COMMAND [ARGS...]   run COMMAND, write JSONL records to stdout")
		fmt.Println("  monopipe -s                  read JSONL on stdin, split to stdout/stderr")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help      show this help message and exit")
		fmt.Println("  --version       show version and exit")
		fmt.Println("  --license       show licence information and exit")
		fmt.Println("  -s, --split     split mode: demultiplex JSONL from stdin")
		fmt.Println()
		fmt.Println("  --              end monopipe options; everything after is the command")
	}
}

// ---------------------------------------------------------------------------------------
//
//	Terminal / Colour Helpers
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// isTerminal reports whether the given file descriptor is connected to a terminal (TTY).
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
