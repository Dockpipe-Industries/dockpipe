// Package remote defines a single-node workflow delivery contract. It does not
// select nodes, schedule graphs, provision environments, or define edge vendors.
package remote

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"time"
)

const Version = "dockpipe.remote/v1"

// Allow base64 artifacts and worst-case JSON escaping of the bounded log.
const MaxBody = 20 << 20
const MaxLog = 1 << 20
const MaxArtifacts = 8 << 20

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func ValidID(value string) bool { return identifier.MatchString(value) }

// Endpoint requires authenticated HTTPS outside explicit numeric loopback.
func Endpoint(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("remote endpoint must be an HTTPS origin without credentials, query, or path")
	}
	if u.Scheme == "https" {
		return nil
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme == "http" && ip != nil && ip.IsLoopback() {
		return nil
	}
	return errors.New("remote endpoint requires HTTPS; HTTP is allowed only on numeric loopback")
}

type Submission struct {
	ID      string `json:"id"`
	Node    string `json:"node"`
	Profile string `json:"profile"`
}

func (s Submission) Validate() error {
	if !ValidID(s.ID) || !ValidID(s.Node) || !ValidID(s.Profile) {
		return errors.New("job, node, and profile must be bounded identifiers")
	}
	return nil
}

// Profile is local authority on the worker. Requests cannot supply paths, argv,
// environment variables, workflow definitions, or executable locations.
type Profile struct {
	Workdir        string   `json:"workdir"`
	Workflow       string   `json:"workflow,omitempty"`
	WorkflowFile   string   `json:"workflow_file,omitempty"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Artifacts      []string `json:"artifacts,omitempty"`
}

type Invitation struct {
	Schema    string    `json:"schema"`
	Endpoint  string    `json:"endpoint"`
	Node      string    `json:"node"`
	Secret    string    `json:"secret"`
	ExpiresAt time.Time `json:"expires_at"`
}

type WorkerConfig struct {
	Schema   string             `json:"schema"`
	Endpoint string             `json:"endpoint"`
	Node     string             `json:"node"`
	Token    string             `json:"token"`
	Profiles map[string]Profile `json:"profiles"`
}

// EdgeConfig is written by a resolver after its provider-specific setup. Core
// only supervises the declared process and verifies the published endpoint.
// Credentials must be file references in argv, never inline values.
type EdgeConfig struct {
	Schema     string   `json:"schema"`
	Endpoint   string   `json:"endpoint"`
	Executable string   `json:"executable"`
	Arguments  []string `json:"arguments"`
}
