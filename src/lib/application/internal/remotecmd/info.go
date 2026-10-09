package remotecmd

import (
	"errors"
	"os"
	"path/filepath"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type connectionInfo struct {
	Mode     string                           `json:"mode"`
	Endpoint string                           `json:"endpoint"`
	Resolver *infrastructure.ResolverMetadata `json:"resolver,omitempty"`
}

type workerInfo struct {
	Endpoint string `json:"endpoint"`
	Node     string `json:"node"`
}

type remoteInfo struct {
	Broker *connectionInfo `json:"broker,omitempty"`
	Worker *workerInfo     `json:"worker,omitempty"`
}

// describeRemote reads configuration only; it neither probes services nor emits
// tokens, profile values, edge arguments, or provider-owned private state.
func describeRemote(root string) (remoteInfo, error) {
	var result remoteInfo
	connection, err := readConnection(root)
	if err != nil {
		return result, err
	}
	if connection != nil && connection.Mode == "hosted" {
		result.Broker = &connectionInfo{Mode: "hosted", Endpoint: connection.Endpoint, Resolver: &connection.Resolver}
	} else {
		var operator OperatorConfig
		if err := remoteio.ReadPrivate(filepath.Join(root, "operator.json"), &operator); err == nil {
			if operator.Schema != contract.Version {
				return result, errors.New("unsupported operator configuration")
			}
			if err := contract.Endpoint(operator.Endpoint); err != nil {
				return result, err
			}
			result.Broker = &connectionInfo{Mode: "local", Endpoint: operator.Endpoint}
			if connection != nil && connection.Endpoint == operator.Endpoint {
				var edge contract.EdgeConfig
				if err := remoteio.ReadPrivate(filepath.Join(root, "edge.json"), &edge); err != nil && !os.IsNotExist(err) {
					return result, err
				} else if err == nil && connection.EdgeDigest == edgeDigest(edge) {
					result.Broker.Resolver = &connection.Resolver
				}
			}
		} else if !os.IsNotExist(err) {
			return result, err
		}
	}
	var worker contract.WorkerConfig
	if err := remoteio.ReadPrivate(filepath.Join(root, "worker.json"), &worker); err == nil {
		if worker.Schema != contract.Version {
			return result, errors.New("unsupported worker configuration")
		}
		if err := contract.Endpoint(worker.Endpoint); err != nil {
			return result, err
		}
		result.Worker = &workerInfo{Endpoint: worker.Endpoint, Node: worker.Node}
	} else if !os.IsNotExist(err) {
		return result, err
	}
	return result, nil
}
