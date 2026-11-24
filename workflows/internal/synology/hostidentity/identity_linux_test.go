//go:build linux

package hostidentity

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestStatusIDsRespectCredentialAndNativeIntegerBounds(t *testing.T) {
	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "ordinary", value: "1026", valid: true},
		{name: "zero", value: "0", valid: true},
		{name: "signed32Maximum", value: "2147483647", valid: true},
		{name: "unsigned32Maximum", value: "4294967295", valid: strconv.IntSize == 64},
		{name: "beyondCredentialRange", value: "4294967296"},
		{name: "negative", value: "-1"},
		{name: "nonNumeric", value: "account"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data := "Uid:\t" + strings.TrimSpace(strings.Repeat(test.value+" ", 4)) + "\n"
			ids, err := statusIDs(data, "Uid")
			if !test.valid {
				if err == nil {
					t.Fatalf("credential %q was accepted: %v", test.value, ids)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, identifier := range ids {
				if identifier < 0 || uint64(identifier) > math.MaxUint32 || strconv.Itoa(identifier) != test.value {
					t.Fatalf("credential %q changed during conversion: %v", test.value, ids)
				}
			}
		})
	}
}
