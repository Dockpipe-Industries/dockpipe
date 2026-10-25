# Secret environments and vault templates

Dockpipe can load an explicitly selected secret environment before workflow steps. Provider adapters live in the `secrets` package; the engine only knows the resolver exchange. Existing `op://` vault templates continue to work.

New projects created with `dockpipe init` have no vault template or secret provider configured. Add the configuration below only when a workflow needs secrets. Running `init` again preserves existing configuration. In a project created by an older version, remove the generated `secrets.vault_template` entry if you do not use a vault; existing projects are not migrated automatically.

## Named environments

Install the published `secrets` package and its chosen vendor CLI. Follow the package’s provider setup instructions before selecting a named environment. Configure references, never credentials, in your private project's `dockpipe.config.json`. Public repositories must use placeholders for account, environment and vault identifiers; see the [public repository boundary](../public-repository-boundary.md):

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

## Provider setup

Provider parameters, vendor CLI requirements and login behavior belong to the [secrets package provider reference](https://github.com/Dockpipe-Industries/dockpipe-packages/blob/master/packages/secrets/docs/providers.md). The package supports adapters for 1Password, AWS Secrets Manager, Azure Key Vault and Infisical. Core does not provision provider permissions or machine credentials.

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

For an existing plain dotenv file, use **`--env-file <path>`**; this reads values directly and does not resolve vault references. Keep files containing credentials private. **`dockpipe doctor`** reports template path and **`op`** availability when **`dockpipe.config.json`** is present.
