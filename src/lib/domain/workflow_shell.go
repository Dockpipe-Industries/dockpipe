package domain

import "dockpipe/src/lib/domain/workflow"

type HostShell = workflow.HostShell

const (
	HostShellBash       = workflow.HostShellBash
	HostShellPowerShell = workflow.HostShellPowerShell
	HostShellPwsh       = workflow.HostShellPwsh
)

func NormalizeHostShell(value string) HostShell {
	return workflow.NormalizeHostShell(value)
}

func ValidateHostScriptShell(shell HostShell, scriptPath string) error {
	return workflow.ValidateHostScriptShell(shell, scriptPath)
}

func ValidateStepShell(index int, step Step) error {
	return workflow.ValidateStepShell(index, step)
}
