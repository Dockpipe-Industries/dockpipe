package infrastructure

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"dockpipe/src/lib/infrastructure/process"
)

const dockerEngineMetadataLimit = 512 * 1024

// Docker's fixed dial-stdio bridge retains its owner-selected context, TLS and
// SSH handling. No transport request can select an executable, endpoint or argv.
// Every API connection confirms the daemon identity on that same connection
// before sending a resource operation; changing context files cannot retarget it.
type dockerEngineClient struct {
	name        string
	environment []string
	daemon      string
	dial        func(context.Context) (net.Conn, error)
}

type dockerEnginePipe struct {
	input  *os.File
	output *os.File
	cancel context.CancelFunc
	done   chan error
	once   sync.Once
	err    error
}

func (connection *dockerEnginePipe) Read(data []byte) (int, error) {
	return connection.output.Read(data)
}

func (connection *dockerEnginePipe) Write(data []byte) (int, error) {
	return connection.input.Write(data)
}

func (connection *dockerEnginePipe) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "docker-native", Net: "pipe"}
}

func (connection *dockerEnginePipe) RemoteAddr() net.Addr {
	return connection.LocalAddr()
}

func (connection *dockerEnginePipe) SetReadDeadline(deadline time.Time) error {
	return connection.output.SetReadDeadline(deadline)
}

func (connection *dockerEnginePipe) SetWriteDeadline(deadline time.Time) error {
	return connection.input.SetWriteDeadline(deadline)
}

func (connection *dockerEnginePipe) SetDeadline(deadline time.Time) error {
	return errors.Join(connection.SetReadDeadline(deadline), connection.SetWriteDeadline(deadline))
}

func (connection *dockerEnginePipe) Close() error {
	connection.once.Do(func() {
		connection.cancel()
		_ = connection.input.Close()
		_ = connection.output.Close()
		select {
		case <-connection.done:
		case <-time.After(time.Second):
			connection.err = errors.New("could not verify Docker native relay cleanup")
		}
	})
	return connection.err
}

func (client *dockerEngineClient) nativeDial(ctx context.Context) (net.Conn, error) {
	lifetime, cancel := context.WithCancel(ctx)
	command := exec.CommandContext(lifetime, client.name, "system", "dial-stdio")
	command.Env = dockerCommandEnv(client.environment, client.name)
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	input, err := command.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		_ = input.Close()
		cancel()
		return nil, err
	}
	writer, writerOK := input.(*os.File)
	reader, readerOK := output.(*os.File)
	if !writerOK || !readerOK {
		_ = input.Close()
		_ = output.Close()
		cancel()
		return nil, errors.New("pollable pipes are required for the Docker native relay")
	}
	closeScope, err := process.AttachScope(ctx, command)
	if err != nil {
		_ = input.Close()
		_ = output.Close()
		cancel()
		return nil, err
	}
	connection := &dockerEnginePipe{input: writer, output: reader, cancel: cancel, done: make(chan error, 1)}
	started := make(chan error, 1)
	go func() {
		defer closeScope()
		observed := false
		err := process.RunObserved(command, func(command *exec.Cmd) func(error) {
			if command.Process != nil && !observed {
				observed = true
				started <- nil
			}
			return nil
		})
		if !observed {
			started <- err
		}
		connection.done <- err
	}()
	select {
	case err := <-started:
		if err != nil {
			_ = connection.Close()
			return nil, err
		}
		return connection, nil
	case <-ctx.Done():
		_ = connection.Close()
		return nil, ctx.Err()
	}
}

type dockerEngineChannel struct {
	net.Conn
	reader  *bufio.Reader
	version string
}

type dockerEngineHeaderReader struct {
	source   io.Reader
	count    int
	tail     uint32
	complete bool
}

func (reader *dockerEngineHeaderReader) Read(data []byte) (int, error) {
	count, err := reader.source.Read(data)
	if reader.complete {
		return count, err
	}
	for _, value := range data[:count] {
		reader.count++
		if reader.count > 32*1024 {
			return 0, errors.New("response headers from the Docker API exceed 32 KiB")
		}
		reader.tail = reader.tail<<8 | uint32(value)
		if reader.tail == 0x0d0a0d0a {
			reader.complete = true
			break
		}
	}
	return count, err
}

func (channel *dockerEngineChannel) roundTrip(method, path string, body []byte, upgrade bool) (*http.Response, error) {
	if path != "/version" {
		path = channel.version + path
	}
	request, err := http.NewRequest(method, "http://docker"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if upgrade {
		request.Header.Set("Connection", "Upgrade")
		request.Header.Set("Upgrade", "tcp")
	}
	if err := request.Write(channel.Conn); err != nil {
		return nil, err
	}
	channel.reader = bufio.NewReaderSize(&dockerEngineHeaderReader{source: channel.reader}, 32*1024)
	return http.ReadResponse(channel.reader, request)
}

func dockerEngineBody(response *http.Response) ([]byte, error) {
	defer response.Body.Close()
	if response.ContentLength > dockerEngineMetadataLimit {
		return nil, errors.New("metadata from the Docker API exceeds 512 KiB")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, dockerEngineMetadataLimit+1))
	if err != nil || len(data) > dockerEngineMetadataLimit {
		return nil, errors.New("metadata from the Docker API is unavailable or exceeds 512 KiB")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("operation failed in the Docker API")
	}
	return data, nil
}

func (client *dockerEngineClient) connect(ctx context.Context) (*dockerEngineChannel, error) {
	dial := client.dial
	if dial == nil {
		dial = client.nativeDial
	}
	connection, err := dial(ctx)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(3 * time.Second)
	if until, ok := ctx.Deadline(); ok && until.Before(deadline) {
		deadline = until
	}
	if err := connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}
	channel := &dockerEngineChannel{Conn: connection, reader: bufio.NewReaderSize(connection, 32*1024)}
	versionResponse, err := channel.roundTrip("GET", "/version", nil, false)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	versionData, err := dockerEngineBody(versionResponse)
	var version struct {
		API     string `json:"ApiVersion"`
		Minimum string `json:"MinAPIVersion"`
	}
	if err != nil || json.Unmarshal(versionData, &version) != nil {
		_ = connection.Close()
		return nil, errors.New("no version available from the Docker API")
	}
	selected, err := dockerEngineVersion(version.API, version.Minimum)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	channel.version = selected
	response, err := channel.roundTrip("GET", "/info", nil, false)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	data, err := dockerEngineBody(response)
	var info struct {
		ID string `json:"ID"`
	}
	if err != nil || json.Unmarshal(data, &info) != nil || info.ID == "" || len(info.ID) > 256 {
		_ = connection.Close()
		return nil, errors.New("identity of the Docker daemon is unavailable")
	}
	digest := sha256.Sum256([]byte(info.ID))
	identity := hex.EncodeToString(digest[:])
	if client.daemon != "" && client.daemon != identity {
		_ = connection.Close()
		return nil, errors.New("identity of the Docker daemon changed")
	}
	if client.daemon == "" {
		client.daemon = identity
	}
	return channel, nil
}

func (client *dockerEngineClient) query(ctx context.Context, method, path string, input any, result any) error {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	connection, err := client.connect(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	response, err := connection.roundTrip(method, path, body, false)
	if err != nil {
		return err
	}
	data, err := dockerEngineBody(response)
	if err != nil {
		return err
	}
	if result != nil && json.Unmarshal(data, result) != nil {
		return errors.New("invalid Docker API response")
	}
	return nil
}

// The exec/attach/resize contracts used here are supported from API 1.41.
// Negotiate against the daemon, capped at the currently qualified API 1.56.
func dockerEngineVersion(server, minimum string) (string, error) {
	minor := func(value string) (int, error) {
		if !strings.HasPrefix(value, "1.") || len(value) < 4 || len(value) > 5 || strings.Trim(strings.TrimPrefix(value, "1."), "0123456789") != "" {
			return 0, errors.New("unsupported Docker API version")
		}
		return strconv.Atoi(strings.TrimPrefix(value, "1."))
	}
	available, err := minor(server)
	if err != nil {
		return "", err
	}
	required := 41
	if minimum != "" {
		required, err = minor(minimum)
		if err != nil {
			return "", err
		}
	}
	selected := min(available, 56)
	if selected < 41 || required > selected {
		return "", errors.New("no supported exec API version is available from the Docker daemon")
	}
	return "/v1." + strconv.Itoa(selected), nil
}
