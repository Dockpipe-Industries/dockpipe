//go:build linux

// This qualification probe reports identity only. It cannot launch user commands.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http/cgi" // #nosec G504 -- Supported Go >=1.25 fixes Httpoxy; this diagnostic serves CGI and does not create an environment-proxied client.
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if filepath.Base(os.Args[0]) == "host-auth.cgi" {
		if err := runCGI(); err != nil {
			// The HTTP response deliberately contains no diagnostic or request data.
			emitCGIFailure("503 Service Unavailable", "unavailable")
		}
		return
	}
	if err := runCommand(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Qualification service failed:", err)
		os.Exit(1)
	}
}

func runCGI() error {
	environment, err := authenticationEnvironment(os.Environ())
	if err != nil {
		emitCGIFailure("400 Bad Request", "invalid_request")
		return nil
	}
	handler := authenticationHandler(environment, authenticate, queryService)
	return cgi.Serve(handler)
}

func emitCGIFailure(status, reason string) {
	// Both values are fixed literals at call sites, never request-derived data.
	fmt.Printf("Status: %s\r\nCache-Control: no-store\r\nContent-Type: application/json\r\nContent-Security-Policy: default-src 'none'; frame-ancestors 'self'\r\nX-Content-Type-Options: nosniff\r\n\r\n{\"error\":%q}\n", status, reason)
}

func runCommand(arguments []string) error {
	if len(arguments) == 0 || (arguments[0] != "serve" && arguments[0] != "status") {
		return errors.New("usage: host-probe {serve|status} --socket /absolute/private-directory/probe.sock")
	}
	flags := flag.NewFlagSet(arguments[0], flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	socket := flags.String("socket", "", "absolute Unix socket in an existing private directory")
	authListen := flags.String("auth-listen", "", "optional fixed 127.0.0.1:38140 DSM report listener")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected service arguments")
	}
	if arguments[0] == "status" {
		if *authListen != "" {
			return errors.New("auth-listen is only valid for serve")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		result := querySocket(ctx, *socket)
		if !result.Available {
			return errors.New("qualification service unavailable")
		}
		return json.NewEncoder(os.Stdout).Encode(result.Status)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return serveWithAuthentication(ctx, *socket, *authListen)
}
