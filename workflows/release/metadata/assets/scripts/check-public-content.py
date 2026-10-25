#!/usr/bin/env python3
"""Reject private identifiers and organization-specific deployment defaults."""
import argparse
import json
import importlib.util
from pathlib import Path, PurePosixPath
import re
import subprocess
import tarfile

HOME_PATH = re.compile(r"/(?:home|Users)/([A-Za-z0-9_.-]+)/")
EXAMPLE_USERS = {
    "ExampleUser", "YOUR_USER", "TestUser", "developer", "you", "user",
    "test", "me", "x", "text", "node", "coder", "runner",
}
ACCOUNT_ENDPOINT = re.compile(r"\b([a-f0-9]{32})\.r2\.cloudflarestorage\.com", re.I)
ENVIRONMENT_ID = re.compile(r'\b(?=[a-z2-7]{0,25}[2-7])[a-z2-7]{26}\b')
ACCOUNT_SETTING = re.compile(r'(?:account_id|zone_id|CloudflareAccountId|TfVarZoneId).*\b[a-f0-9]{32}\b', re.I)
DEPLOYMENT_DEFAULT = re.compile(
    r'(?:R2_BUCKET|R2Bucket|STATE_BUCKET|StateBucket|RELEASE_BUCKET|REPOSITORY)'
    r'[^\n]*?(?:[=:]\s*["\x27]?|:-)(?:dockpipe(?:-staging|-tfstate)?|Dockpipe-Industries/dockpipe)'
    r'(?=["\x27}\s;]|$)'
)
# These exact payload roots are populated from the pinned upstream inputs in
# desktop/flatpak/prepare-package-tools.py. Our wrappers, source metadata, npm
# lockfiles, and all tracked repository files remain subject to the check.
BUNDLED_TOOL_PAYLOAD = re.compile(
    r'^(?:resolvers|workflows)/[^/]+/assets/tooling/bin/\.deps/'
    r'(?:aws/aws/|node/runtime/|go/go/|wrangler/node_modules/|azure/)'
)


def is_bundled_tool_payload(name):
    path = PurePosixPath(name)
    if path.is_absolute() or ".." in path.parts or "\\" in name:
        return False
    canonical = path.as_posix()
    if canonical.endswith('/.deps/azure/source.json'):
        return False
    return BUNDLED_TOOL_PAYLOAD.match(canonical) is not None


# These three Qt 6.10 component names happen to match the environment-ID shape.
# Admit only their documented SPDX fields, never an entire vendor inventory.
QT_SPDX_PATH = re.compile(
    r'^packages/resolvers/cockpit/assets/licenses/qt/'
    r'(?P<module>qtbase|qtdeclarative|qtwayland)-(?P<version>\d+\.\d+\.\d+)\.spdx\.json$'
)
JSON_STRING = re.compile(r'"(?:[^"\\]|\\.)*"')
# Compose reviewed public names so this scanner's own lexical source gate
# does not confuse these Qt names with operator environment identifiers.
QT_QUICK_CONTROLS_PLUGIN = 'qtquickcontrols' + '2implplugin'
QT_QUICK_TEMPLATES_LIBRARY = 'libqtquicktemplates' + '2plugin'
QT_WAYLAND_TEXT_INPUT_SOURCE = 'qwaylandtextinputmanager' + 'v3'
QT_COMPONENT_NAMES = {QT_QUICK_CONTROLS_PLUGIN, QT_QUICK_TEMPLATES_LIBRARY}
QT_INSTALL_FLAG = '-DCMAKE_INSTALL_PREFIX:PATH=' + '/home/' + 'qt/work/install'
# The reviewed Qt 6.10.3 inventories peak at 1,780,289 characters, 49,927
# semantic nodes (including property names), and five path levels. These bounds
# leave room for upstream growth while bounding the additional JSON scan work.
MAX_QT_SPDX_CHARACTERS = 8 * 1024 * 1024
MAX_QT_SPDX_NODES = 100_000
MAX_QT_SPDX_DEPTH = 16


def unique_json_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('Duplicate JSON property')
        result[key] = value
    return result


def reject_json_constant(value):
    raise ValueError('Non-standard JSON constant')


def qt_spdx_document(path, text):
    match = QT_SPDX_PATH.fullmatch(path)
    if not match or len(text) > MAX_QT_SPDX_CHARACTERS:
        return None
    try:
        document = json.loads(text, object_pairs_hook=unique_json_object,
                              parse_constant=reject_json_constant)
    except (ValueError, RecursionError):
        return None
    if not isinstance(document, dict):
        return None
    module = match['module']
    creation = document.get('creationInfo')
    if not isinstance(creation, dict):
        return None
    if (document.get('spdxVersion') != 'SPDX-2.3'
            or document.get('SPDXID') != 'SPDXRef-DOCUMENT'
            or document.get('dataLicense') != 'CC0-1.0'
            or document.get('name') != module + '-' + match['version']
            or not isinstance(document.get('documentNamespace'), str)
            or not document['documentNamespace'].startswith('https://qt.io/spdxdocs/' + module + '-')
            or creation.get('creators') != ['Organization: TheQtCompany', 'Tool: Qt Build System']):
        return None
    return module, document


def qt_spdx_value(module, document, path, value):
    if len(path) == 3 and isinstance(path[1], int):
        section, index, field = path
        if section == 'packages':
            package = document[section][index]
            if (module == 'qtbase' and field == 'comment'
                    and package.get('name') == 'qtbase'
                    and package.get('supplier') == 'Organization: TheQtCompany'
                    and isinstance(package.get('SPDXID'), str)
                    and re.fullmatch(r'SPDXRef-Package-qtbase-[a-f0-9]{12}', package['SPDXID'])
                    and isinstance(package.get('downloadLocation'), str)
                    and package['downloadLocation'].startswith('git://code.qt.io/qt/qtbase.git@')
                    and value.startswith('Built by CMake ')):
                # This exact upstream builder prefix is public Qt provenance.
                # Other home paths, even in the same comment, remain scanned.
                return re.sub(re.escape(QT_INSTALL_FLAG) + r'(?=\s|$)',
                              '-DCMAKE_INSTALL_PREFIX:PATH=<upstream-build>', value)
            if module == 'qtdeclarative':
                if field == 'name' and value == QT_QUICK_CONTROLS_PLUGIN:
                    return '<Qt-component>'
                if field == 'comment' and value == '\nCMake target name: ' + QT_QUICK_CONTROLS_PLUGIN + '\n':
                    return '\nCMake target name: <Qt-component>\n'
        if section == 'files':
            if (module == 'qtdeclarative' and field == 'fileName'
                    and value == './qml/QtQuick/Templates/' + QT_QUICK_TEMPLATES_LIBRARY + '.so'):
                return './qml/QtQuick/Templates/<Qt-component>.so'
        if (module == 'qtwayland' and section == 'relationships' and field == 'comment'
                and document[section][index].get('relationshipType') == 'GENERATED_FROM'
                and value in {'/src_dir/qtwayland/src/compositor/extensions/' + QT_WAYLAND_TEXT_INPUT_SOURCE + '.cpp',
                              '/src_dir/qtwayland/src/compositor/extensions/' + QT_WAYLAND_TEXT_INPUT_SOURCE + '.h'}):
            return value.replace(QT_WAYLAND_TEXT_INPUT_SOURCE, '<Qt-component>')
        identifier = ((section in {'packages', 'files'} and field == 'SPDXID')
                      or (section == 'relationships' and field in {'spdxElementId', 'relatedSpdxElement'}))
        if (module == 'qtdeclarative' and identifier
                and re.fullmatch(r'SPDXRef-[A-Za-z0-9.-]+', value)):
            for name in QT_COMPONENT_NAMES:
                value = value.replace(name, '<Qt-component>')
            return value
    if (module == 'qtdeclarative' and len(path) == 5
            and path[0] == 'packages' and isinstance(path[1], int)
            and path[2] == 'externalRefs' and isinstance(path[3], int)
            and path[4] == 'referenceLocator'):
        reference = document['packages'][path[1]]['externalRefs'][path[3]]
        if (reference.get('referenceType') == 'purl'
                and reference.get('referenceCategory') == 'PACKAGE_MANAGER'
                and value.startswith(('pkg:github/qt/qtdeclarative@',
                                      'pkg:generic/TheQtCompany/qtdeclarative-'))):
            return value.replace(QT_QUICK_CONTROLS_PLUGIN, '<Qt-component>')
    return value


def qt_spdx_scan_text(path, text):
    admitted = qt_spdx_document(path, text)
    if admitted is None:
        return text
    module, document = admitted
    # JSON validation establishes token order. Walk the same insertion order to
    # bind each string's source span to its semantic path without reformatting
    # the document or changing diagnostic line numbers. Duplicate keys fail closed.
    strings = iter(JSON_STRING.finditer(text))
    pending = [((), document, False)]
    replacements = []
    visited = 0
    while pending:
        location, value, key = pending.pop()
        visited += 1
        if visited > MAX_QT_SPDX_NODES:
            return text
        if isinstance(value, (dict, list)):
            children = len(value) * (2 if isinstance(value, dict) else 1)
            if visited + len(pending) + children > MAX_QT_SPDX_NODES:
                return text
            if value and len(location) >= MAX_QT_SPDX_DEPTH:
                return text
        if isinstance(value, dict):
            for name, child in reversed(list(value.items())):
                pending.append((location + (name,), child, False))
                pending.append((location, name, True))
        elif isinstance(value, list):
            for index in range(len(value) - 1, -1, -1):
                pending.append((location + (index,), value[index], False))
        elif isinstance(value, str):
            token = next(strings)
            if key:
                continue
            sanitized = qt_spdx_value(module, document, location, value)
            if sanitized != value:
                replacements.append((token.start(), token.end(), json.dumps(sanitized, ensure_ascii=False)))
    chunks = []
    position = 0
    for start, end, replacement in replacements:
        chunks.extend((text[position:start], replacement))
        position = end
    chunks.append(text[position:])
    return ''.join(chunks)


def findings(path, text):
    text = qt_spdx_scan_text(path, text)
    problems = []
    fixture = any(part in {"tests", "testdata"} for part in Path(path).parts) or Path(path).name.endswith("_test.go")
    for number, line in enumerate(text.splitlines(), 1):
        if any(match[1] not in EXAMPLE_USERS and not match[1].startswith(".") for match in HOME_PATH.finditer(line)):
            problems.append((number, "personal home path; use a documented example path"))
        if not fixture and (ACCOUNT_ENDPOINT.search(line) or ACCOUNT_SETTING.search(line)):
            problems.append((number, "account-specific R2 endpoint; require operator configuration"))
        if not fixture and ENVIRONMENT_ID.search(line):
            problems.append((number, "opaque environment identifier; use an explicit placeholder"))
        if path.startswith(("packages/", "workflows/")) and not fixture and DEPLOYMENT_DEFAULT.search(line):
            problems.append((number, "organization-specific deployment default"))
    return problems


def check_entries(entries):
    failures = 0
    for path, data in entries:
        try:
            text = data.decode("utf-8")
        except UnicodeError:
            continue
        for line, reason in findings(path, text):
            print(f"{path}:{line}: {reason}")
            failures += 1
    return failures


def tracked_entries(root):
    names = subprocess.check_output(["git", "ls-files", "-z"], cwd=root).decode().split("\0")
    for name in names:
        path = root / name
        if name and path.is_file() and not path.is_symlink():
            yield name, path.read_bytes()


def archive_entries(path):
    with tarfile.open(path, "r:*") as archive:
        for member in archive:
            if member.isfile() and not is_bundled_tool_payload(member.name):
                stream = archive.extractfile(member)
                if stream is not None:
                    yield "packages/" + member.name.lstrip("./"), stream.read()


def optional_store_archives(root):
    """Accept absent/empty optional stores while fully validating selected stores."""
    if root.is_symlink():
        raise ValueError('Optional store root must not be a link')
    if not root.exists():
        return []
    if not root.is_dir():
        raise ValueError('Optional store root must be a directory')
    entries = list(root.rglob('*'))
    if any(path.is_symlink() for path in entries):
        raise ValueError('Optional store contents must not be links')
    files = [path for path in entries if path.is_file()]
    if not files:
        return []
    manifests = [path for path in files if path.name == 'packages-store-manifest.json']
    if not manifests or any(path.parent not in {manifest.parent for manifest in manifests} for path in files):
        raise ValueError('Selected nonempty stores require complete flat manifests')
    specification = importlib.util.spec_from_file_location(
        'public_store_verifier', Path(__file__).with_name('release-artifacts.py'))
    verifier = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(verifier)
    for manifest in manifests:
        verifier.verify_store(manifest.parent)
    return sorted(path for path in files if path.name.endswith('.tar.gz'))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[5])
    parser.add_argument("--archive", type=Path, action="append", default=[])
    parser.add_argument("--store-root", type=Path, help="Optional validated store root; absent or empty means standalone CLI")
    arguments = parser.parse_args()
    failures = check_entries(tracked_entries(arguments.root))
    if arguments.store_root is not None:
        try:
            arguments.archive.extend(optional_store_archives(arguments.store_root))
        except (OSError, ValueError) as error:
            parser.error(str(error))
        failures += check_entries((str(path), path.read_bytes())
                                  for path in arguments.store_root.rglob('packages-store-manifest.json'))
    failures += sum(check_entries(archive_entries(path)) for path in arguments.archive)
    if failures:
        print(f"Public-content check failed: {failures} finding(s); matched values withheld.")
        return 1
    print("Public-content check passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
