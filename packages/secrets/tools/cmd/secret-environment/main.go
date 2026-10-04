package main

import (
	"context"
	"encoding/json"
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
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
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
