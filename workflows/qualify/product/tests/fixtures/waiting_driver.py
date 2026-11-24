"""Cancellation protocol fixture; produces no product acceptance receipt."""
from pathlib import Path
import argparse
import json
import os
import time


parser = argparse.ArgumentParser()
parser.add_argument("--request", required=True)
parser.add_argument("--output", type=Path, required=True)
arguments = parser.parse_args()
arguments.output.mkdir(mode=0o700, parents=True)
(arguments.output / "started.json").write_text(json.dumps({"pid": os.getpid()}))
while True:
    time.sleep(1)
