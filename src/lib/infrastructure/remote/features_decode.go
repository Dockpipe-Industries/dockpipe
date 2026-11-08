package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// Feature schemas have fewer than 64 fields per object and fewer than 16 nested
// containers. Reject ambiguous duplicate keys before typed decoding so UI and
// worker interpretation cannot disagree about a reviewed field.
func decodeFeature(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := featureJSONValue(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("unexpected trailing feature JSON")
	}
	return Decode(data, target)
}

func featureJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("feature JSON nesting exceeds its schema bound")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, container := token.(json.Delim)
	if !container {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, valid := token.(string)
			if !valid || keys[key] || len(keys) >= 64 {
				return errors.New("duplicate or excessive feature object fields")
			}
			keys[key] = true
			if err := featureJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := featureJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid feature JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}
