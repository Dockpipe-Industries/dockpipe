package coreir

import "unicode/utf8"

const TextWhitespaceUnicodeVersion = "17.0.0"

// TextWhitespaceRange is one inclusive scalar range from the Unicode 17.0.0
// White_Space derived property. The table is deliberately small, immutable,
// and target-independent.
type TextWhitespaceRange struct {
	First rune
	Last  rune
}

var textWhitespaceRanges = []TextWhitespaceRange{
	{First: 0x0009, Last: 0x000D},
	{First: 0x0020, Last: 0x0020},
	{First: 0x0085, Last: 0x0085},
	{First: 0x00A0, Last: 0x00A0},
	{First: 0x1680, Last: 0x1680},
	{First: 0x2000, Last: 0x200A},
	{First: 0x2028, Last: 0x2029},
	{First: 0x202F, Last: 0x202F},
	{First: 0x205F, Last: 0x205F},
	{First: 0x3000, Last: 0x3000},
}

// TextWhitespaceRanges returns a copy of the pinned Unicode White_Space
// ranges so deterministic backends can emit the exact same language table.
func TextWhitespaceRanges() []TextWhitespaceRange {
	return append([]TextWhitespaceRange(nil), textWhitespaceRanges...)
}

// TrimText validates strict UTF-8, then removes the maximal leading and
// trailing sequences in Unicode 17.0.0's White_Space derived property.
// Interior scalar values are preserved exactly.
func TrimText(value string) (string, error) {
	if err := ValidateText(value); err != nil {
		return "", err
	}
	start := 0
	for start < len(value) {
		scalar, size := utf8.DecodeRuneInString(value[start:])
		if !isTextWhitespace(scalar) {
			break
		}
		start += size
	}
	end := len(value)
	for end > start {
		scalar, size := utf8.DecodeLastRuneInString(value[:end])
		if !isTextWhitespace(scalar) {
			break
		}
		end -= size
	}
	return value[start:end], nil
}

func isTextWhitespace(scalar rune) bool {
	for _, value := range textWhitespaceRanges {
		if scalar >= value.First && scalar <= value.Last {
			return true
		}
	}
	return false
}
