package orchestrationhelper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const nodeConnectorDirectTLSMaxHandshakeTimeout = time.Minute

var nodeConnectorDirectTLSReference = regexp.MustCompile(`^ref-[a-z0-9][a-z0-9._:-]{6,122}$`)

// NodeConnectorDirectTLSSecretReferenceLoader resolves local-only certificate
// material. Callers must not return resolved values in errors.
type NodeConnectorDirectTLSSecretReferenceLoader func(reference string) ([]byte, error)

// NodeConnectorDirectTLSBrokerConfig contains only public location and opaque
// local secret-reference identifiers. It grants no transport or work authority.
type NodeConnectorDirectTLSBrokerConfig struct {
	ListenEndpoint            string        `json:"listen_endpoint"`
	CertificateChainReference string        `json:"certificate_chain_reference"`
	PrivateKeyReference       string        `json:"private_key_reference"`
	HandshakeTimeout          time.Duration `json:"handshake_timeout"`
}

// NodeConnectorDirectTLSConnectorConfig contains one explicit broker endpoint,
// one expected server identity, and one opaque local trust-root reference.
type NodeConnectorDirectTLSConnectorConfig struct {
	Endpoint               string        `json:"endpoint"`
	ExpectedServerIdentity string        `json:"expected_server_identity"`
	TrustRootsReference    string        `json:"trust_roots_reference"`
	HandshakeTimeout       time.Duration `json:"handshake_timeout"`
}

// NodeConnectorDirectTLSBrokerListener is the only listener-owning role. TLS
// adds confidentiality and server identity only; accepted connections still
// carry the unchanged node-connector transport records.
type NodeConnectorDirectTLSBrokerListener struct {
	listener  net.Listener
	tlsConfig *tls.Config
	limits    NodeConnectorTransportLimits
	timeout   time.Duration
}

// NodeConnectorDirectTLSConnector owns only one outbound direct-TLS dial path.
type NodeConnectorDirectTLSConnector struct {
	endpoint  string
	tlsConfig *tls.Config
	limits    NodeConnectorTransportLimits
	timeout   time.Duration
}

func NewNodeConnectorDirectTLSBrokerListener(config NodeConnectorDirectTLSBrokerConfig, limits NodeConnectorTransportLimits, load NodeConnectorDirectTLSSecretReferenceLoader) (*NodeConnectorDirectTLSBrokerListener, error) {
	if err := validateNodeConnectorTransportLimits(limits); err != nil {
		return nil, err
	}
	if err := validateNodeConnectorDirectTLSBrokerConfig(config); err != nil {
		return nil, err
	}
	if load == nil {
		return nil, errors.New("direct TLS broker requires a local secret-reference loader")
	}
	certificatePEM, err := load(config.CertificateChainReference)
	if err != nil || len(certificatePEM) == 0 {
		return nil, errors.New("direct TLS broker certificate reference could not be resolved")
	}
	privateKeyPEM, err := load(config.PrivateKeyReference)
	if err != nil || len(privateKeyPEM) == 0 {
		return nil, errors.New("direct TLS broker private-key reference could not be resolved")
	}
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return nil, errors.New("direct TLS broker certificate and private-key references are malformed or mismatched")
	}
	network, address, err := validateNodeConnectorDirectTLSEndpoint(config.ListenEndpoint, true)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, errors.New("direct TLS broker listener failed")
	}
	return &NodeConnectorDirectTLSBrokerListener{
		listener: listener,
		tlsConfig: &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS13,
			MaxVersion:   tls.VersionTLS13,
		},
		limits:  limits,
		timeout: config.HandshakeTimeout,
	}, nil
}

func NewNodeConnectorDirectTLSConnector(config NodeConnectorDirectTLSConnectorConfig, limits NodeConnectorTransportLimits, load NodeConnectorDirectTLSSecretReferenceLoader) (*NodeConnectorDirectTLSConnector, error) {
	if err := validateNodeConnectorTransportLimits(limits); err != nil {
		return nil, err
	}
	if err := validateNodeConnectorDirectTLSConnectorConfig(config); err != nil {
		return nil, err
	}
	if load == nil {
		return nil, errors.New("direct TLS connector requires a local secret-reference loader")
	}
	trustPEM, err := load(config.TrustRootsReference)
	if err != nil || len(trustPEM) == 0 {
		return nil, errors.New("direct TLS connector trust-root reference could not be resolved")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(trustPEM) {
		return nil, errors.New("direct TLS connector trust-root reference is malformed")
	}
	return &NodeConnectorDirectTLSConnector{
		endpoint: config.Endpoint,
		tlsConfig: &tls.Config{
			RootCAs:    roots,
			ServerName: config.ExpectedServerIdentity,
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		},
		limits:  limits,
		timeout: config.HandshakeTimeout,
	}, nil
}

func (broker *NodeConnectorDirectTLSBrokerListener) Endpoint() string {
	return broker.listener.Addr().String()
}

func (broker *NodeConnectorDirectTLSBrokerListener) Accept() (net.Conn, error) {
	if tcp, ok := broker.listener.(*net.TCPListener); ok {
		if err := tcp.SetDeadline(time.Now().Add(broker.limits.ConnectTimeout)); err != nil {
			return nil, errors.New("direct TLS broker accept deadline failed")
		}
	}
	raw, err := broker.listener.Accept()
	if err != nil {
		return nil, errors.New("direct TLS broker accept failed")
	}
	connection := tls.Server(raw, broker.tlsConfig.Clone())
	ctx, cancel := context.WithTimeout(context.Background(), broker.timeout)
	defer cancel()
	if err := connection.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, errors.New("direct TLS broker handshake failed")
	}
	if connection.ConnectionState().Version != tls.VersionTLS13 {
		_ = connection.Close()
		return nil, errors.New("direct TLS broker rejected a non-TLS-1.3 connection")
	}
	return connection, nil
}

func (broker *NodeConnectorDirectTLSBrokerListener) Close() error {
	return broker.listener.Close()
}

func (connector *NodeConnectorDirectTLSConnector) Dial(ctx context.Context) (net.Conn, error) {
	network, address, err := validateNodeConnectorDirectTLSEndpoint(connector.endpoint, false)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: connector.limits.ConnectTimeout}
	raw, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, errors.New("direct TLS outbound connector dial failed")
	}
	connection := tls.Client(raw, connector.tlsConfig.Clone())
	handshakeContext, cancel := context.WithTimeout(ctx, connector.timeout)
	defer cancel()
	if err := connection.HandshakeContext(handshakeContext); err != nil {
		_ = raw.Close()
		return nil, errors.New("direct TLS connector handshake or server identity verification failed")
	}
	state := connection.ConnectionState()
	if state.Version != tls.VersionTLS13 || len(state.VerifiedChains) == 0 {
		_ = connection.Close()
		return nil, errors.New("direct TLS connector requires a verified TLS 1.3 server chain")
	}
	return connection, nil
}

func validateNodeConnectorDirectTLSBrokerConfig(config NodeConnectorDirectTLSBrokerConfig) error {
	if _, _, err := validateNodeConnectorDirectTLSEndpoint(config.ListenEndpoint, true); err != nil {
		return err
	}
	if err := validateNodeConnectorDirectTLSReference(config.CertificateChainReference); err != nil {
		return err
	}
	if err := validateNodeConnectorDirectTLSReference(config.PrivateKeyReference); err != nil {
		return err
	}
	if config.CertificateChainReference == config.PrivateKeyReference {
		return errors.New("direct TLS certificate and private-key references must be distinct")
	}
	return validateNodeConnectorDirectTLSHandshakeTimeout(config.HandshakeTimeout)
}

func validateNodeConnectorDirectTLSConnectorConfig(config NodeConnectorDirectTLSConnectorConfig) error {
	if _, _, err := validateNodeConnectorDirectTLSEndpoint(config.Endpoint, false); err != nil {
		return err
	}
	identity := strings.TrimSpace(config.ExpectedServerIdentity)
	if identity == "" || identity != config.ExpectedServerIdentity || len(identity) > 253 || strings.ContainsAny(identity, "/\\\x00\r\n\t") {
		return errors.New("direct TLS expected server identity is invalid")
	}
	if err := validateNodeConnectorDirectTLSReference(config.TrustRootsReference); err != nil {
		return err
	}
	return validateNodeConnectorDirectTLSHandshakeTimeout(config.HandshakeTimeout)
}

func validateNodeConnectorDirectTLSReference(reference string) error {
	if !nodeConnectorDirectTLSReference.MatchString(reference) || strings.Contains(reference, "://") {
		return errors.New("direct TLS local secret reference is invalid")
	}
	return nil
}

func validateNodeConnectorDirectTLSHandshakeTimeout(timeout time.Duration) error {
	if timeout <= 0 || timeout > nodeConnectorDirectTLSMaxHandshakeTimeout {
		return errors.New("direct TLS handshake timeout is invalid or unbounded")
	}
	return nil
}

func validateNodeConnectorDirectTLSEndpoint(endpoint string, listener bool) (string, string, error) {
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" || portText == "" {
		return "", "", errors.New("direct TLS requires one explicit numeric endpoint and port")
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() {
		return "", "", errors.New("direct TLS endpoint rejects hostnames, wildcard, and unspecified addresses")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 || !listener && port == 0 {
		return "", "", errors.New("direct TLS endpoint port is invalid")
	}
	network := "tcp6"
	if ip.To4() != nil {
		network = "tcp4"
	}
	return network, net.JoinHostPort(ip.String(), strconv.Itoa(port)), nil
}
