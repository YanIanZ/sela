#!/usr/bin/env python3
"""Auditable TITLE-ONLY relevance triage for Sela's Europe PMC candidate metadata.

Not a systematic review, abstract screening, evidence grade, or clinical validation.
Uses only public bibliographic fields; does not retrieve copyrighted works or call an LLM.

Rulesets (``--ruleset``; every change is a new version, criterion 6 of doc 14):

- ``sela-title-screen-v1.0`` (2026-10-26, superseded): kept only so the
  original outputs can be reproduced byte for byte. Its CSV files were written
  with the ``csv`` module's default CRLF line ending and its manifest hashed
  those CRLF bytes, while git (``* text=auto``) stores the files with LF. The
  committed files therefore never matched their manifest hashes (audit P1).
- ``sela-title-screen-v1.1`` (default): identical title triage, but (a) every
  output is written with LF line endings and hashed over exactly the bytes on
  disk, which are the bytes git stores; (b) protocols, corrections/errata,
  editorials and retraction notices are excluded from the review queue by
  publication type (title cue as a fallback), with the basis recorded for
  every excluded row (audit P5). Priority labels are unchanged, so acceptance
  criterion 1 (exactly one priority per row) still holds.
"""
from __future__ import annotations

import argparse
import collections
import csv
import hashlib
import json
import math
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from corpus import eligibility as ELIG  # noqa: E402

DEFAULT_INPUT = Path("data/research/sela_research_candidates_7580.jsonl")
DEFAULT_OUTPUT = Path("data/research")
RULESET_V10 = "sela-title-screen-v1.0"
RULESET_V11 = "sela-title-screen-v1.1"
RULESETS = (RULESET_V10, RULESET_V11)
RULESET_VERSION = RULESET_V11
V10_MANIFEST_COPY = "data/research/screening_history/sela_screening_manifest_v1.0.json"

# Exact/phrase-oriented cues; topic labels from the original broad search are
# intentionally NOT used to increase the score. They can be extremely noisy.
SIGNAL_REGEX = {
    "mental_health": (
        r"\bmental health\b", r"\bmental illness\b", r"\bmental disorders?\b",
        r"\bpsychiatr(?:y|ic|ists?)\b", r"\bpsycholog(?:y|ical|ist|ists)\b",
        r"\bpsychotherap(?:y|ies|ist|ists|eutic)\b", r"\bdepressi(?:on|ve)\b",
        r"\banxiety\b", r"\bwellbeing\b", r"\bwell-being\b",
        r"\bpsychosocial\b", r"\bsuicid(?:e|al|ality)\b", r"\bself-harm\b",
        r"\bstress(?:ors?|ed)?\b", r"\btrauma\b", r"\bbehavioral health\b",
    ),
    "care_pathway": (
        r"\bhelp[- ]seeking\b", r"\bcare navigation\b", r"\bpatient navigation\b",
        r"\bcare coordinat(?:ion|ed|or)\b", r"\bcontinuity of care\b",
        r"\bwarm hand[- ]off\b", r"\bhandoffs?\b", r"\breferrals?\b",
        r"\bshared decision[- ]making\b", r"\bcare pathways?\b",
        r"\bservice access\b", r"\baccess to (?:mental health|care|services|therapy)\b",
        r"\bfirst consultation\b", r"\bpatient journey\b", r"\bcare seeking\b",
        r"\btreatment seeking\b", r"\bservice utiliz(?:ation|ation)\b",
        r"\bbarriers? to (?:care|services|treatment|seeking)\b",
        r"\bfirst[- ]contact\b",
    ),
    "ai_models": (
        r"\blarge language models?\b", r"\bllms?\b", r"\bgenerative ai\b",
        r"\bartificial intelligence\b", r"\bmachine learning\b",
        r"\bchatbots?\b", r"\bconversational agents?\b", r"\bai[- ](?:powered|based|driven|assisted|enabled)\b",
        r"\bautomated (?:clinical|medical) (?:notes?|summari[sz]ation)\b",
        r"\bnatural language processing\b",
    ),
    "source_fidelity": (
        r"\bsummari[sz]ation\b", r"\bsummari[sz]ing\b", r"\bclinical notes?\b",
        r"\bclinical documentation\b", r"\bhallucinati(?:on|ons)\b",
        r"\bfactual(?:ity|ness)\b", r"\bsource grounding\b",
        r"\binformation extraction\b", r"\bsemantic fidelity\b",
        r"\bnegation\b", r"\bfaithful(?:ness)?\b",
    ),
    "privacy_consent": (
        r"\bdata privacy\b", r"\bprivacy\b", r"\bconsent\b",
        r"\bpatient autonomy\b", r"\bconfidential(?:ity)?\b",
        r"\binformed decision\b", r"\bdata shar(?:e|ing)\b",
        r"\binformation shar(?:e|ing)\b", r"\bdata protection\b",
    ),
    "ai_safety_bias": (
        r"\bsafety\b", r"\bbias\b", r"\bfairness\b", r"\btrustworth(?:y|iness)\b",
        r"\bmodel evaluation\b", r"\badverse (?:event|outcome|effect)s?\b",
        r"\breliab(?:ility|le)\b", r"\bguardrails?\b", r"\bmisinformation\b",
    ),
    "language_equity": (
        r"\bmultilingual\b", r"\bbilingual\b", r"\btranslation\b",
        r"\bcross[- ]lingual\b", r"\blower[- ]resource languages?\b",
        r"\blanguage barriers?\b", r"\bhealth literacy\b",
        r"\bcultural(?:ly)? (?:adaptation|adapted|competence|sensitivity)\b",
    ),
    "indonesia_asean": (
        r"\bindonesia(?:n)?\b", r"\basean\b", r"\bsoutheast asia(?:n)?\b",
        r"\bbahasa\b", r"\bjakarta\b", r"\bsurabaya\b",
    ),
    "digital_health": (
        r"\bdigital (?:mental )?health\b", r"\btelemedicine\b",
        r"\btelehealth\b", r"\be[- ]mental health\b",
        r"\bmobile health\b", r"\bmhealth\b", r"\behealth\b",
    ),
    "engagement": (
        r"\bengagement\b", r"\badherence\b", r"\bretention\b",
        r"\bdropout\b", r"\bacceptability\b", r"\busability\b",
        r"\bimplementation\b", r"\buser experience\b",
    ),
}
PATTERNS = {
    name: tuple(re.compile(r, re.IGNORECASE) for r in terms)
    for name, terms in SIGNAL_REGEX.items()
}
DOMAIN_PENALTY = tuple(re.compile(r, re.IGNORECASE) for r in (
    r"\bendourolog(?:y|ical)\b", r"\burolog(?:y|ical)\b", r"\borthop[ae]dic\b",
    r"\bveterinar(?:y|ian)\b", r"\bnephrolog(?:y|ical)\b",
    r"\bdental\b", r"\bendodontic\b", r"\bdermatolog(?:y|ical)\b",
    r"\boncolog(?:y|ical)\b", r"\bcardiolog(?:y|ical)\b",
))
STUDY_CLUES = {
    "protocol": (r"\bstudy protocol\b", r"\btrial protocol\b", r"\bprotocol for\b"),
    "systematic_review": (r"\bsystematic review\b",),
    "meta_analysis": (r"\bmeta[- ]analysis\b",),
    "randomized_trial": (r"\brandomi[sz]ed\b", r"\brandomised\b", r"\brct\b"),
    "scoping_review": (r"\bscoping review\b",),
    "qualitative": (r"\bqualitative\b", r"\binterviews?\b"),
}
STUDY_PATTERNS = {
    name: tuple(re.compile(r, re.IGNORECASE) for r in terms)
    for name, terms in STUDY_CLUES.items()
}

WEIGHTS = {
    "mental_health": 5, "care_pathway": 5, "ai_models": 3,
    "source_fidelity": 3, "privacy_consent": 3, "ai_safety_bias": 2,
    "language_equity": 2, "indonesia_asean": 2,
    "digital_health": 2, "engagement": 1,
}

def signal_matches(title: str, pattern_sets: dict) -> list[str]:
    return sorted(name for name, patterns in pattern_sets.items()
                  if any(p.search(title) for p in patterns))

def triage(title: str) -> dict:
    """Deterministic discovery priority, never paper-quality/clinical efficacy rating."""
    signals = signal_matches(title, PATTERNS)
    found = set(signals)
    mh = "mental_health" in found
    pathway = "care_pathway" in found
    ai = "ai_models" in found
    methodological = bool(found & {"source_fidelity", "ai_safety_bias", "privacy_consent"})
    if mh and pathway:
        family = "mental_health_care_pathway"
    elif mh and ai:
        family = "ai_for_mental_health"
    elif mh and "privacy_consent" in found:
        family = "privacy_in_mental_health"
    elif mh and "indonesia_asean" in found:
        family = "indonesia_mental_health"
    elif ai and methodological:
        family = "cross_domain_ai_methods"
    elif mh:
        family = "other_mental_health_context"
    elif found & {"care_pathway", "privacy_consent", "source_fidelity", "language_equity"}:
        family = "adjacent_methods_or_access"
    else:
        family = "broad_search_not_prioritized"

    score = sum(WEIGHTS[s] for s in signals)
    if mh and pathway:
        score += 6
    if mh and ai:
        score += 5
    if mh and "privacy_consent" in found:
        score += 3
    if mh and "indonesia_asean" in found:
        score += 2
    if ai and "source_fidelity" in found:
        score += 3
    if ai and "ai_safety_bias" in found:
        score += 2
    out_of_domain = (not mh and not pathway and
                     any(p.search(title) for p in DOMAIN_PENALTY))
    if out_of_domain:
        score -= 8
    study_clues = signal_matches(title, STUDY_PATTERNS)
    if "protocol" in study_clues:
        score -= 2  # Protocols are not outcomes; still discoverable.

    # A broad title-only search is no basis for asserting exclusion/eligibility.
    if ((mh and (pathway or ai or "privacy_consent" in found)) or
            (mh and "indonesia_asean" in found and "digital_health" in found)) and score >= 10:
        priority = "high"
    elif ((mh and score >= 7) or
          (ai and methodological and score >= 8) or
          (pathway and "privacy_consent" in found) or
          ("indonesia_asean" in found and mh)):
        priority = "medium"
    else:
        priority = "deferred"
    if out_of_domain and not mh:
        priority = "deferred"
    return {
        "priority": priority, "score": score, "family": family,
        "signals": signals, "study_design_title_clues": study_clues,
        "out_of_domain_flag": out_of_domain,
        "screening_status": "automated_title_only_unreviewed",
    }

# --- publication-type eligibility (v1.1) --------------------------------------
# One shared, versioned rule (scripts/research/corpus/eligibility.py) so the
# title screener, the v3 gap search and the corpus pipeline never drift apart.
ELIGIBILITY_RULE = ELIG.RULE


def publication_type_eligibility(publication_type: str, title: str) -> dict:
    """Deterministic eligibility by publication type (title cue as fallback)."""
    return ELIG.eligibility(str(publication_type or "").split(";"), title)


def select_review_queue(rows: list[dict], maximum: int) -> list[dict]:
    """Stratify year to reduce dominance by 2026 without manufacturing evidence."""
    good = [r for r in rows if r["screening"]["priority"] in {"high", "medium"}
            and r["screening"].get("eligibility", {}).get("status", "eligible") == "eligible"]
    sort_key = lambda r: (-r["screening"]["score"],
                          0 if r["screening"]["priority"] == "high" else 1,
                          r["title"].casefold(), r["candidate_id"])
    by_year: dict[str, list[dict]] = {}
    for row in good:
        by_year.setdefault(row["first_publication_date"][:4], []).append(row)
    for group in by_year.values():
        group.sort(key=sort_key)
    # Older years are sparse. Reserve slots where relevant matches actually exist.
    chosen = []
    used = set()
    cap_2026 = math.ceil(maximum * 0.45)
    cap_2025 = math.ceil(maximum * 0.45)
    for year in sorted(by_year):
        max_for_year = cap_2026 if year == "2026" else cap_2025 if year == "2025" else maximum
        for row in by_year[year][:max_for_year]:
            if len(chosen) >= maximum:
                break
            chosen.append(row); used.add(row["candidate_id"])
    if len(chosen) < maximum:
        rest = sorted((r for r in good if r["candidate_id"] not in used), key=sort_key)
        chosen.extend(rest[:maximum-len(chosen)])
    return sorted(chosen, key=sort_key)

def _csv_writer(f, fieldnames, ruleset):
    # v1.0 kept the csv module default (CRLF); v1.1 writes LF so the hashed
    # bytes are the bytes git stores (audit P1).
    if ruleset == RULESET_V10:
        return csv.DictWriter(f, fieldnames=fieldnames)
    return csv.DictWriter(f, fieldnames=fieldnames, lineterminator="\n")


INDEX_FIELDS = ["candidate_id", "doi", "pmid", "first_publication_date", "title", "priority",
                "score", "family", "signals", "study_design_title_clues", "out_of_domain_flag", "review_status"]
INDEX_FIELDS_V11 = INDEX_FIELDS + ["eligibility", "eligibility_basis", "ruleset"]
WORKSHEET_FIELDS = ["candidate_id", "doi", "first_publication_date", "title", "automated_priority",
                    "abstract_screening", "screening_reason", "reviewer", "review_date",
                    "full_text_appraised", "study_design_verified", "risk_of_bias",
                    "finding_direction", "indonesia_transferability", "license_reuse_verified",
                    "rag_ready", "notes"]


def _eligibility_text(e):
    return ";".join(f"{b['kind']}:{b['source']}:{b['term']}" for b in e["basis"])


def screen(input_file: Path, output_dir: Path, limit: int = 400, ruleset: str = RULESET_VERSION) -> dict:
    if limit < 1:
        raise ValueError("Review limit must be positive")
    if ruleset not in RULESETS:
        raise ValueError(f"Unknown ruleset {ruleset!r}; choose one of {', '.join(RULESETS)}")
    v11 = ruleset == RULESET_V11
    raw = input_file.read_bytes()
    records = [json.loads(line) for line in raw.splitlines() if line.strip()]
    if not records:
        raise ValueError("Empty dataset")
    seen = set()
    rows = []
    for i, record in enumerate(records, start=1):
        if not record.get("title") or record.get("screening_status") != "candidate_metadata_only_unreviewed":
            raise ValueError(f"Malformed/unreviewed source record at line {i}")
        key = record.get("doi") or record.get("pmid") or record["title"].casefold()
        if key in seen:
            raise ValueError(f"Duplicate source key at line {i}")
        seen.add(key)
        enriched = dict(record)
        enriched["candidate_id"] = f"SL-{i:05d}"
        enriched["screening"] = triage(record["title"])
        if v11:
            enriched["screening"]["eligibility"] = publication_type_eligibility(
                record.get("publication_type"), record["title"])
            enriched["screening"]["ruleset"] = ruleset
        rows.append(enriched)

    output_dir.mkdir(parents=True, exist_ok=True)
    index_path = output_dir / "sela_screening_index.csv"
    with index_path.open("w", encoding="utf-8", newline="") as f:
        writer = _csv_writer(f, INDEX_FIELDS_V11 if v11 else INDEX_FIELDS, ruleset)
        writer.writeheader()
        for r in rows:
            s = r["screening"]
            row = {
                "candidate_id":r["candidate_id"],"doi":r.get("doi",""),
                "pmid":r.get("pmid",""),"first_publication_date":r["first_publication_date"],
                "title":r["title"],"priority":s["priority"],"score":s["score"],
                "family":s["family"],"signals":";".join(s["signals"]),
                "study_design_title_clues":";".join(s["study_design_title_clues"]),
                "out_of_domain_flag":s["out_of_domain_flag"],
                "review_status":s["screening_status"],
            }
            if v11:
                row.update({"eligibility": s["eligibility"]["status"],
                            "eligibility_basis": _eligibility_text(s["eligibility"]), "ruleset": ruleset})
            writer.writerow(row)

    shortlist = select_review_queue(rows, limit)
    queue_path = output_dir / "sela_priority_review_queue.jsonl"
    with queue_path.open("w", encoding="utf-8", newline="\n") as f:
        for r in shortlist:
            f.write(json.dumps(r, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n")
    # Human-review worksheet: blank verdicts are deliberate and must be completed
    # by a reader; the automatic title screen never grants RAG/training permission.
    review_path = output_dir / "sela_human_review_worksheet.csv"
    with review_path.open("w", encoding="utf-8", newline="") as f:
        writer = _csv_writer(f, WORKSHEET_FIELDS, ruleset)
        writer.writeheader()
        for r in shortlist:
            writer.writerow({
                "candidate_id":r["candidate_id"],"doi":r.get("doi",""),
                "first_publication_date":r["first_publication_date"],"title":r["title"],
                "automated_priority":r["screening"]["priority"],
                "abstract_screening":"NOT_REVIEWED",
                "screening_reason":"","reviewer":"","review_date":"",
                "full_text_appraised":"NO","study_design_verified":"NO",
                "risk_of_bias":"NOT_EVALUATED","finding_direction":"UNKNOWN",
                "indonesia_transferability":"NOT_EVALUATED",
                "license_reuse_verified":"NO","rag_ready":"NO","notes":"",
            })
    manifest = {
        "screening_version": ruleset,
        "source_file": input_file.name,
        "source_sha256": hashlib.sha256(raw).hexdigest(),
        "source_count": len(rows),
        "screening_unit": "publication title only (plus year/identifiers for queue)",
        "abstracts_used": False, "full_text_used": False, "llm_used": False,
        "clinical_evidence_quality_evaluated": False,
        "priority_counts": dict(sorted(collections.Counter(r["screening"]["priority"] for r in rows).items())),
        "family_counts": dict(sorted(collections.Counter(r["screening"]["family"] for r in rows).items())),
        "year_counts": dict(sorted(collections.Counter(r["first_publication_date"][:4] for r in rows).items())),
        "queue_count": len(shortlist),
        "queue_year_counts": dict(sorted(collections.Counter(r["first_publication_date"][:4] for r in shortlist).items())),
        "queue_priority_counts": dict(sorted(collections.Counter(r["screening"]["priority"] for r in shortlist).items())),
        "queue_family_counts": dict(sorted(collections.Counter(r["screening"]["family"] for r in shortlist).items())),
        "review_worksheet_sha256": hashlib.sha256(review_path.read_bytes()).hexdigest(),
        "review_worksheet_count": len(shortlist),
        "index_sha256": hashlib.sha256(index_path.read_bytes()).hexdigest(),
        "queue_sha256": hashlib.sha256(queue_path.read_bytes()).hexdigest(),
        "limitations": [
            "Relevance signals are heuristic lexical clues from title only; false positives and false negatives are expected.",
            "High priority is not evidence quality, human approval, clinical safety or permission for model training.",
            "The underlying 7580 candidates have strong 2025–2026 recency bias.",
            "Only licensed full-text sources may later enter a content-based RAG index, after human appraisal.",
        ],
    }
    if v11:
        elig = collections.Counter(r["screening"]["eligibility"]["status"] for r in rows)
        kinds = collections.Counter(k for r in rows for k in r["screening"]["eligibility"]["excluded_kinds"])
        sources = collections.Counter(
            (k, b["source"]) for r in rows for k in r["screening"]["eligibility"]["excluded_kinds"]
            for b in r["screening"]["eligibility"]["basis"] if b["kind"] == k)
        ineligible_hm = [r for r in rows if r["screening"]["priority"] in {"high", "medium"}
                         and r["screening"]["eligibility"]["status"] != "eligible"]
        manifest.update({
            "line_terminator": "LF",
            "hash_basis": ("sha256 over the exact bytes written (UTF-8, LF line endings); these are the bytes git "
                           "stores, and data/research/.gitattributes keeps them LF on checkout"),
            "eligibility_rule": ELIGIBILITY_RULE,
            "eligibility_rule_version": ELIG.RULE_VERSION,
            "eligibility_counts": dict(sorted(elig.items())),
            "excluded_kind_counts": dict(sorted(kinds.items())),
            "excluded_basis_entry_counts": {f"{k}:{s}": n for (k, s), n in sorted(sources.items())},
            "high_or_medium_excluded_by_publication_type": len(ineligible_hm),
            "supersedes": {
                "screening_version": RULESET_V10,
                "frozen_manifest": V10_MANIFEST_COPY,
                "reason": ("v1.0 hashed CRLF bytes (csv default line ending) while git stores LF, so the committed "
                           "index and worksheet never matched their manifest hashes (audit P1); v1.0 also let 3 "
                           "correction notices and 8 protocols into the 400 queue (audit P5)"),
                "reproduce": "python3 scripts/research/screen_candidates.py --ruleset sela-title-screen-v1.0 --output <scratch>",
            },
            "false_negative_audit": ("criterion 5 of doc 14 is not satisfied by this machine run; a deterministic "
                                     "audit sample with blank human columns is in data/research/v3_gap_search/"),
        })
        manifest["limitations"] = manifest["limitations"] + [
            "Publication-type eligibility relies on Europe PMC pubType tokens and explicit title cues; a protocol or "
            "notice without either cue stays eligible, and an excluded row is never deleted from the index.",
        ]
    (output_dir/"sela_screening_manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True)+"\n", encoding="utf-8")
    return manifest

def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--input",type=Path,default=DEFAULT_INPUT)
    p.add_argument("--output",type=Path,default=DEFAULT_OUTPUT)
    p.add_argument("--priority-limit",type=int,default=400)
    p.add_argument("--ruleset",choices=RULESETS,default=RULESET_VERSION,
                   help="screening ruleset (v1.0 only to reproduce the superseded outputs)")
    args=p.parse_args()
    manifest=screen(args.input,args.output,args.priority_limit,ruleset=args.ruleset)
    print(json.dumps({
        "screening_version":manifest["screening_version"],
        "screened":manifest["source_count"],
        "priorities":manifest["priority_counts"],
        "eligibility":manifest.get("eligibility_counts"),
        "queue_count":manifest["queue_count"],
        "queue_by_year":manifest["queue_year_counts"],
    },indent=2))
if __name__=="__main__":
    main()
