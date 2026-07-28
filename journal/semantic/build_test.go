package semantic

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
			if _, err := l.LoadFS(fsys, "in.journal"); err != nil {
				t.Fatalf("LoadFS: %v", err)
			}

			var buf bytes.Buffer
			fprint(&buf, Build(l.Ordered()))
			golden.Assert(t, a, buf.String())
		})
	}
}
