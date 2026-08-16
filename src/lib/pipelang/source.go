package pipelang

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// FileID is the normalized, deterministic identity of a file in a SourceSet.
// It is path-shaped rather than ordinal so adding another file cannot renumber existing spans.
type FileID string

// Span is a half-open UTF-8 byte range in one source file.
type Span struct {
	File  FileID `json:"file"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

func (s Span) IsValid() bool {
	return s.File != "" && s.Start >= 0 && s.End >= s.Start
}

func mergeSpans(first, last Span) Span {
	if !first.IsValid() {
		return last
	}
	if !last.IsValid() || first.File != last.File {
		return first
	}
	return Span{File: first.File, Start: first.Start, End: last.End}
}

// SourcePosition is a one-based source location. UTF16Column is also one-based
// and is provided for editor protocols such as VS Code/LSP.
type SourcePosition struct {
	Offset      int `json:"offset"`
	Line        int `json:"line"`
	Column      int `json:"column"`
	UTF16Column int `json:"utf16_column"`
}

// SourceRange is a file-aware resolved span suitable for CLI/editor consumers.
type SourceRange struct {
	File  FileID         `json:"file"`
	Start SourcePosition `json:"start"`
	End   SourcePosition `json:"end"`
}

type SourceInput struct {
	Path string
	Data []byte
}

type SourceFile struct {
	ID         FileID
	Path       string
	Text       string
	lineStarts []int
}

type SourceSet struct {
	files []*SourceFile
	byID  map[FileID]*SourceFile
}

func NewSourceSet(inputs []SourceInput) (*SourceSet, Diagnostics) {
	set := &SourceSet{byID: map[FileID]*SourceFile{}}
	var diagnostics Diagnostics
	sorted := append([]SourceInput(nil), inputs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return normalizeSourcePath(sorted[i].Path) < normalizeSourcePath(sorted[j].Path)
	})
	for _, input := range sorted {
		path := normalizeSourcePath(input.Path)
		id := FileID(path)
		if _, exists := set.byID[id]; exists {
			diagnostics = append(diagnostics, Diagnostic{
				Code:     CodeDuplicateSource,
				Category: CategorySource,
				Severity: SeverityError,
				Message:  "duplicate source identity " + quoted(path),
				Primary:  Span{File: id, Start: 0, End: 0},
			})
			continue
		}
		if !utf8.Valid(input.Data) {
			start := firstInvalidUTF8(input.Data)
			diagnostics = append(diagnostics, Diagnostic{
				Code:     CodeInvalidUTF8,
				Category: CategorySource,
				Severity: SeverityError,
				Message:  "source is not valid UTF-8",
				Primary:  Span{File: id, Start: start, End: min(start+1, len(input.Data))},
			})
			continue
		}
		file := &SourceFile{ID: id, Path: path, Text: string(input.Data), lineStarts: []int{0}}
		for i, b := range input.Data {
			if b == '\n' {
				file.lineStarts = append(file.lineStarts, i+1)
			}
		}
		set.files = append(set.files, file)
		set.byID[id] = file
	}
	diagnostics.Sort()
	return set, diagnostics
}

func NewSourceSetFromMap(files map[string][]byte) (*SourceSet, Diagnostics) {
	inputs := make([]SourceInput, 0, len(files))
	for path, data := range files {
		inputs = append(inputs, SourceInput{Path: path, Data: data})
	}
	return NewSourceSet(inputs)
}

func (s *SourceSet) Files() []*SourceFile {
	if s == nil {
		return nil
	}
	return append([]*SourceFile(nil), s.files...)
}

func (s *SourceSet) File(id FileID) (*SourceFile, bool) {
	if s == nil {
		return nil, false
	}
	file, ok := s.byID[id]
	return file, ok
}

func (s *SourceSet) Range(span Span) SourceRange {
	result := SourceRange{File: span.File}
	file, ok := s.File(span.File)
	if !ok {
		result.Start = SourcePosition{Offset: span.Start, Line: 1, Column: span.Start + 1, UTF16Column: span.Start + 1}
		result.End = SourcePosition{Offset: span.End, Line: 1, Column: span.End + 1, UTF16Column: span.End + 1}
		return result
	}
	result.Start = file.Position(span.Start)
	result.End = file.Position(span.End)
	return result
}

func (f *SourceFile) Position(offset int) SourcePosition {
	if f == nil {
		return SourcePosition{Offset: offset, Line: 1, Column: offset + 1, UTF16Column: offset + 1}
	}
	offset = max(0, min(offset, len(f.Text)))
	lineIndex := sort.Search(len(f.lineStarts), func(i int) bool { return f.lineStarts[i] > offset }) - 1
	if lineIndex < 0 {
		lineIndex = 0
	}
	lineStart := f.lineStarts[lineIndex]
	prefix := f.Text[lineStart:offset]
	column := utf8.RuneCountInString(prefix) + 1
	utf16Column := 1
	for _, r := range prefix {
		utf16Column += len(utf16.Encode([]rune{r}))
	}
	return SourcePosition{Offset: offset, Line: lineIndex + 1, Column: column, UTF16Column: utf16Column}
}

func normalizeSourcePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "<input>"
	}
	if strings.HasPrefix(path, "<") && strings.HasSuffix(path, ">") {
		return path
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func firstInvalidUTF8(data []byte) int {
	for i := 0; i < len(data); {
		_, size := utf8.DecodeRune(data[i:])
		if size == 1 && data[i] >= utf8.RuneSelf {
			return i
		}
		i += size
	}
	return 0
}
