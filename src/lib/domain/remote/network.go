package remote

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

const NetworkVersion = "dockpipe.remote.network/v1"
const NetworkRelayPath = "/v1/network-relay"
const NetworkProtocol = "dockpipe.network.tcp.v1"

// A broker supports eight browsing contexts with 32 live TCP streams apiece.
// Backpressure keeps each stream to two 64 KiB relay buffers. Streams expire
// after one hour or 120 seconds idle; each direction has a 1 GiB transfer budget.
const MaxNetworkStreams = 256
const MaxNetworkNodeStreams = 32
const MaxNetworkFrame = 64 << 10

// One million frames permits the byte budget at an average 1 KiB payload,
// while independently bounding tiny-frame protocol work.
const MaxNetworkFrames = uint64(1) << 20
const MaxNetworkBytes = uint64(1) << 30
const NetworkLifetime = time.Hour
const NetworkFreshness = 5 * time.Second
const NetworkIdle = 120 * time.Second

type NetworkPermissions struct {
	Schema              string `json:"schema"`
	Enabled             bool   `json:"enabled"`
	ShareTLSTrust       bool   `json:"share_tls_trust,omitempty"`
	LoopbackDestination string `json:"loopback_destination,omitempty"`
}

func (permission NetworkPermissions) Validate() error {
	if permission.Schema != NetworkVersion || !permission.Enabled && (permission.ShareTLSTrust || permission.LoopbackDestination != "") {
		return errors.New("invalid independent network permission schema")
	}
	if permission.LoopbackDestination != "" {
		host, _, err := net.SplitHostPort(permission.LoopbackDestination)
		if err != nil || !net.ParseIP(host).IsLoopback() || ValidateNetworkAddress(permission.LoopbackDestination) != nil {
			return errors.New("network loopback grant requires an exact loopback IP address and port")
		}
	}

	return nil
}

func (permission NetworkPermissions) AllowsDestination(address string) bool {
	return permission.Enabled && (permission.LoopbackDestination == "" || permission.LoopbackDestination == address)
}

type NetworkCatalog struct {
	Schema     string               `json:"schema"`
	Target     FeatureTarget        `json:"target"`
	Enabled    bool                 `json:"enabled"`
	ReceivedAt time.Time            `json:"received_at"`
	TLSTrust   *NetworkTrustSummary `json:"tls_trust,omitempty"`
}

func (catalog NetworkCatalog) Validate() error {
	if catalog.Schema != NetworkVersion || catalog.Target.Validate() != nil || catalog.TLSTrust != nil && catalog.TLSTrust.Validate() != nil {
		return errors.New("invalid network catalog identity")
	}
	return nil
}

type NetworkConnect struct {
	Schema  string        `json:"schema"`
	Target  FeatureTarget `json:"target"`
	Address string        `json:"address"`
}

func (request NetworkConnect) Validate() error {
	if request.Schema != NetworkVersion || request.Target.Validate() != nil || ValidateNetworkAddress(request.Address) != nil {
		return errors.New("invalid exact TCP connection request")
	}
	return nil
}

// Hostnames are deliberately retained until the worker dials. Loopback addresses
// identify the selected worker, never the operator computer or the broker.
func ValidateNetworkAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	number, portErr := strconv.Atoi(port)
	if err != nil || host == "" || len(host) > 253 || strings.ContainsAny(host, "\x00\r\n\t /\\@%") || portErr != nil || number < 1 || number > 65535 || port != strconv.Itoa(number) {
		return errors.New("TCP destination requires a bounded host and canonical port")
	}
	if net.ParseIP(host) == nil {
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return errors.New("invalid destination DNS label")
			}
			for _, character := range label {
				if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-') {
					return errors.New("invalid destination DNS character")
				}
			}
		}
	}
	return nil
}

type NetworkTicket struct {
	ID      string         `json:"id"`
	Secret  string         `json:"secret"`
	Request NetworkConnect `json:"request"`
}
