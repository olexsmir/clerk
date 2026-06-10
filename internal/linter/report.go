package linter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// PathStyle controls how file paths are shown in output.
type PathStyle int

const (
	PathBasename PathStyle = iota // just the filename (e.g. "entry.journal")
	PathAbsolute                  // full absolute path
	PathRelative                  // relative to current directory
)

// Fprint writes finds in text format: file:line:col code: message.
func Fprint(w io.Writer, style PathStyle, finds []Find) {
	for _, find := range finds {
		fmt.Fprintf(w, "%s:%d:%d: %s: %s\n",
			formatPath(style, find.Span.Start.File),
			find.Span.Start.Line, find.Span.Start.Col,
			find.Code, find.Message)
	}
}

type FindJSON struct {
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// FprintJSON writes finds as [FindJSON] array.
func FprintJSON(w io.Writer, style PathStyle, finds []Find) error {
	jsonFinds := make([]FindJSON, len(finds))
	for i, find := range finds {
		jsonFinds[i] = FindJSON{
			Message:  find.Message,
			Severity: find.Severity.String(),
			Code:     string(find.Code),
			File:     formatPath(style, find.Span.Start.File),
			Line:     find.Span.Start.Line,
			Column:   find.Span.Start.Col,
		}
	}
	return json.NewEncoder(w).Encode(jsonFinds)
}

func formatPath(style PathStyle, p string) string {
	switch style {
	case PathBasename:
		return filepath.Base(p)
	case PathAbsolute:
		return p
	case PathRelative:
		wd, err := os.Getwd()
		if err == nil {
			if rel, err := filepath.Rel(wd, p); err == nil {
				return rel
			}
		}
		return p
	default:
		panic("impossible PathStyle value")
	}
}
