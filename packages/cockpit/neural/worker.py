"""Local ONNX scene interpretation and foreground extraction; JSON lines only.

No browser, network, filesystem paths or executable instructions come from pages.
The UI owns cancellation/deadlines. One request runs at a time, with two CPU threads,
12 items, 256 tokens per text, and one 512x320 image. Caches are session-only so the
private browser's page text and pixels never persist to disk.
"""
import argparse
import base64
import contextlib
from collections import OrderedDict
import hashlib
import io
import json
from pathlib import Path
import sys
import time

MAX_LINE = 1024 * 1024
# Independent construction affordances, not whole-world templates. The encoder
# measures each item as well as the page, allowing different districts to coexist.
FEATURES = (
    "nature woodland hiking camping outdoor equipment trees trails wilderness",
    "medieval fantasy books dragons knights castles magic stone towers",
    "astronomy spacecraft planets rockets satellites science fiction space exploration",
    "books literature documentation research reference reading library shelves archives",
    "shopping fashion products retail market department store boutiques displays",
    "engineering robotics machines software tools laboratory workshop technology",
    "cars trucks vehicles motorsport racing driving dealership test track",
    "amusement park carnival rides games entertainment colourful festival pavilions",
)

ANIMALS = (
    ("spider", "spiders tarantulas arachnids eight legs"),
    ("crab", "crabs crustaceans claws shell sideways legs"),
    ("insect", "insects beetles cicadas ants six legs segmented body"),
    ("butterfly", "butterflies moths wide patterned wings"),
    ("worm", "earthworms long segmented limbless body"),
    ("fish", "fish sharks aquatic body fins tail"),
    ("bird", "birds feathers beak wings"),
    ("quadruped", "mammals four legged land animals fur"),
    ("cephalopod", "octopus squid cephalopods mantle tentacles"),
)


def animal_form(scores):
    """Low confidence leaves the source photo alone; never invent anatomy."""
    import math
    if len(scores) != len(ANIMALS) or not all(math.isfinite(float(score)) for score in scores):
        return ""
    ranked = sorted(range(len(scores)), key=lambda index: scores[index], reverse=True)
    if scores[ranked[0]] < 0.4 or scores[ranked[0]] - scores[ranked[1]] < 0.06:
        return ""
    return ANIMALS[ranked[0]][0]


def remember(cache, key, value, capacity):
    cache[key] = value
    cache.move_to_end(key)
    while len(cache) > capacity:
        cache.popitem(last=False)
    return value


def text(value, limit):
    if not isinstance(value, str) or len(value) > limit:
        raise ValueError("Text exceeds the scene input contract")
    return value


class Inference:
    def __init__(self, root, prefer_gpu):
        import numpy as np
        import onnxruntime as ort
        from tokenizers import Tokenizer
        self.np = np
        self.root = root
        self.ort = ort
        self.options = ort.SessionOptions()
        self.options.intra_op_num_threads = 2
        self.options.inter_op_num_threads = 1
        self.options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
        if prefer_gpu and hasattr(ort, "preload_dlls"):
            with contextlib.redirect_stdout(sys.stderr):
                ort.preload_dlls(directory="")
        self.providers = ["CPUExecutionProvider"]
        available = ort.get_available_providers()
        if prefer_gpu:
            if "CUDAExecutionProvider" in available:
                self.providers.insert(0, ("CUDAExecutionProvider", {
                    "gpu_mem_limit": 512 * 1024 * 1024,
                    "arena_extend_strategy": "kSameAsRequested",
                    "cudnn_conv_algo_search": "HEURISTIC",
                }))
            elif "CoreMLExecutionProvider" in available:
                self.providers.insert(0, "CoreMLExecutionProvider")
        self.semantic = self.session("semantic.onnx")
        self.cutout = None
        self.tokenizer = Tokenizer.from_file(str(root / "tokenizer.json"))
        self.tokenizer.enable_truncation(max_length=256)
        self.tokenizer.enable_padding()
        self.references = self.encode(FEATURES)
        self.animal_references = None
        self.plans = OrderedDict()
        self.images = OrderedDict()

    def session(self, name):
        try:
            with contextlib.redirect_stdout(sys.stderr):
                return self.ort.InferenceSession(str(self.root / name), self.options, providers=self.providers)
        except Exception:
            # GPU library/operator support varies. A failed accelerated session
            # falls back once to the same model on the bounded CPU lane.
            return self.ort.InferenceSession(str(self.root / name), self.options,
                                            providers=["CPUExecutionProvider"])

    def encode(self, texts):
        np = self.np
        tokens = self.tokenizer.encode_batch(list(texts))
        values = {
            "input_ids": np.asarray([token.ids for token in tokens], dtype=np.int64),
            "attention_mask": np.asarray([token.attention_mask for token in tokens], dtype=np.int64),
            "token_type_ids": np.asarray([token.type_ids for token in tokens], dtype=np.int64),
        }
        outputs = self.semantic.run(None, {item.name: values[item.name] for item in self.semantic.get_inputs()})
        mask = values["attention_mask"][:, :, None]
        embeddings = (outputs[0] * mask).sum(axis=1) / np.maximum(mask.sum(axis=1), 1)
        return embeddings / np.maximum(np.linalg.norm(embeddings, axis=1, keepdims=True), 1e-9)

    def plan(self, request):
        title = text(request.get("title", ""), 180)
        evidence = text(request.get("evidence", ""), 2200)
        items = request.get("items")
        if not isinstance(items, list) or not 1 <= len(items) <= 12:
            raise ValueError("Scene must contain between one and twelve destinations")
        sentences = [title + ". " + evidence]
        ids = []
        animal_requests = []
        for item in items:
            item_id = text(item.get("id"), 2)
            if not item_id.isdigit() or not 0 <= int(item_id) < 12 or item_id in ids:
                raise ValueError("Invalid destination identity")
            ids.append(item_id)
            animal_requests.append(item.get("animal") is True)
            sentences.append(". ".join(text(item.get(field, ""), limit) for field, limit in
                                       (("title", 140), ("category", 60), ("description", 200))))
        identity = hashlib.sha256(json.dumps([sentences, ids, animal_requests]).encode()).digest()
        if identity in self.plans:
            return {**self.plans[identity], "cached": True}
        embeddings = self.encode(sentences)
        similarities = embeddings @ self.references.T
        # Cosine relevance remains independent per feature, rather than a softmax
        # forcing every site into exactly one theme. Low confidence stays neutral.
        weights = self.np.clip((similarities - 0.18) / 0.42, 0, 1)
        result = {
            "features": weights[0].round(4).tolist(),
            "items": [{"id": item_id, "features": weights[index + 1].round(4).tolist()}
                      for index, item_id in enumerate(ids)],
            "provider": self.semantic.get_providers()[0],
            "cached": False,
        }
        if any(animal_requests):
            if self.animal_references is None:
                self.animal_references = self.encode([entry[1] for entry in ANIMALS])
            animal_scores = embeddings[1:] @ self.animal_references.T
            for index, requested in enumerate(animal_requests):
                if requested:
                    result["items"][index]["animalForm"] = animal_form(animal_scores[index])
        return remember(self.plans, identity, result, 24)

    def mask(self, request):
        from PIL import Image
        np = self.np
        encoded = text(request.get("image"), 900000)
        source = base64.b64decode(encoded, validate=True)
        identity = hashlib.sha256(source).digest()
        if identity in self.images:
            return {**self.images[identity], "cached": True}
        image = Image.open(io.BytesIO(source))
        if image.format != "PNG" or image.width > 512 or image.height > 320 or min(image.size) < 8:
            raise ValueError("Image exceeds the 512x320 PNG contract")
        image = image.convert("RGBA")
        if self.cutout is None:
            self.cutout = self.session("cutout.onnx")
        pixels = np.asarray(image.convert("RGB").resize((320, 320)), dtype=np.float32)
        pixels /= max(float(pixels.max()), 1)
        pixels = (pixels - np.array([0.485, 0.456, 0.406], dtype=np.float32)) / np.array(
            [0.229, 0.224, 0.225], dtype=np.float32)
        prediction = self.cutout.run(None, {self.cutout.get_inputs()[0].name:
            pixels.transpose(2, 0, 1)[None, ...]})[0][0, 0]
        minimum, maximum = float(prediction.min()), float(prediction.max())
        if not np.isfinite(prediction).all() or maximum - minimum < 1e-6:
            return {"accepted": False}
        mask = (prediction - minimum) / (maximum - minimum)
        coverage = float((mask > 0.5).mean())
        # A tiny/no foreground or near-solid mask usually means an unsuitable
        # product image. Keep the original instead of erasing its content.
        if coverage < 0.04 or coverage > 0.94:
            return {"accepted": False}
        alpha = Image.fromarray((mask * 255).astype("uint8")).resize(image.size, Image.Resampling.LANCZOS)
        alpha = np.minimum(np.asarray(alpha), np.asarray(image.getchannel("A")))
        image.putalpha(Image.fromarray(alpha))
        bounds = Image.fromarray((alpha > 24).astype("uint8") * 255).getbbox()
        if bounds:
            left, top, right, bottom = bounds
            image = image.crop((max(0, left - 4), max(0, top - 4),
                                min(image.width, right + 4), min(image.height, bottom + 4)))
        output = io.BytesIO()
        image.save(output, format="PNG")
        result = {"accepted": True, "image": base64.b64encode(output.getvalue()).decode(),
                  "provider": self.cutout.get_providers()[0], "cached": False}
        # At most sixteen bounded RGBA images, not an unbounded browser cache.
        return remember(self.images, identity, result, 16)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--models", type=Path, required=True)
    parser.add_argument("--gpu", action="store_true")
    args = parser.parse_args()
    inference = Inference(args.models, args.gpu)
    for _ in range(4096):  # Parent rotates the helper after this session budget.
        line = sys.stdin.buffer.readline(MAX_LINE + 1)
        if not line:
            return
        if len(line) > MAX_LINE or not line.endswith(b"\n"):
            raise ValueError("Request exceeds protocol budget")
        request = json.loads(line)
        started = time.monotonic()
        try:
            if request.get("operation") == "plan":
                result = inference.plan(request)
            elif request.get("operation") == "cutout":
                result = inference.mask(request)
            else:
                raise ValueError("Unknown inference operation")
            reply = {"success": True, "result": result}
        except (ValueError, TypeError, KeyError, OSError) as error:
            reply = {"success": False, "error": str(error)[:180]}
        reply["milliseconds"] = round((time.monotonic() - started) * 1000)
        print(json.dumps(reply, allow_nan=False), flush=True)


if __name__ == "__main__":
    main()
