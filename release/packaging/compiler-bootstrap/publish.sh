#!/usr/bin/env bash
set -euo pipefail

output="${1:?compiler bootstrap output directory}"
: "${GITHUB_REPOSITORY:?GitHub repository is required}"
: "${GITHUB_SHA:?Exact source commit is required}"
[[ "${GITHUB_REF:-}" == refs/heads/dev ]] || { echo 'Compiler bootstrap publication requires dev.' >&2; exit 1; }
manifest="$output/compiler-bootstrap.json"
tag="$(jq -r .release_tag "$manifest")"
version="$(jq -r .version "$manifest")"
source_sha="$(jq -r .source_sha "$manifest")"
archive="$(jq -r .archive.name "$manifest")"
[[ "$source_sha" == "$GITHUB_SHA" ]] || { echo 'Compiler source commit changed.' >&2; exit 1; }
(cd "$output" && sha256sum --check SHA256SUMS.txt)
current_dev="$(gh api "repos/$GITHUB_REPOSITORY/git/ref/heads/dev" --jq .object.sha)"
[[ "$current_dev" == "$source_sha" ]] || { echo 'A newer dev commit exists; refusing stale bootstrap publication.' >&2; exit 1; }

error_file="$(mktemp)"
trap 'rm -f "$error_file"' EXIT
if gh api "repos/$GITHUB_REPOSITORY/git/ref/tags/$tag" >/dev/null 2>"$error_file"; then
  echo 'Bootstrap tag already exists; begin a fresh complete run.' >&2
  exit 1
elif ! grep -q 'HTTP 404' "$error_file"; then
  echo 'Could not verify that the bootstrap tag is unpublished.' >&2
  exit 1
fi

gh release create "$tag" --repo "$GITHUB_REPOSITORY" --target "$source_sha" \
  --prerelease --latest=false --title "Compiler bootstrap $version (Linux amd64 only)" \
  --notes-file "$output/RELEASE_NOTES.md" \
  "$output/$archive" "$manifest" "$output/SHA256SUMS.txt"
