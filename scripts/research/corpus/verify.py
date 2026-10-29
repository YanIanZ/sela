"""Schema validation and invariant checks for the exported corpus (offline)."""
from __future__ import annotations

import json
import re
from pathlib import Path

from . import SCHEMA_VERSION
from . import claims as C
from . import profiles as PF
from . import snapshots as SN
from . import v3fields as V3
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
RECORD_SCHEMA_V3 = {
    "publication_language": _OPT_STR, "publication_language_detail": (dict,), "age_range": _OPT_DICT,
    "age_range_detail": (dict,), "age_groups_indexed": _OPT_LIST, "age_band": (dict,),
    "country_evidence_fulltext": _OPT_DICT, "study_type_caution": _OPT_DICT,
    "publication_type_eligibility": (dict,), "needs_check_evidence": _OPT_DICT, "schema_version": _STR,
    "reviewer_name": _OPT_STR, "review_date": _OPT_STR, "review_decision": _OPT_STR, "findings": _OPT_STR,
    "study_type_reviewer_override": _OPT_STR, "publication_date_reviewer_override": _OPT_DICT,
    "country_or_language_reviewer_promoted": _OPT_LIST, "appraisal_tool": _OPT_STR, "reviewer_2_name": _OPT_STR,
    "reviewer_2_decision": _OPT_STR, "review_disagreement": _OPT_BOOL, "adjudicated_by": _OPT_STR,
}
KB_SCHEMA_V3 = {
    "publication_language": _OPT_STR, "age_range": _OPT_DICT, "age_band": _STR, "age_groups_indexed": _OPT_LIST,
    "study_type_caution": _OPT_STR, "needs_check_documented": (bool,), "reviewer_name": _OPT_STR,
    "review_date": _OPT_STR, "review_decision": _OPT_STR, "findings": _OPT_STR,
    "study_type_reviewer_override": _OPT_STR, "publication_date_reviewer_override": _OPT_DICT,
    "appraisal_tool": _OPT_STR, "reviewer_2_name": _OPT_STR, "review_disagreement": _OPT_BOOL,
    "adjudicated_by": _OPT_STR, "schema_version": _STR,
}
AGE_BANDS = V3.AGE_BANDS
_ISO1 = re.compile(r"^[a-z]{2}$")
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


def _machine_status(value):
    return isinstance(value, str) and (value.startswith("machine_") or value == "not_extracted")


def validate_record_v3(rec):
    """Schema-v3 invariants: machine fields stay labelled machine; reviewer fields only from a human entry."""
    rid = rec.get("id", "?")
    errors = check_types(rec, RECORD_SCHEMA_V3, rid)
    if errors:
        return errors
    if rec["schema_version"] != SCHEMA_VERSION:
        errors.append(f"{rid}: schema_version {rec['schema_version']!r} != {SCHEMA_VERSION!r}")
    if rec["reviewer_status"] not in V3.REVIEWER_STATUSES:
        errors.append(f"{rid}: unknown reviewer_status {rec['reviewer_status']!r}")
    prov = rec["provenance"]
    if rec["reviewer_status"] == REVIEWER_STATUS:
        for field in V3.REVIEWER_FIELDS:
            if rec[field] is not None:
                errors.append(f"{rid}: {field} is set but no human review is recorded (reviewer fields stay null)")
    else:
        for field in ("reviewer_name", "review_date", "review_decision"):
            if not rec[field]:
                errors.append(f"{rid}: reviewer_status {rec['reviewer_status']} needs {field}")
        if (prov.get("reviewer_status") or {}).get("source") != "human_reviewer":
            errors.append(f"{rid}: reviewer_status not sourced from the human reviewer file")
    for field in V3.REVIEWER_FIELDS:
        if rec[field] is not None and (prov.get(field) or {}).get("source") != "human_reviewer":
            errors.append(f"{rid}: {field} must come from the human reviewer file, not the machine")
    if rec["findings"] is not None and rec["reviewer_status"] != "human_reviewed_full_text":
        errors.append(f"{rid}: findings require a human full-text review")
    if rec["reviewer_status"] == "human_reviewed_full_text" and not rec["appraisal_tool"]:
        errors.append(f"{rid}: a human full-text review needs its appraisal_tool")
    if rec["review_disagreement"] and not rec["adjudicated_by"]:
        errors.append(f"{rid}: a reviewer disagreement needs an adjudicator")
    lang = rec["publication_language"]
    if lang is not None and not _ISO1.match(lang):
        errors.append(f"{rid}: publication_language {lang!r} is not ISO 639-1")
    for field in ("publication_language_detail", "age_range_detail", "age_band"):
        if not _machine_status(rec[field].get("status")):
            errors.append(f"{rid}: {field}.status must be a machine_* label")
    band = rec["age_band"].get("summary")
    ar = rec["age_range"]
    if band not in AGE_BANDS:
        errors.append(f"{rid}: unknown age_band summary {band!r}")
    elif band in V3.EXPLICIT_BANDS:
        # an explicit band is derived from the accepted age_range alone (audit fix: never from raw candidates)
        if ar is None:
            errors.append(f"{rid}: age_band {band} without an accepted age_range")
        elif (band == "includes_15_17_explicit") != (ar["min"] <= 17 and (ar["max"] if ar["max"] is not None
                                                                           else 999) >= 15):
            errors.append(f"{rid}: age_band {band} disagrees with age_range {ar}")
        elif band == "adults_only_explicit" and ar["min"] < 18:
            errors.append(f"{rid}: age_band adults_only_explicit with age_range minimum {ar['min']}")
    elif band in V3.REVIEW_BANDS and ar is not None:
        errors.append(f"{rid}: review age band with a non-null age_range")
    elif band == "conflicting_explicit_ranges" and (ar is not None or len(rec["age_range_detail"].get(
            "candidate_values") or []) < 2):
        errors.append(f"{rid}: conflicting_explicit_ranges needs a null age_range and several candidates")
    if ar is not None:
        want = f"range:{ar.get('min')}-{ar.get('max')}" if ar.get("kind") == "range" else f"min:{ar.get('min')}"
        if rec["extraction_source"] != "europepmc_oa_fulltext_xml" or not any(
                e.get("field") == "age_range" and e.get("value") == want for e in rec["evidence"]):
            errors.append(f"{rid}: age_range without matching full-text evidence")
    ce = rec["country_evidence_fulltext"]
    if ce is not None:
        if not _machine_status(ce.get("status")):
            errors.append(f"{rid}: country_evidence_fulltext.status must be a machine_* label")
        if ce.get("promoted_labels") is not None and rec["country_or_language_reviewer_promoted"] is None:
            errors.append(f"{rid}: country evidence promoted without a human reviewer entry")
    flag_ids = {f.get("id") for f in rec["flags"]}
    if rec["study_type_caution"] is not None and "study_type_label_conflict" not in flag_ids:
        errors.append(f"{rid}: study_type_caution without the study_type_label_conflict flag")
    if (rec["screening_status"] == "needs_check") != (rec["needs_check_evidence"] is not None):
        errors.append(f"{rid}: needs_check_evidence must be present exactly for needs_check records")
    if rec["needs_check_evidence"] is not None and not _machine_status(rec["needs_check_evidence"].get("status")):
        errors.append(f"{rid}: needs_check_evidence.status must be a machine_* label")
    elig = rec["publication_type_eligibility"]
    if (elig.get("status") == "excluded_publication_type") != ("excluded_publication_type" in flag_ids):
        errors.append(f"{rid}: publication-type exclusion and its flag disagree")
    return errors


def validate_record(rec, freeze_date=None, cutoff_date=None, v3=False):
    """Return a list of human-readable violations for one PUBLIC record."""
    rid = rec.get("id", "?")
    errors = check_types(rec, RECORD_SCHEMA, rid)
    if errors:
        return errors
    statuses = V3.SCREENING_STATUSES_V3 if v3 else SCREENING_STATUSES
    if rec["screening_status"] not in statuses:
        errors.append(f"{rid}: unknown screening_status {rec['screening_status']!r}")
    if rec["access_status"] not in ACCESS_STATUSES:
        errors.append(f"{rid}: unknown access_status {rec['access_status']!r}")
    if not v3 and rec["reviewer_status"] != REVIEWER_STATUS:
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
    if v3:
        errors.extend(validate_record_v3(rec))
    return errors


def validate_kb_row(row, record, v3=False):
    rid = row.get("id", "?")
    errors = check_types(row, KB_SCHEMA, f"kb:{rid}")
    if v3:
        errors += check_types(row, KB_SCHEMA_V3, f"kb:{rid}")
    if errors:
        return errors
    if v3:
        for key in ("publication_language", "age_range", "reviewer_name", "review_date", "review_decision",
                    "findings"):
            if row[key] != record.get(key):
                errors.append(f"kb:{rid}: {key} differs from records.jsonl")
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


def verify_outputs(out_dir, root=None):
    """Re-hash outputs and check schema/invariants. Returns (violations, warnings).

    With ``root`` (the repository root), every frozen snapshot is also checked
    for reproducibility: each recorded input must still exist with the recorded
    bytes, live or as a frozen copy.
    """
    out = Path(out_dir)
    violations, warnings = [], []
    manifest_path = out / "manifest.json"
    if not manifest_path.exists():
        return [f"{manifest_path} missing"], warnings
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    try:
        profile = PF.for_version(manifest.get("pipeline_version"))
    except ValueError as exc:
        return [str(exc)], warnings
    v3 = profile.v3
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
        violations.extend(validate_record(rec, freeze, cutoff, v3=v3))
    by_id = {r["id"]: r for r in records}
    if [r.get("id") for r in kb] != ids:
        violations.append("knowledge_base.jsonl ids differ from records.jsonl")
    for row in kb:
        if row.get("id") in by_id:
            violations.extend(validate_kb_row(row, by_id[row["id"]], v3=v3))
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
    snap = out / "snapshots" / profile.snapshot_name(freeze)
    if v3 and manifest.get("snapshot") != f"snapshots/{snap.name}":
        violations.append(f"manifest snapshot {manifest.get('snapshot')!r} != snapshots/{snap.name}")
    for name in ("manifest.json", "records.jsonl"):
        p = snap / name
        if not p.exists():
            violations.append(f"snapshot {p} missing")
        elif p.read_bytes() != (out / name).read_bytes():
            violations.append(f"snapshot {p} differs from current {name}")
    snaps_root = out / "snapshots"
    for d in sorted(p for p in snaps_root.iterdir() if p.is_dir()) if snaps_root.exists() else []:
        violations.extend(SN.check_snapshot_dir(d, PF.snapshot_profile, root=root))
    emap_path = out / "evidence_map.json"
    if emap_path.exists():
        emap = json.loads(emap_path.read_text(encoding="utf-8"))
        corpus_ids = set(ids)
        missing = sorted({r for e in emap.get("entries", []) for r in e.get("r_ids", []) if r not in corpus_ids})
        if missing or emap.get("missing_r_ids"):
            violations.append(f"evidence_map references R-ids missing from the corpus: {missing or emap['missing_r_ids']}")
        for e in emap.get("entries", []):
            if e.get("relies_on_non_included"):
                overrides = e.get("r_id_reviewer_date_override") or {}
                pending = [r for r in e["relies_on_non_included"] if r not in overrides]
                if pending:
                    documented = sorted(set(pending) & set(e.get("relies_on_needs_check_documented") or []))
                    note = f" (needs_check documented, reviewer date pending: {documented})" if documented else ""
                    warnings.append(f"evidence_map {e['source_doc']} row {e['row']} cites non-included records "
                                    f"{pending}{note}")
            if e.get("relies_on_study_type_caution"):
                warnings.append(f"evidence_map {e['source_doc']} row {e['row']} cites records whose design label "
                                f"carries a caution {e['relies_on_study_type_caution']}")
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
