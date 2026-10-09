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

func setup(ctx context.Context, root, listen, resolver, hostname, workdir string, checkDependencies func(string) error) error {
	if resolver == "" {
		return errors.New("setup requires --resolver")
	}
	profilePath, err := infrastructure.ResolveResolverFilePath(workdir, resolver)
	if err != nil {
		return err
	}
	metadata, err := infrastructure.DescribeResolver(profilePath, resolver)
	if err != nil {
		return err
	}
	if metadata.RemoteSetup == "" {
		return errors.New("resolver does not declare a supported remote setup contract")
	}
	if metadata.RemoteSetup == "hosted" && hostname != "" {
		return errors.New("hosted broker setup does not accept --hostname")
	}
	if err := remoteio.PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "setup.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	if err := validateSetupMode(root, metadata.RemoteSetup, resolver); err != nil {
		return err
	}
	if checkDependencies != nil {
		if err := checkDependencies(profilePath); err != nil {
			return err
		}
	}
	profile, err := infrastructure.LoadResolverFile(profilePath)
	if err != nil {
		return err
	}
	if metadata.RemoteSetup == "hosted" {
		return setupHosted(ctx, root, profilePath, profile, metadata)
	}
	config, err := initialize(root, listen)
	if err != nil {
		return err
	}
	script := profile["DOCKPIPE_REMOTE_EDGE_SETUP"]
	if !remoteio.SafeRelative(script) {
		return errors.New("resolver does not declare a relative DOCKPIPE_REMOTE_EDGE_SETUP script")
	}
	resolverRoot := filepath.Dir(profilePath)
	scriptPath := filepath.Join(resolverRoot, filepath.FromSlash(script))
	resolved, err := filepath.EvalSymlinks(scriptPath)
	if err != nil || resolved != scriptPath {
		return errors.New("edge setup script is missing or linked")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "bash", scriptPath,
		"--state", filepath.Join(root, "edge"), "--output", filepath.Join(root, "edge.json"),
		"--hostname", hostname, "--origin", "http://"+config.Listen)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stderr, os.Stderr
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command.Env = append(os.Environ(), "DOCKPIPE_BIN="+executable, "DOCKPIPE_RESOLVER_PROFILE="+profilePath)
	command.WaitDelay = 2 * time.Second
	remoteio.ContainProcess(command)
	defer remoteio.CleanupProcess(command)
	if err := command.Run(); err != nil {
		return errors.New("edge setup failed; existing provider resources and local recovery state were preserved")
	}
	var edge contract.EdgeConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "edge.json"), &edge); err != nil {
		return err
	}
	if err := validateEdge(edge); err != nil {
		return err
	}
	if hostname != "" && edge.Endpoint != "https://"+hostname {
		return errors.New("resolver returned a different public endpoint")
	}
	config.Endpoint = edge.Endpoint
	if err := remoteio.WritePrivate(filepath.Join(root, "operator.json"), config); err != nil {
		return err
	}
	connection := connectionConfig{
		Schema: contract.Version, Mode: "local", Endpoint: edge.Endpoint,
		Resolver: metadata, EdgeDigest: edgeDigest(edge),
	}
	if err := remoteio.WritePrivate(filepath.Join(root, "connection.json"), connection); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Edge configured. Installing the broker user service and checking the public endpoint...")
	if err := installService(ctx, root, "broker"); err != nil {
		return err
	}
	if err := waitForBroker(ctx, edge.Endpoint, config.Token); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Broker ready at %s. In the launcher, open Machines → Add another machine to pair a worker.\n", edge.Endpoint)
	return nil
}

func validateEdge(edge contract.EdgeConfig) error {
	if edge.Schema != contract.Version || !filepath.IsAbs(edge.Executable) || len(edge.Arguments) == 0 || len(edge.Arguments) > 64 {
		return errors.New("invalid edge resolver output")
	}
	if err := contract.Endpoint(edge.Endpoint); err != nil {
		return err
	}
	info, err := os.Stat(edge.Executable)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return errors.New("edge executable is unavailable")
	}
	return nil
}

func waitForBroker(ctx context.Context, endpoint, token string) error {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	client, err := remoteio.NewClient(endpoint, token)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	for ctx.Err() == nil {
		var response struct {
			Schema string `json:"schema"`
		}
		if err := client.Call(ctx, "/v1/health", struct{}{}, &response); err == nil && response.Schema == contract.Version {
			return nil
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return errors.New("broker did not become reachable; preserve setup and inspect the user service and edge logs")
}
