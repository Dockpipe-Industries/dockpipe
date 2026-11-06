package nativehost

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

const maxFrame = (1 << 20) + 4096
const maxTerminalData = 64 << 10

type frame struct {
	Kind         string             `json:"kind"`
	ID           uint64             `json:"id,omitempty"`
	Key          string             `json:"key,omitempty"`
	Nonce        string             `json:"nonce,omitempty"`
	Status       *Status            `json:"status,omitempty"`
	Data         []byte             `json:"data,omitempty"`
	Snapshot     json.RawMessage    `json:"snapshot,omitempty"`
	Error        string             `json:"error,omitempty"`
	System       bool               `json:"system,omitempty"`
	Docker       bool               `json:"docker,omitempty"`
	CWD          string             `json:"cwd,omitempty"`
	Rows         uint16             `json:"rows,omitempty"`
	Columns      uint16             `json:"columns,omitempty"`
	Exit         int                `json:"exit,omitempty"`
	Action       string             `json:"action,omitempty"`
	Container    string             `json:"container,omitempty"`
	ResourceKind string             `json:"resource_kind,omitempty"`
	ResourceID   string             `json:"resource_id,omitempty"`
	DockerQuery  *DockerQueryResult `json:"docker_query,omitempty"`
	DockerResult *DockerResult      `json:"docker_result,omitempty"`
}

type wire struct {
	io.ReadWriteCloser
	writeMu    sync.Mutex
	writeLimit time.Time
}

func (w *wire) send(value frame) error {
	if len(value.Data) > 1<<20 {
		return errors.New("native host payload too large")
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) > maxFrame {
		return errors.New("native host frame too large")
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if connection, ok := w.ReadWriteCloser.(interface{ SetWriteDeadline(time.Time) error }); ok {
		deadline := time.Now().Add(3 * time.Second)
		if !w.writeLimit.IsZero() && w.writeLimit.Before(deadline) {
			deadline = w.writeLimit
		}
		connection.SetWriteDeadline(deadline)
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(data)))
	if err := writeAll(w.ReadWriteCloser, prefix[:]); err != nil {
		return err
	}
	return writeAll(w.ReadWriteCloser, data)
}

func (w *wire) receive() (frame, error) {
	var value frame
	var prefix [4]byte
	if _, err := io.ReadFull(w.ReadWriteCloser, prefix[:]); err != nil {
		return value, err
	}
	size := binary.BigEndian.Uint32(prefix[:])
	if size == 0 || size > maxFrame {
		return value, errors.New("invalid native host frame size")
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(w.ReadWriteCloser, data); err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return value, errors.New("invalid native host frame")
	}
	if len(value.Kind) > 32 || len(value.Key) > 64 || len(value.Nonce) > 64 || len(value.CWD) > 4096 || len(value.Error) > 128 || len(value.Snapshot) > 1<<20 || len(value.Action) > 8 || len(value.Container) > 64 {
		return frame{}, errors.New("native host metadata too large")
	}
	if len(value.ResourceKind) > 16 || len(value.ResourceID) > 255 || (value.DockerQuery != nil && (value.Kind != "docker-query" || !validDockerQueryResult(*value.DockerQuery, value.DockerQuery.Kind, value.DockerQuery.ID))) {
		return frame{}, errors.New("invalid native Docker query metadata")
	}
	if value.DockerResult != nil && (len(value.DockerResult.Schema) > 64 || len(value.DockerResult.Outcome) > 16 || len(value.DockerResult.Action) > 8 || len(value.DockerResult.ID) > 64 || len(value.DockerResult.Reason) > 128) {
		return frame{}, errors.New("native host Docker result too large")
	}
	if len(value.Data) > maxTerminalData || (len(value.Data) != 0 && value.Kind != "input" && value.Kind != "output") || (len(value.Snapshot) != 0 && value.Kind != "snapshot") {
		return frame{}, errors.New("invalid native host payload")
	}
	if value.Status != nil && (len(value.Status.Schema) > 64 || len(value.Status.Node) > 255 || len(value.Status.Username) > 256 || len(value.Status.Home) > 4096 || len(value.Status.Generation) > 256 || len(value.Status.Reason) > 128 || len(value.Status.Groups) > 1024) {
		return frame{}, errors.New("native host identity metadata too large")
	}
	return value, nil
}

func writeAll(output io.Writer, data []byte) error {
	for len(data) != 0 {
		n, err := output.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
