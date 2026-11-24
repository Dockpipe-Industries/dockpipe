package remotecmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

func networkCommand(parent context.Context, root, command, path string) error {
	if command == "network-config" {
		var config contract.WorkerConfig
		if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
			return err
		}
		client, err := remoteio.NewWorkerClient(config)
		if err != nil {
			return err
		}
		defer client.HTTP.CloseIdleConnections()
		if client.Identity == nil {
			return errors.New("network grants require a paired machine identity")
		}
		var permission contract.NetworkPermissions
		if path == "" {
			permission, err = remoteio.LoadNetworkPermissions(root)
		} else {
			var data []byte
			data, err = readNetworkInput(parent, path)
			if err == nil {
				err = decodeFeatureInput(data, &permission)
			}
			if err == nil {
				err = permission.Validate()
			}
			if err == nil {
				err = remoteio.WritePrivate(filepath.Join(root, "network.json"), permission)
			}
		}
		if err != nil {
			return err
		}
		if err = permission.Validate(); err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(permission)
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	if command == "network-worker" || command == "network-proxy" {
		go func() { _, _ = io.Copy(io.Discard, io.LimitReader(os.Stdin, 1)); cancel() }()
	}
	if command == "network-worker" {
		return networkWorker(ctx, root)
	}
	client, err := operatorClient(root)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	if command == "network-catalog" {
		catalogs, err := client.NetworkCatalog(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(catalogs)
	}
	if command == "network-trust" {
		data, err := readNetworkInput(ctx, path)
		if err != nil {
			return err
		}
		var request contract.NetworkTrustRequest
		if decodeFeatureInput(data, &request) != nil || request.Validate() != nil {
			return errors.New("network trust requires an exact private target and digest")
		}
		snapshot, err := client.NetworkTrust(ctx, request)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(snapshot)
	}
	if command != "network-proxy" || path == "" || path == "-" {
		return errors.New("network-proxy requires --request <private-file>")
	}
	data, err := readNetworkInput(ctx, path)
	if err != nil {
		return err
	}
	var request struct {
		Schema string                 `json:"schema"`
		Target contract.FeatureTarget `json:"target"`
	}
	if decodeFeatureInput(data, &request) != nil || request.Schema != contract.NetworkVersion || request.Target.Validate() != nil {
		return errors.New("invalid exact network proxy target")
	}
	listener, server, password, err := remoteio.StartNetworkProxy(ctx, client, request.Target)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer server.Close()
	trust, err := proxyNetworkTrust(ctx, client, request.Target)
	if err != nil {
		return err
	}
	// #nosec G117 -- The private network-proxy readiness protocol delivers the per-instance loopback credential to its caller.
	if err = json.NewEncoder(os.Stdout).Encode(struct {
		Type     string                         `json:"type"`
		Endpoint string                         `json:"endpoint"`
		Username string                         `json:"username"`
		Password string                         `json:"password"`
		Target   contract.FeatureTarget         `json:"target"`
		TLSTrust *contract.NetworkTrustSnapshot `json:"tls_trust,omitempty"`
	}{Type: "ready", Endpoint: remoteio.NetworkProxyEndpoint(listener), Username: "dockpipe", Password: password, Target: request.Target, TLSTrust: trust}); err != nil {
		return err
	}
	failures := make(chan error, 1)
	go func() { failures <- server.Serve(listener) }()
	// An authority/catalog failure terminates this proxy instance, invalidating all
	// active browser streams; recovery requires a freshly captured exact target.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-ticker.C:
			if err := client.Call(ctx, "/v1/network-pulse", request.Target, nil); err != nil {
				return err
			}
			catalogs, err := client.NetworkCatalog(ctx)
			if err != nil {
				return err
			}
			available := false
			for _, catalog := range catalogs {
				if catalog.Target == request.Target && catalog.Enabled {
					available = true
				}
			}
			if !available {
				return errors.New("network target generation or grant became unavailable")
			}
		}
	}
}

func networkWorker(parent context.Context, root string) error {
	var config contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &config); err != nil {
		return err
	}
	client, err := remoteio.NewWorkerClient(config)
	if err != nil {
		return err
	}
	defer client.HTTP.CloseIdleConnections()
	return remoteio.RunNetworkWorker(parent, root, config, client, func() error {
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"type": "ready"})
	})
}

func readNetworkInput(ctx context.Context, path string) ([]byte, error) {
	if path == "" || path == "-" {
		return nil, errors.New("network request requires a named private file")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 || infrastructure.ValidatePrivatePath(path, false) != nil {
		return nil, errors.New("network request requires a private regular file of at most 4 KiB")
	}
	return readFeatureInput(ctx, path)
}

func proxyNetworkTrust(ctx context.Context, client *remoteio.Client, target contract.FeatureTarget) (*contract.NetworkTrustSnapshot, error) {
	catalogs, err := client.NetworkCatalog(ctx)
	if err != nil {
		return nil, err
	}
	for _, catalog := range catalogs {
		if catalog.Target != target || !catalog.Enabled {
			continue
		}
		if catalog.TLSTrust == nil || catalog.TLSTrust.State == "disabled" {
			return nil, nil
		}
		if catalog.TLSTrust.State != "ready" {
			return nil, errors.New("selected network TLS trust sharing is unavailable")
		}
		snapshot, err := client.NetworkTrust(ctx, contract.NetworkTrustRequest{Schema: contract.NetworkTrustVersion, Target: target, Digest: catalog.TLSTrust.Digest})
		if err != nil {
			return nil, err
		}
		return &snapshot, nil
	}
	return nil, errors.New("selected network target changed before readiness")
}
