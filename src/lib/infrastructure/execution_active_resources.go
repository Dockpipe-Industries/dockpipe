package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure/telemetry"
)

// Active runtime associations outlive the bounded recent execution history.
// Recover them only from the runtime registry and an exact live process identity.
func mergeActiveExecutionResources(ctx context.Context, workdir string, records []telemetry.Execution, collector telemetry.Collector) ([]telemetry.Execution, bool, error) {
	if workdir == "" {
		return records, false, nil
	}
	absoluteWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return records, false, err
	}
	workdir = absoluteWorkdir
	directory := HostRunsDir(workdir)
	if err := ValidateUnlinkedPath(directory); err != nil {
		return records, false, nil
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return records, false, nil
	}
	defer root.Close()
	entries, err := root.Open(".")
	if err != nil {
		return records, false, nil
	}
	defer entries.Close()
	seen := make(map[string]bool, len(records))
	for _, record := range records {
		seen[record.ID] = true
	}
	active := make(map[string]bool)
	truncated := false
	for scanned := 0; scanned < telemetry.MaxProcesses; scanned++ {
		if err := ctx.Err(); err != nil {
			return records, truncated, err
		}
		batch, err := entries.ReadDir(1)
		if err == io.EOF {
			break
		}
		if err != nil {
			return records, truncated, err
		}
		if scanned == telemetry.MaxProcesses-1 {
			truncated = true
		}
		name := batch[0].Name()
		id := strings.TrimSuffix(name, ".json")
		if !strings.HasSuffix(name, ".json") || !isValidHostRunID(id) || readExecutionResource(workdir, id) == nil {
			continue
		}
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8192 {
			continue
		}
		file, err := root.Open(name)
		if err != nil {
			continue
		}
		opened, statErr := file.Stat()
		data, readErr := io.ReadAll(io.LimitReader(file, 8193))
		closeErr := file.Close()
		if statErr != nil || readErr != nil || closeErr != nil {
			continue
		}
		var host HostRunRecord
		if !os.SameFile(info, opened) || len(data) > 8192 || json.Unmarshal(data, &host) != nil || host.ID != id {
			continue
		}
		started, err := time.Parse(time.RFC3339, host.StartedAt)
		if err != nil || filepath.Clean(host.Workdir) != filepath.Clean(workdir) {
			continue
		}
		identity := telemetry.ProcessIdentity{PID: host.PID, StartToken: host.ProcessStart}
		process, err := collector.Inspect(ctx, host.PID)
		if err != nil || !telemetry.SameProcess(identity, process.ProcessIdentity) {
			continue
		}
		active[id] = true
		if !seen[id] {
			records = append(records, telemetry.Execution{ID: id, Runtime: "host", Machine: "local", State: "running", Process: identity, StartedAt: started})
			seen[id] = true
		}
	}
	sort.SliceStable(records, func(left, right int) bool {
		if active[records[left].ID] != active[records[right].ID] {
			return active[records[left].ID]
		}
		return records[left].StartedAt.After(records[right].StartedAt)
	})
	if len(records) > telemetry.MaxExecutions {
		records = records[:telemetry.MaxExecutions]
		truncated = true
	}
	return records, truncated, nil
}
