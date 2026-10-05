// Package cloudusage validates accounting counters without interpreting a
// missing or damaged ledger as zero usage. Filesystem and CLI handling belong
// to the orchestration adapter.
package cloudusage

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Ledger retains raw numbers until the requested counter is decoded as int64.
// This avoids precision loss through float64 and preserves extensible fields.
type Ledger struct {
	fields map[string]json.RawMessage
}

func Parse(data []byte) (Ledger, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return Ledger{}, errors.New("invalid cloud usage ledger")
	}
	return Ledger{fields: fields}, nil
}

func (l Ledger) Number(field string) (int64, error) {
	return counter(l.fields[field], field)
}

func (l Ledger) ProviderNumber(provider, field string) (int64, error) {
	var providers map[string]map[string]json.RawMessage
	if err := json.Unmarshal(l.fields["providers"], &providers); err != nil || providers[provider] == nil {
		return 0, fmt.Errorf("invalid cloud usage provider %q", provider)
	}
	return counter(providers[provider][field], field)
}

func counter(raw json.RawMessage, field string) (int64, error) {
	var value *int64
	if err := json.Unmarshal(raw, &value); err != nil || value == nil || *value < 0 {
		return 0, fmt.Errorf("invalid cloud usage counter %q", field)
	}
	return *value, nil
}
