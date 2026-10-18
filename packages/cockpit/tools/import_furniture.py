"""Convert the official Kenney Furniture Kit OBJ/MTL archive to package meshes.

Usage: python3 tools/import_furniture.py /path/to/kenney_furniture-kit.zip
No network requests, external dependencies, or runtime import of arbitrary OBJ.
"""

import hashlib
import json
from pathlib import Path
import re
import sys
import zipfile


OUTPUT = Path(__file__).resolve().parents[1] / "assets/models/kenney-furniture"


def classify(name):
    if name.startswith(("wall", "floor", "doorway", "stairs", "paneling")):
        return None
    if name.startswith(("chair", "bench", "lounge", "stool")):
        family = "chair"
        if name.startswith("lounge"):
            family = "couch"
        elif name.startswith("stool"):
            family = "stool"
        return "Seating", family, "floor"
    if name.startswith(("table", "desk", "sideTable")):
        return "Tables", "table", "floor"
    if name.startswith(("kitchen", "hood", "toaster")):
        mount = "surface" if name.startswith(("toaster", "kitchenBlender", "kitchenCoffee", "kitchenMicrowave")) else "floor"
        if name.startswith(("hood", "kitchenCabinetUpper")):
            mount = "wall"
        return "Kitchen", "model", mount
    if name.startswith(("bath", "toilet", "shower", "washer", "dryer")):
        return "Bathroom", "model", "wall" if name == "bathroomMirror" else "floor"
    if name.startswith(("bed", "pillow")):
        return "Bedroom", "model", "surface" if name.startswith("pillow") else "floor"
    if name.startswith(("plant", "potted")):
        return "Nature", "model", "surface"
    if name.startswith(("lamp", "ceiling")):
        mount = "floor"
        if "Table" in name:
            mount = "surface"
        elif "Ceiling" in name or name == "ceilingFan":
            mount = "ceiling"
        elif "Wall" in name:
            mount = "wall"
        return "Lighting", "model", mount
    if name.startswith("rug"):
        return "Rugs", "rug", "floor"
    if name.startswith(("cabinet", "bookcase", "cardboard")):
        return "Storage", "model", "floor"
    if name.startswith(("computer", "laptop", "radio", "television", "speaker")):
        return "Tech", "model", "surface"
    return "Decor", "model", "surface" if name in ("bear", "books") else "floor"


def convert(archive, name):
    prefix = "Models/OBJ format/" + name
    colors = {}
    material = ""
    for line in archive.read(prefix + ".mtl").decode().splitlines():
        tokens = line.split()
        if not tokens:
            continue
        if tokens[0] == "newmtl":
            material = tokens[1]
        elif tokens[0] == "Kd":
            colors[material] = "#" + "".join(f"{round(float(value) * 255):02x}" for value in tokens[1:4])
    vertices = []
    faces = []
    face_colors = []
    for line in archive.read(prefix + ".obj").decode().splitlines():
        tokens = line.split()
        if not tokens:
            continue
        if tokens[0] == "v":
            vertices.append([float(value) * 250 for value in tokens[1:4]])
        elif tokens[0] == "usemtl":
            material = tokens[1]
        elif tokens[0] == "f":
            indices = [int(token.split("/")[0]) - 1 for token in tokens[1:]]
            if len(indices) not in (3, 4) or any(index < 0 or index >= len(vertices) for index in indices):
                raise ValueError("Unsupported OBJ face in " + name)
            faces.append(indices)
            face_colors.append(colors.get(material, "#b9c4cf"))
    low = [min(vertex[axis] for vertex in vertices) for axis in range(3)]
    high = [max(vertex[axis] for vertex in vertices) for axis in range(3)]
    size = [round(high[axis] - low[axis], 4) for axis in range(3)]
    for vertex in vertices:
        for axis in range(3):
            vertex[axis] = round(vertex[axis] - (low[axis] + high[axis]) / 2, 4)
    if len(vertices) > 4096 or len(faces) > 2048:
        raise ValueError("Mesh exceeds package limits: " + name)
    return {"vertices": vertices, "faces": faces, "colors": face_colors}, size


def main(path):
    OUTPUT.mkdir(parents=True, exist_ok=True)
    catalog = []
    with zipfile.ZipFile(path) as archive:
        for entry in sorted(archive.namelist()):
            if not entry.startswith("Models/OBJ format/") or not entry.endswith(".obj"):
                continue
            name = Path(entry).stem
            classification = classify(name)
            if classification is None:
                continue
            category, family, mount = classification
            mesh, size = convert(archive, name)
            asset = "kenney-" + name
            (OUTPUT / (asset + ".json")).write_text(json.dumps(mesh, separators=(",", ":")) + "\n")
            label = re.sub(r"([a-z])([A-Z0-9])", r"\1 \2", name).capitalize()
            catalog.append({"id": asset, "name": label, "category": category,
                            "family": family, "mount": mount, "size": size,
                            "faces": len(mesh["faces"])})
        (OUTPUT / "LICENSE.txt").write_bytes(archive.read("License.txt"))
    (OUTPUT / "catalog.json").write_text(json.dumps(catalog, indent=2) + "\n")
    digest = hashlib.sha256(Path(path).read_bytes()).hexdigest()
    (OUTPUT / "SOURCE.md").write_text(
        "# Kenney Furniture Kit\n\nSource: https://kenney.nl/assets/furniture-kit\n\n"
        "License: CC0; original license included in LICENSE.txt.\n\n"
        f"Original archive SHA-256: `{digest}`\n\n"
        "Converted by `tools/import_furniture.py`: centered OBJ vertices, original diffuse "
        "material colors, 250 scene units per source unit. Building shells and stairs are "
        "excluded in favor of DockPipe's collision-aware modular pieces.\n")
    resource = ['<RCC>', '  <qresource prefix="/cockpit/models">']
    for filename in sorted(OUTPUT.glob("*.json")):
        resource.append(f'    <file alias="{filename.name}">assets/models/kenney-furniture/{filename.name}</file>')
    for name in ("LICENSE.txt", "SOURCE.md"):
        resource.append(f'    <file alias="{name}">assets/models/kenney-furniture/{name}</file>')
    resource += ['  </qresource>', '</RCC>']
    (OUTPUT.parents[2] / "models.qrc").write_text("\n".join(resource) + "\n")
    print(f"Imported {len(catalog)} reusable models.")


if __name__ == "__main__":
    main(sys.argv[1])
