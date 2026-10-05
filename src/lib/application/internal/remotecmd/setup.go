package remotecmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func setup(ctx context.Context, root, listen, resolver, hostname string, checkDependencies func(string) error) error {
	if resolver == "" {
		return errors.New("setup requires --resolver")
	}
	config, err := initialize(root, listen)
	if err != nil {
		return err
	}
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}
	profilePath, err := infrastructure.ResolveResolverFilePath(workdir, resolver)
	if err != nil {
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
	if err := installService(ctx, root, "broker"); err != nil {
		return err
	}
	return waitForBroker(ctx, edge.Endpoint, config.Token)
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
