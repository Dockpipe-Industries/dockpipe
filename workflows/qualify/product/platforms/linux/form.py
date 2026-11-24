"""Launch only the visibly filled normal installer form for the new fixture."""
import subprocess

from guest_environment import resource_admission
import ui


def installer_button(atspi, session, media, disk):
    visible = [node for node, _depth in ui.nodes(atspi.Registry, session['gui_pid'])
               if node.getState().contains(atspi.STATE_SHOWING)]
    choices = [node for node in visible if node.name == 'Install from ISO'
               and node.getRoleName() == 'combo box']
    entries = [node.queryText().getText(0, -1) for node in visible
               if node.getRoleName() == 'text' and not node.name]
    if len(choices) != 1 or str(media) not in entries or str(disk) not in entries:
        return None
    dialogs = [node for node in visible if node.getRoleName() == 'dialog'
               and 'Linux virtual machine' in node.name]
    if len(dialogs) != 1:
        raise RuntimeError('Expected one normal Linux workflow form.')
    size = ui.select(atspi.Registry, session['gui_pid'], 'DiskSize', 'text')
    if size.queryText().getText(0, -1) != '64G':
        raise RuntimeError('New fixture disk must use the admitted 64G size.')
    if disk.exists():
        raise RuntimeError('New installer destination appeared before normal GUI launch; reconcile it.')
    button = ui.select(atspi.Registry, session['gui_pid'], 'Launch', 'push button')
    if not button.getState().contains(atspi.STATE_SHOWING):
        raise RuntimeError('Normal launch button is not visible.')
    return button.queryComponent().getExtents(atspi.DESKTOP_COORDS)


def launch_filled_installer(desktop, media, disk):
    from gi.repository import GLib

    atspi, session = ui.connect(desktop.output / 'session-private.json')
    try:
        rectangle = installer_button(atspi, session, media, disk)
    except GLib.Error as error:
        if error.domain != 'atspi_error' or 'No such object path' not in error.message:
            raise
        # Qt may dispose a routed page while the read-only probe is traversing it.
        return None
    if rectangle is None:
        return None

    windows = subprocess.check_output(['/usr/bin/xdotool', 'search', '--all', '--pid',
        str(session['gui_pid']), '--name', 'Workflow Settings'],
        env=session['environment'], text=True, timeout=5).splitlines()
    if len(windows) != 1:
        raise RuntimeError('Normal workflow form window identity is ambiguous.')
    subprocess.run(['/usr/bin/xdotool', 'windowfocus', windows[0]],
                   env=session['environment'], check=True, timeout=5)
    resources = resource_admission(disk.parent)
    subprocess.run(['/usr/bin/xdotool', 'mousemove', str(rectangle.x + rectangle.width // 2),
                    str(rectangle.y + rectangle.height // 2), 'click', '1'],
                   env=session['environment'], check=True, timeout=5)
    return {'action': 'normal Workflow Settings Launch button', 'gui_pid': session['gui_pid'],
            'media': str(media), 'disk': str(disk), 'new_destination': True, 'resources': resources}
