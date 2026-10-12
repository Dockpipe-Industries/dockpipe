# Automating dev.to release posts

**Related:** In-repo drafts: **[blog-dockpipe-primitive.md](blog-dockpipe-primitive.md)**.

After each **GitHub Release**, the **Release** workflow can call the [Forem API](https://developers.forem.com/) for two **independent** actions:

| Action | API | When | Article id |
|--------|-----|------|------------|
| **Main** (living changelog) | `PUT /api/articles/{id}` | **`DEVTO_ARTICLE_ID`** is set | You create the post once on dev.to, store its numeric **id** in GitHub — every release **updates that same URL**. |
| **One-time post** | `POST /api/articles` | **`DEVTO_ONE_TIME_POST=true`** | **No id.** Each release **creates a new** article (release blog / announcement). Nothing to save for the next run. |

You can enable **either**, **both**, or **neither** (job runs only if at least one action is configured — see below).

Both actions publish the reviewed **`release/articles/X.Y.Z.md`** matching the `VERSION` baseline. Its first heading supplies the title. Release notes remain the separate GitHub release body. Images live in `release/articles/assets/`; publication pins their public URLs to the source commit and verifies that they are reachable. The company brand image supplies the article cover.

---

## GitHub configuration

**Environment `release`** (recommended) or repository **Variables** + **Secrets**:

### Secret

| Name | Value |
|------|-------|
| **`DEVTO_API_KEY`** | dev.to → Settings → Account → **DEV API keys** |

### Variables

| Name | Description |
|------|-------------|
| **`DEVTO_PUBLISH`** | Set to **`true`** to run the dev.to job (still needs at least one action below). |
| **`DEVTO_ARTICLE_ID`** | Numeric id of the **main** post to **PUT** each release. Omit if you only use one-time posts. |
| **`DEVTO_ONE_TIME_POST`** | Set to **`true`** to **POST** a **new** article every release (no id). Omit if you only update the main post. |
| **`DEVTO_TAGS`** | Optional. Comma-separated tags (lowercase). Default: `dockpipe,cli,golang`. |

**Job runs when:** `DEVTO_PUBLISH=true` **and** **`DEVTO_ARTICLE_ID` is non-empty** **or** **`DEVTO_ONE_TIME_POST=true`**.

If **`DEVTO_PUBLISH=true`** but neither action is set, the job is **skipped** (condition false).

---

## One-time setup

1. Create **`DEVTO_API_KEY`** on dev.to and add it as a **secret** on GitHub.
2. **Main post only:** Create a post on dev.to, copy **`id`** from the editor URL or **`data-article-id`**, set **`DEVTO_ARTICLE_ID`**. Leave **`DEVTO_ONE_TIME_POST`** unset or not `true`.
3. **One-time posts only:** Set **`DEVTO_ONE_TIME_POST=true`**. Do **not** set **`DEVTO_ARTICLE_ID`** (not used for POST).
4. **Both:** Set **`DEVTO_ARTICLE_ID`** **and** **`DEVTO_ONE_TIME_POST=true`** — each release **PUT**s the main article and **POST**s a separate new post.

---

## What gets published

- **Title:** first heading of the reviewed article. Legacy title variables are ignored.
- **Body:** the reviewed article, with image URLs pinned to the source commit and its production-release link pinned to the generated tag. Missing article or image files fail publication instead of falling back to release notes.
- **Cover:** `release/articles/assets/brand.png`, pinned to the same source commit.
- **`canonical_url`:** GitHub release URL.
- **`published`:** `true`.

Workflow: **`.github/workflows/release.yml`** → job **`devto`**.

---

## Troubleshooting

- **`401` / `403`:** Regenerate **DEVTO_API_KEY**.
- **`404` on PUT:** Wrong **`DEVTO_ARTICLE_ID`** or key cannot edit that article.
- **Job skipped:** **`DEVTO_PUBLISH`** must be **`true`** and you need **`DEVTO_ARTICLE_ID`** and/or **`DEVTO_ONE_TIME_POST=true`**.
