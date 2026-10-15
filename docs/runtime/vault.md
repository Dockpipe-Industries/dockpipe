# Secret environments and vault templates

Dockpipe can load an explicitly selected secret environment before workflow steps. Provider adapters live in the `secrets` package; the engine only knows the resolver exchange. Existing `op://` vault templates continue to work.

New projects created with `dockpipe init` have no vault template or secret provider configured. Add the configuration below only when a workflow needs secrets. Running `init` again preserves existing configuration. In a project created by an older version, remove the generated `secrets.vault_template` entry if you do not use a vault; existing projects are not migrated automatically.

## Named environments

Install/build the `secrets` package and install the chosen vendor CLI on the host. Infisical can sign in through your browser during an interactive run; other providers require an existing CLI login. Configure references, never credentials, in your private project's `dockpipe.config.json`. Public repositories must use placeholders for account, environment and vault identifiers; see the [public repository boundary](../public-repository-boundary.md):

```json
{
  "schema": 1,
  "secrets": {
    "environments": {
      "production": {
        "resolver": "onepassword",
        "parameters": { "environment_id": "your-environment-id" },
        "bindings": {
          "AWS_ACCESS_KEY_ID": "PUBLISH_ACCESS_KEY_ID",
          "AWS_SECRET_ACCESS_KEY": "PUBLISH_SECRET_ACCESS_KEY"
        }
      }
    }
  }
}
```

Bindings map a workflow variable name to a provider variable/key/secret name. Only those bindings enter the workflow environment. Values, including multiline keys, stay in process memory. A missing binding, unknown environment, failed login, or malformed response stops the workflow before any values are merged. An empty value is preserved; workflows must reject empty credentials when required.

```yaml
name: publish
vault: environment
docker_preflight: false
steps:
  - kind: host
    run: ./publish.sh
```

```bash
dockpipe --workflow publish --secret-environment production --
```

`--secret-environment` selects one configured name and never falls back to a different environment or the legacy template. It overrides the project vault default but conflicts with an explicit workflow `vault: op`. `vault: none`, `--no-vault` (alias `--no-op-inject`), or `DOCKPIPE_VAULT_INJECT=0` disables injection. The legacy `DOCKPIPE_OP_INJECT=0` is still honored when `DOCKPIPE_VAULT_INJECT` is unset.

Optional `secrets.environment` sets the default name. Use `secrets.vault: environment` with it. A project default loads only when the workflow references one of its binding keys; explicit `vault: environment` or the CLI selection always loads it. Keep the project default unset when different workflows need different privilege levels.

## Providers

| Resolver | Required parameters | Optional parameters | Binding source | Host authentication |
| --- | --- | --- | --- | --- |
| `onepassword` | `environment_id` | `account` | 1Password Environment variable | Authenticated `op` CLI; requires a build supporting `op run --environment` (currently beta) |
| `aws-secretsmanager` | `secret_id` | `region`, `profile`, `version_stage` | Top-level string key in a JSON `SecretString` | AWS CLI credential chain, including profiles, SSO, and workload identity |
| `azure-keyvault` | `vault_name` | `subscription` | Key Vault secret name | Authenticated Azure CLI, including managed/workload identity |
| `infisical` | `project_id`, `environment` | `path`, `domain`, `profile`, `organization_id` | Variable in the selected project/environment/path | CLI session with browser login when needed, or `INFISICAL_TOKEN` |

Azure retrieves each bound secret by name. AWS reads one JSON secret; `SecretBinary` and non-string selected fields are unsupported. Secret resolution errors withhold stdout/stderr so credentials cannot be printed by the resolver error path. Dockpipe does not provision access policies or machine credentials. `op`/Infisical may fetch their whole remote environment internally, so use a tightly scoped remote environment as well as a narrow binding list.

### Infisical browser login

Set a dedicated `profile` and the intended `domain` and `organization_id` alongside
the project/environment parameters. Dockpipe checks that CLI session before reading
secrets. If it is missing, expired or rejected, a terminal run starts browser login,
then resumes the same workflow. Existing profiles are renewed with `--profile`;
new profiles are created with `--save-as`. The global default profile is unchanged.
An inherited `INFISICAL_PROFILE` is also accepted when `profile` is omitted.
Choose the intended account in the browser. `organization_id` scopes subsequent
CLI status/read commands; it is not passed as the CLI's direct-password login flag.

```json
{
  "resolver": "infisical",
  "parameters": {
    "project_id": "project-id",
    "environment": "dev",
    "path": "/",
    "domain": "https://app.infisical.com/api",
    "profile": "my-project-dev",
    "organization_id": "organization-id"
  },
  "bindings": {
    "TOKEN": "APP_TOKEN"
  }
}
```

Use a current Infisical CLI with `login status --json` and named-profile support
(verified with 0.43.140). Authentication belongs to the CLI's credential store;
Dockpipe does not write tokens or application secrets to project files. The login
UI displays only a validated browser challenge URL. On Linux Dockpipe also attempts
to open that URL; it remains clickable if the browser launcher is unavailable.

Both stdin and stderr must be terminals, and `CI` must be unset or false/0.
Unattended runs fail with an authentication-required message instead of opening a
browser. `INFISICAL_TOKEN` always uses machine/token authentication without browser
fallback. Keyring, network, permission and missing-secret failures do not trigger
login loops. Login has a five-minute deadline and is attempted at most once.

Examples of provider parameters:

```json
{"resolver":"aws-secretsmanager","parameters":{"secret_id":"packages/production","region":"us-east-1"},"bindings":{"TOKEN":"publish_token"}}
{"resolver":"azure-keyvault","parameters":{"vault_name":"packages-production"},"bindings":{"TOKEN":"publish-token"}}
{"resolver":"infisical","parameters":{"project_id":"project-id","environment":"prod","path":"/packages"},"bindings":{"TOKEN":"PUBLISH_TOKEN"}}
```

## Resolver contract

A resolver profile declares `DOCKPIPE_SECRET_ENVIRONMENT_RESOLVE=assets/scripts/resolve-environment.sh`. The script must remain inside the resolver directory. The engine passes a bounded JSON request on stdin:

```json
{"schema":"dockpipe.secret-environment/v1","parameters":{"environment_id":"example"},"bindings":{"TOKEN":"PUBLISH_TOKEN"}}
```

The script returns exactly the requested variables on stdout:

```json
{"schema":"dockpipe.secret-environment/v1","values":{"TOKEN":"resolved-value"}}
```

Requests/responses are limited to 1 MiB and 256 bindings. Resolution has a two-minute deadline. Unexpected/missing variables and NUL values fail validation. No resolved template is written. Workflows still own how they use secrets: avoid printing them, shell tracing, or writing them to artifacts.

Resolvers may return a nonzero exit with the fixed response
`{"schema":"dockpipe.secret-environment/v1","error":"authentication_required"}`
and no values. An optional `DOCKPIPE_SECRET_ENVIRONMENT_AUTHENTICATE` profile field
names a package-local login script. In an interactive terminal, the engine calls
it with the same nonsecret request on stdin, then retries resolution once. The
authentication hook's stdout is discarded and stderr is reserved for safe login
progress/challenges; the package must never print tokens or secret values there.
Both hooks are checked for package path containment. Cancellation stops their
process trees. No provider-specific login behavior is built into the engine.

## Legacy vault templates

To opt in, create a template containing only secret references, for example `.env.vault.template`:

```dotenv
TOKEN=op://your-vault/your-item/token
```

Add the template path to your existing `dockpipe.config.json`:

```json
{
  "schema": 1,
  "secrets": {
    "vault_template": ".env.vault.template",
    "vault": "op"
  }
}
```

**`dockpipe.config.json`** points at an env **template** (**`secrets.vault_template`**, legacy **`op_inject_template`**) — often **`.env.template`** (agnostic name). With **`secrets.vault: op`**, lines use **`op://…`** for 1Password. Before workflow steps, Dockpipe runs **`op inject`** (requires **[1Password CLI](https://developer.1password.com/docs/cli/)**’s **`op`** on **`PATH`**) unless **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**.

## In memory only (no resolved secrets file from Dockpipe)

The CLI runs **`op inject -i <template>`** with **no `--out-file`** so **`op`** writes to **stdout** (see **`op inject --help`**: `-o` means “to a file **instead of** stdout”). It reads **stdout into process memory** — it does **not** write a second “resolved” template file. Do **not** use **`op inject -i … -o -`**: **`-o`** takes a **path**; **`-`** is a file named **`-`** in the current directory, not stdout — same pitfall as shell **`> -`**.

If you see a file literally named **`-`** in the repo root, that almost always comes from a **shell mistake**: **`op inject … > -`** redirects output to a **file** named **`-`**, not to stdout. **Do not use `> -`.** When **`dockpipe`** builds step env (including when **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**), it may **remove** that file if it looks like accidental inject output (env-like text); set **`DOCKPIPE_KEEP_DASH_FILE=1`** to keep it. For manual checks, run **`op inject -i .env.op.template`** (writes to stdout) or **`op inject -i .env.op.template | …`**, then **`rm -- -`** to remove any stray file. **Delete** that file if it contains plaintext secrets and rotate credentials if it was ever committed.

- **`secrets.vault`** — optional default backend when workflow YAML **omits** **`vault:`**. With **`op`**, Dockpipe uses the 1Password inject path only when the selected workflow references one of the template keys. This is backend selection, not a force-inject flag.
- **`vault: op`** on a workflow — strict opt-in. Dockpipe requires the template path/config and runs **`op inject`** even when the project default would otherwise skip.

Workflow **`vault:`** overrides **`secrets.vault`** when non-empty. Resolution order for the template file: **`vault_template`** first, then **`op_inject_template`**.

Bundled **secretstore** (dotenv) loads a plain file — see **`src/core/workflows/secretstore/README.md`**. **`dockpipe doctor`** reports template path and **`op`** availability when **`dockpipe.config.json`** is present.
