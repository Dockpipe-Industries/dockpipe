package nativehost

import (
	"errors"
	"math"
)

// Unix credentials and the native-host protocol use unsigned 32-bit IDs. Reject
// unavailable or unrepresentable OS values before narrowing them at either boundary.
func unixIdentityID(value int) (uint32, error) {
	if value < 0 || int64(value) > math.MaxUint32 {
		return 0, errors.New("invalid native host numeric identity")
	}
	return uint32(value), nil
}
