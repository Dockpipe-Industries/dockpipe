package skillsrender

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type instructionSection struct {
	heading string
	text    string
}

func targetInstructions(item skill, target string) (string, error) {
	if item.SourceDir == "" {
		return item.Instructions, nil
	}
	// Targets are adapter names, never arbitrary path components.
	switch target {
	case "codex", "claude", "generic":
	default:
		return "", fmt.Errorf("unsupported target %q", target)
	}
	path := filepath.Join(item.SourceDir, "instructions."+target+".md")
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return item.Instructions, nil
	}
	if err != nil {
		return "", fmt.Errorf("read instruction override %s: %w", path, err)
	}
	instructions, err := replaceInstructionSections(item.Instructions, string(content))
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return instructions, nil
}

func replaceInstructionSections(base, override string) (string, error) {
	sections := splitInstructionSections(base)
	replacements := splitInstructionSections(override)
	if len(replacements) < 2 || strings.TrimSpace(replacements[0].text) != "" {
		return "", fmt.Errorf("instruction overrides must start with a ## section heading")
	}
	seen := make(map[string]bool)
	for _, replacement := range replacements[1:] {
		if seen[replacement.heading] {
			return "", fmt.Errorf("duplicate override section %q", replacement.heading)
		}
		seen[replacement.heading] = true
		matches := 0
		for index, section := range sections {
			if section.heading == replacement.heading {
				sections[index] = replacement
				matches++
			}
		}
		if matches != 1 {
			return "", fmt.Errorf("override section %q must match exactly one shared section (found %d)", replacement.heading, matches)
		}
	}
	var result strings.Builder
	for _, section := range sections {
		if text := strings.TrimSpace(section.text); text != "" {
			result.WriteString(text)
			result.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(result.String()), nil
}

// Only level-two headings outside fenced code blocks identify replaceable sections.
func splitInstructionSections(instructions string) []instructionSection {
	sections := []instructionSection{{}}
	var fence byte
	fenceLength := 0
	for _, line := range strings.SplitAfter(instructions, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[0]
			length := len(trimmed) - len(strings.TrimLeft(trimmed, string(marker)))
			if fence == 0 {
				fence = marker
				fenceLength = length
			} else if marker == fence && length >= fenceLength && strings.TrimSpace(trimmed[length:]) == "" {
				fence = 0
			}
		}
		if fence == 0 && strings.HasPrefix(line, "## ") {
			sections = append(sections, instructionSection{heading: strings.TrimSpace(line)})
		}
		sections[len(sections)-1].text += line
	}
	return sections
}
