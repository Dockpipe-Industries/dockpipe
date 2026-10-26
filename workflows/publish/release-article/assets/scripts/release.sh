#!/usr/bin/env bash
set -euo pipefail
if [[ -z "${DEVTO_API_KEY:-}" ]]; then
  echo "::error::Set secret DEVTO_API_KEY when DEVTO_PUBLISH is true."
  exit 1
fi
python3 workflows/publish/release-article/assets/scripts/prepare-article.py \
  --article "release/articles/announcement.md" \
  --repository "$ARTICLE_REPOSITORY" --source-sha "$ARTICLE_SOURCE_SHA" \
  --tag "$TAG" --output /tmp/devto-article.json
TITLE_MAIN=$(jq -r .title /tmp/devto-article.json)
TITLE_ONETIME="$TITLE_MAIN"
RELEASE_URL=$(jq -r .release_url /tmp/devto-article.json)
MAIN_IMAGE=$(jq -r .main_image /tmp/devto-article.json)
jq -r .body_markdown /tmp/devto-article.json > /tmp/devto-body.md
# Verify public media before publishing an article with broken images.
python3 - <<'PYIMAGES' > /tmp/devto-images.txt
import json, re
article = json.load(open("/tmp/devto-article.json"))
print(article["main_image"])
for url in re.findall(r"!\[[^\]]*\]\(([^)]+)\)", article["body_markdown"]):
    print(url)
PYIMAGES
while IFS= read -r image_url; do
  curl --fail --silent --show-error --location --retry 3 \
    --output /dev/null "$image_url"
done < /tmp/devto-images.txt
TAGS_JSON=$(python3 <<'PY'
import json, os
raw = os.environ.get("DEVTO_TAGS") or "dockpipe,cli,golang"
tags = [t.strip() for t in raw.split(",") if t.strip()]
print(json.dumps(tags))
PY
)

build_payload() {
  local title="$1"
  local out="$2"
  jq -n \
    --arg title "$title" \
    --rawfile body /tmp/devto-body.md \
    --arg canonical "$RELEASE_URL" \
    --arg main_image "$MAIN_IMAGE" \
    --argjson tags "$TAGS_JSON" \
    '{
      article: {
        title: $title,
        body_markdown: ($body | rtrimstr("\n")),
        published: true,
        canonical_url: $canonical,
        main_image: $main_image,
        tags: $tags
      }
    }' > "$out"
}

if [[ -n "${DEVTO_ARTICLE_ID:-}" ]]; then
  build_payload "$TITLE_MAIN" /tmp/devto-payload-main.json
  code=$(curl -sS -o /tmp/devto-resp.json -w "%{http_code}" \
    -X PUT "https://dev.to/api/articles/${DEVTO_ARTICLE_ID}" \
    -H "api-key: ${DEVTO_API_KEY}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    --data-binary @/tmp/devto-payload-main.json)
  if [[ "$code" != "200" && "$code" != "201" ]]; then
    echo "::error::dev.to PUT (main) HTTP ${code}"
    cat /tmp/devto-resp.json >&2 || true
    exit 1
  fi
  echo "dev.to main article ${DEVTO_ARTICLE_ID} updated (HTTP ${code})."
fi

if [[ "${DEVTO_ONE_TIME_POST:-}" == "true" ]]; then
  build_payload "$TITLE_ONETIME" /tmp/devto-payload-onetime.json
  code=$(curl -sS -o /tmp/devto-resp-onetime.json -w "%{http_code}" \
    -X POST "https://dev.to/api/articles" \
    -H "api-key: ${DEVTO_API_KEY}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json" \
    --data-binary @/tmp/devto-payload-onetime.json)
  if [[ "$code" != "200" && "$code" != "201" ]]; then
    echo "::error::dev.to POST (one-time) HTTP ${code}"
    cat /tmp/devto-resp-onetime.json >&2 || true
    exit 1
  fi
  ot_id=$(jq -r '.id // .article.id // empty' /tmp/devto-resp-onetime.json)
  ot_url=$(jq -r '.url // .article.url // empty' /tmp/devto-resp-onetime.json)
  echo "dev.to one-time post created (HTTP ${code}). id=${ot_id} url=${ot_url}"
fi
