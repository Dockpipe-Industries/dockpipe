package nativehost

import (
	"math"
	"strconv"
	"testing"
)

func TestUnixIdentityIDBounds(t *testing.T) {
	for _, value := range []int{-1, -2} {
		if _, err := unixIdentityID(value); err == nil {
			t.Fatalf("negative OS identity %d accepted", value)
		}
	}
	for _, value := range []int{0, 1, 65535} {
		actual, err := unixIdentityID(value)
		if err != nil || uint64(actual) != uint64(value) {
			t.Fatalf("identity %d became %d: %v", value, actual, err)
		}
	}
	if strconv.IntSize == 64 {
		maximum := int64(math.MaxUint32)
		actual, err := unixIdentityID(int(maximum))
		if err != nil || actual != math.MaxUint32 {
			t.Fatalf("maximum identity became %d: %v", actual, err)
		}
		if _, err := unixIdentityID(int(maximum + 1)); err == nil {
			t.Fatal("OS identity above the protocol range accepted")
		}
	}
}
