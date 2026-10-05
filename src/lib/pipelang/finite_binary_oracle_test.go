package pipelang

import (
	"encoding/hex"
	"testing"
)

func TestFiniteBinaryOracleExactEncoding(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []finiteConditionalOracleCase
		hex  string
	}{
		{"no vectors", nil, "00000000"},
		{"nil trace", []finiteConditionalOracleCase{{Value: ""}}, "0100000000000000ffffffff"},
		{"empty trace", []finiteConditionalOracleCase{{Value: "", Trace: []string{}}}, "010000000000000000000000"},
		{"exact strings and order", []finiteConditionalOracleCase{{Value: "A\x00☃", Trace: []string{"", "x\n"}}}, "01000000050000004100e29883020000000000000002000000780a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hex.EncodeToString(encodeFiniteBinaryOracle(tc.rows)); got != tc.hex {
				t.Fatalf("got %s want %s", got, tc.hex)
			}
		})
	}
}
