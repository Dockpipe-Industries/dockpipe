# WSL2 → Windows: git bundle fetch (optional / advanced)

**Most users:** run **`dockpipe.exe` natively on Windows** (Docker Desktop + Git for Windows) and ignore this page — worktrees and commit-on-host use **Windows git** only.

Use this optional flow to transfer committed branches from a WSL clone to a Windows clone through a Git bundle, without a remote. Native Dockpipe on Windows does not require it.

---

Get branches from a repo in WSL into your Windows clone **without pushing to a remote**. Dockpipe can write a git bundle when it commits on the host; you then fetch that bundle from PowerShell.

Install the CLI for each environment you use; see [installation](../install.md). The examples below assume Dockpipe and the selected resolver are installed in WSL, and Git is installed on Windows.

**WSL bridge users** may also read the sections below; the **default** Windows story remains **native `dockpipe.exe`**, not bundle fetch.

**If you use the WSL bridge**, recommended one-time setup (Windows terminal):

```powershell
dockpipe windows setup
dockpipe windows doctor
```

This configures the target distro, bootstraps WSL host-awareness env (`DOCKPIPE_WINDOWS_HOST=1`), and can run a distro install command when needed (`--install-command`).

**Manual test checklist:** **[../manual-qa.md](../manual-qa.md)**.

### Dockpipe vs manual bundle (what’s actually “one branch”)

- **Workflow (worktree strategy + commit-worktree):** work happens in **one new worktree branch** (e.g. `claude/…` or `codex/…`); the **commit** is only on that branch’s tree. From the CLI this feels like a single branch.
- **The bundle file:** after commit-on-host, dockpipe runs **`git bundle create <path> refs/heads/<current-branch>`** (or **`HEAD`** if detached) — only the branch that was committed, so the file is usually **smaller** than `--all` when other branches have unique commits. Set **`DOCKPIPE_BUNDLE_ALL=1`** to restore the old **`--all`** behavior (see [`commit.go`](../../src/lib/infrastructure/commit.go)).
- **Windows fetch:** an explicit `+refs/heads/*:refs/heads/*` refspec updates local branches and is refused for a checked-out branch. Fetch into **`refs/remotes/wsl/…`** first. A plain `git fetch <bundle>` normally records `FETCH_HEAD`; it does not imply that local-branch refspec.

### Running dockpipe from Windows

**Default:** use **`dockpipe.exe`** from PowerShell or CMD; it runs **on Windows** (same as Linux/macOS for git/docker), no WSL shell required.

**WSL bridge:** set **`DOCKPIPE_USE_WSL_BRIDGE=1`** so normal commands are **forwarded into WSL**: your **current Windows folder** becomes the working directory in Linux (via `wslpath`), then `dockpipe` runs there. Only **`dockpipe windows …`** stays on the host. **Windows paths in typical file flags** (e.g. `--workdir`, `--mount`, drive letters and UNC) are rewritten for the Linux argv; the command you pass after **`--`** is not altered.

---

## End-to-end (one script)

Run dockpipe in WSL (worktree, commit, bundle) then fetch the bundle into your Windows repo. Save the script below as `Run-DockpipeWorktreeThenFetch.ps1` and run:

```powershell
.\Run-DockpipeWorktreeThenFetch.ps1 -RepoUrl "https://github.com/org/repo.git" -Branch "my-branch" -Prompt "Fix the bug" -WindowsRepo "C:\Source\YOUR_REPO" -BundlePath "C:\Users\YOUR_USER\YOUR_REPO.bundle"
```

This example assumes an installed `claude` resolver and CLI. Set `-DockpipeExecutable` to an absolute WSL executable path if Dockpipe is not on that distro’s `PATH`. The worktree strategy creates a branch and commits successful changes before writing the bundle; use it only when you intend those Git actions.

**Run-DockpipeWorktreeThenFetch.ps1:**

```powershell
param(
    [Parameter(Mandatory = $true)] [string] $RepoUrl,
    [Parameter(Mandatory = $true)] [string] $Branch,
    [Parameter(Mandatory = $true)] [string] $Prompt,
    [Parameter(Mandatory = $true)] [string] $WindowsRepo,
    [Parameter(Mandatory = $true)] [string] $BundlePath,
    [string] $DockpipeExecutable = "dockpipe"
)
$ErrorActionPreference = "Stop"
$BundlePath = [System.IO.Path]::GetFullPath($BundlePath)
$BundlePathWsl = wsl --exec wslpath -u $BundlePath
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
$BundlePathWsl = $BundlePathWsl.Trim()

# Pass distinct arguments; do not interpolate the prompt into a shell program.
$arguments = @(
    "--exec", $DockpipeExecutable,
    "--strategy", "worktree", "--resolver", "claude",
    "--repo", $RepoUrl, "--branch", $Branch,
    "--bundle-out", $BundlePathWsl,
    "--", "claude", "-p", $Prompt
)
wsl @arguments
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
& "$scriptDir\FetchFromBundle.ps1" -WindowsRepo $WindowsRepo -BundlePath $BundlePath
exit $LASTEXITCODE
```

---

## Step by step

### 1. Generate the bundle (in WSL)

Use a path under `/mnt/c/...` so Windows can see the file. This invokes the compatibility worktree strategy, including its commit-on-success hook; it requires the installed `claude` resolver.

```bash
dockpipe --strategy worktree --resolver claude --repo https://github.com/org/repo.git --branch my-branch \
  --bundle-out /mnt/c/Users/YOUR_USER/YOUR_REPO.bundle \
  -- claude -p "Your prompt"
```

### 2. Fetch from the bundle (PowerShell)

**FetchFromBundle.ps1:**

```powershell
# Fetch from a git bundle into your Windows repo. Optionally create the bundle from a WSL repo first.
# Fetches every branch head that EXISTS IN THE BUNDLE into refs/remotes/wsl/* (safe: does not update your checked-out local branch).
# Dockpipe’s default bundle has only the work branch; a manual "git bundle create --all" has every head.
# -MirrorLocalBranches: copy wsl/* to local branches (skips the branch you have checked out).
# -OnlyResolverBranches: when mirroring, only names under claude/* and codex/* (for fat bundles).
param(
    [Parameter(Mandatory = $true)] [string] $WindowsRepo,
    [Parameter(Mandatory = $true)] [string] $BundlePath,
    [string] $WSLRepoPath,
    [switch] $MirrorLocalBranches,
    [switch] $OnlyResolverBranches
)
$ErrorActionPreference = "Stop"
if ($WSLRepoPath) {
    $BundlePath = [System.IO.Path]::GetFullPath($BundlePath)
    $BundlePathWsl = wsl --exec wslpath -u $BundlePath
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    $BundlePathWsl = $BundlePathWsl.Trim()
    wsl --exec git -C $WSLRepoPath bundle create $BundlePathWsl --all
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}
$BundlePath = [System.IO.Path]::GetFullPath($BundlePath)
Set-Location $WindowsRepo
git fetch $BundlePath "+refs/heads/*:refs/remotes/wsl/*"
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

if ($MirrorLocalBranches) {
    $current = git branch --show-current
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    $refs = git for-each-ref "refs/remotes/wsl" --format="%(refname)"
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    foreach ($ref in $refs) {
        if ($ref -notmatch '^refs/remotes/wsl/(.+)$') {
            continue
        }
        $name = $Matches[1]
        if ($OnlyResolverBranches -and ($name -notmatch '^(claude|codex)/')) {
            continue
        }
        if ($name -eq $current) {
            continue
        }
        git branch -f $name $ref
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    }
}
exit 0
```

Run:

```powershell
.\FetchFromBundle.ps1 -WindowsRepo "C:\Source\YOUR_REPO" -BundlePath "C:\Users\YOUR_USER\YOUR_REPO.bundle"
```

To create the bundle from an existing WSL repo (no dockpipe): pass `-WSLRepoPath "/home/YOUR_USER/source/YOUR_REPO"`.

To also force local branch tips to the imported refs (except the current branch), add **`-MirrorLocalBranches`**. This can replace a local branch’s unmerged tip; review the refs first. Git refuses branches checked out in other worktrees, and the script stops on that error. If the bundle was built with **`--all`** and you only want **claude/** and **codex/** locals: **`-MirrorLocalBranches -OnlyResolverBranches`**.

**Even smaller bundles (optional):** Git can pack only commits not reachable from a basis (e.g. `origin/main..HEAD`), but dockpipe does not pick a basis for you; the default **single-branch** bundle already drops **other branch tips** and their unique commits from the pack.
