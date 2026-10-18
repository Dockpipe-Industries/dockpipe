# Furnishing your spaces

Enter a custom room, spatial website, operations campus, machine lab, or Git review
room and choose **Furnish**, press **F2**, or use **Start → Build / spawn an item**.
Exit a vehicle or release app control first. Furnishing is part of the
optional Cockpit package and does not add dependencies to the normal launcher.

Choose a category and item, then **Spawn**. Aim at a surface: the translucent object
turns green when placement is valid and red when blocked. Confirm with a click,
Enter, or controller A. Coordinates snap in ten-unit increments; rotation uses
15-degree steps. Wall items face away from their supporting surface. The HUD
explains invalid placement and shows the selected object and screen source.

After placement, aim and select to move, rotate, duplicate, remove, or configure an
object. Changes save automatically. Cancel abandons a preview without changing the
saved object. Leaving furnishing mode keeps the room and its content running.

## Controls

| Action | Keyboard / mouse | Controller |
| --- | --- | --- |
| Open / close | F2 or Furnish button | Start menu to open; View to finish |
| Move / aim | WASD / right-drag; pointer targets surfaces | Left / right stick |
| Browse categories | Category menu or Page Up / Down | LB / RB |
| Browse and preview items | Item menu or Up / Down | D-pad Up / Down |
| Spawn chosen item | Spawn button | Browse to an item with D-pad |
| Place / select at aim | Click or Enter / E | A |
| Rotate preview / pick up to rotate | R / Shift+R or Left / Right | D-pad Left / Right |
| Move selected | M | X |
| Cancel placement / selection / exit | Esc | B |
| Delete selected | Delete | Right stick click |
| Duplicate selected | Ctrl+D or Duplicate button | HUD button (mouse) |
| Configure selected screen | I or Configure screen input | Y |
| Choose input kind while configuring | Input menu | D-pad Up / Down |
| Choose scene source or application while configuring | Source menu | D-pad Left / Right |
| Apply input / cancel configuration | Apply input / Esc with scene focus | A / B |

Text and file paths use ordinary text fields. They currently require a keyboard;
there is no controller on-screen keyboard. Source selection, placement and editing
work through controller buttons. Mode-specific bindings take priority over flight
and voice commands. Controller commands have fixture coverage; physical-controller
feel still needs user testing.

## Catalog

The searchable spawn grid includes 56 procedural furnishings, 21 modular building
pieces and 120 imported Kenney models. Tiles preview their actual geometry; type
filters, search, keyboard and controller browsing share the same selection.

**Start → Build / spawn an item** opens a large centered thumbnail browser,
using the same catalogue and preview cache as the furnishing panel. D-pad or the
left stick moves across rows and columns; **LB / RB** filters by type, **A** selects
an item for placement, **B** returns to the command menu, and **Start** closes it.
Keyboard equivalents are arrows, Q / E, Enter, Esc and F1. Moving right browses
the next tile; it does not spawn an item. After selection, the normal placement
controls take over and controller movement must return to neutral before moving.
The browser remembers its category and selected model across menu visits, fits
inside the current view when resized, and generates at most six visible thumbnail
previews per event turn. Offscreen tiles are generated as they scroll into view.

The original procedural collection includes:

- **Seating:** simple chair, office chair, stool, bench, couch, lounge chair.
- **Tables:** small table, desk, long workbench, coffee table, side table, meeting table.
- **Storage:** small shelf, large shelf, cabinet, locker, crate, storage chest, tool cart,
  pedestal stand.
- **Lighting:** floor lamp, desk lamp, wall light, ceiling light, neon strip, accent beacon.
- **Nature:** small plant, large plant, hanging plant, tree planter.
- **Rugs:** small, large, round, runner.
- **Wall art:** poster frame, large poster, wall panel, banner, plaque, neon sign.
- **Screens:** wall screen, freestanding monitor, desk monitor, display board, cinema
  screen, holographic stand, arcade cabinet.
- **Props / tech:** bin, speaker, clock, trophy, sculpture, side console, terminal
  pedestal, small server rack, large server rack.

Lighting, clocks, servers, speakers and holographic stands are visual props in this
slice. They do not run simulations, emit dynamic light or start services. Screen
variants use the same source binding and interaction path.

## Seats and arcade cabinets

Leave furnishing mode, aim at a chair, stool, bench, couch or lounge chair, and
press **E / controller A** to sit. Look around normally; movement stays paused.
Press the action button again or **Back** to stand at your original clear position.
Seats share the room's 32-action budget, including existing cinema seats and lifts.

Choose **Screens → Arcade cabinet** to spawn a cabinet. Configure **Main screen**
with an Application or an existing app Scene surface. Configure **Marquee artwork**
separately with a local image or text. The surface selector, Page Up/Down, or LB/RB
switches between the two. Each cabinet consumes two screen slots; duplication and
saved layouts retain both independent bindings.

With App control enabled, approach the main screen and press **E / controller A**.
The camera moves into a fixed first-person playing position and the native app fits
inside the cabinet screen, leaving the cabinet and marquee visible. Click **Leave
arcade** or use controller Back to restore your standing position and the app's
desktop window. Keyboard input goes to the app during play. Other scene-source
providers retain their existing interactions. This uses the existing supported
native-app handoff and three-stream limit; it adds no emulator or game download.

## Screen inputs

| Input | Available behavior |
| --- | --- |
| Text / notice | Plain text rendered into a passive screen |
| Local file / image | Static image or UTF-8 text preview; apply again to refresh |
| Application | Choose a running or installed app independently for each screen; App control must be enabled |
| Scene surface | Share an existing environment source's pixels and activation, including theatre playback, Git code, website exhibits and legacy app screens |
| Web URL / terminal | Saved binding with an explicit unavailable placeholder; independent providers are not connected yet |

Scene surfaces reuse their producer and frame rather than launching another browser
or capturing another copy. Selecting one forwards to the existing source's action;
its normal behavior and permissions still apply. Sources are limited to the current
environment. Choose **Application** for the broader running/installed app catalog.
Each screen retains its own binding; ambiguous application windows are chosen when
connecting. App screens retain Cockpit's App control permission and three-live-app
budget. Selecting an installed app does not launch it without confirmation.

The X11 picker accepts both modern UTF-8 window titles and legacy `WM_NAME`
titles, including RetroArch. Reopen screen configuration to refresh running apps
after launching one or recreating its window.

Connected app streams follow a replacement window when an application recreates
its renderer, such as RetroArch loading a game. The cabinet keeps its playing
position and reconnects without another app launch. Discovery runs at the existing
500 ms stream cadence and stops after five seconds. A replacement must be the
only matching window from the original process and application class; the process
start time must also match. Windows already present when the stream connected,
other processes and ambiguous matches are not adopted. Failure returns control
with an explanation so you can select a window manually. Leaving the cabinet or
the room cancels control recovery; ordinary visible previews can still reconnect.
Restarting the application itself requires selecting it again.

On X11, Return to flight restores the selected application's desktop geometry,
decoration and sizing hints, window type, transient ownership, desktop and window
state. The native client stays unmapped while the temporary Qt wrapper is removed
and its saved hints are restored, so browsers with their own title bars do not
inherit an extra desktop frame. These bounded hints live only for the handoff;
the application window and its contents are not persisted.

Posters, framed art, wall panels, banners and signs use the same input controls.
Select the object, choose **Configure screen / artwork**, choose **Local file / image**,
then **Choose image or file…** and **Apply input**. Images retain their aspect ratio
within the frame. Each object's image path is saved separately; files must remain
available at that path. Static artwork needs no capture stream or browser.

The old **Add display** placement dialog is replaced by **Furnish room**. Existing
saved app screens and generated displays are adapted into selectable screen objects.
Their source identity survives moving, rebinding or removal. Legacy app-reference
editing remains available for compatibility. New terminal sessions and independent
browser instances are follow-up providers.

## Saved layers and bounds

The environment and user furnishings have separate ownership. `FurnishingLayout`
saves only IDs, types, transforms, screen bindings and removal records, using a
versioned atomic JSON file in the desktop configuration's `cockpit-furnishings`
directory. File names hash the environment key; website keys also hash the URL,
excluding credentials and fragments. File paths and screen text are user settings,
so they are stored locally in that layout. Pixels, native window IDs and application
window titles are not stored.

Custom rooms use their stable building ID. Websites use the complete page URL
(including its query), operations use workspace/section/page, and Git uses workspace
and review page. Reopening or rebuilding that environment restores its overlay.
Removed base displays remain removal records; temporarily absent source identities
retain their saved overrides. Procedural geometry can change underneath a saved
layout, so a changed world may require manually moving existing furnishings.

Writes reject concurrent changes instead of overwriting another instance's layout.
Re-enter the environment to reload. Malformed or oversized files remain untouched,
and furnishing is disabled with an explanation until the layout is repaired.

Budgets per environment:

- 192 saved records, including overrides/removals of base screens.
- 768 furnishing collision parts and 8,192 furnishing mesh faces; 24 active image/screen surfaces including existing scene displays.
- An 8 MiB mesh cache, independent of installed asset size; package meshes load on demand.
- 512 KiB maximum saved layout; 4,096 characters per source target or text.
- File previews: regular files up to 4 MiB; images up to 2048×2048 before scaling;
  UTF-8 reads limited to 64 KiB. Display previews are at most 512×320.
- A 33 ms targeting timer runs only while furnishing; geometry is rebuilt only when
  the preview or edit changes. No inference or network requests occur during placement.

Placement tests use oriented primitive collision boxes, room bounds, floor/wall/
ceiling normals and return-door clearance. Procedural meshes retain their existing
separate collision solids; this is not triangle-accurate editing or loose-body
physics. Excess saved objects are retained and hidden if a regenerated scene exceeds
its rendering budget. Imported furniture uses conservative box collision, not
triangle collision. Mesh colors come from source materials; this renderer does not
provide physically based textures or photorealistic lighting.

## Build a base

Open Custom workspaces, select a planet, choose **Add empty plot**, then **Furnish
room**. The saved plot is a 2,400 × 2,400 outdoor area with no generated building
shell. Existing rooms and their layouts are preserved.

Build categories provide habitat, sandstone and timber floor tiles, walls, open
doorways, window walls, roofs, pillars and railings. Pieces snap to a 100-unit grid
and rotate in 90-degree steps. Floor tiles are 200 units square. Place walls on
tile edges. Roofs start at height 310; aim at the ground to position them horizontally.
The bottom-height field supports automatic surface placement or a fixed height;
**H / left stick click** cycles useful floor and roof heights. Modular placement repeats until
Cancel. Doorways have real collision openings. This is single-level modular
construction; stairs, terrain editing and multi-floor traversal are not included.

## Shared model library

`SpatialAssetLibrary` loads renderer-independent, centered `SpatialMesh` models
with source diffuse colors. Both furnishing and future spatial-web layout rules
can use it without adding another renderer. The current web layouts are not
automatically replaced by this change.

The [Kenney Furniture Kit](https://kenney.nl/assets/furniture-kit) is CC0. Its
license and source/archive hash are retained under `assets/models/kenney-furniture`.
`tools/import_furniture.py` reproduces the package conversion from the official
archive without third-party Python dependencies. Building shells/stairs are omitted
in favor of collision-aware modular pieces. No remote model downloads occur during
walking, placement or page interpretation.

## Validation

`cockpit-test --furnishing` covers catalog and geometry bounds, ray targeting,
placement validity, mouse/keyboard events, controller commands, editing, input
bindings, independent cabinet surfaces, rotated seats, sit/stand, persistence, stale-writer rejection, scene regeneration and legacy screen
adaptation. Set `DOCKPIPE_FURNISHING_CAPTURE` to a temporary path prefix for placement
and compact input-panel captures. A native Qt desktop run exercises the GPU viewport;
the offscreen run exercises the raster fallback. Existing spatial cockpit and local
browser/theatre fixtures cover their source handoffs separately.
