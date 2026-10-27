"""Schema v3 additions to a corpus record (pipeline 1.2.0, rules v3).

Every value here is either copied from a named public metadata field, matched
by an explicit deterministic pattern with a locator, or left ``null``. Nothing
is inferred or paraphrased. Machine values carry ``status`` strings that start
with ``machine_``; the reviewer fields (``reviewer_name``, ``review_date``,
``review_decision``, ``findings`` and the ``*_reviewer_*`` overrides) are set
ONLY from a human entry in ``data/research/register_inputs/reviewer_decisions.json``
and stay ``null`` otherwise. Machine fields are never overwritten by a review;
a review adds a separate, attributed layer.

Additions (audit problems P1-P12 / tasks R-2, R-6):
- ``publication_language`` (+ ``_detail``): Europe PMC ``language`` (MEDLINE
  indexing), else Crossref ``language`` (publisher deposit), else OpenAlex
  ``language`` (machine-inferred, flagged).
- ``age_range`` (+ ``_detail``): explicit age ranges / minimum ages / mean ages
  in the CC full text (abstract and top-level methods), each with a locator in
  ``evidence[]``; a value only when all range candidates agree.
- ``age_groups_indexed``: NLM MeSH age check tags copied from Europe PMC.
- ``age_band``: a derived summary that keeps adult and adolescent evidence apart.
- ``country_evidence_fulltext``: country names found in the full text, as
  candidates a reviewer may promote; never written into ``country_or_language``.
- ``study_type_caution``: a visible flag when the primary design label comes
  only from an index publication type while the title signals another design.
- ``publication_type_eligibility``: protocols, errata, editorials and
  retraction notices become ``excluded_publication_type``.
- ``needs_check_evidence``: public PubMed history and publisher landing-page
  dates for records without a day-precision primary date (documentation only;
  the canonical-date rule and the status do not change).
"""
from __future__ import annotations

import re

from . import eligibility as ELIG
from . import extract as X

MACHINE_EXTRACTED = "machine_extracted_unreviewed"
MACHINE_DERIVED = "machine_derived_unreviewed"
MACHINE_COPIED = "machine_copied_from_index_unreviewed"
MACHINE_DOCUMENTED = "machine_documented_unreviewed"
REVIEWER_STATUSES = ("not_reviewed", "human_reviewed_title_abstract", "human_reviewed_full_text")
REVIEWER_FIELDS = ("reviewer_name", "review_date", "review_decision", "findings", "appraisal_tool",
                   "reviewer_2_name", "reviewer_2_decision", "review_disagreement", "adjudicated_by",
                   "study_type_reviewer_override", "publication_date_reviewer_override",
                   "country_or_language_reviewer_promoted")
SCREENING_STATUSES_V3 = ("included_title_abstract_v0", "excluded_out_of_window", "excluded_retracted",
                         "excluded_publication_type", "needs_check")

# --- publication language -------------------------------------------------------

ISO639_2_TO_1 = {
    "eng": "en", "ind": "id", "may": "ms", "msa": "ms", "spa": "es", "ger": "de", "deu": "de", "fre": "fr",
    "fra": "fr", "chi": "zh", "zho": "zh", "jpn": "ja", "kor": "ko", "por": "pt", "ita": "it", "dut": "nl",
    "nld": "nl", "tha": "th", "vie": "vi", "tur": "tr", "ara": "ar", "rus": "ru", "per": "fa", "fas": "fa",
    "swe": "sv", "nor": "no", "dan": "da", "fin": "fi", "pol": "pl", "heb": "he", "hin": "hi", "tgl": "tl",
    "fil": "tl", "jav": "jv", "sun": "su", "ces": "cs", "cze": "cs", "gre": "el", "ell": "el", "hun": "hu",
}
_ISO1 = re.compile(r"^[a-z]{2}$")


def normalise_language(value):
    """'eng' / 'EN' / 'en-US' -> 'en'; unknown, 'und' or 'mul' -> None (raw kept by the caller)."""
    if not value:
        return None
    v = str(value).strip().lower().replace("_", "-").split("-")[0]
    if v in ("und", "mul", "zxx", ""):
        return None
    if _ISO1.match(v):
        return v
    return ISO639_2_TO_1.get(v)


def publication_language(epmc, crossref, openalex):
    raw = {"europepmc.language": (epmc or {}).get("language"),
           "crossref.language": (crossref or {}).get("language"),
           "openalex.language": (openalex or {}).get("language")}
    norm = {k: normalise_language(v) for k, v in raw.items()}
    curated = [(k, norm[k]) for k in ("europepmc.language", "crossref.language") if norm[k]]
    flags = []
    if curated:
        source, value = curated[0]
        basis = "curated" if source.startswith("europepmc") else "publisher_deposit"
    elif norm["openalex.language"]:
        source, value, basis = "openalex.language", norm["openalex.language"], "machine_inferred_only"
        flags.append("publication_language_inferred_only")
    else:
        source, value, basis = None, None, "not_stated"
    disagree = len({v for _, v in curated}) > 1
    if disagree:
        flags.append("publication_language_sources_disagree")
    notes = []
    if norm["openalex.language"] and value and norm["openalex.language"] != value:
        notes.append(f"OpenAlex infers '{norm['openalex.language']}' from the text; recorded, not used "
                     "(OpenAlex language is machine-detected)")
    return value, {
        "value": value, "source": source, "basis": basis,
        "raw": {k: v for k, v in sorted(raw.items()) if v is not None},
        "curated_sources_disagree": disagree, "notes": notes,
        "rule": ("Europe PMC language (MEDLINE/indexer) > Crossref language (publisher deposit) > OpenAlex "
                 "language (machine-inferred, flagged); ISO 639-2 codes mapped to ISO 639-1"),
        "status": MACHINE_COPIED,
    }, flags


# --- age groups (MeSH) ------------------------------------------------------------

MESH_AGE_GROUPS = {
    "Infant": (0, 1), "Child, Preschool": (2, 5), "Child": (6, 12), "Adolescent": (13, 18),
    "Young Adult": (19, 24), "Adult": (19, 44), "Middle Aged": (45, 64), "Aged": (65, None),
    "Aged, 80 and over": (80, None),
}
MESH_MINOR_GROUPS = {"Infant", "Child, Preschool", "Child", "Adolescent"}


def age_groups_indexed(epmc):
    headings = ((epmc or {}).get("meshHeadingList") or {}).get("meshHeading") or []
    found = sorted({str(h.get("descriptorName")) for h in headings if h.get("descriptorName") in MESH_AGE_GROUPS})
    return found or None


# --- age range from CC full text ---------------------------------------------------

_DASH = r"(?:-|‐|‑|‒|–|—|to|and)"
_N = r"(\d{1,3})"
AGE_PATTERNS = (
    ("aged_range", re.compile(r"\bage[ds]?\s+(?:of\s+|from\s+)?(?:between\s+)?" + _N + r"\s*" + _DASH + r"\s*" + _N
                              + r"(?!\s*(?:%|months?|weeks?|days?|hours?)\b)(?:\s*(?:years?|y))?\b", re.I), "range"),
    ("between_years", re.compile(r"\bbetween\s+(?:the\s+ages\s+of\s+)?" + _N + r"\s+and\s+" + _N
                                 + r"\s+years?(?:\s+(?:of\s+age|old))?\b", re.I), "range"),
    ("range_years_old", re.compile(r"(?<![\d.])" + _N + r"\s*" + _DASH + r"\s*" + _N
                                   + r"(?:\s*|-)years?[- ]olds?\b", re.I), "range"),
    ("range_years_paren", re.compile(r"\(\s*(?:aged?\s+)?" + _N + r"\s*(?:-|–|—|to)\s*" + _N
                                     + r"\s*(?:years?|y)(?:\s+(?:old|of\s+age))?\s*[,;)]", re.I), "range"),
    ("aged_min", re.compile(r"\bage[ds]?\s+(?:≥|>=|of\s+at\s+least\s+|at\s+least\s+)?\s*" + _N
                            + r"\s*(?:years?\s*)?(?:and|or)\s+(?:older|over|above)\b", re.I), "min"),
    ("aged_ge", re.compile(r"\bage[ds]?\s*(?:≥|>=)\s*" + _N + r"\b", re.I), "min"),
    ("ge_years", re.compile(r"(?:≥|>=)\s*" + _N + r"\s*years?(?:\s+(?:of\s+age|old))?\b", re.I), "min"),
    ("mean_age", re.compile(r"\b(?:mean|average)\s+age(?:\s+of(?:\s+the)?(?:\s+(?:participants|sample|"
                            r"respondents|patients))?)?(?:\s+was)?\s*(?:[:=,]\s*|\s)\s*(\d{1,3}(?:\.\d+)?)"
                            r"(?!\s*(?:%|months?|weeks?|days?)\b)", re.I), "mean"),
    ("m_age", re.compile(r"\bM\s?age\s*=\s*(\d{1,3}(?:\.\d+)?)"), "mean"),
)
AGE_RULE = ("explicit age patterns ('aged 12-18 years', 'between 15 and 17 years', '18-25-year-olds', "
            "'aged 65 and older', 'mean age 21.7') in the abstract (except Background/Introduction/Objective "
            "headings) and top-level methods sections of the CC full text; ranges must satisfy 0 <= min < max <= 110 and are not counted when followed by "
            "months/weeks/days; a candidate that describes a non-sample group (an advisory, working, steering or "
            "reference group, named in the same sentence before the age or in the previous sentence that 'This "
            "group' refers back to) is kept but does not vote; age_range is set only when every voting "
            "range/minimum candidate agrees")

# Groups that advise on a study but are not its sample (audit fix: R031's "Lived Experience
# Working Group ... (aged 16-30)" is not the 18-30 sample). Only these explicit group nouns
# count; a plain "panel" is not one, because in Delphi studies the panel IS the sample.
_NON_SAMPLE_GROUP = re.compile(
    r"\b(?:working|advisory|steering|reference)\s+(?:groups?|boards?|committees?|councils?|panels?)\b|"
    r"\blived[- ]experience\s+(?:advisors?|advisers?|consultants?)\b|\byouth\s+advis(?:ors?|ers?)\b", re.I)
# A participant noun before the age means the sentence may describe the sample itself: then the
# candidate keeps its vote (a conflict leaves age_range null, which is the safe direction).
_PARTICIPANT_NOUN = re.compile(
    r"\b(?:participants?|respondents?|interviewees?|informants?|patients?|sample|enrolled|recruited|"
    r"eligible|inclusion)\b", re.I)
_BACK_REFERENCE = re.compile(r"^\s*(?:this|that|the|these|its)\s+(?:group|board|committee|council|panel|members?)\b",
                             re.I)
_SENTENCE_END = re.compile(r"[.!?](?:\s*\[\])?\s+(?=[A-Z(\[\"'])")


def _sentence_start(text, pos):
    start = 0
    for m in _SENTENCE_END.finditer(text, 0, pos):
        start = m.end()
    return start


def non_sample_group_reason(text, start):
    """Reason string when the age at ``start`` describes an advisory/working group, else None."""
    s0 = _sentence_start(text, start)
    before = text[s0:start]
    g = None
    for g in _NON_SAMPLE_GROUP.finditer(before):
        pass
    if g is not None and not _PARTICIPANT_NOUN.search(before):
        return f"describes a non-sample group ('{g.group(0)}' in the same sentence)"
    if s0 > 0 and _BACK_REFERENCE.match(before) and not _PARTICIPANT_NOUN.search(before):
        prev = text[_sentence_start(text, max(s0 - 2, 0)):s0]
        g = _NON_SAMPLE_GROUP.search(prev)
        if g:
            return f"describes a non-sample group ('{g.group(0)}' in the previous sentence)"
    return None


def _num(text):
    return float(text) if "." in text else int(text)


_CONTEXT_SECTION = re.compile(r"^(?:\[[^\]]*\]\s*)?(?:background|introduction|objectives?|aims?|purpose|"
                              r"context|rationale|importance)\b", re.I)


def _age_scope(para):
    """Sample-size scope minus the context headings of structured abstracts
    (they describe a target population, not the people studied)."""
    if not X._in_sample_scope(para):
        return False
    last = para["section"].split(" > ")[-1].strip()
    return not (para["part"] == "abstract" and _CONTEXT_SECTION.match(last))


def extract_age_candidates(paragraphs, source_url):
    out, seen = [], set()
    for para in paragraphs or []:
        if not _age_scope(para):
            continue
        for name, pattern, kind in AGE_PATTERNS:
            for m in pattern.finditer(para["text"]):
                if kind == "range":
                    lo, hi = int(m.group(1)), int(m.group(2))
                    if not (0 <= lo < hi <= 110):
                        continue
                    value = f"range:{lo}-{hi}"
                elif kind == "min":
                    lo = int(m.group(1))
                    if not (0 < lo <= 110):
                        continue
                    value = f"min:{lo}"
                else:
                    mean = _num(m.group(1))
                    if not (0 < mean <= 110):
                        continue
                    value = f"mean:{mean}"
                key = (value, para["part"], para["section"], para["paragraph"], m.start(1))
                if key in seen:
                    continue
                seen.add(key)
                item = X.evidence_item("age_range", value, para, m.start(), m.end(), source_url, name)
                reason = non_sample_group_reason(para["text"], m.start())
                if reason:
                    item["non_voting"] = reason
                out.append(item)
    return out


def _voting(candidates):
    return [c for c in candidates or [] if not c.get("non_voting")]


def _parse_age_value(value):
    kind, _, rest = value.partition(":")
    if kind == "range":
        lo, hi = rest.split("-")
        return kind, int(lo), int(hi)
    if kind == "min":
        return kind, int(rest), None
    return kind, float(rest), None


def age_range_detail(candidates, is_review):
    voting = _voting(candidates)
    bounds = sorted({c["value"] for c in voting if not c["value"].startswith("mean:")})
    means = sorted({c["value"] for c in voting if c["value"].startswith("mean:")})
    non_voting = sorted(({"value": c["value"], "reason": c["non_voting"], "locator": c["locator"]}
                         for c in candidates or [] if c.get("non_voting")),
                        key=lambda n: (n["value"], n["locator"]["part"], n["locator"]["section"],
                                       n["locator"]["paragraph"]))
    value = None
    if is_review:
        note = "review-type record: ages describe included studies, not one sample; value left null"
    elif not bounds:
        note = ("only mean-age candidates found; no explicit range" if means else
                "no explicit age pattern found in abstract or methods sections")
    elif len(bounds) > 1:
        note = "conflicting candidates " + ", ".join(bounds) + "; left null (no guessing)"
    else:
        kind, lo, hi = _parse_age_value(bounds[0])
        value = {"min": lo, "max": hi, "unit": "years", "kind": kind}
        note = "single consistent explicit value"
    return value, {"value": value, "candidate_values": bounds, "mean_candidates": means,
                   "non_voting_candidates": non_voting, "note": note, "rule": AGE_RULE, "status": MACHINE_EXTRACTED}


def _overlaps_15_17(lo, hi):
    return lo <= 17 and (hi if hi is not None else 999) >= 15


AGE_BANDS = ("includes_15_17_explicit", "adults_only_explicit", "conflicting_explicit_ranges",
             "review_eligibility_includes_minors", "review_eligibility_adults_only", "minors_possible_unverified",
             "adults_indexed_or_named_only", "not_stated")
# Bands that come from one accepted, explicit age_range of a primary study (never from a review,
# never from conflicting candidates). Only includes_15_17_explicit can speak about ages 15-17.
EXPLICIT_BANDS = ("includes_15_17_explicit", "adults_only_explicit")
REVIEW_BANDS = ("review_eligibility_includes_minors", "review_eligibility_adults_only")
AGE_BAND_RULE = (
    "Derived only from the age_range value the record accepted. includes_15_17_explicit: the accepted range or "
    "minimum of a primary study overlaps 15-17; adults_only_explicit: the accepted range or minimum starts at 18 or "
    "above. conflicting_explicit_ranges: a primary study with several different voting candidates (age_range "
    "null); never an explicit band. review_eligibility_includes_minors / review_eligibility_adults_only: a "
    "review-type record whose age candidates (eligibility rules or included studies, not one sample) reach below "
    "18 / all start at 18 or above. Otherwise minors_possible_unverified (MeSH Adolescent/Child or a "
    "youth/adolescent/child title term), adults_indexed_or_named_only (adult MeSH groups or adult title terms) or "
    "not_stated. Candidates describing a non-sample group never vote. Adult evidence is never generalised to ages "
    "15-17.")


def age_band(candidates, mesh_groups, population_labels, accepted=None, is_review=False):
    """Derived summary that keeps minors and adults apart; never a verified population.

    ``accepted`` is the ``age_range`` value the record kept (``None`` when there is none or the
    candidates conflict); an explicit band is derived from it alone, never from the raw candidates.
    """
    signals = {"minors": [], "adults": [], "older_adults_50_plus": []}
    voting = _voting(candidates)
    bounds = []
    for c in voting:
        kind, lo, hi = _parse_age_value(c["value"])
        if kind == "mean":
            if lo >= 50:
                signals["older_adults_50_plus"].append(f"fulltext:{c['value']}")
            continue
        bounds.append((c["value"], lo, hi))
        upper = hi if hi is not None else 999
        if lo < 18:
            signals["minors"].append(f"fulltext:{c['value']}")
        if upper >= 18:
            signals["adults"].append(f"fulltext:{c['value']}")
        if upper >= 50 and (hi is not None or lo >= 50):
            signals["older_adults_50_plus"].append(f"fulltext:{c['value']}")
    for g in mesh_groups or []:
        if g in MESH_MINOR_GROUPS:
            signals["minors"].append(f"mesh:{g}")
        else:
            signals["adults"].append(f"mesh:{g}")
        if g in ("Aged", "Aged, 80 and over"):
            signals["older_adults_50_plus"].append(f"mesh:{g}")
    pops = set(population_labels or [])
    for label in sorted(pops & {"adolescents", "children", "young people or youth"}):
        signals["minors"].append(f"title_or_annotation:{label}")
    for label in sorted(pops & {"adults", "young adults", "college or university students", "older adults"}):
        signals["adults"].append(f"title_or_annotation:{label}")
    if "older adults" in pops:
        signals["older_adults_50_plus"].append("title_or_annotation:older adults")
    notes = []
    overlap = sorted({v for v, lo, hi in bounds if _overlaps_15_17(lo, hi)})
    if is_review and bounds:
        summary = ("review_eligibility_includes_minors" if any(lo < 18 for _, lo, _hi in bounds)
                   else "review_eligibility_adults_only")
        notes.append("review-type record: the ages are eligibility rules or included studies' ages, not one sample; "
                     "no included study is shown to report ages 15-17 separately")
    elif accepted is not None:
        lo, hi = accepted["min"], accepted["max"]
        if _overlaps_15_17(lo, hi):
            summary = "includes_15_17_explicit"
        elif lo >= 18:
            summary = "adults_only_explicit"
        else:
            summary = "minors_possible_unverified"  # an accepted range that ends below 15
    elif len({v for v, _lo, _hi in bounds}) > 1:
        summary = "conflicting_explicit_ranges"
        notes.append("several different explicit age candidates; age_range is null, so no explicit band is given; "
                     + (f"candidates overlapping 15-17: {', '.join(overlap)}" if overlap
                        else "every candidate starts at 18 or above"))
    elif signals["minors"]:
        summary = "minors_possible_unverified"
    elif signals["adults"]:
        summary = "adults_indexed_or_named_only"
    else:
        summary = "not_stated"
    if summary == "adults_only_explicit" and any(s == "mesh:Adolescent" for s in signals["minors"]):
        notes.append("MeSH 'Adolescent' covers 13-18 years and is also assigned for 18-year-olds")
    non_voting = sorted({f"fulltext:{c['value']}" for c in candidates or [] if c.get("non_voting")})
    if non_voting:
        notes.append("not counted (non-sample group): " + ", ".join(non_voting))
    return {
        "summary": summary,
        "signals": {k: sorted(set(v)) for k, v in signals.items()},
        "candidates_overlapping_15_17": overlap,
        "notes": notes,
        "rule": AGE_BAND_RULE,
        "status": MACHINE_DERIVED,
    }


# --- country evidence (reviewer-promotable) ---------------------------------------

def country_evidence(evidence, fulltext_processed):
    if not fulltext_processed:
        return None
    items = [e for e in evidence if e.get("field") == "country"]
    cands = sorted(({"label": e["value"], "occurrences": e.get("occurrences", 1), "first_locator": e["locator"],
                     "snippet_sha256": e["snippet_sha256"]} for e in items),
                   key=lambda c: (-c["occurrences"], c["label"]))
    return {
        "candidates": cands,
        "promotable_to": "country_or_language",
        "promoted_labels": None,
        "note": ("country names found in the CC full text (abstract and methods sections); a mention is not the "
                 "study population. Only a human reviewer may promote a candidate (reviewer_decisions.json -> "
                 "country_or_language_reviewer_promoted); country_or_language itself is never changed."),
        "status": MACHINE_EXTRACTED,
    }


# --- study-type caution (audit P4) ---------------------------------------------------

TRIAL_LABELS = {"randomized_controlled_trial", "randomized_trial", "clinical_trial"}
NON_TRIAL_TITLE_LABELS = {"qualitative", "cross_sectional", "cohort", "mixed_methods", "case_series",
                          "longitudinal", "systematic_review", "scoping_review", "meta_analysis", "protocol"}
ASSOCIATION_CUE = re.compile(
    r"\b(?:associated\s+with|associations?\s+(?:between|of|with)|predictors?\s+of|correlates?\s+of|"
    r"secondary\s+analys[ie]s|post[- ]hoc|ancillary\s+(?:study|analysis)|features\s+associated)\b", re.I)


def study_type_caution(stype, title):
    primary = stype.get("primary")
    if primary not in TRIAL_LABELS:
        return None
    basis = stype.get("basis") or []
    trial_sources = sorted({b["source"] for b in basis if b["label"] in TRIAL_LABELS})
    if "title" in trial_sources:
        return None  # the title itself names the trial design
    other = sorted({b["label"] for b in basis if b["source"] == "title" and b["label"] in NON_TRIAL_TITLE_LABELS})
    assoc = ASSOCIATION_CUE.search(title or "")
    if not other and not assoc:
        return None
    signals = [f"title design cue: {label}" for label in other]
    if assoc:
        signals.append(f"title association cue: '{assoc.group(0)}'")
    return {
        "flag": "study_type_label_conflict",
        "machine_primary": primary,
        "primary_label_sources": trial_sources,
        "title_signals": signals,
        "caution": ("Shown as a trial only because an index publication type (often the parent trial) says so; "
                    "the title signals a different design or an association analysis. Do not cite as trial "
                    "evidence until a reviewer sets study_type_reviewer_override."),
        "status": "machine_flag_unreviewed",
    }


# --- publication-type eligibility ----------------------------------------------------

def publication_type_eligibility(epmc, paper, title):
    types = list(((epmc or {}).get("pubTypeList") or {}).get("pubType") or [])
    types += list(paper.get("publication_types_from_index") or [])
    e = ELIG.eligibility(types, title)
    e["rule_version"] = ELIG.RULE_VERSION
    return e


# --- needs_check documentation (R-2) -------------------------------------------------

_DATE_ANY = re.compile(r"^(\d{4})(?:[-/](\d{1,2})(?:[-/](\d{1,2}))?)?")
PUBLICATION_KINDS = {"publisher_landing_page_online", "pubmed_electronic_article_date", "pubmed_epublish"}


def norm_date(text):
    m = _DATE_ANY.match(str(text or "").strip())
    if not m:
        return None, None
    y, mo, d = m.group(1), m.group(2), m.group(3)
    if d:
        return f"{y}-{int(mo):02d}-{int(d):02d}", "day"
    if mo:
        return f"{y}-{int(mo):02d}", "month"
    return y, "year"


def _bounds(date):
    if len(date) == 10:
        return date, date
    if len(date) == 7:
        return date + "-01", date + "-31"
    return date + "-01-01", date + "-12-31"


def _inside(date, cutoff, freeze):
    """True when every day the (possibly partial) date can denote lies inside cutoff..freeze."""
    lo, hi = _bounds(date)
    return cutoff <= lo and hi <= freeze


LANDING_META_KINDS = {
    "citation_online_date": "publisher_landing_page_online",
    "citation_publication_date": "publisher_landing_page_publication",
    "citation_date": "publisher_landing_page_publication",
    "dc.date": "publisher_landing_page_publication",
    "prism.publicationdate": "publisher_landing_page_publication",
    "article:published_time": "publisher_landing_page_publication",
}
PUBMED_HISTORY_KINDS = {
    "aheadofprint": "pubmed_epublish", "epublish": "pubmed_epublish", "ppublish": "pubmed_print",
    "entrez": "pubmed_indexing", "pubmed": "pubmed_indexing", "medline": "pubmed_indexing",
    "received": "pubmed_manuscript_history", "revised": "pubmed_manuscript_history",
    "accepted": "pubmed_manuscript_history", "pmc-release": "pmc_release",
}


def needs_check_evidence(rec, envs, cutoff, freeze):
    if rec["screening_status"] != "needs_check":
        return None
    cands = []

    def add(source, raw, kind, env=None, url=None):
        date, prec = norm_date(raw)
        if date:
            cands.append({"source": source, "date": date, "precision": prec, "kind": kind,
                          "url": url or (env or {}).get("url"), "retrieved_at": (env or {}).get("fetched_at")})

    ph = envs.get("pubmed_history")
    if ph and ph.get("status") == "ok":
        body = ph.get("body") or {}
        for a in body.get("article_dates") or []:
            add(f"pubmed.ArticleDate.{a.get('type')}", a.get("date"), "pubmed_electronic_article_date"
                if str(a.get("type")).lower() == "electronic" else "pubmed_article_date", ph)
        if body.get("journal_issue_date"):
            add("pubmed.JournalIssue.PubDate", body["journal_issue_date"], "pubmed_issue", ph)
        for h in body.get("history") or []:
            status = str(h.get("status"))
            add(f"pubmed.History.{status}", h.get("date"), PUBMED_HISTORY_KINDS.get(status, "pubmed_other"), ph)
    lm = envs.get("landing_meta")
    landing = None
    if lm:
        body = lm.get("body") or {}
        landing = {"status": lm.get("status"), "http_status": lm.get("http_status"),
                   "url": body.get("final_url") or lm.get("url"),
                   "tdm_reservation": body.get("tdm_reservation"), "note": body.get("note")}
        for name, value in sorted((body.get("meta") or {}).items()):
            add(f"landing_page.{name}", value, LANDING_META_KINDS.get(name, "publisher_landing_page_other"),
                lm, url=body.get("final_url"))
    for key, kind in (("crossref.created", "crossref_deposit_timestamp_not_a_publication_date"),
                      ("crossref.published_print", "crossref_print"), ("crossref.issued", "crossref_issued"),
                      ("europepmc.first_publication_date", "europepmc_first_publication"),
                      ("openalex.publication_date", "openalex_publication_date_may_be_padded")):
        if rec["dates"].get(key):
            cands.append({"source": key, "date": rec["dates"][key], "precision": rec["date_precision"].get(key)
                          or norm_date(rec["dates"][key])[1], "kind": kind, "url": None, "retrieved_at": None})
    cands.sort(key=lambda c: (c["date"], c["source"]))
    pub_days = [c for c in cands if c["kind"] in PUBLICATION_KINDS and c["precision"] == "day"]
    dated = [c for c in cands if c["kind"] not in ("pubmed_manuscript_history", "pmc_release")]
    return {
        "screening_reasons": list(rec["screening_reasons"]),
        "candidate_dates": cands,
        "earliest_day_precision_publication_candidate": pub_days[0]["date"] if pub_days else None,
        "all_publication_and_index_dates_inside_window": all(_inside(c["date"], cutoff, freeze) for c in dated)
        if dated else None,
        "landing_page": landing,
        "resolution": "unresolved_pending_reviewer",
        "rule": ("public PubMed history (E-utilities efetch) and the publisher landing page reached via doi.org "
                 "(citation_* / dc / prism meta tags only; no challenge pages bypassed, no TDM-reserved page "
                 "followed) are recorded as evidence. The canonical-date rule is unchanged, so screening_status "
                 "stays needs_check until a reviewer sets publication_date_reviewer_override."),
        "status": MACHINE_DOCUMENTED,
    }


# --- reviewer overlay ----------------------------------------------------------------

def apply_review(rec, review, reviewer_file):
    for field in REVIEWER_FIELDS:
        rec[field] = None
    if not review:
        return rec
    rec["reviewer_name"] = review["reviewer_name"]
    rec["review_date"] = review["review_date"]
    # the final decision: the single or agreed decision, or the adjudicator's after a disagreement
    rec["review_decision"] = review.get("final_decision", review["decision"])
    rec["reviewer_status"] = "human_reviewed_" + review["review_level"]
    rec["findings"] = review.get("findings")
    rec["appraisal_tool"] = review.get("appraisal_tool")
    r2, adj = review.get("reviewer_2"), review.get("adjudication")
    rec["reviewer_2_name"] = r2["name"] if r2 else None
    rec["reviewer_2_decision"] = r2["decision"] if r2 else None
    rec["review_disagreement"] = review.get("review_disagreement")
    rec["adjudicated_by"] = adj["name"] if adj else None
    rec["study_type_reviewer_override"] = review.get("study_type_override")
    rec["publication_date_reviewer_override"] = review.get("publication_date_override")
    rec["country_or_language_reviewer_promoted"] = review.get("country_promoted")
    if rec.get("country_evidence_fulltext") and review.get("country_promoted"):
        rec["country_evidence_fulltext"]["promoted_labels"] = list(review["country_promoted"])
    prov = {"source": "human_reviewer", "reviewer_file": reviewer_file,
            "rule": "copied verbatim from a validated human reviewer entry"}
    for field in REVIEWER_FIELDS:
        if rec[field] is not None:
            rec["provenance"][field] = dict(prov)
    rec["provenance"]["reviewer_status"] = dict(prov)
    return rec


# --- flags -----------------------------------------------------------------------------

def _flag(fid, rule):
    return {"id": fid, "assigned_by": "machine", "rule": rule, "status": "machine_assigned_unreviewed"}


V3_FLAG_RULES = {
    "population_includes_minors_or_youth": ("explicit youth/adolescent/child term; ages are not verified and adult "
                                            "evidence is never generalised to ages 15-17 (or the reverse)"),
}


def v3_flags(rec, extra_ids):
    flags = {f["id"]: dict(f) for f in rec["flags"]}
    for fid, rule in V3_FLAG_RULES.items():
        if fid in flags:
            flags[fid]["rule"] = rule
    rules = {
        "publication_language_inferred_only": "publication language known only from OpenAlex machine inference",
        "publication_language_sources_disagree": "Europe PMC and Crossref state different publication languages",
        "age_range_conflicting_candidates": "several different explicit age ranges; age_range left null",
        "age_band_includes_15_17": ("the accepted explicit full-text age range of a primary study overlaps 15-17; "
                                    "findings may not be adult-only"),
        "age_band_adults_only": ("the accepted explicit full-text age range starts at 18 or above; never generalise "
                                 "to ages 15-17"),
        "age_band_conflicting_ranges": ("explicit full-text age candidates conflict (age_range null); no explicit "
                                        "band; ages 15-17 unverified"),
        "age_band_review_eligibility_includes_minors": ("review-type record whose eligibility or included-study ages "
                                                        "reach below 18; not evidence about ages 15-17 by itself"),
        "age_band_minors_unverified": "youth/adolescent signal without an explicit age range; ages 15-17 unverified",
        "study_type_label_conflict": "primary trial label from index publication type conflicts with the title",
        "excluded_publication_type": "protocol, erratum/correction, editorial or retraction notice",
        "needs_check_documented": "needs_check record with public PubMed / landing-page date evidence recorded",
    }
    for fid in extra_ids:
        flags[fid] = _flag(fid, rules[fid])
    return [flags[k] for k in sorted(flags)]


# --- assembly ---------------------------------------------------------------------------

def augment(rec, ctx):
    """Add the schema-v3 fields to a 1.1.0-shaped record (pure)."""
    extra_flags = []
    lang, lang_detail, lang_flags = publication_language(ctx["epmc"], ctx["crossref"], ctx["openalex"])
    extra_flags += lang_flags
    rec["publication_language"] = lang
    rec["publication_language_detail"] = lang_detail

    is_review = bool(set(ctx["study_type"]["labels"]) & X.REVIEW_LABELS)
    age_items = extract_age_candidates(ctx["paragraphs"], ctx["fulltext_url"]) if ctx["paragraphs"] else []
    if ctx["paragraphs"] is not None:
        value, detail = age_range_detail(age_items, is_review)
    else:
        value, detail = None, {"value": None, "candidate_values": [], "mean_candidates": [],
                               "note": "no licensed full text processed", "rule": AGE_RULE,
                               "status": "not_extracted"}
    if detail["note"].startswith("conflicting"):
        extra_flags.append("age_range_conflicting_candidates")
    rec["age_range"] = value
    rec["age_range_detail"] = detail
    rec["evidence"] = sorted(rec["evidence"] + age_items,
                             key=lambda e: (e["field"], str(e["value"]), e["locator"]["part"],
                                            e["locator"]["section"], e["locator"]["paragraph"], e.get("pattern", "")))
    mesh = age_groups_indexed(ctx["epmc"])
    rec["age_groups_indexed"] = mesh
    band = age_band(age_items, mesh, rec["population"], accepted=value, is_review=is_review)
    rec["age_band"] = band
    extra_flags += {"includes_15_17_explicit": ["age_band_includes_15_17"],
                    "adults_only_explicit": ["age_band_adults_only"],
                    "conflicting_explicit_ranges": ["age_band_conflicting_ranges"],
                    "review_eligibility_includes_minors": ["age_band_review_eligibility_includes_minors"],
                    "minors_possible_unverified": ["age_band_minors_unverified"]}.get(band["summary"], [])

    fulltext_processed = rec["extraction_source"] == "europepmc_oa_fulltext_xml"
    rec["country_evidence_fulltext"] = country_evidence(rec["evidence"], fulltext_processed)

    caution = study_type_caution(ctx["study_type"], ctx["title"])
    rec["study_type_caution"] = caution
    if caution:
        extra_flags.append("study_type_label_conflict")

    elig = publication_type_eligibility(ctx["epmc"], ctx["paper"], ctx["title"])
    rec["publication_type_eligibility"] = elig
    if elig["status"] == "excluded_publication_type":
        extra_flags.append("excluded_publication_type")
        if rec["screening_status"] != "excluded_retracted":
            rec["screening_status"] = "excluded_publication_type"
            rec["screening_reasons"] = [f"{b['kind']} ({b['source']}: {b['term']})" for b in elig["basis"]]

    nce = needs_check_evidence(rec, ctx["envs"], ctx["cutoff_date"], ctx["freeze_date"])
    rec["needs_check_evidence"] = nce
    if nce is not None:
        extra_flags.append("needs_check_documented")

    rec["provenance"].update({
        "publication_language": {"source": lang_detail["source"], "rule": lang_detail["rule"]},
        "age_range": ({"source": "europepmc_fulltext", "endpoint": (ctx["fulltext_env"] or {}).get("endpoint"),
                       "retrieved_at": (ctx["fulltext_env"] or {}).get("fetched_at"), "rule": AGE_RULE}
                      if ctx["paragraphs"] is not None else {"source": None, "rule": "not extracted"}),
        "age_groups_indexed": {"source": "europepmc.meshHeadingList" if mesh else None,
                               "rule": "NLM MeSH age check tags, copied"},
        "age_band": {"source": "derived", "rule": band["rule"]},
        "study_type_caution": {"source": "derived", "rule": "trial primary label without a title trial cue, plus "
                                                            "a title design or association cue"},
        "publication_type_eligibility": {"source": "derived", "rule": ELIG.RULE},
    })
    rec["flags"] = v3_flags(rec, sorted(set(extra_flags)))
    rec["schema_version"] = ctx["profile"].schema_version
    apply_review(rec, ctx["review"], ctx.get("reviewer_file") or "data/research/register_inputs/reviewer_decisions.json")
    return rec
