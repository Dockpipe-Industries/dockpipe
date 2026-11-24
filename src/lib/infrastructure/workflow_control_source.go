package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"dockpipe/src/lib/controlassets"
	"dockpipe/src/lib/domain/workflow"
)

const workflowControlProfile = "pipelang.workflow-host-control.v1"

func controlInputNames(control *workflow.WorkflowControl) []string {
	names := make([]string, 0, len(control.Inputs))
	for name := range control.Inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// controlDefinitionIdentity binds every executable step, not just generated
// signatures: changing a command must invalidate a previously built controller.
func controlDefinitionIdentity(definition *workflow.Workflow) (string, error) {
	if definition == nil || definition.Control == nil {
		return "", fmt.Errorf("workflow has no control program")
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	digest.Write([]byte(workflowControlProfile))
	digest.Write(encoded)
	source, decoder, _ := controlSource(definition)
	digest.Write([]byte(source))
	digest.Write([]byte(decoder))
	for _, name := range []string{"native/Main.cpp.in", "native/CMakeLists.txt"} {
		data, err := controlassets.Files.ReadFile(name)
		if err != nil {
			return "", err
		}
		digest.Write([]byte(name))
		digest.Write(data)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func controlSource(definition *workflow.Workflow) (string, string, int) {
	var source strings.Builder
	source.WriteString("module Dockpipe.WorkflowControl;\n\npublic Record Variable {\n    public string name;\n    public string value;\n}\n\npublic Record Inputs {\n")
	names := controlInputNames(definition.Control)
	var decoder strings.Builder
	fmt.Fprintf(&decoder, "    reader.expectInputCount(%d);\n", len(names))
	readers := map[string]string{"bool": "boolean", "int": "integer", "string": "string", "List<string>": "strings"}
	for _, name := range names {
		kind := definition.Control.Inputs[name].Type
		fmt.Fprintf(&source, "    public %s %s;\n", kind, name)
		fmt.Fprintf(&decoder, "    inputs.%s = reader.%s();\n", name, readers[kind])
	}
	decoder.WriteString("    reader.finish();\n")
	source.WriteString("}\n\npublic Interface StepHost {\n    Result<void,string> Run(string id, List<Variable> arguments);\n}\n")
	for index, step := range definition.Steps {
		fmt.Fprintf(&source, "\npublic Class WorkflowStep%d {\n    private StepHost host;\n\n    public WorkflowStep%d(StepHost host) {\n        this.host = host;\n    }\n\n    public Result<void,string> Run(", index, index)
		if step.Control != nil {
			for argumentIndex, parameter := range step.Control.Parameters {
				if argumentIndex != 0 {
					source.WriteString(", ")
				}
				fmt.Fprintf(&source, "%s %s", parameter.Type, parameter.Name)
			}
		}
		used := map[string]bool{}
		if step.Control != nil {
			for _, parameter := range step.Control.Parameters {
				used[parameter.Name] = true
			}
		}
		fresh := func(base string) string {
			for used[base] {
				base += "_"
			}
			used[base] = true
			return base
		}
		argumentsName := fresh("controlArguments")
		fmt.Fprintf(&source, ") {\n        List<Variable> %s = List<Variable>.Empty();\n", argumentsName)
		if step.Control != nil {
			for argumentIndex, parameter := range step.Control.Parameters {
				value := parameter.Name
				switch parameter.Type {
				case "int":
					value = "string.FromInt(" + parameter.Name + ")"
				case "bool":
					value = fresh(fmt.Sprintf("encodedArgument%d", argumentIndex))
					fmt.Fprintf(&source, "        string %s = \"false\";\n        if (%s)\n            %s = \"true\";\n", value, parameter.Name, value)
				}
				fmt.Fprintf(&source, "        %s.Add(Variable(%s, %s));\n", argumentsName, strconv.Quote(parameter.Name), value)
			}
		}
		fmt.Fprintf(&source, "        return this.host.Run(%s, %s);\n    }\n}\n", strconv.Quote(step.ID), argumentsName)
	}
	source.WriteString("\npublic Class WorkflowSteps {\n")
	for index, step := range definition.Steps {
		fmt.Fprintf(&source, "    public WorkflowStep%d %s;\n", index, step.ID)
	}
	source.WriteString("\n    public WorkflowSteps(StepHost host) {\n")
	for index, step := range definition.Steps {
		fmt.Fprintf(&source, "        this.%s = new WorkflowStep%d(host);\n", step.ID, index)
	}
	source.WriteString("    }\n}\n\npublic model Class Control {\n    private Inputs inputs;\n    private WorkflowSteps steps;\n\n    public Control(Inputs inputs, StepHost host) {\n        this.inputs = inputs;\n        steps = new WorkflowSteps(host);\n    }\n\n    public Result<void,string> Run() {\n")
	bodyLine := bytes.Count([]byte(source.String()), []byte("\n")) + 1
	source.WriteString(definition.Control.Source)
	source.WriteString("\n        return Result<void,string>.Success();\n    }\n}\n")
	return source.String(), decoder.String(), bodyLine
}
