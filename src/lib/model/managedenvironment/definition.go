package managedenvironment

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
)

const Schema = "dockpipe.environment/v1"

var ErrKubernetesPodUnsupported = errors.New("environments backed by Kubernetes pods are unsupported")

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Definition struct {
	Schema    string     `yaml:"schema" json:"schema"`
	Name      string     `yaml:"name" json:"name"`
	Kind      string     `yaml:"kind" json:"kind"`
	Provision *Workflow  `yaml:"provision,omitempty" json:"provision,omitempty"`
	Storage   *Storage   `yaml:"storage,omitempty" json:"storage,omitempty"`
	Container *Container `yaml:"container,omitempty" json:"container,omitempty"`
	VM        *VM        `yaml:"vm,omitempty" json:"vm,omitempty"`
}

type Workflow struct {
	Workflow string `yaml:"workflow" json:"workflow"`
	Package  string `yaml:"package,omitempty" json:"package,omitempty"`
}

type Storage struct {
	Profile string `yaml:"profile" json:"profile"`
}

func ValidName(name string) bool {
	return identifier.MatchString(name) && filepath.IsLocal(name+".yaml")
}

func (definition Definition) Validate() error {
	if definition.Schema != Schema {
		return fmt.Errorf("environment schema must be %q", Schema)
	}
	if !ValidName(definition.Name) {
		return fmt.Errorf("invalid environment name %q", definition.Name)
	}
	switch definition.Kind {
	case "vm", "container":
	case "kubepod", "kube-pod", "kubernetes-pod":
		return ErrKubernetesPodUnsupported
	default:
		return fmt.Errorf("environment kind must be vm or container")
	}
	if definition.Container != nil {
		if definition.Kind != "container" || definition.Provision != nil || definition.Storage != nil || definition.VM != nil {
			return errors.New("container settings require a container environment without a workflow or VM storage profile")
		}
		return definition.Container.Validate()
	}
	if definition.VM != nil {
		if definition.Kind != "vm" || definition.Provision != nil || definition.Storage != nil {
			return errors.New("VM settings require a VM environment without a provisioning workflow or storage profile")
		}
		return definition.VM.Validate()
	}

	if definition.Provision == nil {
		return errors.New("choose a provisioning workflow, container settings, or VM settings")
	}
	if !identifier.MatchString(definition.Provision.Workflow) {
		return fmt.Errorf("invalid environment provisioning workflow %q", definition.Provision.Workflow)
	}
	if definition.Provision.Package != "" && !identifier.MatchString(definition.Provision.Package) {
		return fmt.Errorf("invalid environment provisioning package %q", definition.Provision.Package)
	}
	if definition.Storage != nil {
		if definition.Kind != "vm" {
			return fmt.Errorf("image storage profiles require a VM environment")
		}
		if !ValidName(definition.Storage.Profile) {
			return fmt.Errorf("invalid environment storage profile name")
		}
	}
	return nil
}
