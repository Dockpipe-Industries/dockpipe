package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func configureMonitoring(root string, system, docker, disable bool) error {
	if disable && (system || docker) || !disable && !system && !docker {
		return errors.New("choose --system, --docker, both, or --disable")
	}
	var worker contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &worker); err != nil {
		return errors.New("configure monitoring on an already paired worker")
	}
	if worker.Schema != contract.Version || !contract.ValidID(worker.Node) || len(worker.Token) != 64 || contract.Endpoint(worker.Endpoint) != nil {
		return errors.New("worker identity is invalid; repair pairing before enabling monitoring")
	}
	permission := remoteio.MonitoringPermission{System: system, Docker: docker}
	if err := remoteio.WritePrivate(filepath.Join(root, "monitoring.json"), permission); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Monitoring permission saved. A running updated worker applies it on its next monitoring cycle.")
	return json.NewEncoder(os.Stdout).Encode(permission)
}

func observeRemote(ctx context.Context, root, node string) error {
	if !contract.ValidID(node) {
		return errors.New("--node must select a paired machine")
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var snapshot remoteio.MachineSnapshot
	if err := client.Call(ctx, "/v1/observe", map[string]string{"node": node}, &snapshot); err != nil {
		return fmt.Errorf("remote monitoring unavailable; verify the broker supports monitoring: %w", err)
	}
	if snapshot.Schema != remoteio.MonitoringSchema || snapshot.Node != node {
		return errors.New("remote monitoring returned a different machine or schema")
	}
	return json.NewEncoder(os.Stdout).Encode(snapshot)
}
