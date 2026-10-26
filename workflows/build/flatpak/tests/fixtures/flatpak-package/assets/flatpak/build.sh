#!/usr/bin/env bash
set -euo pipefail
source_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package="${1:?compiled store}/workflows/flatpak-example"
mkdir -p "$package/assets/tooling/bin" "$package/assets/tooling/lib"
cc -fPIC -shared "$source_dir/message.c" -o "$package/assets/tooling/lib/libmessage.so"
cc "$source_dir/main.c" -L"$package/assets/tooling/lib" -lmessage \
  -Wl,-rpath,'$ORIGIN/../lib' -o "$package/assets/tooling/bin/example"
cat > "$package/package.yml" <<'EOF'
schema: 1
name: flatpak-example
version: 1.0.0
kind: workflow
EOF
cat > "$package/config.yml" <<'EOF'
name: flatpak-example
platforms: [flatpak]
docker_preflight: false
dependencies:
  host:
    - command: example
steps:
  - id: bundled-dependency
    kind: host
    run: assets/run.sh
EOF
cat > "$package/assets/run.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exec example
EOF
chmod 755 "$package/assets/run.sh"
