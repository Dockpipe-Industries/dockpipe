package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"dockpipe/src/lib/domain"
	contract "dockpipe/src/lib/domain/secretenv"
	secretio "dockpipe/src/lib/infrastructure/secretenv"
)

var resolveSecretEnvironmentFn = secretio.Resolve

func mergeSecretEnvironment(env map[string]string, opts *CliOpts, cfg *domain.DockpipeProjectConfig, projectRoot, wfConfig string, wf *domain.Workflow) (bool, error) {
	mode := strings.ToLower(domain.EffectiveVaultString(wf, cfg))
	name := ""
	if cfg != nil {
		name = cfg.Secrets.Environment
	}
	explicit := opts != nil && opts.SecretEnvironment != ""
	if explicit {
		name = opts.SecretEnvironment
	}
	if !explicit && mode != "environment" && (name == "" || mode != "") {
		return false, nil
	}
	if cfg == nil || name == "" {
		return true, errors.New("vault environment requires a configured secret environment")
	}
	workflowMode := ""
	if wf != nil {
		workflowMode = strings.TrimSpace(wf.Vault)
	}
	if explicit && workflowMode != "" && mode != "environment" {
		return true, errors.New("--secret-environment conflicts with the workflow vault mode")
	}
	selected, exists := cfg.Secrets.Environments[name]
	if !exists {
		return true, errors.New("selected secret environment is not configured; refusing fallback")
	}
	keys := make(map[string]struct{}, len(selected.Bindings))
	for key := range selected.Bindings {
		keys[key] = struct{}{}
	}
	strict := explicit || strings.EqualFold(workflowMode, "environment")
	if !strict && !workflowReferencesVaultKeys(wfConfig, wf, keys) {
		return true, nil
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	values, err := resolveSecretEnvironmentFn(ctx, projectRoot, selected.Resolver, contract.Request{
		Schema: contract.Schema, Parameters: selected.Parameters, Bindings: selected.Bindings,
	})
	if err != nil {
		return true, err
	}
	for key, value := range values {
		env[key] = value
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] vault: merged %d key(s) from secret environment %s\n", len(values), name)
	return true, nil
}
