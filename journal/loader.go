package journal

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/parser"
)

type ParsedFile struct {
	Path       string
	Src        []byte // file content; token literals in Ast alias it, must NOT be mutated after parsing
	Ast        *ast.Journal
	FileErrors []*ast.FileError
	Errors     []*ast.ParseError
}

// ResolvedItem is one item in an occurrence-ordered flat view; child items inlined at include site.
type ResolvedItem struct {
	Occurrence *ParsedFile
	IsInclude  bool // true for IncludeDirective entries (skipped by consumers)
	EntryIndex int  // index into Occurrence.Ast.Entries
}

// ResolvedJournal is a flat, occurrence-ordered view of a journal tree.
// Resolve/ResolveBytes returns a fresh journal; same source path may appear
// as multiple occurrences with different parser contexts.
type ResolvedJournal struct {
	Occurrences []*ParsedFile  // all occurrences in depth-first order
	Items       []ResolvedItem // flat stream, occur-order
	ByPath      map[string][]*ParsedFile
}

// FileErrors returns all file errors from all occurrences.
func (rj *ResolvedJournal) FileErrors() []*ast.FileError {
	var all []*ast.FileError
	for _, pf := range rj.Occurrences {
		all = append(all, pf.FileErrors...)
	}
	return all
}

// parseCacheMax bounds the number of parsed files retained by the loader.
const parseCacheMax = 64

type parseKey struct {
	canon, content string
	defaultYear    int
}

type parseEntry struct {
	src []byte
	ast *ast.Journal
}

// Loader include-aware journal parsing caching.
type Loader struct {
	mu           sync.RWMutex
	contentCache map[string][]byte // canonical path: normalised content
	parseCache   map[parseKey]parseEntry

	// ContentProvider, when set, is consulted before any disk read. It returns
	// the file's authoritative content and ok=true, or ok=false to fall back to
	// disk. The zero value (nil) keeps the loader disk-only.
	ContentProvider func(path string) ([]byte, bool)
}

func NewLoader() *Loader {
	return &Loader{
		contentCache: make(map[string][]byte),
		parseCache:   make(map[parseKey]parseEntry),
	}
}

// Resolve performs a fresh include-aware parse of fpath, returning a flat
// occurrence view. Same file may appear multiple times when included from
// multiple sites.
func (l *Loader) Resolve(fpath string) (*ResolvedJournal, error) {
	src, err := l.readContent(fpath)
	if err != nil {
		return nil, err
	}
	return l.ResolveBytes(fpath, src), nil
}

// ResolveBytes is like [Loader.Resolve] but parses from a byte slice; includes resolved relative to fpath.
func (l *Loader) ResolveBytes(fpath string, src []byte) *ResolvedJournal {
	rj := &ResolvedJournal{
		ByPath: make(map[string][]*ParsedFile),
	}
	l.resolveOccurrence(rj, nil, fpath, src, 0, nil)
	return rj
}

// ResolveFiles resolves multiple entry files into one flat view.
func (l *Loader) ResolveFiles(paths []string) *ResolvedJournal {
	rj := &ResolvedJournal{
		ByPath: make(map[string][]*ParsedFile),
	}
	for _, p := range paths {
		src, err := l.readContent(p)
		if err != nil {
			continue
		}
		l.resolveOccurrence(rj, nil, p, src, 0, nil)
	}
	return rj
}

// ResolveFS loads a journal from [fs.FS] via temp dir.
func (l *Loader) ResolveFS(fsys fs.FS, fpath string) (*ResolvedJournal, error) {
	dir, err := os.MkdirTemp("", "clerk-loadfs-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if cerr := os.CopyFS(dir, fsys); cerr != nil {
		return nil, fmt.Errorf("copying fs to temp dir: %w", cerr)
	}

	rj, err := l.Resolve(filepath.Join(dir, fpath))
	if err != nil {
		return nil, err
	}

	// remap temp dir paths to FS-relative paths for deterministic output
	l.remapFilePaths(rj, dir)
	return rj, nil
}

func (l *Loader) remapFilePaths(rj *ResolvedJournal, oldRoot string) {
	for _, pf := range rj.Occurrences {
		if rel, err := filepath.Rel(oldRoot, pf.Path); err == nil {
			pf.Path = filepath.ToSlash(rel)
		}
	}

	newByPath := make(map[string][]*ParsedFile, len(rj.ByPath))
	for oldPath, pfs := range rj.ByPath {
		rel, err := filepath.Rel(oldRoot, oldPath)
		newPath := oldPath
		if err == nil {
			newPath = filepath.ToSlash(rel)
		}
		newByPath[newPath] = pfs
	}
	rj.ByPath = newByPath
}

// InvalidateFile removes a file from the content cache.
func (l *Loader) InvalidateFile(fpath string) {
	canon := CanonicalPath(fpath)
	l.mu.Lock()
	delete(l.contentCache, canon)
	l.mu.Unlock()
}

// Evict drops a file's cached content.
func (l *Loader) Evict(fpath string) {
	canon := CanonicalPath(fpath)
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.contentCache, canon)
	for k := range l.parseCache {
		if k.canon == canon {
			delete(l.parseCache, k)
		}
	}
}

// readContent reads a file, preferring the content provider, then the disk content cache.
func (l *Loader) readContent(fpath string) ([]byte, error) {
	if l.ContentProvider != nil {
		if content, ok := l.ContentProvider(fpath); ok {
			return normaliseNewlines(content), nil
		}
	}

	canon := CanonicalPath(fpath)

	l.mu.RLock()
	content, ok := l.contentCache[canon]
	l.mu.RUnlock()
	if ok {
		return content, nil
	}

	raw, err := os.ReadFile(fpath)
	if err != nil {
		return nil, err
	}

	content = normaliseNewlines(raw)
	l.mu.Lock()
	l.contentCache[canon] = content
	l.mu.Unlock()
	return content, nil
}

func normaliseNewlines(raw []byte) []byte {
	content := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(content, []byte("\r"), []byte("\n"))
}

// resolveOccurrence recursively parses one occurrence and its includes
func (l *Loader) resolveOccurrence(rj *ResolvedJournal, parent *ParsedFile, fpath string, src []byte, defaultYear int, stack []string) {
	// cycle detection uses canonical paths to catch cycles through symlinks.
	canon := CanonicalPath(fpath)
	if slices.Contains(stack, canon) {
		if parent != nil {
			parent.FileErrors = append(parent.FileErrors, &ast.FileError{
				Path:    fpath,
				Message: fmt.Sprintf("include cycle: %s", strings.Join(append(stack, canon), " → ")),
			})
		}
		return
	}

	key := parseKey{canon: canon, content: string(src), defaultYear: defaultYear}
	entry, ok := l.parseLookup(key)
	if !ok {
		lex := lexer.New(fpath, src)
		par := parser.NewWithYear(lex, defaultYear)
		j := par.ParseJournal()
		entry = parseEntry{src: src, ast: j}
		l.parseStore(key, entry)
	}

	pf := &ParsedFile{
		Path:   fpath,
		Src:    entry.src,
		Ast:    entry.ast,
		Errors: entry.ast.Errors,
	}
	rj.Occurrences = append(rj.Occurrences, pf)
	rj.ByPath[fpath] = append(rj.ByPath[fpath], pf)

	currentYear := defaultYear
	for i, entry := range entry.ast.Entries {
		switch e := entry.(type) {
		case *ast.BlankLine:
			continue

		case *ast.IncludeDirective:
			rj.Items = append(rj.Items, ResolvedItem{
				Occurrence: pf,
				IsInclude:  true,
				EntryIndex: i,
			})

			incPath, err := resolveIncludePath(fpath, e.Path)
			if err != nil {
				pf.FileErrors = append(pf.FileErrors, &ast.FileError{
					Path:    e.Path,
					Span:    e.Span,
					Message: err.Error(),
				})
				continue
			}

			matches, err := filepath.Glob(incPath)
			if err != nil || len(matches) == 0 {
				pf.FileErrors = append(pf.FileErrors, &ast.FileError{
					Path:    incPath,
					Span:    e.Span,
					Message: fmt.Sprintf("include not found: %s", e.Path),
				})
				continue
			}

			for _, match := range matches {
				childSrc, err := l.readContent(match)
				if err != nil {
					pf.FileErrors = append(pf.FileErrors, &ast.FileError{
						Path:    match,
						Span:    e.Span,
						Message: err.Error(),
					})
					continue
				}
				l.resolveOccurrence(rj, pf, match, childSrc, currentYear, append(stack, canon))
			}

		default:
			rj.Items = append(rj.Items, ResolvedItem{Occurrence: pf, EntryIndex: i})
		}

		// track year directive for context propagation to child includes
		if yd, ok := entry.(*ast.YearDirective); ok && yd.Year > 0 {
			currentYear = yd.Year
		}
	}
}

func resolveIncludePath(parentPath, incPattern string) (string, error) {
	base := filepath.Clean(filepath.Dir(parentPath))
	target := filepath.Clean(filepath.Join(base, incPattern))
	if filepath.IsAbs(incPattern) {
		return target, nil
	}

	// reject includes that escape the parent directory, e.g. "../../other.journal"
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal: %s", incPattern)
	}
	return target, nil
}

// CanonicalPath resolvea path to it's canonical form: absolute, symlinks evaluated, cleaned.
func CanonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(canonical)
}

func (l *Loader) parseLookup(key parseKey) (parseEntry, bool) {
	l.mu.RLock()
	entry, ok := l.parseCache[key]
	l.mu.RUnlock()
	return entry, ok
}

func (l *Loader) parseStore(key parseKey, entry parseEntry) {
	l.mu.Lock()
	l.parseCache[key] = entry
	if len(l.parseCache) > parseCacheMax {
		for k := range l.parseCache {
			delete(l.parseCache, k)
			break
		}
	}
	l.mu.Unlock()
}
