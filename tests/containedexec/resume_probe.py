#!/usr/bin/env python3
"""Two tiny real units prove fresh-coordinator recovery without replay."""
import argparse
import json
import os
from pathlib import Path
import sys
from campaign import Campaign
from verification import StageRunner
from job import verify_job


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--phase', choices=['crash', 'resume'], required=True)
    args = parser.parse_args()
    verify_job()
    args.root.mkdir(parents=True, exist_ok=True, mode=0o700)
    with Campaign(args.root / 'campaign', 'fresh' if args.phase == 'crash' else 'resume') as campaign:
        stage = StageRunner(campaign, 'synthetic', 'fixed-inputs', ['one', 'two'], args.root / 'cache', lambda: 'fixed-inputs')
        for case in ['one', 'two']:
            path = args.root / (case + '.count')
            command = [sys.executable, '-B', '-c', 'import pathlib,sys; p=pathlib.Path(sys.argv[1]); p.open("a").write("executed\\n"); print("passed")', str(path)]
            result = stage.run([case], command)
            if result['exit']:
                return 1
            if args.phase == 'crash':
                os._exit(19)
        result = stage.finish()
        if any((args.root / (case + '.count')).read_text() != 'executed\n' for case in ['one', 'two']):
            raise RuntimeError('completed unit replayed')
        print(json.dumps(result))
    return 0


if __name__ == '__main__': sys.exit(main())
