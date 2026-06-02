package tags

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
)

type Entry struct {
	Name     string
	Kind     byte
	File     string // file path (relative or absolute)
	Line     int    // line number
	Pattern  string // the full line content (without /^ $/ wrappers)
	Language string // language field, emitted per-entry when non-empty
}

type Writer struct {
	entries []Entry
	kinds   map[byte]string // kind: description
}

func NewWriter() *Writer {
	return &Writer{kinds: make(map[byte]string)}
}

func (w *Writer) DescribeKind(kind byte, description string) {
	w.kinds[kind] = description
}

func (w *Writer) Add(entry Entry) {
	w.entries = append(w.entries, entry)
}

func (w *Writer) Write(to io.Writer) error {
	// Pseudo-tags
	_, _ = fmt.Fprintf(to, "!_TAG_FILE_FORMAT\t2\t/extended format/\n")
	_, _ = fmt.Fprintf(to, "!_TAG_FILE_SORTED\t1\t/1=sorted/\n")
	_, _ = fmt.Fprintf(to, "!_TAG_PROGRAM_NAME\tclerk\t//\n")
	_, _ = fmt.Fprintf(to, "!_TAG_PROGRAM_URL\thttps://olexsmir.xyz/clerk\t//\n") // TODO:
	_, _ = fmt.Fprintf(to, "!_TAG_PROGRAM_VERSION\t0.1.0\t//\n")

	// Kind descriptions
	var kindLetters []byte
	for k := range w.kinds {
		kindLetters = append(kindLetters, k)
	}

	slices.Sort(kindLetters)
	for _, k := range kindLetters {
		_, _ = fmt.Fprintf(to, "!_TAG_KIND_DESCRIPTION!%c\t%c,%s\t/%s/\n", k, k, w.kinds[k], w.kinds[k])
	}

	// Tags
	sort.Slice(w.entries, func(i, j int) bool {
		if w.entries[i].Name != w.entries[j].Name {
			return w.entries[i].Name < w.entries[j].Name
		}
		if w.entries[i].Kind != w.entries[j].Kind {
			return w.entries[i].Kind < w.entries[j].Kind
		}
		if w.entries[i].File != w.entries[j].File {
			return w.entries[i].File < w.entries[j].File
		}
		return w.entries[i].Line < w.entries[j].Line
	})

	for _, e := range w.entries {
		_, _ = fmt.Fprintf(to, "%s\t%s\t/^%s$/;\"\tkind:%c\tline:%d", e.Name, e.File, escapePattern(e.Pattern), e.Kind, e.Line)
		if e.Language != "" {
			_, _ = fmt.Fprintf(to, "\tlanguage:%s", e.Language)
		}
		_, _ = fmt.Fprintf(to, "\n")
	}

	return nil
}

func escapePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `/`, `\/`)
	return s
}
