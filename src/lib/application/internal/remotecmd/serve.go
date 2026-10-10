package remotecmd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func serve(ctx context.Context, root string) error {
	if err := requireLocalBroker(root); err != nil {
		return err
	}
	if err := remoteio.PrivateDirectory(root); err != nil {
		return err
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "broker.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	var config OperatorConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &config); err != nil {
		return err
	}
	if err := validateListen(config.Listen); err != nil {
		return err
	}
	broker, err := remoteio.NewBroker(root)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: broker, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	defer server.Close()
	failures := make(chan error, 2)
	go func() { failures <- server.Serve(listener) }()
	edgeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	var edge contract.EdgeConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "edge.json"), &edge); err == nil {
		if err := validateEdge(edge); err != nil {
			return err
		}
		command := exec.CommandContext(edgeContext, edge.Executable, edge.Arguments...)
		for _, name := range []string{"PATH", "HOME", "USER", "TMPDIR", "LANG"} {
			if value, exists := os.LookupEnv(name); exists {
				command.Env = append(command.Env, name+"="+value)
			}
		}
		command.Stdout, command.Stderr = os.Stderr, os.Stderr
		command.WaitDelay = 2 * time.Second
		remoteio.ContainProcess(command)
		if err := command.Start(); err != nil {
			return err
		}
		defer remoteio.CleanupProcess(command)
		go func() {
			_ = command.Wait()
			failures <- errors.New("remote edge process exited")
		}()
	} else if !os.IsNotExist(err) {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-failures:
		return err
	}
}
