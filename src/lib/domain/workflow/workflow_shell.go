package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
)

// HostShell selects a host command interpreter. An empty value uses script
// extension inference, then the operating system's default host shell.
type HostShell string

const (
	HostShellBash       HostShell = "bash"
	HostShellPowerShell HostShell = "powershell"
	HostShellPwsh       HostShell = "pwsh"
)

func NormalizeHostShell(value string) HostShell {
	return HostShell(strings.TrimSpace(value))
}

func (shell HostShell) IsValid() bool {
	switch shell {
	case "", HostShellBash, HostShellPowerShell, HostShellPwsh:
		return true
	default:
		return false
	}
}

// ValidateHostScriptShell prevents an explicit interpreter from executing a file
// authored for a different shell. Unknown extensions use the selected shell.
func ValidateHostScriptShell(shell HostShell, scriptPath string) error {
	if !shell.IsValid() {
		return fmt.Errorf("shell must be bash, powershell, or pwsh")
	}
	if shell == "" {
		return nil
	}
	extension := strings.ToLower(filepath.Ext(scriptPath))
	if extension == ".sh" && shell != HostShellBash {
		return fmt.Errorf("shell %q conflicts with Bash script %q", shell, scriptPath)
	}
	if extension == ".ps1" && shell == HostShellBash {
		return fmt.Errorf("shell %q conflicts with PowerShell script %q", shell, scriptPath)
	}
	return nil
}

func ValidateStepShell(index int, step Step) error {
	shell := NormalizeHostShell(step.Shell)
	if !shell.IsValid() {
		return fmt.Errorf("step %d: shell must be bash, powershell, or pwsh", index+1)
	}
	if shell == "" {
		return nil
	}
	if !step.IsHostStep() || step.UsesPackagedWorkflow() {
		return fmt.Errorf("step %d: shell requires kind: host", index+1)
	}
	if strings.TrimSpace(step.HostBuiltin) != "" {
		return fmt.Errorf("step %d: shell cannot be combined with host_builtin", index+1)
	}
	for _, scriptPath := range step.RunPaths() {
		if err := ValidateHostScriptShell(shell, scriptPath); err != nil {
			return fmt.Errorf("step %d: %w", index+1, err)
		}
	}
	return nil
}
