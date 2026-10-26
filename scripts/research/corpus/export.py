"""Step 10: deterministic export (sorted keys, UTF-8, trailing newline)."""
from __future__ import annotations

import copy
import hashlib
import json
import os
from pathlib import Path

WITHHELD = "licence does not permit redistribution; locator and sha256 only"
KB_FIELDS = ("id", "title", "authors", "year", "publication_date", "journal", "doi", "url", "theme",
             "note_id", "transfer_limitation_id", "study_type", "population", "country_or_language",
             "sample_size", "license", "access_status", "screening_status", "reviewer_status",
             "within_cutoff", "flags", "evidence", "index_version")
KB_FIELDS_V3 = KB_FIELDS + ("publication_language", "age_range", "age_band", "age_groups_indexed",
                             "study_type_caution", "needs_check_documented", "reviewer_name", "review_date",
                             "review_decision", "findings", "study_type_reviewer_override",
                             "publication_date_reviewer_override", "appraisal_tool", "reviewer_2_name",
                             "review_disagreement", "adjudicated_by", "schema_version")
LABELLED_FIELDS = ("intervention_or_exposure", "comparator", "outcomes", "limitations")


class SnapshotConflict(RuntimeError):
    """An existing snapshot for this freeze date holds different content."""


def dumps_line(obj):
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def jsonl_bytes(rows):
    return "".join(dumps_line(r) + "\n" for r in rows).encode("utf-8")


def json_bytes(obj):
    return (json.dumps(obj, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode("utf-8")


def sha256(data):
    return hashlib.sha256(data).hexdigest()


def redact_evidence(items, redistribution_allowed):
    """Keep verbatim snippets only when the licence permits redistribution."""
    out = []
    for item in items:
        it = dict(item)
        if not redistribution_allowed:
            it["snippet"] = None
            it["snippet_withheld"] = WITHHELD
        out.append(it)
    return out


def public_record(record):
    """The committed form of a record (snippets withheld unless redistributable)."""
    rec = copy.deepcopy(record)
    allowed = bool(rec.get("redistribution_allowed"))
    rec["evidence"] = redact_evidence(rec.get("evidence") or [], allowed)
    for field in LABELLED_FIELDS:
        if rec.get(field) and not allowed:
            rec[field]["snippet"] = None
            rec[field]["snippet_withheld"] = WITHHELD
    return rec


def kb_row(rec, v3=False):
    """Reviewer search-index row built from a PUBLIC record (``v3``: schema-v3 columns too)."""
    evidence = []
    for e in rec["evidence"]:
        item = {k: e.get(k) for k in ("field", "value", "locator", "source_url", "status", "snippet_sha256")}
        if e.get("snippet") is not None:
            item["snippet"] = e["snippet"]
        evidence.append(item)
    rel = rec["relevance_to_sela"]
    row = {
        "id": rec["id"], "title": rec["title"], "authors": rec["authors"], "year": rec["year"],
        "publication_date": rec["publication_date"], "journal": rec["journal"], "doi": rec["doi"],
        "url": rec["source_url"], "theme": rec["theme"], "note_id": rel["note_id"],
        "transfer_limitation_id": rel["transfer_limitation_id"], "study_type": rec["study_type"],
        "population": rec["population"], "country_or_language": rec["country_or_language"],
        "sample_size": rec["sample_size"], "license": rec["license"], "access_status": rec["access_status"],
        "screening_status": rec["screening_status"], "reviewer_status": rec["reviewer_status"],
        "within_cutoff": rec["within_cutoff"], "flags": [f["id"] for f in rec["flags"]],
        "evidence": evidence,
    }
    if not v3:
        return row
    caution = rec.get("study_type_caution")
    row.update({
        "publication_language": rec["publication_language"], "age_range": rec["age_range"],
        "age_band": rec["age_band"]["summary"], "age_groups_indexed": rec["age_groups_indexed"],
        "study_type_caution": caution["caution"] if caution else None,
        "needs_check_documented": rec["needs_check_evidence"] is not None,
        "reviewer_name": rec["reviewer_name"], "review_date": rec["review_date"],
        "review_decision": rec["review_decision"], "findings": rec["findings"],
        "study_type_reviewer_override": rec["study_type_reviewer_override"],
        "publication_date_reviewer_override": rec["publication_date_reviewer_override"],
        "appraisal_tool": rec["appraisal_tool"], "reviewer_2_name": rec["reviewer_2_name"],
        "review_disagreement": rec["review_disagreement"], "adjudicated_by": rec["adjudicated_by"],
        "schema_version": rec["schema_version"],
    })
    return row


def with_index_version(rows):
    """index_version = first 16 hex of sha256 over the rows serialised WITHOUT index_version.

    (Embedding a hash of the final file in the file itself is impossible, so the
    basis excludes the field; verify recomputes it the same way.)
    """
    basis = jsonl_bytes(rows)
    version = sha256(basis)[:16]
    return [dict(r, index_version=version) for r in rows], version


def index_version_of(kb_rows):
    stripped = [{k: v for k, v in r.items() if k != "index_version"} for r in kb_rows]
    return sha256(jsonl_bytes(stripped))[:16]


def atomic_write(path, data):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(path.name + ".tmp")
    tmp.write_bytes(data)
    os.replace(tmp, path)


def check_snapshot(snapshot_dir, files):
    """Raise SnapshotConflict if any existing file differs; return names still to write."""
    to_write = []
    for name, data in sorted(files.items()):
        p = Path(snapshot_dir) / name
        if p.exists():
            if p.read_bytes() != data:
                raise SnapshotConflict(
                    f"snapshot {p} already exists with different content; refusing to overwrite. "
                    "Use a new --freeze-date (or remove the snapshot deliberately in a reviewed commit).")
        else:
            to_write.append(name)
    return to_write


def write_snapshot(snapshot_dir, files):
    for name in check_snapshot(snapshot_dir, files):
        atomic_write(Path(snapshot_dir) / name, files[name])
