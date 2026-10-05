package pipelang

import (
	"fmt"
	"strings"
	"testing"
)

// Keep the historical formatter as an independent byte-compatibility oracle.
// Inputs include invalid identities because key construction must not normalize
// bytes or silently become an admission policy.
func TestSemanticCanonicalKeyEncodingCompatibility(t *testing.T) {
	field := func(value string) string { return fmt.Sprintf("%d:%s", len(value), value) }
	var historicalType func(SemanticTypeIdentity) string
	historicalType = func(identity SemanticTypeIdentity) string {
		var key strings.Builder
		for _, value := range []string{string(identity.Kind), string(identity.Primitive), string(identity.PackageID), string(identity.Path), identity.Name, fmt.Sprint(len(identity.Arguments))} {
			key.WriteString(field(value))
		}
		for _, argument := range identity.Arguments {
			key.WriteString(field(historicalType(argument)))
		}
		return key.String()
	}
	for _, value := range []string{"", "a", "\x00", ":9:ab", "é", "😀", string([]byte{0xff, 0x80}), strings.Repeat("x", 9), strings.Repeat("x", 10), strings.Repeat("x", 99), strings.Repeat("x", 100), strings.Repeat("x", 1000)} {
		var key strings.Builder
		writeCanonicalKeyField(&key, value)
		if key.String() != field(value) {
			t.Fatalf("field bytes changed for %q", value)
		}
		typ := SemanticTypeIdentity{Kind: TypeRefPrimitive, Primitive: PrimitiveType(value), PackageID: PackageID(value), Path: SemanticID(value), Name: value}
		for depth := 0; depth < 4; depth++ {
			if got, want := semanticTypeKey(typ), historicalType(typ); got != want {
				t.Fatalf("type encoding differs at depth %d", depth)
			}
			for _, count := range []int{0, 1, 9, 10, 99, 100, 128} {
				callable := CallableIdentity{Returns: typ, Parameters: make([]SemanticTypeIdentity, count)}
				var want strings.Builder
				want.WriteString(field(fmt.Sprint(count)))
				for i := range callable.Parameters {
					callable.Parameters[i] = typ
					want.WriteString(field(historicalType(typ)))
				}
				want.WriteString(field(historicalType(typ)))
				if semanticCallableKey(callable) != want.String() {
					t.Fatalf("callable encoding differs: depth %d count %d", depth, count)
				}
				identity := SemanticIdentity{PackageID: PackageID(value), Path: SemanticID(value), Callable: &callable}
				if semanticIdentityKey(identity) != value+"\x00"+value+"\x00"+want.String() {
					t.Fatal("identity framing changed")
				}
				identity.Callable = nil
				if semanticIdentityKey(identity) != value+"\x00"+value {
					t.Fatal("noncallable framing changed")
				}
			}
			typ = SemanticTypeIdentity{Kind: TypeRefApplied, Name: value, Arguments: []SemanticTypeIdentity{typ}}
		}
	}
}
