// Package environment owns provider-specific secret retrieval using authenticated vendor CLIs.
package environment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	contract "dockpipe/src/lib/domain/secretenv"
	"dockpipe/src/lib/infrastructure/process"
)

type Runner func(context.Context, []string, []byte, []string) ([]byte, error)

type limitedBuffer struct{ bytes.Buffer }

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	if buffer.Len()+len(data) > contract.MaxBytes {
		return 0, errors.New("provider response exceeds limit")
	}
	return buffer.Buffer.Write(data)
}

func Run(ctx context.Context, arguments []string, input []byte, environment []string) ([]byte, error) {
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	command.Stdin = bytes.NewReader(input)
	command.Env = environment
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	var output limitedBuffer
	command.Stdout = &output
	if err := process.RunNested(command); err != nil {
		clear(output.Bytes())
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("provider command failed (output withheld)")
	}
	return output.Bytes(), nil
}

func Resolve(ctx context.Context, provider string, request contract.Request, run Runner, executable string) (contract.Response, error) {
	response := contract.Response{Schema: contract.Schema}
	if request.Schema != contract.Schema {
		return response, errors.New("unsupported secret environment schema")
	}
	if err := contract.ValidateBindings(request.Bindings); err != nil {
		return response, err
	}
	var err error
	switch provider {
	case "onepassword":
		response.Values, err = onepassword(ctx, request, run, executable)
	case "infisical":
		response.Values, err = infisical(ctx, request, run, executable)
	case "aws-secretsmanager":
		response.Values, err = aws(ctx, request, run)
	case "azure-keyvault":
		response.Values, err = azure(ctx, request, run)
	default:
		err = errors.New("unknown secret provider")
	}
	if err != nil {
		return contract.Response{}, err
	}
	return response, contract.ValidateResponse(request, response)
}

func parameters(request contract.Request, required []string, optional ...string) error {
	allowed := map[string]bool{}
	for _, name := range required {
		if strings.TrimSpace(request.Parameters[name]) == "" {
			return errors.New("required provider parameter is missing")
		}
		allowed[name] = true
	}
	for _, name := range optional {
		allowed[name] = true
	}
	for name, value := range request.Parameters {
		if !allowed[name] || strings.ContainsRune(value, 0) {
			return errors.New("unsupported provider parameter")
		}
	}
	return nil
}

// Remove requested variables from the parent so missing provider values cannot fall back to it.
func authenticationEnvironment(request contract.Request) []string {
	removed := map[string]bool{}
	for name, reference := range request.Bindings {
		removed[name], removed[reference] = true, true
	}
	var environment []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !removed[name] {
			environment = append(environment, entry)
		}
	}
	return environment
}

func captureWith(ctx context.Context, request contract.Request, run Runner, arguments []string, executable string) (map[string]string, error) {
	arguments = append(arguments, "--", executable, "capture")
	input, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	output, err := run(ctx, arguments, input, authenticationEnvironment(request))
	if err != nil {
		return nil, err
	}
	defer clear(output)
	var response contract.Response
	if json.Unmarshal(output, &response) != nil {
		return nil, errors.New("provider environment response is invalid (output withheld)")
	}
	if err := contract.ValidateResponse(request, response); err != nil {
		return nil, err
	}
	return response.Values, nil
}

func Capture(request contract.Request, lookup func(string) (string, bool)) (contract.Response, error) {
	response := contract.Response{Schema: contract.Schema, Values: map[string]string{}}
	if request.Schema != contract.Schema {
		return response, errors.New("invalid capture request")
	}
	if err := contract.ValidateBindings(request.Bindings); err != nil {
		return response, err
	}
	for name, reference := range request.Bindings {
		value, exists := lookup(reference)
		if !exists {
			return contract.Response{}, errors.New("required variable is absent from provider environment")
		}
		response.Values[name] = value
	}
	return response, contract.ValidateResponse(request, response)
}
