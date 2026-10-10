package remotecmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

// hostedSetupResult is produced privately by the resolver after browser login.
// The resolver owns identity, account selection and credential renewal.
type hostedSetupResult struct {
	Schema   string `json:"schema"`
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
}

func requireLocalBroker(root string) error {
	connection, err := readConnection(root)
	if err != nil {
		return err
	}
	if connection != nil && connection.Mode == "hosted" {
		return errors.New("this connection uses a hosted broker; no local broker service is needed")
	}
	return nil
}

func validateSetupMode(root, mode, resolver string) error {
	connection, err := readConnection(root)
	if err != nil {
		return err
	}
	if connection != nil && (connection.Mode != mode || connection.Resolver.Name != resolver) {
		return errors.New("a different remote provider is configured; use a separate --state directory to preserve existing machines")
	}
	if mode == "hosted" {
		for _, name := range []string{"operator.json", "broker.json", "edge.json"} {
			if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
				return errors.New("local broker state exists; use a separate --state directory for a hosted connection")
			}
		}
	}
	return nil
}

func setupHosted(ctx context.Context, root, profilePath string, profile map[string]string, metadata infrastructure.ResolverMetadata) error {
	script := profile["DOCKPIPE_REMOTE_BROKER_SETUP"]
	if !remoteio.SafeRelative(script) {
		return errors.New("resolver must declare a relative DOCKPIPE_REMOTE_BROKER_SETUP script")
	}
	scriptPath := filepath.Join(filepath.Dir(profilePath), filepath.FromSlash(script))
	resolved, err := filepath.EvalSymlinks(scriptPath)
	if err != nil || resolved != scriptPath {
		return errors.New("hosted setup script is missing or linked")
	}
	staging, err := os.MkdirTemp(root, "hosted-login-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	output := filepath.Join(staging, "connection.json")
	providerState := filepath.Join(root, "hosted")
	if err := remoteio.PrivateDirectory(providerState); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", scriptPath, "--state", providerState, "--output", output)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stderr, os.Stderr
	command.Env = append(os.Environ(), "DOCKPIPE_BIN="+executable, "DOCKPIPE_RESOLVER_PROFILE="+profilePath)
	command.WaitDelay = 2 * time.Second
	remoteio.ContainProcess(command)
	defer remoteio.CleanupProcess(command)
	if err := command.Run(); err != nil {
		return errors.New("hosted sign-in did not finish; existing connection was preserved")
	}
	if err := adoptHostedConnection(ctx, root, output, metadata, waitForBroker); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Hosted broker connected. Open Machines in the launcher to manage your workers.")
	return nil
}

func adoptHostedConnection(ctx context.Context, root, output string, metadata infrastructure.ResolverMetadata, check func(context.Context, string, string) error) error {
	info, err := os.Lstat(output)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 {
		return errors.New("hosted resolver output is missing or exceeds its size limit")
	}
	var result hostedSetupResult
	if err := remoteio.ReadPrivate(output, &result); err != nil {
		return err
	}
	if result.Schema != contract.Version {
		return errors.New("unsupported hosted resolver output")
	}
	if err := validateHostedCredential(result.Endpoint, result.Token); err != nil {
		return err
	}
	if err := validateSetupMode(root, "hosted", metadata.Name); err != nil {
		return err
	}
	previous, err := readConnection(root)
	if err != nil {
		return err
	}
	if previous != nil && previous.Endpoint != result.Endpoint {
		return errors.New("hosted broker address changed; use a separate --state directory to preserve the existing connection")
	}
	if err := check(ctx, result.Endpoint, result.Token); err != nil {
		return errors.New("hosted broker did not accept the connection; existing connection was preserved")
	}
	config := connectionConfig{
		Schema: contract.Version, Mode: "hosted", Endpoint: result.Endpoint,
		Resolver: metadata, Token: result.Token,
	}
	return remoteio.WritePrivate(filepath.Join(root, "connection.json"), config)
}
