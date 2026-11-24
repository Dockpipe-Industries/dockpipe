package telemetry

import "time"

const MachineSchema = "dockpipe.machine_snapshot.v1"

// Machine snapshots share the remote-monitoring wire budget. This bounds one
// response independently of the transport used to request it.
const MaxMachineSnapshotBytes = 1 << 20

type MachineSnapshot struct {
	Schema     string          `json:"schema"`
	Node       string          `json:"node"`
	ReceivedAt time.Time       `json:"received_at"`
	State      string          `json:"state"`
	Execution  *Snapshot       `json:"execution,omitempty"`
	Docker     *DockerSnapshot `json:"docker,omitempty"`
	Notices    []string        `json:"notices,omitempty"`
}
