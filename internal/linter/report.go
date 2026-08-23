package linter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	sortFinds(finds)
	for _, find := range finds {
		_, _ = fmt.Fprintf(w, "%s:%d:%d: %s: %s\n",
			formatPath(style, find.Span.File),
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
	sortFinds(finds)
	jsonFinds := make([]FindJSON, len(finds))
	for i, find := range finds {
		jsonFinds[i] = FindJSON{
			Message:  find.Message,
			Severity: find.Severity.String(),
			Code:     string(find.Code),
			File:     formatPath(style, find.Span.File),
			Line:     find.Span.Start.Line,
			Column:   find.Span.Start.Col,
		}
	}
	return json.NewEncoder(w).Encode(jsonFinds)
}

// Reporter collects lint findings across files and flushes them in the desired format.
type Reporter struct {
	w     io.Writer
	finds []Find
	style PathStyle
}

func NewReporter(w io.Writer, style PathStyle) *Reporter {
	return &Reporter{w: w, style: style}
}

func (r *Reporter) Collect(finds []Find) {
	r.finds = append(r.finds, finds...)
}

func (r *Reporter) HasIssues() bool {
	return len(r.finds) > 0
}

func (r *Reporter) Flush(format string) error {
	switch format {
	case "json":
		return FprintJSON(r.w, r.style, r.finds)
	case "text":
		Fprint(r.w, r.style, r.finds)
		return nil
	default:
		return errors.New("unsupported format")
	}
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

func sortFinds(finds []Find) {
	sort.Slice(finds, func(i, j int) bool {
		if finds[i].Span.Start.Line != finds[j].Span.Start.Line {
			return finds[i].Span.Start.Line < finds[j].Span.Start.Line
		}
		if finds[i].Span.Start.Col != finds[j].Span.Start.Col {
			return finds[i].Span.Start.Col < finds[j].Span.Start.Col
		}
		if finds[i].Code != finds[j].Code {
			return finds[i].Code < finds[j].Code
		}
		return finds[i].Message < finds[j].Message
	})
}
