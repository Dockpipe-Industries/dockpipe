"""Group remaining installed remote checks by reusable platform environments."""

from collections import Counter

from remote_matrix import MANUAL_SERVER


HANDS_ON_ASSERTIONS = {
    "hardware-key-enrollment-and-revocation": ("hardware-key", ("operator", "broker")),
    "native-authentication-required": ("native-authentication", ("operator", "broker")),
    "all-attached-monitor-identities": ("attached-displays", ("desktop-sender",)),
    "frame-from-every-attached-monitor": ("attached-displays", ("desktop-sender",)),
    "mixed-dpi-alignment": ("mixed-dpi-displays", ("desktop-sender",)),
}


def blocked_reasons(lane, role_blockers):
    if "observation_sha256" in lane:
        return [lane["reason"]]

    return list(dict.fromkeys(role_blockers[(role, profile)]
                              for role, profile in lane["participants"].items()
                              if (role, profile) in role_blockers))


def remote_work_plan(registry, matrix):
    """Keep the matrix verdicts and plan work without admitting or executing fixtures."""
    profiles = registry["platforms"]
    profile_order = {profile["id"]: index for index, profile in enumerate(profiles)}
    role_blockers = {(row["role"], row["platform"]): row["reason"]
                     for row in matrix["roles"] if row["status"] == "blocked"}
    blockers = Counter()
    hands_on = Counter()
    batches = {}

    for lane in matrix["lanes"]:
        status = lane["status"]
        if status == "passed":
            continue

        participants = lane["participants"]
        platforms = tuple(sorted(set(participants.values()), key=profile_order.__getitem__))
        observed = set(lane.get("passed_assertions", []))
        pending = [assertion for assertion in lane["required_assertions"] if assertion not in observed]
        requirements = [HANDS_ON_ASSERTIONS[assertion] for assertion in pending
                        if assertion in HANDS_ON_ASSERTIONS]
        manual = {(check, participants[role]) for check, roles in requirements
                  for role in roles if role in participants}
        if MANUAL_SERVER in platforms:
            manual.add(("manual-windows-server-vm", MANUAL_SERVER))
        hands_on.update(manual)

        if status == "blocked":
            blockers.update(blocked_reasons(lane, role_blockers))
            continue

        batches.setdefault(platforms, []).append({
            "feature": lane["feature"],
            "participants": participants,
            "status": status,
            "remaining_assertions": pending,
        })

    grouped = []
    ordered = sorted(batches, key=lambda items: (MANUAL_SERVER in items, tuple(profile_order[item] for item in items)))
    for platforms in ordered:
        checks = batches[platforms]
        grouped.append({
            "platforms": list(platforms),
            "checks": checks,
        })

    return {
        "schema": "dockpipe.product-integration.remote-work-plan/v1",
        "counts": matrix["counts"],
        "platforms": profiles,
        "blockers": [{"reason": reason, "affected_lanes": count} for reason, count in blockers.most_common()],
        "hands_on": [{"check": check, "platform": profile, "affected_lanes": count}
                     for (check, profile), count in sorted(hands_on.items(),
                                                          key=lambda row: (profile_order[row[0][1]], row[0][0]))],
        "batches": grouped,
    }
