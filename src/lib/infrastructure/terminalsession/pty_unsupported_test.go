//go:build !linux && !darwin

package terminalsession

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupportedPlatformNeverFallsBackToPipes(t *testing.T) {
	if backend, err := startTerminal(context.Background(), Profile{}, Size{80, 24}); backend != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatal("unsupported platform supplied a pretend terminal")
	}
}
