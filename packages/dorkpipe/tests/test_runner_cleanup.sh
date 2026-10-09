#!/usr/bin/env bash
# Exercise the real suite runner with inert tools/tests, including interrupted runs.
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/dorkpipe-runner-cleanup.XXXXXX")"
trap 'rm -rf -- "$fixture"' EXIT
export RUNNER_FIXTURE_ROOT="$fixture/repo"
mkdir -p "$fixture/tools" "$fixture/homes" "$RUNNER_FIXTURE_ROOT/src/bin" \
	"$RUNNER_FIXTURE_ROOT/packages/dorkpipe/tests" \
	"$RUNNER_FIXTURE_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"

cat > "$fixture/tools/git" <<'SCRIPT'
#!/usr/bin/env bash
[[ "$*" == 'rev-parse --show-toplevel' ]] || exit 1
printf '%s\n' "$RUNNER_FIXTURE_ROOT"
SCRIPT
cat > "$fixture/tools/go" <<'SCRIPT'
#!/usr/bin/env bash
[[ "$*" == 'env GOMODCACHE' ]] || exit 1
printf '%s/modules\n' "$RUNNER_FIXTURE_ROOT"
SCRIPT
cat > "$RUNNER_FIXTURE_ROOT/src/bin/dockpipe" <<'SCRIPT'
#!/usr/bin/env bash
[[ "$1" == sdk ]] || exit 1
printf '%s\n' 'dockpipe_sdk() { printf "%s/build/%s\n" "$RUNNER_FIXTURE_ROOT" "$3"; }'
SCRIPT
chmod +x "$fixture/tools/git" "$fixture/tools/go" "$RUNNER_FIXTURE_ROOT/src/bin/dockpipe"

for test_script in "$DIR"/test_*.sh; do
	printf '#!/usr/bin/env bash\nexit 0\n' > "$RUNNER_FIXTURE_ROOT/packages/dorkpipe/tests/$(basename "$test_script")"
done
printf 'exit 0\n' > "$RUNNER_FIXTURE_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/skills-render.sh"
cat > "$RUNNER_FIXTURE_ROOT/packages/dorkpipe/tests/test_runner_cleanup.sh" <<'SCRIPT'
#!/usr/bin/env bash
printf '%s\n' "$HOME" > "$RUNNER_FIXTURE_ROOT/created-home"
mkdir -p "$HOME/.cache/nested"
printf 'disposable\n' > "$HOME/.cache/nested/payload"
chmod u-w "$HOME/.cache/nested"
ln -s "$RUNNER_FIXTURE_ROOT/protected-cache" "$HOME/shared-cache-link"
case "$RUNNER_TEST_OUTCOME" in
	success) exit 0 ;;
	failure) exit 1 ;;
	interrupt) kill -INT "$PPID" ;;
	terminate) kill -TERM "$PPID" ;;
esac
SCRIPT

printf 'preserve\n' > "$fixture/homes/unrelated"
mkdir -p "$RUNNER_FIXTURE_ROOT/protected-cache"
printf 'preserve\n' > "$RUNNER_FIXTURE_ROOT/protected-cache/payload"
for outcome in success failure interrupt terminate; do
	case "$outcome" in
		success) expected=0 ;;
		failure) expected=1 ;;
		interrupt) expected=130 ;;
		terminate) expected=143 ;;
	esac
	status=0
	PATH="$fixture/tools:$PATH" DORKPIPE_PACKAGE_TEST_TMPDIR="$fixture/homes" \
		RUNNER_TEST_OUTCOME="$outcome" bash "$DIR/run.sh" > "$fixture/$outcome.log" 2>&1 || status=$?
	if [[ "$status" -ne "$expected" ]]; then
		cat "$fixture/$outcome.log" >&2
		printf '%s: expected status %s, got %s\n' "$outcome" "$expected" "$status" >&2
		exit 1
	fi
	created_home="$(cat "$RUNNER_FIXTURE_ROOT/created-home")"
	[[ "$created_home" == "$fixture/homes/dorkpipe-package-test-home."* ]]
	[[ ! -e "$created_home" ]]
	[[ "$(cat "$fixture/homes/unrelated")" == preserve ]]
	[[ "$(cat "$RUNNER_FIXTURE_ROOT/protected-cache/payload")" == preserve ]]
done
echo 'runner cleanup: success, failure, SIGINT, and SIGTERM passed'
