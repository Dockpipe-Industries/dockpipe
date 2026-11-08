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
		if request.Container != "" {
			return errors.New("container snapshot does not accept a container target")
		}
		result, err := infrastructure.CollectLocalContainerInventory(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(result)
	}
	result, err := infrastructure.ControlContainer(ctx, request)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}

func decodeRequest(data []byte) (infrastructure.ContainerControlRequest, error) {
	var request infrastructure.ContainerControlRequest
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
		if !ok || seen[name] || (name != "action" && name != "container") {
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
		if name == "action" {
			request.Action = value
		} else {
			request.Container = value
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
