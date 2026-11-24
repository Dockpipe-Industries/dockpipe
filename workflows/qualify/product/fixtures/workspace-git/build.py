#!/usr/bin/env python3
"""Manual producer for a test-owned Git repository broker, with no source edits."""
import argparse
import json
import os
from pathlib import Path
import sys


ROOT = Path(__file__).resolve().parents[5]
OWNER = ROOT / 'workflows/qualify/product'
sys.path.insert(0, str(OWNER / 'assets/scripts'))
from contracts import digest_file, require, write_json
from process import run_driver


def documents(text):
    decoder = json.JSONDecoder()
    offset = 0
    while offset < len(text):
        while offset < len(text) and text[offset].isspace():
            offset += 1
        if offset == len(text):
            break
        value, offset = decoder.raw_decode(text, offset)
        yield value


def source_inventory(listing, overlay):
    paths = {ROOT / 'go.mod', ROOT / 'go.sum', Path(__file__).resolve(),
             Path(__file__).with_name('config.yml'), Path(__file__).with_name('broker_test.go')}
    for package in documents(listing):
        if not package.get('Dir'):
            continue
        for field in ['GoFiles', 'CgoFiles', 'CFiles', 'HFiles', 'SFiles', 'SysoFiles',
                      'EmbedFiles', 'TestGoFiles', 'XTestGoFiles']:
            for name in package.get(field, []):
                path = Path(name) if Path(name).is_absolute() else Path(package['Dir']) / name
                path = Path(overlay.get(str(path), str(path)))
                require(path.is_file(), 'Go source input is unavailable: ' + str(path))
                paths.add(path.resolve(strict=True))
    require(len(paths) <= 8192, 'Go input inventory exceeded its file bound')
    require(sum(path.stat().st_size for path in paths) <= 256 << 20, 'Go inputs exceeded 256MiB')
    return {str(path): digest_file(path) for path in sorted(paths)}


def produce(configuration):
    go = configuration.go
    git = configuration.git
    require(go.is_absolute() and digest_file(go) == configuration.go_sha256, 'Go executable differs from its pin')
    require(git.is_absolute() and digest_file(git) == configuration.git_sha256, 'Git executable differs from its pin')
    output = configuration.output
    require(output.is_absolute() and not output.exists(), 'Select fresh absolute producer output')
    output.mkdir(mode=0o700, parents=True)
    directories = {}
    for name in ['home', 'cache', 'tmp', 'templates']:
        directories[name] = output / name
        directories[name].mkdir(mode=0o700)
    environment = {'PATH': str(go.parent) + os.pathsep + '/usr/bin:/bin', 'HOME': str(directories['home']),
                   'GOCACHE': str(directories['cache']), 'TMPDIR': str(directories['tmp']),
                   'GOTMPDIR': str(directories['tmp']), 'GOMAXPROCS': '2',
                   'CGO_ENABLED': '0', 'GOTOOLCHAIN': 'local', 'GOENV': 'off', 'GOPROXY': 'off',
                   'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': '/dev/null',
                   'GIT_TEMPLATE_DIR': str(directories['templates']), 'GIT_TERMINAL_PROMPT': '0'}
    overlay = {str(ROOT / 'src/lib/infrastructure/workspacegit/product_fixture_broker_test.go'):
               str(Path(__file__).with_name('broker_test.go').resolve(strict=True))}
    overlay_path = output / 'overlay.json'
    write_json(overlay_path, {'Replace': overlay})

    def run(name, arguments, timeout=300):
        logs = output / name
        logs.mkdir(mode=0o700)
        result = run_driver([str(go), *arguments], logs, timeout, environment, ROOT)
        write_json(logs / 'process.json', result)
        require(result['exit_code'] == 0 and not result['failure'], 'Fixture producer failed: ' + name)
        return (logs / 'driver.stdout').read_text()

    arguments = ['list', '-overlay', str(overlay_path), '-deps', '-test', '-json',
                 './src/lib/infrastructure/workspacegit']
    before = source_inventory(run('input-list-before', arguments), overlay)
    binary = output / 'workspace-git-fixture'
    run('compile', ['test', '-c', '-p', '2', '-vet=off', '-overlay', str(overlay_path),
                    '-o', str(binary), './src/lib/infrastructure/workspacegit'])
    binary.chmod(0o700)
    after = source_inventory(run('input-list-after', arguments), overlay)
    require(before == after, 'Fixture source inputs changed during compilation')
    checks = output / 'configuration-fault-tests'
    checks.mkdir(mode=0o700)
    result = run_driver([str(binary), '-test.run=^TestProductWorkspaceGitFixtureConfig$', '-test.timeout=10s'],
                        checks, 15, environment, ROOT)
    write_json(checks / 'process.json', result)
    require(result['exit_code'] == 0 and not result['failure'], 'Fixture configuration fault tests failed')
    require(digest_file(go) == configuration.go_sha256 and digest_file(git) == configuration.git_sha256,
            'Selected producer tools changed')
    manifest = {'schema': 'dockpipe.product-git-fixture-build/v1', 'binary': str(binary),
                'binary_sha256': digest_file(binary), 'go': str(go), 'go_sha256': configuration.go_sha256,
                'git': str(git.resolve(strict=True)), 'git_sha256': configuration.git_sha256,
                'source_inputs': after, 'source_overlay_only': True, 'repository_created_during_build': False,
                'configuration_fault_tests_passed': True}
    write_json(output / 'manifest.json', manifest)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', type=Path, required=True)
    parser.add_argument('--go-sha256', required=True)
    parser.add_argument('--git', type=Path, required=True)
    parser.add_argument('--git-sha256', required=True)
    parser.add_argument('--output', type=Path, required=True)
    produce(parser.parse_args())


if __name__ == '__main__':
    main()
