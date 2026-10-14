package telemetry

// SystemResources contains counters, not percentages derived from unrelated
// samples. Consumers reset rate baselines when BootID or machine changes.
type SystemResources struct {
	BootID          string   `json:"boot_id"`
	LogicalCPUs     int      `json:"logical_cpus"`
	CPUTotal        *uint64  `json:"cpu_total"`
	CPUIdle         *uint64  `json:"cpu_idle"`
	MemoryTotal     *uint64  `json:"memory_total"`
	MemoryAvailable *uint64  `json:"memory_available"`
	SwapTotal       *uint64  `json:"swap_total"`
	SwapFree        *uint64  `json:"swap_free"`
	DiskTotal       *uint64  `json:"disk_total"`
	DiskAvailable   *uint64  `json:"disk_available"`
	NetworkReceived *uint64  `json:"network_received"`
	NetworkSent     *uint64  `json:"network_sent"`
	UptimeSeconds   *float64 `json:"uptime_seconds"`
	Load1           *float64 `json:"load_1"`
	Notices         []string `json:"notices"`
}
