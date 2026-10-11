package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockpipe/packages/secrets/tools/environment"
	contract "dockpipe/src/lib/domain/secretenv"
)

func main() {
	if err := run(); err != nil {
		if errors.Is(err, contract.ErrAuthenticationRequired) {
			_ = json.NewEncoder(os.Stdout).Encode(contract.Response{Schema: contract.Schema, Error: "authentication_required"})
		}
		fmt.Fprintln(os.Stderr, "secret environment resolution failed; check provider authentication and configuration")
		os.Exit(1)
	}
}

func run() error {
	input, err := io.ReadAll(io.LimitReader(os.Stdin, contract.MaxBytes+1))
	if err != nil || len(input) > contract.MaxBytes || len(os.Args) != 2 {
		return fmt.Errorf("invalid request")
	}
	var request contract.Request
	if err := json.Unmarshal(input, &request); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	timeout := 90 * time.Second
	if os.Args[1] == "infisical-login" {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if os.Args[1] == "infisical-login" {
		return environment.AuthenticateInfisical(ctx, request, environment.Run, environment.BrowserLogin(os.Stderr))
	}
	var response contract.Response
	if os.Args[1] == "capture" {
		response, err = environment.Capture(request, os.LookupEnv)
	} else {
		executable, executableErr := os.Executable()
		if executableErr != nil {
			return executableErr
		}
		response, err = environment.Resolve(ctx, os.Args[1], request, environment.Run, executable)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(response)
}
