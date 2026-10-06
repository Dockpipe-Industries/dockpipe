// Package remote defines a single-node workflow delivery contract. It does not
// select nodes, schedule graphs, provision environments, or define edge vendors.
package remote

import (
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
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
	ID         string `json:"id"`
	Node       string `json:"node"`
	Profile    string `json:"profile,omitempty"`
	BundleHash string `json:"bundle_hash,omitempty"`
}

func (s Submission) Validate() error {
	if !ValidID(s.ID) || !ValidID(s.Node) {
		return errors.New("job and node must be bounded identifiers")
	}
	if s.BundleHash != "" {
		decoded, err := hex.DecodeString(s.BundleHash)
		if err != nil || len(decoded) != 32 || strings.ToLower(s.BundleHash) != s.BundleHash || s.Profile != "" {
			return errors.New("delivery must select a SHA256 bundle instead of a profile")
		}
	} else if !ValidID(s.Profile) {
		return errors.New("select a worker profile or a workflow delivery")
	}
	return nil
}

// Profile selects a preinstalled workflow using worker-local authority.
// Bundle submissions instead require a separate DeliveryPermission.
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
	Schema   string              `json:"schema"`
	Endpoint string              `json:"endpoint"`
	Node     string              `json:"node"`
	Token    string              `json:"token"`
	Profiles map[string]Profile  `json:"profiles"`
	Delivery *DeliveryPermission `json:"delivery,omitempty"`
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
