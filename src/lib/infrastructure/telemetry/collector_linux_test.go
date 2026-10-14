//go:build linux

package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxStatNamesParentsMetricsAndMalformedData(t *testing.T) {
	fields := []string{"S", "7", "0", "0", "0", "0", "0", "0", "0", "0", "0", "125", "75", "0", "0", "0", "0", "0", "0", "12345", "0", "10"}
	record, err := parseStat("42 (worker (child)) "+strings.Join(fields, " "), 4096)
	if err != nil {
		t.Fatal(err)
	}
	if record.Name != "worker (child)" || record.ParentPID != 7 || record.StartToken != "12345" || *record.CPUSeconds != 2 || *record.MemoryBytes != 40960 {
		t.Fatal(record)
	}
	if _, err := parseStat("42 (short) S", 4096); err == nil {
		t.Fatal("accepted incomplete metadata")
	}
}

func TestLinuxCollectorCancellationAndMissingMetadata(t *testing.T) {
	collector := NativeCollector{Root: t.TempDir()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := collector.Discover(ctx); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := collector.Inspect(context.Background(), 42); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(collector.Root, "42"), 0700); err != nil {
		t.Fatal(err)
	}
	records, _, err := collector.Discover(context.Background())
	if err != nil || len(records) != 0 {
		t.Fatalf("short-lived process should be skipped: %v", err)
	}
}
