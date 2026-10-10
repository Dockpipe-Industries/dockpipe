# Your first workflow

Start with an [installed Dockpipe CLI or desktop app](install.md). You do not need
Dockpipe's source repository, Go, or its contributor tools. Use a folder you own;
all project paths below refer to that folder.

For Flatpak, replace `dockpipe` in the commands below with
`flatpak run --command=dockpipe com.dockpipe.Dockpipe`.

## 1. Check your installation

```sh
dockpipe --version
```

Native installations need Bash for host steps (Git for Windows supplies it on
Windows). The Flatpak includes its shell tools. Docker is only needed for container
steps; the first workflow below does not use it.

## 2. Create a workflow in your project

Open a terminal in your project folder and run:

```sh
dockpipe init hello
```

This creates project configuration when missing and an empty
`workflows/hello/config.yml`. Replace that file's contents with:

```yaml
name: hello
description: Print a message from this project.
docker_preflight: false

steps:
  - id: greet
    kind: host
    cmd: printf 'Hello from Dockpipe!\n'
```

Validate and run it from the same project folder:

```sh
dockpipe workflow validate workflows/hello/config.yml
dockpipe --workflow hello --
```

The run prints `Hello from Dockpipe!` alongside its progress messages. A `kind: host`
step runs in the environment where Dockpipe is running. For Flatpak, that means the
app sandbox; specific integrations can explicitly call host tools.

## 3. Try a container

For this step, install and start Docker or configure a supported container engine
as described in the [installation guide](install.md). Check connectivity with:

```sh
dockpipe doctor
```

Run a small command in an explicitly selected image:

```sh
dockpipe --runtime dockerimage --isolate alpine:3.22 -- pwd
```

The first run may download the image. Your project is mounted at `/work`; `pwd`
should print `/work`. The container is removed after the command finishes, while
files written into the mounted project remain.

To save that command as a workflow, replace `workflows/hello/config.yml` with:

```yaml
name: hello
runtime: dockerimage
isolate: alpine:3.22

steps:
  - id: location
    cmd: pwd
```

Validate and run it using the same two commands from step 2. Replace `cmd` with your
own project's command and select an image that contains the tools it needs.

## 4. Add reusable tools when you need them

The installer includes required core. Open **Packages → Marketplace** in the
launcher to install optional workflows and resolvers. Staging users first select
**Settings → Package Remotes → Use staging** and save.

The [package quickstart](packages/package-quickstart.md) covers discovery,
installation and dependencies from the terminal. A workflow shown in an example
is not necessarily installed: create it in your project or install its package
before running it.

## If something fails

| Problem | Check |
| --- | --- |
| `dockpipe` is not found | Open a new terminal after installation; Flatpak users use the command prefix above. |
| Bash is missing | Install Bash for the native CLI; on Windows use Git for Windows. |
| Docker cannot be reached | Start your engine and check its selected context/socket; host-only workflows do not need Docker. |
| Workflow is not found | Run from the folder containing `workflows/hello/config.yml`, or use `--workdir /path/to/project`. |
| Tool is missing inside a container | Choose an image or installed resolver that supplies that tool. |

## Next steps

- [Author workflows](workflows/workflow-authoring.md): steps, scripts and outputs.
- [Install packages](packages/package-quickstart.md): optional tools and workflows.
- [CLI reference](cli-reference.md): flags, project selection and diagnostics.
- [Security policy](security/security-policy.md): container permissions and host-step limits.
