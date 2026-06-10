package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "fmt", "format":
		runFormat(os.Args[2:])
	case "tags":
		runTags(os.Args[2:])
	case "lint":
		runLint(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: clerk <command> [options]

Commands:
  tags       Generate ctags-compatible tag file
  format     Format journal files
  lint       Lint journal files
`)
}
