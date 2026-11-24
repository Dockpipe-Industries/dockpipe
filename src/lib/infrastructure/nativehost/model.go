// Package nativehost connects a local client to a reverse-connected worker
// running as an explicitly authorized operating-system user.
package nativehost

import (
	"context"
	"errors"
	"time"
)

const StatusSchema = "dockpipe.native_host_status.v1"
const ConnectionSchema = "dockpipe.native_host_connection.v1"

var ErrUnavailable = errors.New("native host unavailable")

type Grant struct {
	UID        uint32
	Generation string
}

type Options struct {
	WorkerSocket string
	ClientSocket string
	ClientKey    string
	ClientUID    uint32
	Authorize    func() (Grant, error)
}

type Status struct {
	Schema     string   `json:"schema"`
	Available  bool     `json:"available"`
	Connected  bool     `json:"connected"`
	Node       string   `json:"node,omitempty"`
	Username   string   `json:"username,omitempty"`
	UID        uint32   `json:"uid"`
	GID        uint32   `json:"gid"`
	Groups     []uint32 `json:"groups,omitempty"`
	Home       string   `json:"home,omitempty"`
	Generation string   `json:"generation,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

type WorkerOptions struct {
	Socket string
	Nonce  string
	// Ready is called after the broker accepts this worker. It must not block.
	Ready func() error
}

type TerminalOptions struct {
	CWD     string
	Rows    uint16
	Columns uint16
}

type DockerResult struct {
	Schema  string `json:"schema"`
	Outcome string `json:"outcome"`
	Action  string `json:"action"`
	ID      string `json:"id"`
	Reason  string `json:"reason,omitempty"`
}

const DockerActionSchema = "dockpipe.native_host_docker_action.v1"

type workerExpectation struct {
	uid     uint32
	gid     uint32
	nonce   string
	expires time.Time
	grant   Grant
}

func unavailableStatus() Status {
	return Status{Schema: StatusSchema, Reason: "native_host_unavailable"}
}

func boundedContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 8*time.Second)
}
