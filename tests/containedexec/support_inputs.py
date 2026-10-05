"""Explicit installed inputs for tests that invoke tools outside the Go closure."""
import json
import os
from pathlib import Path

NATIVE_CPP_CASES = {'TestNativeStreamsGeneratedRuntime', 'TestNativeStreamCompositionValueTrace'}


class SupportInputs:
    def __init__(self, manifest=None):
        self.manifest = Path(manifest) if manifest else None
        self.paths, self.watch_directories, self.links = [], [], {}
        self.native_cxx = None
        if self.manifest is None:
            return
        if not self.manifest.is_absolute() or self.manifest.resolve() != self.manifest:
            raise ValueError('absolute canonical support manifest required')
        value = json.loads(self.manifest.read_text())
        if value.get('version') != 1 or not value.get('files'):
            raise ValueError('complete support input manifest required')
        self.paths = [self.manifest, *map(Path, value['files'])]
        for path in self.paths:
            if not path.is_absolute() or path.resolve() != path or not path.is_file():
                raise ValueError('canonical regular support input required: ' + str(path))
        self.watch_directories = sorted({path.parent for path in self.paths[1:]})
        for raw in value.get('watch_directories', []):
            path = Path(raw)
            if not path.is_absolute() or not path.is_dir() or path.resolve() != path:
                raise ValueError('canonical installed support directory required')
            self.watch_directories.append(path)
        self.links = value.get('links', {})
        for raw in self.links:
            path = Path(raw)
            if not path.is_absolute():
                raise ValueError('absolute support link required')
            self.watch_directories.append(path.parent.resolve())
        if value.get('native_cxx'):
            self.native_cxx = Path(value['native_cxx'])
            if self.native_cxx not in self.paths or not os.access(self.native_cxx, os.X_OK):
                raise ValueError('native compiler must be a declared executable input')
        self.check()

    def check(self):
        for raw, target in self.links.items():
            path = Path(raw)
            if not path.is_symlink() or os.readlink(path) != target:
                raise RuntimeError('installed support link changed: ' + raw)

    def native_environment(self, cases):
        if not any(case.split('/')[0] in NATIVE_CPP_CASES for case in cases):
            return []
        if self.native_cxx is None:
            raise RuntimeError('native C++ cases require --support-inputs with a declared native_cxx')
        # The manifest binds the installed compiler and its system tools. Do not
        # let a user tool shim select a different assembler or linker.
        return ['PIPELANG_TEST_CXX=' + str(self.native_cxx), 'PATH=/usr/bin:/bin']
