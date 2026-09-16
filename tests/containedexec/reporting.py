"""Rebuildable linear reporting; no execution or receipt-admission authority."""
import json
import math
from pathlib import Path
import re
import statistics
import time
import zlib
from campaign import atomic_json, atomic_json_array, canonical, digest, read_sealed


class CompactRows:
    """Retain compressed exact snapshots; materialize one row per iteration."""
    def __init__(self):
        self.entries = []

    def append(self, row, *, index=None):
        # An external index preserves report schemas that have no index field.
        order = row['index'] if index is None else index
        self.entries.append((order, zlib.compress(canonical(row), level=1)))

    def extend(self, rows):
        for row in rows:
            self.append(row)

    def __len__(self):
        return len(self.entries)

    def __iter__(self):
        for _, encoded in self.entries:
            yield json.loads(zlib.decompress(encoded))

    def sort(self):
        self.entries.sort(key=lambda entry: entry[0])

    def write(self, path):
        return atomic_json_array(path, self)


def distribution(values):
    if not values:
        return dict(count=0, p50=None, p95=None, maximum=None)
    values = sorted(values)
    return dict(count=len(values), p50=statistics.median(values), p95=values[math.ceil(.95 * len(values)) - 1], maximum=values[-1])


def summarize(rows, wall_seconds, workers, campaign_root=None):
    families, phases, audits = {}, {}, {}
    hits = misses = native_children = 0
    cache_reasons = {}
    for row in rows:
        family = row['tests'][0].split('/')[0]
        families.setdefault(family, []).append(row['report'].get('elapsed_s'))
        source_audit = []
        log = Path(row['directory']) / 'unit.output'
        if log.exists():
            with log.open() as stream:
                for line in stream:
                    if 'generated_case_audit ' in line:
                        source_audit.append(line.split('generated_case_audit ', 1)[1].strip())
                    if 'generated_compiled_artifact ' in line:
                        hits += 'cache_hit=true' in line
                        misses += 'cache_hit=false' in line
                        reason = re.search(r'reason=(\w+)', line)
                        name = reason.group(1) if reason else 'unknown_legacy_reason'
                        cache_reasons[name] = cache_reasons.get(name, 0) + 1
                    if 'generated_native_run ' in line:
                        native_children += 1
                    match = re.search(r'(generated_\w+) .*?elapsed_ns=(\d+)', line)
                    if match:
                        phase = re.search(r'phase=(\w+)', line)
                        name = phase.group(1) if phase else match[1]
                        phases.setdefault(name, []).append(int(match[2]) / 1e9)
        audits[row['receipt']] = dict(cases=row['tests'], source_fixture_digest=digest(source_audit), generated_cases=len(source_audit))
    elapsed = [row['report'].get('elapsed_s') for row in rows if not row.get('resumed')]
    summed = sum(v for v in elapsed if v is not None)
    attempts = attempt_costs(campaign_root) if campaign_root else None
    return dict(attempts=attempts, families={name: distribution([x for x in values if x is not None]) for name, values in families.items()},
                phases={name: dict(distribution(values), summed_s=sum(values)) for name, values in phases.items()},
                wall_s=wall_seconds, summed_workload_s=summed,
                workload_occupancy=summed / (wall_seconds * workers) if wall_seconds else None,
                peak_bytes=max((row['report'].get('aggregate_peak_bytes', 0) for row in rows), default=0),
                cache_hits=hits, cache_misses=misses, cache_reasons=cache_reasons, native_children=native_children, audits=audits,
                timing_policy='nested phase sums overlap workload; missing spans are unknown; resumed proof contributes no current workload')


def attempt_costs(root):
    counts, measured, unknown, retained = {}, {}, 0, set()
    for path in (Path(root) / 'attempts').glob('*/state.json'):
        try:
            state = read_sealed(path)
        except (OSError, ValueError):
            counts['corrupt'] = counts.get('corrupt', 0) + 1
            unknown += 1
            continue
        status = state['state']
        counts[status] = counts.get(status, 0) + 1
        elapsed = state.get('report', {}).get('elapsed_s')
        if elapsed is None:
            unknown += 1
        else:
            measured[status] = measured.get(status, 0) + elapsed
        log = path.parent / 'unit.output'
        if status != 'passed' and log.exists():
            with log.open() as stream:
                for line in stream:
                    match = re.search(r'generated_preparation cache_hit=false .*key=(\S+)', line)
                    if match:
                        retained.add(match[1])
    return dict(states=counts, measured_workload_s=measured, unknown_elapsed_attempts=unknown,
                preparation_keys_from_failed_attempts=sorted(retained),
                scope='all retained attempts, including prior revisions; never add this to current wall time')
