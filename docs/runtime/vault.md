# Secret environments and vault templates

Dockpipe can load an explicitly selected secret environment before workflow steps. Provider adapters live in the `secrets` package; the engine only knows the resolver exchange. Existing `op://` vault templates continue to work.

## Named environments

Install/build the `secrets` package and authenticate the chosen vendor CLI on the host. Configure references, never credentials, in `dockpipe.config.json`:

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
| `infisical` | `project_id`, `environment` | `path`, `domain` | Variable in the selected project/environment/path | Authenticated Infisical CLI or `INFISICAL_TOKEN` |

Azure retrieves each bound secret by name. AWS reads one JSON secret; `SecretBinary` and non-string selected fields are unsupported. Provider errors withhold stdout/stderr so credentials cannot be printed by the resolver error path. CLI authentication must already be configured; Dockpipe does not provision access policies or credentials. `op`/Infisical may fetch their whole remote environment internally, so use a tightly scoped remote environment as well as a narrow binding list.

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

## Legacy vault templates

**`dockpipe.config.json`** points at an env **template** (**`secrets.vault_template`**, legacy **`op_inject_template`**) — often **`.env.template`** (agnostic name). With **`secrets.vault: op`**, lines use **`op://…`** for 1Password. Before workflow steps, Dockpipe runs **`op inject`** (requires **[1Password CLI](https://developer.1password.com/docs/cli/)**’s **`op`** on **`PATH`**) unless **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**.

## In memory only (no resolved secrets file from Dockpipe)

The CLI runs **`op inject -i <template>`** with **no `--out-file`** so **`op`** writes to **stdout** (see **`op inject --help`**: `-o` means “to a file **instead of** stdout”). It reads **stdout into process memory** — it does **not** write a second “resolved” template file. Do **not** use **`op inject -i … -o -`**: **`-o`** takes a **path**; **`-`** is a file named **`-`** in the current directory, not stdout — same pitfall as shell **`> -`**.

If you see a file literally named **`-`** in the repo root, that almost always comes from a **shell mistake**: **`op inject … > -`** redirects output to a **file** named **`-`**, not to stdout. **Do not use `> -`.** When **`dockpipe`** builds step env (including when **`DOCKPIPE_OP_INJECT=0`** or **`--no-op-inject`**), it may **remove** that file if it looks like accidental inject output (env-like text); set **`DOCKPIPE_KEEP_DASH_FILE=1`** to keep it. For manual checks, run **`op inject -i .env.op.template`** (writes to stdout) or **`op inject -i .env.op.template | …`**, then **`rm -- -`** to remove any stray file. **Delete** that file if it contains plaintext secrets and rotate credentials if it was ever committed.

- **`secrets.vault`** — optional default backend when workflow YAML **omits** **`vault:`**. With **`op`**, Dockpipe uses the 1Password inject path only when the selected workflow references one of the template keys. This is backend selection, not a force-inject flag.
- **`vault: op`** on a workflow — strict opt-in. Dockpipe requires the template path/config and runs **`op inject`** even when the project default would otherwise skip.

Workflow **`vault:`** overrides **`secrets.vault`** when non-empty. Resolution order for the template file: **`vault_template`** first, then **`op_inject_template`**.

Bundled **secretstore** (dotenv) loads a plain file — see **`src/core/workflows/secretstore/README.md`**. **`dockpipe doctor`** reports template path and **`op`** availability when **`dockpipe.config.json`** is present.
