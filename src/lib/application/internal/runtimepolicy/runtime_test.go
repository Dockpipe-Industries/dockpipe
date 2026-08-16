package runtimepolicy

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func TestCompiledRuntimePolicyLogLinesExplainAdvisoryAllowlist(t *testing.T) {
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "allowlist",
				Enforcement: "advisory",
				Allow:       []string{"api.openai.com", "*.anthropic.com", "api.github.com"},
			},
		},
	}
	lines := strings.Join(CompiledRuntimePolicyLogLines(rm), "\n")
	for _, want := range []string{
		"runtime policy: network=allowlist, allow=api.openai.com,*.anthropic.com,+1",
		"policy enforcement: network allowlist is advisory in this build; full egress filtering is not active yet",
		"policy coverage: domain allow/block rules are compiled for inspection but are not enforced natively by Docker",
	} {
		if !strings.Contains(lines, want) {
			t.Fatalf("expected log lines to contain %q, got:\n%s", want, lines)
		}
	}
}

func TestApplyCompiledRuntimePolicyInjectsProxyEnv(t *testing.T) {
	t.Setenv("DOCKPIPE_POLICY_PROXY_URL", "http://policy-proxy:8080")
	t.Setenv("DOCKPIPE_POLICY_PROXY_NO_PROXY", "metadata.local")
	runOpts := &infrastructure.RunOpts{
		ExtraEnv: []string{"BASE=1"},
	}
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "allowlist",
				Enforcement: "proxy",
				Allow:       []string{"api.openai.com", "*.anthropic.com"},
				Block:       []string{"*.facebook.com"},
			},
		},
	}
	if err := ApplyCompiledRuntimeManifest(runOpts, rm); err != nil {
		t.Fatalf("applyCompiledRuntimeManifest failed: %v", err)
	}
	em := domain.EnvSliceToMap(runOpts.ExtraEnv)
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		raw := em[key]
		if raw == "" {
			t.Fatalf("expected %s in proxy env, got %#v", key, em)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", key, err)
		}
		if u.Host != "policy-proxy:8080" || u.User == nil || u.User.Username() == "" {
			t.Fatalf("expected tokenized proxy URL for %s, got %q", key, raw)
		}
	}
	for key, want := range map[string]string{
		"DOCKPIPE_POLICY_PROXY_BASE_URL":      "http://policy-proxy:8080",
		"NO_PROXY":                            "metadata.local,localhost,127.0.0.1,::1",
		"no_proxy":                            "metadata.local,localhost,127.0.0.1,::1",
		"DOCKPIPE_POLICY_NETWORK_MODE":        "allowlist",
		"DOCKPIPE_POLICY_NETWORK_ENFORCEMENT": "proxy",
		"DOCKPIPE_POLICY_NETWORK_ALLOW":       "api.openai.com,*.anthropic.com",
		"DOCKPIPE_POLICY_NETWORK_BLOCK":       "*.facebook.com",
	} {
		if em[key] != want {
			t.Fatalf("expected %s=%q, got %#v", key, want, em)
		}
	}
}

func TestApplyCompiledRuntimePolicyProxyRequiresProxyURL(t *testing.T) {
	runOpts := &infrastructure.RunOpts{}
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "proxy",
			},
		},
	}
	if err := ApplyCompiledRuntimeManifest(runOpts, rm); err == nil || !strings.Contains(err.Error(), "DOCKPIPE_POLICY_PROXY_URL") {
		t.Fatalf("expected missing proxy URL error, got %v", err)
	}
}

func TestApplyCompiledRuntimePolicyUsesRunEnvProxyExport(t *testing.T) {
	runOpts := &infrastructure.RunOpts{
		ExtraEnv: []string{
			"DOCKPIPE_POLICY_PROXY_URL=http://proxy-sidecar:8080",
			"DOCKPIPE_POLICY_PROXY_NO_PROXY=metadata.local",
		},
	}
	rm := &domain.CompiledRuntimeManifest{
		Security: domain.CompiledSecurityPolicy{
			Network: domain.CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "proxy",
			},
		},
	}
	if err := ApplyCompiledRuntimeManifest(runOpts, rm); err != nil {
		t.Fatalf("applyCompiledRuntimeManifest failed: %v", err)
	}
	em := domain.EnvSliceToMap(runOpts.ExtraEnv)
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY"} {
		raw := em[key]
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s: %v", key, err)
		}
		if u.Host != "proxy-sidecar:8080" || u.User == nil || u.User.Username() == "" {
			t.Fatalf("expected tokenized proxy URL for %s, got %q", key, raw)
		}
	}
	for key, want := range map[string]string{
		"DOCKPIPE_POLICY_PROXY_BASE_URL":      "http://proxy-sidecar:8080",
		"NO_PROXY":                            "metadata.local,localhost,127.0.0.1,::1",
		"DOCKPIPE_POLICY_NETWORK_ENFORCEMENT": "proxy",
	} {
		if em[key] != want {
			t.Fatalf("expected %s=%q, got %#v", key, want, em)
		}
	}
}

func TestPolicyProxyURLWithTokenEncodesCompiledPolicy(t *testing.T) {
	raw := PolicyProxyURLWithToken("http://policy-proxy:8080", domain.CompiledNetworkPolicy{
		Mode:  "allowlist",
		Allow: []string{"api.openai.com", "*.anthropic.com"},
		Block: []string{"*.facebook.com"},
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse tokenized proxy url: %v", err)
	}
	if u.Host != "policy-proxy:8080" || u.User == nil {
		t.Fatalf("unexpected tokenized proxy url: %q", raw)
	}
	username := u.User.Username()
	decoded, err := base64.RawURLEncoding.DecodeString(username)
	if err != nil {
		t.Fatalf("decode token username: %v", err)
	}
	var payload struct {
		Version string   `json:"version"`
		Mode    string   `json:"mode"`
		Allow   []string `json:"allow"`
		Block   []string `json:"block"`
	}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("unmarshal token payload: %v", err)
	}
	if payload.Version != "dockpipe-proxy-v1" || payload.Mode != "allowlist" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if !strings.Contains(strings.Join(payload.Allow, ","), "api.openai.com") {
		t.Fatalf("unexpected allow payload: %+v", payload.Allow)
	}
	if !strings.Contains(strings.Join(payload.Block, ","), "*.facebook.com") {
		t.Fatalf("unexpected block payload: %+v", payload.Block)
	}
}
