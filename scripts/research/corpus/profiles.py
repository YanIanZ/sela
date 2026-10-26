"""Build profiles: one per pipeline version whose outputs were ever frozen.

A profile fixes the version strings written into the outputs and switches the
schema additions on or off. ``1.1.0`` is the profile of the frozen snapshot
``snapshots/2026-10-26/``; it must keep producing exactly those bytes, so its
code path is the pre-1.2.0 path with no v3 field, flag or input.
"""
from __future__ import annotations

from dataclasses import dataclass

from . import PIPELINE_VERSION, RULES_VERSION, SCHEMA_VERSION


@dataclass(frozen=True)
class Profile:
    pipeline_version: str
    rules_version: str
    schema_version: str
    v3: bool

    def snapshot_name(self, freeze_date):
        """1.1.0 snapshots are keyed by freeze date alone; later ones also by version."""
        if not self.v3:
            return str(freeze_date)
        return f"{freeze_date}_pipeline-{self.pipeline_version}"


LEGACY = Profile("1.1.0", "sela-corpus-rules-v2", "sela-corpus-schema-v2", v3=False)
CURRENT = Profile(PIPELINE_VERSION, RULES_VERSION, SCHEMA_VERSION, v3=True)
PROFILES = {LEGACY.pipeline_version: LEGACY, CURRENT.pipeline_version: CURRENT}


def for_version(version):
    try:
        return PROFILES[str(version)]
    except KeyError:
        raise ValueError(f"no build profile for pipeline version {version!r}; known: {sorted(PROFILES)}") from None


def snapshot_profile(name):
    """Profile implied by a snapshot directory name ('2026-10-26' or '2026-10-26_pipeline-1.2.0')."""
    if "_pipeline-" in name:
        return for_version(name.split("_pipeline-", 1)[1])
    return LEGACY
