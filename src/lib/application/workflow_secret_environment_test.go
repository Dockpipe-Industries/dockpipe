package application

import (
	"context"
	"errors"
	"testing"

	"dockpipe/src/lib/domain"
	contract "dockpipe/src/lib/domain/secretenv"
	projectmodel "dockpipe/src/lib/model/project"
)

func TestSecretEnvironmentSelectionAndFailureAtomicity(t *testing.T) {
	previous := resolveSecretEnvironmentFn
	t.Cleanup(func() { resolveSecretEnvironmentFn = previous })
	mode := "environment"
	cfg := &domain.DockpipeProjectConfig{Secrets: domain.DockpipeSecretsConfig{
		Vault: &mode, Environment: "dev", Environments: map[string]projectmodel.SecretEnvironment{
			"dev":        {Resolver: "test", Parameters: map[string]string{"id": "dev-id"}, Bindings: map[string]string{"TOKEN": "dev-key"}},
			"production": {Resolver: "test", Parameters: map[string]string{"id": "prod-id"}, Bindings: map[string]string{"TOKEN": "prod-key"}},
		},
	}}
	called := 0
	resolveSecretEnvironmentFn = func(ctx context.Context, root, resolver string, request contract.Request) (map[string]string, error) {
		called++
		if request.Parameters["id"] != "prod-id" {
			t.Fatal("wrong environment selected")
		}
		return nil, errors.New("provider unavailable")
	}
	env := map[string]string{"TOKEN": "original"}
	opts := &CliOpts{SecretEnvironment: "production"}
	handled, err := mergeSecretEnvironment(env, opts, cfg, t.TempDir(), "", &domain.Workflow{})
	if !handled || err == nil || env["TOKEN"] != "original" || called != 1 {
		t.Fatal("failed load changed the environment")
	}
	opts.SecretEnvironment = "typo"
	_, err = mergeSecretEnvironment(env, opts, cfg, t.TempDir(), "", &domain.Workflow{})
	if err == nil || called != 1 {
		t.Fatal("unknown environment fell back")
	}
}

func TestSecretEnvironmentProjectDefaultOnlyLoadsForReferencedKeys(t *testing.T) {
	previous := resolveSecretEnvironmentFn
	t.Cleanup(func() { resolveSecretEnvironmentFn = previous })
	resolveSecretEnvironmentFn = func(context.Context, string, string, contract.Request) (map[string]string, error) {
		t.Fatal("unrelated workflow caused secret fetch")
		return nil, nil
	}
	mode := "environment"
	cfg := &domain.DockpipeProjectConfig{Secrets: domain.DockpipeSecretsConfig{
		Vault: &mode, Environment: "prod", Environments: map[string]projectmodel.SecretEnvironment{
			"prod": {Resolver: "test", Bindings: map[string]string{"TOKEN": "key"}},
		},
	}}
	handled, err := mergeSecretEnvironment(map[string]string{}, &CliOpts{}, cfg, t.TempDir(), "", &domain.Workflow{})
	if !handled || err != nil {
		t.Fatalf("unexpected result: %v", err)
	}
}

func TestSecretEnvironmentExplicitSelectionDoesNotLoadLegacyTemplate(t *testing.T) {
	previous := resolveSecretEnvironmentFn
	t.Cleanup(func() { resolveSecretEnvironmentFn = previous })
	mode := "op"
	cfg := &domain.DockpipeProjectConfig{Secrets: domain.DockpipeSecretsConfig{
		Vault: &mode, Environments: map[string]projectmodel.SecretEnvironment{
			"prod": {Resolver: "test", Bindings: map[string]string{"TOKEN": "key"}},
		},
	}}
	resolveSecretEnvironmentFn = func(context.Context, string, string, contract.Request) (map[string]string, error) {
		return map[string]string{"TOKEN": "new-value"}, nil
	}
	env := map[string]string{"UNRELATED": "preserved"}
	opts := &CliOpts{SecretEnvironment: "prod"}
	handled, err := mergeSecretEnvironment(env, opts, cfg, t.TempDir(), "", &domain.Workflow{})
	if !handled || err != nil || env["TOKEN"] != "new-value" || env["UNRELATED"] != "preserved" {
		t.Fatalf("explicit selection failed: %v", err)
	}
	_, err = mergeSecretEnvironment(env, opts, cfg, t.TempDir(), "", &domain.Workflow{Vault: " op "})
	if err == nil {
		t.Fatal("explicit workflow backend conflict was ignored")
	}
}

func TestSecretEnvironmentFlagsAndOptOut(t *testing.T) {
	_, opts, err := ParseFlags(t.TempDir(), []string{"--workflow", "example", "--secret-environment", "prod", "--no-vault"})
	if err != nil || opts.SecretEnvironment != "prod" || !opts.NoOpInject || opInjectWanted(opts) {
		t.Fatalf("secret environment flags not honored: %v", err)
	}
	t.Setenv("DOCKPIPE_OP_INJECT", "1")
	t.Setenv("DOCKPIPE_VAULT_INJECT", "0")
	if opInjectWanted(&CliOpts{}) {
		t.Fatal("generic vault opt-out ignored")
	}
}
