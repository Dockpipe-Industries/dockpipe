package nativehost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

var dockerAPIVersion = regexp.MustCompile(`^1\.[0-9]{1,3}$`)

// Docker actions deliberately use only the local system daemon. Environment
// variables, CLI configuration, URLs and arbitrary arguments are not inputs.
func localDockerClient() (*http.Client, func()) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", "/var/run/docker.sock")
		},
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: 4096,
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("Docker redirects forbidden")
		},
	}
	return client, transport.CloseIdleConnections
}

func runDocker(ctx context.Context, action, id string) DockerResult {
	client, closeClient := localDockerClient()
	defer closeClient()
	return dockerAction(ctx, client, action, id)
}

func dockerAction(ctx context.Context, client *http.Client, action, id string) DockerResult {
	result := failedDocker(action, id, "docker_action_failed")
	if !validDockerAction(action, id) {
		return result
	}
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/version", nil)
	response, err := client.Do(request)
	if err != nil {
		return dockerFailure(ctx, action, id, "docker_unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	response.Body.Close()
	var version struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err != nil || response.StatusCode != http.StatusOK || len(data) > 16384 || json.Unmarshal(data, &version) != nil || !dockerAPIVersion.MatchString(version.APIVersion) {
		return dockerFailure(ctx, action, id, "docker_unavailable")
	}
	path := "http://docker/v" + version.APIVersion + "/containers/" + id + "/" + action
	if action == "stop" {
		path += "?t=5"
	}
	request, _ = http.NewRequestWithContext(ctx, http.MethodPost, path, nil)
	response, err = client.Do(request)
	if err != nil {
		return dockerFailure(ctx, action, id, "docker_action_failed")
	}
	defer response.Body.Close()
	n, err := io.Copy(io.Discard, io.LimitReader(response.Body, 4097))
	if err != nil || n > 4096 {
		return dockerFailure(ctx, action, id, "docker_action_failed")
	}
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotModified {
		return result
	}
	result.Outcome = "Success"
	result.Reason = ""
	return result
}

func dockerFailure(ctx context.Context, action, id, reason string) DockerResult {
	result := failedDocker(action, id, reason)
	if ctx.Err() != nil {
		result.Outcome = "Cancelled"
		result.Reason = "operation_cancelled"
	}
	return result
}

func (c *Client) Docker(ctx context.Context, action, id string) (DockerResult, error) {
	if !validDockerAction(action, id) {
		return DockerResult{}, errors.New("invalid native Docker action")
	}
	result := failedDocker(action, id, "native_host_unavailable")
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	transport, err := c.connect(ctx, frame{Kind: "docker", Action: action, Container: id})
	if err != nil {
		return result, err
	}
	defer transport.Close()
	stop := context.AfterFunc(ctx, func() { transport.Close() })
	defer stop()
	response, err := transport.receive()
	if err != nil || response.Kind != "docker" || response.DockerResult == nil {
		if ctx.Err() != nil {
			result.Outcome = "Cancelled"
			result.Reason = "operation_cancelled"
		}
		return result, ErrUnavailable
	}
	value := response.DockerResult
	if value.Schema != DockerActionSchema || value.Action != action || value.ID != id || (value.Outcome != "Success" && value.Outcome != "Failure" && value.Outcome != "Cancelled") {
		return result, ErrUnavailable
	}
	switch value.Reason {
	case "", "docker_unavailable", "docker_action_failed", "operation_cancelled":
	default:
		return result, ErrUnavailable
	}
	if value.Outcome != "Success" {
		return *value, errors.New("native Docker action failed")
	}
	return *value, nil
}
