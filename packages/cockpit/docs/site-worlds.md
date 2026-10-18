# Site-wide spatial worlds

Cockpit → Web now starts with **Site-wide** enabled. Public website structure
becomes a persistent world overview, semantic sections and source exhibits. The
entrance appears while discovery continues. Enter a section by walking to its
portal or selecting it; select an exhibit to read attributed source excerpts and
follow related sources. **Original page** opens the existing single-page browser
experience. **Live page / Return to space** retains the current spatial scene.
Turning Site-wide off restores the existing DOM-based single-page experience,
including theatre playback and vehicle interactions.

## Controls

- **World overview** returns to the site's section entrances.
- **Map / discovery** shows accepted, pending, excluded and unavailable sources,
  their scores, depths, canonical references, duplicate groups and reasons.
  Selecting a section also shows any cached neural layout feature scores.
  Search filters the hierarchy by title, subject, URL or reason. Activate a row
  to enter its section or source exhibit directly.
- **Area ← / →** pages through sections with more than twelve destinations.
- **Update area** rebuilds the current area's content using its saved placement
  slots. Discovery does not continually replace the scene under the user.
- **Pages**, **Depth** and **Relevance** control discovery and deterministic
  inclusion. **Discover / refresh** starts another bounded pass, revalidating
  known public content. **Stop** cancels pending network work.
- **Include / Exclude / Automatic** override relevance decisions for a selected
  source, or the currently discovered sources in a selected section. Overrides
  cannot bypass robots, source indexing restrictions or authentication rules.
  Section selection does not create a rule for undiscovered future sources.
- **Refresh source** refreshes one known URL; **Original page** opens its source.
- **Clear world cache** removes this site's graph, slots and neural suggestions.
  Discover again to rebuild. Furnishing overlays have their own lifecycle.

The graph caches source excerpts and metadata, not a complete offline website.
It retains at most 256 URL records and 24 semantic sections per site. Sources are
grouped using explicit article sections, JSON-LD or HTML breadcrumbs, then subject
rules. URL paths do not determine room geometry. Extra topics share a clearly
labelled More topics section. Empty sections retain their slots so removing and
restoring content does not move neighbouring destinations.

## Discovery and trust boundary

`SpatialSitePolicy`, `SpatialSiteContent`, `SpatialSiteWorld`,
`SpatialSiteFetch`, `SpatialSiteCrawler` and `SpatialSiteExplorer` separate policy,
interpretation, persistent semantics, network I/O, scheduling and UI. The graph
reuses `SpatialWebNode` identities and projects a bounded `SpatialWebDocument`
into the existing world planner/compiler and `WorkspaceFlightView`. No separate
renderer or new engine primitive is introduced.

HTML interpretation runs on one bounded worker, separate from the UI thread.
Cancellation invalidates queued results, and shutdown drains the bounded active
parse before releasing its owner.

Discovery uses an anonymous Qt Network client, with no browser cookies, credentials
or system proxy. It checks robots.txt before fetching content, follows nested XML
sitemaps and falls back to HTML navigation and internal links. Primary navigation
is prioritized. Canonical references, JSON-LD, headings, descriptions, related
links and bounded image references contribute to the graph. Scripts are never
executed by discovery and their markup is excluded from extracted content.

Images are resources, not crawl destinations. Up to four image references per
source come from JSON-LD, Open Graph, ordinary `src`, lazy-image attributes and
responsive candidates. CDN query strings remain intact instead of passing through
content-URL deduplication. Only assets explicitly referenced by an accepted source
may be requested across origins. They share the crawl's request, byte, time and
spacing limits, retain the original identity through redirects, and never expand
the page graph. Same-origin robots rules still apply; CDN image requests use the
same anonymous transport and public-address/DNS-pinning checks. Responses must be
supported raster images before the bounded decoder accepts them.

Content URL normalization discards fragments, tracking and unknown query parameters.
Only `id`, `v`, `article`, `lang` and `language` query keys are retained. Login,
account and transaction routes are restricted; utilities, pagination, repeated
listings, version-switch links, low-information and duplicate content are filtered
with visible reasons. A version switch requires both a version label and a
version-only path; dated historical records are retained for normal scoring.
Legal boilerplate is downranked except on sites whose subject is legal/policy
research. Exact hashes, canonical references and bounded text fingerprints detect
duplicates. Relevance is a deterministic score, not a statistical confidence.

Every request, redirect and image stays on the exact entered origin. Only standard
HTTP(S) ports are allowed. DNS resolution must return public addresses; the actual
connection is pinned to a validated IP while retaining the source HTTP hostname
and TLS certificate verification name. Private, loopback, link-local, special-use
and transition addresses are blocked, including IPv4-mapped IPv6. Redirects are
manual and revalidated. TLS errors are not ignored. Discovery never reaches the
launcher's privileged command functionality.

Robots processing supports DockpipeSpatial and wildcard groups, longest matching
allow/disallow rules, wildcards, end anchors, percent-encoded paths, sitemap hints
and crawl delays. 404/410 robots responses permit bounded discovery; other failures
fail closed. Source noindex/noarchive/nofollow directives are respected. Restricted records
retain their URL and decision, but their excerpts and asset references are cleared. 429/503
responses stop the pass without retrying. Cross-origin robots redirects are
intentionally refused. See [RFC 9309](https://www.rfc-editor.org/rfc/rfc9309.html),
the [sitemap protocol](https://www.sitemaps.org/protocol.html), and Qt's
[peer verification API](https://doc.qt.io/qt-6/qnetworkrequest.html#setPeerVerifyName).

## Resource bounds and persistence

| Resource | Default / hard bound |
| --- | --- |
| Concurrent fetches | One |
| Request spacing | At least 750 ms after completion; longer crawl delays honored |
| Content pages per pass | 32 / 64 |
| Link depth | 3 / 6 |
| Pass duration | 90 / 120 seconds, including waits |
| Pass response bodies | 8 / 16 MiB |
| Individual response | 512 KiB, further limited by remaining pass bytes |
| DNS + fetch timeout | 8 seconds |
| Total requests | Page budget + 29; at most sixteen sitemap fetches |
| Redirect hops | Five; pages stay within the site, referenced images may use public CDNs |
| Graph / sections | 256 sources / 24 sections |
| Visible destinations | Twelve in one active area |
| Content per source | 24,000 characters analyzed; 1,200-character source excerpt cached |
| Link / image references | 64 links / four images per source |
| Image pixels | At most 2048² decoded; 512×320 display texture; twelve cached images |
| Persistent cache | 2 MiB per site; sixteen most recently written site files |
| Neural area suggestions | 48 sets of eight validated feature scores |

Entering a section may prefetch at most two pending related sources within normal
crawl limits. Entering a pending source starts a bounded pass if needed. A source
fetched within the last hour is reused unless explicitly refreshed. Hiding the
browser cancels DNS, replies, queued jobs and deferred image work. Old scene
resources are released through the existing renderer lifecycle.

Cache files live under Qt's application CacheLocation in `cockpit-sites`. Atomic
writes, per-site locks and revision checks prevent a stale second window from
silently overwriting another window's graph. Invalid caches are preserved until
explicitly cleared. IDs derive from canonicalized URLs or semantic section names;
placement slots are retained across refreshes and exclusions. Deleted/error pages
are marked unavailable after a fetch. Unchanged areas need no regeneration.

Existing optional local neural interpretation can propose combinations of the
current eight architectural features for an area. Results are validated by the
existing compiler, cached and reused. Inference is not part of frame updates or
navigation. Source text is data, never a tool instruction; exhibits contain source
excerpts rather than invented facts. Procedural structures are navigation scenery,
not claimed reconstructions of real buildings or artifacts.

## Compatibility and current limits

Discovery reads bounded public static HTML. JavaScript-only, authenticated,
anti-bot, cross-origin and oversized sites may need **Site-wide off**; unavailable
or unproductive discovery automatically offers the existing single-page path.
Cross-subdomain discovery, compressed sitemap files and cross-origin/CDN image
fetching are intentionally not included. Compressed responses are rejected when
an origin ignores the identity encoding request. No credentials are copied into
the crawler, including after using the live page.

The current semantic hierarchy is world → section → source/excerpt, with related
source links. It does not infer a comprehensive scholarly ontology, chronology or
course prerequisites. Neural interpretation refines architecture through the
existing local model; it is not a new general-purpose site-planning model.
Layouts reuse bounded construction parts, preserving source locations even when
new content arrives. Site-wide source exhibits can open the original single-page
experience for richer interactions such as films and vehicles.

## Validation

`cockpit-test --site-world` uses injected controlled HTTP responses and isolated
cache/settings directories. It covers robots, nested/missing/malformed sitemaps,
metadata and breadcrumb grouping, education/museum/docs/product/encyclopedia
content, filtering and overrides, duplicates, stable placements, cache conflicts,
progressive discovery, cancellation during parsing, UI event-loop responsiveness,
limits and network policy rejections.
It renders the real explorer and existing world compiler. On a native desktop it
also tests the full WebEngine browser shell through overview, sections, excerpts
and exit cancellation. No external site is required for deterministic tests.
The separate native `--spatial-web` fixture covers single-page DOM extraction,
theatre playback and live-page return behavior.

## Walk-through loading passages

Walking into a web destination starts loading behind a physical entrance. Source
artwork is mounted on the building's door, at readable height. In walking mode,
the current area stays visible while a short covered passage extends ahead of the
player. It starts with two 400-unit sections and stops at four sections (1,600
scene units). If preparation is still pending, the far door remains solid and
reads **Loading — please wait**. Once the validated next area is available, the
door opens and that area appears beyond it; images continue arriving in place.
Holding movement carries through the handoff without a fade or camera reset.

This is bounded area streaming, not an entire website resident in memory. At most
two generated areas and their connection are displayed during a handoff: 224
solids and the existing 24-display limit. Existing mesh/texture bounds still
apply. After entering the destination, the previous area's geometry is released
and coordinates are rebased without changing the player's view direction. Normal
areas retain their 96-solid limit. Corridor geometry changes only when another
section is needed or loading changes state; there is no per-frame generation.

**Back / Esc** cancels a passage and returns to its entrance. A 30-second
preparation deadline leaves a closed failure door with return instructions;
failed destinations cannot open from a late result. Opening Live page, hiding the
browser or entering another address cancels the passage. Theatre film selection
continues to play on the centre screen. Flight and driving keep their existing
navigation behavior; the covered passage currently serves walking entrances.
Map navigation and explicit rebuilds remain direct area navigation.
