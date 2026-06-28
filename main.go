package main

import (
	"context"
	"fmt"
	"os"

	"olexsmir.xyz/clerk/internal/cli"
)

// NOTE: sets during build
// go build -ldflags="-X 'main.version=v1.0.0'"
var version = "develop"

func main() {
	if err := cli.New(version).Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
