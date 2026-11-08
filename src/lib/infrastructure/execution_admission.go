package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/nativeinvocation"
)

// NativeExecutionAdmission is supplied by trusted local composition, never by
// workflow YAML, process environment, or a remote request. It admits the whole
// script/command, not inferred operations inside arbitrary executable code.
// Context is checked before dispatch. Existing executors continue to own active
// process cancellation and cleanup; admission does not add a process supervisor.
type NativeExecutionAdmission struct {
	Context   context.Context
	Authorize invocation.Authorize
}

func (admission NativeExecutionAdmission) ExecutionContext() context.Context {
	if admission.Context != nil {
		return admission.Context
	}
	return context.Background()
}

// NativeExecutionDigest binds ordered typed fields without serializing local
// requests or placing their potentially secret values in authorization records.
// Call only when applicable authorization needs exact input binding.
func NativeExecutionDigest(groups ...[]string) string {
	digest := sha256.New()
	var size [8]byte
	for _, group := range groups {
		binary.LittleEndian.PutUint64(size[:], uint64(len(group)))
		_, _ = digest.Write(size[:])
		for _, value := range group {
			binary.LittleEndian.PutUint64(size[:], uint64(len(value)))
			_, _ = digest.Write(size[:])
			_, _ = digest.Write([]byte(value))
		}
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// ValidateNativeExecutionStrings checks the OS argument representation. Domain
// owners remain responsible for interpreting commands, shells and workflow flags.
func ValidateNativeExecutionStrings(values []string) error {
	for _, value := range values {
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("native execution arguments cannot contain NUL")
		}
	}
	return nil
}

func ValidateNativeExecutionEnvironment(environment []string) error {
	if err := ValidateNativeExecutionStrings(environment); err != nil {
		return err
	}
	for _, entry := range environment {
		// Windows may include hidden drive-current-directory entries (=C:=...).
		separator := strings.IndexByte(entry, '=')
		if separator < 0 || separator == 0 && (len(entry) < 4 || entry[2] != ':' || entry[3] != '=') {
			return fmt.Errorf("native execution environment requires named assignments")
		}
	}
	return nil
}

func invokeNativeExecution[Input, Output any](admission NativeExecutionAdmission, capability string, input Input, prepare invocation.Prepare[Input, Output]) (Output, error) {
	target, identity := nativeinvocation.Local()
	definition := invocation.Definition{
		ID:                   invocation.ID{Capability: capability, Action: "run", Version: "1"},
		Target:               target,
		RequireAuthorization: admission.Authorize != nil,
	}
	contract, err := invocation.New(definition, prepare, admission.Authorize)
	if err != nil {
		var zero Output
		return zero, err
	}
	return contract.Invoke(admission.ExecutionContext(), invocation.Call{
		Contract: definition.ID, Target: target, Identity: identity,
	}, input)
}

type hostExecutionInput struct {
	text        string
	shell       string
	environment []string
}

func prepareHostExecution[Output any](admission NativeExecutionAdmission, input hostExecutionInput, mode string, execute func(hostExecutionInput) (Output, error)) (invocation.Prepared[Output], error) {
	if err := ValidateNativeExecutionStrings([]string{input.text, input.shell}); err != nil {
		return invocation.Prepared[Output]{}, err
	}
	if err := ValidateNativeExecutionEnvironment(input.environment); err != nil {
		return invocation.Prepared[Output]{}, err
	}
	if mode != "inline" && strings.TrimSpace(input.text) == "" {
		return invocation.Prepared[Output]{}, fmt.Errorf("host script path is required")
	}
	path := ""
	if mode != "inline" {
		path = input.text
	}
	if _, err := hostShellForScript(input.shell, path, runtime.GOOS); err != nil {
		return invocation.Prepared[Output]{}, err
	}
	owned := input
	owned.environment = append([]string(nil), input.environment...)
	prepared := invocation.Prepared[Output]{Execute: func(context.Context) (Output, error) { return execute(owned) }}
	if admission.Authorize != nil {
		prepared.Digest = NativeExecutionDigest([]string{mode, owned.text, owned.shell}, owned.environment)
	}
	return prepared, nil
}

// AdmitHostScript captures the actual domain adapter selected by native
// composition. It must not be exposed as a transport accepting arbitrary code.
func AdmitHostScript(admission NativeExecutionAdmission, path string, environment []string, shell string, execute func(string, []string, string) error) error {
	if execute == nil {
		return invocation.ErrInvalidContract
	}
	_, err := invokeNativeExecution(admission, "script", hostExecutionInput{path, shell, environment}, func(_ context.Context, input hostExecutionInput) (invocation.Prepared[struct{}], error) {
		return prepareHostExecution(admission, input, "file", func(owned hostExecutionInput) (struct{}, error) {
			return struct{}{}, execute(owned.text, owned.environment, owned.shell)
		})
	})
	return err
}

func AdmitHostCommand(admission NativeExecutionAdmission, command string, environment []string, shell string, execute func(string, []string, string) error) error {
	if execute == nil {
		return invocation.ErrInvalidContract
	}
	_, err := invokeNativeExecution(admission, "command", hostExecutionInput{command, shell, environment}, func(_ context.Context, input hostExecutionInput) (invocation.Prepared[struct{}], error) {
		// Preserve the existing empty command no-op without interpreting a shell.
		if strings.TrimSpace(input.text) == "" {
			prepared := invocation.Prepared[struct{}]{Execute: func(context.Context) (struct{}, error) { return struct{}{}, nil }}
			if admission.Authorize != nil {
				prepared.Digest = NativeExecutionDigest([]string{input.text, input.shell}, input.environment)
			}
			return prepared, nil
		}
		return prepareHostExecution(admission, input, "inline", func(owned hostExecutionInput) (struct{}, error) {
			return struct{}{}, execute(owned.text, owned.environment, owned.shell)
		})
	})
	return err
}

func AdmitSourceHostScript(admission NativeExecutionAdmission, path string, environment []string, execute func(string, []string) (map[string]string, error)) (map[string]string, error) {
	if execute == nil {
		return nil, invocation.ErrInvalidContract
	}
	return invokeNativeExecution(admission, "script", hostExecutionInput{path, "bash", environment}, func(_ context.Context, input hostExecutionInput) (invocation.Prepared[map[string]string], error) {
		return prepareHostExecution(admission, input, "source", func(owned hostExecutionInput) (map[string]string, error) {
			return execute(owned.text, owned.environment)
		})
	})
}
