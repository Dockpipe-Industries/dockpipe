package workflow

import (
	"strings"
	"testing"
)

const validControlWorkflow = `name: control-test
control:
  schema: 1
  kind: host
  source: |
    real PipeLang source is compiled by the owning compiler
  inputs:
    enabled:
      type: bool
      value: false
    names:
      type: List<string>
      value: []
    label:
      type: string
      value: ""
    count:
      type: int
      env: COUNT
  limits:
    max_calls: 128
    compute_timeout_seconds: 5
steps:
  - id: build
    kind: host
    cmd: echo build
    control:
      parameters:
        - name: label
          type: string
          env: BUILD_LABEL
finally:
  - kind: host
    cmd: echo cleanup
`

func TestWorkflowControlParsesTypedBindings(t *testing.T) {
	workflow, err := ParseWorkflowYAML([]byte(validControlWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLoadedWorkflow(workflow); err != nil {
		t.Fatal(err)
	}
	if workflow.Control.SourceLine != 6 {
		t.Fatalf("source line = %d, want 6", workflow.Control.SourceLine)
	}
	if !workflow.Control.Inputs["enabled"].HasValue || workflow.Control.Inputs["enabled"].Value != false {
		t.Fatal("false literal lost")
	}
	if !workflow.Control.Inputs["label"].HasValue || workflow.Control.Inputs["label"].Value != "" {
		t.Fatal("empty string literal lost")
	}
	if workflow.Control.Inputs["count"].HasValue || workflow.Control.Inputs["count"].Env != "COUNT" {
		t.Fatal("explicit env reference lost")
	}
	if workflow.Steps[0].Control.Parameters[0].Name != "label" {
		t.Fatal("ordered parameter lost")
	}
}

func TestWorkflowControlRejectsUnsafeYAML(t *testing.T) {
	tests := []struct{ name, old, replacement string }{
		{"unknown root field", "  schema: 1", "  schema: 1\n  surprise: true"},
		{"unknown limits field", "    max_calls: 128", "    surprise: 128"},
		{"unknown input field", "      value: false", "      value: false\n      surprise: true"},
		{"unknown step control field", "      parameters:", "      surprise: true\n      parameters:"},
		{"unknown parameter field", "          env: BUILD_LABEL", "          env: BUILD_LABEL\n          surprise: true"},
		{"quoted source", "  source: |\n    real PipeLang source is compiled by the owning compiler", "  source: \"program\""},
		{"folded source", "  source: |", "  source: >"},
		{"source alias", "  source: |\n    real PipeLang source is compiled by the owning compiler", "  source: *program"},
		{"input alias", "      value: false", "      value: *flag"},
		{"input explicit tag", "      value: false", "      value: !!bool false"},
		{"input merge", "      value: false", "      <<: {value: false}"},
		{"empty env", "      env: COUNT", "      env: ''"},
		{"nonstring env", "      env: COUNT", "      env: 123"},
		{"both bindings", "      value: false", "      value: false\n      env: FLAG"},
		{"missing binding", "      value: false", ""},
		{"quoted schema", "  schema: 1", "  schema: '1'"},
		{"null limits", "  limits:\n    max_calls: 128\n    compute_timeout_seconds: 5", "  limits: null"},
		{"null parameters", "      parameters:\n        - name: label\n          type: string\n          env: BUILD_LABEL", "      parameters: null"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := "program: &program |\n  program\nflag: &flag false\n" + strings.Replace(validControlWorkflow, test.old, test.replacement, 1)
			if _, err := ParseWorkflowYAML([]byte(data)); err == nil {
				t.Fatal("expected parse rejection")
			}
		})
	}
}

func TestWorkflowControlValidationBounds(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Workflow)
	}{
		{"schema", func(w *Workflow) { w.Control.Schema = 2 }},
		{"kind", func(w *Workflow) { w.Control.Kind = "container" }},
		{"empty source", func(w *Workflow) { w.Control.Source = " " }},
		{"source bytes", func(w *Workflow) { w.Control.Source = strings.Repeat("a", 65537) }},
		{"source NUL", func(w *Workflow) { w.Control.Source = "a\x00b" }},
		{"source UTF8", func(w *Workflow) { w.Control.Source = string([]byte{255}) }},
		{"call limit", func(w *Workflow) { w.Control.Limits.MaxCalls = 4097 }},
		{"negative limit", func(w *Workflow) { w.Control.Limits.MaxCalls = -1 }},
		{"timeout", func(w *Workflow) { w.Control.Limits.ComputeTimeoutSeconds = 61 }},
		{"missing steps", func(w *Workflow) { w.Steps = nil }},
		{"too many steps", func(w *Workflow) { w.Steps = make([]Step, 129) }},
		{"missing id", func(w *Workflow) { w.Steps[0].ID = "" }},
		{"invalid id", func(w *Workflow) { w.Steps[0].ID = "a-b" }},
		{"duplicate id", func(w *Workflow) { w.Steps = append(w.Steps, w.Steps[0]) }},
		{"async", func(w *Workflow) { value := false; w.Steps[0].Blocking = &value }},
		{"finally parameters", func(w *Workflow) { w.Finally[0].Control = &StepControl{} }},
		{"orphan parameters", func(w *Workflow) { w.Control = nil }},
		{"float int", func(w *Workflow) { w.Control.Inputs["count"] = ControlInput{Type: "int", Value: 1.5, HasValue: true} }},
		{"wrong bool", func(w *Workflow) {
			w.Control.Inputs["enabled"] = ControlInput{Type: "bool", Value: "false", HasValue: true}
		}},
		{"null value", func(w *Workflow) { w.Control.Inputs["enabled"] = ControlInput{Type: "bool", HasValue: true} }},
		{"input name", func(w *Workflow) {
			w.Control.Inputs["bad-name"] = ControlInput{Type: "bool", Value: true, HasValue: true}
		}},
		{"list size", func(w *Workflow) {
			w.Control.Inputs["names"] = ControlInput{Type: "List<string>", Value: make([]string, 257), HasValue: true}
		}},
		{"list element", func(w *Workflow) {
			w.Control.Inputs["names"] = ControlInput{Type: "List<string>", Value: []any{"a", false}, HasValue: true}
		}},
		{"parameter type", func(w *Workflow) { w.Steps[0].Control.Parameters[0].Type = "List<string>" }},
		{"parameter env", func(w *Workflow) { w.Steps[0].Control.Parameters[0].Env = "DOCKPIPE_ROOT" }},
		{"parameter duplicate", func(w *Workflow) {
			w.Steps[0].Control.Parameters = append(w.Steps[0].Control.Parameters, w.Steps[0].Control.Parameters[0])
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workflow, err := ParseWorkflowYAML([]byte(validControlWorkflow))
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(workflow)
			if err := ValidateWorkflowControl(workflow); err == nil {
				t.Fatal("expected validation rejection")
			}
		})
	}
}

func TestWorkflowControlRejectsControlNullAndInheritedNamespaces(t *testing.T) {
	for _, data := range []string{
		"control: null\n",
		"base: &base\n  control: {schema: 1, kind: host, source: anything}\n<<: *base\n",
		"base: &base\n  control: {parameters: []}\nsteps: [*base]\n",
	} {
		if _, err := ParseWorkflowYAML([]byte(data)); err == nil {
			t.Fatalf("expected raw control rejection: %s", data)
		}
	}
}

func TestWorkflowControlRejectsImports(t *testing.T) {
	readFile := func(string) ([]byte, error) { return []byte(validControlWorkflow), nil }
	if _, err := ParseWorkflowFromDisk([]byte("name: parent\nimports: [child.yml]\n"), ".", readFile); err == nil {
		t.Fatal("imported control must not be silently dropped")
	}
	data := validControlWorkflow + "imports: [child.yml]\n"
	if _, err := ParseWorkflowFromDisk([]byte(data), ".", readFile); err == nil {
		t.Fatal("root control cannot use YAML imports")
	}
}

func TestWorkflowControlRejectsReservedParameterEnvironment(t *testing.T) {
	for _, key := range []string{"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "COMSPEC", "BASH_ENV", "ENV", "LD_PRELOAD", "DYLD_LIBRARY_PATH", "DOCKPIPE_RUNTIME", "lowercase", "A=B"} {
		if controlParameterEnvValid(key) {
			t.Errorf("accepted reserved or invalid key %s", key)
		}
	}
	if !controlParameterEnvValid("BUILD_LABEL") {
		t.Fatal("safe parameter env rejected")
	}
}
