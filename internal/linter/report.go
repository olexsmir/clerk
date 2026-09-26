package linter

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
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
	w     *bufio.Writer
	finds []Find
	style PathStyle
	cfg   Config
}

func NewReporter(w io.Writer, style PathStyle, cfg Config) *Reporter {
	return &Reporter{w: bufio.NewWriter(w), style: style, cfg: cfg}
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
	defer r.w.Flush()
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
	wd, _ := os.Getwd()
	for _, find := range finds {
		io.WriteString(w, formatPath(style, find.Span.File, wd))
		io.WriteString(w, ":")
		io.WriteString(w, strconv.Itoa(find.Span.Start.Line))
		io.WriteString(w, ":")
		io.WriteString(w, strconv.Itoa(find.Span.Start.Col))
		io.WriteString(w, ": ")
		io.WriteString(w, string(find.Code))
		io.WriteString(w, ": ")
		io.WriteString(w, find.Message)
		io.WriteString(w, "\n")
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
	wd, _ := os.Getwd()
	jsonFinds := make([]findJSON, len(finds))
	for i, find := range finds {
		jsonFinds[i] = findJSON{
			Message:  find.Message,
			Severity: find.Severity.String(), // TODO: it's unset
			Code:     string(find.Code),
			File:     formatPath(style, find.Span.File, wd),
			Line:     find.Span.Start.Line,
			Column:   find.Span.Start.Col,
		}
	}
	return json.NewEncoder(w).Encode(jsonFinds)
}

func formatPath(style PathStyle, path, wdir string) string {
	switch style {
	case PathBasename:
		return filepath.Base(path)
	case PathAbsolute:
		return path
	case PathRelative:
		if wdir != "" {
			if rel, err := filepath.Rel(wdir, path); err == nil {
				return rel
			}
		}
		return path
	default:
		panic("impossible PathStyle value")
	}
}

func sortFinds(finds []Find) {
	slices.SortFunc(finds, func(a, b Find) int {
		if a.Span.Start.Line != b.Span.Start.Line {
			return a.Span.Start.Line - b.Span.Start.Line
		}
		if a.Span.Start.Col != b.Span.Start.Col {
			return a.Span.Start.Col - b.Span.Start.Col
		}
		if a.Code != b.Code {
			return strings.Compare(string(a.Code), string(b.Code))
		}
		return strings.Compare(a.Message, b.Message)
	})
}
