package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure/telemetry"
)

// RegisterExecutionResource is called by an existing runtime after launching a
// resource. It binds to a live host-run identity, never to executable names.
func RegisterExecutionResource(ctx context.Context, workdir, runID, runtimeName, resourceID string, pid int) error {
	if workdir == "" || !isValidHostRunID(runID) || runtimeName == "" || len(runtimeName) > 32 || resourceID == "" || len(resourceID) > 128 || strings.ContainsAny(runtimeName+resourceID, "\x00\r\n") {
		return errors.New("invalid runtime association")
	}
	directory := HostRunsDir(workdir)
	if err := ValidateUnlinkedPath(directory); err != nil {
		return errors.New("run directory unavailable")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.Open(runID + ".json")
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	file.Close()
	if err != nil || len(data) > 8192 {
		return errors.New("host run metadata unavailable")
	}
	var host HostRunRecord
	if json.Unmarshal(data, &host) != nil || host.ID != runID {
		return errors.New("host run identity mismatch")
	}
	collector := telemetry.NativeCollector{}
	owner, err := collector.Inspect(ctx, host.PID)
	if err != nil || !telemetry.SameProcess(telemetry.ProcessIdentity{PID: host.PID, StartToken: host.ProcessStart}, owner.ProcessIdentity) {
		return errors.New("host run process identity unavailable or changed")
	}
	process, err := collector.Inspect(ctx, pid)
	if err != nil {
		return errors.New("resource process identity unavailable")
	}
	resource := telemetry.Resource{ID: resourceID, Runtime: runtimeName, Process: process.ProcessIdentity}
	data, err = json.Marshal(resource)
	if err != nil {
		return err
	}
	// A run owns one runtime resource. Re-registration must not overwrite an
	// earlier association; cleanup follows the existing host-run sidecar lifetime.
	file, err = root.OpenFile(runID+".resource", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func readExecutionResource(workdir, runID string) *telemetry.Resource {
	if workdir == "" || !isValidHostRunID(runID) {
		return nil
	}
	path := filepath.Join(HostRunsDir(workdir), runID+".resource")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 8192 {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(data) > 8192 {
		return nil
	}
	var resource telemetry.Resource
	if json.Unmarshal(data, &resource) != nil || resource.ID == "" || resource.Process.StartToken == "" {
		return nil
	}
	return &resource
}
