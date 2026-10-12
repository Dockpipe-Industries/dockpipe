package remotecmd

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"

	contract "dockpipe/src/lib/domain/remote"
	"dockpipe/src/lib/infrastructure"
	remoteio "dockpipe/src/lib/infrastructure/remote"
)

type OperatorConfig struct {
	Schema   string `json:"schema"`
	Endpoint string `json:"endpoint"`
	Listen   string `json:"listen"`
	Token    string `json:"token"`
}

func stateRoot(value string) (string, error) {
	if value != "" {
		return filepath.Abs(value)
	}
	root, err := infrastructure.GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "remote"), nil
}

func initialize(root, listen string) (OperatorConfig, error) {
	if err := requireLocalBroker(root); err != nil {
		return OperatorConfig{}, err
	}
	if err := remoteio.PrivateDirectory(root); err != nil {
		return OperatorConfig{}, err
	}
	config := OperatorConfig{}
	path := filepath.Join(root, "operator.json")
	if err := remoteio.ReadPrivate(path, &config); err == nil {
		if config.Schema != contract.Version || config.Listen != listen {
			return config, errors.New("existing broker configuration differs")
		}
		if _, err := os.Stat(filepath.Join(root, "broker.json")); err != nil {
			return config, errors.New("incomplete broker initialization; preserve state and inspect")
		}
		return config, nil
	} else if !os.IsNotExist(err) {
		return config, err
	}
	unlock, err := remoteio.Lock(filepath.Join(root, "broker.lock"))
	if err != nil {
		return config, err
	}
	defer unlock()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return config, errors.New("broker initialization changed concurrently; retry initialization")
	}
	if _, err := os.Lstat(filepath.Join(root, "broker.json")); !os.IsNotExist(err) {
		return config, errors.New("broker state already exists without operator configuration")
	}
	if err := validateListen(listen); err != nil {
		return config, err
	}
	token, err := remoteio.Secret()
	if err != nil {
		return config, err
	}
	config = OperatorConfig{Schema: contract.Version, Endpoint: "http://" + listen, Listen: listen, Token: token}
	state := remoteio.BrokerState{Schema: contract.Version, AdminHash: remoteio.Hash(token), Nodes: map[string]remoteio.Enrollment{}, Jobs: map[string]contract.Job{}}
	if err := remoteio.WritePrivate(path, config); err != nil {
		return config, err
	}
	return config, remoteio.WritePrivate(filepath.Join(root, "broker.json"), state)
}

func validateListen(value string) error {
	host, port, err := net.SplitHostPort(value)
	ip := net.ParseIP(host)
	number, parseErr := strconv.Atoi(port)
	if err != nil || parseErr != nil || number < 1 || number > 65535 || ip == nil || !ip.IsLoopback() {
		return errors.New("broker origin must bind an explicit numeric loopback address and port")
	}
	return nil
}
