//go:build !linux

package nativehost

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupportedPlatformCannotEnableNativeHostAccess(t *testing.T) {
	if client, err := NewClient("unavailable.json"); client != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsupported client did not fail closed: client=%v err=%v", client, err)
	}
	if broker, err := NewBroker(Options{}); broker != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsupported broker did not fail closed: broker=%v err=%v", broker, err)
	}
	if err := RunWorker(context.Background(), WorkerOptions{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsupported worker did not fail closed: %v", err)
	}
}
