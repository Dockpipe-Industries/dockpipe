package telemetry

const MaxDockerObjects = 256

type DockerContainer struct {
	ID         string
	Names      string
	State      string
	Status     string
	Image      string
	Ports      string
	RunningFor string
}

type DockerNetwork struct {
	ID     string
	Name   string
	Driver string
	Scope  string
}

type DockerVolume struct {
	Name   string
	Driver string
}

type DockerTopology struct {
	ID       string `json:"Id"`
	Name     string
	Labels   map[string]string
	Networks map[string]struct{}
	Mounts   []DockerMount
}

type DockerMount struct {
	Name string
	Type string
}

// DockerSnapshot is an allowlisted inventory. No environment, arbitrary labels,
// mount source paths, command arguments, or application logs leave the worker.
type DockerSnapshot struct {
	Containers []DockerContainer `json:"containers"`
	Networks   []DockerNetwork   `json:"networks"`
	Volumes    []DockerVolume    `json:"volumes"`
	Topology   []DockerTopology  `json:"topology"`
	Notices    []string          `json:"notices"`
	Available  bool              `json:"available"`
	Truncated  bool              `json:"truncated"`
}
