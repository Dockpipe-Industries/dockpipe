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

type dockerImageInput struct {
	action        string
	image         string
	dockerfileDir string
	contextDir    string
}

var dockerImageCommandFn = exec.CommandContext

// Standalone image operations admit their typed selection before executable or
// filesystem discovery. They retain the existing caller-owned lifetime; supplied
// cancellation now also stops the Docker process tree. Builds and pulls have no
// arbitrary wall-clock cap because their duration depends on the selected image.
func runDockerImageOperation(admission NativeExecutionAdmission, input dockerImageInput) (bool, error) {
	return invokeNativeExecution(admission, "dockpipe.containers.image", input, func(_ context.Context, owned dockerImageInput) (invocation.Prepared[bool], error) {
		owned.image = strings.TrimSpace(owned.image)
		if owned.image == "" || strings.HasPrefix(owned.image, "-") {
			return invocation.Prepared[bool]{}, fmt.Errorf("a Docker image reference is required")
		}
		fields := []string{owned.action, owned.image, owned.dockerfileDir, owned.contextDir}
		if err := ValidateNativeExecutionStrings(fields); err != nil {
			return invocation.Prepared[bool]{}, err
		}
		if owned.action == "build" && (owned.contextDir == "" || strings.HasPrefix(owned.contextDir, "-")) {
			return invocation.Prepared[bool]{}, fmt.Errorf("a Docker build context is required")
		}
		environment := os.Environ()
		prepared := invocation.Prepared[bool]{Execute: func(ctx context.Context) (bool, error) {
			name, err := resolveDockerCommandFromEnvironment(environment)
			if err != nil {
				return false, err
			}
			var arguments []string
			switch owned.action {
			case "build":
				arguments = []string{"build", "-q", "-t", owned.image, "-f", filepath.Join(owned.dockerfileDir, "Dockerfile"), owned.contextDir}
			case "pull":
				arguments = []string{"pull", owned.image}
			case "exists":
				arguments = []string{"image", "inspect", owned.image}
			default:
				return false, invocation.ErrInvalidContract
			}
			command := dockerImageCommandFn(ctx, name, arguments...)
			command.Env = dockerCommandEnv(environment, name)
			command.WaitDelay = time.Second
			if owned.action == "build" {
				command.Env = dockerBuildEnv(environment, name)
			}
			if owned.action != "exists" {
				command.Stdout, command.Stderr = os.Stdout, os.Stderr
			}
			err = process.Run(command)
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			if owned.action == "exists" {
				if _, exited := err.(*exec.ExitError); exited {
					return false, nil
				}
			}
			return err == nil, err
		}}
		if admission.Authorize != nil {
			prepared.Digest = NativeExecutionDigest(fields, environment)
		}
		return prepared, nil
	})
}

func DockerBuildWithAdmission(admission NativeExecutionAdmission, image, dockerfileDir, contextDir string) error {
	_, err := runDockerImageOperation(admission, dockerImageInput{action: "build", image: image, dockerfileDir: dockerfileDir, contextDir: contextDir})
	return err
}

func DockerImageExistsWithAdmission(admission NativeExecutionAdmission, image string) (bool, error) {
	return runDockerImageOperation(admission, dockerImageInput{action: "exists", image: image})
}

func DockerPullWithAdmission(admission NativeExecutionAdmission, image string) error {
	_, err := runDockerImageOperation(admission, dockerImageInput{action: "pull", image: image})
	return err
}
