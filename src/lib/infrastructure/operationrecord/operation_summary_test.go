package operationrecord

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperationSummariesBoundHistoryAndExcludePrivateFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	event := OperationEvent{Schema: OperationEventSchemaV1, Unit: "step.run", Status: "done", Timestamp: "2026-01-01T00:00:00Z", Message: "PRIVATE_VALUE", Error: "PRIVATE_VALUE", IDs: map[string]string{"token": "PRIVATE_VALUE"}}
	for index := 0; index < MaxOperationSummaries+10; index++ {
		if err := AppendOperationEvent(path, event); err != nil {
			t.Fatal(err)
		}
	}
	summaries, err := ReadOperationSummaries(path)
	if err != nil || len(summaries) != MaxOperationSummaries {
		t.Fatalf("%d %v", len(summaries), err)
	}
	data, _ := json.Marshal(summaries)
	if strings.Contains(string(data), "PRIVATE_VALUE") {
		t.Fatal("raw log field exposed")
	}
	if err := os.WriteFile(path, append([]byte(strings.Repeat("x", operationSummaryBytes+1)), '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	summaries, err = ReadOperationSummaries(path)
	if err != nil || len(summaries) != 0 {
		t.Fatalf("malformed tail: %v", err)
	}
}
