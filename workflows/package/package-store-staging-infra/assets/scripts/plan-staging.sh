#!/usr/bin/env bash
# Maintainer composition: credentials arrive through the normal vault injection;
# staging resource identities are pinned afterwards, before the package runner.
set -euo pipefail
umask 077
root="${DOCKPIPE_WORKDIR:?Run through package-store-staging-infra}"
cli="${DOCKPIPE_BIN:-$root/src/bin/dockpipe}"
[[ -x "$cli" ]] || { echo "Repo-local DockPipe binary is required" >&2; exit 1; }
case "${DOCKPIPE_TF_COMMANDS:-plan}" in
  plan|init,plan|init,validate,plan) ;;
  *) echo "This workflow prepares a plan only; apply the reviewed saved plan separately." >&2; exit 1 ;;
esac
zone="${TF_VAR_zone_id:?Cloudflare zone id is required}"
[[ "$zone" =~ ^[a-fA-F0-9]{32}$ ]] || { echo "Invalid Cloudflare zone id" >&2; exit 1; }
# Do not inherit production tfvars, imports, workspaces, CLI flags or backend paths.
while IFS= read -r key; do
  case "$key" in
    DOCKPIPE_TF_STATE_ACCESS_KEY_ID|DOCKPIPE_TF_STATE_SECRET_ACCESS_KEY) ;;
    DOCKPIPE_TF_*|R2_TERRAFORM_*|R2_TF_*|TF_VAR_*|TF_CLI_ARGS*|TF_WORKSPACE|TF_DATA_DIR) unset "$key" ;;
  esac
done < <(compgen -e)
module="$root/packages/cloud/storage/resolvers/r2/dockpipe.cloudflare.r2infra"
artifacts="$("$cli" scope artifacts terraform --workdir "$root")"
mkdir -p "$artifacts"
run_directory="$(mktemp -d "$artifacts/staging-plan.XXXXXXXX")"
cp "$module"/terraform/*.tf "$run_directory/"
if [[ -f "$module/terraform/.terraform.lock.hcl" ]]; then
  cp "$module/terraform/.terraform.lock.hcl" "$run_directory/"
fi
export R2_BUCKET=dockpipe-staging
export DOCKPIPE_TF_MODULE_DIR="$run_directory"
export DOCKPIPE_TF_BACKEND=remote
export DOCKPIPE_TF_STATE_BUCKET=dockpipe-tfstate
export DOCKPIPE_TF_STATE_KEY=state/package-store-staging/terraform.tfstate
export DOCKPIPE_TF_COMMANDS=init,validate,plan
export DOCKPIPE_TF_INIT_ARGS=-reconfigure
export DOCKPIPE_TF_PLAN_ARGS=-out=staging.tfplan
export DOCKPIPE_TF_SKIP_INIT=0
export DOCKPIPE_TF_IMPORT_R2_BUCKET=0
export DOCKPIPE_TF_APPLY_AUTO_APPROVE=0
export DOCKPIPE_TF_ATTACH_CLOUDFLARE_PROVIDER=1
export DOCKPIPE_TF_DRY_RUN=0
export TF_DATA_DIR="$run_directory/.terraform"
export TF_VAR_zone_id="$zone"
export TF_VAR_public_hostname=packages.staging.dockpipe.com
export TF_VAR_enable_r2_custom_domain=true
export TF_VAR_r2_custom_domain_enabled=true
export TF_VAR_enable_cache_rules=false
export TF_VAR_enable_waf_baseline=false
# Invoke the existing package entrypoint without a second vault injection that
# could replace the staging hostname with the production template value.
bash "$module/assets/scripts/terraform-cloudflare-r2-run.sh"
printf 'Saved staging plan: %s/staging.tfplan\n' "$run_directory"
