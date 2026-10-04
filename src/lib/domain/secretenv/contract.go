// Package secretenv defines the provider-neutral resolver exchange for secret environments.
package secretenv

import (
	"errors"
	"regexp"
	"strings"
)

const Schema = "dockpipe.secret-environment/v1"
const MaxBytes = 1 << 20

var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Request struct {
	Schema     string            `json:"schema"`
	Parameters map[string]string `json:"parameters"`
	Bindings   map[string]string `json:"bindings"`
}

type Response struct {
	Schema string            `json:"schema"`
	Values map[string]string `json:"values"`
}

func ValidateBindings(bindings map[string]string) error {
	if len(bindings) == 0 || len(bindings) > 256 {
		return errors.New("secret environment requires between 1 and 256 bindings")
	}
	for name, reference := range bindings {
		if !variableName.MatchString(name) || strings.TrimSpace(reference) == "" || strings.ContainsRune(reference, 0) {
			return errors.New("invalid secret environment binding")
		}
	}
	return nil
}

func ValidateResponse(request Request, response Response) error {
	if response.Schema != Schema || len(response.Values) != len(request.Bindings) {
		return errors.New("secret resolver returned an incomplete or unexpected environment")
	}
	for name := range request.Bindings {
		value, exists := response.Values[name]
		if !exists || strings.ContainsRune(value, 0) {
			return errors.New("secret resolver returned a missing or invalid value")
		}
	}
	return nil
}
