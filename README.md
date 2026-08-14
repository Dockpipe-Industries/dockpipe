# DockPipe

**Run any command in a disposable container, then optionally act on the result.**

DockPipe gives you a simple way to run tests, scripts, code generation, and AI tools in clean Docker environments. Your working directory is mounted into the container, files remain owned by your user, and the container disappears when the command finishes.

> [!IMPORTANT]
> **DockPipe 0.6.0 is coming soon—and it is a massive improvement.**
>
> Preview the next version on the **[`dev` branch](https://github.com/Dockpipe-Industries/dockpipe/tree/dev)**.

## Quick Start

### Install

Download the latest `.deb` from [GitHub Releases](https://github.com/Dockpipe-Industries/dockpipe/releases):

```bash
sudo dpkg -i dockpipe_*_all.deb
```

Or install from source:

```bash
git clone https://github.com/Dockpipe-Industries/dockpipe.git
cd dockpipe
export PATH="$PWD/bin:$PATH"
```

DockPipe requires **Docker** and **Bash**.

### Run a Command

```bash
dockpipe -- make test
```

DockPipe runs `make test` in a clean container with your current directory mounted at `/work`. When the command exits, the container is removed.

The same pattern works for any command:

```bash
dockpipe -- npm test
dockpipe -- cargo test
dockpipe -- ./scripts/generate-docs.sh
```

## What You Can Do

| Use case | Command |
| --- | --- |
| Run tests in isolation | `dockpipe -- make test` |
| Run a script | `dockpipe -- ./scripts/generate-docs.sh` |
| Pipe standard input | `echo "input" \| dockpipe -- command` |
| Run and then commit changes | `dockpipe --action examples/actions/commit-worktree.sh -- ./scripts/generate-docs.sh` |
| Run an AI tool | `dockpipe --template agent-dev -- claude -p "Review this project"` |
| Run an AI tool and commit its work | `dockpipe --template agent-dev --action examples/actions/commit-worktree.sh -- claude -p "Implement this task"` |

## How It Works

DockPipe has one small, composable lifecycle:

1. **Spawn** — Start a disposable container.
2. **Run** — Execute the command passed after `--`.
3. **Act** — Optionally run an action script on the result.

You choose the image, command, and optional action. DockPipe handles the Docker boilerplate, working-directory mount, user mapping, cleanup, and persistent tool state.

DockPipe is not an AI framework. AI tools are simply one of the many command types it can run.

## Why Not Just `docker run`?

You could write:

```bash
docker run --rm \
  -v "$(pwd):/work" \
  -w /work \
  -u "$(id -u):$(id -g)" \
  some-image \
  make test
```

DockPipe gives you the same isolation with a shorter command:

```bash
dockpipe -- make test
```

It also adds:

- an optional action phase
- reusable container templates
- persistent tool data
- pipe-friendly command handling
- automatic UID/GID mapping
- attached and detached execution

Files created inside the container remain owned by your host user.

## Persistent Data

By default, DockPipe mounts a named volume called `dockpipe-data` at `/dockpipe-data` and uses it as `HOME`.

This lets tools preserve state between disposable runs—for example, an authenticated CLI session or downloaded tool configuration.

Use a different named volume:

```bash
dockpipe --data-vol my-project-data -- command
```

Use a host directory:

```bash
dockpipe --data-dir "$HOME/.dockpipe" -- command
```

Disable persistent data:

```bash
dockpipe --no-data -- command
```

Recreate the default named volume:

```bash
dockpipe --reinit -- command
```

`--reinit` asks for confirmation. Use `--force` to skip the prompt.

If a tool exits unexpectedly while using the default data volume, try `--no-data` or recreate the volume with `--reinit`.

## Actions

Actions are scripts that run inside the container after the main command finishes.

For example:

```bash
dockpipe \
  --action examples/actions/commit-worktree.sh \
  -- ./scripts/generate-docs.sh
```

Actions receive:

- `DOCKPIPE_EXIT_CODE`
- `DOCKPIPE_CONTAINER_WORKDIR`

Create an action:

```bash
dockpipe action init my-action.sh
```

Start from a bundled action:

```bash
dockpipe action init my-commit.sh --from commit-worktree
```

Bundled examples include:

- [`commit-worktree`](examples/actions/commit-worktree.sh)
- [`export-patch`](examples/actions/export-patch.sh)
- [`print-summary`](examples/actions/print-summary.sh)

## Templates

| Template | Description |
| --- | --- |
| `base-dev` | Lightweight development environment with Git, curl, Bash, ripgrep, and jq |
| `dev` | General development environment with additional build tools |
| `agent-dev` | Development environment for AI coding tools |
| `claude` | Alias for `agent-dev` |

Use a template with any command:

```bash
dockpipe --template dev -- make test
```

## Examples

### Run a command

```bash
dockpipe -- ls -la
```

### Run a shell command

```bash
dockpipe -- bash -c "npm test"
```

### Run with a development template

```bash
dockpipe --template dev -- make test
```

### Run a script and commit its changes

```bash
dockpipe \
  --action examples/actions/commit-worktree.sh \
  -- ./my-script.sh
```

### Run Claude and commit its work

```bash
cd /path/to/repository

dockpipe \
  --template agent-dev \
  --action examples/actions/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=agent: implement task" \
  -- claude --dangerously-skip-permissions -p "Implement this task"
```

### Run in the background

```bash
dockpipe -d --template agent-dev -- claude -p "Review this repository"
```

Use Docker to inspect or reconnect to the running container:

```bash
docker logs <container-id>
docker attach <container-id>
```

### Resume a Claude session

```bash
dockpipe \
  --template agent-dev \
  -- claude --resume <session-id> --dangerously-skip-permissions
```

### Chain isolated commands

Each command runs in a fresh container:

```bash
dockpipe -- make lint \
  && dockpipe -- make test \
  && dockpipe -- make build
```

## Usage

```text
dockpipe [options] -- <command> [args...]
dockpipe action init [--from <bundled-action>] <filename>
```

| Option | Description |
| --- | --- |
| `--image <name>` | Select the Docker image |
| `--template <name>` | Use a predefined environment |
| `--action <script>` | Run an action after the command |
| `--workdir <path>` | Select the host directory mounted at `/work` |
| `--data-vol <name>` | Use a named volume for persistent data |
| `--data-dir <path>` | Use a host directory for persistent data |
| `--no-data` | Disable persistent data |
| `--reinit` | Recreate the named data volume |
| `-f`, `--force` | Skip confirmation when using `--reinit` |
| `--mount` | Add another volume mount |
| `--env` | Pass an environment variable |
| `-d`, `--detach` | Run the container in the background |
| `--help` | Show command help |

## Platform Support

| Platform | Installation |
| --- | --- |
| Linux | Install the `.deb` from [GitHub Releases](https://github.com/Dockpipe-Industries/dockpipe/releases) |
| macOS | Clone the repository and add `bin` to `PATH` |
| Windows | Use WSL with Docker and install from source |

See [docs/install.md](docs/install.md) for details.

## More Examples

- [Chained non-AI commands](examples/chained-non-ai/README.md)
- [Chained multi-AI commands](examples/chained-multi-ai/README.md)
- [Claude worktree example](examples/claude-worktree/README.md)
- [Codex worktree example](examples/codex-worktree/README.md)

## Development

Run the test suite from the repository root:

```bash
bash tests/run_tests.sh
```

Integration tests require Docker and the `agent-dev` image:

```bash
bash integration-tests/run.sh
```

See [integration-tests/README.md](integration-tests/README.md) for details.

## License

DockPipe is licensed under the [Apache License 2.0](LICENSE).
