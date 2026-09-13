"""Conservative measured grouping. Unknown, memory and special cases stay single."""
import json
import math
from pathlib import Path
import re


def load_profile(path, identity, cache, *, automatic=False):
    """Automatic hints may fall back; an explicit profile must match inputs."""
    try:
        profile = json.loads(Path(path).read_text())
        if not isinstance(profile, dict) or not isinstance(profile.get('cases'), dict):
            raise ValueError('invalid schedule profile')
        if profile.get('identity') != identity:
            raise ValueError('schedule profile input/host/worker/policy drift')
    except (OSError, ValueError) as error:
        if not automatic:
            raise
        return {}, dict(status='singleton_fallback', reason=str(error))
    admitted = warm_profile(profile, identity, cache)
    return admitted, dict(status='admitted' if admitted else 'singleton_fallback',
                         reason='verified warm observations' if admitted else 'no verified warm observations')


def measured_plan(jobs, profile, maximum=2):
    result, pending, prediction = [], [], 0
    def flush():
        nonlocal prediction
        if pending:
            names = pending.copy()
            base = names[0].split('/')[0]
            pattern = '^' + re.escape(base) + '$/^(' + '|'.join(re.escape(n.split('/')[-1]) for n in names) + ')$'
            result.append((names, pattern))
            pending.clear()
            prediction = 0
    for names, pattern in jobs:
        name = names[0]
        measurement = profile.get(name, {})
        elapsed, peak = measurement.get('elapsed_s'), measurement.get('peak_bytes')
        eligible = (len(names) == 1 and len(name.split('/')) == 2 and 'Memory' not in name
                    and re.fullmatch(r'(shape)?\d+', name.split('/')[-1]) and measurement.get('warm') is True
                    and elapsed is not None and peak is not None and peak < 450 << 20 and elapsed < 4)
        estimate = elapsed * 1.5 + .1 if eligible else 0
        if not eligible:
            flush(); result.append((names, pattern)); continue
        if pending and (pending[0].split('/')[0] != name.split('/')[0] or prediction + estimate >= 10 or len(pending) >= maximum):
            flush()
        pending.append(name); prediction += estimate
    flush()
    return result


def warm_profile(profile, identity, cache):
    """Admit measured predictions only with matching policy and verified objects."""
    from campaign import file_identity
    if profile.get('identity') != identity:
        return {}
    verified, result = {}, {}
    for case, measurement in profile.get('cases', {}).items():
        if not isinstance(measurement, dict):
            continue
        if any(type(measurement.get(field)) not in (int, float)
               or not math.isfinite(measurement[field]) or measurement[field] < 0
               for field in ('elapsed_s', 'peak_bytes')):
            continue
        keys = measurement.get('artifacts', {})
        if measurement.get('warm') is not True or not isinstance(keys, dict) or not keys:
            continue
        valid = True
        for key, expected in keys.items():
            if not all(isinstance(value, str) and re.fullmatch('[a-f0-9]{64}', value)
                       for value in (key, expected)):
                valid = False
                break
            if key not in verified:
                try:
                    directory = Path(cache) / key
                    record = json.loads((directory / 'record.json').read_text())
                    actual = file_identity(directory / 'program.test')['sha256']
                    verified[key] = actual if (isinstance(record, dict)
                                              and record.get('Version') == 'pipelang-native-validation-v2'
                                              and record.get('Key') == key
                                              and record.get('BinarySHA256') == actual) else None
                except (OSError, ValueError):
                    verified[key] = None
            valid = valid and verified[key] == expected
        if valid:
            result[case] = measurement
    return result


def observed_profile(rows, identity, cache, inherited=None):
    """Persist warm singleton observations; grouped/failed runs cannot invent them."""
    import json
    from pathlib import Path
    successful = {case for row in rows if not row['exit'] for case in row['tests']}
    failed = {case for row in rows if row['exit'] for case in row['tests']}
    # These are already admitted singleton measurements, never timings inferred
    # from a group or a resumed receipt. Retain only cases that succeeded here.
    cases = {case: value for case, value in (inherited or {}).items()
             if case in successful and case not in failed}
    for row in rows:
        if row['exit'] or len(row['tests']) != 1 or row.get('resumed'):
            continue
        hits, misses, artifacts = 0, 0, {}
        with (Path(row['directory']) / 'unit.output').open() as stream:
            for line in stream:
                match = re.search(r'generated_compiled_artifact .*cache_hit=(true|false) key=([a-f0-9]{64})', line)
                if match:
                    hits += match[1] == 'true'
                    misses += match[1] == 'false'
                    record = json.loads((Path(cache) / match[2] / 'record.json').read_text())
                    artifacts[match[2]] = record['BinarySHA256']
        cases[row['tests'][0]] = dict(elapsed_s=row['report']['elapsed_s'], peak_bytes=row['report']['aggregate_peak_bytes'],
                                     warm=bool(hits) and not misses, artifacts=artifacts)
    return dict(identity=identity, cases=cases)
