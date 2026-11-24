package infrastructure

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"dockpipe/src/lib/domain/invocation"
)

type containerExecutionInput struct {
	options   RunOpts
	arguments []string
}

// AdmitContainer admits the selected container command with the already compiled
// runtime policy. It does not reinterpret or weaken that policy. OS descriptors
// remain caller-owned resources; their lifetime is not guaranteed by admission.
func AdmitContainer(admission NativeExecutionAdmission, options RunOpts, arguments []string, execute func(RunOpts, []string) (int, error)) (int, error) {
	if execute == nil {
		return 1, invocation.ErrInvalidContract
	}
	result, err := invokeNativeExecution(admission, "command", containerExecutionInput{options, arguments}, func(_ context.Context, input containerExecutionInput) (invocation.Prepared[int], error) {
		if input.options.Image == "" {
			return invocation.Prepared[int]{}, fmt.Errorf("DOCKPIPE_IMAGE is required")
		}
		fields := containerInvocationFields(input.options)
		groups := [][]string{fields, input.arguments, input.options.ObservationEnv, input.options.ExtraEnv,
			input.options.ExtraMounts, input.options.TmpfsPaths, input.options.SecurityOpt, input.options.CapDrop, input.options.CapAdd}
		for _, group := range groups {
			if err := ValidateNativeExecutionStrings(group); err != nil {
				return invocation.Prepared[int]{}, err
			}
		}
		// Docker accepts bare names in ExtraEnv to request host environment values.
		// Preserve that Docker-specific contract rather than applying host env rules.
		owned := input.options
		owned.ObservationEnv = append([]string(nil), input.options.ObservationEnv...)
		owned.ExtraEnv = append([]string(nil), input.options.ExtraEnv...)
		owned.ExtraMounts = append([]string(nil), input.options.ExtraMounts...)
		owned.TmpfsPaths = append([]string(nil), input.options.TmpfsPaths...)
		owned.SecurityOpt = append([]string(nil), input.options.SecurityOpt...)
		owned.CapDrop = append([]string(nil), input.options.CapDrop...)
		owned.CapAdd = append([]string(nil), input.options.CapAdd...)
		ownedArguments := append([]string(nil), input.arguments...)
		prepared := invocation.Prepared[int]{Execute: func(context.Context) (int, error) { return execute(owned, ownedArguments) }}
		if admission.Authorize != nil {
			prepared.Digest = NativeExecutionDigest(groups...)
		}
		return prepared, nil
	})
	if err != nil && result == 0 {
		return 1, err
	}
	return result, err
}

func containerInvocationFields(options RunOpts) []string {
	return []string{
		"docker", options.Image, options.ObservationWorkdir, options.WorkdirHost, options.WorkdirVolume,
		options.WorkPath, options.ActionPath, options.ContainerUser, options.CPULimit, options.MemoryLimit,
		options.NetworkMode, options.DataVolume, options.DataDir, options.CommitMessage, options.BundleOut,
		options.StdoutTeePath, strconv.Itoa(options.PIDLimit), strconv.FormatBool(options.SkipVolumeWorkspaceSync),
		strconv.FormatBool(options.ReadOnlyRootFS), strconv.FormatBool(options.Reinit), strconv.FormatBool(options.Force),
		strconv.FormatBool(options.Detach), strconv.FormatBool(options.CommitOnHost), strconv.FormatBool(options.BundleAll),
		invocationFileName(options.Stdin), invocationFileName(options.Stdout), invocationFileName(options.Stderr),
	}
}

func invocationFileName(file *os.File) string {
	if file == nil {
		return ""
	}
	return file.Name()
}
