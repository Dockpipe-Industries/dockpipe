// Package secretenv invokes package-owned resolvers without persisting or logging their output.
package secretenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/secretenv"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

type boundedOutput struct{ bytes.Buffer }

func (output *boundedOutput) Write(data []byte) (int, error) {
	if output.Len()+len(data) > contract.MaxBytes {
		return 0, errors.New("secret resolver output exceeds limit")
	}
	return output.Buffer.Write(data)
}

func Resolve(ctx context.Context, workdir, resolver string, request contract.Request) (map[string]string, error) {
	if request.Schema != contract.Schema {
		return nil, errors.New("unsupported secret environment schema")
	}
	if err := contract.ValidateBindings(request.Bindings); err != nil {
		return nil, err
	}
	profilePath, err := infrastructure.ResolveResolverFilePath(workdir, resolver)
	if err != nil {
		return nil, err
	}
	profile, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return nil, err
	}
	script := filepath.FromSlash(profile["DOCKPIPE_SECRET_ENVIRONMENT_RESOLVE"])
	if !filepath.IsLocal(script) || script == "." {
		return nil, errors.New("resolver must declare a local DOCKPIPE_SECRET_ENVIRONMENT_RESOLVE script")
	}
	scriptPath := filepath.Join(filepath.Dir(profilePath), script)
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Dir(profilePath))
	if err != nil {
		return nil, err
	}
	resolvedScript, err := filepath.EvalSymlinks(scriptPath)
	if err != nil {
		return nil, errors.New("secret environment resolver script is unavailable")
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedScript)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, errors.New("secret environment resolver script escapes its package")
	}
	input, err := json.Marshal(request)
	if err != nil || len(input) > contract.MaxBytes {
		return nil, errors.New("invalid secret environment request")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", resolvedScript)
	command.Dir = workdir
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	command.Env = append(os.Environ(), "DOCKPIPE_BIN="+executable)
	command.Stdin = bytes.NewReader(input)
	var output boundedOutput
	defer func() { clear(output.Bytes()) }()
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if err := process.Run(command); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("secret environment resolution failed; check the resolver CLI, authentication, and references (provider output withheld)")
	}
	var response contract.Response
	if json.Unmarshal(output.Bytes(), &response) != nil {
		return nil, errors.New("secret resolver returned invalid JSON (output withheld)")
	}
	if err := contract.ValidateResponse(request, response); err != nil {
		return nil, err
	}
	return response.Values, nil
}
