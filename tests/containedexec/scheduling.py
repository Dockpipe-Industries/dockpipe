"""Conservative measured grouping. Unknown, memory and special cases stay single."""
import re


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
    import json
    from pathlib import Path
    from campaign import file_identity
    if profile.get('identity') != identity:
        return {}
    verified, result = {}, {}
    for case, measurement in profile.get('cases', {}).items():
        keys = measurement.get('artifacts', {})
        if not measurement.get('warm') or not keys:
            continue
        valid = True
        for key, expected in keys.items():
            if key not in verified:
                try:
                    directory = Path(cache) / key
                    record = json.loads((directory / 'record.json').read_text())
                    verified[key] = (record.get('Version') == 'pipelang-native-validation-v2' and record.get('Key') == key
                                     and record.get('BinarySHA256') == expected
                                     and file_identity(directory / 'program.test')['sha256'] == expected)
                except (OSError, ValueError):
                    verified[key] = False
            valid = valid and verified[key]
        if valid:
            result[case] = measurement
    return result


def observed_profile(rows, identity, cache):
    """Persist warm singleton observations; grouped/failed runs cannot invent them."""
    import json
    from pathlib import Path
    cases = {}
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
