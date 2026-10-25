# Light dev base: minimal tooling for scripts and generic commands.
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    iproute2 \
    jq \
    lsof \
    procps \
    ripgrep \
    && rm -rf /var/lib/apt/lists/*

# Numeric container users need a writable home for portable tooling.
ENV HOME=/work

# Accept the portable BSD base64 decode spelling alongside GNU coreutils.
RUN cat > /usr/local/bin/base64 <<'EOF'
#!/usr/bin/env bash
rb=/usr/bin/base64
[[ -x "$rb" ]] || rb=/bin/base64
args=()
for arg; do
  case "$arg" in
    -D) args+=(-d) ;;
    *) args+=("$arg") ;;
  esac
done
exec "$rb" "${args[@]}"
EOF
RUN chmod +x /usr/local/bin/base64
ENV PATH=/usr/local/bin:${PATH}

# HOME is /work; git --global needs that directory to exist before writing ~/.gitconfig.
RUN mkdir -p /work && chmod 777 /work \
    && git config --global --add safe.directory '*' \
    && git config --global init.defaultBranch main

COPY assets/entrypoint.sh /entrypoint.sh
RUN chmod 0755 /entrypoint.sh

WORKDIR /work
ENTRYPOINT ["/entrypoint.sh"]
