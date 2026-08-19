package coreir

import (
	"bufio"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	CaseFoldingUnicodeVersion = "17.0.0"
	CaseFoldingDataSHA256     = "ff8d8fefbf123574205085d6714c36149eb946d717a0c585c27f0f4ef58c4183"
)

//go:embed unicode/CaseFolding-17.0.0.txt
var caseFoldingData []byte

// CaseFoldingMapping is one full default case-fold mapping from the pinned
// Unicode Character Database. Only status C and F mappings are included;
// simple-only and locale-specific Turkic mappings are deliberately excluded.
type CaseFoldingMapping struct {
	Source      rune
	Replacement string
}

var (
	caseFoldingMappings, caseFoldingByRune = mustLoadCaseFoldingData()
)

// CaseFoldingMappings returns a copy of the canonical, source-sorted mappings
// so deterministic backends can emit the exact same language table.
func CaseFoldingMappings() []CaseFoldingMapping {
	return append([]CaseFoldingMapping(nil), caseFoldingMappings...)
}

// FoldCaseText applies Unicode 17.0.0 full default case folding without
// normalization or locale tailoring.
func FoldCaseText(value string) (string, error) {
	if err := ValidateText(value); err != nil {
		return "", err
	}
	return foldValidCaseText(value), nil
}

// ContainsCaseFoldedText validates both operands before applying Unicode
// 17.0.0 full default case folding and contiguous scalar-sequence containment.
func ContainsCaseFoldedText(value, query string) (bool, error) {
	if err := ValidateText(value); err != nil {
		return false, fmt.Errorf("value %w", err)
	}
	if err := ValidateText(query); err != nil {
		return false, fmt.Errorf("query %w", err)
	}
	return strings.Contains(foldValidCaseText(value), foldValidCaseText(query)), nil
}

func foldValidCaseText(value string) string {
	var folded strings.Builder
	folded.Grow(len(value))
	for _, scalar := range value {
		if replacement, ok := caseFoldingByRune[scalar]; ok {
			folded.WriteString(replacement)
		} else {
			folded.WriteRune(scalar)
		}
	}
	return folded.String()
}

func mustLoadCaseFoldingData() ([]CaseFoldingMapping, map[rune]string) {
	digest := fmt.Sprintf("%x", sha256.Sum256(caseFoldingData))
	if digest != CaseFoldingDataSHA256 {
		panic(fmt.Sprintf("PipeLang Unicode case-fold data digest %s does not match %s", digest, CaseFoldingDataSHA256))
	}

	mappings := make([]CaseFoldingMapping, 0, 1585)
	byRune := make(map[rune]string, 1585)
	scanner := bufio.NewScanner(strings.NewReader(string(caseFoldingData)))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		fields := strings.Split(line, ";")
		if len(fields) < 3 {
			panic(fmt.Sprintf("PipeLang Unicode case-fold data line %d is malformed", lineNumber))
		}
		status := strings.TrimSpace(fields[1])
		if status != "C" && status != "F" {
			continue
		}
		source := parseCaseFoldingScalar(strings.TrimSpace(fields[0]), lineNumber)
		if _, duplicate := byRune[source]; duplicate {
			panic(fmt.Sprintf("PipeLang Unicode case-fold data line %d duplicates U+%04X", lineNumber, source))
		}
		var replacement strings.Builder
		for _, encoded := range strings.Fields(strings.TrimSpace(fields[2])) {
			replacement.WriteRune(parseCaseFoldingScalar(encoded, lineNumber))
		}
		if replacement.Len() == 0 {
			panic(fmt.Sprintf("PipeLang Unicode case-fold data line %d has an empty mapping", lineNumber))
		}
		mapped := replacement.String()
		byRune[source] = mapped
		mappings = append(mappings, CaseFoldingMapping{Source: source, Replacement: mapped})
	}
	if err := scanner.Err(); err != nil {
		panic(fmt.Sprintf("PipeLang Unicode case-fold data cannot be read: %v", err))
	}
	if len(mappings) != 1585 {
		panic(fmt.Sprintf("PipeLang Unicode case-fold data contains %d default mappings, want 1585", len(mappings)))
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].Source < mappings[j].Source })
	return mappings, byRune
}

func parseCaseFoldingScalar(encoded string, lineNumber int) rune {
	value, err := strconv.ParseUint(encoded, 16, 32)
	if err != nil || value > utf8.MaxRune || value >= 0xD800 && value <= 0xDFFF {
		panic(fmt.Sprintf("PipeLang Unicode case-fold data line %d has invalid scalar %q", lineNumber, encoded))
	}
	return rune(value)
}
