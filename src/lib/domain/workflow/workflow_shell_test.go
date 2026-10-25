package workflow

import (
	"strings"
	"testing"
)

func TestWorkflowHostShellValidation(t *testing.T) {
	tests := []struct {
		name string
		step Step
		want string
	}{
		{name: "native default", step: Step{Kind: "host", Cmd: "echo hello"}},
		{name: "Bash script", step: Step{Kind: "host", Shell: "bash", Run: RunSpec{"setup.sh"}}},
		{name: "PowerShell script", step: Step{Kind: "host", Shell: "powershell", Run: RunSpec{"setup.ps1"}}},
		{name: "PowerShell 7", step: Step{Kind: "host", Shell: "pwsh", PreScript: "setup.ps1"}},
		{name: "unknown", step: Step{Kind: "host", Shell: "cmd"}, want: "shell must be"},
		{name: "invalid case", step: Step{Kind: "host", Shell: "PowerShell"}, want: "shell must be"},
		{name: "container", step: Step{Shell: "bash", Cmd: "echo hello"}, want: "requires kind: host"},
		{name: "package", step: Step{WorkflowName: "child", Package: "example", Shell: "bash"}, want: "requires kind: host"},
		{name: "builtin", step: Step{Kind: "host", Shell: "bash", HostBuiltin: "compose_up"}, want: "cannot be combined with host_builtin"},
		{name: "Bash file with PowerShell", step: Step{Kind: "host", Shell: "pwsh", Run: RunSpec{"setup.sh"}}, want: "conflicts with Bash script"},
		{name: "PowerShell file with Bash", step: Step{Kind: "host", Shell: "bash", PreScript: "setup.PS1"}, want: "conflicts with PowerShell script"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateStepShell(0, test.step)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestWorkflowHostShellParsesMainAndFinallySteps(t *testing.T) {
	workflow, err := ParseWorkflowYAML([]byte(`name: host-shell
steps:
  - kind: host
    shell: pwsh
    command: Write-Output hello
finally:
  - kind: host
    shell: bash
    command: echo cleanup
`))
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Steps[0].Shell != "pwsh" || workflow.Finally[0].Shell != "bash" {
		t.Fatalf("shell fields lost: %+v %+v", workflow.Steps, workflow.Finally)
	}
	if err := ValidateLoadedWorkflow(workflow); err != nil {
		t.Fatal(err)
	}
	workflow.Finally[0].Shell = "unknown"
	if err := ValidateLoadedWorkflow(workflow); err == nil {
		t.Fatal("finally shell was not validated")
	}
}
