package tags

import (
	"bytes"
	"strings"
	"testing"
)

// ctags extended format: https://ctags.sourceforge.net/FORMAT

func TestWriterRoundTrip(t *testing.T) {
	w := NewWriter()
	w.DescribeKind('a', "account")
	w.DescribeKind('c', "commodity")
	w.Add(Entry{Name: "expenses:food", Kind: 'a', File: "test.journal", Line: 3, Pattern: "account expenses:food", Language: "hledger"})
	w.Add(Entry{Name: "$", Kind: 'c', File: "test.journal", Line: 9, Pattern: "    expenses:food   $50.00", Language: "hledger"})
	w.Add(Entry{Name: "assets:bank", Kind: 'a', File: "test.journal", Line: 5, Pattern: "account assets:bank", Language: "hledger"})

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// pseudo-tags
	for _, tag := range []string{
		"!_TAG_FILE_FORMAT\t2",
		"!_TAG_FILE_SORTED\t1",
		"!_TAG_PROGRAM_NAME\tclerk",
		"!_TAG_PROGRAM_URL\t",
		"!_TAG_PROGRAM_VERSION\t",
	} {
		if !strings.Contains(out, tag) {
			t.Errorf("missing: %q", tag)
		}
	}

	// Verify language field on each tag entry
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "!_TAG_") || line == "" {
			continue
		}
		if !strings.Contains(line, "\tlanguage:hledger") {
			t.Errorf("tag entry missing language:hledger: %q", line)
		}
	}

	var firstTag string
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "!_TAG_") && line != "" {
			firstTag = line
			break
		}
	}

	if !strings.HasPrefix(firstTag, "$\ttest.journal\t/") {
		t.Errorf("unexpected first entry: %q", firstTag)
	}
	if !strings.Contains(firstTag, "kind:c") {
		t.Errorf("missing kind:c: %q", firstTag)
	}
	if !strings.Contains(firstTag, "line:9") {
		t.Errorf("missing line:9: %q", firstTag)
	}
}

func TestNoLanguageNoFieldDescription(t *testing.T) {
	w := NewWriter()
	w.DescribeKind('a', "account")
	w.Add(Entry{Name: "test", Kind: 'a', File: "f", Line: 1, Pattern: "account test"})

	var buf bytes.Buffer
	if err := w.Write(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if strings.Contains(out, "\tlanguage:") {
		t.Error("language: field should not appear when no entries have Language set")
	}
}
