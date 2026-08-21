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

// Reporter collects lint findings across files and flushes them in the desired format.
type Reporter struct {
	w     io.Writer
	finds []Find
	style PathStyle
	cfg   Config
}

func NewReporter(w io.Writer, style PathStyle, cfg Config) *Reporter {
	return &Reporter{w: w, style: style, cfg: cfg}
}

func (r *Reporter) Collect(finds []Find) {
	for i := range finds {
		finds[i].Severity = r.cfg.SeverityFor(finds[i].Code)
	}
	r.finds = append(r.finds, finds...)
}

func (r *Reporter) HasFailures() bool {
	for i := range r.finds {
		if r.finds[i].Severity <= SeverityWarning {
			return true
		}
	}
	return false
}

func (r *Reporter) Flush(format string) error {
	switch format {
	case "json":
		return fprintJSON(r.w, r.style, r.finds)
	case "text":
		fprint(r.w, r.style, r.finds)
		return nil
	default:
		return errors.New("unsupported format")
	}
}

// fprint writes finds in text format: file:line:col code: message.
func fprint(w io.Writer, style PathStyle, finds []Find) {
	sortFinds(finds)
	for _, find := range finds {
		_, _ = fmt.Fprintf(w, "%s:%d:%d: %s: %s\n",
			formatPath(style, find.Span.Start.File),
			find.Span.Start.Line, find.Span.Start.Col,
			find.Code, find.Message)
	}
}

type findJSON struct {
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// fprintJSON writes finds as [findJSON] array.
func fprintJSON(w io.Writer, style PathStyle, finds []Find) error {
	sortFinds(finds)
	jsonFinds := make([]findJSON, len(finds))
	for i, find := range finds {
		jsonFinds[i] = findJSON{
			Message:  find.Message,
			Severity: find.Severity.String(), // TODO: it's unset
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
