package journal

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/parser"
)

type ParsedFile struct {
	Path       string
	Src        []byte
	Ast        *ast.Journal
	Includes   []*ParsedFile
	Errors     []*ast.ParseError
	FileErrors []*ast.FileError
}

// CollectFiles returns root and all its transitive includes.
func CollectFiles(root *ParsedFile) []*ParsedFile {
	var files []*ParsedFile
	visited := make(map[*ParsedFile]bool)
	var walk func(*ParsedFile)
	walk = func(pf *ParsedFile) {
		if visited[pf] {
			return
		}
		visited[pf] = true
		files = append(files, pf)
		for _, inc := range pf.Includes {
			walk(inc)
		}
	}
	walk(root)
	return files
}

type Loader struct {
	files map[string]*ParsedFile // key is absolute path (OS) or FS-relative path
	fsys  fs.FS                  // set during LoadFS
}

func NewLoader() *Loader {
	return &Loader{files: make(map[string]*ParsedFile)}
}

func (l *Loader) Load(fpath string) (*ParsedFile, error) {
	return l.loadFile(fpath, nil)
}

// LoadFS loads a journal from an [fs.FS], resolving includes through the same filesystem.
func (l *Loader) LoadFS(fsys fs.FS, fpath string) (*ParsedFile, error) {
	l.fsys = fsys
	defer func() { l.fsys = nil }()
	return l.loadFile(fpath, nil)
}

func (l *Loader) LoadBytes(path string, src []byte) (*ParsedFile, error) {
	return l.loadBytes(path, src, nil)
}

func (l *Loader) Reload(path string, src []byte) (*ParsedFile, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	delete(l.files, abs)
	return l.loadBytes(abs, src, nil)
}

// Roots returns files not transitively included by any loaded file.
func (l *Loader) Roots() []*ParsedFile {
	included := make(map[string]bool)
	for _, pf := range l.files {
		for _, inc := range pf.Includes {
			included[inc.Path] = true
		}
	}
	var roots []*ParsedFile
	for _, pf := range l.files {
		if !included[pf.Path] {
			roots = append(roots, pf)
		}
	}
	return roots
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
	if l.fsys != nil {
		fpath = path.Clean(fpath)
		if pf, ok := l.files[fpath]; ok {
			return pf, nil
		}
		src, err := fs.ReadFile(l.fsys, fpath)
		if err != nil {
			return nil, err
		}
		return l.loadBytes(fpath, src, stack)
	}

	abs, err := filepath.Abs(fpath)
	if err != nil {
		return nil, err
	}
	if pf, ok := l.files[abs]; ok {
		return pf, nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return l.loadBytes(abs, src, stack)
}

func (l *Loader) loadBytes(pathStr string, src []byte, stack []string) (*ParsedFile, error) {
	var fpath string
	if l.fsys != nil {
		fpath = path.Clean(pathStr)
	} else {
		abs, err := filepath.Abs(pathStr)
		if err != nil {
			return nil, err
		}
		fpath = abs
	}

	if slices.Contains(stack, fpath) {
		return nil, fmt.Errorf("include cycle: %s", strings.Join(append(stack, fpath), " → "))
	}
	if pf, ok := l.files[fpath]; ok {
		return pf, nil
	}

	lex := lexer.New(fpath, src)
	par := parser.New(lex)
	j := par.ParseJournal()

	pf := &ParsedFile{
		Path:     fpath,
		Src:      src,
		Ast:      j,
		Includes: []*ParsedFile{},
		Errors:   j.Errors,
	}
	l.files[fpath] = pf

	for _, entry := range j.Entries {
		inc, ok := entry.(*ast.IncludeDirective)
		if !ok {
			continue
		}

		var incPath string
		var matches []string
		var err error
		if l.fsys != nil {
			incPath = path.Join(path.Dir(fpath), inc.Path)
			matches, err = fs.Glob(l.fsys, incPath)
		} else {
			incPath = filepath.Join(filepath.Dir(fpath), inc.Path)
			matches, err = filepath.Glob(incPath)
		}

		if err != nil || len(matches) == 0 {
			pf.FileErrors = append(pf.FileErrors, &ast.FileError{
				Path:    incPath,
				Span:    inc.Span,
				Message: fmt.Sprintf("include not found: %s", inc.Path),
			})
			continue
		}

		for _, match := range matches {
			child, err := l.loadFile(match, append(stack, fpath))
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
