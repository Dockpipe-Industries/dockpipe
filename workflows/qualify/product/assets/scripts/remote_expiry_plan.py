"""Explicit-only preparation of the owning installed Flatpak expiry lane."""
from pathlib import Path
import hashlib
import importlib.util

from contracts import digest_file, read_json_document, require

OWNER = Path(__file__).resolve().parents[2]
REPOSITORY = OWNER.parents[2]


def prepare(path, expected_sha256, run_id, registry_sha256, execute_reviewed=False):
    require(path.is_absolute(), 'Select an absolute private expiry plan')
    plan, data = read_json_document(path)
    require(hashlib.sha256(data).hexdigest() == expected_sha256, 'Expiry plan differs from the explicit requested digest')
    require(plan.get('schema') == 'dockpipe.flatpak.remote-lease-expiry-plan/v1'
            and plan.get('run_id') == run_id and plan['registry']['sha256'] == registry_sha256,
            'Use a fresh expiry plan matching this product run and current registry; historical receipts are not accepted')
    owner_path = REPOSITORY / 'workflows/build/flatpak/assets/scripts/installed-remote-product.py'
    specification = importlib.util.spec_from_file_location('remote_expiry_preparation_owner', owner_path)
    module = importlib.util.module_from_spec(specification)
    specification.loader.exec_module(module)
    profile = module.host_profile()
    platform = ('linux-amd64-flatpak-bazzite-kde6.10' if profile['ID'] == 'bazzite'
                else 'linux-amd64-flatpak-kde6.10')
    paths = {owner_path, owner_path.with_name('installed-remote-evidence.py'),
             owner_path.with_name('installed-remote-expiry-run.py'),
             OWNER / 'assets/scripts/contracts.py', OWNER / 'assets/scripts/process.py',
             OWNER / 'assets/scripts/remote_expiry_plan.py', OWNER / 'assets/scripts/prepare-plan.py',
             OWNER / 'registry.json'}
    for record in plan['source_pins']:
        selected = Path(record['path'])
        if selected.is_relative_to(REPOSITORY):
            require(digest_file(selected) == record['sha256'], 'Reviewed GUI source dependency changed')
            paths.add(selected)
    dependencies = [{'path': 'repo://' + selected.relative_to(REPOSITORY).as_posix(),
                     'sha256': digest_file(selected)} for selected in sorted(paths)]
    entrypoint = 'drivers/remote_expiry.py'
    driver = {'id': 'remote-view-expiry', 'entrypoint': entrypoint,
              'sha256': digest_file(OWNER / entrypoint), 'dependencies': dependencies,
              'features': ['remote.desktop.viewer.lease-expiry'], 'scenarios': ['failure'],
              'layers': ['installed_e2e'], 'timeout_seconds': 420,
              'configuration': {'expiry_plan': str(path), 'expiry_plan_sha256': expected_sha256,
                                'execute_reviewed': execute_reviewed, 'host_profile': profile, 'observer_context': module.observer_context()}}
    return {'id': 'remote-view-expiry-flatpak', 'platform': platform, 'kind': 'flatpak',
            'artifact_sha256': plan['configuration']['bundle']['sha256'],
            'capabilities': [], 'drivers': [driver]}
