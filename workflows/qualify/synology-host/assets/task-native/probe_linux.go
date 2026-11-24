//go:build linux

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"dockpipe/workflows/internal/synology/boundedcmd"
	"dockpipe/workflows/internal/synology/hostidentity"
)

const maximumReportBytes = 64 << 10

// The HTTP envelope adds one fixed script/challenge, a 256-byte username and
// kernel peer metadata. Reserve four KiB beyond the bounded callback frame.
const maximumResponseBytes = maximumReportBytes + (4 << 10)
const maximumContainers = 128
const probeTimeout = 15 * time.Second
const dockerPath = "/var/packages/ContainerManager/target/usr/bin/docker"

type callback struct {
	Nonce  string      `json:"nonce"`
	Report probeReport `json:"report"`
}

func dockerCheck(ctx context.Context) commandCheck {
	commandContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(commandContext, dockerPath, "--host", "unix:///var/run/docker.sock", "ps", "--all", "--no-trunc", "--format", `{"ID":{{json .ID}},"Names":{{json .Names}},"State":{{json .State}}}`)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "HOME=/nonexistent"}
	output, failure := boundedcmd.Run(command, 64<<10)
	if failure != nil {
		return commandCheck{Outcome: "docker_" + failure.Reason}
	}
	containers, err := parseContainers(output)
	if err != nil {
		return commandCheck{Outcome: "invalid_or_oversized_inventory"}
	}
	return commandCheck{Available: true, Outcome: "ok", Containers: containers}
}

func parseContainers(output string) ([]containerInfo, error) {
	result := make([]containerInfo, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		if len(result) >= maximumContainers {
			return nil, errors.New("inventory exceeds qualification budget")
		}
		var record struct{ ID, Names, State string }
		if json.Unmarshal(scanner.Bytes(), &record) != nil || len(record.ID) != 64 || len(record.Names) == 0 || len(record.Names) > 255 || len(record.State) > 32 {
			return nil, errors.New("invalid inventory record")
		}
		for _, character := range record.ID {
			if !strings.ContainsRune("0123456789abcdef", character) {
				return nil, errors.New("invalid container identifier")
			}
		}
		result = append(result, containerInfo{ID: record.ID, Name: record.Names, State: record.State})
	}
	if scanner.Err() != nil {
		return nil, errors.New("inventory record exceeds budget")
	}
	return result, nil
}

func httpsCheck(ctx context.Context) commandCheck {
	requestContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodHead, "https://github.com/", nil)
	if err != nil {
		return commandCheck{Outcome: "request_unavailable"}
	}
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, TLSHandshakeTimeout: 3 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return commandCheck{Outcome: "https_unavailable"}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return commandCheck{Outcome: "unexpected_http_status"}
	}
	return commandCheck{Available: true, Outcome: "https_200"}
}

func runProbe(socket, nonce string) error {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	metadata, err := hostidentity.Read(schema)
	if err != nil {
		return errors.New("host_identity_unavailable")
	}
	report := callback{Nonce: nonce, Report: probeReport{Host: metadata}}
	report.Report.Docker = dockerCheck(ctx)
	report.Report.HTTPS = httpsCheck(ctx)
	data, err := json.Marshal(report)
	if err != nil || len(data)+1 > maximumReportBytes {
		return errors.New("probe_report_exceeds_budget")
	}
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return errors.New("probe_callback_unavailable")
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	if connection.SetDeadline(deadline) != nil {
		return errors.New("probe_callback_deadline_unavailable")
	}
	if _, err := connection.Write(append(data, '\n')); err != nil {
		return errors.New("probe_callback_failed")
	}
	var acknowledgement struct {
		Accepted bool `json:"accepted"`
	}
	if json.NewDecoder(io.LimitReader(connection, 128)).Decode(&acknowledgement) != nil || !acknowledgement.Accepted {
		return errors.New("probe_callback_rejected")
	}
	return nil
}
