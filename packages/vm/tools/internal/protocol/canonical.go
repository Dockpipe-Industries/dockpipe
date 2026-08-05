package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

var canonicalInteger = regexp.MustCompile(`^(0|-?[1-9][0-9]*)$`)

func Canonicalize(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty JSON")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if err := requireEOF(dec); err != nil {
		return nil, err
	}
	if err := validateNumbers(value); err != nil {
		return nil, err
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode canonical JSON: %w", err)
	}
	return out, nil
}

func RequireCanonical(data []byte) error {
	canonical, err := Canonicalize(data)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, canonical) {
		return fmt.Errorf("JSON is not in canonical form")
	}
	return nil
}

func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := scanValue(dec); err != nil {
		return err
	}
	return requireEOF(dec)
}

func scanValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("decode JSON token: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("decode object key: %w", err)
			}
			key, ok := keyTok.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanValue(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return fmt.Errorf("close object: %w", err)
		}
	case '[':
		for dec.More() {
			if err := scanValue(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return fmt.Errorf("close array: %w", err)
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

func requireEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}

func validateNumbers(value any) error {
	switch typed := value.(type) {
	case json.Number:
		if !canonicalInteger.MatchString(string(typed)) {
			return fmt.Errorf("only canonical integers are permitted")
		}
	case []any:
		for _, item := range typed {
			if err := validateNumbers(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range typed {
			if err := validateNumbers(item); err != nil {
				return err
			}
		}
	}
	return nil
}
