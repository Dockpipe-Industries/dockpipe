# Private qualification VM images

The cache stores content-addressed chunks of clean, standalone QCOW2 base images
and immutable snapshot manifests. The upload process uses Dockpipe's
[pinned R2 bulk uploader](../../publish/r2/README.md)
with an Object Read & Write credential scoped to the qualification bucket. The
runner uses a separate Object Read credential scoped to that same bucket.
Keep credentials and the account endpoint in owner-only local files.

The bucket uses R2 Standard with the managed public endpoint disabled and no
custom domains. It is managed by the existing `dockpipe.cloudflare.r2infra`
Terraform module with a dedicated remote state key. The release, staging and
Terraform state buckets have their own access scopes. Keep one coordinated
writer for the qualification bucket's immutable keys.

## Add an image

1. Select a clean upstream image or a reproducible build from a named release.
   Verify the vendor checksum or signature before preparing the upload. For
   Ubuntu 24.04, reuse the VM package's image profile and
   `manifests/linux-qualification.json` URL and SHA-256. Inspect the base with
   `qemu-img info` and keep all backing files, live disks, credentials and
   provisioning inputs out of the upload.
2. Start from a prior snapshot manifest or create a `dockpipe.qualification-image.v1`
   JSON source profile. Record the name,
   OS ID, release, release stamp, architecture, upstream URL, preparation recipe,
   verification source, byte count and SHA-256. Include the firmware, machine
   type, CPU count, memory, disk bus, datasource and network policy. The Linux
   pilot fixture admits amd64 QCOW2 images with Q35/KVM, virtio, NoCloud and no
   guest network interface.
3. Run `fixtures/image-cache/prepare.py --image PROFILE READONLY_BASE --output
   PRIVATE_STAGING`. The script writes an ordered snapshot manifest and a bulk
   upload plan for its 16 MiB chunks. Each stored chunk has an immutable
   `chunks/sha256/<stored-digest>` key. Snapshot manifests use
   `manifests/sha256/<metadata-digest>.json` and publish after their chunks.
   Run `dockpipe.cloudflare.r2bulk` with the private bucket, local uploader
   profile, pinned SDK, and a journal dedicated to that plan. Keep the verified
   HEAD receipt and the snapshot manifest SHA-256.
4. Fetch and verify from an empty private runner cache. Use the boot workflow
   below with the returned paths and a fresh, local output directory. The
   receipt requires a guest write and clean shutdown, overlay growth and an
   unchanged SHA-256 for the backing image.

```bash
python3 workflows/qualify/product/fixtures/image-cache/download.py \
  --reader "$READER_CREDENTIALS_FILE" \
  --endpoint "$R2_ENDPOINT_FILE" \
  --bucket "$QUALIFICATION_BUCKET" \
  --manifest-sha256 "$MANIFEST_SHA256" \
  --cache "$PRIVATE_CACHE"

"$DOCKPIPE_BIN" --workflow-file \
  "$PWD/workflows/qualify/product/fixtures/image-cache/config.yml" \
  --workdir "$PWD" -- \
  --manifest "$VERIFIED_MANIFEST" --base "$VERIFIED_BASE" --output "$FRESH_RUN"
```

The downloader expects a local AWS credentials file with the
`qualification-reader` profile and an endpoint file containing the HTTPS R2
origin. Both files must be owner-only. The downloader verifies stored chunks,
their decoded bytes and the complete reconstructed image. The boot fixture
attaches its own read-only NoCloud seed
and creates a writable overlay in the run directory. Keep that output local.

## Compression and incremental reuse

On one local runner, the Alpine pilot used 11 R2 reads in 6.80 seconds for a
cold restore, zero reads in 0.34 seconds for a warm SHA-256 check, and one read
in 2.75 seconds to reconstruct with one missing chunk. All three produced the
same base digest. These timings include local verification.

Zstandard level 3 is the default. The Ubuntu and Alpine comparison produced
701.3 MB in 2.8 seconds with Zstandard and 706.7 MB in 17.6 seconds with gzip.
Preparation also accepts `--codec gzip` or `--codec identity`. A chunk that
does not get smaller is stored byte-for-byte. Zstandard restore requires
`zstd` and GNU `timeout`.
Each manifest entry records the codec, lengths and SHA-256 of both the stored
and decoded bytes. With the same encoded representation, an unchanged chunk can be reused by
another snapshot without another upload. Chunking follows QCOW2 file offsets,
so an image repack can change file layout and reduce reuse.

Add a codec by registering a stable ID and bounded encoder and decoder in
`chunk_codecs.py`. Keep existing codec IDs and decoders for saved snapshots.
Nucleon can use that interface when its chunk encoder and decoder are available.

## Pilots

| Image | Source | Clean base size | R2 restore and overlay boot |
| --- | --- | --- | --- |
| Ubuntu Server 24.04 LTS amd64, build 20260801 | [Canonical cloud image](https://cloud-images.ubuntu.com/releases/noble/release-20260801/) | 624,239,616 bytes | Passed with 38 chunks |
| Alpine 3.24.4 amd64, cloud-init r0 | [Alpine cloud images](https://alpinelinux.org/cloud/) | 158,007,296 bytes | Passed with 10 chunks |

The two compressed pilots and their metadata total about 0.701 GB. That is about $0.011
per month at the listed Standard storage rate before Cloudflare's account-level
free allowance and rounding. See [R2 pricing](https://developers.cloudflare.com/r2/pricing/).

Windows Server remains the final manual lane in the
[platform matrix](PLATFORM-MATRIX.md). Before admitting any Windows image, record
the edition, acquisition channel and specific license terms covering a cached
copy and use by the intended runners. Keep Windows images outside the cache
until those terms have been checked for the selected image.
Microsoft's evaluation download expires after 180 days and requires activation
within ten days. Review the license supplied with the exact image and the
intended runner use before caching it. [Microsoft Evaluation Center](https://www.microsoft.com/en-us/evalcenter/download-windows-server-2025).
