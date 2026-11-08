package workspacefilescmd

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

	"dockpipe/src/lib/infrastructure/nativeinvocation"
	"dockpipe/src/lib/infrastructure/workspacefiles"
)

const maxRequestBytes = 2 << 20

// Envelope is a local CLI request. Remote protocols select a worker-local root
// from grants instead; they must never forward this caller-selected Root field.
type Envelope struct {
	Root    string                 `json:"root"`
	Request workspacefiles.Request `json:"request"`
}

func Run(arguments []string) error {
	if len(arguments) != 2 || arguments[0] != "--request" {
		return errors.New("usage: dockpipe files --request <file.json|->")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	input := os.Stdin
	if arguments[1] != "-" {
		file, err := openRequest(arguments[1])
		if err != nil {
			return err
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil || !opened.Mode().IsRegular() {
			return errors.New("workspace file request must be a regular JSON file")
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
		return errors.New("workspace file request must be UTF-8 JSON up to 2 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("workspace file request must contain one JSON object")
	}
	// The local native entrypoint, not request JSON, asserts its own process.
	// This is not a remote authentication credential or a permission grant.
	target, identity := nativeinvocation.Local()
	service, err := workspacefiles.New(envelope.Root, target, identity, nil)
	if err != nil {
		return err
	}
	defer service.Close()
	result, err := service.Run(ctx, envelope.Request)
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(result)
}
