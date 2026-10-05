package cloudusage

import (
	"math"
	"testing"
)

func TestLedgerCountersRemainExact(t *testing.T) {
	ledger, err := Parse([]byte(`{"total_estimated_tokens":9223372036854775807,"providers":{"codex":{"estimated_tokens":9007199254740993,"task_count":0}},"extension":{"note":"retained"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if value, err := ledger.Number("total_estimated_tokens"); err != nil || value != math.MaxInt64 {
		t.Fatalf("total counter lost precision: %d, %v", value, err)
	}
	if value, err := ledger.ProviderNumber("codex", "estimated_tokens"); err != nil || value != 9007199254740993 {
		t.Fatalf("provider counter lost precision: %d, %v", value, err)
	}
	if value, err := ledger.ProviderNumber("codex", "task_count"); err != nil || value != 0 {
		t.Fatalf("explicit zero counter rejected: %d, %v", value, err)
	}
	if _, err := ledger.Number("missing"); err == nil {
		t.Fatal("missing counter became zero")
	}
	if _, err := ledger.ProviderNumber("missing", "task_count"); err == nil {
		t.Fatal("missing provider became zero")
	}
}

func TestLedgerRejectsInvalidCountersAtBothScopes(t *testing.T) {
	for _, value := range []string{`null`, `-1`, `1.5`, `1e3`, `"12"`, `true`, `{}`, `[]`, `9223372036854775808`} {
		t.Run(value, func(t *testing.T) {
			ledger, err := Parse([]byte(`{"count":` + value + `,"providers":{"provider":{"count":` + value + `}}}`))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ledger.Number("count"); err == nil {
				t.Fatal("invalid total accepted")
			}
			if _, err := ledger.ProviderNumber("provider", "count"); err == nil {
				t.Fatal("invalid provider total accepted")
			}
		})
	}
	for _, data := range []string{"", `null`, `[]`, `{"count":`, `{} {}`} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Fatalf("invalid ledger accepted: %s", data)
		}
	}
	for _, data := range []string{`{}`, `{"providers":null}`, `{"providers":[]}`, `{"providers":{"provider":null}}`} {
		ledger, err := Parse([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.ProviderNumber("provider", "count"); err == nil {
			t.Fatalf("invalid provider accepted: %s", data)
		}
	}
}
