package semantic

import (
	"bytes"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestBuild(t *testing.T) {
	tests := map[string][]string{
		"empty":   {},
		"journal": {"journal"},
		"include": {"include0", "include1"},
	}

	for name, inputs := range tests {
		t.Run(name, func(t *testing.T) {
			var files []*journal.ParsedFile
			for _, in := range inputs {
				l := journal.NewLoader()
				pf, err := l.LoadBytes(in+".journal", golden.Load(t, in))
				if err != nil {
					t.Fatalf("LoadBytes(%q): %v", in, err)
				}
				files = append(files, pf)
			}
			ctx := Build(files)

			var buf bytes.Buffer
			fprint(&buf, ctx)
			golden.Assert(t, name, buf.String())
		})
	}
}
