package mcpmodel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPDecodeBudget(t *testing.T) {
	data := []byte(`{"servers":{},"padding":"` + strings.Repeat("x", 8<<20) + `"}`)
	var config Config
	if err := json.Unmarshal(data, &config); err == nil || !strings.Contains(err.Error(), "8 MiB") {
		t.Fatal("aggregate decode budget not enforced", err)
	}
}
