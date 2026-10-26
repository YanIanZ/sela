"""Frozen snapshots: where inputs for a rebuild come from, and self-consistency.

A snapshot ``<out>/snapshots/<name>/`` is immutable. To rebuild it later, each
input whose SHA-256 is recorded in its manifest must still be available with
exactly those bytes: either the live file at its repository path, or a frozen
copy (committed
when an input such as doc 10 gets a dated note after the freeze).
"""
from __future__ import annotations

import hashlib
import json
from pathlib import Path

FROZEN_INPUTS = Path("data/research/register_inputs/frozen")
FROZEN_SUFFIX = ".frozen"


def frozen_path(root, snapshot_name, rel):
    return Path(root) / FROZEN_INPUTS / snapshot_name / (rel + FROZEN_SUFFIX)


def _sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def resolve_frozen_inputs(root, snapshot_name, inputs_hashes):
    """Return ({rel: path}, problems) for the inputs recorded by a snapshot manifest."""
    root = Path(root)
    found, problems = {}, []
    for rel, digest in sorted(inputs_hashes.items()):
        live = root / rel
        frozen = frozen_path(root, snapshot_name, rel)
        if live.exists() and _sha(live) == digest:
            found[rel] = live
        elif frozen.exists() and _sha(frozen) == digest:
            found[rel] = frozen
        else:
            problems.append(f"input {rel} changed after snapshot {snapshot_name} was frozen and no matching "
                            f"frozen copy exists at {(FROZEN_INPUTS / snapshot_name / (rel + FROZEN_SUFFIX)).as_posix()}")
    return found, problems


def check_snapshot_dir(snap_dir, profile_for_name, root=None):
    """Violations for one snapshot directory (self-consistency and, with ``root``, reproducibility)."""
    snap_dir = Path(snap_dir)
    name = snap_dir.name
    problems = []
    mpath, rpath = snap_dir / "manifest.json", snap_dir / "records.jsonl"
    if not mpath.exists() or not rpath.exists():
        return [f"snapshot {name}: manifest.json and records.jsonl are both required"]
    try:
        manifest = json.loads(mpath.read_text(encoding="utf-8"))
    except ValueError as exc:
        return [f"snapshot {name}: manifest.json is not JSON ({exc})"]
    try:
        profile = profile_for_name(name)
    except ValueError as exc:
        return [f"snapshot {name}: {exc}"]
    if manifest.get("pipeline_version") != profile.pipeline_version:
        problems.append(f"snapshot {name}: manifest pipeline_version {manifest.get('pipeline_version')!r} does not "
                        f"match the directory name")
    if profile.snapshot_name(manifest.get("freeze_date")) != name:
        problems.append(f"snapshot {name}: directory name does not match freeze_date {manifest.get('freeze_date')!r}")
    if manifest.get("outputs", {}).get("records.jsonl") != _sha(rpath):
        problems.append(f"snapshot {name}: records.jsonl sha256 differs from the snapshot manifest")
    if root is not None:
        _, missing = resolve_frozen_inputs(root, name, manifest.get("inputs", {}))
        problems.extend(f"snapshot {name} is no longer reproducible: {m}" for m in missing)
    return problems
