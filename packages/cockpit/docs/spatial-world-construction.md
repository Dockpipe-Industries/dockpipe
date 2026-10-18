# Spatial world construction

Cockpit Web interprets real browser content into a transient, explorable scene.
This is an initial construction-engine slice, not a complete general game engine.
Everything runs locally without a model service. Cockpit remains optional and off
by default; these scenes do not change saved spatial workspace JSON.

## Boundaries

1. `resources/extract.js` observes bounded DOM content: titles, destinations,
   media/product roles, categories, prices, and page evidence. It does not choose
   building shapes or visual themes. The legacy `venue` field identifies interaction
   purpose (repository browsing, commerce, media selection, etc.).
2. `SpatialWebDocument` admits HTTP(S) URLs, unique display identities, bounded
   strings, and at most twelve items from at most 96 KiB of metadata.
3. `SpatialWebGraph` represents a hub, collections, destinations, containment,
   relative importance, source bindings and navigate/play actions. It contains no
   geometry. Parents precede children; invalid identities, URLs and cycles fail.
4. `SpatialWorldPlanner` produces a construction proposal. Topology, spacing,
   locomotion, materials, repetition and primitive shapes are separate inputs.
   Branches, a loop, a spine and layered flight use the same content graph.
   Site evidence selects starting parameters; it does not select a finished world.
5. `SpatialWorldCompiler` resolves hierarchical translation/yaw/uniform-scale
   transforms into solids, validates routes and interaction bindings, and creates
   a path mesh. Its success result is the publication boundary.
6. `WorkspaceFlightView` caches geometry once and handles movement, collision,
   picking and painting. It knows primitives, faces and surfaces, not websites or
   architectural themes. `SpatialWebBrowser` owns navigation and cancellation;
   `SpatialWebMedia` owns one browser-backed central-screen video stream.

## Construction vocabulary

A plan can combine box, pyramid and octahedron parts with arbitrary placement,
size, collision, material, label and parent transforms. One repeated assembly can
have different caps, spacing and scale without defining a new environment type.
Content surfaces retain source IDs; actual actions remain bound to the admitted
semantic graph. Physical traversal connections are distinct from containment.
Plans also accept bounded triangle/quad meshes, used for route surfaces and
custom landmarks. Mesh faces participate in selection occlusion; movement uses
explicit primitive collision volumes.

Animal collections prioritize content cards over site headers, pagination and
self-links. Animal exhibits retain their source photo and exact article link;
approaching one does not automatically reload a category. Selecting it opens its
facts as source excerpts in the spatial view. Following a page link does not
restart site discovery or publish a cached overview into its loading corridor.
Loading readiness depends on an article heading and meaningful main content;
unrelated scripts later in the HTML do not need to finish before the corridor
opens. A destination that has only delivered its head still waits, under the
existing bounded load and corridor deadlines.
Navigation uses Qt's typed loading status so cancellation of an unfinished source
page is distinct from failure of its destination.
Article destinations exclude links outside their main article content, keeping
site-wide menus and unrelated recommendation rails out of the exhibit selection.

The spatial browser requires a matching Qt 6.8+ launcher SDK. Older Chromium 90
builds cannot parse modern website scripts and may leave their live pages full of
one-pixel placeholders. The media regression build uses Qt 6.10.3, without a new
rendering engine or user-agent spoofing.

Image capture streams into existing exhibits in rotating passes, sharing a
ten-second deadline across at most twelve images. A one-pixel placeholder is not
a successful image. Selected lazy/responsive sources are activated in the page,
and uniquely identified images replaced during hydration can be rebound. Missing
images retain their source signs, are reported in the status, and can be retried
with Build space. Capture completion freezes the background page; navigating,
hiding or returning to HQ cancels pending callbacks. The world and camera remain
stable while textures arrive.

`SpatialAnimalModel` composes illustrative bodies, segments, limbs and wings for
nine coarse body plans. At most four models accompany the twelve source exhibits;
paths and scenery reserve their share of the existing polygon budget first.
Models are labelled illustrative and not to scale, not scientific reconstructions.
Recognized names work immediately without inference. With Enhance enabled, the
existing local semantic encoder can suggest a body plan for an explicitly
identified animal with an unknown form. Low confidence or ambiguous scores retain
the source image alone. No new model download, renderer or per-frame inference is
required; validated suggestions pass the existing scene compiler.

Pretrained image-to-3D generation is a planned, separate opt-in capability under
Cockpit / Spatial Web, disabled by default. It must disclose its download and
hardware requirements before installation, generate in the background with bounded
work and cached results, and release its worker when disabled. Source images and
procedural exhibits remain available without it. No image-to-3D backend is
currently installed or integrated by this feature.

This is deliberately a small vocabulary. The current automatic assembly rules
still produce recognizable repeated supports and display fixtures. Richer walls,
terrain, vegetation, arches and landmarks should extend those composable rules;
they should not add a separate renderer for each kind of world. The existing
shared-screen cinema remains a specialized compatibility recipe while media
interaction is developed. Its layout is not yet generated by the general planner.

## Validation and fallback

A proposal is rejected for invalid transforms, non-finite geometry, unsupported
traversal, lost content, obstructed interaction rays, blocked reserved routes or
disconnected anchors. Each route is sampled no farther apart than the camera's
25-unit collision radius; edges are capped at 800 samples. This is a conservative
check against static primitive collision, not a general navmesh solver. All
non-flight interaction anchors must remain at the walking/driving eye height.

On construction failure, a simple spine with minimal fixtures is planned and
validated. Invalid semantic input cannot be repaired by this fallback. The browser
preserves the previous world when construction fails. Empty destinations also
preserve the previous world. Page text cannot supply scripts or native code.
Future local-model proposals must pass the same construction boundary.

Bounds: twelve content textures; 25 semantic nodes/transforms/routes; 96 solids;
16 meshes with 512 vertices and 256 triangle/quad faces in aggregate. Octahedra
have eight faces, so the solid upper bound is 768 faces. A world spans at most
16,000 units per ground axis and 4,000 units vertically. These bounds keep graph
expansion, route validation, projection and texture storage independent of page
size. DOM extraction is capped at 5,000 nodes, with a 15-second conversion deadline.
Geometry arrives before image sprites; there is no per-frame planning or parsing.
Walking transitions temporarily compose two validated areas and a capped loading
passage, with at most 224 solids and 24 displays. The ordinary per-area compiler
limits above remain unchanged. See [loading passages](site-worlds.md#walk-through-loading-passages).

## Trying the same content in different worlds

After a page finishes building, use the layout selector:

- **Branching paths**: collections connect their nearby destinations to the hub;
  alternating radii create unequal branches.
- **Loop**: destinations connect sequentially around a perimeter.
- **Promenade**: destinations line alternating sides of a long central spine.
- **Flight network**: destinations occupy three elevations with flight controls.

These reuse the current page and image textures; they do not reload the website.
The theme remains independent of the selection. Walk/drive/flight can be selected
separately, although elevated content requires flight. This is not walkable stairs,
terrain-following driving, suspension, rigid-body physics or platform jumping.

## Vehicle components

Vehicle/Car markup and model-specific automotive cards supply a bounded body type,
optional model year and source identity. `SpatialWebDocument` validates those fields.
Ground composition places a `SpatialVehicle` behind its listing sign, outside the
reserved routes. The compiler rejects unbound, overlapping, out-of-bounds or
excessive vehicles. No website owns executable vehicle behaviour.

`SpatialVehicleModel` builds local low-poly faces for a pickup, SUV, coupe, sedan or
van. Year/name identify the listing; the geometry is explicitly approximate and
is not a manufacturer model. There is no GLB/glTF loader or image-to-mesh service
in this slice. A scene has at most twelve vehicles, each at most 224 faces, in
addition to the existing static geometry budget. Models are generated once;
only a moving vehicle's world transform is updated. Parked vehicles have no timer.

`WorkspaceFlightVehicle` owns entry, driver pose, footprint collision and parking.
Entry requires proximity and an unobstructed ray. The driver looks independently
of wheel heading. Steering and motion use steps bounded by 15 units / 0.03 radians;
rotated rectangles collide with primitive solids, other cars and world edges.
Exit checks six candidate positions and stays seated if all are blocked. Leaving
the Web view stops input/momentum; rebuilding or navigating replaces transient
vehicles. Elevated flight layouts retain source signs without ground vehicles.
Terrain following, rigid-body physics, exact-year body geometry and persistence
across world rebuilds remain future work.

## Media and limits

A theatre wall selection loads the real video page behind the scene and captures
only its video region into the central surface. It leaves the camera and theatre
in place. One 512 × 288 stream is sampled at up to twelve captures per second;
paused or stopped playback freezes the browser. Ordinary background loads remain
muted. Live page remains available for consent, authentication, forms and provider
controls. Browser codec/DRM support and the provider determine playability; a
local generated-video fixture does not qualify YouTube or other services. A
separate native test on this Linux host played the public Big Buck Bunny YouTube
video in the central screen with advancing frames and media time. This qualifies
that case only, not arbitrary YouTube sessions or DRM content. When Chromium
requires activation, the explicit poster action is forwarded once to an exposed
video/play button; consent and sign-in overlays are not clicked.

Other current limits: only two containment levels; no inferred cross-page site
map; no dynamic physics, arbitrary mesh collision, stairs or terrain navigation;
no generated detailed 3D product models; and no cross-platform browser packaging
qualification. Product imagery retains any background supplied by the website.

## Validation

`cockpit-test --spatial-web-model` covers graph separation, four distinct spatial
realizations, stable regeneration, URL bindings, resource bounds, sparse input,
invalid transforms, blocked/disconnected routes, lost content and fallback.
`cockpit-test --spatial-web` runs the native browser fixture: actual JavaScript,
image capture, approach navigation, history, cinema video frame/time advancement,
pause/resume, cancellation, idle freeze and Cockpit teardown. Offscreen CTest skips
native browser tests; run them with a real display to establish visual behavior.

## Page-driven composition

The bounded extraction includes image proportions and a validated page accent.
It can capture images, SVG artwork and decorative CSS backgrounds. Three or more
content categories produce branches; dense text catalogues produce aisles; visual
collections produce a loop. The seed includes the page path and semantic structure,
so pages on the same host need not share a layout. Artwork proportions affect
fixture and display widths, while the existing compiler still validates routes,
interaction anchors and resource budgets. This adds variation to the current
construction vocabulary; arbitrary geometry generation remains future work.

## Local semantic construction and foreground enrichment

`SpatialNeuralService` owns an optional isolated ONNX helper and model installer.
The page supplies bounded title/evidence/item text and captured PNG pixels only.
It cannot supply model paths, download URLs, scripts, operations or executable
prompts. The semantic encoder compares page/item embeddings with eight independent
construction-affordance descriptions. `SpatialNeuralService::apply` validates a
complete identity-preserving vector set atomically; malformed, missing, duplicate,
out-of-range and non-finite proposals leave the document unchanged.

`SpatialWorldArchitecture` blends material/terrain/traversal controls and builds
up to two three-part side assemblies for each destination. Together with the
platform and sign this fits the existing 96-solid ceiling. Outer scenery uses at
most eight meshes, forty vertices and thirty-two faces. The existing compiler
still checks bounds, every approach, route clearance, reachability and picking
visibility before publication. Dense-aisle exhibits face the aisle directly.

Inference uses one active request and at most twelve queued images. Text input is
at most twelve items plus page evidence, each encoded at 256 tokens. Images are
PNG only, at most 512x320. Input/output JSON lines are limited to 1 MiB. The UI has
32 cache entries (each limited by that reply cap); the helper retains 24 small
plans and 16 bounded images. Caches are private RAM, keyed by content, and cleared
when the browser is destroyed. No model service sees browsing content.

Navigation, explicit disable and teardown cancel the helper and invalidate queued
cache deliveries. Late designs cannot reset a moving user's camera: a pending
explicit entrance action replaces the design instead. The native fixture checks
this along with mixed content, cutouts, malformed output and continuous motion.
Each request has a 30-second deadline; idle workers release models after 30 seconds.
Linux setup runs in its own process group so cancellation also stops pip children.
The installer has a ten-minute parent deadline, bounded downloads, SHA-256 checks
and an atomic ready marker. Failed setup never enables inference.

Current limitations: this is semantic feature matching, not unrestricted model
scene synthesis; image depth, arbitrary GLB import and generated meshes are still
future extensions. CUDA selection can fall back to CPU when dependencies or
operators are unavailable. Runtime/model installation is currently native Linux.

### Article entrances and themed buildings

Walking destinations use a closed door, a recessed display window and a source
title on the building. Approaching the entrance starts the existing bounded
passage transition. Wildlife and habitat evidence selects timber, vegetation and
natural ground; storefronts are one application of these parts, not a universal
theme. Article excerpts remain exhibits rather than doors back to themselves.

Opening a specific article in site-wide mode preserves the discovered map and
loads that requested source through the browser. Its embedded video does not by
itself turn an article into a cinema. The parser retains up to four excerpts and
four illustrations within the existing twelve-item limit. It uses source text
and imagery, not generated factual content.

Article excerpts prefer distinct prose paragraphs over flattened page text.
Gallery controls, hidden duplicates, captions and credit/rights containers are
excluded from article prose in both the site reader and live browser extraction;
ordinary article text about copyright remains eligible. This selection does not
modify the live page or bypass source restrictions. Source URLs remain attached
to exhibits, and the original page retains its gallery notices and credits.
Older flattened site-cache excerpts are refreshed under the existing crawl
budgets while preserving node identities, placements and user overrides.

A parsed article can be projected before slow subresources finish. Readiness is
checked at most twenty times, one second apart, with cancellation on navigation
or hide. Image capture allows five 160 ms readiness checks per image within the
existing overall preparation deadline, then keeps a labeled placeholder.

### Civic spaces: headquarters, cinema and search cities

Spatial Web starts in the furnished Dockpipe Industries headquarters. Its
reception, street directory and **Search city** button open Dockpipe's native
search terminal in the main workspace. Enter a query and press **Explore search**;
Google runs behind the terminal and the result street replaces it when ready.
Escape cancels pending work and returns to the resident world. The address field
also accepts search terms or HTTP addresses. Search URLs bypass site crawling,
which intentionally excludes temporary search pages.
The HQ button returns here and cancels discovery, projection and playback.
The company logo is the actual company-site asset, bundled with provenance under
`resources/branding/`. No model or asset download is required for the hub.

Civic façades use full-height wings, windows, layered marquees, cornices and
illuminated trim. The cinema adds Art Deco ceiling ribs, luminous coves and
sixteen usable seats. These are cached geometry and bright materials, not dynamic
lighting or ray tracing. The busiest cinema uses 95 solids and 488 mesh vertices;
the twelve-result city uses 69 solids and 480 vertices, within existing limits.

The grand cinema keeps the existing single live video texture, eleven poster
slots, fullscreen playback and Live page round trip. Its auditorium, raised
balcony, lobby and city street are one resident scene. E/controller A sits in
marked seats; the same action or Back stands at the original clear position.
Balcony lifts provide an accessible route to the supported upper floor. The
street drone pad starts the existing flight controls; climbing above the city
returns to orbit, while Back returns to its clear boarding point. Flight retains the city ground rather than replacing it with a
starfield. Generated furnishings reuse the existing catalogue; user additions
remain in the separate furnishing overlay.

Recognized Google, Bing, DuckDuckGo and Yahoo search pages become a bounded city
of up to twelve ranked source destinations. Search chrome is filtered, supported
plain redirect targets are decoded, and opaque Google `/goto` links retain the
provider's original navigation URL rather than guessing a destination. The
visible source citation labels the storefront. Each result has a source poster
beside its door. Approaching doors uses the existing cancellable, capped corridor loader.
Consent, login or unavailable search results still require the live page; the
spatial interpreter does not bypass them. This is an authored city composition,
not a simulation of Manhattan or an unlimited generated city.

Civic rooms retain the 96-solid, 512-mesh-vertex, 256-mesh-face and twelve-texture
budgets. Transient actions are capped at 32 and raised walking surfaces at 16;
actions obey range and geometry occlusion. Geometry and windows are built once,
not during movement. Streaming disables local seat/lift actions until arrival.
`spatial-civic` covers bounds, door clearance, seating, occlusion, balcony support,
drone boarding/return and city-to-cinema passages; the browser fixture covers
search interpretation and continued theatre playback with real Chromium frames.

### Interior occlusion

Interior faces use a cached binary space partition for painter ordering. Large
walls, balcony floors and furniture are ordered by their separating planes rather
than centre-point distance, which previously let walls erase nearer seats at
oblique angles. Split fragments retain their original screen texture coordinates.
The same ordering supports the raster and accelerated Qt painter backends.

The cache rebuilds on resident geometry edits, not camera movement or texture
updates. Moving vehicles and furnishing previews traverse the existing partition.
Construction is limited to 48 levels, 16,384 fragments and two million face-plane
classifications; moving geometry allows 1,024 splits per frame. If construction
hits a limit, remaining leaf faces use distance sorting without dropping content.
The fallback is observable through the ordering model. Leaving the room releases
its partition. No new renderer, dependencies or continuous neural inference are
introduced. Civic tests cover oblique wall/exhibit visibility, crossing surfaces,
moving geometry, bounded fallback and cache cleanup.
