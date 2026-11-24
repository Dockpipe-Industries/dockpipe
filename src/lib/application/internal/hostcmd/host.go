// Package hostcmd owns the generic native-host CLI surface.
package hostcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"dockpipe/src/lib/infrastructure/nativehost"
)

const usage = `dockpipe host — connect to an authorized native user worker.

Usage:
  dockpipe host status --connection /absolute/connection.json --json
  dockpipe host snapshot --connection /absolute/connection.json --json [--system] [--docker]
  dockpipe host terminal --connection /absolute/connection.json [--cwd /absolute/path]
  dockpipe host docker start|stop --connection /absolute/connection.json --id HEX --json
  dockpipe host docker-query --connection /absolute/connection.json --kind container|network|volume --id ID --json
  dockpipe host worker --socket /absolute/socket --nonce 64HEX [--detach]

Connection files contain socket and key-file references, never user credentials.
Status, snapshots and Docker queries have an 8-second limit. Docker queries
inspect one resource; container logs retain at most 200 lines and 64 KiB.
A disconnected or revoked host
fails closed. The worker runs only as its existing ordinary operating-system user.
`

type arguments struct {
	command      string
	connection   string
	socket       string
	nonce        string
	cwd          string
	system       bool
	docker       bool
	detach       bool
	ready        bool
	action       string
	resourceKind string
	resourceID   string
}

func parse(args []string) (arguments, error) {
	var result arguments
	if len(args) == 0 {
		return result, nil
	}
	result.command = args[0]
	start := 1
	if result.command == "docker" {
		if len(args) < 2 || (args[1] != "start" && args[1] != "stop") {
			return result, errors.New("native Docker action must be start or stop")
		}
		result.action = args[1]
		start = 2
	}
	for index := start; index < len(args); index++ {
		argument := args[index]
		switch argument {
		case "--json":
			if result.command != "status" && result.command != "snapshot" && result.command != "docker" && result.command != "docker-query" {
				return result, errors.New("--json requires status, snapshot, docker or docker-query")
			}
		case "--system", "--docker":
			if result.command != "snapshot" {
				return result, errors.New("category flags require snapshot")
			}
			result.system = result.system || argument == "--system"
			result.docker = result.docker || argument == "--docker"
		case "--detach":
			if result.command != "worker" {
				return result, errors.New("--detach requires worker")
			}
			result.detach = true
		case "--connection", "--socket", "--nonce", "--cwd", "--ready-fd", "--id", "--kind":
			index++
			if index == len(args) {
				return result, errors.New("native host option requires a value")
			}
			value := args[index]
			switch argument {
			case "--kind":
				if result.command != "docker-query" || result.resourceKind != "" {
					return result, errors.New("invalid Docker query kind option")
				}
				result.resourceKind = value
			case "--id":
				if (result.command != "docker" && result.command != "docker-query") || result.resourceID != "" {
					return result, errors.New("invalid native Docker identity option")
				}
				result.resourceID = value
			case "--connection":
				if result.command == "worker" || result.connection != "" {
					return result, errors.New("invalid connection option")
				}
				result.connection = value
			case "--socket":
				if result.command != "worker" || result.socket != "" {
					return result, errors.New("invalid worker socket option")
				}
				result.socket = value
			case "--nonce":
				if result.command != "worker" || result.nonce != "" {
					return result, errors.New("invalid worker nonce option")
				}
				result.nonce = value
			case "--cwd":
				if result.command != "terminal" || result.cwd != "" {
					return result, errors.New("invalid terminal directory option")
				}
				result.cwd = value
			case "--ready-fd":
				if result.command != "worker" || value != "3" || result.ready {
					return result, errors.New("invalid worker readiness option")
				}
				result.ready = true
			}
		default:
			return result, fmt.Errorf("unknown native host option %q", argument)
		}
	}
	if result.command == "worker" {
		if result.socket == "" || result.nonce == "" || (result.detach && result.ready) {
			return result, errors.New("worker requires socket and nonce")
		}
	} else if result.command == "status" || result.command == "snapshot" || result.command == "terminal" || result.command == "docker" || result.command == "docker-query" {
		if result.connection == "" {
			return result, errors.New("native host connection reference required")
		}
		if result.command == "docker-query" {
			if err := nativehost.ValidateDockerQuery(result.resourceKind, result.resourceID); err != nil {
				return result, err
			}
		}
		if result.command == "docker" && result.resourceID == "" {
			return result, errors.New("native Docker container ID required")
		}
		if result.command == "docker" {
			if err := nativehost.ValidateDockerAction(result.action, result.resourceID); err != nil {
				return result, err
			}
		}
	} else {
		return result, errors.New("unknown native host command")
	}
	return result, nil
}

func Run(args []string) error {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h")) {
		_, err := io.WriteString(os.Stdout, usage)
		return err
	}
	options, err := parse(args)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, options, os.Stdout)
}

func run(ctx context.Context, options arguments, output io.Writer) error {
	if options.command == "worker" {
		worker := nativehost.WorkerOptions{Socket: options.socket, Nonce: options.nonce}
		if options.detach {
			return nativehost.DetachWorker(ctx, worker)
		}
		if options.ready {
			file := os.NewFile(3, "native-host-ready")
			defer file.Close()
			worker.Ready = func() error {
				_, err := file.Write([]byte{1})
				return errors.Join(err, file.Close())
			}
		}
		return nativehost.RunWorker(ctx, worker)
	}
	client, err := nativehost.NewClient(options.connection)
	if err != nil {
		if options.command == "status" {
			if encodeErr := json.NewEncoder(output).Encode(nativehost.Status{Schema: nativehost.StatusSchema, Reason: "native_host_unavailable"}); encodeErr != nil {
				return errors.Join(nativehost.ErrUnavailable, encodeErr)
			}
		}
		if options.command == "docker" {
			if encodeErr := json.NewEncoder(output).Encode(nativehost.DockerResult{Schema: nativehost.DockerActionSchema, Outcome: "Failure", Action: options.action, ID: options.resourceID, Reason: "native_host_unavailable"}); encodeErr != nil {
				return errors.Join(nativehost.ErrUnavailable, encodeErr)
			}
		}
		if options.command == "docker-query" {
			if encodeErr := json.NewEncoder(output).Encode(nativehost.DockerQueryResult{Schema: nativehost.DockerQuerySchema, Outcome: "Failure", Kind: options.resourceKind, ID: options.resourceID, Reason: "native_host_unavailable"}); encodeErr != nil {
				return errors.Join(nativehost.ErrUnavailable, encodeErr)
			}
		}
		return nativehost.ErrUnavailable
	}
	switch options.command {
	case "docker-query":
		result, err := client.DockerQuery(ctx, options.resourceKind, options.resourceID)
		if encodeErr := json.NewEncoder(output).Encode(result); encodeErr != nil {
			return encodeErr
		}
		return err
	case "docker":
		result, err := client.Docker(ctx, options.action, options.resourceID)
		if encodeErr := json.NewEncoder(output).Encode(result); encodeErr != nil {
			return encodeErr
		}
		return err
	case "status":
		status, err := client.Status(ctx)
		if encodeErr := json.NewEncoder(output).Encode(status); encodeErr != nil {
			return encodeErr
		}
		return err
	case "snapshot":
		if !options.system && !options.docker {
			options.system, options.docker = true, true
		}
		snapshot, err := client.Snapshot(ctx, options.system, options.docker)
		if err != nil {
			return err
		}
		return json.NewEncoder(output).Encode(snapshot)
	case "terminal":
		return relayTerminal(ctx, client, options.cwd)
	}
	return errors.New("unknown native host command")
}
