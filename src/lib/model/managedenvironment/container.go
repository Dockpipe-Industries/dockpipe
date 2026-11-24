package managedenvironment

import (
	"errors"
	"path"
	"regexp"
	"strings"
)

var (
	containerImage     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(?::[0-9]+)?(?:/[a-z0-9][a-z0-9._-]*)*(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})?(?:@sha256:[a-f0-9]{64})?$`)
	containerDirectory = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)
)

// Container describes a Linux container on the owning host's Docker engine.
type Container struct {
	Image    string     `yaml:"image" json:"image"`
	Startup  string     `yaml:"startup,omitempty" json:"startup,omitempty"`
	Network  string     `yaml:"network,omitempty" json:"network,omitempty"`
	Security string     `yaml:"security,omitempty" json:"security,omitempty"`
	Restart  string     `yaml:"restart,omitempty" json:"restart,omitempty"`
	User     string     `yaml:"user,omitempty" json:"user,omitempty"`
	ReadOnly bool       `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	Publish  *LocalPort `yaml:"publish,omitempty" json:"publish,omitempty"`
	DataPath string     `yaml:"data_path,omitempty" json:"data_path,omitempty"`
}

type LocalPort struct {
	Host      uint16 `yaml:"host" json:"host"`
	Container uint16 `yaml:"container" json:"container"`
}

func (settings Container) Validate() error {
	if len(settings.Image) > 512 || !containerImage.MatchString(settings.Image) {
		return errors.New("enter a Docker image such as ubuntu:24.04 or nginx:alpine")
	}
	switch settings.Startup {
	case "", "image", "keep-running":
	default:
		return errors.New("container startup must be image or keep-running")
	}
	switch settings.Network {
	case "", "bridge", "none":
	default:
		return errors.New("container network must be bridge or none")
	}
	switch settings.Security {
	case "", "standard", "restricted":
	default:
		return errors.New("container security must be standard or restricted")
	}
	switch settings.Restart {
	case "", "no", "unless-stopped":
	default:
		return errors.New("container restart must be no or unless-stopped")
	}
	switch settings.User {
	case "", "image", "non-root":
	default:
		return errors.New("container user must be image or non-root")
	}
	if settings.Publish != nil && (settings.Publish.Host == 0 || settings.Publish.Container == 0 || settings.Network == "none") {
		return errors.New("publishing a port requires bridge networking and two ports from 1 to 65535")
	}
	if settings.DataPath != "" {
		target := settings.DataPath
		if len(target) > 256 || !containerDirectory.MatchString(target) || path.Clean(target) != target ||
			target == "/" || target == "/dev" || target == "/proc" || target == "/sys" ||
			strings.HasPrefix(target, "/dev/") || strings.HasPrefix(target, "/proc/") || strings.HasPrefix(target, "/sys/") {
			return errors.New("persistent storage requires an absolute container folder such as /data")
		}
	}
	return nil
}
