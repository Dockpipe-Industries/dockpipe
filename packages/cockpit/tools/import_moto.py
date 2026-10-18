#!/usr/bin/env python3
"""Prepare selected CC0 bike surfaces for the existing SpatialAssetLibrary.

Offline authoring only: requires numpy and fast-simplification==0.1.12.
No importer, GLB parser, Python or simplifier is shipped into the runtime.
The pinned source checksum deliberately makes this a reproducible asset recipe,
not a general-purpose importer for untrusted downloaded models.
"""

import argparse
import hashlib
import json
from pathlib import Path
import struct

import fast_simplification
import numpy as np


SOURCE_SHA256 = "830ad2dff8dde469af9390997fb1424ea6fd8b58874f2bf177159ed55f2a1441"
SURFACES = {
    "Original sculpted fuel tank": ("tank", 280, "#ed6837"),
    "Closed contoured rider and pillion saddle": ("seat", 180, "#202730"),
    "Cast engine crankcase": ("engine", 160, "#879094"),
    "Cylinder 1 closed jacket": ("cylinder", 80, "#7b858a"),
    "Finite formed front mudguard": ("fender", 240, "#ed6837"),
    "Closed unmarked front number panel": ("number", 120, "#efe3bf"),
    "Dirt bike shaped side panel 1": ("sideLeft", 20, "#efe3bf"),
    "Dirt bike shaped side panel -1": ("sideRight", 20, "#efe3bf"),
    "Closed tapered exhaust silencer": ("exhaust", 160, "#a9b2b7"),
}


def read_source(path):
    if path.stat().st_size > 12 * 1024 * 1024:
        raise ValueError("Source exceeds the 12 MiB authoring budget")
    data = path.read_bytes()
    if hashlib.sha256(data).hexdigest() != SOURCE_SHA256:
        raise ValueError("Source does not match the reviewed CC0 model")
    magic, version, size = struct.unpack_from("<4sII", data)
    if magic != b"glTF" or version != 2 or size != len(data):
        raise ValueError("Invalid GLB header")
    json_size, json_kind = struct.unpack_from("<II", data, 12)
    if json_kind != 0x4E4F534A:
        raise ValueError("Missing GLB JSON chunk")
    model = json.loads(data[20:20 + json_size])
    offset = 20 + json_size
    binary_size, binary_kind = struct.unpack_from("<II", data, offset)
    if binary_kind != 0x004E4942 or offset + 8 + binary_size != len(data):
        raise ValueError("Invalid GLB binary chunk")
    return model, data[offset + 8:]


def accessor(model, binary, index):
    item = model["accessors"][index]
    view = model["bufferViews"][item["bufferView"]]
    if item.get("sparse") or view.get("buffer", 0) != 0:
        raise ValueError("Unsupported source buffer")
    dtype = {5123: "<u2", 5125: "<u4", 5126: "<f4"}[item["componentType"]]
    columns = {"SCALAR": 1, "VEC3": 3}[item["type"]]
    offset = view.get("byteOffset", 0) + item.get("byteOffset", 0)
    stride = view.get("byteStride", np.dtype(dtype).itemsize * columns)
    return np.ndarray((item["count"], columns), dtype=dtype, buffer=binary,
        offset=offset, strides=(stride, np.dtype(dtype).itemsize)).copy()


def convert(model, binary, node, identifier, target, color):
    if any(key in node for key in ("matrix", "translation", "rotation", "scale", "children")):
        raise ValueError("Reviewed mesh must already contain world coordinates")
    primitives = model["meshes"][node["mesh"]]["primitives"]
    if len(primitives) != 1 or primitives[0].get("mode", 4) != 4:
        raise ValueError("Expected one triangle primitive")
    primitive = primitives[0]
    points = accessor(model, binary, primitive["attributes"]["POSITION"])
    triangles = accessor(model, binary, primitive["indices"]).reshape((-1, 3))
    # Weld duplicated material/UV seam vertices before offline simplification.
    points, mapping = np.unique(np.round(points, 6), axis=0, return_inverse=True)
    triangles = mapping[triangles]
    if len(triangles) > target:
        points, triangles = fast_simplification.simplify(points, triangles,
            target_count=target, agg=7)
    if len(triangles) > target:
        raise ValueError("Simplified model exceeds its face budget")
    # Source +X is forward, +Y is up, metres. Our bike uses +Z forward, cm,
    # with axle centres at +/-74 and a 35 cm rolling radius.
    points = np.column_stack((-points[:, 2] * 100,
        points[:, 1] * 100, points[:, 0] * 100 + 5.309073))
    # Adapt the source's trail-bike bodywork to the existing motocross frame.
    # Apply these shape edits before recomputing normals, not per frame.
    if identifier == "fender":
        points[:, 0] *= 1.7
        points[:, 1] = 94 + (points[:, 1] - 76) * .2
        points[:, 2] = 78 + (points[:, 2] - 78) * .85
    elif identifier == "number":
        points[:, 0] *= 1.3
        points[:, 1] = points[:, 1] * 1.2 - 10
        points[:, 2] += 12
    elif identifier == "tank":
        points[:, 0] *= 1.6
        points[:, 2] += 10
    elif identifier in ("sideLeft", "sideRight"):
        points[:, 0] *= 1.5
    face_normals = np.cross(points[triangles[:, 1]] - points[triangles[:, 0]],
        points[triangles[:, 2]] - points[triangles[:, 0]])
    lengths = np.linalg.norm(face_normals, axis=1)
    triangles = triangles[lengths > 1e-7]
    face_normals = face_normals[lengths > 1e-7]
    normals = np.zeros_like(points)
    for corner in range(3):
        np.add.at(normals, triangles[:, corner], face_normals)
    lengths = np.linalg.norm(normals, axis=1)
    if np.any(lengths < 1e-7):
        raise ValueError("Unreferenced or degenerate simplified vertex")
    normals /= lengths[:, None]
    if not np.isfinite(points).all() or np.max(np.abs(points)) > 250:
        raise ValueError("Unexpected bike dimensions")
    return {"vertices": np.round(points, 4).tolist(),
        "normals": np.round(normals, 6).tolist(),
        "faces": triangles.tolist(), "colors": [color] * len(triangles)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path)
    parser.add_argument("destination", type=Path)
    args = parser.parse_args()
    model, binary = read_source(args.source)
    nodes = {node.get("name"): node for node in model["nodes"]}
    args.destination.mkdir(parents=True, exist_ok=True)
    total = 0
    for name, (identifier, target, color) in SURFACES.items():
        mesh = convert(model, binary, nodes[name], identifier, target, color)
        output = args.destination / f"moto-{identifier}.json"
        output.write_text(json.dumps(mesh, separators=(",", ":")) + "\n")
        total += len(mesh["faces"])
        print(f"{output.name}: {len(mesh['vertices'])} vertices, {len(mesh['faces'])} faces")
    print(f"Total: {total} faces")


if __name__ == "__main__":
    main()
