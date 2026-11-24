package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/domain/invocation"
	"dockpipe/src/lib/infrastructure/process"
)

var composeExecCommandFn = exec.CommandContext

type ComposeLifecycleOpts struct {
	Action           string
	File             string
	Project          string
	ProjectDirectory string
	Services         []string
	Env              []string
}

func RunComposeLifecycle(opts ComposeLifecycleOpts) error {
	return RunComposeLifecycleWithAdmission(NativeExecutionAdmission{}, opts)
}

// Compose remains a typed lifecycle adapter. Its file and service selection is
// captured in memory before any Docker or filesystem discovery; the executor
// retains Compose interpretation and the existing two-minute operation budget.
func RunComposeLifecycleWithAdmission(admission NativeExecutionAdmission, opts ComposeLifecycleOpts) error {
	_, err := invokeNativeExecution(admission, "dockpipe.containers.compose", opts,
		func(_ context.Context, input ComposeLifecycleOpts) (invocation.Prepared[struct{}], error) {
			owned := input
			owned.Services = append([]string(nil), input.Services...)
			owned.Env = append([]string(nil), input.Env...)
			fields := []string{owned.Action, owned.File, owned.Project, owned.ProjectDirectory}
			for _, group := range [][]string{fields, owned.Services, owned.Env} {
				if err := ValidateNativeExecutionStrings(group); err != nil {
					return invocation.Prepared[struct{}]{}, err
				}
			}
			arguments, err := composeLifecycleArguments(owned)
			if err != nil {
				return invocation.Prepared[struct{}]{}, err
			}
			environment := mergeComposeProcessEnv(owned.Env)
			prepared := invocation.Prepared[struct{}]{Execute: func(ctx context.Context) (struct{}, error) {
				return struct{}{}, runComposeLifecycle(ctx, arguments, environment)
			}}
			if admission.Authorize != nil {
				prepared.Digest = NativeExecutionDigest(fields, owned.Services, owned.Env, environment)
			}
			return prepared, nil
		})
	return err
}

func composeLifecycleArguments(opts ComposeLifecycleOpts) ([]string, error) {
	file := strings.TrimSpace(opts.File)
	if file == "" {
		return nil, fmt.Errorf("compose file is required")
	}
	action := strings.TrimSpace(opts.Action)
	if action == "" {
		return nil, fmt.Errorf("compose action is required")
	}
	projectDir := strings.TrimSpace(opts.ProjectDirectory)
	if projectDir == "" {
		projectDir = filepath.Dir(file)
	}
	args := []string{"compose"}
	if project := strings.TrimSpace(opts.Project); project != "" {
		args = append(args, "-p", project)
	}
	args = append(args, "-f", file)
	if projectDir != "" {
		args = append(args, "--project-directory", projectDir)
	}
	switch action {
	case "up":
		args = append(args, "up", "-d", "--remove-orphans")
	case "down":
		args = append(args, "down")
	case "ps":
		args = append(args, "ps")
	default:
		return nil, fmt.Errorf("unsupported compose action %q", action)
	}
	for _, service := range opts.Services {
		service = strings.TrimSpace(service)
		if strings.HasPrefix(service, "-") {
			return nil, fmt.Errorf("compose services cannot select command options")
		}
		if service != "" {
			args = append(args, service)
		}
	}
	return args, nil
}

func runComposeLifecycle(parent context.Context, arguments, environment []string) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	name, err := resolveDockerCommandFromEnvironment(environment)
	if err != nil {
		return err
	}
	cmd := composeExecCommandFn(ctx, name, arguments...)
	cmd.Env = dockerCommandEnv(environment, name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = time.Second
	err = process.Run(cmd)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func mergeComposeProcessEnv(overrides []string) []string {
	base := append([]string(nil), os.Environ()...)
	for _, pair := range overrides {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, _, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		prefix := key + "="
		replaced := false
		for i, existing := range base {
			if strings.HasPrefix(existing, prefix) {
				base[i] = pair
				replaced = true
				break
			}
		}
		if !replaced {
			base = append(base, pair)
		}
	}
	return base
}
