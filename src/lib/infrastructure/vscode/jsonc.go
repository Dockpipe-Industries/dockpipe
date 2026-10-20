package vscode

import "fmt"

// normalizeJSONC preserves strings and newlines while removing comments and
// trailing commas. The 64-level depth contract keeps variable/config traversal
// and JSON decoding bounded for editor-authored documents.
func normalizeJSONC(content []byte) ([]byte, error) {
	output := append([]byte(nil), content...)
	inString, escaped, depth := false, false, 0
	for index := 0; index < len(output); index++ {
		current := output[index]
		if inString {
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			continue
		}
		if current == '/' && index+1 < len(output) && output[index+1] == '/' {
			for index < len(output) && output[index] != '\n' {
				output[index] = ' '
				index++
			}
			continue
		}
		if current == '/' && index+1 < len(output) && output[index+1] == '*' {
			output[index], output[index+1] = ' ', ' '
			index += 2
			for index+1 < len(output) && !(output[index] == '*' && output[index+1] == '/') {
				if output[index] != '\n' && output[index] != '\r' {
					output[index] = ' '
				}
				index++
			}
			if index+1 >= len(output) {
				return nil, fmt.Errorf("unterminated JSONC comment")
			}
			output[index], output[index+1] = ' ', ' '
			index++
			continue
		}
		if current == '{' || current == '[' {
			depth++
			if depth > 64 {
				return nil, fmt.Errorf("VS Code JSONC exceeds 64 nesting levels")
			}
		}
		if current == '}' || current == ']' {
			depth--
		}
	}
	inString, escaped = false, false
	for index, current := range output {
		if inString {
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			continue
		}
		if current != ',' {
			continue
		}
		next := index + 1
		for next < len(output) && (output[next] == ' ' || output[next] == '\r' || output[next] == '\n' || output[next] == '\t') {
			next++
		}
		if next < len(output) && (output[next] == '}' || output[next] == ']') {
			output[index] = ' '
		}
	}
	return output, nil
}
