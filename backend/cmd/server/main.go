package main

import "os"

// Exit codes following the ESLint/yamllint/Ruff convention.
const (
	exitOK        = 0
	exitFindings  = 1
	exitToolError = 2
)

func main() {
	if len(os.Args) < 2 {
		runServe(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "analyze":
		os.Exit(runAnalyze(os.Args[2:]))
	case "serve":
		runServe(os.Args[2:])
	default:
		// If the first arg doesn't match a subcommand, default to serve
		// for backward compatibility.
		runServe(os.Args[1:])
	}
}
