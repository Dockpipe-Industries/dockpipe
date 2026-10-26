# CLI candidate reproduction

The GitHub adapter invokes this Dockpipe build workflow and the sibling
`workflows/publish/cli-candidate` workflow. Builds use the pinned public module
inputs and produce five CLI archives plus their metadata. Publication retains
the exact source/tag, checksums, immutable assets and hosted build attestations.

Normal accepted `dev` pushes build and publish automatically. Path filters can
omit a metadata-only history correction because its source tree is unchanged.
Use the manual `CLI bootstrap candidate` action to reproduce that exact ref.
Manual runs build only by default. Explicit `publish_candidate: true` publishes
only when the selected ref is `dev`; staging, master and feature refs cannot
publish through this route. The publisher checks the request independently.

An identity correction cannot repair existing build attestations. Preserve the
old uploaded candidate as historical evidence and produce a separate candidate
whose tag, source metadata and hosted provenance agree. Never replace existing
assets or relax consumer provenance guards to admit a moved historical tag.

For normal pull-request merges, supply the intended author email explicitly
through `MergePullRequestInput.authorEmail`, bind `expectedHeadOid`, and read back
the actual merge author before adopting its produced candidate. A repository's
local Git configuration does not select GitHub's server-side merge identity.
