// Package secretenv invokes package-owned resolvers without persisting or logging their output.
package secretenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/secretenv"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/process"
)

var interactiveLogin = func() bool {
	return loginAllowed(os.Getenv("CI"), term.IsTerminal(int(os.Stdin.Fd())), term.IsTerminal(int(os.Stderr.Fd())))
}

func loginAllowed(ci string, inputTerminal, outputTerminal bool) bool {
	ci = strings.ToLower(strings.TrimSpace(ci))
	return (ci == "" || ci == "0" || ci == "false") && inputTerminal && outputTerminal
}

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
	scriptPath, err := resolverScript(profilePath, profile["DOCKPIPE_SECRET_ENVIRONMENT_RESOLVE"])
	if err != nil {
		return nil, err
	}
	input, err := json.Marshal(request)
	if err != nil || len(input) > contract.MaxBytes {
		return nil, errors.New("invalid secret environment request")
	}
	values, err := resolveProcess(ctx, workdir, scriptPath, input, request)
	if !errors.Is(err, contract.ErrAuthenticationRequired) || !interactiveLogin() {
		return values, err
	}
	if profile["DOCKPIPE_SECRET_ENVIRONMENT_AUTHENTICATE"] == "" {
		return nil, contract.ErrAuthenticationRequired
	}
	authenticatePath, err := resolverScript(profilePath, profile["DOCKPIPE_SECRET_ENVIRONMENT_AUTHENTICATE"])
	if err != nil {
		return nil, err
	}
	if err := authenticate(ctx, workdir, authenticatePath, input); err != nil {
		return nil, err
	}
	// One login attempt, then one resolution. Never loop on provider failures.
	return resolveProcess(ctx, workdir, scriptPath, input, request)
}

func resolverScript(profilePath, declared string) (string, error) {
	script := filepath.FromSlash(declared)
	if !filepath.IsLocal(script) || script == "." {
		return "", errors.New("secret environment hook must declare a local script")
	}
	scriptPath := filepath.Join(filepath.Dir(profilePath), script)
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Dir(profilePath))
	if err != nil {
		return "", err
	}
	resolvedScript, err := filepath.EvalSymlinks(scriptPath)
	if err != nil {
		return "", errors.New("secret environment resolver script is unavailable")
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedScript)
	if err != nil || !filepath.IsLocal(relative) {
		return "", errors.New("secret environment resolver script escapes its package")
	}
	return resolvedScript, nil
}

func resolveProcess(ctx context.Context, workdir, script string, input []byte, request contract.Request) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", script)
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
		var failure contract.Response
		if json.Unmarshal(output.Bytes(), &failure) == nil && contract.AuthenticationRequired(failure) {
			return nil, contract.ErrAuthenticationRequired
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

func authenticate(ctx context.Context, workdir, script string, input []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "bash", script)
	command.Dir = workdir
	command.Env = append(os.Environ(), "DOCKPIPE_BIN="+executable)
	command.Stdin = bytes.NewReader(input)
	command.Stdout = io.Discard
	// This hook owns a login UI, never secret resolution. Providers must emit
	// only safe progress and browser challenges on this separate channel.
	command.Stderr = os.Stderr
	command.WaitDelay = time.Second
	fmt.Fprintln(os.Stderr, "[dockpipe] vault: authentication required; complete the browser sign-in to continue")
	if err := process.Run(command); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("secret environment login failed; check the resolver login configuration; no automatic retry was made")
	}
	return nil
}
