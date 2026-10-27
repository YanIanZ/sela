#!/usr/bin/env python3
"""Reproducible research CORPUS pipeline over Sela's 100-publication register.

  fetch   fill the local API cache (Europe PMC, Crossref, OpenAlex, doi.org;
          open-access full text only where a Creative Commons licence allows)
  build   offline: cache + register -> data/research/corpus/ outputs
          (--pipeline-version 1.1.0 rebuilds the legacy schema of the frozen
          2026-10-26 snapshot; the default is the current version)
  reproduce  offline: rebuild a frozen snapshot into a scratch directory and
          compare it byte for byte with the committed one (exit 1 on any difference)
  verify  offline: re-hash outputs and check schema/invariants (exit 1 on violation)
  scan-claims FILE...  report forbidden product claims (eval_fixtures/claim_gates.json)

Stdlib-only. Not a systematic review, not full-text appraisal, not clinical
evidence and not training data. Every record keeps reviewer_status
'not_reviewed'. Full text is cached under data/research/corpus/.cache/
(gitignored) and is never written to the committed outputs.
See docs/research/CORPUS_PIPELINE.md.
"""
from __future__ import annotations

import argparse
import collections
import datetime as dt
import json
import os
import shutil
import sys
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from corpus import DEFAULT_FREEZE_DATE, PIPELINE_VERSION  # noqa: E402
from corpus import claims as C  # noqa: E402
from corpus import dates as D  # noqa: E402
from corpus import evidence as EV  # noqa: E402
from corpus import export as E  # noqa: E402
from corpus import fetch as F  # noqa: E402
from corpus import ids  # noqa: E402
from corpus import ingest as IN  # noqa: E402
from corpus import profiles as PF  # noqa: E402
from corpus import records as R  # noqa: E402
from corpus import reviews as RV  # noqa: E402
from corpus import snapshots as SN  # noqa: E402
from corpus import rights as RT  # noqa: E402
from corpus import verify as V  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
RELAY = Path("docs/research/sela-relay")
DEFAULT_OUT = Path("data/research/corpus")
REGISTER_INPUTS = Path("data/research/register_inputs")
REVIEWER_FILE = REGISTER_INPUTS / "reviewer_decisions.json"
# Frozen copies of inputs that changed after a snapshot was frozen, mirrored at
# their repository-relative paths: FROZEN_INPUTS/<snapshot name>/<path>.
FROZEN_INPUTS = SN.FROZEN_INPUTS


class MissingCache(RuntimeError):
    pass


class Inputs:
    """Input documents, resolved against ``root``."""

    def __init__(self, root=ROOT, relay=RELAY):
        self.root = Path(root)
        base = self.root / relay
        self.register_json = base / "research_100.json"
        self.bib = base / "research_100.bib"
        self.register_md = base / "09_RESEARCH_REGISTER.md"
        self.doc07 = base / "07_EVIDENCE_TO_FEATURE.md"
        self.doc10 = base / "10_EVIDENCE_APPRAISAL_AND_CLAIMS.md"
        self.reviewer_file = self.root / REVIEWER_FILE

    def all(self, profile=None):
        """Inputs hashed into the manifest; the legacy profile keeps its original five."""
        files = [self.register_json, self.bib, self.register_md, self.doc07, self.doc10]
        if (profile or PF.CURRENT).v3:
            files.append(self.reviewer_file)
        return files

    def rel(self, path):
        try:
            return Path(path).resolve().relative_to(self.root.resolve()).as_posix()
        except ValueError:
            return Path(path).name


def load_papers(inputs):
    _, papers = IN.load_register_json(inputs.register_json.read_bytes())
    return sorted(papers, key=lambda p: p["id"])


def metadata_requests(paper):
    doi = ids.normalize_doi(paper.get("doi"))
    if not ids.is_valid_doi(doi):
        return doi, []
    return doi, [
        ("europepmc_doi", "europepmc") + F.epmc_doi_request(doi),
        ("crossref", "crossref") + F.crossref_request(doi),
        ("openalex", "openalex") + F.openalex_request(doi),
        ("doi_handle", "doi_handle") + F.doi_handle_request(doi),
    ]


def needs_pmid_fallback(envs, doi, pmid):
    if not pmid:
        return False
    chosen, _, _ = R.select_epmc([envs.get("europepmc_doi")], doi, pmid)
    return chosen is None


def fulltext_target(envs, doi, pmid, freeze_date):
    """PMCID to fetch iff the cached metadata shows an explicit CC licence and OA full text."""
    epmc, _, _ = R.select_epmc([envs.get("europepmc_doi"), envs.get("europepmc_pmid")], doi, pmid)
    cr = (R.body_if_ok(envs.get("crossref")) or {}).get("message")
    lic = RT.licence_detail(cr, epmc, freeze_date)
    pmcid = ids.normalize_pmcid((epmc or {}).get("pmcid"))
    if lic["text_processing_allowed"] and pmcid and RT.fulltext_in_europepmc(epmc):
        return pmcid
    return None


def cached_envs(cache, paper, freeze_date):
    """Collect cached envelopes for one paper; report what is missing."""
    doi, reqs = metadata_requests(paper)
    pmid = ids.pmid_from_url(paper.get("index_url"))
    envs, missing = {}, []
    for slot, service, key, _url in reqs:
        envs[slot] = cache.get(service, key)
        if envs[slot] is None:
            missing.append((paper["id"], service, key))
    if envs.get("europepmc_doi") is not None and needs_pmid_fallback(envs, doi, pmid):
        key, _ = F.epmc_pmid_request(pmid)
        envs["europepmc_pmid"] = cache.get("europepmc", key)
        if envs["europepmc_pmid"] is None:
            missing.append((paper["id"], "europepmc", key))
    xml = None
    pmcid = fulltext_target(envs, doi, pmid, freeze_date) if not missing else None
    if pmcid:
        key, _ = F.fulltext_request(pmcid)
        envs["fulltext"] = cache.get("fulltext", key)
        if envs["fulltext"] is None:
            missing.append((paper["id"], "fulltext", key))
        elif envs["fulltext"].get("status") == "ok":
            xml = cache.get_text("fulltext", key)
            if xml is None:
                missing.append((paper["id"], "fulltext", key + " (xml body)"))
    return envs, xml, missing


def needs_check_requests(paper, envs, freeze_date):
    """v3: extra date lookups for records the legacy rules leave at needs_check."""
    cutoff = D.cutoff_for(freeze_date)
    rec = R.build_record(paper, envs, freeze_date, cutoff, {}, profile=PF.LEGACY)
    if rec["screening_status"] != "needs_check":
        return []
    out = []
    if rec["pmid"]:
        out.append(("pubmed_history", "pubmed_history") + F.pubmed_history_request(rec["pmid"]) + (rec["pmid"],))
    if rec["doi"]:
        out.append(("landing_meta", "landing_meta") + F.landing_request(rec["doi"]) + (rec["doi"],))
    return out


def cached_v3_envs(cache, paper, envs, freeze_date):
    missing = []
    for slot, service, key, _url, _arg in needs_check_requests(paper, envs, freeze_date):
        envs[slot] = cache.get(service, key)
        if envs[slot] is None:
            missing.append((paper["id"], service, key))
    return missing


# --- fetch --------------------------------------------------------------------

def run_fetch(inputs, cache_dir, freeze_date, offline=False, refresh=False, only=None, client=None, log=print):
    cache = F.Cache(cache_dir)
    papers = [p for p in load_papers(inputs) if not only or p["id"] in only]
    if client is None and not offline:
        client = F.Client(mailto=os.environ.get("SELA_RESEARCH_MAILTO"))
    pool = "polite" if (client and client.mailto) else "public"
    errors, missing, fetched = [], [], collections.Counter()

    def get(service, key, url, rid):
        if offline:
            env = cache.get(service, key)
            if env is None:
                missing.append((rid, service, key))
            return env
        extra = {"pool": pool} if service == "crossref" else None
        env, did = F.fetch_json(client, cache, service, key, url, refresh=refresh, extra=extra)
        if "fetch_error" in env:
            errors.append({"id": rid, "service": service, "key": key, "error": env["fetch_error"]})
            return None
        if did:
            fetched[service] += 1
        return env

    for n, paper in enumerate(papers, start=1):
        rid = paper["id"]
        doi, reqs = metadata_requests(paper)
        pmid = ids.pmid_from_url(paper.get("index_url"))
        if not reqs:
            errors.append({"id": rid, "service": "register", "key": str(doi), "error": "invalid DOI"})
            continue
        envs = {slot: get(service, key, url, rid) for slot, service, key, url in reqs}
        if envs.get("europepmc_doi") is not None and needs_pmid_fallback(envs, doi, pmid):
            key, url = F.epmc_pmid_request(pmid)
            envs["europepmc_pmid"] = get("europepmc", key, url, rid)
        pmcid = fulltext_target(envs, doi, pmid, freeze_date)
        if pmcid:
            key, url = F.fulltext_request(pmcid)
            if offline:
                env = cache.get("fulltext", key)
                if env is None or (env.get("status") == "ok" and cache.get_text("fulltext", key) is None):
                    missing.append((rid, "fulltext", key))
            else:
                env, did = F.fetch_fulltext(client, cache, pmcid, refresh=refresh)
                if "fetch_error" in env:
                    errors.append({"id": rid, "service": "fulltext", "key": key, "error": env["fetch_error"]})
                elif did:
                    fetched["fulltext"] += 1
        if all(envs.get(slot) is not None for slot, *_ in reqs):
            for slot, service, key, url, arg in needs_check_requests(paper, envs, freeze_date):
                if offline:
                    if cache.get(service, key) is None:
                        missing.append((rid, service, key))
                    continue
                fetcher = F.fetch_pubmed_history if service == "pubmed_history" else F.fetch_landing_meta
                env, did = fetcher(client, cache, arg, refresh=refresh)
                if "fetch_error" in env:
                    errors.append({"id": rid, "service": service, "key": key, "error": env["fetch_error"]})
                elif did:
                    fetched[service] += 1
        if n % 10 == 0 or n == len(papers):
            log(f"[{n}/{len(papers)}] {rid} done; new responses this run: {dict(sorted(fetched.items()))}")
    report = {
        "finished_at": F.utc_now(), "freeze_date": freeze_date, "offline": offline,
        "records": len(papers), "new_responses": dict(sorted(fetched.items())),
        "http_requests": client.requests if client else 0, "crossref_pool": pool,
        "errors": errors, "missing": [list(m) for m in missing], "pipeline_version": PIPELINE_VERSION,
    }
    Path(cache_dir).mkdir(parents=True, exist_ok=True)
    report_name = "cache_check_report.json" if offline else "fetch_report.json"  # never clobber the online log
    (Path(cache_dir) / report_name).write_text(json.dumps(report, indent=1, sort_keys=True) + "\n",
                                               encoding="utf-8")
    for e in errors:
        log(f"ERROR {e['id']} {e['service']}: {e['error']}")
    if offline:
        log(f"offline cache check: {len(missing)} missing entries")
    return 1 if errors or missing else 0


# --- build --------------------------------------------------------------------

def _count(values):
    return dict(sorted(collections.Counter("null" if v is None else str(v).lower() if isinstance(v, bool) else str(v)
                                           for v in values).items()))


def year_check(doc10_text, doc10_rel, papers, records):
    claimed, raw = EV.parse_year_claim(doc10_text)
    if claimed is None:
        return {"claim_source": doc10_rel, "claimed": None, "note": "claim not found in document"}
    json_year = collections.Counter(str(p.get("publication_year")) for p in papers)
    json_first = collections.Counter(str(p.get("first_publication_date"))[:4] for p in papers)
    verified = collections.Counter(str(r["year"]) for r in records if r["year"] is not None)
    unknown = sum(1 for r in records if r["year"] is None)
    a = EV.compare_distribution(claimed, json_year)
    b = EV.compare_distribution(claimed, json_first)
    c = EV.compare_distribution(claimed, verified)
    c["records_without_verified_date"] = unknown
    parts = []
    parts.append("claim equals the JSON publication_year field" if a["matches_claim"]
                 else "claim differs from the JSON publication_year field")
    parts.append("equals the year of JSON first_publication_date" if b["matches_claim"]
                 else "differs from the year of JSON first_publication_date")
    parts.append("equals verified canonical dates" if c["matches_claim"]
                 else f"differs from verified canonical dates ({unknown} record(s) without a verified date)")
    return {
        "claim_source": f"{doc10_rel} (Verified local register integrity: First-publication year)",
        "claimed_text": raw, "claimed": claimed,
        "vs_json_publication_year": a, "vs_json_first_publication_date_year": b,
        "vs_verified_canonical_dates": c,
        "conclusion": "; ".join(parts) + ".",
    }


def v3_counts(records, reviews):
    return {
        "schema_version": PF.CURRENT.schema_version,
        "publication_language": _count(r["publication_language"] for r in records),
        "publication_language_basis": _count(r["publication_language_detail"]["basis"] for r in records),
        "age_range_extracted": sum(1 for r in records if r["age_range"] is not None),
        "age_range_conflicting": sum(1 for r in records if r["age_range_detail"]["note"].startswith("conflicting")),
        "age_groups_indexed_present": sum(1 for r in records if r["age_groups_indexed"]),
        "age_band_summary": _count(r["age_band"]["summary"] for r in records),
        "country_evidence_fulltext_with_candidates": sum(
            1 for r in records if (r["country_evidence_fulltext"] or {}).get("candidates")),
        "study_type_caution": sorted(r["id"] for r in records if r["study_type_caution"]),
        "publication_type_eligibility": _count(r["publication_type_eligibility"]["status"] for r in records),
        "needs_check_documented": sorted(r["id"] for r in records if r["needs_check_evidence"]),
        "reviewer_status": _count(r["reviewer_status"] for r in records),
        "reviewer_entries": len(reviews),
        "findings_set": sum(1 for r in records if r["findings"] is not None),
    }


def make_manifest(inputs, freeze_date, cutoff, papers, records, public, kb_version, files, cross,
                  dup_register, dup_enriched, emap, used_envs, year_info, profile=None, reviews=None):
    profile = profile or PF.CURRENT
    discrepancies = [dict(d, id=r["id"]) for r in records for d in r["date_discrepancies"]]
    by_service = {}
    pools = collections.Counter()
    times = []
    for env in used_envs:
        s = by_service.setdefault(env["service"], {"endpoint": env["endpoint"], "responses": {}})
        s["responses"][env["status"]] = s["responses"].get(env["status"], 0) + 1
        times.append(env["fetched_at"])
        if env["service"] == "crossref" and env.get("pool"):
            pools[env["pool"]] += 1
    sample_detail = [r["sample_size_detail"] for r in records if r["extraction_source"] != "metadata_only"]
    counts = {
        "records": len(records),
        "screening_status": _count(r["screening_status"] for r in records),
        "within_cutoff": _count(r["within_cutoff"] for r in records),
        "year_verified": _count(r["year"] for r in records),
        "access_status": _count(r["access_status"] for r in records),
        "license": _count(r["license"] for r in records),
        "license_family": _count(r["license_detail"]["family"] for r in records),
        "retraction_status": _count(r["retraction_status"] for r in records),
        "study_type_primary": _count(r["study_type"] for r in records),
        "theme": _count(r["theme"] for r in records),
        "doi_resolvable": _count(r["doi_resolvable"] for r in records),
        "in_europepmc": sum(1 for r in records if r["europepmc_id"]),
        "with_pmcid": sum(1 for r in records if r["pmcid"]),
        "text_processing_allowed": sum(1 for r in records if r["text_processing_allowed"]),
        "redistribution_allowed": sum(1 for r in records if r["redistribution_allowed"]),
        "fulltext_eligible": sum(1 for r in records if r["text_processing_allowed"] and r["pmcid"]),
        "fulltexts_processed": sum(1 for r in records if r["extraction_source"] == "europepmc_oa_fulltext_xml"),
        # Disjoint outcomes over processed full texts: review | extracted | conflicting | none found.
        "sample_size_review_left_null": sum(1 for d in sample_detail if d["note"].startswith("review-type")),
        "sample_sizes_extracted": sum(1 for r in records if r["sample_size"] is not None),
        "sample_size_conflicting": sum(1 for d in sample_detail if d["note"].startswith("conflicting")),
        "sample_size_none_found": sum(1 for d in sample_detail if d["note"].startswith("no explicit")),
        "labelled_sections": {f: sum(1 for r in records if r[f] is not None)
                              for f in ("intervention_or_exposure", "comparator", "outcomes", "limitations")},
        "evidence_items": sum(len(r["evidence"]) for r in records),
        "date_discrepancies_total": len(discrepancies),
        "records_with_date_discrepancies": sum(1 for r in records if r["date_discrepancies"]),
        "date_discrepancies_by_type": _count(d["type"] for d in discrepancies),
        "date_discrepancies_by_kind": _count(d["kind"] for d in discrepancies),
        "records_with_primary_date_discrepancy": sum(
            1 for r in records if any(d["kind"] == "primary" for d in r["date_discrepancies"])),
        "records_with_register_date_discrepancy": sum(
            1 for r in records if any(d["kind"] == "register" for d in r["date_discrepancies"])),
        "flags": _count(f["id"] for r in records for f in r["flags"]),
    }
    manifest = {
        "dataset": "Sela research corpus: 100-publication register re-verified against public metadata",
        "status": "machine_verified_metadata_not_reviewed",
        "pipeline_version": profile.pipeline_version,
        "rules_version": profile.rules_version,
        "claim_gates_version": C.GATES_VERSION,
        "freeze_date": freeze_date,
        "cutoff_date": cutoff,
        "window_rule": ("cutoff_date <= publication_date <= freeze_date; cutoff_date = freeze_date minus five "
                        "calendar years (29 February maps to 28 February)"),
        "canonical_date_rule": ("earliest DAY-precision date from crossref.published_online, "
                                "europepmc.first_publication_date or europepmc.electronic_publication_date; "
                                "discrepancy when |diff| > 31 days, year differs, or a primary source is "
                                "month/year precision"),
        "index_version": kb_version,
        "index_version_basis": "first 16 hex of sha256(knowledge_base.jsonl rows serialised without index_version)",
        "inputs": {inputs.rel(p): IN.sha256_file(p) for p in inputs.all(profile)},
        "outputs": {name: E.sha256(data) for name, data in sorted(files.items())},
        "counts": counts,
        "date_discrepancies": sorted(discrepancies, key=lambda d: (d["id"], d["source"], d["type"])),
        "year_distribution_check": year_info,
        "input_cross_checks": {"count": len(cross), "issues": cross},
        "duplicates": {"register_fields": dup_register, "after_enrichment": dup_enriched,
                       "count": len(dup_register) + len(dup_enriched)},
        "evidence_map": {"entries": len(emap["entries"]), "missing_r_ids": emap["missing_r_ids"],
                         "entries_citing_non_included": sum(1 for e in emap["entries"] if e["relies_on_non_included"])},
        "sources": by_service,
        "crossref_pool": dict(sorted(pools.items())),
        "retrieval_window": {"first": min(times) if times else None, "last": max(times) if times else None},
        "rights_policy": {
            "metadata": "bibliographic metadata from public APIs, with source and retrieval time per field",
            "annotations": "register notes (note_id, transfer_limitation_id) are Sela's own text",
            "full_text": "processed only under an explicit CC licence; cached locally (gitignored); never committed",
            "snippets": "verbatim <= 240 chars only for CC BY / BY-SA / CC0; otherwise locator + sha256 only",
            "training_use": "none; this corpus is not a training dataset",
            "inference_rag": "not used; product answers never retrieve literature",
        },
        "limitations": [
            "Not a systematic review; the 100 records were selected earlier and are re-verified, not re-searched.",
            "reviewer_status is not_reviewed for every record: no human reviewed any field produced here.",
            "Screening is inherited title/abstract screening; no full-text appraisal or risk-of-bias assessment.",
            "Population and country terms are explicit-word matches in titles/annotations, not verified study populations.",
            "Full-text extraction is pattern-based and limited to CC-licensed Europe PMC open-access XML.",
            "Licence fields reflect metadata published by Crossref and Europe PMC, not legal advice.",
        ],
    }
    if not profile.v3:
        return manifest
    manifest["schema_version"] = profile.schema_version
    manifest["snapshot"] = f"snapshots/{profile.snapshot_name(freeze_date)}"
    manifest["counts"]["v3"] = v3_counts(records, reviews or {})
    manifest["reviewer_policy"] = (
        "reviewer_name, review_date, review_decision, findings and the *_reviewer_* overrides are copied only from "
        "human entries in data/research/register_inputs/reviewer_decisions.json; machine fields are never "
        "overwritten; with no entry every reviewer field is null and reviewer_status is not_reviewed")
    manifest["supersedes"] = {
        "pipeline_version": PF.LEGACY.pipeline_version,
        "snapshot": f"snapshots/{PF.LEGACY.snapshot_name(freeze_date)}",
        "note": ("the 1.1.0 snapshot is frozen and still rebuilds byte for byte with "
                 "`corpus_pipeline.py reproduce --snapshot " + PF.LEGACY.snapshot_name(freeze_date) + "`"),
    }
    manifest["limitations"] = manifest["limitations"] + [
        "publication_language comes from index/publisher metadata (OpenAlex only as a flagged machine inference).",
        "age_range, age_band and country_evidence_fulltext are pattern matches with locators, not verified "
        "study populations; adult evidence is never generalised to ages 15-17.",
        "needs_check_evidence documents public PubMed and landing-page dates; it does not change any status.",
    ]
    return manifest


def run_build(inputs, cache_dir, out_dir, freeze_date, allow_missing=False, profile=None):
    profile = profile or PF.CURRENT
    cutoff = D.cutoff_for(freeze_date)
    papers = load_papers(inputs)
    bib = IN.parse_bib(inputs.bib.read_text(encoding="utf-8"))
    md = IN.parse_register_md(inputs.register_md.read_text(encoding="utf-8"))
    cross = IN.cross_check(papers, bib, md)
    reviews = {}
    if profile.v3:
        reviews = RV.load_reviews(inputs.reviewer_file.read_bytes(), [p["id"] for p in papers])
    dup_register = ids.find_duplicates([
        {"id": p["id"], "doi": p.get("doi"), "pmid": ids.pmid_from_url(p.get("index_url")),
         "title": p.get("title"), "year": p.get("publication_year")} for p in papers])
    doc07, doc10 = inputs.doc07.read_text(encoding="utf-8"), inputs.doc10.read_text(encoding="utf-8")
    rid_list = [p["id"] for p in papers]
    pre_map = EV.build_evidence_map(inputs.rel(inputs.doc07), doc07, inputs.rel(inputs.doc10), doc10, rid_list, {})
    cache = F.Cache(cache_dir)
    records, missing, used_envs = [], [], []
    for paper in papers:
        envs, xml, miss = cached_envs(cache, paper, freeze_date)
        if profile.v3 and not miss:
            miss += cached_v3_envs(cache, paper, envs, freeze_date)
        missing.extend(miss)
        rec = R.build_record(paper, envs, freeze_date, cutoff, pre_map["features_by_r_id"], fulltext_xml=xml,
                             profile=profile, review=reviews.get(paper["id"]))
        records.append(rec)
        for slot, env in sorted(envs.items()):
            if env and (slot != "fulltext" or rec["extraction_source"] != "metadata_only"):
                used_envs.append(env)
    if missing and not allow_missing:
        raise MissingCache(f"{len(missing)} cache entries missing (run fetch first), e.g. {missing[:3]}")
    dup_enriched = ids.find_duplicates([
        {"id": r["id"], "doi": r["doi"], "pmid": r["pmid"], "pmcid": r["pmcid"], "title": r["title"],
         "year": r["year"]} for r in records])
    emap = EV.build_evidence_map(inputs.rel(inputs.doc07), doc07, inputs.rel(inputs.doc10), doc10, rid_list,
                                 {r["id"]: r["screening_status"] for r in records})
    emap["pipeline_version"] = profile.pipeline_version
    emap["inputs"] = {inputs.rel(inputs.doc07): IN.sha256_file(inputs.doc07),
                      inputs.rel(inputs.doc10): IN.sha256_file(inputs.doc10)}
    if profile.v3:
        EV.annotate_v3(emap, {r["id"]: r for r in records})
    public = [E.public_record(r) for r in records]
    kb, version = E.with_index_version([E.kb_row(r, v3=profile.v3) for r in public])
    gates = C.export_document()
    files = {
        "records.jsonl": E.jsonl_bytes(public),
        "knowledge_base.jsonl": E.jsonl_bytes(kb),
        "evidence_map.json": E.json_bytes(emap),
        "eval_fixtures/claim_gates.json": E.json_bytes(gates),
    }
    year_info = year_check(doc10, inputs.rel(inputs.doc10), papers, records)
    manifest = make_manifest(inputs, freeze_date, cutoff, papers, records, public, version, files, cross,
                             dup_register, dup_enriched, emap, used_envs, year_info, profile=profile, reviews=reviews)
    if missing:
        manifest["cache_missing"] = [list(m) for m in missing]
    manifest_bytes = E.json_bytes(manifest)
    out = Path(out_dir)
    snapshot = {"manifest.json": manifest_bytes, "records.jsonl": files["records.jsonl"]}
    snap_dir = out / "snapshots" / profile.snapshot_name(freeze_date)
    E.check_snapshot(snap_dir, snapshot)  # fail before writing anything
    for name, data in files.items():
        E.atomic_write(out / name, data)
    E.atomic_write(out / "manifest.json", manifest_bytes)
    E.write_snapshot(snap_dir, snapshot)
    return manifest


# --- reproduce a frozen snapshot -------------------------------------------------

class NotReproducible(RuntimeError):
    pass


resolve_frozen_inputs = SN.resolve_frozen_inputs


def run_reproduce(root, out_dir, cache_dir, snapshot_name, keep=None, log=print):
    """Rebuild ``snapshots/<snapshot_name>`` offline and compare byte for byte.

    Returns a report dict; raises NotReproducible when inputs are unavailable.
    """
    out_dir = Path(out_dir)
    snap = out_dir / "snapshots" / snapshot_name
    manifest = json.loads((snap / "manifest.json").read_text(encoding="utf-8"))
    profile = PF.for_version(manifest["pipeline_version"])
    if profile.snapshot_name(manifest["freeze_date"]) != snapshot_name:
        raise NotReproducible(f"snapshot directory {snapshot_name} does not match its manifest "
                              f"(pipeline {profile.pipeline_version}, freeze {manifest['freeze_date']})")
    found, problems = resolve_frozen_inputs(root, snapshot_name, manifest["inputs"])
    if problems:
        raise NotReproducible("; ".join(problems))
    work = Path(keep) if keep else Path(tempfile.mkdtemp(prefix="sela-reproduce-"))
    try:
        stage = work / "root"
        for rel, src in found.items():
            dst = stage / rel
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(src, dst)
        rebuilt = work / "out"
        run_build(Inputs(stage), cache_dir, rebuilt, manifest["freeze_date"], profile=profile)
        compared = {}
        for name in sorted(p.name for p in snap.iterdir() if p.is_file()):
            a, b = (snap / name).read_bytes(), (rebuilt / "snapshots" / snapshot_name / name).read_bytes()
            compared[f"snapshots/{snapshot_name}/{name}"] = {"identical": a == b, "sha256": E.sha256(a),
                                                             "rebuilt_sha256": E.sha256(b)}
        for rel, digest in sorted(manifest.get("outputs", {}).items()):
            got = E.sha256((rebuilt / rel).read_bytes())
            compared[f"outputs/{rel} (sha256 recorded in the snapshot manifest)"] = {
                "identical": got == digest, "sha256": digest, "rebuilt_sha256": got}
        report = {"snapshot": snapshot_name, "pipeline_version": profile.pipeline_version,
                  "freeze_date": manifest["freeze_date"],
                  "inputs": {rel: ("live" if Path(src).resolve() == (Path(root) / rel).resolve() else "frozen copy")
                             for rel, src in found.items()},
                  "compared": compared, "identical": all(v["identical"] for v in compared.values())}
        for key, v in compared.items():
            log(f"{'IDENTICAL' if v['identical'] else 'DIFFERENT'}  {key}  {v['rebuilt_sha256']}")
        return report
    finally:
        if not keep:
            shutil.rmtree(work, ignore_errors=True)


# --- v3 research outputs (gap search, screening, evidence map, claim check) ------

def verify_v3_outputs(root, out):
    """Run the v3 checks too when ``out`` is the repository's corpus directory and the files exist."""
    root = Path(root)
    if Path(out).resolve() != (root / DEFAULT_OUT).resolve():
        return []
    import v3_evidence as VE  # noqa: E402  (sibling scripts; imported lazily)
    import v3_gap_search as G  # noqa: E402
    problems = []
    for freeze_dir in sorted((root / G.OUT_BASE).glob("*/manifest.json")):
        problems += [f"v3 gap search {freeze_dir.parent.name}: {p}"
                     for p in G.run_verify(root, freeze_dir.parent.name)]
    if (root / VE.CLAIM_OUT).exists() or (root / VE.EMAP_OUT).exists():
        problems += [f"v3 evidence: {p}" for p in VE.verify(root)]
    import leak_check as LC  # noqa: E402  (CI half of the leak check: committed fingerprints, no cache needed)
    if (root / LC.FINGERPRINTS).exists():
        problems += [f"leak check: {p}" for p in LC.check_fingerprints(root)]
    return problems


# --- CLI ----------------------------------------------------------------------

def _freeze(value):
    try:
        return dt.date.fromisoformat(value).isoformat()
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"invalid --freeze-date {value!r}: {exc}") from exc


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--root", type=Path, default=ROOT, help="repository root (inputs are resolved here)")
    p.add_argument("--out", type=Path, default=None, help="output directory (default data/research/corpus)")
    p.add_argument("--cache", type=Path, default=None, help="cache directory (default <out>/.cache)")
    sub = p.add_subparsers(dest="command", required=True)
    pf = sub.add_parser("fetch", help="fill the API cache (network unless --offline)")
    pf.add_argument("--freeze-date", type=_freeze, default=DEFAULT_FREEZE_DATE)
    pf.add_argument("--offline", action="store_true", help="no network: only report missing cache entries")
    pf.add_argument("--refresh", action="store_true", help="re-fetch entries that are already cached")
    pf.add_argument("--only", default="", help="comma-separated R-ids (default: all)")
    pb = sub.add_parser("build", help="offline build from cache -> outputs")
    pb.add_argument("--freeze-date", type=_freeze, default=DEFAULT_FREEZE_DATE)
    pb.add_argument("--allow-missing", action="store_true", help="build even if cache entries are missing")
    pb.add_argument("--pipeline-version", default=PIPELINE_VERSION, choices=sorted(PF.PROFILES),
                    help="build profile (1.1.0 = legacy schema of the frozen 2026-10-26 snapshot)")
    pr = sub.add_parser("reproduce", help="rebuild a frozen snapshot in a scratch dir and compare byte for byte")
    pr.add_argument("--snapshot", default=PF.LEGACY.snapshot_name(DEFAULT_FREEZE_DATE),
                    help="snapshot directory name under <out>/snapshots/ (default: the frozen 1.1.0 snapshot)")
    pr.add_argument("--keep", type=Path, default=None, help="keep the rebuilt tree in this directory")
    pr.add_argument("--report", type=Path, default=None, help="also write the comparison report (JSON) here")
    sub.add_parser("verify", help="re-hash outputs and check invariants (exit 1 on violation)")
    ps = sub.add_parser("scan-claims", help="scan product copy files for forbidden claims")
    ps.add_argument("files", nargs="+", type=Path)
    ps.add_argument("--show-negated", action="store_true",
                    help="also list claim wordings suppressed by the negation window (informational)")
    args = p.parse_args(argv)
    out = args.out or (args.root / DEFAULT_OUT)
    cache = args.cache or (out / ".cache")
    inputs = Inputs(args.root)
    if args.command == "fetch":
        only = {x.strip() for x in args.only.split(",") if x.strip()}
        return run_fetch(inputs, cache, args.freeze_date, offline=args.offline, refresh=args.refresh, only=only)
    if args.command == "reproduce":
        try:
            report = run_reproduce(args.root, out, cache, args.snapshot, keep=args.keep)
        except (MissingCache, NotReproducible, E.SnapshotConflict, OSError, ValueError) as exc:
            print(f"reproduce failed: {exc}", file=sys.stderr)
            return 1
        if args.report:
            args.report.write_text(json.dumps(report, indent=1, sort_keys=True) + "\n", encoding="utf-8")
        print(f"reproduce {args.snapshot} (pipeline {report['pipeline_version']}): "
              f"{'BYTE-IDENTICAL' if report['identical'] else 'DIFFERENT'}; inputs: "
              + ", ".join(f"{k}={v}" for k, v in sorted(report["inputs"].items())))
        return 0 if report["identical"] else 1
    if args.command == "build":
        try:
            m = run_build(inputs, cache, out, args.freeze_date, allow_missing=args.allow_missing,
                          profile=PF.for_version(args.pipeline_version))
        except (MissingCache, E.SnapshotConflict, RV.ReviewError) as exc:
            print(f"build failed: {exc}", file=sys.stderr)
            return 1
        c = m["counts"]
        print(json.dumps({
            "freeze_date": m["freeze_date"], "cutoff_date": m["cutoff_date"], "index_version": m["index_version"],
            "records": c["records"], "screening_status": c["screening_status"],
            "date_discrepancies_total": c["date_discrepancies_total"],
            "records_with_primary_date_discrepancy": c["records_with_primary_date_discrepancy"],
            "retraction_status": c["retraction_status"], "license_family": c["license_family"],
            "access_status": c["access_status"], "fulltexts_processed": c["fulltexts_processed"],
            "sample_sizes_extracted": c["sample_sizes_extracted"],
            "year_check": m["year_distribution_check"].get("conclusion"),
            "pipeline_version": m["pipeline_version"], "snapshot": m.get("snapshot",
                                                                         f"snapshots/{m['freeze_date']}"),
            "v3": c.get("v3"),
        }, indent=2, ensure_ascii=False))
        return 0
    if args.command == "verify":
        violations, warnings = V.verify_outputs(out, root=args.root)
        violations += verify_v3_outputs(args.root, out)
        for w in warnings:
            print("WARNING:", w)
        for v in violations:
            print("VIOLATION:", v)
        print(f"verify: {len(violations)} violation(s), {len(warnings)} warning(s)")
        return 1 if violations else 0
    if args.command == "scan-claims":
        total = negated = 0
        for path in args.files:
            for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
                for hit in C.scan_text(line, include_negated=args.show_negated):
                    if hit.get("negated"):
                        negated += 1
                        print(f"{path}:{lineno}: negated, not counted: {hit['gate']}: {hit['match']!r}")
                        continue
                    total += 1
                    print(f"{path}:{lineno}: {hit['gate']}: {hit['match']!r}")
        extra = f"; {negated} negated mention(s) not counted" if args.show_negated else ""
        print(f"scan-claims: {total} hit(s){extra}")
        return 1 if total else 0
    return 2


if __name__ == "__main__":
    sys.exit(main())
