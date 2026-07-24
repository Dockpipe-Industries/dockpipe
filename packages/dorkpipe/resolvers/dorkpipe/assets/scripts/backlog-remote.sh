#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(dockpipe get script_dir)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/orchestrate-common.sh"
requested_root="${ROOT:-${DOCKPIPE_WORKDIR:-}}"
eval "$(dockpipe sdk)"
dockpipe_sdk init-script

ROOT="${requested_root:-$(dockpipe_sdk get workdir)}"
artifact_root="${DORKPIPE_BACKLOG_ARTIFACT_ROOT:-$(dockpipe scope artifacts backlog-remote)}"
step_id="${DOCKPIPE_STEP_ID:-}"
unit="backlog.${step_id:-unknown}"
started_ms="$(dorkpipe_orchestrate_now_ms)"
helper_bin="$(dorkpipe_orchestrate_helper_bin)"
completion_details=()

backlog_remote_fail() {
  local rc=$?
  dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "offline backlog ${step_id:-unknown} failed" \
    "artifact_root=$artifact_root"
  exit "$rc"
}
trap backlog_remote_fail ERR

if command -v cygpath >/dev/null 2>&1; then
  ROOT="$(cygpath -m "$ROOT")"
  artifact_root="$(cygpath -m "$artifact_root")"
fi

dorkpipe_orchestrate_operation_emit "$unit" start "" "artifact_root=$artifact_root"

case "$step_id" in
  inspect)
    MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-inspect \
      "$ROOT" \
      "${DORKPIPE_BACKLOG_TASK_INDEX:-docs/agents/task-index.yaml}" \
      "${DORKPIPE_BACKLOG_TASK_ID:-}" \
      "${DORKPIPE_BACKLOG_SLICE:-}" \
      "${DORKPIPE_BACKLOG_BASELINE:-}" \
      "$artifact_root"
    ;;
  compile)
    MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compile \
      "$ROOT" \
      "$artifact_root" \
      "${DORKPIPE_BACKLOG_ENVIRONMENT_REF:-}" \
      "${DORKPIPE_BACKLOG_BRANCH_REF:-}" \
      "${DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON:-[]}" \
      "${DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON:-[]}" \
      "${DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON:-[]}" \
      "${DORKPIPE_BACKLOG_VALIDATION_INPUTS_JSON:-[]}" \
      "${DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON:-[]}"
    ;;
  compatibility)
    fixture_root="${DORKPIPE_BACKLOG_COMPATIBILITY_FIXTURE:-${DOCKPIPE_ASSETS_DIR}/fixtures/backlog-remote-codex-cli}"
    if command -v cygpath >/dev/null 2>&1; then
      fixture_root="$(cygpath -m "$fixture_root")"
    fi
    MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compatibility-preflight "$artifact_root" "$fixture_root"
    completion_details=(
      "compatibility=unsupported"
      "reason=machine_readable_submission_receipt_not_documented"
      "live_submission=false"
    )
    ;;
  dispatch)
    fixture="${DORKPIPE_BACKLOG_DISPATCH_FIXTURE:-${DOCKPIPE_ASSETS_DIR}/fixtures/backlog-remote-dispatch.json}"
    if command -v cygpath >/dev/null 2>&1; then
      fixture="$(cygpath -m "$fixture")"
    fi
    MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-dispatch-fixture "$artifact_root" "$fixture"
    ;;
  completion_candidate)
    fixture="${DORKPIPE_BACKLOG_COMPLETION_FIXTURE:-}"
    if [[ -z "$fixture" ]]; then
      echo "DORKPIPE_BACKLOG_COMPLETION_FIXTURE is required for fixture-backed completion candidate ingestion" >&2
      exit 1
    fi
    if command -v cygpath >/dev/null 2>&1; then
      fixture="$(cygpath -m "$fixture")"
    fi
    trap - ERR
    set +e
    completion_error="$(MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-ingest-completion-candidate "$artifact_root" "$fixture" 2>&1)"
    completion_rc=$?
    set -e
    trap backlog_remote_fail ERR
    if (( completion_rc != 0 )); then
      printf '%s\n' "$completion_error" >&2
      completion_reason_code="${completion_error%%:*}"
      dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "$completion_error" \
        "artifact_root=$artifact_root" "reason_code=$completion_reason_code"
      trap - ERR
      exit "$completion_rc"
    fi
    completion_details=(
      "authoritative_state=completion_candidate"
      "ready_for_review=false"
      "terminal_claim_trusted=false"
    )
    ;;
  status)
    fixture="${DORKPIPE_BACKLOG_STATUS_FIXTURE:-}"
    if [[ -z "$fixture" ]]; then
      echo "DORKPIPE_BACKLOG_STATUS_FIXTURE is required for fixture-backed remote status retrieval" >&2
      exit 1
    fi
    if command -v cygpath >/dev/null 2>&1; then
      fixture="$(cygpath -m "$fixture")"
    fi
    trap - ERR
    set +e
    status_error="$(MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-status-fixture "$artifact_root" "$fixture" 2>&1)"
    status_rc=$?
    set -e
    trap backlog_remote_fail ERR
    if (( status_rc != 0 )); then
      printf '%s\n' "$status_error" >&2
      status_reason_code="${status_error%%:*}"
      dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "$status_error" \
        "artifact_root=$artifact_root" "reason_code=$status_reason_code"
      trap - ERR
      exit "$status_rc"
    fi
    completion_details=(
      "artifact=remote-status.json"
      "authoritative_state=completion_candidate"
      "ready_for_review=false"
      "status_evidence_trusted=false"
      "status_evidence_authoritative=false"
    )
    ;;
  diff)
    fixture="${DORKPIPE_BACKLOG_DIFF_FIXTURE:-}"
    if [[ -z "$fixture" ]]; then
      echo "DORKPIPE_BACKLOG_DIFF_FIXTURE is required for fixture-backed remote diff retrieval" >&2
      exit 1
    fi
    if command -v cygpath >/dev/null 2>&1; then
      fixture="$(cygpath -m "$fixture")"
    fi
    trap - ERR
    set +e
    diff_error="$(MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-diff-fixture "$artifact_root" "$fixture" 2>&1)"
    diff_rc=$?
    set -e
    trap backlog_remote_fail ERR
    if (( diff_rc != 0 )); then
      printf '%s\n' "$diff_error" >&2
      diff_reason_code="${diff_error%%:*}"
      dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "$diff_error" \
        "artifact_root=$artifact_root" "reason_code=$diff_reason_code"
      trap - ERR
      exit "$diff_rc"
    fi
    completion_details=(
      "artifact=remote-diff.json"
      "patch_artifact=remote-diff.patch"
      "authoritative_state=completion_candidate"
      "ready_for_review=false"
      "diff_evidence_trusted=false"
      "patch_treated_as_opaque=true"
    )
    ;;
  result)
    fixture="${DORKPIPE_BACKLOG_RESULT_FIXTURE:-}"
    if [[ -z "$fixture" ]]; then
      echo "DORKPIPE_BACKLOG_RESULT_FIXTURE is required for fixture-backed remote result retrieval" >&2
      exit 1
    fi
    if command -v cygpath >/dev/null 2>&1; then
      fixture="$(cygpath -m "$fixture")"
    fi
    trap - ERR
    set +e
    result_error="$(MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-result-fixture "$artifact_root" "$fixture" 2>&1)"
    result_rc=$?
    set -e
    trap backlog_remote_fail ERR
    if (( result_rc != 0 )); then
      printf '%s\n' "$result_error" >&2
      result_reason_code="${result_error%%:*}"
      dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "$result_error" \
        "artifact_root=$artifact_root" "reason_code=$result_reason_code"
      trap - ERR
      exit "$result_rc"
    fi
    completion_details=(
      "artifact=remote-result.json"
      "authoritative_state=completion_candidate"
      "ready_for_review=false"
      "result_evidence_opaque=true"
      "result_evidence_trusted=false"
      "result_evidence_authoritative=false"
    )
    ;;
  validation_receipt)
    fixture="${DORKPIPE_BACKLOG_VALIDATION_RECEIPT_FIXTURE:-}"
    if [[ -z "$fixture" ]]; then
      echo "DORKPIPE_BACKLOG_VALIDATION_RECEIPT_FIXTURE is required for fixture-backed validation receipt retrieval" >&2
      exit 1
    fi
    if command -v cygpath >/dev/null 2>&1; then
      fixture="$(cygpath -m "$fixture")"
    fi
    trap - ERR
    set +e
    receipt_error="$(MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-validation-receipt-fixture "$artifact_root" "$fixture" 2>&1)"
    receipt_rc=$?
    set -e
    trap backlog_remote_fail ERR
    if (( receipt_rc != 0 )); then
      printf '%s\n' "$receipt_error" >&2
      receipt_reason_code="${receipt_error%%:*}"
      dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "$receipt_error" \
        "artifact_root=$artifact_root" "reason_code=$receipt_reason_code"
      trap - ERR
      exit "$receipt_rc"
    fi
    completion_details=(
      "artifact=validation-receipt.json"
      "authoritative_state=completion_candidate"
      "ready_for_review=false"
      "receipt_evidence_opaque=true"
      "receipt_evidence_trusted=false"
      "receipt_evidence_authoritative=false"
      "validation_executed=false"
    )
    ;;
  patch_boundary)
    trap - ERR
    set +e
    boundary_error="$(MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-verify-patch-boundary "$artifact_root" 2>&1)"
    boundary_rc=$?
    set -e
    trap backlog_remote_fail ERR
    if (( boundary_rc != 0 )); then
      printf '%s\n' "$boundary_error" >&2
      boundary_reason_code="${boundary_error%%:*}"
      dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "$boundary_error" \
        "artifact_root=$artifact_root" "reason_code=$boundary_reason_code"
      trap - ERR
      exit "$boundary_rc"
    fi
    completion_details=(
      "artifact=patch-boundary.json"
      "authoritative_state=completion_candidate"
      "ready_for_review=false"
      "patch_structure_verified=true"
      "allowed_path_boundary_verified=true"
      "semantic_correctness_reviewed=false"
      "validation_executed=false"
      "patch_applied=false"
    )
    ;;
  patch_application)
    consumer_root="${DORKPIPE_BACKLOG_CONSUMER_ROOT:-$ROOT}"
    if command -v cygpath >/dev/null 2>&1; then
      consumer_root="$(cygpath -m "$consumer_root")"
    fi
    trap - ERR
    set +e
    application_error="$(MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-apply-patch-temporary "$consumer_root" "$artifact_root" 2>&1)"
    application_rc=$?
    set -e
    trap backlog_remote_fail ERR
    if (( application_rc != 0 )); then
      printf '%s\n' "$application_error" >&2
      application_reason_code="${application_error%%:*}"
      dorkpipe_orchestrate_operation_fail "$unit" "$started_ms" "$application_error" \
        "artifact_root=$artifact_root" "reason_code=$application_reason_code"
      trap - ERR
      exit "$application_rc"
    fi
    completion_details=(
      "artifact=patch-application.json"
      "authoritative_state=completion_candidate"
      "application_scope=temporary_copy_only"
      "mechanical_application_succeeded=true"
      "temporary_workspace_cleanup_succeeded=true"
      "consumer_checkout_mutated=false"
      "semantic_correctness_reviewed=false"
      "validation_executed=false"
      "ready_for_review=false"
    )
    ;;
  *)
    echo "unsupported backlog.remote workflow step: ${step_id:-<empty>}" >&2
    exit 1
    ;;
esac

duration_ms="$(dorkpipe_orchestrate_operation_duration_ms "$started_ms")"
dorkpipe_orchestrate_operation_emit "$unit" done "$duration_ms" "artifact_root=$artifact_root" "adapter=fixture_only" "${completion_details[@]}"
