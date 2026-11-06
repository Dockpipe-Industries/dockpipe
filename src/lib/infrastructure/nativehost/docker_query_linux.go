package nativehost

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

func runDockerQuery(ctx context.Context, kind, id string) DockerQueryResult {
	client, closeClient := localDockerClient()
	defer closeClient()
	return dockerQuery(ctx, client, kind, id)
}

func dockerRead(ctx context.Context, client *http.Client, path string, maximum int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("Docker query failed")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil || int64(len(content)) > maximum {
		return nil, errors.New("Docker response exceeded bounds")
	}
	return content, nil
}

func dockerQueryFailure(ctx context.Context, kind, id, reason string) DockerQueryResult {
	result := failedDockerQuery(kind, id, reason)
	if ctx.Err() != nil {
		result.Outcome = "Cancelled"
		result.Reason = "operation_cancelled"
	}
	return result
}

func dockerQuery(ctx context.Context, client *http.Client, kind, id string) DockerQueryResult {
	if ValidateDockerQuery(kind, id) != nil {
		return failedDockerQuery(kind, id, "docker_query_failed")
	}
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	versionBytes, err := dockerRead(ctx, client, "/version", 16384)
	var version struct {
		APIVersion string `json:"ApiVersion"`
	}
	if err != nil || json.Unmarshal(versionBytes, &version) != nil || !dockerAPIVersion.MatchString(version.APIVersion) {
		return dockerQueryFailure(ctx, kind, id, "docker_unavailable")
	}
	prefix := "/v" + version.APIVersion
	path := prefix + "/" + kind + "s/" + url.PathEscape(id)
	if kind == "container" {
		path += "/json?size=false"
	}
	inspected, err := dockerRead(ctx, client, path, maxDockerInspect)
	if err != nil {
		return dockerQueryFailure(ctx, kind, id, "docker_query_failed")
	}
	var identity struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Config struct {
			TTY bool `json:"Tty"`
		} `json:"Config"`
	}
	if json.Unmarshal(inspected, &identity) != nil || (kind == "volume" && identity.Name != id) || (kind != "volume" && (len(identity.ID) < len(id) || !bytes.EqualFold([]byte(identity.ID[:len(id)]), []byte(id)))) {
		return dockerQueryFailure(ctx, kind, id, "docker_response_invalid")
	}
	result := DockerQueryResult{Schema: DockerQuerySchema, Outcome: "Success", Kind: kind, ID: id, Inspect: inspected}
	if kind == "container" {
		result.Logs, result.LogsTruncated, result.LogsError = dockerContainerLogs(ctx, client, prefix, id, identity.Config.TTY)
	}
	if ctx.Err() != nil {
		return dockerQueryFailure(ctx, kind, id, "operation_cancelled")
	}
	if !validDockerQueryResult(result, kind, id) {
		return dockerQueryFailure(ctx, kind, id, "docker_response_too_large")
	}
	return result
}

func dockerContainerLogs(ctx context.Context, client *http.Client, prefix, id string, tty bool) (string, bool, string) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+prefix+"/containers/"+id+"/logs?stdout=true&stderr=true&tail=200&timestamps=true", nil)
	response, err := client.Do(request)
	if err != nil {
		return "", false, "docker_logs_unavailable"
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", false, "docker_logs_unavailable"
	}
	content, truncated, err := decodeDockerLogs(response.Body, tty)
	if err != nil {
		return "", false, "docker_logs_invalid"
	}
	return string(bytes.ToValidUTF8(content, []byte("?"))), truncated, ""
}

// Docker's non-TTY log response multiplexes stdout/stderr with eight-byte
// headers. TTY containers return a plain byte stream. Both have a hard byte cap.
func decodeDockerLogs(reader io.Reader, tty bool) ([]byte, bool, error) {
	if tty {
		content, err := io.ReadAll(io.LimitReader(reader, maxDockerLogs+1))
		if len(content) > maxDockerLogs {
			return content[:maxDockerLogs], true, err
		}
		return content, false, err
	}
	var output bytes.Buffer
	for frames := 0; frames < 4096; frames++ {
		var header [8]byte
		n, err := io.ReadFull(reader, header[:])
		if err == io.EOF && n == 0 {
			return output.Bytes(), false, nil
		}
		if err != nil || (header[0] != 1 && header[0] != 2) || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return nil, false, errors.New("invalid Docker log frame")
		}
		size := int64(binary.BigEndian.Uint32(header[4:]))
		remaining := int64(maxDockerLogs - output.Len())
		readSize := min(size, remaining)
		if _, err := io.CopyN(&output, reader, readSize); err != nil {
			return nil, false, err
		}
		if size > remaining {
			return output.Bytes(), true, nil
		}
	}
	return nil, false, errors.New("Docker log frame count exceeded")
}

func (c *Client) DockerQuery(ctx context.Context, kind, id string) (DockerQueryResult, error) {
	if err := ValidateDockerQuery(kind, id); err != nil {
		return DockerQueryResult{}, err
	}
	ctx, cancel := boundedContext(ctx)
	defer cancel()
	transport, err := c.connect(ctx, frame{Kind: "docker-query", ResourceKind: kind, ResourceID: id})
	if err != nil {
		return dockerQueryFailure(ctx, kind, id, "native_host_unavailable"), err
	}
	defer transport.Close()
	stop := context.AfterFunc(ctx, func() { transport.Close() })
	defer stop()
	response, err := transport.receive()
	if err != nil || response.Kind != "docker-query" || response.DockerQuery == nil || !validDockerQueryResult(*response.DockerQuery, kind, id) {
		return dockerQueryFailure(ctx, kind, id, "native_host_unavailable"), ErrUnavailable
	}
	if response.DockerQuery.Outcome != "Success" {
		return *response.DockerQuery, errors.New("native Docker query failed")
	}
	return *response.DockerQuery, nil
}
