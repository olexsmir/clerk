//go:build debug

package cli

import (
	"fmt"
	"net/http"
	"os"

	_ "net/http/pprof"
)

func init() {
	addr := os.Getenv("CLERK_PPROF")
	if addr == "" {
		addr = "localhost:6969"
	}
	go func() {
		fmt.Printf("pprof: http://%s/debug/pprof\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			fmt.Fprintf(os.Stderr, "pprof: %v\n", err)
		}
	}()
}
