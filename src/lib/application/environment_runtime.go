package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
	"dockpipe/src/lib/model/managedenvironment"
)

const environmentRuntimeSchema = "dockpipe.environment.runtime/v1"
const environmentRuntimeRecordSchema = "dockpipe.environment.runtime-record/v1"
const vmInstanceStatusSchema = "dockpipe.vm.instance-status/v1"

var environmentRuntimeRunID = regexp.MustCompile(`^[0-9a-f]{8}$`)

type environmentRuntimeRecord struct {
	Schema     string `json:"schema"`
	InstanceID string `json:"instance_id"`
	Resolver   string `json:"resolver"`
	Workdir    string `json:"workdir"`
	RunID      string `json:"run_id"`
}

type environmentRuntimeStatus struct {
	Schema      string `json:"schema"`
	InstanceID  string `json:"instance_id,omitempty"`
	State       string `json:"state"`
	Reason      string `json:"reason,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Workdir     string `json:"workdir,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

func recordEnvironmentRuntime(args, environ []string) error {
	if len(args) != 1 || !managedenvironment.ValidName(args[0]) {
		return errors.New("environment record-runtime requires the reporting resolver name")
	}

	record := environmentRuntimeRecord{Schema: environmentRuntimeRecordSchema, Resolver: args[0]}
	var stateWorkdir string
	for _, entry := range environ {
		key, value, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case environmentInstanceKey:
			record.InstanceID = value
		case environmentStateWorkdirKey:
			stateWorkdir = value
		case "DOCKPIPE_WORKDIR":
			record.Workdir = value
		case "DOCKPIPE_RUN_ID":
			record.RunID = value
		}
	}
	if !record.valid(record.InstanceID) {
		return errors.New("environment runtime record requires a managed instance and host run scope")
	}

	path, err := environmentRuntimeRecordPath(record.InstanceID)
	if err != nil {
		return err
	}
	expected := filepath.Dir(path)
	relative, err := filepath.Rel(expected, stateWorkdir)
	if !filepath.IsAbs(stateWorkdir) || err != nil || relative != "." {
		return errors.New("environment runtime record is outside the managed instance scope")
	}
	if err := infrastructure.ValidatePrivatePath(expected, true); err != nil {
		return err
	}

	return infrastructure.WritePrivateJSON(path, record)
}

func (record environmentRuntimeRecord) valid(instanceID string) bool {
	return record.Schema == environmentRuntimeRecordSchema && environmentInstanceID.MatchString(instanceID) &&
		record.InstanceID == instanceID && managedenvironment.ValidName(record.Resolver) &&
		filepath.IsAbs(record.Workdir) && environmentRuntimeRunID.MatchString(record.RunID)
}

func environmentRuntimeRecordPath(instanceID string) (string, error) {
	root, err := infrastructure.GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(root, "environment-instances", "runtime", instanceID, "runtime.json"))
}

func readEnvironmentRuntime(instanceID string) (environmentRuntimeRecord, error) {
	var record environmentRuntimeRecord
	path, err := environmentRuntimeRecordPath(instanceID)
	if err != nil {
		return record, err
	}
	if err := infrastructure.ValidatePrivatePath(path, false); err != nil {
		return record, err
	}
	file, err := os.Open(path)
	if err != nil {
		return record, err
	}
	defer file.Close()

	const maximumRecordBytes = 8 << 10
	data, err := io.ReadAll(io.LimitReader(file, maximumRecordBytes+1))
	if err != nil || len(data) > maximumRecordBytes ||
		decodeStorageJSON(data, &record) != nil || !record.valid(instanceID) {
		return environmentRuntimeRecord{}, errors.New("invalid environment runtime record")
	}

	return record, nil
}

func writeEnvironmentStatus(path string, definition managedenvironment.Definition) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	status := observeEnvironmentRuntime(ctx, definition.Kind, observedEnvironmentInstanceID(path))
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return json.NewEncoder(os.Stdout).Encode(status)
}

func observeEnvironmentRuntime(ctx context.Context, kind, instanceID string) environmentRuntimeStatus {
	if kind == "vm" {
		return observeEnvironmentVM(ctx, instanceID)
	}

	status := environmentRuntimeStatus{
		Schema: environmentRuntimeSchema, InstanceID: instanceID, State: "unverified", Reason: "no-runtime-record",
	}
	if instanceID == "" || ctx.Err() != nil {
		return status
	}

	container, err := infrastructure.ObserveDockerEnvironment(ctx, instanceID)
	switch {
	case err != nil:
		status.Reason = "observation-unavailable"
	case container.State == "running":
		status.State, status.Reason = "running", ""
		status.ContainerID = container.ID
	case container.State == "exited" || container.State == "dead" || container.State == "created":
		status.State, status.Reason = "stopped", ""
	}
	return status
}

func observeEnvironmentVM(ctx context.Context, instanceID string) environmentRuntimeStatus {
	status := environmentRuntimeStatus{
		Schema: environmentRuntimeSchema, InstanceID: instanceID, State: "unverified", Reason: "no-runtime-record",
	}
	if instanceID == "" || ctx.Err() != nil {
		return status
	}
	record, err := readEnvironmentRuntime(instanceID)
	if errors.Is(err, os.ErrNotExist) {
		return status
	}
	status.Reason = "observation-unavailable"
	if err != nil {
		return status
	}
	profilePath, err := infrastructure.ResolveResolverFilePath(record.Workdir, record.Resolver)
	if err != nil {
		return status
	}
	profile, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return status
	}
	hook := profile["DOCKPIPE_ENVIRONMENT_INSTANCE_STATUS"]
	if hook == "" {
		return status
	}
	if runtime.GOOS == "windows" {
		hook += ".exe"
	}
	executable, err := infrastructure.ResolveResolverHook(profilePath, hook)
	if err != nil {
		return status
	}
	dockpipe, err := os.Executable()
	if err != nil {
		return status
	}

	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(deadline, executable, "instance-status", instanceID, "--run", record.RunID)
	command.Dir = record.Workdir
	command.Env = []string{"DOCKPIPE_BIN=" + dockpipe, "DOCKPIPE_WORKDIR=" + record.Workdir}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "PATH", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "PROGRAMDATA", "SYSTEMROOT", "WINDIR",
			"TEMP", "TMP", "TMPDIR", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME",
			"XDG_RUNTIME_DIR", "DOCKPIPE_GLOBAL_ROOT", "DOCKPIPE_SYSTEM_ROOT":
			command.Env = append(command.Env, entry)
		}
	}
	command.WaitDelay = time.Second
	var output environmentStorageOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	if err := process.Run(command); err != nil {
		return status
	}

	var reply struct {
		Schema         string `json:"schema"`
		InstanceID     string `json:"instance_id"`
		State          string `json:"state"`
		MatchedRecords int    `json:"matched_records"`
		RequestedRunID string `json:"requested_run_id"`
		RunID          string `json:"run_id"`
	}
	if json.Unmarshal(output.Bytes(), &reply) != nil || reply.Schema != vmInstanceStatusSchema ||
		reply.InstanceID != instanceID || reply.RequestedRunID != record.RunID ||
		reply.RunID != record.RunID || reply.MatchedRecords != 1 ||
		(reply.State != "running" && reply.State != "stopped") {
		return status
	}

	status.State = reply.State
	status.Reason = ""
	status.RunID = reply.RunID
	status.Workdir = record.Workdir
	return status
}
