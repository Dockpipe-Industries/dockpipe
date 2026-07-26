#!/usr/bin/env bash
set -euo pipefail
trap 'rc=$?; echo "test_backlog_remote_workflow failed at line ${LINENO}: ${BASH_COMMAND}" >&2; exit "$rc"' ERR

REPO_ROOT="$(git rev-parse --show-toplevel)"
REAL_GIT="$(command -v git)"
# shellcheck source=packages/dorkpipe/tests/lib/test-tools.sh
source "$REPO_ROOT/packages/dorkpipe/tests/lib/test-tools.sh"
dorkpipe_test_require_go "test_backlog_remote_workflow"
dorkpipe_test_init_go_cache "$REPO_ROOT"

tmp="$(dorkpipe_test_mktemp_dir "$REPO_ROOT")"
consumer="$REPO_ROOT"
application_consumer="$tmp/application-consumer"
application_pristine="$tmp/application-pristine"
application_expected="$tmp/application-expected"
artifact_root="$tmp/artifacts"
second_root="$tmp/artifacts-second"
fixture_root="$REPO_ROOT/packages/dorkpipe/tests/fixtures/backlog.remote"
compatibility_fixture="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/fixtures/backlog-remote-codex-cli"
helper_bin="$tmp/orchestrate-helper"
invocation_log="$tmp/forbidden-invocations.log"
validation_input_file_count=0
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/fake-bin"
cp -R "$fixture_root/application-consumer" "$application_consumer"
while IFS= read -r validation_input; do
  validation_input="${validation_input#  \"}"
  validation_input="${validation_input%\",}"
  validation_input="${validation_input%\"}"
  if [[ -z "$validation_input" || "$validation_input" == "[" || "$validation_input" == "]" ]]; then
    continue
  fi
  validation_input_file_count=$((validation_input_file_count + 1))
  if [[ "$validation_input" == "packages/dorkpipe/README.md" ]]; then
    continue
  fi
  mkdir -p "$application_consumer/$(dirname "$validation_input")"
  cp "$REPO_ROOT/$validation_input" "$application_consumer/$validation_input"
done <"$fixture_root/validation-input-files.json"
cp -R "$application_consumer" "$application_pristine"
cp -R "$application_consumer" "$application_expected"
printf '%s\n' '# Fixture package' 'Untrusted remote fixture change.' >"$application_expected/packages/dorkpipe/README.md"
"$REAL_GIT" status --short --untracked-files=all >"$tmp/repo-status-before"
(
  cd "$REPO_ROOT/packages/dorkpipe/lib"
  go build -o "$helper_bin" ./cmd/orchestrate-helper
)

cat >"$tmp/fake-bin/forbidden-tool" <<'TOOL'
#!/usr/bin/env bash
printf '%s\n' "$(basename "$0") $*" >>"${DORKPIPE_BACKLOG_FORBIDDEN_LOG:?}"
exit 97
TOOL
chmod +x "$tmp/fake-bin/forbidden-tool"
for tool in codex curl docker git ssh; do
  cp "$tmp/fake-bin/forbidden-tool" "$tmp/fake-bin/$tool"
done

export PATH="$tmp/fake-bin:$REPO_ROOT/src/bin:$PATH"
export DORKPIPE_BACKLOG_FORBIDDEN_LOG="$invocation_log"
export DOCKPIPE_SCRIPT_DIR="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets/scripts"
export DOCKPIPE_ASSETS_DIR="$REPO_ROOT/packages/dorkpipe/resolvers/dorkpipe/assets"
export DOCKPIPE_WORKFLOW_CONFIG="$REPO_ROOT/packages/dorkpipe/workflows/backlog.remote/config.yml"
export DOCKPIPE_WORKFLOW_NAME="backlog.remote"
export DORKPIPE_ORCH_HELPER_BIN="$helper_bin"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$artifact_root"
export DORKPIPE_BACKLOG_TASK_INDEX="docs/agents/task-index.yaml"
export DORKPIPE_BACKLOG_TASK_ID="TASK-015"
export DORKPIPE_BACKLOG_SLICE="Implement only the bounded offline fixture dispatch slice."
export DORKPIPE_BACKLOG_BASELINE="0123456789abcdef0123456789abcdef01234567"
export DORKPIPE_BACKLOG_ENVIRONMENT_REF="fixture-environment"
export DORKPIPE_BACKLOG_BRANCH_REF="js/dev"
export DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON='["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]'
export DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON='["No live provider invocation","No apply, commit, push, or publication"]'
export DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON='["go test ./packages/dorkpipe/lib/orchestrationhelper"]'
export DORKPIPE_BACKLOG_VALIDATION_INPUTS_JSON="$(<"$fixture_root/validation-input-files.json")"
export DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON='["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]'
export DORKPIPE_BACKLOG_COMPATIBILITY_FIXTURE="$compatibility_fixture"
export DORKPIPE_BACKLOG_DISPATCH_FIXTURE="$fixture_root/dispatch.json"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$fixture_root/completion-candidate.json"
export DORKPIPE_BACKLOG_STATUS_FIXTURE="$fixture_root/remote-status.json"
export DORKPIPE_BACKLOG_DIFF_FIXTURE="$fixture_root/remote-diff.json"
export DORKPIPE_BACKLOG_RESULT_FIXTURE="$fixture_root/remote-result.json"
export DORKPIPE_BACKLOG_VALIDATION_RECEIPT_FIXTURE="$fixture_root/validation-receipt.json"
export DORKPIPE_BACKLOG_SEMANTIC_REVIEW_FIXTURE="$fixture_root/semantic-review-decision.json"
export DORKPIPE_BACKLOG_CHECKOUT_APPLICATION_FIXTURE="$fixture_root/checkout-application-approval.json"
export DORKPIPE_BACKLOG_CHECKPOINT_FIXTURE="$fixture_root/checkout-checkpoint-approval.json"
export DORKPIPE_BACKLOG_PUBLICATION_FIXTURE="$fixture_root/checkout-publication-approval.json"
export DORKPIPE_BACKLOG_CONSUMER_ROOT="$application_consumer"
export ROOT="$consumer"

for required_input in VERSION assets/entrypoint.sh embed.go go.mod go.sum src/core/package.yml \
  src/lib/infrastructure/schema/workflow.schema.json workflows/README.md; do
  grep -Fq "\"$required_input\"" "$fixture_root/validation-input-files.json"
done
while IFS= read -r required_input; do
  required_input="${required_input#./}"
  grep -Fq "\"$required_input\"" "$fixture_root/validation-input-files.json"
done < <(
  cd "$REPO_ROOT"
  find packages/dorkpipe/lib/orchestrationhelper -maxdepth 1 -type f -name '*.go' -print
  find src/lib/domain src/lib/infrastructure/packagebuild -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' -print
  find src/lib/infrastructure -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' -print
)

log="$tmp/workflow.err"
for step in inspect compile compatibility dispatch completion_candidate status diff result validation_receipt patch_boundary patch_application validation_execution semantic_review checkout_application; do
  export DOCKPIPE_STEP_ID="$step"
  if ! bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>>"$log"; then
    cat "$log" >&2
    exit 1
  fi
done

grep -Fq '  - id: checkout_checkpoint' "$DOCKPIPE_WORKFLOW_CONFIG"
grep -Fq '  - id: checkout_publication' "$DOCKPIPE_WORKFLOW_CONFIG"
grep -Fq 'checkpoint: manual' "$DOCKPIPE_WORKFLOW_CONFIG"
grep -Fq 'publish: none' "$DOCKPIPE_WORKFLOW_CONFIG"
grep -Fq 'backlog-request-checkpoint' "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh"
grep -Fq 'dockpipe session checkpoint' "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh"
grep -Fq 'backlog-request-publication' "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh"
grep -Fq 'dockpipe session publish' "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh"
if rg -n 'exec\.Command\("git"|runCommandString\([^\n]*"git"' \
  "$REPO_ROOT/packages/dorkpipe/lib/orchestrationhelper/backlogcheckoutcheckpoint.go"; then
  echo "package checkpoint helper invokes Git directly" >&2
  exit 1
fi
if rg -n 'exec\.Command\("git"|runCommandString\([^\n]*"git"' \
  "$REPO_ROOT/packages/dorkpipe/lib/orchestrationhelper/backlogcheckoutpublication.go"; then
  echo "package publication helper invokes Git directly" >&2
  exit 1
fi

for step in inspect compile compatibility dispatch completion_candidate status diff result validation_receipt patch_boundary patch_application validation_execution semantic_review checkout_application; do
  grep -Fq "unit=backlog.$step status=start" "$log"
  grep -Fq "unit=backlog.$step status=done" "$log"
done
grep -Fq "unit=backlog.compatibility status=done" "$log"
grep -Fq "compatibility=unsupported" "$log"
grep -Fq "reason=machine_readable_submission_receipt_not_documented" "$log"
grep -Fq "live_submission=false" "$log"
grep -Fq "unit=backlog.completion_candidate status=done" "$log"
grep -Fq "authoritative_state=completion_candidate" "$log"
grep -Fq "ready_for_review=false" "$log"
grep -Fq "terminal_claim_trusted=false" "$log"
grep -Fq "unit=backlog.status status=done" "$log"
grep -Fq "artifact=remote-status.json" "$log"
grep -Fq "status_evidence_trusted=false" "$log"
grep -Fq "status_evidence_authoritative=false" "$log"
grep -Fq "unit=backlog.diff status=done" "$log"
grep -Fq "artifact=remote-diff.json" "$log"
grep -Fq "patch_artifact=remote-diff.patch" "$log"
grep -Fq "diff_evidence_trusted=false" "$log"
grep -Fq "patch_treated_as_opaque=true" "$log"
grep -Fq "unit=backlog.result status=done" "$log"
grep -Fq "artifact=remote-result.json" "$log"
grep -Fq "result_evidence_opaque=true" "$log"
grep -Fq "result_evidence_trusted=false" "$log"
grep -Fq "result_evidence_authoritative=false" "$log"
grep -Fq "unit=backlog.validation_receipt status=done" "$log"
grep -Fq "artifact=validation-receipt.json" "$log"
grep -Fq "receipt_evidence_opaque=true" "$log"
grep -Fq "receipt_evidence_trusted=false" "$log"
grep -Fq "receipt_evidence_authoritative=false" "$log"
grep -Fq "validation_executed=false" "$log"
grep -Fq "unit=backlog.patch_boundary status=done" "$log"
grep -Fq "artifact=patch-boundary.json" "$log"
grep -Fq "patch_structure_verified=true" "$log"
grep -Fq "allowed_path_boundary_verified=true" "$log"
grep -Fq "semantic_correctness_reviewed=false" "$log"
grep -Fq "patch_applied=false" "$log"
grep -Fq "unit=backlog.patch_application status=done" "$log"
grep -Fq "artifact=patch-application.json" "$log"
grep -Fq "application_scope=temporary_copy_only" "$log"
grep -Fq "mechanical_application_succeeded=true" "$log"
grep -Fq "temporary_workspace_cleanup_succeeded=true" "$log"
grep -Fq "consumer_checkout_mutated=false" "$log"
grep -Fq "unit=backlog.validation_execution status=done" "$log"
grep -Fq "artifact=validation-execution.json" "$log"
grep -Fq "validation_success_authoritative=false" "$log"
grep -Fq "unit=backlog.semantic_review status=done" "$log"
grep -Fq "artifact=semantic-review-decision.json" "$log"
grep -Fq "readiness_artifact=ready-for-review.json" "$log"
grep -Fq "authoritative_state=ready_for_review" "$log"
grep -Fq "explicit_local_semantic_decision=approved" "$log"
grep -Fq "apply_authorized=false" "$log"
grep -Fq "commit_authorized=false" "$log"
grep -Fq "push_authorized=false" "$log"
grep -Fq "publication_authorized=false" "$log"
grep -Fq "unit=backlog.checkout_application status=done" "$log"
grep -Fq "approval_artifact=checkout-application-approval.json" "$log"
grep -Fq "artifact=checkout-application.json" "$log"
grep -Fq "authoritative_state=applied_for_review" "$log"
grep -Fq "explicit_local_checkout_decision=approved" "$log"
grep -Fq "consumer_checkout_mutated=true" "$log"
grep -Fq "consumer_postimages_verified=true" "$log"
grep -Fq "rollback_plan_prepared=true" "$log"
grep -Fq "cleanup_succeeded=true" "$log"
grep -Fq "checkpoint_authorized=false" "$log"
grep -Fq "sync_authorized=false" "$log"
grep -Fq "next_task_authorized=false" "$log"
for name in backlog-selection.json remote-request.md remote-request.json remote-adapter-compatibility.json remote-task.json completion-candidate.json remote-status.json remote-diff.json remote-diff.patch remote-result.json validation-receipt.json patch-boundary.json patch-application.json validation-execution.json semantic-review-decision.json ready-for-review.json checkout-application-approval.json checkout-application.json; do
  test -f "$artifact_root/$name"
done
grep -Fq '"status": "selected"' "$artifact_root/backlog-selection.json"
grep -Fq '"contract_version": "dorkpipe.remote-request/v2"' "$artifact_root/remote-request.json"
grep -Fq '"validation_input_files": [' "$artifact_root/remote-request.json"
grep -Fq '"semantics": "complete_list"' "$artifact_root/remote-request.json"
grep -Fq "\"file_count\": $validation_input_file_count" "$artifact_root/remote-request.json"
grep -Fq '"validation_inputs_fingerprint": "sha256:' "$artifact_root/validation-receipt.json"
grep -Fq '"validation_inputs_fingerprint": "sha256:' "$artifact_root/patch-boundary.json"
grep -Fq '"validation_inputs_fingerprint": "sha256:' "$artifact_root/patch-application.json"
grep -Fq '"adapter_mode": "fixture_only"' "$artifact_root/remote-request.json"
grep -Fq '"live_provider": false' "$artifact_root/remote-request.json"
grep -Fq '"contract_version": "dorkpipe.remote-adapter-compatibility/v1"' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"version": "codex-cli 0.144.1"' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"status": "unsupported"' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"machine_readable_documented": false' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"stable_opaque_task_id_recoverable": false' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"live_submission_enabled": false' "$artifact_root/remote-adapter-compatibility.json"
grep -Fq '"provider_invoked": false' "$artifact_root/remote-task.json"
grep -Fq '"remote_task_id": "remote_fixture_task_015"' "$artifact_root/remote-task.json"
grep -Fq '"compatibility_fingerprint": "sha256:' "$artifact_root/remote-task.json"
grep -Fq '"contract_version": "dorkpipe.remote-completion-candidate/v1"' "$artifact_root/completion-candidate.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/completion-candidate.json"
grep -Fq '"candidate_id": "completion_fixture_candidate_015"' "$artifact_root/completion-candidate.json"
grep -Fq '"replay_identity": "completion_fixture_replay_015"' "$artifact_root/completion-candidate.json"
grep -Fq '"terminal_claim_trusted": false' "$artifact_root/completion-candidate.json"
grep -Fq '"ready_for_review": false' "$artifact_root/completion-candidate.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/completion-candidate.json"; then
  echo "completion candidate unexpectedly enabled ready_for_review" >&2
  exit 1
fi
grep -Fq '"contract_version": "dorkpipe.remote-status/v1"' "$artifact_root/remote-status.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/remote-status.json"
grep -Fq '"observation_id": "status_fixture_observation_015"' "$artifact_root/remote-status.json"
grep -Fq '"candidate_id": "completion_fixture_candidate_015"' "$artifact_root/remote-status.json"
grep -Fq '"claimed_remote_status": "completed"' "$artifact_root/remote-status.json"
grep -Fq '"trusted": false' "$artifact_root/remote-status.json"
grep -Fq '"authoritative": false' "$artifact_root/remote-status.json"
grep -Fq '"ready_for_review": false' "$artifact_root/remote-status.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/remote-status.json"; then
  echo "remote status unexpectedly enabled ready_for_review" >&2
  exit 1
fi
grep -Fq '"contract_version": "dorkpipe.remote-diff/v1"' "$artifact_root/remote-diff.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/remote-diff.json"
grep -Fq '"observation_id": "diff_fixture_observation_015"' "$artifact_root/remote-diff.json"
grep -Fq '"observation_id": "status_fixture_observation_015"' "$artifact_root/remote-diff.json"
grep -Fq '"candidate_id": "completion_fixture_candidate_015"' "$artifact_root/remote-diff.json"
grep -Fq '"opaque": true' "$artifact_root/remote-diff.json"
grep -Fq '"trusted": false' "$artifact_root/remote-diff.json"
grep -Fq '"package_owned_metadata": true' "$artifact_root/remote-diff.json"
grep -Fq '"ready_for_review": false' "$artifact_root/remote-diff.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/remote-diff.json"; then
  echo "remote diff unexpectedly enabled ready_for_review" >&2
  exit 1
fi
cmp "$fixture_root/remote-diff.patch" "$artifact_root/remote-diff.patch"
grep -Fq '"contract_version": "dorkpipe.remote-result/v1"' "$artifact_root/remote-result.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/remote-result.json"
grep -Fq '"observation_id": "result_fixture_observation_015"' "$artifact_root/remote-result.json"
grep -Fq '"observation_id": "diff_fixture_observation_015"' "$artifact_root/remote-result.json"
grep -Fq '"patch_sha256": "sha256:4027895ace152e2d66d11143b9e7841adb68e8d625977b7c123508f221114b1b"' "$artifact_root/remote-result.json"
grep -Fq '"patch_bytes": 236' "$artifact_root/remote-result.json"
grep -Fq '"opaque_result": "fixture-owned opaque result evidence"' "$artifact_root/remote-result.json"
grep -Fq '"opaque": true' "$artifact_root/remote-result.json"
grep -Fq '"trusted": false' "$artifact_root/remote-result.json"
grep -Fq '"authoritative": false' "$artifact_root/remote-result.json"
grep -Fq '"interpreted": false' "$artifact_root/remote-result.json"
grep -Fq '"package_owned_metadata": true' "$artifact_root/remote-result.json"
grep -Fq '"fixture_contract": "dorkpipe.remote-result-observation-fixture/v1"' "$artifact_root/remote-result.json"
grep -Fq '"provider_response": false' "$artifact_root/remote-result.json"
grep -Fq '"callback": false' "$artifact_root/remote-result.json"
grep -Fq '"signed_receipt": false' "$artifact_root/remote-result.json"
grep -Fq '"ready_for_review": false' "$artifact_root/remote-result.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/remote-result.json"; then
  echo "remote result unexpectedly enabled ready_for_review" >&2
  exit 1
fi
grep -Fq '"contract_version": "dorkpipe.validation-receipt/v2"' "$artifact_root/validation-receipt.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/validation-receipt.json"
grep -Fq '"observation_id": "receipt_fixture_observation_015"' "$artifact_root/validation-receipt.json"
grep -Fq '"observation_id": "result_fixture_observation_015"' "$artifact_root/validation-receipt.json"
grep -Fq '"fingerprint": "sha256:258e1a860f1e6aaac790a6dc88495483c59375c0131ee31445a11cea3c3063d9"' "$artifact_root/validation-receipt.json"
grep -Fq '"patch_sha256": "sha256:4027895ace152e2d66d11143b9e7841adb68e8d625977b7c123508f221114b1b"' "$artifact_root/validation-receipt.json"
grep -Fq '"required_validation": [' "$artifact_root/validation-receipt.json"
grep -Fq '"go test ./packages/dorkpipe/lib/orchestrationhelper"' "$artifact_root/validation-receipt.json"
grep -Fq '"fingerprint": "sha256:1dc90fee068fa97e7f2fafae5ac63498e0ace0c0260e06dd759ea164761c9b0c"' "$artifact_root/validation-receipt.json"
grep -Fq '"compatibility_fingerprint": "sha256:33c40e342c40e967c9673c612f9f6ec3be1226d7e19f8d1b0e0e6fb485bf8309"' "$artifact_root/validation-receipt.json"
grep -Fq '"opaque_receipt": "fixture-owned opaque validation receipt evidence"' "$artifact_root/validation-receipt.json"
grep -Fq '"trusted": false' "$artifact_root/validation-receipt.json"
grep -Fq '"authoritative": false' "$artifact_root/validation-receipt.json"
grep -Fq '"validation_success_interpreted": false' "$artifact_root/validation-receipt.json"
grep -Fq '"executed": false' "$artifact_root/validation-receipt.json"
grep -Fq '"package_owned_metadata": true' "$artifact_root/validation-receipt.json"
grep -Fq '"provider_response": false' "$artifact_root/validation-receipt.json"
grep -Fq '"callback": false' "$artifact_root/validation-receipt.json"
grep -Fq '"signed_receipt": false' "$artifact_root/validation-receipt.json"
grep -Fq '"hidden_transcript": false' "$artifact_root/validation-receipt.json"
grep -Fq '"ready_for_review": false' "$artifact_root/validation-receipt.json"
grep -Fq '"validation_execution": false' "$artifact_root/validation-receipt.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/validation-receipt.json"; then
  echo "validation receipt unexpectedly enabled ready_for_review" >&2
  exit 1
fi
grep -Fq '"contract_version": "dorkpipe.patch-boundary/v2"' "$artifact_root/patch-boundary.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/patch-boundary.json"
grep -Fq '"patch_sha256": "sha256:4027895ace152e2d66d11143b9e7841adb68e8d625977b7c123508f221114b1b"' "$artifact_root/patch-boundary.json"
grep -Fq '"patch_bytes": 236' "$artifact_root/patch-boundary.json"
grep -Fq '"allowed_paths_fingerprint": "sha256:' "$artifact_root/patch-boundary.json"
grep -Fq '"matching_rule": "exact_or_true_descendant"' "$artifact_root/patch-boundary.json"
grep -Fq '"packages/dorkpipe/README.md"' "$artifact_root/patch-boundary.json"
grep -Fq '"patch_structure": "verified_ordinary_unified_text_modifications"' "$artifact_root/patch-boundary.json"
grep -Fq '"allowed_path_boundary": "verified_segment_aware_lexical_containment"' "$artifact_root/patch-boundary.json"
grep -Fq '"mechanical_only": true' "$artifact_root/patch-boundary.json"
grep -Fq '"semantic_correctness_reviewed": false' "$artifact_root/patch-boundary.json"
grep -Fq '"validation_executed": false' "$artifact_root/patch-boundary.json"
grep -Fq '"patch_applied": false' "$artifact_root/patch-boundary.json"
grep -Fq '"ready_for_review_emitted": false' "$artifact_root/patch-boundary.json"
grep -Fq '"ready_for_review": false' "$artifact_root/patch-boundary.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/patch-boundary.json"; then
  echo "patch boundary unexpectedly enabled ready_for_review" >&2
  exit 1
fi
grep -Fq '"contract_version": "dorkpipe.patch-application/v2"' "$artifact_root/patch-application.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/patch-application.json"
grep -Fq '"application_scope": "temporary_copy_only"' "$artifact_root/patch-application.json"
grep -Fq '"mechanical_application_succeeded": true' "$artifact_root/patch-application.json"
grep -Fq '"temporary_workspace_cleanup_succeeded": true' "$artifact_root/patch-application.json"
grep -Fq '"sha256": "sha256:3dc160ac1641f7a12a1ab717c64c90aba3b5da8a7123eba6fb303b5d7dfdcb0a"' "$artifact_root/patch-application.json"
grep -Fq '"baseline_commit_git_verified": false' "$artifact_root/patch-application.json"
grep -Fq '"consumer_checkout_mutated": false' "$artifact_root/patch-application.json"
grep -Fq '"ready_for_review": false' "$artifact_root/patch-application.json"
grep -Fq '"validation_executed": false' "$artifact_root/patch-application.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/patch-application.json"; then
  echo "patch application unexpectedly enabled ready_for_review" >&2
  exit 1
fi
grep -Fq '"contract_version": "dorkpipe.validation-execution/v1"' "$artifact_root/validation-execution.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/validation-execution.json"
grep -Fq '"authority": "exact_validation_inputs_plus_changed_path_overlay"' "$artifact_root/validation-execution.json"
if ! grep -Fq '"status": "passed"' "$artifact_root/validation-execution.json"; then
  cat "$artifact_root/validation-execution.json" >&2
  echo "checked fixture validation did not pass" >&2
  exit 1
fi
grep -Fq '"argv": [' "$artifact_root/validation-execution.json"
grep -Fq '"go"' "$artifact_root/validation-execution.json"
grep -Fq '"test"' "$artifact_root/validation-execution.json"
grep -Fq '"./packages/dorkpipe/lib/orchestrationhelper"' "$artifact_root/validation-execution.json"
grep -Fq '"exit_code": 0' "$artifact_root/validation-execution.json"
grep -Fq '"validation_executed": true' "$artifact_root/validation-execution.json"
grep -Fq '"validation_success_authoritative": false' "$artifact_root/validation-execution.json"
grep -Fq '"ready_for_review": false' "$artifact_root/validation-execution.json"
if grep -Fq '"ready_for_review": true' "$artifact_root/validation-execution.json"; then
  echo "validation execution unexpectedly enabled ready_for_review" >&2
  exit 1
fi
grep -Fq '"contract_version": "dorkpipe.semantic-review-decision/v1"' "$artifact_root/semantic-review-decision.json"
grep -Fq '"state": "completion_candidate"' "$artifact_root/semantic-review-decision.json"
grep -Fq '"value": "approved"' "$artifact_root/semantic-review-decision.json"
grep -Fq '"review_scope": "semantic_correctness_of_bound_candidate"' "$artifact_root/semantic-review-decision.json"
grep -Fq '"explicit_local_decision": true' "$artifact_root/semantic-review-decision.json"
grep -Fq '"validation_status": "passed"' "$artifact_root/semantic-review-decision.json"
grep -Fq '"contract_version": "dorkpipe.ready-for-review/v1"' "$artifact_root/ready-for-review.json"
grep -Fq '"state": "ready_for_review"' "$artifact_root/ready-for-review.json"
grep -Fq '"ready_for_review_recorded": true' "$artifact_root/ready-for-review.json"
for name in semantic-review-decision.json ready-for-review.json; do
  grep -Fq '"apply_to_checkout": false' "$artifact_root/$name"
  grep -Fq '"commit": false' "$artifact_root/$name"
  grep -Fq '"push": false' "$artifact_root/$name"
  grep -Fq '"publication": false' "$artifact_root/$name"
  grep -Fq '"start_another_backlog_item": false' "$artifact_root/$name"
done
grep -Fq '"contract_version": "dorkpipe.checkout-application-approval/v1"' "$artifact_root/checkout-application-approval.json"
grep -Fq '"application_scope": "accepted_changed_paths_exact_patch_once"' "$artifact_root/checkout-application-approval.json"
grep -Fq '"contract_version": "dorkpipe.checkout-application/v1"' "$artifact_root/checkout-application.json"
grep -Fq '"state": "applied_for_review"' "$artifact_root/checkout-application.json"
grep -Fq '"consumer_checkout_applied": true' "$artifact_root/checkout-application.json"
grep -Fq '"consumer_postimages_verified": true' "$artifact_root/checkout-application.json"
grep -Fq '"cleanup_succeeded": true' "$artifact_root/checkout-application.json"
for capability in apply_to_checkout commit push publication checkpoint sync start_another_backlog_item; do
  grep -Fq "\"$capability\": false" "$artifact_root/checkout-application.json"
done
test ! -e "$invocation_log"
"$REAL_GIT" status --short --untracked-files=all >"$tmp/repo-status-after"
cmp "$tmp/repo-status-before" "$tmp/repo-status-after"
diff -r "$application_expected" "$application_consumer"
rm -rf "$application_consumer"
cp -R "$application_pristine" "$application_consumer"

MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-inspect \
  "$consumer" docs/agents/task-index.yaml TASK-015 \
  "$DORKPIPE_BACKLOG_SLICE" "$DORKPIPE_BACKLOG_BASELINE" "$second_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compile \
  "$consumer" "$second_root" "$DORKPIPE_BACKLOG_ENVIRONMENT_REF" "$DORKPIPE_BACKLOG_BRANCH_REF" \
  "$DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON" "$DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON" \
  "$DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON" "$DORKPIPE_BACKLOG_VALIDATION_INPUTS_JSON" \
  "$DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compatibility-preflight "$second_root" "$compatibility_fixture"
test ! -e "$second_root/remote-task.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-dispatch-fixture "$second_root" "$DORKPIPE_BACKLOG_DISPATCH_FIXTURE"
for name in backlog-selection.json remote-request.md remote-request.json remote-adapter-compatibility.json remote-task.json; do
  if ! cmp "$artifact_root/$name" "$second_root/$name"; then
    diff -u "$artifact_root/$name" "$second_root/$name" >&2 || true
    exit 1
  fi
done

malformed_candidate_root="$tmp/malformed-candidate-artifacts"
tampered_dispatch_root="$tmp/tampered-dispatch-artifacts"
cp -R "$second_root" "$malformed_candidate_root"
cp -R "$second_root" "$tampered_dispatch_root"

malformed_root="$tmp/malformed-compatibility"
malformed_fixture="$tmp/malformed-fixture"
mkdir -p "$malformed_fixture"
printf '{}\n' >"$malformed_fixture/contract.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-inspect \
  "$consumer" docs/agents/task-index.yaml TASK-015 \
  "$DORKPIPE_BACKLOG_SLICE" "$DORKPIPE_BACKLOG_BASELINE" "$malformed_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-compile \
  "$consumer" "$malformed_root" "$DORKPIPE_BACKLOG_ENVIRONMENT_REF" "$DORKPIPE_BACKLOG_BRANCH_REF" \
  "$DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON" "$DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON" \
  "$DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON" "$DORKPIPE_BACKLOG_VALIDATION_INPUTS_JSON" \
  "$DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$malformed_root"
export DORKPIPE_BACKLOG_COMPATIBILITY_FIXTURE="$malformed_fixture"
export DOCKPIPE_STEP_ID="compatibility"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/malformed.err"; then
  echo "malformed compatibility contract unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'unit=backlog.compatibility status=start' "$tmp/malformed.err"
grep -Fq 'unit=backlog.compatibility status=fail' "$tmp/malformed.err"
grep -Fq '"status": "error"' "$malformed_root/remote-adapter-compatibility.json"
grep -Fq '"reason_code": "invalid_compatibility_fixture"' "$malformed_root/remote-adapter-compatibility.json"
test ! -e "$malformed_root/remote-task.json"
export DORKPIPE_BACKLOG_COMPATIBILITY_FIXTURE="$compatibility_fixture"

malformed_candidate_fixture="$tmp/malformed-completion-candidate.json"
printf '{"unexpected":true}\n' >"$malformed_candidate_fixture"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$malformed_candidate_root"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$malformed_candidate_fixture"
export DOCKPIPE_STEP_ID="completion_candidate"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/malformed-candidate.err"; then
  echo "malformed completion candidate unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'unit=backlog.completion_candidate status=start' "$tmp/malformed-candidate.err"
grep -Fq 'unit=backlog.completion_candidate status=fail' "$tmp/malformed-candidate.err"
if ! grep -Fq 'completion_candidate_fixture_malformed:' "$tmp/malformed-candidate.err"; then
  cat "$tmp/malformed-candidate.err" >&2
  exit 1
fi
grep -Fq 'reason_code=completion_candidate_fixture_malformed' "$tmp/malformed-candidate.err"
grep -Fq 'reason_code=completion_candidate_fixture_malformed' "$tmp/malformed-candidate.err"
test ! -e "$malformed_candidate_root/completion-candidate.json"

sed -i 's/remote_fixture_task_015/remote_fixture_task_tampered/' "$tampered_dispatch_root/remote-task.json"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$tampered_dispatch_root"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$fixture_root/completion-candidate.json"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/tampered-candidate.err"; then
  echo "tampered immutable dispatch unexpectedly ingested a completion candidate" >&2
  exit 1
fi
grep -Fq 'unit=backlog.completion_candidate status=start' "$tmp/tampered-candidate.err"
grep -Fq 'unit=backlog.completion_candidate status=fail' "$tmp/tampered-candidate.err"
grep -Fq 'completion_candidate_dispatch_invalid:' "$tmp/tampered-candidate.err"
grep -Fq 'reason_code=completion_candidate_dispatch_invalid' "$tmp/tampered-candidate.err"
grep -Fq 'reason_code=completion_candidate_dispatch_invalid' "$tmp/tampered-candidate.err"
test ! -e "$tampered_dispatch_root/completion-candidate.json"

rejected_root="$tmp/rejected"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$rejected_root"
export DORKPIPE_BACKLOG_TASK_ID="TASK-999"
export DOCKPIPE_STEP_ID="inspect"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/rejected.err"; then
  echo "unknown backlog task unexpectedly inspected" >&2
  exit 1
fi
grep -Fq 'unit=backlog.inspect status=start' "$tmp/rejected.err"
grep -Fq 'unit=backlog.inspect status=fail' "$tmp/rejected.err"
grep -Fq '"code": "unknown_task_id"' "$rejected_root/backlog-selection.json"
for name in remote-request.md remote-request.json remote-task.json; do
  test ! -e "$rejected_root/$name"
done

export DORKPIPE_BACKLOG_TASK_ID="TASK-015"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$artifact_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-followup "$artifact_root" >"$tmp/followup.json"
grep -Fq '"contract_version": "dorkpipe.remote-followup/v1"' "$tmp/followup.json"
grep -Fq '"remote_task_id": "remote_fixture_task_015"' "$tmp/followup.json"

MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-ingest-completion-candidate "$second_root" "$fixture_root/completion-candidate.json"
cmp "$artifact_root/completion-candidate.json" "$second_root/completion-candidate.json"

stale_status_root="$tmp/stale-status-artifacts"
mismatched_status_root="$tmp/mismatched-status-artifacts"
malformed_status_root="$tmp/malformed-status-artifacts"
tampered_status_root="$tmp/tampered-status-artifacts"
cp -R "$second_root" "$stale_status_root"
cp -R "$second_root" "$mismatched_status_root"
cp -R "$second_root" "$malformed_status_root"
cp -R "$second_root" "$tampered_status_root"

MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-status-fixture "$second_root" "$fixture_root/remote-status.json"
cmp "$artifact_root/remote-status.json" "$second_root/remote-status.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-diff-fixture "$second_root" "$fixture_root/remote-diff.json"
cmp "$artifact_root/remote-diff.json" "$second_root/remote-diff.json"
cmp "$artifact_root/remote-diff.patch" "$second_root/remote-diff.patch"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-result-fixture "$second_root" "$fixture_root/remote-result.json"
cmp "$artifact_root/remote-result.json" "$second_root/remote-result.json"

cp "$artifact_root/completion-candidate.json" "$tmp/accepted-completion-candidate.json"
cp "$artifact_root/remote-task.json" "$tmp/accepted-remote-task.json"
cp "$artifact_root/remote-status.json" "$tmp/accepted-remote-status.json"
cp "$artifact_root/remote-diff.json" "$tmp/accepted-remote-diff.json"
cp "$artifact_root/remote-diff.patch" "$tmp/accepted-remote-diff.patch"
cp "$artifact_root/remote-result.json" "$tmp/accepted-remote-result.json"
export DORKPIPE_BACKLOG_COMPLETION_FIXTURE="$fixture_root/completion-candidate.json"
export DOCKPIPE_STEP_ID="completion_candidate"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/duplicate-candidate.err"; then
  echo "duplicate completion candidate unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'unit=backlog.completion_candidate status=start' "$tmp/duplicate-candidate.err"
grep -Fq 'unit=backlog.completion_candidate status=fail' "$tmp/duplicate-candidate.err"
grep -Fq 'completion_candidate_duplicate:' "$tmp/duplicate-candidate.err"
grep -Fq 'reason_code=completion_candidate_duplicate' "$tmp/duplicate-candidate.err"
grep -Fq 'reason_code=completion_candidate_duplicate' "$tmp/duplicate-candidate.err"
cmp "$tmp/accepted-completion-candidate.json" "$artifact_root/completion-candidate.json"
cmp "$tmp/accepted-remote-task.json" "$artifact_root/remote-task.json"

run_status_rejection() {
  local root="$1"
  local fixture="$2"
  local code="$3"
  local output="$4"
  export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$root"
  export DORKPIPE_BACKLOG_STATUS_FIXTURE="$fixture"
  export DOCKPIPE_STEP_ID="status"
  if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$output"; then
    echo "$code status observation unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq 'unit=backlog.status status=start' "$output"
  grep -Fq 'unit=backlog.status status=fail' "$output"
  grep -Fq "$code:" "$output"
  grep -Fq "reason_code=$code" "$output"
}

run_status_rejection "$artifact_root" "$fixture_root/remote-status.json" remote_status_duplicate "$tmp/duplicate-status.err"
cmp "$tmp/accepted-remote-status.json" "$artifact_root/remote-status.json"
cmp "$tmp/accepted-completion-candidate.json" "$artifact_root/completion-candidate.json"
cmp "$tmp/accepted-remote-task.json" "$artifact_root/remote-task.json"
cmp "$tmp/accepted-remote-diff.json" "$artifact_root/remote-diff.json"
cmp "$tmp/accepted-remote-diff.patch" "$artifact_root/remote-diff.patch"

replay_status_fixture="$tmp/replay-status.json"
sed 's/status_fixture_observation_015/status_fixture_observation_016/' "$fixture_root/remote-status.json" >"$replay_status_fixture"
run_status_rejection "$artifact_root" "$replay_status_fixture" remote_status_replay "$tmp/replay-status.err"
cmp "$tmp/accepted-remote-status.json" "$artifact_root/remote-status.json"

stale_status_fixture="$tmp/stale-status.json"
sed 's/2026-07-19T00:02:00Z/2026-07-19T00:01:00Z/' "$fixture_root/remote-status.json" >"$stale_status_fixture"
run_status_rejection "$stale_status_root" "$stale_status_fixture" remote_status_stale "$tmp/stale-status.err"
test ! -e "$stale_status_root/remote-status.json"

mismatched_status_fixture="$tmp/mismatched-status.json"
sed 's/remote_fixture_task_015/remote_fixture_task_wrong/' "$fixture_root/remote-status.json" >"$mismatched_status_fixture"
run_status_rejection "$mismatched_status_root" "$mismatched_status_fixture" remote_status_binding_mismatch "$tmp/mismatched-status.err"
test ! -e "$mismatched_status_root/remote-status.json"

malformed_status_fixture="$tmp/malformed-status.json"
printf '{"unexpected":true}\n' >"$malformed_status_fixture"
run_status_rejection "$malformed_status_root" "$malformed_status_fixture" remote_status_fixture_malformed "$tmp/malformed-status.err"
test ! -e "$malformed_status_root/remote-status.json"

tampered_status_fixture="$tmp/tampered-status.json"
sed 's/"completed"/"ready_for_review"/' "$fixture_root/remote-status.json" >"$tampered_status_fixture"
run_status_rejection "$tampered_status_root" "$tampered_status_fixture" remote_status_claim_invalid "$tmp/tampered-status.err"
test ! -e "$tampered_status_root/remote-status.json"

for root in "$stale_status_root" "$mismatched_status_root" "$malformed_status_root" "$tampered_status_root"; do
  for name in ready-for-review.json remote-diff.json remote-diff.patch remote-result.json validation-receipt.json apply.json; do
    test ! -e "$root/$name"
  done
done
test ! -e "$invocation_log"

run_diff_rejection() {
  local root="$1"
  local fixture="$2"
  local code="$3"
  local output="$4"
  export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$root"
  export DORKPIPE_BACKLOG_DIFF_FIXTURE="$fixture"
  export DOCKPIPE_STEP_ID="diff"
  if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$output"; then
    echo "$code diff observation unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq 'unit=backlog.diff status=start' "$output"
  grep -Fq 'unit=backlog.diff status=fail' "$output"
  grep -Fq "$code:" "$output"
  grep -Fq "reason_code=$code" "$output"
}

run_diff_rejection "$artifact_root" "$fixture_root/remote-diff.json" remote_diff_duplicate "$tmp/duplicate-diff.err"
cmp "$tmp/accepted-remote-diff.json" "$artifact_root/remote-diff.json"
cmp "$tmp/accepted-remote-diff.patch" "$artifact_root/remote-diff.patch"
cmp "$tmp/accepted-remote-status.json" "$artifact_root/remote-status.json"
cmp "$tmp/accepted-completion-candidate.json" "$artifact_root/completion-candidate.json"
cmp "$tmp/accepted-remote-task.json" "$artifact_root/remote-task.json"

replay_diff_fixture="$tmp/replay-diff.json"
sed 's/diff_fixture_observation_015/diff_fixture_observation_016/' "$fixture_root/remote-diff.json" >"$replay_diff_fixture"
cp "$fixture_root/remote-diff.patch" "$tmp/remote-diff.patch"
run_diff_rejection "$artifact_root" "$replay_diff_fixture" remote_diff_replay "$tmp/replay-diff.err"
cmp "$tmp/accepted-remote-diff.json" "$artifact_root/remote-diff.json"
cmp "$tmp/accepted-remote-diff.patch" "$artifact_root/remote-diff.patch"

prepare_diff_rejection_root() {
  local root="$1"
  cp -R "$second_root" "$root"
  rm "$root/remote-diff.json" "$root/remote-diff.patch" "$root/remote-result.json"
}

stale_diff_root="$tmp/stale-diff-artifacts"
prepare_diff_rejection_root "$stale_diff_root"
stale_diff_fixture_root="$tmp/stale-diff-fixture"
mkdir -p "$stale_diff_fixture_root"
sed 's/2026-07-19T00:03:00Z/2026-07-19T00:02:00Z/' "$fixture_root/remote-diff.json" >"$stale_diff_fixture_root/remote-diff.json"
cp "$fixture_root/remote-diff.patch" "$stale_diff_fixture_root/remote-diff.patch"
run_diff_rejection "$stale_diff_root" "$stale_diff_fixture_root/remote-diff.json" remote_diff_stale "$tmp/stale-diff.err"

mismatched_diff_root="$tmp/mismatched-diff-artifacts"
prepare_diff_rejection_root "$mismatched_diff_root"
mismatched_diff_fixture_root="$tmp/mismatched-diff-fixture"
mkdir -p "$mismatched_diff_fixture_root"
sed 's/remote_fixture_task_015/remote_fixture_task_wrong/' "$fixture_root/remote-diff.json" >"$mismatched_diff_fixture_root/remote-diff.json"
cp "$fixture_root/remote-diff.patch" "$mismatched_diff_fixture_root/remote-diff.patch"
run_diff_rejection "$mismatched_diff_root" "$mismatched_diff_fixture_root/remote-diff.json" remote_diff_binding_mismatch "$tmp/mismatched-diff.err"

malformed_diff_root="$tmp/malformed-diff-artifacts"
prepare_diff_rejection_root "$malformed_diff_root"
malformed_diff_fixture_root="$tmp/malformed-diff-fixture"
mkdir -p "$malformed_diff_fixture_root"
printf '{"unexpected":true}\n' >"$malformed_diff_fixture_root/remote-diff.json"
cp "$fixture_root/remote-diff.patch" "$malformed_diff_fixture_root/remote-diff.patch"
run_diff_rejection "$malformed_diff_root" "$malformed_diff_fixture_root/remote-diff.json" remote_diff_fixture_malformed "$tmp/malformed-diff.err"

missing_diff_root="$tmp/missing-diff-artifacts"
prepare_diff_rejection_root "$missing_diff_root"
run_diff_rejection "$missing_diff_root" "$tmp/missing-remote-diff.json" remote_diff_fixture_missing "$tmp/missing-diff.err"

missing_patch_root="$tmp/missing-patch-artifacts"
prepare_diff_rejection_root "$missing_patch_root"
missing_patch_fixture_root="$tmp/missing-patch-fixture"
mkdir -p "$missing_patch_fixture_root"
cp "$fixture_root/remote-diff.json" "$missing_patch_fixture_root/remote-diff.json"
run_diff_rejection "$missing_patch_root" "$missing_patch_fixture_root/remote-diff.json" remote_diff_patch_missing "$tmp/missing-patch.err"

tampered_patch_root="$tmp/tampered-patch-artifacts"
prepare_diff_rejection_root "$tampered_patch_root"
tampered_patch_fixture_root="$tmp/tampered-patch-fixture"
mkdir -p "$tampered_patch_fixture_root"
cp "$fixture_root/remote-diff.json" "$tampered_patch_fixture_root/remote-diff.json"
cp "$fixture_root/remote-diff.patch" "$tampered_patch_fixture_root/remote-diff.patch"
printf 'tampered\n' >>"$tampered_patch_fixture_root/remote-diff.patch"
run_diff_rejection "$tampered_patch_root" "$tampered_patch_fixture_root/remote-diff.json" remote_diff_patch_tampered "$tmp/tampered-patch.err"

for root in "$stale_diff_root" "$mismatched_diff_root" "$malformed_diff_root" "$missing_diff_root" "$missing_patch_root" "$tampered_patch_root"; do
  for name in remote-diff.patch remote-diff.json ready-for-review.json remote-result.json validation-receipt.json apply.json; do
    test ! -e "$root/$name"
  done
  cmp "$tmp/accepted-remote-status.json" "$root/remote-status.json"
  cmp "$tmp/accepted-completion-candidate.json" "$root/completion-candidate.json"
  cmp "$tmp/accepted-remote-task.json" "$root/remote-task.json"
done
test ! -e "$invocation_log"

run_result_rejection() {
  local root="$1"
  local fixture="$2"
  local code="$3"
  local output="$4"
  export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$root"
  export DORKPIPE_BACKLOG_RESULT_FIXTURE="$fixture"
  export DOCKPIPE_STEP_ID="result"
  if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$output"; then
    echo "$code result observation unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq 'unit=backlog.result status=start' "$output"
  grep -Fq 'unit=backlog.result status=fail' "$output"
  grep -Fq "$code:" "$output"
  grep -Fq "reason_code=$code" "$output"
}

run_result_rejection "$artifact_root" "$fixture_root/remote-result.json" remote_result_duplicate "$tmp/duplicate-result.err"
cmp "$tmp/accepted-remote-result.json" "$artifact_root/remote-result.json"
cmp "$tmp/accepted-remote-diff.json" "$artifact_root/remote-diff.json"
cmp "$tmp/accepted-remote-diff.patch" "$artifact_root/remote-diff.patch"
cmp "$tmp/accepted-remote-status.json" "$artifact_root/remote-status.json"
cmp "$tmp/accepted-completion-candidate.json" "$artifact_root/completion-candidate.json"
cmp "$tmp/accepted-remote-task.json" "$artifact_root/remote-task.json"

replay_result_fixture="$tmp/replay-result.json"
sed 's/result_fixture_observation_015/result_fixture_observation_016/' "$fixture_root/remote-result.json" >"$replay_result_fixture"
run_result_rejection "$artifact_root" "$replay_result_fixture" remote_result_replay "$tmp/replay-result.err"
cmp "$tmp/accepted-remote-result.json" "$artifact_root/remote-result.json"

prepare_result_rejection_root() {
  local root="$1"
  cp -R "$second_root" "$root"
  rm "$root/remote-result.json"
}

stale_result_root="$tmp/stale-result-artifacts"
prepare_result_rejection_root "$stale_result_root"
stale_result_fixture="$tmp/stale-result.json"
sed 's/2026-07-19T00:04:00Z/2026-07-19T00:03:00Z/' "$fixture_root/remote-result.json" >"$stale_result_fixture"
run_result_rejection "$stale_result_root" "$stale_result_fixture" remote_result_stale "$tmp/stale-result.err"

mismatched_result_root="$tmp/mismatched-result-artifacts"
prepare_result_rejection_root "$mismatched_result_root"
mismatched_result_fixture="$tmp/mismatched-result.json"
sed 's/remote_fixture_task_015/remote_fixture_task_wrong/' "$fixture_root/remote-result.json" >"$mismatched_result_fixture"
run_result_rejection "$mismatched_result_root" "$mismatched_result_fixture" remote_result_binding_mismatch "$tmp/mismatched-result.err"

malformed_result_root="$tmp/malformed-result-artifacts"
prepare_result_rejection_root "$malformed_result_root"
malformed_result_fixture="$tmp/malformed-result.json"
printf '{"unexpected":true}\n' >"$malformed_result_fixture"
run_result_rejection "$malformed_result_root" "$malformed_result_fixture" remote_result_fixture_malformed "$tmp/malformed-result.err"

missing_result_root="$tmp/missing-result-artifacts"
prepare_result_rejection_root "$missing_result_root"
run_result_rejection "$missing_result_root" "$tmp/missing-remote-result.json" remote_result_fixture_missing "$tmp/missing-result.err"

tampered_result_patch_root="$tmp/tampered-result-patch-artifacts"
prepare_result_rejection_root "$tampered_result_patch_root"
printf 'tampered\n' >>"$tampered_result_patch_root/remote-diff.patch"
cp "$tampered_result_patch_root/remote-diff.patch" "$tmp/tampered-accepted-remote-diff.patch"
run_result_rejection "$tampered_result_patch_root" "$fixture_root/remote-result.json" remote_result_diff_invalid "$tmp/tampered-result-patch.err"
cmp "$tmp/tampered-accepted-remote-diff.patch" "$tampered_result_patch_root/remote-diff.patch"

for root in "$stale_result_root" "$mismatched_result_root" "$malformed_result_root" "$missing_result_root" "$tampered_result_patch_root"; do
  for name in remote-result.json ready-for-review.json validation-receipt.json apply.json; do
    test ! -e "$root/$name"
  done
  cmp "$tmp/accepted-remote-diff.json" "$root/remote-diff.json"
  cmp "$tmp/accepted-remote-status.json" "$root/remote-status.json"
  cmp "$tmp/accepted-completion-candidate.json" "$root/completion-candidate.json"
  cmp "$tmp/accepted-remote-task.json" "$root/remote-task.json"
done
test ! -e "$invocation_log"

MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-retrieve-validation-receipt-fixture "$second_root" "$fixture_root/validation-receipt.json"
cmp "$artifact_root/validation-receipt.json" "$second_root/validation-receipt.json"
second_boundary_root="$tmp/artifacts-second-boundary"
cp -R "$second_root" "$second_boundary_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-verify-patch-boundary "$second_boundary_root"
cmp "$artifact_root/patch-boundary.json" "$second_boundary_root/patch-boundary.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-verify-patch-boundary "$second_boundary_root"
cmp "$artifact_root/patch-boundary.json" "$second_boundary_root/patch-boundary.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-apply-patch-temporary "$application_consumer" "$second_boundary_root"
cmp "$artifact_root/patch-application.json" "$second_boundary_root/patch-application.json"
cp "$second_boundary_root/patch-application.json" "$tmp/accepted-patch-application.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-apply-patch-temporary "$application_consumer" "$second_boundary_root"
cmp "$tmp/accepted-patch-application.json" "$second_boundary_root/patch-application.json"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-execute-validation "$application_consumer" "$second_boundary_root"
cmp "$artifact_root/validation-execution.json" "$second_boundary_root/validation-execution.json"
cp "$second_boundary_root/validation-execution.json" "$tmp/accepted-validation-execution.json"
artifact_restart_root="$tmp/artifact-restart"
cp -R "$second_boundary_root" "$artifact_restart_root"
MSYS2_ARG_CONV_EXCL='*' "$helper_bin" backlog-execute-validation "$tmp/missing-consumer" "$artifact_restart_root"
cmp "$tmp/accepted-validation-execution.json" "$artifact_restart_root/validation-execution.json"
diff -r "$application_pristine" "$application_consumer"
cp "$artifact_root/validation-receipt.json" "$tmp/accepted-validation-receipt.json"

run_receipt_rejection() {
  local root="$1"
  local fixture="$2"
  local code="$3"
  local output="$4"
  export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$root"
  export DORKPIPE_BACKLOG_VALIDATION_RECEIPT_FIXTURE="$fixture"
  export DOCKPIPE_STEP_ID="validation_receipt"
  if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$output"; then
    echo "$code validation receipt observation unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq 'unit=backlog.validation_receipt status=start' "$output"
  grep -Fq 'unit=backlog.validation_receipt status=fail' "$output"
  grep -Fq "$code:" "$output"
  grep -Fq "reason_code=$code" "$output"
}

run_receipt_rejection "$artifact_root" "$fixture_root/validation-receipt.json" validation_receipt_duplicate "$tmp/duplicate-receipt.err"
cmp "$tmp/accepted-validation-receipt.json" "$artifact_root/validation-receipt.json"
cmp "$tmp/accepted-remote-result.json" "$artifact_root/remote-result.json"
cmp "$tmp/accepted-remote-diff.json" "$artifact_root/remote-diff.json"
cmp "$tmp/accepted-remote-diff.patch" "$artifact_root/remote-diff.patch"
cmp "$tmp/accepted-remote-status.json" "$artifact_root/remote-status.json"
cmp "$tmp/accepted-completion-candidate.json" "$artifact_root/completion-candidate.json"
cmp "$tmp/accepted-remote-task.json" "$artifact_root/remote-task.json"

replay_receipt_fixture="$tmp/replay-validation-receipt.json"
sed 's/receipt_fixture_observation_015/receipt_fixture_observation_016/' "$fixture_root/validation-receipt.json" >"$replay_receipt_fixture"
run_receipt_rejection "$artifact_root" "$replay_receipt_fixture" validation_receipt_replay "$tmp/replay-receipt.err"
cmp "$tmp/accepted-validation-receipt.json" "$artifact_root/validation-receipt.json"

prepare_receipt_rejection_root() {
  local root="$1"
  cp -R "$second_root" "$root"
  rm "$root/validation-receipt.json"
}

stale_receipt_root="$tmp/stale-receipt-artifacts"
prepare_receipt_rejection_root "$stale_receipt_root"
stale_receipt_fixture="$tmp/stale-validation-receipt.json"
sed 's/2026-07-19T00:05:00Z/2026-07-19T00:04:00Z/' "$fixture_root/validation-receipt.json" >"$stale_receipt_fixture"
run_receipt_rejection "$stale_receipt_root" "$stale_receipt_fixture" validation_receipt_stale "$tmp/stale-receipt.err"

mismatched_receipt_root="$tmp/mismatched-receipt-artifacts"
prepare_receipt_rejection_root "$mismatched_receipt_root"
mismatched_receipt_fixture="$tmp/mismatched-validation-receipt.json"
sed 's/remote_fixture_task_015/remote_fixture_task_wrong/' "$fixture_root/validation-receipt.json" >"$mismatched_receipt_fixture"
run_receipt_rejection "$mismatched_receipt_root" "$mismatched_receipt_fixture" validation_receipt_binding_mismatch "$tmp/mismatched-receipt.err"

malformed_receipt_root="$tmp/malformed-receipt-artifacts"
prepare_receipt_rejection_root "$malformed_receipt_root"
malformed_receipt_fixture="$tmp/malformed-validation-receipt.json"
printf '{"unexpected":true}\n' >"$malformed_receipt_fixture"
run_receipt_rejection "$malformed_receipt_root" "$malformed_receipt_fixture" validation_receipt_fixture_malformed "$tmp/malformed-receipt.err"

missing_receipt_root="$tmp/missing-receipt-artifacts"
prepare_receipt_rejection_root "$missing_receipt_root"
run_receipt_rejection "$missing_receipt_root" "$tmp/missing-validation-receipt.json" validation_receipt_fixture_missing "$tmp/missing-receipt.err"

tampered_receipt_patch_root="$tmp/tampered-receipt-patch-artifacts"
prepare_receipt_rejection_root "$tampered_receipt_patch_root"
printf 'tampered\n' >>"$tampered_receipt_patch_root/remote-diff.patch"
cp "$tampered_receipt_patch_root/remote-diff.patch" "$tmp/tampered-receipt-accepted-remote-diff.patch"
run_receipt_rejection "$tampered_receipt_patch_root" "$fixture_root/validation-receipt.json" validation_receipt_diff_invalid "$tmp/tampered-receipt-patch.err"
cmp "$tmp/tampered-receipt-accepted-remote-diff.patch" "$tampered_receipt_patch_root/remote-diff.patch"

for root in "$stale_receipt_root" "$mismatched_receipt_root" "$malformed_receipt_root" "$missing_receipt_root" "$tampered_receipt_patch_root"; do
  for name in validation-receipt.json ready-for-review.json validation-execution.json apply.json; do
    test ! -e "$root/$name"
  done
  cmp "$tmp/accepted-remote-result.json" "$root/remote-result.json"
  cmp "$tmp/accepted-remote-diff.json" "$root/remote-diff.json"
  cmp "$tmp/accepted-remote-status.json" "$root/remote-status.json"
  cmp "$tmp/accepted-completion-candidate.json" "$root/completion-candidate.json"
  cmp "$tmp/accepted-remote-task.json" "$root/remote-task.json"
done
test ! -e "$invocation_log"

tampered_boundary_patch_root="$tmp/tampered-boundary-patch-artifacts"
cp -R "$second_boundary_root" "$tampered_boundary_patch_root"
rm "$tampered_boundary_patch_root/validation-execution.json"
rm "$tampered_boundary_patch_root/patch-boundary.json"
printf 'tampered\n' >>"$tampered_boundary_patch_root/remote-diff.patch"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$tampered_boundary_patch_root"
export DOCKPIPE_STEP_ID="patch_boundary"
if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/tampered-boundary-patch.err"; then
  echo "tampered accepted patch unexpectedly passed patch-boundary verification" >&2
  exit 1
fi
grep -Fq 'unit=backlog.patch_boundary status=start' "$tmp/tampered-boundary-patch.err"
grep -Fq 'unit=backlog.patch_boundary status=fail' "$tmp/tampered-boundary-patch.err"
grep -Fq 'patch_boundary_diff_invalid:' "$tmp/tampered-boundary-patch.err"
grep -Fq 'reason_code=patch_boundary_diff_invalid' "$tmp/tampered-boundary-patch.err"
test ! -e "$tampered_boundary_patch_root/patch-boundary.json"
for name in ready-for-review.json validation-execution.json apply.json; do
  test ! -e "$tampered_boundary_patch_root/$name"
done
test ! -e "$invocation_log"

cat >"$tmp/fake-boundary-helper" <<'HELPER'
#!/usr/bin/env bash
printf '%s: deterministic fixture rejection\n' "${DORKPIPE_BACKLOG_FAKE_BOUNDARY_ERROR:?}" >&2
exit 1
HELPER
chmod +x "$tmp/fake-boundary-helper"
real_helper_bin="$DORKPIPE_ORCH_HELPER_BIN"
export DORKPIPE_ORCH_HELPER_BIN="$tmp/fake-boundary-helper"
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$second_root"
export DOCKPIPE_STEP_ID="patch_boundary"
for code in patch_boundary_patch_unsupported patch_boundary_path_out_of_scope; do
  export DORKPIPE_BACKLOG_FAKE_BOUNDARY_ERROR="$code"
  if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$tmp/$code.err"; then
    echo "$code unexpectedly passed patch-boundary operation-result proof" >&2
    exit 1
  fi
  grep -Fq 'unit=backlog.patch_boundary status=start' "$tmp/$code.err"
  grep -Fq 'unit=backlog.patch_boundary status=fail' "$tmp/$code.err"
  grep -Fq "$code: deterministic fixture rejection" "$tmp/$code.err"
  grep -Fq "reason_code=$code" "$tmp/$code.err"
done
export DORKPIPE_ORCH_HELPER_BIN="$real_helper_bin"
unset DORKPIPE_BACKLOG_FAKE_BOUNDARY_ERROR

run_application_rejection() {
  local root="$1"
  local source_root="$2"
  local code="$3"
  local output="$4"
  export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$root"
  export DORKPIPE_BACKLOG_CONSUMER_ROOT="$source_root"
  export DOCKPIPE_STEP_ID="patch_application"
  if bash "$DOCKPIPE_SCRIPT_DIR/backlog-remote.sh" 2>"$output"; then
    echo "$code patch application unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq 'unit=backlog.patch_application status=start' "$output"
  grep -Fq 'unit=backlog.patch_application status=fail' "$output"
  grep -Fq "$code:" "$output"
  grep -Fq "reason_code=$code" "$output"
}

mismatched_application_root="$tmp/mismatched-application-artifacts"
cp -R "$second_boundary_root" "$mismatched_application_root"
rm "$mismatched_application_root/validation-execution.json"
rm "$mismatched_application_root/patch-application.json"
mismatched_application_consumer="$tmp/mismatched-application-consumer"
mkdir -p "$mismatched_application_consumer/packages/dorkpipe"
printf '# Wrong fixture package\n' >"$mismatched_application_consumer/packages/dorkpipe/README.md"
run_application_rejection "$mismatched_application_root" "$mismatched_application_consumer" patch_application_preimage_mismatch "$tmp/mismatched-application.err"
test ! -e "$mismatched_application_root/patch-application.json"

tampered_application_root="$tmp/tampered-application-artifacts"
cp -R "$second_boundary_root" "$tampered_application_root"
rm "$tampered_application_root/validation-execution.json"
rm "$tampered_application_root/patch-application.json"
printf 'tampered\n' >>"$tampered_application_root/remote-diff.patch"
run_application_rejection "$tampered_application_root" "$application_consumer" patch_application_boundary_invalid "$tmp/tampered-application.err"
test ! -e "$tampered_application_root/patch-application.json"

tampered_application_receipt_root="$tmp/tampered-application-receipt-artifacts"
cp -R "$second_boundary_root" "$tampered_application_receipt_root"
rm "$tampered_application_receipt_root/validation-execution.json"
sed 's/"completion_candidate"/"ready_for_review"/' "$tampered_application_receipt_root/patch-application.json" >"$tmp/tampered-patch-application.json"
mv "$tmp/tampered-patch-application.json" "$tampered_application_receipt_root/patch-application.json"
cp "$tampered_application_receipt_root/patch-application.json" "$tmp/preserved-tampered-patch-application.json"
run_application_rejection "$tampered_application_receipt_root" "$application_consumer" patch_application_artifact_invalid "$tmp/tampered-application-receipt.err"
cmp "$tmp/preserved-tampered-patch-application.json" "$tampered_application_receipt_root/patch-application.json"

cat >"$tmp/fake-application-helper" <<'HELPER'
#!/usr/bin/env bash
printf '%s: deterministic fixture rejection\n' "${DORKPIPE_BACKLOG_FAKE_APPLICATION_ERROR:?}" >&2
exit 1
HELPER
chmod +x "$tmp/fake-application-helper"
export DORKPIPE_ORCH_HELPER_BIN="$tmp/fake-application-helper"
for code in patch_application_patch_unsupported patch_application_boundary_invalid patch_application_artifact_invalid patch_application_cleanup_failed; do
  export DORKPIPE_BACKLOG_FAKE_APPLICATION_ERROR="$code"
  run_application_rejection "$second_boundary_root" "$application_consumer" "$code" "$tmp/$code.err"
done
export DORKPIPE_ORCH_HELPER_BIN="$real_helper_bin"
unset DORKPIPE_BACKLOG_FAKE_APPLICATION_ERROR
export DORKPIPE_BACKLOG_ARTIFACT_ROOT="$artifact_root"
export DORKPIPE_BACKLOG_CONSUMER_ROOT="$application_consumer"

if find "$artifact_root" -mindepth 1 \( -iname '*apply*' -o -iname '*commit*' -o -iname '*push*' -o -iname '*publish*' \) -print -quit | grep -q .; then
  echo "fixture slice created a forbidden lifecycle artifact" >&2
  exit 1
fi

echo "test_backlog_remote_workflow OK"
