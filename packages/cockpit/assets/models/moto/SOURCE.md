# Moto bodywork

Nine selected surfaces from **Dirt motorcycle**, an original unbranded model by
**Innerscene**, released under **CC0**. Source reviewed 2026-10-16:

- Asset and licence: https://www.innerscene.com/tools/library/3d-parts/dirt-motorcycle-0a9c8589
- Original GLB: https://www.innerscene.com/api/library/dirt-motorcycle-0a9c8589/download
- Original size: 7,033,016 bytes.
- Original SHA-256: `830ad2dff8dde469af9390997fb1424ea6fd8b58874f2bf177159ed55f2a1441`.
- Licence: https://creativecommons.org/publicdomain/zero/1.0/

The original has 108,138 triangles across 815 meshes. DockPipe includes only
selected tank, seat, engine, cylinder, front mudguard, number panel, side panels,
and silencer geometry, simplified offline to **1,260 triangles**. Coordinates
are rotated into +Z forward, scaled from metres to centimetres, and aligned to
the existing 148 cm wheelbase. The trail-bike front fender is raised, flattened
and widened, the number plate and tank are resized for the motocross frame, and
the side panels are widened. Normals are recalculated after these edits;
materials are replaced with DockPipe's unbranded paint and metal colours.
No source textures, branding, executable code, GLB importer or external model
network requests are used at runtime. The remaining bike and rider are original
procedural geometry, articulated by the existing vehicle system.

Reproduce with Python, NumPy and `fast-simplification==0.1.12` (offline authoring
dependencies only):

```sh
python packages/cockpit/tools/import_moto.py downloaded-dirt-motorcycle.glb packages/cockpit/assets/models/moto
```

The recipe verifies the exact reviewed source checksum before processing.
Generated JSON uses the existing bounded SpatialAssetLibrary format with optional
smooth vertex normals. A missing asset falls back to procedural bodywork.
