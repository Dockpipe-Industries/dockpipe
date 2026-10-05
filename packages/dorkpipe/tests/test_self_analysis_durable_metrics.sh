#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/dorkpipe-self-analysis-metrics.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin" "$tmp/out" "$tmp/durable/learning"

cat > "$tmp/bin/dockpipe" <<EOF
#!/usr/bin/env bash
set -euo pipefail
case "\${1:-}" in
  get)
    [[ "\${2:-}" == script_dir ]]
    printf '%s\n' '$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts'
    ;;
  sdk)
    printf '%s\n' 'dockpipe_sdk() { return 0; }'
    ;;
  __state)
	[[ "\${2:-}" == package-runtime ]]
    printf '%s\n' '$tmp/out'
    ;;
  *)
    exit 1
    ;;
esac
EOF
cat > "$tmp/bin/orchestrate-helper" <<EOF
#!/usr/bin/env bash
set -euo pipefail
[[ "\${1:-}" == durable-metrics-path ]]
[[ "\${2:-}" == '$ROOT' ]]
printf '%s\n' '$tmp/durable/learning/metrics.jsonl'
EOF
chmod 755 "$tmp/bin/dockpipe" "$tmp/bin/orchestrate-helper"
printf '%s\n' '{"run":1}' '{"run":2}' > "$tmp/durable/learning/metrics.jsonl"

(
	cd "$ROOT"
	PATH="$tmp/bin:$PATH" DORKPIPE_ORCH_HELPER_BIN="$tmp/bin/orchestrate-helper" \
		bash "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/self-analysis-signals.sh"
)

cmp "$tmp/durable/learning/metrics.jsonl" "$tmp/out/signals_metrics_tail.txt"
echo "self-analysis durable metrics test OK"
