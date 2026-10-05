package remote

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

type boundedLog struct {
	mutex     sync.Mutex
	data      []byte
	truncated bool
}

func (log *boundedLog) Write(data []byte) (int, error) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	remaining := contract.MaxLog - len(log.data)
	if len(data) > remaining {
		log.truncated = true
	}
	log.data = append(log.data, data[:min(len(data), remaining)]...)
	return len(data), nil
}

// Keep the terminal cause visible even when workflow output filled the log.
func (log *boundedLog) appendDiagnostic(message string) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	diagnostic := []byte("\n" + message + "\n")
	if len(diagnostic) > contract.MaxLog {
		diagnostic = diagnostic[:contract.MaxLog]
	}
	retained := min(len(log.data), contract.MaxLog-len(diagnostic))
	if retained < len(log.data) {
		log.truncated = true
	}
	log.data = append(log.data[:retained], diagnostic...)
}

func ValidateProfiles(profiles map[string]contract.Profile) error {
	if len(profiles) == 0 || len(profiles) > 32 {
		return errors.New("configure between 1 and 32 local workflow profiles")
	}
	for name, profile := range profiles {
		if !contract.ValidID(name) || !filepath.IsAbs(profile.Workdir) || profile.TimeoutSeconds < 1 || profile.TimeoutSeconds > 86400 || len(profile.Artifacts) > 32 {
			return errors.New("invalid worker profile bounds")
		}
		if (profile.Workflow == "") == (profile.WorkflowFile == "") {
			return errors.New("profile must select exactly one workflow name or absolute workflow_file")
		}
		if profile.Workflow != "" && (strings.HasPrefix(profile.Workflow, "-") || strings.ContainsAny(profile.Workflow, "\\/:\x00\r\n")) {
			return errors.New("invalid workflow name")
		}
		if profile.WorkflowFile != "" && !filepath.IsAbs(profile.WorkflowFile) {
			return errors.New("workflow_file must be an absolute local path")
		}
		if err := infrastructure.ValidateUnlinkedPath(profile.Workdir); err != nil {
			return errors.New("profile checkout must exist without symlinks")
		}
		info, err := os.Stat(profile.Workdir)
		if err != nil || !info.IsDir() {
			return errors.New("profile checkout must be a directory")
		}
		for _, path := range profile.Artifacts {
			if !SafeRelative(path) {
				return errors.New("profile artifacts must be exact relative file paths")
			}
		}
	}
	return nil
}

// Execute invokes the existing local DockPipe boundary, with argv fixed by a
// locally approved profile. The broker has no generic remote-shell capability.
func Execute(ctx context.Context, executable string, profile contract.Profile) contract.Result {
	result := contract.Result{Status: "failure", ExitCode: -1, StartedAt: time.Now().UTC(), OS: runtime.GOOS, Architecture: runtime.GOARCH}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(profile.TimeoutSeconds)*time.Second)
	defer cancel()
	log := &boundedLog{}
	arguments := []string{"--workdir", profile.Workdir}
	if profile.WorkflowFile != "" {
		arguments = append(arguments, "--workflow-file", profile.WorkflowFile)
	} else {
		arguments = append(arguments, "--workflow", profile.Workflow)
	}
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = profile.Workdir
	command.Stdout = log
	command.Stderr = log
	command.Stdin = nil
	// Do not inherit broker or provider credentials from the worker environment.
	for _, name := range []string{"PATH", "HOME", "USER", "TMPDIR", "LANG", "LC_ALL", "DOCKPIPE_GLOBAL_ROOT"} {
		if value, exists := os.LookupEnv(name); exists {
			command.Env = append(command.Env, name+"="+value)
		}
	}
	command.Env = append(command.Env, "DOCKPIPE_REMOTE_WORKER=1")
	command.WaitDelay = 2 * time.Second
	err := process.Run(command)
	if command.ProcessState != nil {
		result.ExitCode = command.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		if errors.Is(context.Cause(ctx), context.Canceled) {
			result.Status = "cancelled"
		} else {
			log.appendDiagnostic("DockPipe workflow stopped: " + context.Cause(ctx).Error())
		}
	} else if err == nil {
		result.Status = "success"
	} else {
		log.appendDiagnostic("DockPipe workflow process failed.")
	}
	result.Log = string(log.data)
	result.LogTruncated = log.truncated
	result.FinishedAt = time.Now().UTC()
	if result.Status == "success" {
		result.Artifacts, err = collectArtifacts(profile)
		if err != nil {
			result.Status = "failure"
			result.ExitCode = -1
			result.Log = "Workflow completed, but artifact collection failed: " + err.Error()
		}
	}
	return result
}

func collectArtifacts(profile contract.Profile) (map[string][]byte, error) {
	artifacts := make(map[string][]byte)
	root, err := os.OpenRoot(profile.Workdir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	total := 0
	for _, name := range profile.Artifacts {
		if !SafeRelative(name) {
			return nil, errors.New("invalid artifact path")
		}
		if err := infrastructure.ValidateUnlinkedPath(filepath.Join(profile.Workdir, name)); err != nil {
			return nil, errors.New("artifact is missing or linked")
		}
		file, err := root.Open(name)
		if err != nil {
			return nil, err
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > int64(contract.MaxArtifacts-total) {
			_ = file.Close()
			return nil, errors.New("artifact is not a bounded regular file")
		}
		data, err := io.ReadAll(io.LimitReader(file, int64(contract.MaxArtifacts-total)+1))
		closeErr := file.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		total += len(data)
		if total > contract.MaxArtifacts {
			return nil, errors.New("artifact total exceeds 8 MiB")
		}
		artifacts[name] = data
	}
	return artifacts, nil
}
