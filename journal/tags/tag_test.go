package tags

import (
	"bytes"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestTags(t *testing.T) {
	tests := []string{"basic", "kinds", "duplicates", "crossfile"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			t.Parallel()
			a := golden.Read(t, tt)
			fsys, err := a.FS()
			if err != nil {
				t.Fatal(err)
			}
			l := journal.NewLoader()
			rj, err := l.ResolveFS(fsys, "in.journal")
			if err != nil {
				t.Fatalf("loading journal: %v", err)
			}
			tagger := New(rj, "")
			var buf bytes.Buffer
			if err := tagger.Write(&buf); err != nil {
				t.Fatal(err)
			}
			golden.Assert(t, a, buf.String())
		})
	}
}
