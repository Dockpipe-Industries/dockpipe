// Package packagemodel owns the authored package-manifest wire shape.
package packagemodel

import modeldependency "dockpipe/src/lib/model/dependency"

// PackageManifest is optional metadata for a DockPipe package (workflow slice, core slice, or umbrella package).
// Stored as package.yml next to the package contents. Schema may evolve; extra YAML keys are ignored by the parser.
// Rich fields support store discovery, authoring, and dependency hints (workflows vs resolver packs).
type PackageManifest struct {
	Schema      int    `yaml:"schema"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Author      string `yaml:"author"`
	Website     string `yaml:"website"`
	License     string `yaml:"license"`
	// Icon: optional package-owned artwork path for package/tooling surfaces. Relative paths resolve next to package.yml.
	Icon string `yaml:"icon,omitempty"`
	// Artwork: optional named artwork variants (e.g. icon, vscode, cursor-dev) relative to package.yml.
	Artwork map[string]string `yaml:"artwork,omitempty"`
	// Kind hints for tooling: workflow | resolver | core | assets | bundle | package (optional).
	// kind: package — umbrella metadata for a maintainer tree (e.g. dockpipe/agent) whose child resolvers
	// live under resolvers/; use includes_resolvers for optional resolver profile names (store installs stay per-resolver).
	Kind string `yaml:"kind,omitempty"`
	// Provider: optional platform / vendor id for filtering and store facets (e.g. cloudflare, aws, github).
	// Use a short stable label, not a URL — see docs/packages/package-model.md.
	Provider string `yaml:"provider,omitempty"`
	// Capability: dotted capability id this resolver package provides (e.g. cli.codex, app.vscode). See docs/concepts/capabilities.md.
	Capability string `yaml:"capability,omitempty"`
	// PrimitiveYAMLDeprecated is the deprecated YAML key "primitive" — merged into Capability after parse.
	PrimitiveYAMLDeprecated string `yaml:"primitive,omitempty"`
	// Namespace: optional author/org label (same rules as workflow namespace; optional).
	Namespace string `yaml:"namespace,omitempty"`
	// Tags and keywords: search / store facets (optional).
	Tags     []string `yaml:"tags,omitempty"`
	Keywords []string `yaml:"keywords,omitempty"`
	// MinDockpipeVersion is a semver constraint for the CLI/engine (optional).
	MinDockpipeVersion string `yaml:"min_dockpipe_version,omitempty"`
	// Repository is source URL (optional).
	Repository string `yaml:"repository,omitempty"`
	// Provides names capabilities for resolver-style packages (e.g. codex, claude-code).
	Provides []string `yaml:"provides,omitempty"`
	// RequiresCapabilities: for kind workflow — dotted capability ids this workflow expects (e.g. cli.codex).
	// See docs/concepts/capabilities.md. Complements requires_resolvers (profile names).
	RequiresCapabilities []string `yaml:"requires_capabilities,omitempty"`
	// RequiresPrimitivesYAMLDeprecated is the deprecated YAML key "requires_primitives" — merged into RequiresCapabilities after parse.
	RequiresPrimitivesYAMLDeprecated []string `yaml:"requires_primitives,omitempty"`
	// RequiresResolvers hints default or required resolver profile names for a workflow package (optional).
	RequiresResolvers []string `yaml:"requires_resolvers,omitempty"`
	// IncludesResolvers lists resolver profile names under resolvers/ for kind: package (umbrella metadata only; not a single tarball).
	IncludesResolvers []string `yaml:"includes_resolvers,omitempty"`
	// Depends lists other package names this package expects (optional).
	Depends []string `yaml:"depends,omitempty"`
	// Platforms declares supported host platforms for package-owned workflows/scripts.
	Platforms []string `yaml:"platforms,omitempty"`
	// Dependencies declares host tools expected by this package's workflows/scripts.
	Dependencies modeldependency.DependencySpec `yaml:"dependencies,omitempty"`
	// AllowClone: when true, dockpipe clone may copy this compiled package into an authoring tree (e.g. workflows/).
	// Omitted or false: clone is refused — use for commercial/binary-only drops where the publisher does not grant source export.
	AllowClone bool `yaml:"allow_clone,omitempty"`
	// Distribution is optional policy for humans and tooling: "source" (recoverable YAML/assets) or "binary" (no meaningful source in the artifact).
	// Binary releases should set allow_clone: false and ship only non-recoverable artifacts if reverse-engineering must be impractical.
	Distribution string `yaml:"distribution,omitempty"`
	// Image declares a package-owned runtime image reference.
	// Keep this to normal OCI/registry refs; compile resolves it into the image artifact manifest.
	Image PackageImageSpec `yaml:"image,omitempty"`
	// ScriptContract declares generic package-level script context that DockPipe-aware tooling may inject for package assets.
	// This is intentionally generic package/runtime context only, not package-specific tooling handles.
	ScriptContract PackageScriptContract `yaml:"script_contract,omitempty"`
	// PackageState declares package-owned compatibility migration for maintained mixed state.
	// Undeclared public package scopes are conservatively imported whole by the engine.
	PackageState PackageStateSpec `yaml:"package_state,omitempty"`
	// Build declares optional package-owned authoring-tree build behavior.
	// This is for source checkouts only; installed tarballs should already contain the artifacts they ship.
	Build PackageBuildSpec `yaml:"build,omitempty"`
	// Test declares optional package-owned test behavior for source checkouts and CI.
	// Keep this generic: package authors own the script, DockPipe only executes it.
	Test PackageTestSpec `yaml:"test,omitempty"`
}

type PackageImageSpec struct {
	Source     string `yaml:"source,omitempty"`
	Ref        string `yaml:"ref,omitempty"`
	PullPolicy string `yaml:"pull_policy,omitempty"`
}

type PackageScriptContract struct {
	Inject []string `yaml:"inject,omitempty"`
}

type PackageStateSpec struct {
	CompatibilityImport string   `yaml:"compatibility_import,omitempty"`
	OwnerIDs            []string `yaml:"owner_ids,omitempty"`
}

type PackageBuildSpec struct {
	Source *PackageSourceBuildSpec `yaml:"source,omitempty"`
}

type PackageSourceBuildSpec struct {
	Script string `yaml:"script,omitempty"`
}

type PackageTestSpec struct {
	Script string `yaml:"script,omitempty"`
}
