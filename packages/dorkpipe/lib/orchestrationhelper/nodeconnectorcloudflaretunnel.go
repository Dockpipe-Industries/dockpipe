package orchestrationhelper

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	NodeConnectorCloudflareTunnelMode = "locally_managed_credentials_file_access_tcp"

	nodeConnectorCloudflareMaxTimeout = time.Minute
	nodeConnectorCloudflareTempPrefix = "dorkpipe-cloudflare-tunnel-"
)

var (
	nodeConnectorCloudflareTunnelID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	nodeConnectorCloudflareVersion  = regexp.MustCompile(`^20[0-9]{2}\.(?:[1-9]|1[0-2])\.[0-9]+$`)
	nodeConnectorCloudflareHostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

// NodeConnectorCloudflareTunnelCapabilities is resolver-owned compatibility
// evidence. Cloudflare documents no minimum version for this selected command
// set, so every required capability must be declared explicitly.
type NodeConnectorCloudflareTunnelCapabilities struct {
	Version                      string
	LocallyManagedCredentialFile bool
	TCPIngress                   bool
	AccessTCP                    bool
	PIDFileReadiness             bool
}

// NodeConnectorCloudflareTunnelExecutable is resolved immediately before a
// process launch. Path is never inferred from PATH or shell expansion.
type NodeConnectorCloudflareTunnelExecutable struct {
	Path         string
	Capabilities NodeConnectorCloudflareTunnelCapabilities
}

type NodeConnectorCloudflareTunnelExecutableResolver func(reference string) (NodeConnectorCloudflareTunnelExecutable, error)
type NodeConnectorCloudflareTunnelCredentialFileResolver func(reference string) (string, error)

// NodeConnectorCloudflareTunnelProcess is the narrow injected process seam
// used by the offline proof. Implementations cannot grant transport or work
// authority.
type NodeConnectorCloudflareTunnelProcess interface {
	PID() int
	Done() <-chan error
	Stop(context.Context) error
}

type NodeConnectorCloudflareTunnelProcessStarter func(executable string, arguments []string) (NodeConnectorCloudflareTunnelProcess, error)

type NodeConnectorCloudflareTunnelOriginConfig struct {
	ExecutableReference string        `json:"cloudflared_executable_reference"`
	CredentialReference string        `json:"credential_file_reference"`
	TunnelID            string        `json:"tunnel_id"`
	PublicHostname      string        `json:"public_hostname"`
	BrokerEndpoint      string        `json:"broker_endpoint"`
	StartupTimeout      time.Duration `json:"startup_timeout"`
	ShutdownTimeout     time.Duration `json:"shutdown_timeout"`
	TemporaryRoot       string        `json:"-"`
}

type NodeConnectorCloudflareTunnelClientConfig struct {
	ExecutableReference string        `json:"cloudflared_executable_reference"`
	PublicHostname      string        `json:"public_hostname"`
	ProxyEndpoint       string        `json:"proxy_endpoint"`
	StartupTimeout      time.Duration `json:"startup_timeout"`
	ShutdownTimeout     time.Duration `json:"shutdown_timeout"`
}

// NodeConnectorCloudflareTunnelOrigin owns only the outbound origin-side
// cloudflared process and its private temporary configuration.
type NodeConnectorCloudflareTunnelOrigin struct {
	process         NodeConnectorCloudflareTunnelProcess
	done            chan error
	temporaryRoot   string
	temporaryDir    string
	shutdownTimeout time.Duration
	closeOnce       sync.Once
	closeErr        error
}

// NodeConnectorCloudflareTunnelClient owns only the adapter-local loopback
// cloudflared proxy and the existing direct-TLS connection carried through it.
type NodeConnectorCloudflareTunnelClient struct {
	process         NodeConnectorCloudflareTunnelProcess
	done            chan error
	connection      net.Conn
	shutdownTimeout time.Duration
	closeOnce       sync.Once
	closeErr        error
}

func StartNodeConnectorCloudflareTunnelOrigin(ctx context.Context, config NodeConnectorCloudflareTunnelOriginConfig, resolveExecutable NodeConnectorCloudflareTunnelExecutableResolver, resolveCredential NodeConnectorCloudflareTunnelCredentialFileResolver, start NodeConnectorCloudflareTunnelProcessStarter) (*NodeConnectorCloudflareTunnelOrigin, error) {
	if err := validateNodeConnectorCloudflareTunnelOriginConfig(config); err != nil {
		return nil, err
	}
	if resolveExecutable == nil || resolveCredential == nil {
		return nil, errors.New("Cloudflare Tunnel origin requires explicit local resolvers")
	}
	if start == nil {
		start = startNodeConnectorCloudflareTunnelProcess
	}
	temporaryRoot, temporaryDir, err := makeNodeConnectorCloudflareTunnelTempDir(config.TemporaryRoot)
	if err != nil {
		return nil, errors.New("Cloudflare Tunnel origin temporary state could not be prepared")
	}
	cleanup := func() error {
		if err := removeNodeConnectorCloudflareTunnelTempDir(temporaryRoot, temporaryDir); err != nil {
			return errors.New("Cloudflare Tunnel origin cleanup failed")
		}
		return nil
	}

	credentialPath, err := resolveCredential(config.CredentialReference)
	if err != nil || validateNodeConnectorCloudflareTunnelLocalFile(credentialPath, false) != nil {
		_ = cleanup()
		return nil, errors.New("Cloudflare Tunnel credential reference could not be resolved")
	}
	executable, err := resolveExecutable(config.ExecutableReference)
	if err != nil || validateNodeConnectorCloudflareTunnelExecutable(executable) != nil {
		_ = cleanup()
		return nil, errors.New("Cloudflare Tunnel executable reference is unavailable or incompatible")
	}
	configPath := filepath.Join(temporaryDir, "config.yml")
	pidPath := filepath.Join(temporaryDir, "origin.pid")
	rawConfig, err := nodeConnectorCloudflareTunnelOriginConfiguration(config, credentialPath)
	if err != nil || os.WriteFile(configPath, rawConfig, 0o600) != nil {
		_ = cleanup()
		return nil, errors.New("Cloudflare Tunnel origin temporary configuration failed")
	}
	arguments := nodeConnectorCloudflareTunnelOriginArguments(configPath, pidPath, config.TunnelID)
	process, err := start(executable.Path, arguments)
	if err != nil || process == nil || process.PID() < 1 || process.Done() == nil {
		_ = cleanup()
		return nil, errors.New("Cloudflare Tunnel origin process failed to start")
	}
	origin := &NodeConnectorCloudflareTunnelOrigin{
		process: process, done: nodeConnectorCloudflareTunnelRedactedDone(process.Done(), "Cloudflare Tunnel origin process exited"),
		temporaryRoot: temporaryRoot, temporaryDir: temporaryDir, shutdownTimeout: config.ShutdownTimeout,
	}
	if err := waitForNodeConnectorCloudflareTunnelPID(ctx, process, pidPath, config.StartupTimeout); err != nil {
		_ = origin.Close()
		return nil, err
	}
	return origin, nil
}

// StartNodeConnectorCloudflareTunnelClient launches the documented local
// access TCP proxy and dials it only through the existing direct-TLS connector.
func StartNodeConnectorCloudflareTunnelClient(ctx context.Context, config NodeConnectorCloudflareTunnelClientConfig, directTLS NodeConnectorDirectTLSConnectorConfig, limits NodeConnectorTransportLimits, loadTLS NodeConnectorDirectTLSSecretReferenceLoader, resolveExecutable NodeConnectorCloudflareTunnelExecutableResolver, start NodeConnectorCloudflareTunnelProcessStarter) (*NodeConnectorCloudflareTunnelClient, net.Conn, error) {
	if err := validateNodeConnectorCloudflareTunnelClientConfig(config); err != nil {
		return nil, nil, err
	}
	if directTLS.Endpoint != config.ProxyEndpoint {
		return nil, nil, errors.New("Cloudflare Tunnel proxy and direct TLS endpoints must match exactly")
	}
	if resolveExecutable == nil {
		return nil, nil, errors.New("Cloudflare Tunnel client requires an explicit executable resolver")
	}
	if start == nil {
		start = startNodeConnectorCloudflareTunnelProcess
	}
	executable, err := resolveExecutable(config.ExecutableReference)
	if err != nil || validateNodeConnectorCloudflareTunnelExecutable(executable) != nil {
		return nil, nil, errors.New("Cloudflare Tunnel executable reference is unavailable or incompatible")
	}
	process, err := start(executable.Path, nodeConnectorCloudflareTunnelClientArguments(config.PublicHostname, config.ProxyEndpoint))
	if err != nil || process == nil || process.PID() < 1 || process.Done() == nil {
		return nil, nil, errors.New("Cloudflare Tunnel client process failed to start")
	}
	client := &NodeConnectorCloudflareTunnelClient{
		process: process, done: nodeConnectorCloudflareTunnelRedactedDone(process.Done(), "Cloudflare Tunnel client process exited"), shutdownTimeout: config.ShutdownTimeout,
	}
	connector, err := NewNodeConnectorDirectTLSConnector(directTLS, limits, loadTLS)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	connection, err := dialNodeConnectorCloudflareTunnelClient(ctx, process, connector, config.StartupTimeout)
	if err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	client.connection = connection
	return client, connection, nil
}

func (origin *NodeConnectorCloudflareTunnelOrigin) Done() <-chan error {
	return origin.done
}

func (origin *NodeConnectorCloudflareTunnelOrigin) Close() error {
	origin.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), origin.shutdownTimeout)
		defer cancel()
		if err := origin.process.Stop(ctx); err != nil {
			origin.closeErr = errors.New("Cloudflare Tunnel origin shutdown failed")
		}
		if err := removeNodeConnectorCloudflareTunnelTempDir(origin.temporaryRoot, origin.temporaryDir); err != nil && origin.closeErr == nil {
			origin.closeErr = errors.New("Cloudflare Tunnel origin cleanup failed")
		}
	})
	return origin.closeErr
}

func (client *NodeConnectorCloudflareTunnelClient) Done() <-chan error {
	return client.done
}

func (client *NodeConnectorCloudflareTunnelClient) Close() error {
	client.closeOnce.Do(func() {
		if client.connection != nil {
			_ = client.connection.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), client.shutdownTimeout)
		defer cancel()
		if err := client.process.Stop(ctx); err != nil {
			client.closeErr = errors.New("Cloudflare Tunnel client shutdown failed")
		}
	})
	return client.closeErr
}

func validateNodeConnectorCloudflareTunnelOriginConfig(config NodeConnectorCloudflareTunnelOriginConfig) error {
	if err := validateNodeConnectorCloudflareTunnelReference(config.ExecutableReference); err != nil {
		return err
	}
	if err := validateNodeConnectorCloudflareTunnelReference(config.CredentialReference); err != nil {
		return err
	}
	if config.ExecutableReference == config.CredentialReference {
		return errors.New("Cloudflare Tunnel executable and credential references must be distinct")
	}
	if !nodeConnectorCloudflareTunnelID.MatchString(config.TunnelID) {
		return errors.New("Cloudflare Tunnel identity is invalid")
	}
	if err := validateNodeConnectorCloudflareTunnelHostname(config.PublicHostname); err != nil {
		return err
	}
	if _, _, err := validateNodeConnectorTransportEndpoint(config.BrokerEndpoint, false); err != nil {
		return errors.New("Cloudflare Tunnel origin requires one explicit numeric loopback broker endpoint")
	}
	return validateNodeConnectorCloudflareTunnelTimeouts(config.StartupTimeout, config.ShutdownTimeout)
}

func validateNodeConnectorCloudflareTunnelClientConfig(config NodeConnectorCloudflareTunnelClientConfig) error {
	if err := validateNodeConnectorCloudflareTunnelReference(config.ExecutableReference); err != nil {
		return err
	}
	if err := validateNodeConnectorCloudflareTunnelHostname(config.PublicHostname); err != nil {
		return err
	}
	if _, _, err := validateNodeConnectorTransportEndpoint(config.ProxyEndpoint, false); err != nil {
		return errors.New("Cloudflare Tunnel client proxy requires one explicit numeric loopback endpoint")
	}
	return validateNodeConnectorCloudflareTunnelTimeouts(config.StartupTimeout, config.ShutdownTimeout)
}

func validateNodeConnectorCloudflareTunnelReference(reference string) error {
	if !nodeConnectorDirectTLSReference.MatchString(reference) || strings.Contains(reference, "://") {
		return errors.New("Cloudflare Tunnel local reference is invalid")
	}
	return nil
}

func validateNodeConnectorCloudflareTunnelHostname(hostname string) error {
	if len(hostname) > 253 || hostname != strings.ToLower(strings.TrimSpace(hostname)) || net.ParseIP(hostname) != nil || !nodeConnectorCloudflareHostname.MatchString(hostname) {
		return errors.New("Cloudflare Tunnel public hostname is invalid")
	}
	return nil
}

func validateNodeConnectorCloudflareTunnelTimeouts(startup, shutdown time.Duration) error {
	if startup <= 0 || startup > nodeConnectorCloudflareMaxTimeout || shutdown <= 0 || shutdown > nodeConnectorCloudflareMaxTimeout {
		return errors.New("Cloudflare Tunnel startup or shutdown timeout is invalid or unbounded")
	}
	return nil
}

func validateNodeConnectorCloudflareTunnelExecutable(executable NodeConnectorCloudflareTunnelExecutable) error {
	capabilities := executable.Capabilities
	if !nodeConnectorCloudflareVersion.MatchString(capabilities.Version) || !capabilities.LocallyManagedCredentialFile || !capabilities.TCPIngress || !capabilities.AccessTCP || !capabilities.PIDFileReadiness {
		return errors.New("Cloudflare Tunnel declared version or capability set is unsupported")
	}
	return validateNodeConnectorCloudflareTunnelLocalFile(executable.Path, true)
}

func validateNodeConnectorCloudflareTunnelLocalFile(path string, executable bool) error {
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("Cloudflare Tunnel local file path must be explicit and absolute")
	}
	clean := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(clean)
	if err != nil || filepath.Clean(resolved) != clean {
		return errors.New("Cloudflare Tunnel local file path is missing or linked")
	}
	info, err := os.Lstat(clean)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("Cloudflare Tunnel local file path is not a regular file")
	}
	if executable && info.Mode().Perm()&0o111 == 0 && filepath.Ext(clean) == "" {
		return errors.New("Cloudflare Tunnel executable path is not executable")
	}
	return nil
}

func nodeConnectorCloudflareTunnelOriginConfiguration(config NodeConnectorCloudflareTunnelOriginConfig, credentialPath string) ([]byte, error) {
	values := []string{config.TunnelID, filepath.ToSlash(credentialPath), config.PublicHostname, "tcp://" + config.BrokerEndpoint}
	quoted := make([]string, len(values))
	for index, value := range values {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		quoted[index] = string(raw)
	}
	content := "tunnel: " + quoted[0] + "\n" +
		"credentials-file: " + quoted[1] + "\n" +
		"ingress:\n" +
		"  - hostname: " + quoted[2] + "\n" +
		"    service: " + quoted[3] + "\n" +
		"  - service: http_status:404\n"
	return []byte(content), nil
}

func nodeConnectorCloudflareTunnelOriginArguments(configPath, pidPath, tunnelID string) []string {
	return []string{"tunnel", "--config", configPath, "--no-autoupdate", "--pidfile", pidPath, "run", tunnelID}
}

func nodeConnectorCloudflareTunnelClientArguments(hostname, proxyEndpoint string) []string {
	return []string{"access", "tcp", "--hostname", hostname, "--url", proxyEndpoint}
}

func waitForNodeConnectorCloudflareTunnelPID(ctx context.Context, process NodeConnectorCloudflareTunnelProcess, path string, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return errors.New("Cloudflare Tunnel origin readiness was cancelled")
		case <-deadline.C:
			return errors.New("Cloudflare Tunnel origin readiness timed out")
		case <-process.Done():
			return errors.New("Cloudflare Tunnel origin exited before readiness")
		case <-ticker.C:
			raw, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil || len(raw) == 0 || len(raw) > 32 || strings.TrimSpace(string(raw)) != strconv.Itoa(process.PID()) {
				return errors.New("Cloudflare Tunnel origin readiness evidence is invalid")
			}
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return errors.New("Cloudflare Tunnel origin readiness evidence is invalid")
			}
			return nil
		}
	}
}

func dialNodeConnectorCloudflareTunnelClient(ctx context.Context, process NodeConnectorCloudflareTunnelProcess, connector *NodeConnectorDirectTLSConnector, timeout time.Duration) (net.Conn, error) {
	startupContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		connection, err := connector.Dial(startupContext)
		if err == nil {
			return connection, nil
		}
		select {
		case <-startupContext.Done():
			if ctx.Err() != nil {
				return nil, errors.New("Cloudflare Tunnel client connection was cancelled")
			}
			return nil, errors.New("Cloudflare Tunnel client connection timed out")
		case <-process.Done():
			return nil, errors.New("Cloudflare Tunnel client exited before connection")
		case <-ticker.C:
		}
	}
}

func nodeConnectorCloudflareTunnelRedactedDone(source <-chan error, message string) chan error {
	result := make(chan error, 1)
	go func() {
		<-source
		result <- errors.New(message)
		close(result)
	}()
	return result
}

func makeNodeConnectorCloudflareTunnelTempDir(root string) (string, string, error) {
	if root == "" {
		root = os.TempDir()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", "", errors.New("temporary root is invalid")
	}
	directory, err := os.MkdirTemp(resolved, nodeConnectorCloudflareTempPrefix)
	if err != nil {
		return "", "", err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		_ = os.Remove(directory)
		return "", "", err
	}
	return resolved, directory, nil
}

func removeNodeConnectorCloudflareTunnelTempDir(root, directory string) error {
	root = filepath.Clean(root)
	directory = filepath.Clean(directory)
	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.Dir(relative) != "." || !strings.HasPrefix(filepath.Base(directory), nodeConnectorCloudflareTempPrefix) {
		return errors.New("Cloudflare Tunnel temporary directory escaped its owned root")
	}
	return os.RemoveAll(directory)
}

type nodeConnectorCloudflareTunnelOSProcess struct {
	command  *exec.Cmd
	done     chan error
	stopOnce sync.Once
}

func startNodeConnectorCloudflareTunnelProcess(executable string, arguments []string) (NodeConnectorCloudflareTunnelProcess, error) {
	command := exec.Command(executable, arguments...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return startNodeConnectorCloudflareTunnelCommand(command)
}

func startNodeConnectorCloudflareTunnelCommand(command *exec.Cmd) (NodeConnectorCloudflareTunnelProcess, error) {
	if err := command.Start(); err != nil {
		return nil, err
	}
	process := &nodeConnectorCloudflareTunnelOSProcess{command: command, done: make(chan error, 1)}
	go func() {
		process.done <- command.Wait()
		close(process.done)
	}()
	return process, nil
}

func (process *nodeConnectorCloudflareTunnelOSProcess) PID() int {
	if process.command.Process == nil {
		return 0
	}
	return process.command.Process.Pid
}

func (process *nodeConnectorCloudflareTunnelOSProcess) Done() <-chan error {
	return process.done
}

func (process *nodeConnectorCloudflareTunnelOSProcess) Stop(ctx context.Context) error {
	select {
	case <-process.done:
		return nil
	default:
	}
	process.stopOnce.Do(func() {
		if err := process.command.Process.Signal(os.Interrupt); err != nil {
			_ = process.command.Process.Kill()
		}
	})
	select {
	case <-process.done:
		return nil
	case <-ctx.Done():
		_ = process.command.Process.Kill()
		select {
		case <-process.done:
			return nil
		case <-time.After(100 * time.Millisecond):
			return errors.New("process did not stop")
		}
	}
}
