"""Native Linux guest-fixture profile; independent of desktop ownership."""
import os
from pathlib import Path


def resource_admission(directory):
    memory = dict(line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines())
    available = int(memory['MemAvailable'].split()[0]) * 1024
    filesystem = os.statvfs(directory)
    free = filesystem.f_bavail * filesystem.f_frsize
    if available < 6 << 30 or free < 32 << 30:
        raise RuntimeError('Linux guest requires at least 6 GiB available RAM and 32 GiB free disk; no tuning performed.')
    return {'available_memory_bytes': available, 'free_disk_bytes': free,
            'guest_memory_mib': 4096, 'guest_vcpus': 4}


def prepare_native_guest_environment(selections, environment):
    qt = Path(selections['qt_prefix'])
    browser = Path(selections['browser_sdk_prefix'])
    qemu = Path(selections['qemu_prefix'])
    environment.update(LD_LIBRARY_PATH=f'{qt}/lib:{browser}/lib',
                       QT_PLUGIN_PATH=str(qt / 'plugins'),
                       QTWEBENGINEPROCESS_PATH=str(qt / 'libexec/QtWebEngineProcess'),
                       QTWEBENGINE_RESOURCES_PATH=str(qt / 'resources'),
                       QTWEBENGINE_LOCALES_PATH=str(qt / 'translations/qtwebengine_locales'),
                       DOCKPIPE_BIN=selections['cli'],
                       DOCKPIPE_PACKAGES_ROOT=selections['store'],
                       DOCKPIPE_TERMINAL_PROVIDER=str(Path(selections['gui']).parent / 'dockpipe-terminal-host'),
                       DOCKPIPE_VM_QEMU_BIN=str(qemu / 'bin/qemu-system-x86_64'),
                       DOCKPIPE_VM_QEMU_REAL_BIN=str(qemu / 'bin/qemu-system-x86_64'),
                       DOCKPIPE_VM_NATIVE_QEMU_BIN=str(qemu / 'bin/qemu-system-x86_64'),
                       DOCKPIPE_VM_QEMU_IMG_BIN=str(qemu / 'bin/qemu-img'),
                       DOCKPIPE_VM_MEMORY='4096', DOCKPIPE_VM_CPUS='4', DOCKPIPE_VM_ACCEL='kvm',
                       DOCKPIPE_VM_GUEST_PREPARATION='provided-only',
                       DOCKPIPE_VM_SECURE_BOOT='off', DOCKPIPE_VM_TPM='off')
    for key, value in selections.get('guest_environment', {}).items():
        if not key.startswith('DOCKPIPE_VM_') or not isinstance(value, str) or len(value) > 4096:
            raise ValueError('Guest environment overrides must be bounded VM provider selections.')
        if any(word in key for word in ['PASSWORD', 'SECRET', 'TOKEN']):
            raise ValueError('Pass guest credentials through private file references, not environment snapshots.')
        environment[key] = value
