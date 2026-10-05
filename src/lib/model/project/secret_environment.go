package projectmodel

// SecretEnvironment selects a resolver and explicitly maps remote names to process variables.
// Parameters and bindings contain identifiers or references, never resolved secret values.
type SecretEnvironment struct {
	Resolver   string            `json:"resolver"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Bindings   map[string]string `json:"bindings"`
}
