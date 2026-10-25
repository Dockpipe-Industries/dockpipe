# Use VS Code tasks and launch configurations

Dockpipe discovers `.vscode/tasks.json` and `.vscode/launch.json` in the selected
project. Comments and trailing commas are accepted. No import, source rewrite or
VS Code installation is required. Discovery never starts a task automatically.

Tasks appear as workflows. Launch configurations appear as apps and run without
a debugger. The GUI labels these entries **VS Code · tasks.json** or
**VS Code · launch.json** in its workflow picker, app cards and dashboard widgets.
Unsupported entries remain visible with the reason they cannot run.

## Run from the source files

List entries and copy the stable `workflow_id` for the task or app:

```sh
dockpipe workflow list --workdir /path/to/project --format json
dockpipe --workdir /path/to/project --workflow vscode-task-<id-from-list> --
```

The same entries are included in `dockpipe catalog list --format json`, with
`source`, `source_kind`, `source_path`, `source_name`, `supported`,
`unsupported_reason` and `cloneable` metadata. Editor JSON is not reported as a
Dockpipe YAML `config_path`.

Supported behavior includes:

- Shell and process tasks, ordered dependencies and dependency-only tasks.
- Windows, Linux and macOS overrides, task working directories and environment.
- Workspace, parent-environment and home-directory variables.
- `promptString` and `pickString` inputs. The GUI prompts for values and
  validates choices. Password values are masked, kept out of saved application
  settings and passed through the child environment rather than command arguments.
- Node, Python, .NET and native executable launch configurations, including
  supported `preLaunchTask` dependencies.

Input records in the JSON catalog expose their `env_name`. CLI callers supply
that key through the process environment or `--env KEY=value`. Avoid putting
password values in command-line arguments. Shell arguments retain their value
when they contain whitespace, quotes or shell metacharacters.

These are normal Dockpipe host steps: the configured tools must be installed.
Bash is required only when a task selects Bash or invokes a Bash script.
Windows shell tasks default to PowerShell; Linux and macOS shell tasks default
to Bash. Use `options.shell` to select a supported shell explicitly when your
VS Code terminal default differs. A Flatpak host step remains inside its sandbox;
this adapter does not grant access to host tools that are unavailable there.

## Clone into editable Dockpipe workflows

Use **Clone to workflows** on the imported entry, or:

```sh
dockpipe workflow clone vscode-task-<id-from-list> \
  --workdir /path/to/project --name my-build --format json
dockpipe --workdir /path/to/project --workflow my-build --
```

The command creates `workflows/my-build/config.yml`, using the configured
workflow directory when overridden. Input-bearing clones also include an
`ImportedInputs.pipe` file so the GUI retains input labels, choices and
password handling. Existing directories are never overwritten. Omit `--name`
to use `imported-task-<id>` or `imported-launch-<id>`.

A clone is independent of `.vscode`: edit its YAML and input model, and run it
after removing the original editor configuration. Workspace and environment
references remain references; the current machine's paths, environment values,
supplied input values and password defaults are not copied. Platform overrides
are resolved for the platform performing the clone, and the generated workflow
declares that platform. Review the clone before adapting it for another OS.

## Compatibility boundaries

This provides command execution, not VS Code's extension host or debugger.
Attach requests, breakpoints, editor-command variables, selected-file context,
extension-contributed task types, debug-adapter hooks and launch-settings profiles
are unsupported. Parallel task dependencies and background readiness matchers
are also unsupported; they are not silently changed to sequential execution.
Multi-root `.code-workspace` configuration and launch compounds are not discovered;
use the individual configurations in the selected folder's `.vscode` directory.
`${config:...}` needs editor settings and cannot currently run or clone through
the GUI. Shell-specific argument objects and `cmd.exe` tasks with separate
arguments require an additional quoting adapter; use a process task or PowerShell.

Direct editor entries run locally. Clone them to native workflows before using
Dockpipe's remote workflow delivery. A clone still needs its source project
files and tools on the execution machine.

The source formats are documented in Microsoft's
[task reference](https://code.visualstudio.com/docs/debugtest/tasks) and
[variable reference](https://code.visualstudio.com/docs/reference/variables-reference).
