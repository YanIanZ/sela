"""Schema validation and invariant checks for the exported corpus (offline)."""
from __future__ import annotations

import json
import re
from pathlib import Path

from . import claims as C
from .export import SnapshotConflict, index_version_of, sha256  # noqa: F401 (re-export for callers)
from .records import ACCESS_STATUSES, REVIEWER_STATUS, SCREENING_STATUSES
from .rights import REDISTRIBUTABLE_FAMILIES

_STR = (str,)
_OPT_STR = (str, type(None))
_OPT_INT = (int, type(None))
_OPT_BOOL = (bool, type(None))
_OPT_LIST = (list, type(None))
_OPT_DICT = (dict, type(None))
RECORD_SCHEMA = {
    "id": _STR, "title": _STR, "authors": (list,), "publication_date": _OPT_STR, "doi": _OPT_STR,
    "source_url": _OPT_STR, "study_type": _OPT_STR, "population": _OPT_LIST, "country_or_language": _OPT_LIST,
    "sample_size": _OPT_INT, "intervention_or_exposure": _OPT_DICT, "comparator": _OPT_DICT,
    "outcomes": _OPT_DICT, "limitations": _OPT_DICT, "relevance_to_sela": (dict,), "license": _OPT_STR,
    "access_status": _STR, "screening_status": _STR, "extraction_source": _STR, "retrieval_date": _OPT_STR,
    "reviewer_status": _STR, "pmid": _OPT_STR, "pmcid": _OPT_STR, "journal": _OPT_STR, "theme": _OPT_STR,
    "dates": (dict,), "date_discrepancies": (list,), "within_cutoff": _OPT_BOOL, "retraction_status": _STR,
    "text_processing_allowed": (bool,), "redistribution_allowed": (bool,), "flags": (list,),
    "evidence": (list,), "provenance": (dict,), "year": _OPT_INT, "license_detail": (dict,),
}
KB_SCHEMA = {
    "id": _STR, "title": _STR, "authors": (list,), "year": _OPT_INT, "publication_date": _OPT_STR,
    "journal": _OPT_STR, "doi": _OPT_STR, "url": _OPT_STR, "theme": _OPT_STR, "note_id": _OPT_STR,
    "transfer_limitation_id": _OPT_STR, "study_type": _OPT_STR, "population": _OPT_LIST,
    "country_or_language": _OPT_LIST, "sample_size": _OPT_INT, "license": _OPT_STR, "access_status": _STR,
    "screening_status": _STR, "reviewer_status": _STR, "within_cutoff": _OPT_BOOL, "flags": (list,),
    "evidence": (list,), "index_version": _STR,
}
RETRACTION_STATUSES = ("retracted", "partially_retracted", "is_update_notice", "expression_of_concern",
                       "corrected", "none_found", "unknown")
MAX_STRING = 2000  # guards against full text slipping into committed files
_DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")


def _walk_strings(obj, path="$"):
    if isinstance(obj, str):
        yield path, obj
    elif isinstance(obj, dict):
        for k, v in obj.items():
            yield from _walk_strings(v, f"{path}.{k}")
    elif isinstance(obj, list):
        for i, v in enumerate(obj):
            yield from _walk_strings(v, f"{path}[{i}]")


def check_types(obj, schema, where):
    errors = []
    for key, types in schema.items():
        if key not in obj:
            errors.append(f"{where}: missing field {key!r}")
        elif type(obj[key]) is bool and bool not in types:
            errors.append(f"{where}: {key!r} has type bool")
        elif not isinstance(obj[key], types):
            errors.append(f"{where}: {key!r} has type {type(obj[key]).__name__}")
    return errors


def validate_record(rec, freeze_date=None, cutoff_date=None):
    """Return a list of human-readable violations for one PUBLIC record."""
    rid = rec.get("id", "?")
    errors = check_types(rec, RECORD_SCHEMA, rid)
    if errors:
        return errors
    if rec["screening_status"] not in SCREENING_STATUSES:
        errors.append(f"{rid}: unknown screening_status {rec['screening_status']!r}")
    if rec["access_status"] not in ACCESS_STATUSES:
        errors.append(f"{rid}: unknown access_status {rec['access_status']!r}")
    if rec["reviewer_status"] != REVIEWER_STATUS:
        errors.append(f"{rid}: reviewer_status must be {REVIEWER_STATUS!r} (no human review happened)")
    if rec["retraction_status"] not in RETRACTION_STATUSES:
        errors.append(f"{rid}: unknown retraction_status {rec['retraction_status']!r}")
    pd = rec["publication_date"]
    if pd is not None and not _DATE.match(pd):
        errors.append(f"{rid}: publication_date {pd!r} is not day precision")
    if freeze_date and cutoff_date:
        expected = None if pd is None else (cutoff_date <= pd <= freeze_date)
        if rec["within_cutoff"] != expected:
            errors.append(f"{rid}: within_cutoff {rec['within_cutoff']} != {expected} for {pd}")
    lic = rec["license_detail"]
    if rec["text_processing_allowed"] and not lic.get("is_creative_commons"):
        errors.append(f"{rid}: text_processing_allowed without an explicit CC licence")
    if rec["redistribution_allowed"] and lic.get("family") not in REDISTRIBUTABLE_FAMILIES:
        errors.append(f"{rid}: redistribution_allowed for licence family {lic.get('family')!r}")
    status = rec["screening_status"]
    if (status == "excluded_retracted") != (rec["retraction_status"] == "retracted"):
        errors.append(f"{rid}: excluded_retracted must match retraction_status 'retracted'")
    if status == "included_title_abstract_v0" and rec["within_cutoff"] is not True:
        errors.append(f"{rid}: included record outside window")
    if status == "excluded_out_of_window" and rec["within_cutoff"] is not False:
        errors.append(f"{rid}: out-of-window status but within_cutoff={rec['within_cutoff']}")
    from_fulltext = rec["extraction_source"] == "europepmc_oa_fulltext_xml"
    if from_fulltext and not rec["text_processing_allowed"]:
        errors.append(f"{rid}: full text processed without text_processing_allowed")
    if rec["evidence"] and not from_fulltext:
        errors.append(f"{rid}: evidence present but extraction_source is {rec['extraction_source']!r}")
    for i, e in enumerate(rec["evidence"]):
        for key in ("field", "locator", "source_url", "status", "snippet_sha256"):
            if key not in e:
                errors.append(f"{rid}: evidence[{i}] missing {key!r}")
        if e.get("status") != "machine_extracted_unreviewed":
            errors.append(f"{rid}: evidence[{i}] status must be machine_extracted_unreviewed")
        snip = e.get("snippet")
        if snip is not None and (not rec["redistribution_allowed"] or len(snip) > 240):
            errors.append(f"{rid}: evidence[{i}] snippet not allowed (licence) or longer than 240 chars")
    if rec["sample_size"] is not None:
        if not from_fulltext or not any(e.get("field") == "sample_size" and e.get("value") == rec["sample_size"]
                                        for e in rec["evidence"]):
            errors.append(f"{rid}: sample_size without matching full-text evidence")
    for field in ("intervention_or_exposure", "comparator", "outcomes", "limitations"):
        v = rec[field]
        if v is not None:
            if not from_fulltext or "locator" not in v:
                errors.append(f"{rid}: {field} set without full-text locator")
            if v.get("snippet") is not None and not rec["redistribution_allowed"]:
                errors.append(f"{rid}: {field} snippet present without redistribution right")
    for f in rec["flags"]:
        if f.get("assigned_by") != "machine":
            errors.append(f"{rid}: flag {f.get('id')} not marked machine-assigned")
    for key in ("publication_date", "license", "access_status", "screening_status", "sample_size"):
        if key not in rec["provenance"]:
            errors.append(f"{rid}: provenance missing {key!r}")
    for path, s in _walk_strings(rec):
        if len(s) > MAX_STRING:
            errors.append(f"{rid}: string at {path} longer than {MAX_STRING} chars")
    return errors


def validate_kb_row(row, record):
    rid = row.get("id", "?")
    errors = check_types(row, KB_SCHEMA, f"kb:{rid}")
    if errors:
        return errors
    for key in ("title", "doi", "publication_date", "license", "access_status", "screening_status",
                "within_cutoff", "sample_size", "study_type"):
        if row[key] != record.get(key):
            errors.append(f"kb:{rid}: {key} differs from records.jsonl")
    for e in row["evidence"]:
        if "snippet" in e and not record["redistribution_allowed"]:
            errors.append(f"kb:{rid}: snippet exported without redistribution right")
    return errors


def check_claim_gates(doc):
    errors = []
    for gate in doc.get("gates", []):
        try:
            patterns = [re.compile(p, re.IGNORECASE) for p in gate["patterns"]]
        except re.error as exc:
            errors.append(f"claim_gates:{gate.get('id')}: pattern does not compile ({exc})")
            continue
        if not gate.get("sources"):
            errors.append(f"claim_gates:{gate['id']}: no source section")
        for ex in gate.get("forbidden_examples", []):
            if not any(p.search(ex) for p in patterns):
                errors.append(f"claim_gates:{gate['id']}: forbidden example not matched: {ex!r}")
    for gate in doc.get("gates", []):
        for ex in gate.get("allowed_examples", []):
            if C.scan_text(ex, doc["gates"]):
                errors.append(f"claim_gates:{gate['id']}: allowed example matched: {ex!r}")
    return errors


def _read_jsonl(path):
    return [json.loads(line) for line in Path(path).read_text(encoding="utf-8").splitlines() if line.strip()]


def verify_outputs(out_dir):
    """Re-hash outputs and check schema/invariants. Returns (violations, warnings)."""
    out = Path(out_dir)
    violations, warnings = [], []
    manifest_path = out / "manifest.json"
    if not manifest_path.exists():
        return [f"{manifest_path} missing"], warnings
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    for rel, digest in sorted(manifest.get("outputs", {}).items()):
        p = out / rel
        if not p.exists():
            violations.append(f"output {rel} missing")
        elif sha256(p.read_bytes()) != digest:
            violations.append(f"output {rel} sha256 differs from manifest")
    try:
        records = _read_jsonl(out / "records.jsonl")
        kb = _read_jsonl(out / "knowledge_base.jsonl")
    except (OSError, ValueError) as exc:
        return violations + [f"cannot read records/knowledge base: {exc}"], warnings
    ids = [r.get("id") for r in records]
    if ids != sorted(ids) or len(set(ids)) != len(ids):
        violations.append("records.jsonl ids must be unique and sorted")
    if len(records) != manifest.get("counts", {}).get("records"):
        violations.append("record count differs from manifest")
    freeze, cutoff = manifest.get("freeze_date"), manifest.get("cutoff_date")
    for rec in records:
        violations.extend(validate_record(rec, freeze, cutoff))
    by_id = {r["id"]: r for r in records}
    if [r.get("id") for r in kb] != ids:
        violations.append("knowledge_base.jsonl ids differ from records.jsonl")
    for row in kb:
        if row.get("id") in by_id:
            violations.extend(validate_kb_row(row, by_id[row["id"]]))
    version = index_version_of(kb)
    if version != manifest.get("index_version"):
        violations.append("index_version in manifest does not match knowledge base content")
    if any(r.get("index_version") != version for r in kb):
        violations.append("knowledge base rows carry a stale index_version")
    counts = manifest.get("counts", {})
    observed = {}
    for r in records:
        observed[r["screening_status"]] = observed.get(r["screening_status"], 0) + 1
    if observed != counts.get("screening_status"):
        violations.append("screening_status counts differ from manifest")
    snap = out / "snapshots" / str(freeze)
    for name in ("manifest.json", "records.jsonl"):
        p = snap / name
        if not p.exists():
            violations.append(f"snapshot {p} missing")
        elif p.read_bytes() != (out / name).read_bytes():
            violations.append(f"snapshot {p} differs from current {name}")
    emap_path = out / "evidence_map.json"
    if emap_path.exists():
        emap = json.loads(emap_path.read_text(encoding="utf-8"))
        corpus_ids = set(ids)
        missing = sorted({r for e in emap.get("entries", []) for r in e.get("r_ids", []) if r not in corpus_ids})
        if missing or emap.get("missing_r_ids"):
            violations.append(f"evidence_map references R-ids missing from the corpus: {missing or emap['missing_r_ids']}")
        for e in emap.get("entries", []):
            if e.get("relies_on_non_included"):
                warnings.append(f"evidence_map {e['source_doc']} row {e['row']} cites non-included records "
                                f"{e['relies_on_non_included']}")
    else:
        violations.append("evidence_map.json missing")
    gates_path = out / "eval_fixtures" / "claim_gates.json"
    if gates_path.exists():
        violations.extend(check_claim_gates(json.loads(gates_path.read_text(encoding="utf-8"))))
    else:
        violations.append("eval_fixtures/claim_gates.json missing")
    for p in sorted(out.rglob("*")):
        rel = p.relative_to(out)
        if rel.parts and rel.parts[0] == ".cache":
            continue
        if p.is_file() and p.suffix not in (".json", ".jsonl"):
            violations.append(f"unexpected file type in outputs (full text must stay in .cache): {rel}")
    return violations, warnings
