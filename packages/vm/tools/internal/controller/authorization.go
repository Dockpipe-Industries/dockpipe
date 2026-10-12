package controller

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"path/filepath"
)

type ProcessIdentity struct {
	PID           int    `json:"pid"`
	UID           int    `json:"uid"`
	StartTicks    uint64 `json:"start_ticks"`
	ExecutableSHA string `json:"executable_sha256"`
	CommandSHA    string `json:"command_sha256"`
	InstanceRoot  string `json:"instance_root"`
}

type DestructiveAuthorization struct {
	Qualification           bool
	Disposable              bool
	MachineUUID             string
	DiskSerial              string
	RunID                   string
	CheckpointAuthenticated bool
	ExpectedProcess         ProcessIdentity
	ObservedProcess         ProcessIdentity
	ExpectedTokenSHA256     string
	PresentedToken          string
}

type HardPowerPlan struct {
	PID           int    `json:"pid"`
	StartTicks    uint64 `json:"start_ticks"`
	ExecutableSHA string `json:"executable_sha256"`
	Mechanism     string `json:"mechanism"`
	Signal        string `json:"signal"`
	Execute       bool   `json:"execute"`
}

// PlanHardPower performs no lookup and sends no signal. A later separately
// approved slice must re-read pidfd ownership immediately before execution.
func PlanHardPower(auth DestructiveAuthorization) (HardPowerPlan, error) {
	if !auth.Qualification || !auth.Disposable || !auth.CheckpointAuthenticated || auth.MachineUUID == "" || auth.DiskSerial == "" || auth.RunID == "" {
		return HardPowerPlan{}, fmt.Errorf("destructive planning requires qualification, disposable identity, and authenticated checkpoint")
	}
	if err := validateProcess(auth.ExpectedProcess); err != nil {
		return HardPowerPlan{}, err
	}
	if auth.ExpectedProcess != auth.ObservedProcess {
		return HardPowerPlan{}, fmt.Errorf("QEMU process ownership or identity changed")
	}
	sum := sha256.Sum256([]byte(auth.PresentedToken))
	want, err := hex.DecodeString(auth.ExpectedTokenSHA256)
	if err != nil || len(want) != sha256.Size || subtle.ConstantTimeCompare(sum[:], want) != 1 {
		return HardPowerPlan{}, fmt.Errorf("destructive authorization token rejected")
	}
	return HardPowerPlan{PID: auth.ObservedProcess.PID, StartTicks: auth.ObservedProcess.StartTicks, ExecutableSHA: auth.ObservedProcess.ExecutableSHA, Mechanism: "pidfd_send_signal", Signal: "SIGKILL", Execute: false}, nil
}

func validateProcess(p ProcessIdentity) error {
	if p.PID <= 1 || p.UID < 0 || p.StartTicks == 0 || len(p.ExecutableSHA) != 64 || len(p.CommandSHA) != 64 || !filepath.IsAbs(p.InstanceRoot) {
		return fmt.Errorf("exact owned QEMU process identity is incomplete")
	}
	return nil
}
