package analyzer

import (
	"bytes"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestBuild(t *testing.T) {
	tests := []string{"empty", "journal", "include"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			a := golden.Read(t, tt)
			fsys, err := a.FS()
			if err != nil {
				t.Fatal(err)
			}

			l := journal.NewLoader()
			rj, err := l.ResolveFS(fsys, "in.journal")
			if err != nil {
				t.Fatalf("ResolveFS: %v", err)
			}

			var buf bytes.Buffer
			fprint(&buf, Build(rj))
			golden.Assert(t, a, buf.String())
		})
	}
}
