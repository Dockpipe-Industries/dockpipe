#!/usr/bin/env bash
set -euo pipefail
set -euo pipefail
mapfile -t shell_scripts < <(git ls-files '*.sh' ':!:src/app/tooling/vscode-extensions/**/node_modules/**' ':!:bin/**')
existing_shell_scripts=()
for script in "${shell_scripts[@]}"; do
  [[ -f "$script" ]] && existing_shell_scripts+=("$script")
done
if ((${#existing_shell_scripts[@]})); then
  shellcheck -S error "${existing_shell_scripts[@]}"
fi
