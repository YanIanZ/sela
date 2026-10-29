#!/usr/bin/env python3
"""Sela v3 evidence outputs (research audit R-8, R-9). Machine-made, unreviewed.

  fetch-abstracts  network: public abstracts of every source cited by docs 07/10 and of
                   every other register record (Europe PMC core by DOI; arXiv API for
                   R099; Crossref for R100) into the gitignored cache
                   data/research/corpus/.cache/v3_abstracts/ (the claim check reads only
                   the doc 07/10 sources; the rest serve row drafting and leak_check.py)
  claim-check      offline: every doc 07/10 statement, decomposed into explicit
                   checkable assertions (data/research/corpus/claim_check_v3_spec.json),
                   is matched against the cached abstracts ->
                   data/research/corpus/claim_check_v3.json (machine_checked_unreviewed,
                   with mismatches; no abstract text is written)
  evidence-map     offline: one row per v3 feature ->
                   data/research/corpus/evidence_map_v3.json and docs/research/EVIDENCE_MAP_V3.md
  verify           offline: hashes, staleness and labelling invariants of both files

A term match is not verification that a source supports a claim; it tells a
reviewer where wording and abstract diverge. Nothing here is appraisal, and
adult evidence is never mapped to ages 15-17.
"""
from __future__ import annotations

import argparse
import collections
import csv
import hashlib
import json
import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path
from urllib.parse import urlencode

sys.path.insert(0, str(Path(__file__).resolve().parent))

from corpus import fetch as F  # noqa: E402
from corpus import gzstore as S  # noqa: E402
from corpus import ids  # noqa: E402
from corpus import v3fields as V3  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
CORPUS = Path("data/research/corpus")
CACHE = CORPUS / ".cache" / "v3_abstracts"
CLAIM_SPEC = CORPUS / "claim_check_v3_spec.json"
CLAIM_OUT = CORPUS / "claim_check_v3.json"
EMAP_SPEC = CORPUS / "evidence_map_v3_spec.json"
EMAP_OUT = CORPUS / "evidence_map_v3.json"
EMAP_DOC = Path("docs/research/EVIDENCE_MAP_V3.md")
# The latest frozen gap-search run (v1.2, screening v1.2). Runs v1.0 and v1.1 stay frozen in their directories.
GAP_RUN = Path("data/research/v3_gap_search/2026-10-26_search-1.2")
GAP_QUEUE = GAP_RUN / "screening" / "queue.jsonl"
# Recorded (inputs) under its logical name with the hash of the uncompressed content; committed as .gz.
GAP_CANDIDATES = GAP_RUN / "candidates.jsonl"
CHECK_VERSION = "sela-claim-check-v3.1"  # v3.1: terms are matched in the title and the abstract
# v3.1: age bands from accepted ranges only; minors roles split. v3.2: a help-seeking row; brief topics from
# per-record tags; per-row age groups, theme coverage, inference labels; candidate language with its basis
EMAP_VERSION = "sela-evidence-map-v3.2"
ARXIV = {"R099": "2504.18412"}
OUT_OF_REGISTER = {  # cited in doc 10, not in the 100-record register
    "npj_dm_scoping_review": "10.1038/s41746-025-01611-4",
    "nature_medicine_2026_audit": "10.1038/s41591-026-04577-2",
}
# Cited in doc 10 by DOI only (no R-id in the text) although they are register records.
REGISTER_CITED_BY_DOI = {"R070": "10.1002/wps.21352"}
NOT_PEER_REVIEWED = {
    "stanford_news_2025": "https://news.stanford.edu/stories/2025/06/ai-mental-health-care-tools-dangers-risks",
}
_TAG = re.compile(r"<[^>]+>")


def clean(text):
    import html as _h
    return " ".join(_h.unescape(_TAG.sub(" ", str(text or ""))).split())


def _sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def json_bytes(obj):
    return (json.dumps(obj, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode("utf-8")


def read_jsonl(path):
    """Rows of a JSONL file; a ``.gz`` path is decompressed (corpus.gzstore)."""
    return [json.loads(x) for x in S.read_text(path).splitlines() if x.strip()]


def _input_sha(path):
    """SHA-256 recorded for an input: of the file, or of the uncompressed content of its ``.gz``."""
    return S.content_sha256(path)


def records(root):
    return {r["id"]: r for r in read_jsonl(Path(root) / CORPUS / "records.jsonl")}


# --- abstracts (local cache only) -------------------------------------------------

def epmc_doi_url(doi):
    return F.EPMC_SEARCH + "?" + urlencode({"query": f'DOI:"{doi}"', "format": "json", "resultType": "core",
                                            "pageSize": 5})


def arxiv_url(arxiv_id):
    return "https://export.arxiv.org/api/query?" + urlencode({"id_list": arxiv_id})


def _register_source(rid, rec):
    if rid in ARXIV:
        return (rid, "arxiv", "arxiv:" + ARXIV[rid], arxiv_url(ARXIV[rid]))
    if rec.get("europepmc_id"):
        return (rid, "europepmc", "doi:" + rec["doi"], epmc_doi_url(rec["doi"]))
    return (rid, "crossref", "crossref:" + rec["doi"], F.CROSSREF_WORK.format(doi=rec["doi"]))


def source_list(root):
    """[(source_id, kind, key, url)] for every doc 07/10 source (the claim check's sources)."""
    emap = json.loads((Path(root) / CORPUS / "evidence_map.json").read_text(encoding="utf-8"))
    rids = sorted({r for e in emap["entries"] for r in e["r_ids"]} | set(REGISTER_CITED_BY_DOI))
    recs = records(root)
    out = [_register_source(rid, recs[rid]) for rid in rids]
    for sid, doi in sorted(OUT_OF_REGISTER.items()):
        out.append((sid, "europepmc", "doi:" + doi, epmc_doi_url(doi)))
    return out


def register_source_list(root):
    """Every other register record: its public abstract is cached too, so that the evidence-map rows can be
    drafted against it and the leak check (leak_check.py) can test every register abstract. The claim check
    still reads only source_list(), so its output does not depend on this list."""
    have = {s[0] for s in source_list(root)}
    return [_register_source(rid, rec) for rid, rec in sorted(records(root).items()) if rid not in have]


def parse_arxiv(raw):
    root = ET.fromstring(raw)
    ns = {"a": "http://www.w3.org/2005/Atom"}
    e = root.find("a:entry", ns)
    if e is None:
        return None
    return {"id": e.findtext("a:id", default="", namespaces=ns), "title": clean(e.findtext("a:title", "", ns)),
            "abstract": clean(e.findtext("a:summary", "", ns))}


def run_fetch(root, log=print):
    cache = F.Cache(Path(root) / CACHE)
    client = F.Client()
    errors = []
    for sid, kind, key, url in source_list(root) + register_source_list(root):
        if cache.get(kind if kind != "crossref" else "crossref", key) is not None:
            continue
        service = {"europepmc": "europepmc", "arxiv": "arxiv", "crossref": "crossref"}[kind]
        status, raw, error = client.get(service, url, accept="application/xml" if kind == "arxiv" else
                                        "application/json")
        if status != 200:
            errors.append((sid, error or f"HTTP {status}"))
            continue
        if kind == "europepmc":
            body = json.loads(raw.decode("utf-8"))
            res = [r for r in (body.get("resultList") or {}).get("result") or []
                   if ids.normalize_doi(r.get("doi")) == key[4:]]
            res.sort(key=lambda r: ({"MED": 0, "PMC": 1}.get(r.get("source"), 9), str(r.get("id"))))
            r = res[0] if res else {}
            body = {"id": f"{r.get('source')}:{r.get('id')}" if r else None, "title": clean(r.get("title")),
                    "abstract": clean(r.get("abstractText")) or None}
        elif kind == "arxiv":
            body = parse_arxiv(raw) or {}
        else:
            m = json.loads(raw.decode("utf-8")).get("message") or {}
            body = {"id": "crossref:" + key.split(":", 1)[1], "title": clean((m.get("title") or [""])[0]),
                    "abstract": clean(m.get("abstract")) or None}
        cache.put(service, key, F.envelope(service, key, url, F.utc_now(), 200, "ok", body=body))
        log(f"{sid}: abstract {'cached' if body.get('abstract') else 'not available'}")
    for e in errors:
        log(f"ERROR {e[0]}: {e[1]}")
    return 1 if errors else 0


def load_abstracts(root, every_register_record=False):
    """{source_id: abstract info} from the local cache: the doc 07/10 sources (claim check), or with
    ``every_register_record`` also every other register record (leak check, row drafting)."""
    cache = F.Cache(Path(root) / CACHE)
    out = {}
    sources = source_list(root) + (register_source_list(root) if every_register_record else [])
    for sid, kind, key, url in sources:
        env = cache.get(kind, key)
        body = (env or {}).get("body") or {}
        out[sid] = {"kind": kind, "url": url, "retrieved_at": (env or {}).get("fetched_at"),
                    "source_record": body.get("id"), "title": body.get("title") or None,
                    "abstract": body.get("abstract"), "cached": env is not None}
    return out


# --- claim check -------------------------------------------------------------------------

def _rx(pattern):
    return re.compile(pattern, re.IGNORECASE)


STATUS_RANK = {"terms_found": 0, "absent_as_expected": 0, "partially_found": 1, "not_found": 2,
               "unexpectedly_present": 2, "contradiction_cue_found": 3, "abstract_unavailable": 4}
CONSISTENT = ("terms_found", "absent_as_expected")


def check_assertion(assertion, abstracts):
    """An assertion lists ``terms`` (all must match: each is a regex alternative set)
    and optional ``contradiction`` patterns; it is checked against the public title and
    abstract of each cited source (a title such as '... Termination Report' carries facts
    the abstract may not repeat). Without an abstract the source stays abstract_unavailable."""
    per_source = {}
    for sid in assertion["sources"]:
        a = abstracts.get(sid) or {}
        abstract = a.get("abstract")
        if not abstract:
            per_source[sid] = {"status": "abstract_unavailable", "matched_terms": [], "missing_terms": [],
                               "contradiction_cues": []}
            continue
        title = a.get("title") or ""
        text = title + "\n" + abstract
        matched, missing, in_title = [], [], []
        for t in assertion["terms"]:
            rx = _rx(t["pattern"])
            (matched if rx.search(text) else missing).append(t["label"])
            if title and rx.search(title):
                in_title.append(t["label"])
        contra = [c["label"] for c in assertion.get("contradiction", []) if _rx(c["pattern"]).search(text)]
        if assertion.get("expect") == "absent":
            status = "unexpectedly_present" if matched else "absent_as_expected"
        elif contra:
            status = "contradiction_cue_found"
        elif not missing:
            status = "terms_found"
        elif matched:
            status = "partially_found"
        else:
            status = "not_found"
        per_source[sid] = {"status": status, "matched_terms": matched, "missing_terms": missing,
                           "matched_in_title": in_title, "contradiction_cues": contra,
                           "searched": ["title", "abstract"] if title else ["abstract"],
                           "abstract_sha256": hashlib.sha256(abstract.encode("utf-8")).hexdigest()}
    rank = STATUS_RANK
    best = min((v["status"] for v in per_source.values()), key=lambda s: rank[s]) if per_source else "no_source"
    mode = assertion.get("support_mode", "any")
    if mode == "all":
        statuses = [v["status"] for v in per_source.values()]
        best = max(statuses, key=lambda s: rank[s]) if statuses else "no_source"
    return best, per_source


def run_claim_check(root):
    spec = json.loads((Path(root) / CLAIM_SPEC).read_text(encoding="utf-8"))
    abstracts = load_abstracts(root)
    statements, mismatches = [], []
    for st in spec["statements"]:
        results = []
        for a in st["assertions"]:
            status, per = check_assertion(a, abstracts)
            results.append({"id": a["id"], "claim_part": a["claim_part"], "sources": a["sources"],
                            "support_mode": a.get("support_mode", "any"), "expect": a.get("expect", "present"),
                            "status": status, "per_source": per, "note": a.get("note")})
            if status not in CONSISTENT:
                mismatches.append({"statement": st["id"], "assertion": a["id"], "status": status,
                                   "claim_part": a["claim_part"], "note": a.get("note")})
        worst = max((r["status"] for r in results), key=lambda s: STATUS_RANK[s])
        statements.append({"id": st["id"], "doc": st["doc"], "location": st["location"], "statement": st["statement"],
                           "cited": st["cited"], "assertions": results, "overall": worst,
                           "agent_reading": st.get("agent_reading"), "wording_issue": st.get("wording_issue")})
    srcs = {sid: {"kind": a["kind"], "url": a["url"], "retrieved_at": a["retrieved_at"],
                  "source_record": a["source_record"], "abstract_available": bool(a["abstract"]),
                  "abstract_sha256": hashlib.sha256(a["abstract"].encode("utf-8")).hexdigest() if a["abstract"] else None}
            for sid, a in sorted(abstracts.items())}
    for sid, url in NOT_PEER_REVIEWED.items():
        srcs[sid] = {"kind": "news", "url": url, "retrieved_at": None, "source_record": None,
                     "abstract_available": False, "abstract_sha256": None,
                     "note": "news item, not peer reviewed; not checked; doc 10 should cite R099 instead"}
    out = {
        "check_version": CHECK_VERSION,
        "status": "machine_checked_unreviewed",
        "is_appraisal": False,
        "method": ("Each doc 07/10 statement was decomposed (by the AI research agent, unreviewed) into explicit "
                   "assertions; each assertion lists regular-expression term sets that must all appear in the public "
                   "title or abstract of a cited source, plus contradiction cues. 'terms_found' means the words are present, "
                   "not that the abstract supports the claim; anything else is listed under mismatches for a person "
                   "to read. Abstracts were read from the local cache only and are not committed."),
        "spec_sha256": _sha(Path(root) / CLAIM_SPEC),
        "inputs": {p: _sha(Path(root) / p) for p in ("docs/research/sela-relay/07_EVIDENCE_TO_FEATURE.md",
                                                      "docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md")},
        "sources": srcs,
        "statements": statements,
        "counts": {"statements": len(statements),
                   "assertions": sum(len(s["assertions"]) for s in statements),
                   "overall": dict(sorted(collections.Counter(s["overall"] for s in statements).items())),
                   "assertion_status": dict(sorted(collections.Counter(
                       r["status"] for s in statements for r in s["assertions"]).items())),
                   "mismatches": len(mismatches),
                   "wording_issues": sorted(s["id"] for s in statements if s.get("wording_issue"))},
        "mismatches": mismatches,
        "wording_issues": [{"statement": s["id"], "statement_text": s["statement"], "issue": s["wording_issue"]}
                           for s in statements if s.get("wording_issue")],
    }
    (Path(root) / CLAIM_OUT).write_bytes(json_bytes(out))
    return out


# --- v3 evidence map ------------------------------------------------------------------------

def gap_queue(root):
    p = Path(root) / GAP_QUEUE
    return read_jsonl(p) if p.exists() else []


def gap_candidates(root):
    """{candidate_id: candidate} of the gap-search run the map reads (for the language detail)."""
    p = S.gz_path(Path(root) / GAP_CANDIDATES)
    return {r["candidate_id"]: r for r in read_jsonl(p)} if p.exists() else {}


def gap_screening_version(root):
    p = Path(root) / GAP_RUN / "screening" / "manifest.json"
    return json.loads(p.read_text(encoding="utf-8")).get("screening_version") if p.exists() else None


# Only an accepted explicit age range of a primary study that overlaps 15-17 can speak about
# ages 15-17 (role supports_for_minors). A minors SIGNAL (unverified youth words, a review whose
# eligibility includes minors, or conflicting candidates of which one overlaps 15-17) allows only
# the role context_for_minors. Adult evidence is never mapped to minors.
EXPLICIT_MINOR_BANDS = {"includes_15_17_explicit"}
MINOR_SIGNAL_BANDS = {"includes_15_17_explicit", "minors_possible_unverified", "review_eligibility_includes_minors"}
ADULT_BANDS = {"adults_only_explicit", "adults_indexed_or_named_only", "review_eligibility_adults_only"}
MINOR_ROLES = {"supports_for_minors", "context_for_minors"}
# Coarse age groups shown for every row, so that adult and adolescent evidence never sit in one list.
AGE_GROUPS = ("includes_15_17_explicit", "minors_signal_ages_unverified", "adults_only", "ages_not_stated")
# How far a cited record can be read, in the agent's words (unreviewed). A machine design label is a separate
# column; these labels say what kind of inference the record allows for this row.
INFERENCE = {
    "perceptions_qualitative": "reported perceptions (qualitative); no effect measured",
    "association_cross_sectional": "cross-sectional associations; not causal",
    "mediation_on_cross_sectional_data": "mediation model on cross-sectional data: associations only, not causal "
                                         "pathways",
    "secondary_analysis_association": "secondary analysis: associations only",
    "pilot_trial_feasibility": "pilot trial: feasibility; effect estimates preliminary",
    "review_of_reported_factors": "review of reported factors; included studies not appraised here",
    "uncontrolled_evaluation": "uncontrolled evaluation: no comparison group",
}


def has_minor_signal(rec):
    band = rec["age_band"]["summary"]
    if band == "conflicting_explicit_ranges":
        return bool(rec["age_band"].get("candidates_overlapping_15_17"))
    return band in MINOR_SIGNAL_BANDS


def is_adult_only(rec):
    """Adult MeSH/title words only, an accepted adult range, an adult-only review, or conflicting
    candidates that all start at 18 or above."""
    band = rec["age_band"]["summary"]
    if band == "conflicting_explicit_ranges":
        vals = rec["age_range_detail"].get("candidate_values") or []
        return bool(vals) and all(V3._parse_age_value(v)[1] >= 18 for v in vals)
    return band in ADULT_BANDS


def age_group(view):
    if view["age_band"] in EXPLICIT_MINOR_BANDS:
        return "includes_15_17_explicit"
    if view["minor_signal"]:
        return "minors_signal_ages_unverified"
    if view["adult_only"]:
        return "adults_only"
    return "ages_not_stated"


def record_view(rec):
    caution = rec.get("study_type_caution")
    return {
        "id": rec["id"], "title": rec["title"], "doi": rec["doi"],
        "design_machine_label": rec["study_type"],
        "design_caution": caution["caution"] if caution else None,
        "population_terms": rec["population"], "country_or_language": rec["country_or_language"],
        "age_band": rec["age_band"]["summary"], "age_range": rec["age_range"],
        "age_candidates_overlapping_15_17": rec["age_band"].get("candidates_overlapping_15_17") or [],
        "minor_signal": has_minor_signal(rec), "adult_only": is_adult_only(rec),
        "age_groups_indexed": rec["age_groups_indexed"],
        "screening_status": rec["screening_status"], "reviewer_status": rec["reviewer_status"],
        "publication_language": rec["publication_language"],
    }


def population_split(views):
    by = collections.defaultdict(list)
    for v in views:
        by[v["age_band"]].append(v["id"])
    return {k: sorted(v) for k, v in sorted(by.items())}


def age_groups(views):
    by = {g: [] for g in AGE_GROUPS}
    for v in views:
        by[age_group(v)].append(v["id"])
    return {g: sorted(ids) for g, ids in by.items()}


def entry_topics(row_spec, entry):
    """Brief topics a cited record is evidence for: its own tags, or the row's only topic."""
    if "topics" in entry:
        return list(entry["topics"])
    return list(row_spec["brief_topics"]) if len(row_spec["brief_topics"]) == 1 else []


def language_display(cand):
    """Language cell for a candidate: the metadata value with its basis and the Indonesian title cue."""
    d = cand.get("publication_language_detail") or {}
    lang = cand.get("publication_language")
    basis = {"machine_inferred_only": "inferred", "publisher_deposit": "deposited", "curated": "indexed"}.get(
        d.get("basis"))
    cue = d.get("title_suggests_indonesian")
    if lang is None:
        return "not stated (title suggests id)" if cue else "not stated"
    if d.get("title_cue_disagrees"):
        return f"{lang} ({basis or 'source unknown'}; title suggests id)"
    if basis == "inferred":
        return f"{lang} (inferred)"
    return lang


def theme_coverage(f, recs, cited):
    tc = f.get("theme_coverage")
    if not tc:
        return None
    theme_ids = sorted(r for r, rec in recs.items() if rec.get("theme") == tc["theme"])
    excluded = {e["id"]: e["reason"] for e in tc.get("deliberately_excluded", [])}
    return {"theme": tc["theme"], "records_in_theme": theme_ids,
            "cited": sorted(set(theme_ids) & cited), "deliberately_excluded": tc.get("deliberately_excluded", []),
            "uncovered": sorted(set(theme_ids) - cited - set(excluded))}


def run_evidence_map(root):
    spec = json.loads((Path(root) / EMAP_SPEC).read_text(encoding="utf-8"))
    recs = records(root)
    queue = gap_queue(root)
    gcands = gap_candidates(root)
    screen_version = gap_screening_version(root)
    claim = json.loads((Path(root) / CLAIM_OUT).read_text(encoding="utf-8")) if (Path(root) / CLAIM_OUT).exists() else None
    rows = []
    shown = []
    for f in spec["features"]:
        sup = []
        for r in f["register_support"]:
            v = dict(record_view(recs[r["id"]]), role=r["role"], relation=r["relation"], topics=entry_topics(f, r))
            if r.get("note"):
                v["note"] = r["note"]
            if r.get("inference"):
                v["inference"] = r["inference"]
                v["inference_label"] = INFERENCE.get(r["inference"])
            sup.append(v)
        counter = [dict(record_view(recs[r["id"]]), reason=r["reason"]) for r in f.get("counter_evidence", [])]
        fams = set(f["v3_families"])
        cands = [q for q in queue if fams & set(q["screen"]["feature_families"])]
        title_hit = lambda q: any(q["screen"]["feature_families"].get(fm, {}).get("title") for fm in fams)
        cands.sort(key=lambda q: (0 if title_hit(q) else 1, 0 if q["screen"]["priority"] == "high" else 1,
                                  -q["screen"]["score"], q["candidate_id"]))
        top = []
        for q in cands[:f.get("show_candidates", 8)]:
            c = gcands.get(q["candidate_id"], {})
            d = c.get("publication_language_detail") or {}
            top.append({"candidate_id": q["candidate_id"], "title": q["title"], "doi": q["doi"],
                        "first_publication_date": q["first_publication_date"],
                        "publication_language": q["publication_language"],
                        "publication_language_basis": d.get("basis"),
                        "title_suggests_indonesian": bool(d.get("title_suggests_indonesian")),
                        "title_cue_disagrees": bool(d.get("title_cue_disagrees")),
                        "language_display": language_display(c) if c else (q["publication_language"] or "not stated"),
                        "machine_priority": q["screen"]["priority"], "population_band": q["screen"]["population_band"],
                        "design_cues": q["screen"]["design_cues"],
                        "possible_null_or_negative_finding": bool(q["screen"]["possible_null_or_negative_finding"]),
                        "in_register": q["in_register"]})
        shown += [q for q in top[:5] if q["candidate_id"] not in {x["candidate_id"] for x in shown}]
        bands = population_split(sup)
        adult_only = all(s["adult_only"] or s["age_band"] == "not_stated" for s in sup) if sup else None
        direct = [s for s in sup if s["relation"] == "direct"]
        cited = {s["id"] for s in sup} | {s["id"] for s in counter}
        row = {
            "feature": f["feature"], "feature_id": f["id"], "brief_topics": f["brief_topics"],
            "families": f["v3_families"],
            "question": f["question"],
            "register_support": sup,
            "support_population_split": bands,
            "support_age_groups": age_groups(sup),
            "support_is_adult_only_or_unstated": adult_only,
            "direct_register_support": len(direct),
            "counter_evidence": counter,
            "v3_candidates_for_human_screening": {
                "count_in_queue": len(cands), "top": top,
                "note": f"machine title/abstract screening only ({screen_version}); not evidence until a "
                        "person screens and appraises them",
            },
            "claim_boundary": f["claim_boundary"],
            "declared_gaps": f["declared_gaps"],
            "required_before_stronger_claims": f["required_before_stronger_claims"],
            "minors_rule": ("Adult evidence is not evidence for ages 15-17. Only records with age band "
                            "includes_15_17_explicit (an accepted explicit age range of a primary study) can be read "
                            "for minors (role supports_for_minors), and only within the limits shown; records with a "
                            "minors signal but no such range are context only (role context_for_minors)."),
            "status": "machine_assembled_agent_drafted_unreviewed",
        }
        if f.get("brief_topics_note"):
            row["brief_topics_note"] = f["brief_topics_note"]
        tc = theme_coverage(f, recs, cited)
        if tc:
            row["theme_coverage"] = tc
        rows.append(row)
    # brief topics come from per-record tags (a record counts for a topic only when it is tagged for it, or when
    # its row has that topic alone), never from the union of every record of every row that lists the topic
    topics = {t: {"features": [], "register_ids": set()} for t in spec["brief_section_9_topics"]}
    for r in rows:
        for t in r["brief_topics"]:
            topics.setdefault(t, {"features": [], "register_ids": set()})["features"].append(r["feature_id"])
        for s in r["register_support"]:
            for t in s["topics"]:
                topics[t]["register_ids"].add(s["id"])
    # 'adults (computed from age bands)' is not a feature: it is every cited record whose machine age band is adult-only
    computed = "adults (computed from age bands)"
    topics[computed] = {"features": [], "register_ids": set()}
    for r in rows:
        adult = [s["id"] for s in r["register_support"] if s["adult_only"]]
        if adult:
            topics[computed]["features"].append(r["feature_id"])
            topics[computed]["register_ids"].update(adult)
    out = {
        "map_version": EMAP_VERSION,
        "status": "machine_assembled_agent_drafted_unreviewed",
        "is_appraisal": False,
        "how_to_read": spec["how_to_read"],
        "freeze_date": "2026-10-26", "cutoff_date": "2021-10-26",
        "gap_search_run": GAP_RUN.as_posix(),
        "inputs": {p: _input_sha(Path(root) / p) for p in (EMAP_SPEC.as_posix(), (CORPUS / "records.jsonl").as_posix(),
                                                           GAP_QUEUE.as_posix(), GAP_CANDIDATES.as_posix(),
                                                           CLAIM_OUT.as_posix())
                   if S.exists(Path(root) / p)},
        "features": rows,
        "brief_section_9_topics": spec["brief_section_9_topics"],
        "brief_topics": {t: {"features": sorted(set(v["features"])), "register_ids": sorted(v["register_ids"]),
                             "rule": ("computed: every cited record whose machine age band is adult-only"
                                      if t == computed else "per-record topic tags")}
                         for t, v in topics.items()},
        "claim_check_summary": claim["counts"] if claim else None,
        "counts": {"features": len(rows),
                   "features_without_direct_register_support": sorted(r["feature_id"] for r in rows
                                                                      if r["direct_register_support"] == 0),
                   "features_supported_by_adult_or_unstated_ages_only": sorted(
                       r["feature_id"] for r in rows if r["support_is_adult_only_or_unstated"]),
                   "candidates_shown": len(shown),  # distinct candidates in the rendered tables (top 5 per row)
                   "candidates_shown_language_disagrees_with_indonesian_title": sorted(
                       q["candidate_id"] for q in shown if q["title_cue_disagrees"]),
                   "candidates_shown_language_inferred_only": sorted(
                       q["candidate_id"] for q in shown if q["publication_language_basis"] == "machine_inferred_only"),
                   "candidates_shown_language_not_stated": sorted(
                       q["candidate_id"] for q in shown if q["publication_language"] is None)},
    }
    (Path(root) / EMAP_OUT).write_bytes(json_bytes(out))
    return out


def verify_spec(spec, recs):
    """Spec-level rules: topic tags, inference labels and theme coverage."""
    problems = []
    allowed_topics = set(spec.get("brief_section_9_topics") or [])
    if not allowed_topics:
        return ["evidence_map_v3_spec: brief_section_9_topics missing"]
    used = set()
    for f in spec["features"]:
        bt = f["brief_topics"]
        used.update(bt)
        for t in bt:
            if t not in allowed_topics:
                problems.append(f"evidence_map_v3_spec {f['id']}: brief topic {t!r} is not a brief section 9 topic")
        if not bt and not f.get("brief_topics_note"):
            problems.append(f"evidence_map_v3_spec {f['id']}: a row without brief topics needs brief_topics_note")
        for e in f["register_support"]:
            if len(bt) > 1 and "topics" not in e:
                problems.append(f"evidence_map_v3_spec {f['id']}: {e['id']} needs topics (the row has {len(bt)} "
                                "brief topics; a record is evidence only for the topics it is tagged with)")
            if not set(e.get("topics", [])) <= set(bt):
                problems.append(f"evidence_map_v3_spec {f['id']}: {e['id']} is tagged with a topic its row lacks")
            if e.get("inference") and e["inference"] not in INFERENCE:
                problems.append(f"evidence_map_v3_spec {f['id']}: {e['id']} has unknown inference {e['inference']!r}")
        tc = theme_coverage(f, recs, {e["id"] for e in f["register_support"]} |
                            {e["id"] for e in f.get("counter_evidence", [])})
        if tc and tc["uncovered"]:
            problems.append(f"evidence_map_v3_spec {f['id']}: {tc['theme']} records neither mapped nor listed as "
                            f"deliberately excluded: {tc['uncovered']}")
        if tc:
            for e in f["theme_coverage"].get("deliberately_excluded", []):
                if not str(e.get("reason") or "").strip():
                    problems.append(f"evidence_map_v3_spec {f['id']}: exclusion of {e.get('id')} needs a reason")
    for t in sorted(allowed_topics - used):
        problems.append(f"evidence_map_v3_spec: brief section 9 topic {t!r} has no feature row")
    return problems


def verify(root):
    problems = []
    root = Path(root)
    for out, spec_key in ((CLAIM_OUT, "spec_sha256"), (EMAP_OUT, None)):
        p = root / out
        if not p.exists():
            problems.append(f"{out} missing")
            continue
        doc = json.loads(p.read_text(encoding="utf-8"))
        if doc.get("is_appraisal") is not False or not str(doc.get("status", "")).startswith("machine_"):
            problems.append(f"{out}: must stay labelled machine_* and is_appraisal false")
        for rel, digest in (doc.get("inputs") or {}).items():  # the candidates input is the content of its .gz
            if not S.exists(root / rel) or _input_sha(root / rel) != digest:
                problems.append(f"{out}: input {rel} changed; rebuild ({'claim-check' if out == CLAIM_OUT else 'evidence-map'})")
        if spec_key and doc.get(spec_key) != _sha(root / CLAIM_SPEC):
            problems.append(f"{out}: spec changed; rerun claim-check")
    recs = records(root)
    if (root / EMAP_SPEC).exists():
        problems += verify_spec(json.loads((root / EMAP_SPEC).read_text(encoding="utf-8")), recs)
    if (root / EMAP_OUT).exists():
        emap = json.loads((root / EMAP_OUT).read_text(encoding="utf-8"))
        for row in emap["features"]:
            for s in row["register_support"] + row["counter_evidence"]:
                if s["id"] not in recs:
                    problems.append(f"evidence_map_v3 {row['feature_id']}: unknown R-id {s['id']}")
                elif s["age_band"] != recs[s["id"]]["age_band"]["summary"]:
                    problems.append(f"evidence_map_v3 {row['feature_id']}: stale age band for {s['id']}")
                elif (s.get("minor_signal"), s.get("adult_only")) != (has_minor_signal(recs[s["id"]]),
                                                                       is_adult_only(recs[s["id"]])):
                    problems.append(f"evidence_map_v3 {row['feature_id']}: stale age reading for {s['id']}")
            if row["direct_register_support"] == 0 and not row["declared_gaps"]:
                problems.append(f"evidence_map_v3 {row['feature_id']}: no direct support and no declared gap")
            if row.get("support_age_groups") != age_groups(row["register_support"]):
                problems.append(f"evidence_map_v3 {row['feature_id']}: support_age_groups do not match the records")
            if (row.get("theme_coverage") or {}).get("uncovered"):
                problems.append(f"evidence_map_v3 {row['feature_id']}: theme records not mapped: "
                                f"{row['theme_coverage']['uncovered']}")
            for s in row["register_support"]:
                if s["id"] not in recs:
                    continue
                rec = recs[s["id"]]
                if s["role"] == "supports_for_minors" and rec["age_band"]["summary"] not in EXPLICIT_MINOR_BANDS:
                    problems.append(f"evidence_map_v3 {row['feature_id']}: {s['id']} supports minors without an "
                                    f"accepted explicit 15-17 age range (band {rec['age_band']['summary']})")
                if s["role"] == "context_for_minors" and not has_minor_signal(rec):
                    problems.append(f"evidence_map_v3 {row['feature_id']}: adult evidence mapped to minors {s['id']}")
                if not set(s.get("topics", [])) <= set(row["brief_topics"]):
                    problems.append(f"evidence_map_v3 {row['feature_id']}: {s['id']} tagged with a topic its row lacks")
            if "minors" in row["feature_id"] or "adolescents" in row["brief_topics"]:
                bad = [s["id"] for s in row["register_support"] if s["role"] not in MINOR_ROLES]
                if bad:
                    problems.append(f"evidence_map_v3 {row['feature_id']}: minors row cites {bad} without a minors role")
        # brief topics must equal what the per-record tags give (never the union of whole rows)
        tagged = collections.defaultdict(set)
        for row in emap["features"]:
            for s in row["register_support"]:
                for t in s.get("topics", []):
                    tagged[t].add(s["id"])
        for t, v in (emap.get("brief_topics") or {}).items():
            if v.get("rule") == "per-record topic tags" and set(v["register_ids"]) != tagged.get(t, set()):
                problems.append(f"evidence_map_v3 brief topic {t!r}: register_ids differ from the per-record tags")
    doc = root / EMAP_DOC
    if (root / EMAP_OUT).exists() and doc.exists():
        if f"evidence_map_v3.json sha256 `{_sha(root / EMAP_OUT)}`" not in doc.read_text(encoding="utf-8"):
            problems.append(f"{EMAP_DOC} was not regenerated from the current evidence_map_v3.json")
    return problems


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--root", type=Path, default=ROOT)
    sub = p.add_subparsers(dest="command", required=True)
    for name in ("fetch-abstracts", "claim-check", "evidence-map", "verify"):
        sub.add_parser(name)
    a = p.parse_args(argv)
    if a.command == "fetch-abstracts":
        return run_fetch(a.root)
    if a.command == "claim-check":
        out = run_claim_check(a.root)
        print(json.dumps(out["counts"], indent=1))
        return 0
    if a.command == "evidence-map":
        out = run_evidence_map(a.root)
        import v3_evidence_doc as DOC  # renders docs/research/EVIDENCE_MAP_V3.md from the JSON
        DOC.render(a.root)
        print(json.dumps(out["counts"], indent=1))
        return 0
    if a.command == "verify":
        problems = verify(a.root)
        for x in problems:
            print("VIOLATION:", x)
        print(f"v3 evidence verify: {len(problems)} violation(s)")
        return 1 if problems else 0
    return 2


if __name__ == "__main__":
    sys.exit(main())
