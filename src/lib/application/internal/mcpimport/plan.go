// Package mcpimport binds request authorization to the selected import and its
// compiled runtime. It has no default authorizer, host executor or CLI surface.
package mcpimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dockpipe/src/lib/application/internal/runtimepolicy"
	"dockpipe/src/lib/domain"
	mcpdomain "dockpipe/src/lib/domain/mcp"
	"dockpipe/src/lib/infrastructure"
	mcpmodel "dockpipe/src/lib/model/mcp"
)

// Selection is supplied by the project/runtime owner, never by upstream MCP
// metadata. The executor must reverify the installed receipt and payload before
// returning a lease; a claimed digest alone is not verification.
type Selection struct {
	Name           string
	Server         mcpmodel.Server
	Runtime        domain.CompiledRuntimeManifest
	ArtifactSHA256 string
	Workspace      string
}

type Plan struct {
	selection   Selection
	fingerprint string
	options     infrastructure.RunOpts
}

// Compile supports only an explicitly selected, digest-pinned dockerimage with
// offline networking and the existing secure container policy. Unsupported
// policies fail before any authorizer, credential resolver or executor runs.
func Compile(selection Selection) (*Plan, error) {
	if err := mcpdomain.Validate(&mcpmodel.Config{Servers: map[string]mcpmodel.Server{selection.Name: selection.Server}}, nil); err != nil {
		return nil, err
	}
	local := selection.Server.Local
	if selection.Server.Transport != "stdio" || local == nil || local.SecretEnvironment != "" {
		return nil, fmt.Errorf("MCP import seam supports credential-free stdio only")
	}
	if local.Runtime != "dockerimage" || selection.Runtime.RuntimeProfile != local.Runtime || !strings.HasPrefix(local.Platform, "linux/") {
		return nil, fmt.Errorf("MCP import requires the selected Linux dockerimage runtime; no host fallback")
	}
	if !digest(selection.ArtifactSHA256) {
		return nil, fmt.Errorf("MCP import requires an exact artifact SHA256")
	}
	root, err := filepath.EvalSymlinks(selection.Workspace)
	if err != nil || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("MCP import requires an existing canonical workspace")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("MCP import workspace is not a directory")
	}
	working, err := filepath.EvalSymlinks(local.WorkingDirectory)
	if err != nil {
		return nil, fmt.Errorf("MCP import working directory cannot be resolved")
	}
	relative, err := filepath.Rel(root, working)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("MCP import working directory escapes workspace")
	}
	selection.Workspace = root
	selection.Server.Local = &mcpmodel.Local{Executable: local.Executable, Arguments: append([]string(nil), local.Arguments...), WorkingDirectory: working, Runtime: local.Runtime, Platform: local.Platform}
	// Freeze authored slices/maps before approval so callers cannot mutate a plan
	// through aliased configuration memory after its fingerprint was calculated.
	encoded, err := json.Marshal(selection)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(encoded, &selection); err != nil {
		return nil, err
	}
	manifest := &selection.Runtime
	if err = domain.ValidateCompiledRuntimeManifest(manifest); err != nil {
		return nil, err
	}
	expected, err := domain.FingerprintJSON(manifest.Security)
	if err != nil || expected != manifest.PolicyFingerprint {
		return nil, fmt.Errorf("MCP runtime policy fingerprint is stale")
	}
	policy := manifest.Security
	if policy.Network.Mode != "offline" || policy.Network.Enforcement != "native" || policy.FS.Root != "readonly" || policy.FS.Writes != "workspace-only" || len(policy.FS.WritablePaths) != 0 || len(policy.FS.TempPaths) != 1 || policy.FS.TempPaths[0] != "/tmp" || policy.Process.User != "non-root" || !policy.Process.NoNewPrivileges || len(policy.Process.AddCaps) != 0 || len(policy.Process.DropCaps) != 1 || policy.Process.DropCaps[0] != "ALL" || policy.Process.PIDLimit <= 0 {
		return nil, fmt.Errorf("MCP runtime policy is outside the supported offline secure-container subset")
	}
	image := manifest.Image
	suffix := "@sha256:"
	index := strings.LastIndex(image.Ref, suffix)
	if image.Source != "registry" || image.Build != nil || image.AutoBuild != "" || image.PullPolicy != "never" || index <= 0 || !digest(image.Ref[index+len(suffix):]) || image.ExpectedDigest != "sha256:"+image.Ref[index+len(suffix):] {
		return nil, fmt.Errorf("MCP runtime requires an existing digest-pinned image with builds and pulls disabled")
	}
	options := infrastructure.RunOpts{Image: image.Ref, WorkdirHost: root}
	if err = runtimepolicy.ApplyCompiledRuntimeManifest(&options, manifest); err != nil {
		return nil, err
	}
	fingerprint, err := domain.FingerprintJSON(selection)
	if err != nil {
		return nil, err
	}
	return &Plan{selection: selection, fingerprint: fingerprint, options: options}, nil
}

// Request binds each distinct operation and its exact arguments to a frozen
// configuration/runtime/artifact/workspace plan. Tool metadata never grants access.
type Request struct {
	Server             string `json:"server"`
	Workspace          string `json:"workspace"`
	RuntimeProfile     string `json:"runtime_profile"`
	PolicySHA256       string `json:"policy_sha256"`
	ArtifactSHA256     string `json:"artifact_sha256"`
	PlanSHA256         string `json:"plan_sha256"`
	Operation          string `json:"operation"`
	Tool               string `json:"tool,omitempty"`
	ToolMetadataSHA256 string `json:"tool_metadata_sha256,omitempty"`
	ArgumentsSHA256    string `json:"arguments_sha256,omitempty"`
}

type Decision struct {
	RequestSHA256 string
	Principal     string
	ExpiresAt     time.Time
}

// Authorizer must be backed by a trusted identity/approval owner. CLI access,
// repository JSON and an upstream server are not authorizers. No grant is saved.
type Authorizer func(context.Context, Request) (Decision, error)

func (p *Plan) authorize(ctx context.Context, authorize Authorizer, operation, tool, metadata string, arguments json.RawMessage) error {
	if authorize == nil {
		return fmt.Errorf("MCP import has no trusted authorizer")
	}
	request := Request{Server: p.selection.Name, Workspace: p.selection.Workspace, RuntimeProfile: p.selection.Runtime.RuntimeProfile, PolicySHA256: p.selection.Runtime.PolicyFingerprint, ArtifactSHA256: p.selection.ArtifactSHA256, PlanSHA256: p.fingerprint, Operation: operation, Tool: tool, ToolMetadataSHA256: metadata}
	if operation == "call" {
		if tool == "" || !digest(metadata) || !json.Valid(arguments) {
			return fmt.Errorf("MCP call authorization requires exact tool metadata and JSON arguments")
		}
		sum := sha256.Sum256(arguments)
		request.ArgumentsSHA256 = hex.EncodeToString(sum[:])
	}
	expected, err := domain.FingerprintJSON(request)
	if err != nil {
		return err
	}
	decision, err := authorize(ctx, request)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if decision.RequestSHA256 != expected || strings.TrimSpace(decision.Principal) == "" || !time.Now().Before(decision.ExpiresAt) {
		return fmt.Errorf("MCP import authorization is absent, stale or expired")
	}
	return nil
}

func digest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
