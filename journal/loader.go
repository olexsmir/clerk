package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/olexsmir/ledger-tools/journal/ast"
	"github.com/olexsmir/ledger-tools/journal/lexer"
	"github.com/olexsmir/ledger-tools/journal/parser"
)

type ParsedFile struct {
	Path       string
	Src        []byte
	Ast        *ast.Journal
	Includes   []*ParsedFile
	Errors     []*ast.ParseError
	FileErrors []*ast.FileError
}

type Loader struct {
	files map[string]*ParsedFile // key is absolute path
}

func NewLoader() *Loader {
	return &Loader{make(map[string]*ParsedFile)}
}

func (l *Loader) Load(fpath string) (*ParsedFile, error) {
	return l.loadFile(fpath, nil)
}

func (l *Loader) LoadBytes(path string, src []byte) (*ParsedFile, error) {
	return l.loadBytes(path, src, nil)
}

// Ordered returns all files in dependency order (included before includer)
func (l *Loader) Ordered() []*ParsedFile {
	visited := make(map[string]bool)
	var res []*ParsedFile
	var visit func(*ParsedFile)
	visit = func(pf *ParsedFile) {
		if visited[pf.Path] {
			return
		}
		visited[pf.Path] = true
		for _, inc := range pf.Includes {
			visit(inc)
		}
		res = append(res, pf)
	}
	for _, pf := range l.files {
		visit(pf)
	}
	return res
}

func (l *Loader) loadFile(fpath string, stack []string) (*ParsedFile, error) {
	abs, err := filepath.Abs(fpath)
	if err != nil {
		return nil, err
	}

	// reuse already loaded
	if pf, ok := l.files[abs]; ok {
		return pf, nil
	}

	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}

	return l.loadBytes(abs, src, stack)
}

func (l *Loader) loadBytes(path string, src []byte, stack []string) (*ParsedFile, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// cycle includes
	if slices.Contains(stack, abs) {
		return nil, fmt.Errorf("include cycle: %s", strings.Join(append(stack, abs), " → "))
	}

	// reuse already loaded
	if pf, ok := l.files[abs]; ok {
		return pf, nil
	}

	lex := lexer.New(abs, src)
	par := parser.New(lex)
	j := par.ParseJournal()

	pf := &ParsedFile{
		Path:     abs,
		Src:      src,
		Ast:      j,
		Includes: []*ParsedFile{},
		Errors:   j.Errors,
	}
	l.files[abs] = pf

	for _, entry := range j.Entries {
		inc, ok := entry.(*ast.IncludeDirective)
		if !ok {
			continue
		}

		incPath := filepath.Join(filepath.Dir(abs), inc.Path)

		matches, err := filepath.Glob(incPath)
		if err != nil || len(matches) == 0 {
			pf.FileErrors = append(pf.FileErrors, &ast.FileError{
				Path:    incPath,
				Span:    inc.Span,
				Message: fmt.Sprintf("include not found: %s", inc.Path),
			})
			continue
		}

		for _, match := range matches {
			child, err := l.loadFile(match, append(stack, abs))
			if err != nil {
				pf.FileErrors = append(pf.FileErrors, &ast.FileError{
					Path:    match,
					Span:    inc.Span,
					Message: err.Error(),
				})
				continue
			}
			pf.Includes = append(pf.Includes, child)
		}
	}

	return pf, nil
}
