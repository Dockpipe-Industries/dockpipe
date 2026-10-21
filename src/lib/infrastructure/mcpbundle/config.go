package mcpbundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Bindings are explicit operator input, never values inferred from a bundle or
// repository. Only required single directory fields are currently supported.
type Bindings struct {
	WorkspaceRoot string
	Values        map[string]string
}

type directoryField struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive,omitempty"`
}

func bindArguments(fields map[string]json.RawMessage, arguments []string, bindings Bindings) ([]string, string, string, error) {
	if len(fields) == 0 {
		if len(bindings.Values) != 0 || bindings.WorkspaceRoot != "" {
			return nil, "", "", fmt.Errorf("bundle declares no user configuration")
		}
		for _, arg := range arguments {
			if strings.Contains(arg, "${") || strings.ContainsRune(arg, 0) {
				return nil, "", "", fmt.Errorf("unsupported MCPB argument interpolation")
			}
		}
		return arguments, "", "", nil
	}
	if len(bindings.Values) != len(fields) || bindings.WorkspaceRoot == "" {
		return nil, "", "", fmt.Errorf("MCPB requires explicit directory values and workspace root")
	}
	root, err := canonicalDirectory(bindings.WorkspaceRoot)
	if err != nil {
		return nil, "", "", err
	}
	values := map[string]string{}
	for name, raw := range fields {
		if name == "" || strings.ContainsAny(name, "${}.\\/\x00") {
			return nil, "", "", fmt.Errorf("invalid MCPB configuration field")
		}
		var field directoryField
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&field); err != nil {
			return nil, "", "", fmt.Errorf("unsupported MCPB configuration declaration")
		}
		if field.Type != "directory" || !field.Required || field.Sensitive {
			return nil, "", "", fmt.Errorf("only required non-secret single directory fields are supported")
		}
		value, ok := bindings.Values[name]
		if !ok {
			return nil, "", "", fmt.Errorf("missing explicit MCPB directory binding")
		}
		directory, err := canonicalDirectory(value)
		if err != nil {
			return nil, "", "", err
		}
		relative, err := filepath.Rel(root, directory)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return nil, "", "", fmt.Errorf("MCPB directory binding escapes authorized workspace root")
		}
		values[name] = directory
	}
	resolved := make([]string, len(arguments))
	for i, arg := range arguments {
		if strings.ContainsRune(arg, 0) {
			return nil, "", "", fmt.Errorf("invalid MCPB argument")
		}
		if !strings.Contains(arg, "${") {
			resolved[i] = arg
			continue
		}
		if !strings.HasPrefix(arg, "${user_config.") || !strings.HasSuffix(arg, "}") {
			return nil, "", "", fmt.Errorf("unsupported MCPB argument interpolation")
		}
		name := strings.TrimSuffix(strings.TrimPrefix(arg, "${user_config."), "}")
		value, ok := values[name]
		if !ok {
			return nil, "", "", fmt.Errorf("MCPB argument references undeclared directory field")
		}
		resolved[i] = value
	}
	contract, _ := json.Marshal(struct {
		Root      string
		Values    map[string]string
		Arguments []string
	}{root, values, resolved})
	digest := sha256.Sum256(contract)
	return resolved, root, hex.EncodeToString(digest[:]), nil
}

func canonicalDirectory(value string) (string, error) {
	if !filepath.IsAbs(value) || strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("MCPB directory binding must be absolute")
	}
	canonical, err := filepath.EvalSymlinks(value)
	if err != nil {
		return "", fmt.Errorf("MCPB directory binding cannot be resolved")
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("MCPB directory binding is not a directory")
	}
	return filepath.Clean(canonical), nil
}
