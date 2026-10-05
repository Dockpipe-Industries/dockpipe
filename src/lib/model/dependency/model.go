// Package dependency owns the authored host-dependency wire shape shared by
// workflow and package manifests.
package dependency

// DependencySpec declares the host tools required by an authored workflow or package.
type DependencySpec struct {
	Host []HostDependency `yaml:"host,omitempty"`
}

// HostDependency declares one host tool and optional platform-specific installation hints.
type HostDependency struct {
	ID          string                    `yaml:"id,omitempty"`
	Command     string                    `yaml:"command,omitempty"`
	Description string                    `yaml:"description,omitempty"`
	Required    *bool                     `yaml:"required,omitempty"`
	Install     HostDependencyInstallHint `yaml:"install,omitempty"`
}

// HostDependencyInstallHint carries authored platform-specific installation instructions.
type HostDependencyInstallHint struct {
	Windows string `yaml:"windows,omitempty"`
	MacOS   string `yaml:"macos,omitempty"`
	Linux   string `yaml:"linux,omitempty"`
	Deb     string `yaml:"deb,omitempty"`
}
