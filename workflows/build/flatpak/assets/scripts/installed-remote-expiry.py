"""Observe lease expiry while only an exactly admitted local GUI is stopped.

This helper cannot open a remote session, send input, close Core references, or
claim a product case. The caller supplies independently observed Core/native
facts and must retain their original hashed evidence separately.
"""
import math
import os
import signal
import time
from pathlib import Path


def require(condition, message):
    if not condition:
        raise ValueError(message)


def process_identity(pid):
    require(type(pid) is int and pid > 1, 'Expected one exact admitted GUI PID')
    root = Path('/proc') / str(pid)
    value = (root / 'stat').read_text().rsplit(')', 1)[1].split()
    return {'pid': pid, 'starttime': value[19], 'state': value[0],
            'uid': root.stat().st_uid}


class GuiLeaseExpiry:
    """One non-replayable pause, with causal ordering recorded before cleanup."""

    def __init__(self, token, deadline_monotonic):
        require(isinstance(token, dict) and set(token) == {'pid', 'starttime'},
                'Expected the admitted GUI kernel identity token')
        current = process_identity(token['pid'])
        require(current['starttime'] == token['starttime'] and current['uid'] == os.getuid(),
                'GUI kernel identity changed before expiry admission')
        require(type(deadline_monotonic) in [int, float] and math.isfinite(deadline_monotonic)
                and time.monotonic() < deadline_monotonic <= time.monotonic() + 18,
                'Expiry observation must precede the independent hard cutoff')
        self.token = dict(token)
        self.deadline = deadline_monotonic
        # pidfd prevents PID reuse between identity validation and signalling.
        self.descriptor = os.pidfd_open(token['pid'], 0)
        try:
            require(self.current()['state'] not in ['T', 't', 'Z'],
                    'Expiry probe requires an initially running admitted GUI')
        except BaseException:
            os.close(self.descriptor)
            raise
        self.started = None
        self.pause_sent = False
        self.closed_observed = None
        self.interventions = []
        self.resumed = False
        self.finished = False

    def current(self):
        current = process_identity(self.token['pid'])
        require(current['starttime'] == self.token['starttime'] and current['uid'] == os.getuid(),
                'Original GUI identity is no longer available')
        return current

    def pause(self):
        require(self.started is None and not self.finished, 'The GUI pause cannot be replayed')
        require(time.monotonic() < self.deadline, 'Expiry observation cutoff already elapsed')
        self.current()
        signal.pidfd_send_signal(self.descriptor, signal.SIGSTOP)
        self.pause_sent = True
        until = min(time.monotonic() + 0.5, self.deadline)
        while time.monotonic() < until:
            if self.current()['state'] in ['T', 't']:
                self.started = time.monotonic()
                return {'gui': self.token, 'pause_observed_monotonic': self.started,
                        'input_permitted': False, 'carrier_signalled': False}
            time.sleep(0.005)
        raise ValueError('Original GUI stopped state was not observed before the cutoff')

    def intervention(self, operation):
        require(operation in ['core-close', 'gui-resume', 'gui-signal', 'carrier-signal',
                              'watchdog-assisted', 'controller-cancelled'],
                'Unknown expiry-intervention classification')
        self.interventions.append({'operation': operation, 'monotonic': time.monotonic()})

    def observe(self, reference, core, native):
        require(self.started is not None and not self.finished and self.closed_observed is None,
                'Expiry requires exactly one original paused GUI observation')
        require(time.monotonic() < self.deadline and not self.interventions,
                'Cleanup, resume or cutoff occurred before independent expiry')
        require(self.current()['state'] in ['T', 't'], 'GUI resumed before independent expiry')
        require(isinstance(reference, dict) and isinstance(core, dict) and isinstance(native, dict),
                'Independent original-reference Core/native objects are required')
        require(core.get('reference') == reference and core.get('state') == 'closed'
                and type(core.get('ack')) is int and core['ack'] >= 1
                and core.get('lease_millis') == 0 and not core.get('signals'),
                'Original Core reference is not independently closed')
        require(native.get('native_idle_proven') is True and not native.get('active_channels')
                and not native.get('unclassified_worker_children'),
                'Native capture closure is not independently proven')
        # Check again after examining the observations. No signal/close operation
        # may make a late result appear to precede the independent cutoff.
        require(time.monotonic() < self.deadline and self.current()['state'] in ['T', 't'],
                'Expiry observations arrived after the cutoff or GUI resume')
        self.closed_observed = time.monotonic()
        return {'schema': 'dockpipe.flatpak.gui-lease-expiry-observation/v1',
                'gui': self.token, 'paused_at_monotonic': self.started,
                'closure_observed_monotonic': self.closed_observed,
                'paused_elapsed_seconds': self.closed_observed - self.started,
                'before_hard_cutoff': True, 'core_closed': True, 'native_idle': True,
                'cleanup_assisted': False, 'input_permitted': False,
                'gui_viewer_acceptance_claimed': False}

    def close(self):
        if self.finished:
            return
        self.finished = True
        try:
            # Resume only this pidfd. A failed observation remains failed even
            # when subsequent normal or emergency cleanup succeeds.
            if self.pause_sent:
                self.intervention('gui-resume')
                signal.pidfd_send_signal(self.descriptor, signal.SIGCONT)
                self.resumed = True
        except ProcessLookupError:
            pass
        finally:
            os.close(self.descriptor)
