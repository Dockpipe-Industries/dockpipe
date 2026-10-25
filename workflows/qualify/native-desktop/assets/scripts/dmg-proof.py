#!/usr/bin/env python3
"""Preserve the native DMG proof's existing three-minute deadline."""
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time


def run(command, deadline):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise TimeoutError('Native DMG qualification exceeded its three-minute deadline')
    process = subprocess.Popen(command, start_new_session=True)
    try:
        returncode = process.wait(timeout=remaining)
    except BaseException:
        # Terminate the whole disposable fixture group, including hdiutil, on
        # deadline or cancellation. Never leave the native child running.
        try:
            os.killpg(process.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait()
        raise
    if returncode:
        raise subprocess.CalledProcessError(returncode, command)


def cancelled(_signal, _frame):
    raise InterruptedError('Native DMG qualification cancelled')


def main():
    signal.signal(signal.SIGTERM, cancelled)
    deadline = time.monotonic() + 180
    run([sys.executable, '-m', 'unittest', 'discover', '-s', 'workflows/package/macos/tests',
         '-p', 'test_macos_dmg.py', '-v'], deadline)
    with tempfile.TemporaryDirectory(prefix='dockpipe-dmg-proof-', dir=os.environ['RUNNER_TEMP']) as temporary:
        root = Path(temporary)
        source = root / 'source'
        source.mkdir()
        (source / 'fixture.txt').write_text('Disposable Dockpipe DMG qualification fixture\n')
        image = root / 'fixture.dmg'
        run(['bash', 'workflows/package/macos/assets/scripts/create-dmg.sh', str(source), str(image)], deadline)
        run(['bash', 'workflows/package/macos/assets/scripts/verify-dmg.sh', str(image)], deadline)


if __name__ == '__main__':
    main()
