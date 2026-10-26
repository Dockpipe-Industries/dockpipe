#!/usr/bin/env bash
# Run inside the SDK. Git implements the core runtime's workspace operations.
set -euo pipefail
mkdir -p /app/share
install -m755 /usr/bin/git /app/libexec/dockpipe/git
cp -a /usr/libexec/git-core /app/libexec/git-core
cp -a /usr/share/git-core /app/share/git-core
cat > /app/bin/git <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
export GIT_EXEC_PATH=/app/libexec/git-core
export GIT_TEMPLATE_DIR=/app/share/git-core/templates
exec /app/libexec/dockpipe/git "$@"
SCRIPT
chmod 755 /app/bin/git
