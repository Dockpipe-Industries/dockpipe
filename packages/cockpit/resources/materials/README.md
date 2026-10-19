# Motocross surface textures

These photographs are bundled with the optional Cockpit package. Runtime use is
entirely offline. The painter fallback uses 512-pixel tiles and four smaller mip
levels. The native renderer shares 1024-pixel sources, GPU mipmaps and bounded
anisotropic filtering. Both normalize luminance for scene tinting while
preserving photographic colour variation. No remote downloads occur while riding.

The dirt/rock assets are by **Rob Tuytel**, sparse grass by **Amal Kumar**, leafy
grass by **Charlotte Baglioni**, and sky by **Greg Zaal** with sky edits by
**Jarod Guest**,
distributed by **Poly Haven** under **CC0**:
https://polyhaven.com/license

| File | Source | Original download | SHA-256 |
| --- | --- | --- | --- |
| dirt.jpg | https://polyhaven.com/a/brown_mud_dry | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/brown_mud_dry/brown_mud_dry_diff_1k.jpg | a56281b644de614787340f3db45e28d2f72ec32fc19a442f99002bae0ab11398 |
| rock.jpg | https://polyhaven.com/a/rock_01 | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/rock_01/rock_01_diff_1k.jpg | 0b5f0418eb1e686190c5d33bb54c400b1f9ed58813b5e34aa5e573a5197302b5 |
| grass.jpg | https://polyhaven.com/a/sparse_grass | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/sparse_grass/sparse_grass_diff_1k.jpg | ae94f2b34597b9108eefd88217f55eccaec6d6b382e858a478ee92df90e66617 |
| leafy-grass.jpg | https://polyhaven.com/a/leafy_grass | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/leafy_grass/leafy_grass_diff_1k.jpg | cfa40bc9d9417d1852db8753a8d5917f110c40101179f63543c382e39bc05e4a |
| cloudy-sky.jpg | https://polyhaven.com/a/kloofendal_48d_partly_cloudy_puresky | https://dl.polyhaven.org/file/ph-assets/HDRIs/extra/Tonemapped%20JPG/kloofendal_48d_partly_cloudy_puresky.jpg | 2bdd7abe32f1d2f7a626e0a6e45bb808313f29bd41d758eacbdba91560e93f04 |
| dirt-normal.jpg | https://polyhaven.com/a/brown_mud_dry | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/brown_mud_dry/brown_mud_dry_nor_gl_1k.jpg | 4dd82ecb3e810d4de49d34c7d6cc30152129434a58b3bd786bc9952bc8874877 |
| dirt-roughness.jpg | https://polyhaven.com/a/brown_mud_dry | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/brown_mud_dry/brown_mud_dry_rough_1k.jpg | 31fb99629f533924b1150a7dac8d9432de85450bb45cda5033e3eb723f8dda04 |
| rock-normal.jpg | https://polyhaven.com/a/rock_01 | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/rock_01/rock_01_nor_gl_1k.jpg | 7a7146f51159a7667233c2c14c34ff18a9f4374cc8869d53a1162648d0941023 |
| rock-roughness.jpg | https://polyhaven.com/a/rock_01 | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/rock_01/rock_01_rough_1k.jpg | e98b9c8ffb35513bd7b36a8730bf9745d05325376a85fae87ebf56a7c071a123 |
| leafy-grass-normal.jpg | https://polyhaven.com/a/leafy_grass | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/leafy_grass/leafy_grass_nor_gl_1k.jpg | 832328216adc0a7e1f70a31d5ee48c9ab7f2152d83816122736cf42ac4b2ebd6 |
| leafy-grass-roughness.jpg | https://polyhaven.com/a/leafy_grass | https://dl.polyhaven.org/file/ph-assets/Textures/jpg/1k/leafy_grass/leafy_grass_rough_1k.jpg | 0c2a1d91aacb8e54a496051b59854e51e456fe858abeade778e308359ef16463 |

Source JPEGs are unchanged, with download size and MD5 checked against Poly
Haven's asset API. Surface tiles are 1024 × 1024. The 8192 × 4096 sky panorama is
decoded at 2048 × 1024 using JPEG decoder scaling; the full-size bitmap is never
allocated. Original three surface files total 2,564,484 bytes; leafy grass is
1,195,394 bytes and the sky is 22,162,150 bytes. This is a tonemapped colour sky,
not a claim of HDR image-based lighting.
The rider, wheels and frame remain original procedural work. Selected bike
bodywork comes from a separately documented [CC0 Innerscene model](../../assets/models/moto/SOURCE.md).
Rubber and fabric material patterns are original procedural work.

The six normal/roughness source JPEGs add 5,089,427 bytes. Normal RGB and
roughness alpha are packed once into three shared, linear 1024-square CPU tiles,
then uploaded through the existing bounded GPU cache. The images are not
premultiplied and roughness is not opacity. No displaced geometry is generated.

## Juniper scenery

`juniper.png` is original AI-generated transparent scenery, created for this
project with the built-in image generation tool on 2026-10-16. It is decorative
art, not a factual reconstruction or third-party game asset. SHA-256:
`3c19abfa9fe7d68f8c03830f91f0bb35c75e253edc224f2cd5ebc023fa3e1b3e`.
The unchanged PNG is bundled offline and shared across all foliage instances.
The painter fallback uses a shared alpha silhouette for its ground projection;
native rendering casts cutout shadows onto actual geometry. No inference runs
while playing.

## Instanced pine scenery

Native Moto foliage uses original procedural trunk/branch geometry with CC0
photographs from [Poly Haven Pine Tree 01](https://polyhaven.com/a/pine_tree_01)
by Rico Cilliers (modeling) and Rob Tuytel (photography). The large source model
is not bundled. These unchanged 1024-square source textures total 1,588,125 bytes;
download sizes and MD5 were checked against the Poly Haven asset API.

| File | Original download | SHA-256 |
| --- | --- | --- |
| pine-bark.jpg | https://dl.polyhaven.org/file/ph-assets/Models/jpg/1k/pine_tree_01/pine_tree_01_bark_diff_1k.jpg | 5c76836b01c3536fa0e24207b67bf4f3315b53ebe2b4ecc6f08dfcf3f8d74929 |
| pine-twig.jpg | https://dl.polyhaven.org/file/ph-assets/Models/jpg/1k/pine_tree_01/pine_tree_01_twig_diff_1k.jpg | e649f0f9bc74d1af5f1a45eaf2fe7a29adaaddfab251f746b983da301e4b80e1 |
| pine-alpha.png | https://dl.polyhaven.org/file/ph-assets/Models/png/1k/pine_tree_01/pine_tree_01_twig_alpha_1k.png | 6f91aa99f2c27108d6f817f874954aa7df8bfbb4dea2673e9322555a6ae79d99 |

The upper-left needle cluster is cropped and combined with its opacity mask
once at runtime. Opaque neighboring UV islands connected to the crop boundary
are removed with a bounded flood fill, retaining the isolated needle silhouette.
A single shared mesh contains 2,512 triangles with two shared
images. Individual trees supply only position, scale, heading and shade to GPU
instancing. Branch foliage uses fixed intersecting cutout sheets, not a full
photogrammetry tree or camera-facing whole-tree card. The existing painter
fallback retains juniper cards. Placement still uses the admitted canopy bounds.
