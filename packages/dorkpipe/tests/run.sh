#!/usr/bin/env bash
# Self-contained shell tests for the dorkpipe maintainer package (resolver scripts).
# From repo root: dockpipe package test --only dorkpipe
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
DIR="$ROOT/packages/dorkpipe/tests"
export DOCKPIPE_CI_ARTIFACT_SCOPE="${DOCKPIPE_CI_ARTIFACT_SCOPE:-package:dorkpipe}"
export DOCKPIPE_TEST_DOCKPIPE_BIN="${DOCKPIPE_TEST_DOCKPIPE_BIN:-$ROOT/src/bin/dockpipe}"
eval "$("$ROOT/src/bin/dockpipe" sdk --workdir "$ROOT")"
export TMPDIR="${DORKPIPE_PACKAGE_TEST_TMPDIR:-$ROOT/bin/.dockpipe/tmp/package-tests}"
mkdir -p "$TMPDIR"
mkdir -p "$(dockpipe_sdk path build go-cache)" "$(dockpipe_sdk path build go-tmp)"
export GOCACHE="${GOCACHE:-$(dockpipe_sdk path build go-cache)}"
export GOTMPDIR="${GOTMPDIR:-$(dockpipe_sdk path build go-tmp)}"
# Preserve the admitted Go module/toolchain cache before isolating HOME. Without this, Go derives
# GOMODCACHE from the temporary test home and offline package tests lose already-cached modules.
export GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"
TEST_HOME="$(mktemp -d "$TMPDIR/dorkpipe-package-test-home.XXXXXX")"
cleanup_test_home() {
	local status=$?
	trap - EXIT
	# Go module caches can contain read-only directories inside the isolated home.
	# Do not follow symlinks into shared caches or the caller's real home.
	if ! find "$TEST_HOME" -type d -exec chmod u+w {} +; then
		printf 'Failed to make temporary test directories writable: %s\n' "$TEST_HOME" >&2
	fi
	if ! rm -rf -- "$TEST_HOME"; then
		printf 'Failed to remove temporary test home: %s\n' "$TEST_HOME" >&2
		if [[ "$status" -eq 0 ]]; then
			status=1
		fi
	fi
	exit "$status"
}
trap cleanup_test_home EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export HOME="$TEST_HOME"
export USERPROFILE="$TEST_HOME"
export XDG_CONFIG_HOME="$TEST_HOME/.config"
export DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING="${DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING:-never}"
failed=0
for f in test_runner_cleanup.sh test_cloud_usage_failure.sh test_normalize_ci_scans.sh test_user_insight_queue.sh test_durable_training_metrics.sh test_self_analysis_durable_metrics.sh test_repo_tools.sh test_disposable_package_runtime.sh test_build_source_operation_results.sh test_orchestration_approval_operation_results.sh test_orchestration_verify_status.sh test_orchestration_lanes.sh test_software_dev_workflow.sh test_backlog_remote_workflow.sh test_example_brain_contract.sh test_orchestration_optimize.sh test_orchestration_container_auth.sh test_dev_stack_gpu_policy.sh test_cas01_app_server.sh test_codex_cli_update.sh test_task_skill_lifecycle.sh; do
	echo "--- dorkpipe/tests/$f ---"
	if ! bash "$DIR/$f"; then
		echo "dorkpipe/tests/$f FAILED" >&2
		failed=1
	fi
done
echo "--- dorkpipe skills.render smoke ---"
if DOCKPIPE_ASSETS_DIR="$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets" \
	DOCKPIPE_WORKFLOW_NAME="skills.render.smoke" \
	DOCKPIPE_WORKFLOW_CONFIG="$ROOT/packages/dorkpipe/workflows/skills.render/config.yml" \
	DOCKPIPE_STEP_ID="render" \
	DOCKPIPE_ARGS_JSON='["--target","generic","--output","/tmp/dorkpipe-skills-render-test","--dry-run","--skills","dorkpipe-core-review,dorkpipe-objective-execution,dorkpipe-task-handoff"]' \
	bash "$ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts/skills-render.sh"; then
	echo "dorkpipe skills.render smoke OK"
else
	echo "dorkpipe skills.render smoke FAILED" >&2
	failed=1
fi
if [[ $failed -ne 0 ]]; then
	exit 1
fi
echo "dorkpipe/tests/run.sh OK"
