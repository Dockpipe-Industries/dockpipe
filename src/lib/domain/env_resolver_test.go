package domain

import (
	"testing"
)

func TestEnvVarNamesFromHint(t *testing.T) {
	got := EnvVarNamesFromHint(" OPENAI_API_KEY , ANTHROPIC_API_KEY ")
	if len(got) != 2 || got[0] != "OPENAI_API_KEY" || got[1] != "ANTHROPIC_API_KEY" {
		t.Fatalf("unexpected: %#v", got)
	}
	if EnvVarNamesFromHint("") != nil {
		t.Fatalf("empty hint should return nil slice")
	}
}

// TestBranchPrefixForTemplate maps template/isolate names to random work-branch slug prefixes.
func TestBranchPrefixForTemplate(t *testing.T) {
	if got := BranchPrefixForTemplate("claude"); got != "claude" {
		t.Fatalf("claude prefix: %q", got)
	}
	if got := BranchPrefixForTemplate("agent-dev"); got != "claude" {
		t.Fatalf("agent-dev prefix: %q", got)
	}
	if got := BranchPrefixForTemplate("codex"); got != "codex" {
		t.Fatalf("codex prefix: %q", got)
	}
	if got := BranchPrefixForTemplate("something-else"); got != "dockpipe" {
		t.Fatalf("default prefix: %q", got)
	}
}

// TestFromResolverMap builds a Resolver from DOCKPIPE_RESOLVER_* env-style map entries.
func TestFromResolverMap(t *testing.T) {
	r := FromResolverMap(map[string]string{
		"DOCKPIPE_RESOLVER_TEMPLATE":     "codex",
		"DOCKPIPE_RESOLVER_PRE_SCRIPT":   "scripts/pre.sh",
		"DOCKPIPE_RESOLVER_ACTION":       "actions/do.sh",
		"DOCKPIPE_RESOLVER_CMD":          "codex",
		"DOCKPIPE_RESOLVER_ENV":          "OPENAI_API_KEY",
		"DOCKPIPE_RESOLVER_EXPERIMENTAL": "1",
	})
	if r.Template != "codex" || r.PreScript != "scripts/pre.sh" || r.Action != "actions/do.sh" {
		t.Fatalf("unexpected resolver assignments: %#v", r)
	}
	if r.Cmd != "codex" || r.EnvHint != "OPENAI_API_KEY" || !r.Experimental {
		t.Fatalf("unexpected extended fields: %#v", r)
	}
}
