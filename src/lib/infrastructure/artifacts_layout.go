package infrastructure

// DefaultRepoArtifactsDir is the repo-relative directory for local publish outputs
// (package archives, checksums, manifests and release binaries when built in-tree).
// Workflow-owned source stays under workflows/; release notes and articles stay under release/.
// This output subdirectory is gitignored.
const DefaultRepoArtifactsDir = "release/artifacts"
