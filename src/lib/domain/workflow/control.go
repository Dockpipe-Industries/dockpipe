package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// WorkflowControl embeds a real PipeLang host orchestration program.
type WorkflowControl struct {
	Schema     int                     `yaml:"schema"`
	Kind       string                  `yaml:"kind"`
	Source     string                  `yaml:"source"`
	Inputs     map[string]ControlInput `yaml:"inputs,omitempty"`
	Limits     ControlLimits           `yaml:"limits,omitempty"`
	SourceLine int                     `yaml:"-" json:"-"`
}

type ControlLimits struct {
	MaxCalls              int `yaml:"max_calls,omitempty"`
	ComputeTimeoutSeconds int `yaml:"compute_timeout_seconds,omitempty"`
}

type ControlInput struct {
	Type     string `yaml:"type"`
	Value    any    `yaml:"value,omitempty"`
	Env      string `yaml:"env,omitempty"`
	HasValue bool   `yaml:"-" json:"-"`
}

type StepControl struct {
	Parameters []ControlParameter `yaml:"parameters"`
}

type ControlParameter struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Env  string `yaml:"env"`
}

var controlIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
var controlEnvironment = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func controlMapping(n *yaml.Node, allowed ...string) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("control: expected mapping")
	}
	seen := map[string]bool{}
	for i := 0; i < len(n.Content); i += 2 {
		key := n.Content[i].Value
		known := false
		for _, candidate := range allowed {
			if candidate == key {
				known = true
			}
		}
		if !known || seen[key] {
			return fmt.Errorf("control: unknown or duplicate field %q", key)
		}
		seen[key] = true
	}
	return nil
}

// guardControlNode runs before decoding so aliases, merge keys, and explicit tags
// cannot change the meaning of the strict control namespace.
func guardControlNode(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Tag == "!!merge" || n.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf("control: aliases, merges, and explicit tags are not supported")
	}
	for _, child := range n.Content {
		if err := guardControlNode(child); err != nil {
			return err
		}
	}
	return nil
}

// hasControlNamespace follows YAML references only to discover control fields.
// The visited set prevents recursive aliases from looping before YAML decoding.
func hasControlNamespace(n *yaml.Node, seen map[*yaml.Node]bool) bool {
	if n == nil || seen[n] {
		return false
	}
	seen[n] = true
	if n.Kind == yaml.AliasNode {
		return hasControlNamespace(n.Alias, seen)
	}
	if n.Kind == yaml.SequenceNode {
		for _, child := range n.Content {
			if hasControlNamespace(child, seen) {
				return true
			}
		}
	}
	if n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(n.Content); i += 2 {
		key, value := n.Content[i].Value, n.Content[i+1]
		if key == "control" {
			return true
		}
		if key == "<<" || key == "steps" || key == "finally" || key == "tasks" || key == "group" {
			if hasControlNamespace(value, seen) {
				return true
			}
		}
	}
	return false
}

func validateControlDocument(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode {
		if hasControlNamespace(n, map[*yaml.Node]bool{}) {
			return fmt.Errorf("control: aliases cannot supply control namespaces")
		}
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(n.Content); i += 2 {
		key, value := n.Content[i].Value, n.Content[i+1]
		if key == "<<" && hasControlNamespace(value, map[*yaml.Node]bool{}) {
			return fmt.Errorf("control: merges cannot supply control namespaces")
		}
		if key == "control" {
			if value.Kind != yaml.MappingNode {
				return fmt.Errorf("control: expected mapping")
			}
			if err := guardControlNode(value); err != nil {
				return err
			}
		}
		if key == "steps" || key == "finally" || key == "tasks" {
			if value.Kind == yaml.AliasNode && hasControlNamespace(value, map[*yaml.Node]bool{}) {
				return fmt.Errorf("control: aliases cannot supply control namespaces")
			}
			if value.Kind == yaml.SequenceNode {
				for _, step := range value.Content {
					if err := validateControlDocument(step); err != nil {
						return err
					}
				}
			}
		}
		if key == "group" {
			if err := validateControlDocument(value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *WorkflowControl) UnmarshalYAML(n *yaml.Node) error {
	if err := controlMapping(n, "schema", "kind", "source", "inputs", "limits"); err != nil {
		return err
	}
	var source *yaml.Node
	for i := 0; i < len(n.Content); i += 2 {
		key, value := n.Content[i].Value, n.Content[i+1]
		if key == "schema" && value.Tag != "!!int" {
			return fmt.Errorf("control.schema: requires integer 1")
		}
		if key == "kind" && value.Tag != "!!str" {
			return fmt.Errorf("control.kind: requires host string")
		}
		if key == "inputs" && value.Kind == yaml.MappingNode {
			for j := 0; j < len(value.Content); j += 2 {
				if value.Content[j].Tag != "!!str" {
					return fmt.Errorf("control.inputs: field names must be strings")
				}
			}
		}
		if (key == "inputs" || key == "limits") && value.Kind != yaml.MappingNode {
			return fmt.Errorf("control.%s: requires mapping", key)
		}
	}
	for i := 0; i < len(n.Content); i += 2 {
		if n.Content[i].Value == "source" {
			source = n.Content[i+1]
		}
	}
	if source == nil || source.Kind != yaml.ScalarNode || source.Tag != "!!str" || source.Style != yaml.LiteralStyle {
		return fmt.Errorf("control.source: requires a literal block string (|)")
	}
	type plain WorkflowControl
	var decoded plain
	if err := n.Decode(&decoded); err != nil {
		return err
	}
	*c = WorkflowControl(decoded)
	c.SourceLine = source.Line + 1
	return nil
}

func (l *ControlLimits) UnmarshalYAML(n *yaml.Node) error {
	if err := controlMapping(n, "max_calls", "compute_timeout_seconds"); err != nil {
		return err
	}
	for i := 1; i < len(n.Content); i += 2 {
		if n.Content[i].Tag != "!!int" {
			return fmt.Errorf("control.limits: requires integer values")
		}
	}
	type plain ControlLimits
	return n.Decode((*plain)(l))
}

func (c *ControlInput) UnmarshalYAML(n *yaml.Node) error {
	if err := controlMapping(n, "type", "value", "env"); err != nil {
		return err
	}
	hasEnv := false
	for i := 0; i < len(n.Content); i += 2 {
		switch n.Content[i].Value {
		case "value":
			c.HasValue = true
		case "env":
			hasEnv = true
			if n.Content[i+1].Tag != "!!str" {
				return fmt.Errorf("control input.env: requires a string")
			}
		case "type":
			if n.Content[i+1].Tag != "!!str" {
				return fmt.Errorf("control input.type: requires a string")
			}
		}
	}
	type plain ControlInput
	var decoded plain
	if err := n.Decode(&decoded); err != nil {
		return err
	}
	decoded.HasValue = c.HasValue
	*c = ControlInput(decoded)
	if c.HasValue == hasEnv || (hasEnv && strings.TrimSpace(c.Env) == "") {
		return fmt.Errorf("control input: requires exactly one of value or nonempty env")
	}
	return nil
}

func (c *StepControl) UnmarshalYAML(n *yaml.Node) error {
	if err := controlMapping(n, "parameters"); err != nil {
		return err
	}
	if len(n.Content) != 2 || n.Content[1].Kind != yaml.SequenceNode {
		return fmt.Errorf("control.parameters: requires an ordered list")
	}
	type plain StepControl
	return n.Decode((*plain)(c))
}

func (p *ControlParameter) UnmarshalYAML(n *yaml.Node) error {
	if err := controlMapping(n, "name", "type", "env"); err != nil {
		return err
	}
	for i := 1; i < len(n.Content); i += 2 {
		if n.Content[i].Tag != "!!str" {
			return fmt.Errorf("control parameter: requires string fields")
		}
	}
	type plain ControlParameter
	return n.Decode((*plain)(p))
}

func controlStringValid(value string) bool {
	return len(value) <= 65536 && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func validateControlInput(input ControlInput) error {
	switch input.Type {
	case "bool", "int", "string", "List<string>":
	default:
		return fmt.Errorf("unsupported type %q", input.Type)
	}
	if input.HasValue == (input.Env != "") {
		return fmt.Errorf("requires exactly one of value or env")
	}
	if !input.HasValue {
		if strings.TrimSpace(input.Env) == "" || !controlStringValid(input.Env) {
			return fmt.Errorf("invalid env reference")
		}
		return nil
	}
	valid := false
	switch input.Type {
	case "bool":
		_, valid = input.Value.(bool)
	case "int":
		switch input.Value.(type) {
		case int, int64:
			valid = true
		}
	case "string":
		value, ok := input.Value.(string)
		valid = ok && controlStringValid(value)
	case "List<string>":
		switch values := input.Value.(type) {
		case []any:
			valid = len(values) <= 256
			for _, value := range values {
				text, ok := value.(string)
				valid = valid && ok && controlStringValid(text)
			}
		case []string:
			valid = len(values) <= 256
			for _, value := range values {
				valid = valid && controlStringValid(value)
			}
		}
	}
	if !valid {
		return fmt.Errorf("value must be a bounded %s literal", input.Type)
	}
	return nil
}

func controlParameterEnvValid(key string) bool {
	if !controlEnvironment.MatchString(key) {
		return false
	}
	for _, prefix := range []string{"DOCKPIPE_", "LD_", "DYLD_"} {
		if strings.HasPrefix(key, prefix) {
			return false
		}
	}
	switch key {
	case "PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "COMSPEC", "BASH_ENV", "ENV":
		return false
	}
	return true
}

// ValidateWorkflowControl checks the bounded host control profile without parsing PipeLang.
func ValidateWorkflowControl(w *Workflow) error {
	if w == nil {
		return nil
	}
	for _, step := range w.Finally {
		if step.Control != nil {
			return fmt.Errorf("finally: control.parameters is not supported")
		}
	}
	if w.Control == nil {
		for _, step := range w.Steps {
			if step.Control != nil {
				return fmt.Errorf("step control requires workflow control")
			}
		}
		return nil
	}
	c := w.Control
	if c.Schema != 1 || c.Kind != "host" {
		return fmt.Errorf("control: requires schema: 1 and kind: host")
	}
	if strings.TrimSpace(c.Source) == "" || !controlStringValid(c.Source) {
		return fmt.Errorf("control.source: requires nonempty UTF-8 source up to 65536 bytes without NUL")
	}
	if c.Limits.MaxCalls < 0 || c.Limits.MaxCalls > 4096 || c.Limits.ComputeTimeoutSeconds < 0 || c.Limits.ComputeTimeoutSeconds > 60 {
		return fmt.Errorf("control.limits: max_calls must be 0..4096 and compute_timeout_seconds 0..60")
	}
	if len(c.Inputs) > 64 {
		return fmt.Errorf("control.inputs: maximum 64 fields")
	}
	for name, input := range c.Inputs {
		if !controlIdentifier.MatchString(name) {
			return fmt.Errorf("control.inputs: invalid field name %q", name)
		}
		if err := validateControlInput(input); err != nil {
			return fmt.Errorf("control.inputs.%s: %w", name, err)
		}
	}
	if len(w.Steps) == 0 || len(w.Steps) > 128 {
		return fmt.Errorf("control: requires 1..128 body steps")
	}
	ids := map[string]bool{}
	for _, step := range w.Steps {
		if !controlIdentifier.MatchString(step.ID) || ids[step.ID] {
			return fmt.Errorf("control: body step IDs must be unique ASCII identifiers up to 64 characters")
		}
		ids[step.ID] = true
		if step.Blocking != nil && !*step.Blocking {
			return fmt.Errorf("control: asynchronous body steps are not supported")
		}
		if step.Control == nil {
			continue
		}
		if len(step.Control.Parameters) > 16 {
			return fmt.Errorf("step %s control.parameters: maximum 16 parameters", step.ID)
		}
		names, envs := map[string]bool{}, map[string]bool{}
		for _, parameter := range step.Control.Parameters {
			if !controlIdentifier.MatchString(parameter.Name) || names[parameter.Name] {
				return fmt.Errorf("step %s: invalid or duplicate control parameter name", step.ID)
			}
			if !controlParameterEnvValid(parameter.Env) || envs[parameter.Env] {
				return fmt.Errorf("step %s: invalid, reserved, or duplicate control parameter env", step.ID)
			}
			if parameter.Type != "bool" && parameter.Type != "int" && parameter.Type != "string" {
				return fmt.Errorf("step %s: control parameters require bool, int, or string", step.ID)
			}
			names[parameter.Name], envs[parameter.Env] = true, true
		}
	}
	return nil
}
