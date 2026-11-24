//go:build linux

// Package dsmauth owns the bounded fixed DSM session authentication boundary.
package dsmauth

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"dockpipe/workflows/internal/synology/boundedcmd"
)

const HelperPath = "/usr/syno/synoman/webman/modules/authenticate.cgi"
const Timeout = 3 * time.Second
const MaximumHelperOutput = 4096

var ErrUnauthenticated = errors.New("unauthenticated")
var ErrFault = errors.New("authentication unavailable")

type Failure struct {
	Reason   string
	ExitCode *int
}

func (failure *Failure) Error() string {
	return ErrFault.Error()
}
func (failure *Failure) Unwrap() error {
	return ErrFault
}

// The qualification endpoint allows 16 KiB cookies plus 16 KiB of CGI routing
// metadata. Nothing from the general service environment reaches the helper.
func Environment(environment []string) ([]string, error) {
	allowed := map[string]bool{
		"HTTP_COOKIE": true, "REMOTE_ADDR": true, "SERVER_ADDR": true,
		"SERVER_NAME": true, "SERVER_PORT": true, "SERVER_PROTOCOL": true,
		"HTTP_HOST": true, "GATEWAY_INTERFACE": true,
		"REQUEST_METHOD": true, "REQUEST_URI": true, "SCRIPT_NAME": true,
		"QUERY_STRING": true, "HTTPS": true, "HTTP_USER_AGENT": true,
		"HTTP_X_SYNO_TOKEN": true,
	}
	result := []string{"PATH=/usr/bin:/bin", "LANG=C"}
	seen := make(map[string]bool)
	total := 0
	for _, entry := range environment {
		key, value, found := strings.Cut(entry, "=")
		if !found || !allowed[key] {
			continue
		}
		limit := 2048
		if key == "HTTP_COOKIE" {
			limit = 16 << 10
		}
		total += len(entry)
		if seen[key] || len(value) > limit || total > 32<<10 || strings.ContainsAny(value, "\r\n\x00") {
			return nil, errors.New("invalid CGI metadata")
		}
		seen[key] = true
		result = append(result, entry)
	}
	return result, nil
}

func Authenticate(ctx context.Context, environment []string) (string, error) {
	command := exec.CommandContext(ctx, HelperPath)
	command.Env = environment
	output, err := Run(command)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", &Failure{Reason: "helper_timeout"}
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", &Failure{Reason: "helper_cancelled"}
		}
		return "", err
	}
	return Username(output)
}

func Run(command *exec.Cmd) (string, error) {
	output, failure := boundedcmd.Run(command, MaximumHelperOutput)
	if failure != nil {
		return "", &Failure{Reason: "helper_" + failure.Reason, ExitCode: failure.ExitCode}
	}
	return output, nil
}

func Username(output string) (string, error) {
	username := strings.TrimSuffix(strings.TrimSuffix(output, "\n"), "\r")
	if username == "" || len(username) > 256 || !utf8.ValidString(username) {
		return "", ErrUnauthenticated
	}
	for _, character := range username {
		if unicode.IsControl(character) || unicode.IsSpace(character) || strings.ContainsRune("/:\x00", character) {
			return "", ErrUnauthenticated
		}
	}
	return username, nil
}
