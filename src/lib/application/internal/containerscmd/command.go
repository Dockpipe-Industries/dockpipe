package containerscmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unicode/utf8"

	"dockpipe/src/lib/infrastructure"
)

const maxRequestBytes = 4096

func Run(arguments []string) error {
	if len(arguments) > 0 && arguments[0] == "exec" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runExec(ctx, arguments[1:], os.Stdin, os.Stdout, os.Stderr)
	}
	if len(arguments) != 2 || arguments[0] != "--request" {
		return errors.New("usage: dockpipe containers --request <file.json|->")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	input := os.Stdin
	if arguments[1] != "-" {
		info, err := os.Stat(arguments[1])
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > maxRequestBytes {
			return errors.New("container request must be a regular JSON file up to 4 KiB")
		}
		file, err := openRequest(arguments[1])
		if err != nil {
			return err
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil || !opened.Mode().IsRegular() {
			return errors.New("container request must be a regular JSON file")
		}
		input = file
	}
	stopReadCancellation := context.AfterFunc(ctx, func() { _ = input.Close() })
	defer stopReadCancellation()
	return handle(ctx, input, os.Stdout)
}

func handle(ctx context.Context, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(input, maxRequestBytes+1))
	if err := ctx.Err(); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	if len(data) > maxRequestBytes || !utf8.Valid(data) {
		return errors.New("container request must be UTF-8 JSON up to 4 KiB")
	}
	request, err := decodeRequest(data)
	if err != nil {
		return err
	}
	if request.Action == "snapshot" {
		if request.Container != "" || request.Kind != "" || request.Object != "" {
			return errors.New("container snapshot does not accept a container target")
		}
		result, err := infrastructure.CollectLocalContainerInventory(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(result)
	}
	if request.Action == "inspect" || request.Action == "logs" {
		if request.Container != "" {
			return errors.New("container read selects kind and object, not a control target")
		}
		result, err := infrastructure.ReadContainer(ctx, infrastructure.ContainerReadRequest{Action: request.Action, Kind: request.Kind, Object: request.Object})
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(result)
	}
	if request.Kind != "" || request.Object != "" {
		return errors.New("container control does not accept read targets")
	}
	result, err := infrastructure.ControlContainer(ctx, infrastructure.ContainerControlRequest{Action: request.Action, Container: request.Container})
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

type containerRequest struct {
	Action    string
	Container string
	Kind      string
	Object    string
}

func decodeRequest(data []byte) (containerRequest, error) {
	var request containerRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return request, errors.New("container request must be one JSON object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		field, err := decoder.Token()
		if err != nil {
			return request, err
		}
		name, ok := field.(string)
		if !ok || seen[name] || (name != "action" && name != "container" && name != "kind" && name != "object") {
			return request, errors.New("unknown or duplicate container request field")
		}
		seen[name] = true
		fieldValue, err := decoder.Token()
		if err != nil {
			return request, err
		}
		value, ok := fieldValue.(string)
		if !ok {
			return request, errors.New("container request fields must be strings")
		}
		switch name {
		case "action":
			request.Action = value
		case "container":
			request.Container = value
		case "kind":
			request.Kind = value
		case "object":
			request.Object = value
		}
	}
	if _, err := decoder.Token(); err != nil {
		return request, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return request, errors.New("container request must contain one JSON object")
	}
	return request, nil
}

func runExec(ctx context.Context, arguments []string, stdin, stdout, stderr *os.File) error {
	request, err := decodeExecArguments(arguments)
	if err != nil {
		return err
	}
	return infrastructure.ExecLocalContainerWithAdmission(infrastructure.NativeExecutionAdmission{Context: ctx}, request, stdin, stdout, stderr)
}

func decodeExecArguments(arguments []string) (infrastructure.ContainerExecRequest, error) {
	var request infrastructure.ContainerExecRequest
	if len(arguments) != 4 {
		return request, errors.New("usage: dockpipe containers exec --container <full-id> --shell </bin/sh|/bin/bash>")
	}
	seen := map[string]bool{}
	for index := 0; index < len(arguments); index += 2 {
		flag, value := arguments[index], arguments[index+1]
		if seen[flag] {
			return request, errors.New("unknown or duplicate container exec flag")
		}
		seen[flag] = true
		switch flag {
		case "--container":
			request.Container = value
		case "--shell":
			request.Shell = value
		default:
			return request, errors.New("unknown or duplicate container exec flag")
		}
	}
	return request, infrastructure.ValidateContainerExecRequest(request)
}
