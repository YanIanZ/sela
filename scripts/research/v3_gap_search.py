#!/usr/bin/env python3
"""Sela v3 gap search: a NEW frozen search run (metadata only) plus MACHINE
title/abstract screening for the v3 evidence gaps (research audit R-3, R-4).

  fetch   network: Europe PMC and Crossref searches per query family and year
          bucket, OpenAlex singleton lookups for the screened-in candidates,
          and Europe PMC abstracts for the false-negative audit sample of the
          7,580 pool. Everything goes through the local, gitignored cache
          data/research/corpus/.cache/v3_search/<freeze>/.
  build   offline: cache -> data/research/v3_gap_search/<freeze>/candidates.jsonl.gz
          + manifest.json (bibliographic metadata, publication language, no
          abstract text: only an abstract SHA-256 and availability flag). The
          candidates are stored as deterministic gzip (corpus.gzstore); recorded
          hashes of 'candidates.jsonl' are of the uncompressed content.
  screen  offline: the run's screening version -> screening/ (index, queue,
          dual-screening worksheet with blank human columns, false-negative
          audit sheet with blank auditor columns, manifest); from v1.1 the
          audit key goes to the local private directory, not the repository.
  verify  offline: re-hash the committed outputs and check the invariants.
  release-audit-key  after a complete audit (v1.2+): copy the withheld key
          into screening/ when it matches the recorded SHA-256.

Runs are versioned and frozen; a later run never rewrites an earlier one:
  sela-v3-gap-search-v1.0  data/research/v3_gap_search/<freeze>/, screening
                           sela-v3-tiab-screen-v1.0 (lexicon sela-v3-families-1)
  sela-v3-gap-search-v1.1  data/research/v3_gap_search/<freeze>_search-1.1/, screening
                           sela-v3-tiab-screen-v1.1 (lexicon sela-v3-families-2, blinded
                           false-negative audit sheet); id_digital now needs a mental-health
                           term, a11y_core is narrowed, a working-age/mid-life adults family
                           is added, and the queries retrieved at 5% or less get a higher
                           per-bucket cap. Unchanged requests reuse the v1.0 cache entries.
  sela-v3-gap-search-v1.2  data/research/v3_gap_search/<freeze>_search-1.2/, screening
                           sela-v3-tiab-screen-v1.2 (same lexicon and rules as v1.1): every hit
                           of the D-048 gap queries and of continuity, safety and help-seeking
                           (Europe PMC cursorMark pagination), 1,000 rows per Indonesian Crossref
                           query, and the audit key withheld until the audit is complete.
                           Unchanged requests reuse the v1.1 (or v1.0) cache entries.

Rules: free public APIs only; no paywall bypass; abstracts are read from the
local cache for screening and NEVER written to committed files; every output
is labelled machine/unreviewed; nothing here is appraisal, and a screening
label is never evidence. Stdlib only. See docs/research/V3_GAP_SEARCH.md.
"""
from __future__ import annotations

import argparse
import collections
import copy
import csv
import datetime as dt
import hashlib
import html
import json
import os
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import urlencode

sys.path.insert(0, str(Path(__file__).resolve().parent))

from corpus import dates as D  # noqa: E402
from corpus import eligibility as ELIG  # noqa: E402
from corpus import fetch as F  # noqa: E402
from corpus import gzstore as S  # noqa: E402
from corpus import ids  # noqa: E402
from corpus import v3fields as V3  # noqa: E402
from corpus import v3lexicon as LX  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
RUN_VERSION = "sela-v3-gap-search-v1.0"
SCREEN_VERSION = "sela-v3-tiab-screen-v1.0"
DEFAULT_FREEZE = "2026-10-26"
OUT_BASE = Path("data/research/v3_gap_search")
CACHE_BASE = Path("data/research/corpus/.cache/v3_search")
REGISTER = Path("docs/research/sela-relay/research_100.json")
POOL = Path("data/research/sela_research_candidates_7580.jsonl")
POOL_INDEX = Path("data/research/sela_screening_index.csv")
# The candidates file of a run: recorded (hashes, inputs) under its logical name, committed compressed.
CANDIDATES = "candidates.jsonl"
CANDIDATES_GZ = CANDIDATES + S.SUFFIX
EPMC_BUCKET_CAP = 50
CROSSREF_ROWS = 40
OPENALEX_MAX = 450
AUTHORS_MAX = 120  # leading author names only; the DOI is the identifier
AUDIT_SEED = "sela-fn-audit-2026-10-26"
AUDIT_STRATA = {  # (source, stratum) -> sample size; deferred rows are the false-negative audit
    ("pool_7580", "deferred"): 100, ("pool_7580", "high"): 20, ("pool_7580", "medium"): 20,
    ("v3_run", "deferred"): 60, ("v3_run", "high"): 20, ("v3_run", "medium"): 20,
}
# Deferred titles the research audit (section 5) named as likely false negatives.
AUDIT_NAMED_DOIS = ("10.1007/s11013-026-10010-7", "10.1192/bjp.2025.10488", "10.1016/j.invent.2026.101005")

# --- query families ---------------------------------------------------------------
# Europe PMC syntax (TITLE_ABS, wildcards, phrases). Crossref queries are plain
# keyword strings (Crossref ranks by relevance; noisier, but it indexes
# Indonesian journals that Europe PMC lacks).
QUERY_FAMILIES = {
    "listening_feeling_heard": {
        "purpose": "listening room: feeling heard, empathic or non-judgemental listening, alliance with digital agents",
        "europepmc": [("listen_core", 'TITLE_ABS:("feel heard" OR "feeling heard" OR "felt heard" OR "being heard" OR '
                                      '"active listening" OR "empathic listening" OR "reflective listening" OR '
                                      '"perceived empathy" OR "feeling understood" OR "digital alliance") AND '
                                      '("mental health" OR wellbeing OR "well-being" OR distress OR loneliness OR '
                                      'psychological)')],
        "crossref": [("listen_cr", "feeling heard empathic listening chatbot mental health")],
    },
    "journaling_expressive_writing": {
        "purpose": "journal: expressive writing, journaling and diaries, including digital journaling",
        "europepmc": [("journal_core", 'TITLE_ABS:("expressive writing" OR journaling OR journalling OR '
                                       '"reflective writing" OR "diary intervention" OR "writing intervention" OR '
                                       '"digital journal" OR "online journal" OR "gratitude journal" OR '
                                       '"emotional disclosure" OR "written disclosure") AND ("mental health" OR '
                                       'depress* OR anxiety OR wellbeing OR "well-being" OR stress OR distress)')],
        "crossref": [("journal_cr", "expressive writing journaling intervention mental health"),
                     ("journal_digital_cr", "digital journaling app wellbeing")],
    },
    "check_in_self_monitoring": {
        "purpose": "check-in and timeline: mood tracking, self-monitoring, momentary assessment in apps",
        "europepmc": [("checkin_core", 'TITLE_ABS:("mood tracking" OR "mood monitoring" OR "self-monitoring" OR '
                                       '"ecological momentary assessment" OR "mood diary" OR "check-in" OR '
                                       '"symptom tracking") AND ("mental health" OR depress* OR anxiety OR '
                                       'wellbeing) AND (app OR smartphone OR digital OR mobile OR online)')],
        "crossref": [("checkin_cr", "mood tracking self-monitoring app mental health users experience")],
    },
    "consultation_preparation": {
        "purpose": "consultation summary: question prompt lists, pre-visit patient-held or patient-generated summaries",
        "europepmc": [("qpl_core", 'TITLE_ABS:("question prompt list" OR "question prompt sheet" OR "prompt list" OR '
                                   '"pre-visit" OR previsit OR "pre-consultation" OR "consultation preparation" OR '
                                   '"visit preparation" OR "patient-held record" OR "patient-generated summary" OR '
                                   '"agenda setting" OR "communication aid") AND (consultation OR visit OR '
                                   'appointment OR clinician OR physician OR "primary care" OR "mental health" OR '
                                   'psychiatr*)')],
        "crossref": [("qpl_cr", "question prompt list consultation preparation patients"),
                     ("previsit_cr", "pre-visit summary patient generated mental health consultation")],
    },
    "continuity_after_consultation": {
        "purpose": "timeline and follow-up: after-visit summaries, open notes, between-session continuity",
        "europepmc": [("continuity_core", 'TITLE_ABS:("after-visit summary" OR "after visit summary" OR '
                                          '"visit summary" OR "open notes" OR opennotes OR "patient portal" OR '
                                          '"between sessions" OR "between-session" OR "post-visit" OR '
                                          '"continuity of care") AND ("mental health" OR psychiatr* OR '
                                          'psychotherap* OR depress*)')],
        "crossref": [("continuity_cr", "open notes mental health patients reading clinical notes")],
    },
    "adolescents_15_17": {
        "purpose": "minors mode: adolescents 15-17, consent, confidentiality, parental or guardian access, digital tools",
        "europepmc": [
            ("teen_consent", 'TITLE_ABS:(adolescen* OR teen* OR minors OR "high school" OR youth) AND '
                             'TITLE_ABS:(consent OR assent OR confidential* OR "parental access" OR "parental consent" OR '
                             'guardian OR "proxy access" OR privacy) AND ("mental health" OR "digital health" OR app '
                             'OR "patient portal" OR psychiatr* OR counsel*)'),
            ("teen_digital", 'TITLE_ABS:(adolescen* OR teen* OR "high school" OR "secondary school") AND '
                             '("digital mental health" OR "mental health app" OR chatbot OR "conversational agent" OR '
                             'smartphone OR "online support" OR "e-mental health")'),
        ],
        "crossref": [("teen_consent_cr", "adolescent confidentiality parental access patient portal mental health"),
                     ("teen_consent_cr2", "adolescent consent digital mental health app minors")],
    },
    "older_adults_50_plus": {
        "purpose": "older adults (50+) with digital mental health tools",
        "europepmc": [("older_digital", 'TITLE_ABS:("older adults" OR "older people" OR elderly OR "late life" OR '
                                        '"middle-aged" OR midlife OR seniors) AND ("digital mental health" OR '
                                        '"mental health app" OR chatbot OR "conversational agent" OR '
                                        '"internet-based" OR "online intervention" OR smartphone OR '
                                        '"digital intervention" OR "voice assistant")')],
        "crossref": [("older_cr", "older adults digital mental health app usability")],
    },
    "accessibility_disability": {
        "purpose": "accessibility and disability in mental-health apps",
        "europepmc": [("a11y_core", 'TITLE_ABS:(accessibility OR accessible OR disability OR disabilities OR '
                                    '"visual impairment" OR blind OR "low vision" OR deaf OR "hard of hearing" OR '
                                    '"screen reader" OR WCAG OR "universal design" OR "intellectual disability" OR '
                                    '"cognitive impairment" OR "low literacy" OR "plain language" OR dyslexia) AND '
                                    '("mental health app" OR "digital mental health" OR mHealth OR "mobile health" OR '
                                    'chatbot OR "e-mental health" OR "online intervention")')],
        "crossref": [("a11y_cr", "accessibility disability mental health app"),
                     ("a11y_cr2", "WCAG accessibility evaluation mental health applications")],
    },
    "indonesia_digital_mh_privacy": {
        "purpose": "Indonesian digital mental health and health-data privacy (UU PDP)",
        "europepmc": [("id_digital", '(Indonesia OR Indonesian) AND ("digital mental health" OR "mental health app" OR '
                                     'chatbot OR "artificial intelligence" OR telemedicine OR telehealth OR '
                                     '"online counseling" OR "mobile health" OR mHealth OR "data privacy" OR '
                                     '"data protection" OR "personal data")')],
        "crossref": [("id_kesehatan_mental_aplikasi", "aplikasi kesehatan mental digital"),
                     ("id_konseling_daring", "konseling online kesehatan mental mahasiswa remaja"),
                     ("id_pdp", "pelindungan data pribadi kesehatan rekam medis"),
                     ("id_digital_en", "Indonesia digital mental health application")],
    },
    "ai_bahasa_code_switching": {
        "purpose": "AI handling of Bahasa Indonesia and Indonesian-English code-switching",
        "europepmc": [("ai_id_lang", '("Bahasa Indonesia" OR "Indonesian language" OR Indonesian OR Malay OR '
                                     '"code-switching" OR "code-mixing" OR "code switching" OR "code mixing" OR '
                                     '"low-resource language") AND ("language model" OR LLM OR "natural language '
                                     'processing" OR NLP OR chatbot) AND (health OR "mental health" OR depress* OR '
                                     'suicid* OR emotion* OR clinical)')],
        "crossref": [("ai_codeswitch_cr", "code-switching Indonesian English language model"),
                     ("ai_id_mh_cr", "deteksi depresi bahasa Indonesia machine learning"),
                     ("ai_id_llm_cr", "large language model Bahasa Indonesia evaluation health")],
    },
    "help_seeking": {
        "purpose": "help-seeking with digital tools, adolescents, older adults or Indonesia",
        "europepmc": [("help_v3", 'TITLE_ABS:("help-seeking" OR "help seeking") AND "mental health" AND '
                                  '(digital OR online OR app OR chatbot OR adolescen* OR "older adults" OR Indonesia*)')],
        "crossref": [("help_cr", "help-seeking mental health digital young people")],
    },
    "safety": {
        "purpose": "safety: crisis and self-harm handling by chatbots and mental-health apps",
        "europepmc": [("safety_v3", 'TITLE_ABS:(suicid* OR "self-harm" OR crisis OR "risk detection" OR safety) AND '
                                    '(chatbot OR "large language model" OR "conversational agent" OR '
                                    '"mental health app") AND (evaluation OR detection OR response OR "red teaming" OR '
                                    'harm)')],
        "crossref": [("safety_cr", "chatbot suicide risk response evaluation large language model")],
    },
    "privacy_sharing_revocation": {
        "purpose": "privacy, sharing and revocation: user views, consent withdrawal, granular or dynamic consent",
        "europepmc": [
            ("privacy_v3", 'TITLE_ABS:(privacy OR confidential* OR "data sharing" OR consent OR "data protection") AND '
                           '("mental health app" OR "digital mental health" OR chatbot OR "patient portal" OR '
                           '"health app") AND (user* OR patient* OR adolescen* OR perception* OR attitude*)'),
            ("revocation_v3", 'TITLE_ABS:("dynamic consent" OR "granular consent" OR "consent withdrawal" OR '
                              '"withdraw consent" OR revocation OR "patient-controlled" OR "data sharing preferences") '
                              'AND (health OR "mental health")'),
        ],
        "crossref": [("revocation_cr", "dynamic consent revocation health data sharing patients")],
    },
}


def _families_1_1():
    """Query families of run v1.1: v1.0 plus the review fixes (never edits v1.0 in place)."""
    fams = copy.deepcopy(QUERY_FAMILIES)
    fams["indonesia_digital_mh_privacy"]["purpose"] = (
        "Indonesian digital mental health (a mental-health term is now required) and Indonesian health-data "
        "privacy (UU PDP) as its own query")
    fams["indonesia_digital_mh_privacy"]["europepmc"] = [
        ("id_digital_mh", 'TITLE_ABS:(Indonesia OR Indonesian) AND TITLE_ABS:("mental health" OR "kesehatan mental" OR '
                          '"kesehatan jiwa" OR depress* OR anxiety OR psychological OR suicid* OR wellbeing OR '
                          '"well-being" OR stress) AND TITLE_ABS:("digital mental health" OR "mental health app" OR '
                          'app OR apps OR smartphone OR chatbot OR "artificial intelligence" OR telemedicine OR '
                          'telehealth OR "online counseling" OR "online counselling" OR online OR internet OR '
                          '"mobile health" OR mHealth OR digital OR "social media")'),
        ("id_health_data_law", 'TITLE_ABS:(Indonesia OR Indonesian) AND TITLE_ABS:("personal data protection" OR '
                               '"data protection" OR "data privacy" OR "health data" OR "medical record" OR '
                               '"medical records" OR "health information" OR confidentiality OR privacy) AND '
                               'TITLE_ABS:(health OR medical OR patient* OR hospital*)'),
    ]
    fams["accessibility_disability"]["purpose"] = (
        "accessibility and disability in mental-health apps (narrowed: digital-accessibility or disability terms, a "
        "mental-health term and a digital-tool term, all in title/abstract; 'accessible' alone and 'cognitive "
        "impairment' dropped)")
    fams["accessibility_disability"]["europepmc"] = [
        ("a11y_core_v2", 'TITLE_ABS:(WCAG OR "digital accessibility" OR "web accessibility" OR "accessibility '
                         'guidelines" OR "accessibility evaluation" OR "screen reader" OR "assistive technology" OR '
                         '"universal design" OR "inclusive design" OR disability OR disabilities OR "visual '
                         'impairment" OR "low vision" OR blind OR deaf OR "hard of hearing" OR "intellectual '
                         'disability" OR "low literacy" OR "plain language" OR "easy read" OR dyslexia) AND '
                         'TITLE_ABS:("mental health" OR depress* OR anxiety OR wellbeing OR "well-being" OR '
                         'psychological OR distress) AND TITLE_ABS:(app OR apps OR smartphone OR chatbot OR '
                         '"conversational agent" OR "online intervention" OR "e-mental health" OR mHealth OR '
                         '"digital mental health" OR "internet-based" OR website OR "digital intervention")'),
    ]
    fams["adolescents_15_17"]["crossref"] = fams["adolescents_15_17"]["crossref"] + [
        ("id_remaja_privasi_cr", "kerahasiaan remaja persetujuan orang tua layanan kesehatan mental")]
    fams["older_adults_50_plus"]["crossref"] = fams["older_adults_50_plus"]["crossref"] + [
        ("id_lansia_cr", "kesehatan mental lansia aplikasi digital teknologi")]
    fams["working_age_adults"] = {
        "purpose": "working-age and mid-life adults (dewasa) with digital mental-health tools, beyond students",
        "europepmc": [("working_adults_digital", 'TITLE_ABS:("working-age" OR "working age" OR employees OR employee '
                                                 'OR workers OR workplace OR occupational OR "middle-aged" OR midlife '
                                                 'OR "mid-life" OR "general adult population" OR "community-dwelling '
                                                 'adults") AND TITLE_ABS:("mental health" OR depress* OR anxiety OR '
                                                 'wellbeing OR "well-being" OR stress OR distress OR burnout) AND '
                                                 'TITLE_ABS:("digital mental health" OR "mental health app" OR chatbot '
                                                 'OR "conversational agent" OR "internet-based" OR "online '
                                                 'intervention" OR smartphone OR "e-mental health" OR "digital '
                                                 'intervention" OR "mobile app")')],
        "crossref": [("working_adults_cr", "employees working adults digital mental health app intervention"),
                     ("id_dewasa_pekerja_cr", "kesehatan mental pekerja dewasa intervensi digital aplikasi")],
    }
    return fams


QUERY_FAMILIES_1_1 = _families_1_1()


EPMC_PAGE_MAX = 1000  # Europe PMC's largest pageSize; an 'all' query follows cursorMark to its last page
AUDIT_SEED_1_2 = "sela-fn-audit-2026-10-26-run-1.2"


@dataclass(frozen=True)
class RunProfile:
    """One frozen search run: its query families, caps, screening version and output directory."""
    run_version: str
    screen_version: str
    lexicon_version: str
    dir_suffix: str
    families: dict = field(hash=False, compare=False)
    caps: dict = field(default_factory=dict, hash=False, compare=False)  # Europe PMC query id -> per-bucket cap
    # v1: unblinded sample (v1.0); v2: blinded sheet + key + excluded stratum (v1.1);
    # v3: v2 with the key withheld from the repository until the audit is complete, a run-specific seed
    #     and a hash of the sheet's machine columns, so that the auditor's entries never break verify (v1.2)
    audit_design: str = "v1"
    epmc_all: frozenset = field(default_factory=frozenset, hash=False, compare=False)  # retrieved in full
    crossref_rows: dict = field(default_factory=dict, hash=False, compare=False)  # Crossref query id -> rows
    previous_runs: tuple = ()  # run versions whose identical requests are reused, newest first
    audit_seed: str = AUDIT_SEED

    @property
    def legacy(self):
        return self.run_version == RUN_VERSION

    def dir_name(self, freeze):
        return f"{freeze}{self.dir_suffix}"

    def cap(self, query_id):
        """Rows per year bucket, or 'all' (every hit, paginated with cursorMark)."""
        if query_id in self.epmc_all:
            return "all"
        return self.caps.get(query_id, EPMC_BUCKET_CAP)

    def crossref_cap(self, query_id):
        return self.crossref_rows.get(query_id, CROSSREF_ROWS)

    def lexicon(self):
        return LX.LEXICONS[self.lexicon_version]

    @property
    def in_run_field(self):
        """Field naming this run's candidate id in a later run ('in_run_1_0', 'in_run_1_1', ...)."""
        return "in_run_" + self.run_version.rsplit("-v", 1)[1].replace(".", "_")


RUN_1_0 = RunProfile(RUN_VERSION, SCREEN_VERSION, LX.LEXICON_VERSION, "", QUERY_FAMILIES)
# Queries retrieved at 5% or less in v1.0 (checkin_core 5.5%, teen_digital 4.9%, older_digital 5.0%; the replaced
# id_digital 1.9% and a11y_core 4.3%) and the new or narrowed queries get 200 rows per year bucket.
RUN_1_1 = RunProfile("sela-v3-gap-search-v1.1", "sela-v3-tiab-screen-v1.1", "sela-v3-families-2", "_search-1.1",
                     QUERY_FAMILIES_1_1,
                     caps={q: 200 for q in ("checkin_core", "teen_digital", "older_digital", "a11y_core_v2",
                                            "id_digital_mh", "id_health_data_law", "working_adults_digital")},
                     audit_design="v2", previous_runs=(RUN_VERSION,))
# Run v1.2 (review round 3): every hit of the D-048 gap queries (listening, journaling, consultation preparation,
# adolescents' consent and confidentiality, older adults) and of continuity, safety and help-seeking, which v1.1
# still truncated to the relevance top 50 per year bucket (9.5% to 40.9% retrieved). Accessibility
# (a11y_core_v2) was already complete at 200 per bucket (665 of 665) and keeps that request. The Indonesian-language
# Crossref queries are raised from 40 rows to Crossref's largest page (1,000 rows): a first pass at 200 rows still
# found 35-40 Indonesian-titled rows in ranks 161-200 of most of them. Same query strings and lexicon as v1.1.
CROSSREF_ROWS_MAX = 1000
EPMC_ALL_1_2 = frozenset({"listen_core", "journal_core", "qpl_core", "teen_consent", "older_digital",
                          "continuity_core", "safety_v3", "help_v3"})
CROSSREF_ID_QUERIES = ("id_kesehatan_mental_aplikasi", "id_konseling_daring", "id_pdp", "id_remaja_privasi_cr",
                       "id_lansia_cr", "id_dewasa_pekerja_cr", "ai_id_mh_cr")
RUN_1_2 = RunProfile("sela-v3-gap-search-v1.2", "sela-v3-tiab-screen-v1.2", "sela-v3-families-2", "_search-1.2",
                     QUERY_FAMILIES_1_1, caps=dict(RUN_1_1.caps), audit_design="v3", epmc_all=EPMC_ALL_1_2,
                     crossref_rows={q: CROSSREF_ROWS_MAX for q in CROSSREF_ID_QUERIES},
                     previous_runs=("sela-v3-gap-search-v1.1", RUN_VERSION), audit_seed=AUDIT_SEED_1_2)
RUNS = {r.run_version: r for r in (RUN_1_0, RUN_1_1, RUN_1_2)}
LATEST = RUN_1_2


def run_for_dir(name):
    """Run profile of an output directory name ('2026-10-26', '2026-10-26_search-1.1', ...)."""
    for r in RUNS.values():
        if r.dir_suffix and name.endswith(r.dir_suffix):
            return r
    return RUN_1_0


def previous_runs(run):
    return [RUNS[v] for v in run.previous_runs]


def year_buckets(cutoff, freeze):
    y0, y1 = int(cutoff[:4]), int(freeze[:4])
    out = []
    for y in range(y0, y1 + 1):
        a = cutoff if y == y0 else f"{y}-01-01"
        b = freeze if y == y1 else f"{y}-12-31"
        out.append((a, b))
    return out


def epmc_search_request(query, bucket, page_size=EPMC_BUCKET_CAP, cursor="*"):
    q = f"({query}) AND (SRC:MED OR SRC:PMC) AND FIRST_PDATE:[{bucket[0]} TO {bucket[1]}]"
    return F.EPMC_SEARCH + "?" + urlencode({"query": q, "format": "json", "resultType": "core",
                                            "pageSize": page_size, "cursorMark": cursor})


def crossref_search_request(query, cutoff, freeze, rows=CROSSREF_ROWS):
    return "https://api.crossref.org/works?" + urlencode({
        "query": query, "rows": rows, "sort": "relevance",
        "filter": f"from-pub-date:{cutoff},until-pub-date:{freeze},type:journal-article",
        # no 'select': Crossref's select list has no 'language' field; the response is
        # whitelisted by trim_crossref before it is cached
    })


def openalex_request(doi):
    return F.openalex_request(doi)


def epmc_ext_ids_request(pmids):
    q = "(" + " OR ".join(f"EXT_ID:{p}" for p in pmids) + ") AND SRC:MED"
    return F.EPMC_SEARCH + "?" + urlencode({"query": q, "format": "json", "resultType": "core",
                                            "pageSize": max(25, len(pmids))})


# --- trimming (what the local cache keeps) ------------------------------------------

EPMC_KEEP = ("id", "source", "pmid", "pmcid", "doi", "title", "authorString", "journalInfo", "pubYear",
             "firstPublicationDate", "electronicPublicationDate", "pubTypeList", "language", "isOpenAccess",
             "inEPMC", "license", "meshHeadingList", "abstractText", "hasTextMinedTerms")
CR_KEEP = ("DOI", "title", "container-title", "published-online", "published-print", "issued", "created", "type",
           "language", "license", "update-to", "abstract", "author", "subject")


def trim_epmc(body):
    results = []
    for r in ((body or {}).get("resultList") or {}).get("result") or []:
        t = {k: r[k] for k in EPMC_KEEP if k in r}
        if "authorString" in t:
            t["authorString"] = str(t["authorString"])[:300]
        if "journalInfo" in t:
            ji = t["journalInfo"] or {}
            t["journalInfo"] = {"dateOfPublication": ji.get("dateOfPublication"),
                                "journal": {"title": ((ji.get("journal") or {}).get("title"))}}
        if "meshHeadingList" in t:
            t["meshHeadingList"] = {"meshHeading": [{"descriptorName": h.get("descriptorName")}
                                                    for h in (t["meshHeadingList"] or {}).get("meshHeading") or []]}
        results.append(t)
    return {"hitCount": (body or {}).get("hitCount"), "resultList": {"result": results}}


def trim_epmc_paged(body):
    """trim_epmc plus the cursor for the next page (only for requests that retrieve every hit)."""
    out = trim_epmc(body)
    out["nextCursorMark"] = (body or {}).get("nextCursorMark")
    return out


def trim_crossref(body):
    msg = (body or {}).get("message") or {}
    items = []
    for it in msg.get("items") or []:
        t = {k: it[k] for k in CR_KEEP if k in it}
        if "author" in t:
            t["author"] = [{"family": a.get("family"), "given": (a.get("given") or "")[:1]} for a in t["author"][:6]]
        items.append(t)
    return {"message": {"total-results": msg.get("total-results"), "items": items}}


def get_json(client, cache, service, key, url, trim, refresh=False, log=print):
    cached = cache.get(service, key)
    if cached is not None and not refresh:
        return cached, False
    status, raw, error = client.get(service, url)
    fetched_at = F.utc_now()
    if status == 200:
        try:
            body = trim(json.loads(raw.decode("utf-8")))
        except (ValueError, UnicodeDecodeError) as exc:
            return {"fetch_error": f"invalid JSON: {exc}"}, True
        env = F.envelope(service, key, url, fetched_at, 200, "ok", body=body)
    elif status == 404:
        env = F.envelope(service, key, url, fetched_at, 404, "not_found")
    else:
        return {"fetch_error": error or f"HTTP {status}"}, True
    cache.put(service, key, env)
    return env, True


# --- normalisation -------------------------------------------------------------------

_TAG = re.compile(r"<[^>]+>")


def clean_text(text):
    return " ".join(html.unescape(_TAG.sub(" ", str(text or ""))).split())


def crossref_dates(item):
    out = {}
    for key in ("published-online", "published-print", "issued", "created"):
        v = item.get(key)
        if v:
            parts = (v.get("date-parts") or [[None]])[0]
            if parts and parts[0]:
                out[key] = "-".join([str(parts[0])] + [f"{int(p):02d}" for p in parts[1:3]])
    return out


def window_status(date, cutoff, freeze):
    if not date:
        return None
    lo, hi = V3._bounds(date)
    if cutoff <= lo and hi <= freeze:
        return True
    if hi < cutoff or lo > freeze:
        return False
    return None


# Indonesian function words: two or more distinct ones in a title is a cue (never a value)
# that the publication may be in Indonesian whatever the deposited metadata says.
_ID_WORDS = re.compile(r"\b(?:dan|yang|untuk|dengan|pada|terhadap|dalam|di|dari|kepada|sebagai|melalui|bagi|"
                       r"antara|tentang|oleh|kesehatan|remaja|mahasiswa|pengaruh|hubungan|penggunaan|aplikasi|"
                       r"layanan|masyarakat|siswa|anak)\b", re.I)


def indonesian_title_cue(title):
    return len({m.group(0).lower() for m in _ID_WORDS.finditer(title or "")}) >= 2


def norm_title_key(title, year):
    return ids.normalize_title(title) + "|" + str(year or "")


def _as_list(v):
    if v is None:
        return []
    return v if isinstance(v, list) else [v]


def candidate_from_epmc(r):
    pts = [str(x) for x in _as_list(((r.get("pubTypeList") or {}).get("pubType")))]
    date = r.get("firstPublicationDate") or r.get("electronicPublicationDate")
    return {
        "title": clean_text(r.get("title")),
        "authors": clean_text(r.get("authorString"))[:AUTHORS_MAX] or None,
        "journal": ((r.get("journalInfo") or {}).get("journal") or {}).get("title"),
        "doi": ids.normalize_doi(r.get("doi")),
        "pmid": ids.normalize_pmid(r.get("pmid")),
        "pmcid": ids.normalize_pmcid(r.get("pmcid")),
        "first_publication_date": date,
        "date_source": "europepmc.firstPublicationDate" if r.get("firstPublicationDate") else
        ("europepmc.electronicPublicationDate" if date else None),
        "publication_types": sorted(set(pts)),
        "languages_raw": {"europepmc.language": r.get("language")},
        "is_open_access": r.get("isOpenAccess") == "Y",
        "license": r.get("license"),
        "mesh_age_groups": V3.age_groups_indexed(r),
        "abstract": clean_text(r.get("abstractText")) or None,
        "abstract_source": "europepmc" if r.get("abstractText") else None,
        "source_record": f"europepmc:{r.get('source')}:{r.get('id')}",
        "crossref_update_to": None,
    }


def candidate_from_crossref(it):
    dates = crossref_dates(it)
    day = sorted(d for k, d in dates.items() if k in ("published-online", "published-print") and len(d) == 10)
    part = sorted(d for k, d in dates.items() if k in ("published-online", "published-print", "issued"))
    date = day[0] if day else (part[0] if part else None)
    title = clean_text((it.get("title") or [""])[0])
    authors = ", ".join(f"{a.get('family') or ''} {a.get('given') or ''}".strip() for a in it.get("author") or [])
    lic = [l.get("URL") for l in it.get("license") or [] if l.get("URL")]
    return {
        "title": title,
        "authors": authors[:AUTHORS_MAX] or None,
        "journal": (it.get("container-title") or [None])[0],
        "doi": ids.normalize_doi(it.get("DOI")),
        "pmid": None, "pmcid": None,
        "first_publication_date": date,
        "date_source": ("crossref.earliest_of_published_online_print" if day else
                        "crossref.issued_or_partial" if date else None),
        "publication_types": sorted({str(it.get("type") or "")} - {""}),
        "languages_raw": {"crossref.language": it.get("language")},
        "is_open_access": None,
        "license": sorted(set(lic))[0] if lic else None,
        "mesh_age_groups": None,
        "abstract": clean_text(it.get("abstract")) or None,
        "abstract_source": "crossref" if it.get("abstract") else None,
        "source_record": f"crossref:{ids.normalize_doi(it.get('DOI'))}",
        "crossref_update_to": [u.get("type") for u in it.get("update-to") or []] or None,
    }


# --- cache walk -------------------------------------------------------------------------

def cache_dir(root, freeze, run=None):
    """Each run has its own local cache; a later run copies the identical requests it reuses from the
    v1.0 cache at fetch time and never writes into it (so v1.0 rebuilds stay byte-identical)."""
    return Path(root) / CACHE_BASE / (run or RUN_1_0).dir_name(freeze)


def report_name(run):
    return "fetch_report.json"


def search_plan(freeze, run=RUN_1_0):
    """Every request of a run. A cap other than 50 is part of the cache key, so a v1.1 request reuses
    a v1.0 cache entry only when query, bucket and cap are identical."""
    cutoff = D.cutoff_for(freeze)
    plan = []
    for fam, spec in run.families.items():
        for qid, q in spec["europepmc"]:
            cap = run.cap(qid)
            for b in year_buckets(cutoff, freeze):
                if cap == "all":  # first page; later pages follow nextCursorMark (epmc_pages)
                    plan.append({"source": "europepmc", "family": fam, "query_id": qid, "bucket": f"{b[0]}..{b[1]}",
                                 "key": f"epmc:{qid}:{b[0]}:{b[1]}:all",
                                 "url": epmc_search_request(q, b, page_size=EPMC_PAGE_MAX), "service": "europepmc",
                                 "cap": "all", "paged": True, "query": q})
                    continue
                suffix = "" if cap == EPMC_BUCKET_CAP else f":n{cap}"
                item = {"source": "europepmc", "family": fam, "query_id": qid, "bucket": f"{b[0]}..{b[1]}",
                        "key": f"epmc:{qid}:{b[0]}:{b[1]}{suffix}", "url": epmc_search_request(q, b, page_size=cap),
                        "service": "europepmc"}
                if not run.legacy:
                    item["cap"] = cap
                plan.append(item)
        for qid, q in spec["crossref"]:
            rows = run.crossref_cap(qid)
            suffix = "" if rows == CROSSREF_ROWS else f":n{rows}"
            item = {"source": "crossref", "family": fam, "query_id": qid, "bucket": f"{cutoff}..{freeze}",
                    "key": f"crossref:{qid}{suffix}", "url": crossref_search_request(q, cutoff, freeze, rows=rows),
                    "service": "crossref_search"}
            if not run.legacy:
                item["cap"] = rows
            plan.append(item)
    return plan


def page_key(item, n):
    return item["key"] if n == 1 else f"{item['key']}:p{n}"


def epmc_pages(cache, item):
    """Cached pages of an 'all' request: ([(key, envelope)], next missing page as (key, cursor) or None).
    A bucket is complete when a page returns fewer than EPMC_PAGE_MAX rows, the rows reach hitCount, or
    Europe PMC returns the cursor it was sent (its end-of-results signal)."""
    pages, cursor, seen, total, n = [], "*", 0, None, 1
    while True:
        key = page_key(item, n)
        env = cache.get(item["service"], key)
        if env is None:
            return pages, (key, cursor)
        pages.append((key, env))
        body = env.get("body") or {}
        results = (body.get("resultList") or {}).get("result") or []
        total = body.get("hitCount") if total is None else total
        seen += len(results)
        nxt = body.get("nextCursorMark")
        if (env.get("status") != "ok" or len(results) < EPMC_PAGE_MAX or seen >= (total or 0) or not nxt
                or nxt == cursor):
            return pages, None
        cursor, n = nxt, n + 1


def _rank_bands(cands, width=40):
    """Indonesian title cues per rank band of one Crossref query (a check of how deep the relevance
    ranking still finds Indonesian records)."""
    out = []
    for start in range(0, len(cands), width):
        band = cands[start:start + width]
        out.append({"ranks": f"{start + 1}-{start + len(band)}", "rows": len(band),
                    "indonesian_title_cue": sum(1 for c in band if indonesian_title_cue(c["title"]))})
    return out


def raw_hits(cache, plan):
    """[(plan_item, rank, candidate_dict)] in plan order; plus per-request stats and missing keys."""
    rows, stats, missing = [], [], []
    for item in plan:
        if item.get("paged"):
            pages, need = epmc_pages(cache, item)
            if need is not None:
                missing.append(need[0])
                continue
            cands = [candidate_from_epmc(r) for _k, env in pages
                     for r in ((env.get("body") or {}).get("resultList") or {}).get("result") or []]
            first = pages[0][1]
            total = (first.get("body") or {}).get("hitCount")
            stats.append({"key": item["key"], "source": item["source"], "family": item["family"],
                          "query_id": item["query_id"], "bucket": item["bucket"], "http_status": first.get("http_status"),
                          "retrieved_at": first.get("fetched_at"), "retrieved_at_last_page": pages[-1][1].get("fetched_at"),
                          "hit_count": total, "returned": len(cands), "cap": "all", "pages": len(pages),
                          "complete": total is not None and len(cands) >= total})
            for rank, c in enumerate(cands, start=1):
                rows.append((item, rank, c))
            continue
        env = cache.get(item["service"], item["key"])
        if env is None:
            missing.append(item["key"])
            continue
        body = env.get("body") or {}
        if item["source"] == "europepmc":
            results = (body.get("resultList") or {}).get("result") or []
            total = body.get("hitCount")
            cands = [candidate_from_epmc(r) for r in results]
        else:
            msg = body.get("message") or {}
            results = msg.get("items") or []
            total = msg.get("total-results")
            cands = [candidate_from_crossref(it) for it in results]
        stat = {"key": item["key"], "source": item["source"], "family": item["family"],
                "query_id": item["query_id"], "bucket": item["bucket"], "http_status": env.get("http_status"),
                "retrieved_at": env.get("fetched_at"), "hit_count": total, "returned": len(cands)}
        if "cap" in item:
            stat["cap"] = item["cap"]
        if item["source"] == "crossref" and item.get("cap", CROSSREF_ROWS) > CROSSREF_ROWS:
            stat["indonesian_title_cue_by_rank_band"] = _rank_bands(cands)
        stats.append(stat)
        for rank, c in enumerate(cands, start=1):
            rows.append((item, rank, c))
    return rows, stats, missing


def merge_candidates(rows, cutoff, freeze):
    by_key, order = {}, []
    alias = {}
    for item, rank, c in rows:
        if not c["title"]:
            continue
        year = (c["first_publication_date"] or "")[:4]
        keys = [k for k in (("doi:" + c["doi"]) if c["doi"] else None, ("pmid:" + c["pmid"]) if c["pmid"] else None,
                            "title:" + norm_title_key(c["title"], year)) if k]
        hit = next((alias[k] for k in keys if k in alias), None)
        hit_rec = {"source": item["source"], "family": item["family"], "query_id": item["query_id"],
                   "bucket": item["bucket"], "rank": rank}
        if hit is None:
            rec = dict(c)
            rec["retrieval"] = [hit_rec]
            rec["source_records"] = [c["source_record"]]
            hit = len(order)
            order.append(rec)
        else:
            rec = order[hit]
            rec["retrieval"].append(hit_rec)
            if c["source_record"] not in rec["source_records"]:
                rec["source_records"].append(c["source_record"])
            for f in ("doi", "pmid", "pmcid", "journal", "authors", "license", "mesh_age_groups"):
                if not rec.get(f) and c.get(f):
                    rec[f] = c[f]
            rec["languages_raw"].update({k: v for k, v in c["languages_raw"].items() if v})
            rec["publication_types"] = sorted(set(rec["publication_types"]) | set(c["publication_types"]))
            if c["date_source"] and str(c["date_source"]).startswith("europepmc") and not str(
                    rec.get("date_source") or "").startswith("europepmc"):
                rec["first_publication_date"], rec["date_source"] = c["first_publication_date"], c["date_source"]
            if not rec.get("abstract") and c.get("abstract"):
                rec["abstract"], rec["abstract_source"] = c["abstract"], c["abstract_source"]
            if c.get("crossref_update_to"):
                rec["crossref_update_to"] = c["crossref_update_to"]
        for k in keys:
            alias.setdefault(k, hit)
    return order


def load_register_keys(root):
    obj = json.loads((Path(root) / REGISTER).read_text(encoding="utf-8"))
    doi, pmid = {}, {}
    for p in obj["papers"]:
        if p.get("doi"):
            doi[ids.normalize_doi(p["doi"])] = p["id"]
        pm = ids.pmid_from_url(p.get("index_url"))
        if pm:
            pmid[pm] = p["id"]
    return doi, pmid


def load_pool_keys(root):
    doi, pmid, title = {}, {}, {}
    for i, line in enumerate((Path(root) / POOL).read_text(encoding="utf-8").splitlines(), start=1):
        r = json.loads(line)
        cid = f"SL-{i:05d}"
        if r.get("doi"):
            doi[ids.normalize_doi(r["doi"])] = cid
        if r.get("pmid"):
            pmid[str(r["pmid"])] = cid
        title[norm_title_key(r["title"], (r.get("first_publication_date") or "")[:4])] = cid
    return doi, pmid, title


def build_candidates(root, freeze, cache, run=RUN_1_0):
    cutoff = D.cutoff_for(freeze)
    plan = search_plan(freeze, run)
    rows, stats, missing = raw_hits(cache, plan)
    merged = merge_candidates(rows, cutoff, freeze)
    rdoi, rpmid = load_register_keys(root)
    pdoi, ppmid, ptitle = load_pool_keys(root)
    out = []
    for c in merged:
        year = (c["first_publication_date"] or "")[:4]
        oa_env = cache.get("openalex", F.openalex_request(c["doi"])[0]) if c["doi"] else None
        oa = (oa_env or {}).get("body") if (oa_env or {}).get("status") == "ok" else None
        if oa and oa.get("language"):
            c["languages_raw"]["openalex.language"] = oa.get("language")
        epmc_l = {"language": c["languages_raw"].get("europepmc.language")}
        cr_l = {"language": c["languages_raw"].get("crossref.language")}
        lang, ldetail, lflags = V3.publication_language(epmc_l, cr_l, {"language": c["languages_raw"].get(
            "openalex.language")})
        elig = ELIG.eligibility(c["publication_types"], c["title"])
        if c.get("crossref_update_to"):
            elig["basis"].append({"kind": "correction_or_erratum", "source": "crossref.update-to",
                                  "term": ",".join(c["crossref_update_to"])})
            elig["excluded_kinds"] = sorted(set(elig["excluded_kinds"]) | {"correction_or_erratum"})
            elig["status"] = "excluded_publication_type"
        if oa and oa.get("is_retracted"):
            elig["basis"].append({"kind": "retraction_notice_or_retracted", "source": "openalex.is_retracted",
                                  "term": "true"})
            elig["excluded_kinds"] = sorted(set(elig["excluded_kinds"]) | {"retraction_notice_or_retracted"})
            elig["status"] = "excluded_publication_type"
        abstract = c.pop("abstract", None)
        rec = {
            "title": c["title"], "authors": c["authors"], "journal": c["journal"], "doi": c["doi"],
            "pmid": c["pmid"], "pmcid": c["pmcid"],
            "first_publication_date": c["first_publication_date"], "date_source": c["date_source"],
            "within_window": window_status(c["first_publication_date"], cutoff, freeze),
            "publication_types": c["publication_types"],
            "publication_type_eligibility": elig,
            "publication_language": lang,
            "publication_language_detail": dict(
                {k: ldetail[k] for k in ("source", "basis", "raw", "curated_sources_disagree", "notes", "status")},
                title_suggests_indonesian=indonesian_title_cue(c["title"]),
                title_cue_disagrees=bool(indonesian_title_cue(c["title"]) and lang not in (None, "id"))),
            "is_open_access": c["is_open_access"], "license": c["license"],
            "mesh_age_groups": c["mesh_age_groups"],
            "abstract_available": bool(abstract),
            "abstract_source": c["abstract_source"] if abstract else None,
            "abstract_sha256": hashlib.sha256(abstract.encode("utf-8")).hexdigest() if abstract else None,
            "openalex": ({"id": oa.get("id"), "language": oa.get("language"), "type": oa.get("type"),
                          "is_retracted": oa.get("is_retracted"), "retrieved_at": oa_env.get("fetched_at")}
                         if oa else None),
            "families_retrieved": sorted({h["family"] for h in c["retrieval"]}),
            "retrieval": sorted(c["retrieval"], key=lambda h: (h["family"], h["query_id"], h["bucket"], h["rank"])),
            "source_records": sorted(c["source_records"]),
            "in_register": (rdoi.get(c["doi"]) if c["doi"] else None) or (rpmid.get(c["pmid"]) if c["pmid"] else None),
            "in_pool_7580": ((pdoi.get(c["doi"]) if c["doi"] else None) or (ppmid.get(c["pmid"]) if c["pmid"] else None)
                             or ptitle.get(norm_title_key(c["title"], year))),
            "status": "candidate_metadata_only_unreviewed",
        }
        out.append((rec, abstract))
    out.sort(key=lambda t: (t[0]["first_publication_date"] or "9999", ids.normalize_title(t[0]["title"]),
                            t[0]["doi"] or "", t[0]["pmid"] or ""))
    final, abstracts = [], {}
    for n, (rec, abstract) in enumerate(out, start=1):
        cid = f"V3-{n:05d}"
        final.append(dict({"candidate_id": cid}, **rec))
        if abstract:
            abstracts[cid] = abstract
    return final, abstracts, stats, missing, plan


# --- screening (machine, title/abstract) ---------------------------------------------------

WEIGHT_TITLE, WEIGHT_ABSTRACT = 4, 2


# Indonesian age phrases (screening v1.1 only): "berusia 15-17 tahun", "usia 12 sampai 18 tahun",
# "usia 60 tahun ke atas". Values use the corpus notation (range:lo-hi, min:lo).
_ID_AGE_RANGE = re.compile(r"\b(?:usia|umur|berusia|berumur)\s+(?:antara\s+)?(\d{1,2})\s*(?:-|–|sampai|hingga|"
                           r"s\.?d\.?|dan)\s*(\d{1,2})\s*tahun\b", re.I)
_ID_AGE_MIN = re.compile(r"\b(?:usia|umur|berusia|berumur)\s+(?:minimal\s+|≥\s*)?(\d{1,2})\s*tahun\s+"
                         r"(?:ke\s+atas|atau\s+lebih)\b", re.I)


def id_age_values(text):
    out = set()
    for m in _ID_AGE_RANGE.finditer(text or ""):
        lo, hi = int(m.group(1)), int(m.group(2))
        if 0 <= lo < hi <= 110:
            out.add(f"range:{lo}-{hi}")
    for m in _ID_AGE_MIN.finditer(text or ""):
        out.add(f"min:{int(m.group(1))}")
    return out


def screen_one(title, abstract, mesh, eligibility, within_window=True, lexicon=None):
    """Machine title/abstract screen. ``lexicon`` None is sela-v3-families-1 (screening v1.0,
    byte-identical behaviour); sela-v3-families-2 is screening v1.1."""
    lx = lexicon or LX.FAMILIES_1
    v2 = lx is not LX.FAMILIES_1
    text = (title or "") + " " + (abstract or "")
    fam = lx.family_hits(title, abstract or "")
    mh_t, mh_a = lx.hits(lx.MENTAL_HEALTH, title), lx.hits(lx.MENTAL_HEALTH, abstract or "")
    digital = lx.hits(lx.DIGITAL, text)
    design = lx.hits(lx.DESIGN, text)
    null_neg = lx.hits(lx.NULL_OR_NEGATIVE, abstract or "")
    pops = {k: lx.hits(v, text) for k, v in lx.POPULATION.items()}
    if mesh:
        if set(mesh) & V3.MESH_MINOR_GROUPS:
            pops["adolescent_signal"] = sorted(set(pops["adolescent_signal"]) | {"mesh:" + m for m in mesh
                                                                                 if m in V3.MESH_MINOR_GROUPS})
        # MeSH 'Middle Aged' (45-64) is tagged on most adult studies, so it counts as an adult
        # signal only; the 50+ signal needs 'Aged' (65+) or explicit older-adult words or ages
        if set(mesh) & {"Aged", "Aged, 80 and over"}:
            pops["older_adult_signal"] = sorted(set(pops["older_adult_signal"]) | {
                "mesh:" + m for m in mesh if m in ("Aged", "Aged, 80 and over")})
        if set(mesh) & {"Adult", "Young Adult", "Middle Aged"}:
            pops["adult_signal"] = sorted(set(pops["adult_signal"]) | {"mesh:" + m for m in mesh
                                                                       if m in ("Adult", "Young Adult", "Middle Aged")})
    cands = V3.extract_age_candidates(
        [{"part": "abstract", "section": "(screening)", "paragraph": 1, "text": abstract}], None) if abstract else []
    if v2:
        # v1.1: ages describing an advisory/working group do not vote; Indonesian age phrases count
        ages = sorted({i["value"] for i in cands if not i.get("non_voting")} | id_age_values(text))
        non_voting_ages = sorted({i["value"] for i in cands if i.get("non_voting")})
    else:
        ages = sorted({i["value"] for i in cands})
    feature = {f: v for f, v in fam.items() if f in lx.FEATURE_FAMILIES}
    # family-specific gates: consent terms count for minors only with an adolescent hit,
    # and the AI-language family only with a digital/AI term
    if "adolescent_consent_confidentiality" in feature and "adolescents" not in fam and not pops["adolescent_signal"]:
        feature.pop("adolescent_consent_confidentiality")
    if "ai_language_code_switching" in feature and not lx.hits(lx.AI_TERMS, text):
        feature.pop("ai_language_code_switching")
    score = 0
    for f, v in feature.items():
        score += WEIGHT_TITLE if v["title"] else WEIGHT_ABSTRACT
    score += 2 if mh_t else (1 if mh_a else 0)
    score += 1 if digital else 0
    score += 2 if (pops["adolescent_signal"] or pops["older_adult_signal"]) else 0
    score += 2 if "indonesian_context" in feature else 0
    score += 1 if design else 0
    has_mh = bool(mh_t or mh_a)
    title_feature = any(v["title"] for v in feature.values())
    if eligibility["status"] != "eligible":
        priority = "excluded_publication_type"
    elif within_window is False:
        priority = "excluded_out_of_window"
    elif has_mh and feature and (title_feature and score >= 8 or score >= 11):
        priority = "high"
    elif feature and (has_mh and score >= 5 or score >= 8):
        priority = "medium"
    else:
        priority = "deferred"
    out = {
        "priority": priority, "score": score,
        "feature_families": {f: {"title": v["title"], "abstract": v["abstract"]} for f, v in sorted(feature.items())},
        "mental_health_terms": {"title": mh_t, "abstract": mh_a},
        "digital_terms": digital, "design_cues": design,
        "population_signals": {k: v for k, v in sorted(pops.items())},
        "abstract_age_values": ages,
        "possible_null_or_negative_finding": null_neg,
        "abstract_used": bool(abstract),
        "status": "machine_title_abstract_screen_unreviewed",
    }
    if v2:
        out["age_values_not_counted"] = non_voting_ages
        out["lexicon_version"] = lx.version
    return out


def population_band(s):
    p = s["population_signals"]
    ages = [V3._parse_age_value(v) for v in s["abstract_age_values"] if not v.startswith("mean:")]
    explicit_minor = any(lo <= 17 and (hi if hi is not None else 999) >= 15 for _, lo, hi in ages)
    explicit_adult_only = ages and all(lo >= 18 for _, lo, _hi in ages)
    if explicit_minor:
        return "includes_15_17_explicit"
    if p["adolescent_signal"] and not explicit_adult_only:
        return "adolescent_or_youth_unverified"
    if p["older_adult_signal"]:
        return "older_adults_signal"
    if p["adult_signal"] or explicit_adult_only:
        return "adults_signal"
    return "not_stated"


POPULATION_BAND_V2_RULE = (
    "Screening v1.1: includes_15_17_explicit only when the title/abstract ages agree on ONE range or minimum "
    "that overlaps 15-17 and the record is not a review (a review's ages are eligibility rules, not one sample), "
    "the same rule as the corpus age band; ages that describe an advisory or working group do not count. Any "
    "other minor age, or youth words without such a range, is adolescent_or_youth_unverified.")


def population_band_v2(s):
    """See POPULATION_BAND_V2_RULE (a constant, so the manifest text never depends on how a Python
    version formats docstrings)."""
    p = s["population_signals"]
    ages = [V3._parse_age_value(v) for v in s["abstract_age_values"] if not v.startswith("mean:")]
    distinct = {(lo, hi) for _, lo, hi in ages}
    is_review = "systematic review" in s["design_cues"]
    overlaps = [lo <= 17 and (hi if hi is not None else 999) >= 15 for lo, hi in distinct]
    explicit_adult_only = bool(ages) and all(lo >= 18 for _, lo, _hi in ages)
    if len(distinct) == 1 and overlaps[0] and not is_review:
        return "includes_15_17_explicit"
    if (p["adolescent_signal"] or any(lo < 18 for lo, _hi in distinct)) and not explicit_adult_only:
        return "adolescent_or_youth_unverified"
    if p["older_adult_signal"]:
        return "older_adults_signal"
    if p["adult_signal"] or explicit_adult_only:
        return "adults_signal"
    return "not_stated"


def rank_key(seed, stratum, cid):
    return hashlib.sha256(f"{seed}|{stratum}|{cid}".encode("utf-8")).hexdigest()


def select_queue(screened, maximum=400, per_family=30, families=None):
    """Stratified by feature family so every v3 feature gets candidates; high before medium."""
    eligible = [s for s in screened if s["screen"]["priority"] in ("high", "medium")]
    key = lambda s: (0 if s["screen"]["priority"] == "high" else 1, -s["screen"]["score"], s["candidate_id"])

    def fam_key(fam):
        # a title hit for THIS family first, so broad multi-topic records do not crowd out specific ones
        return lambda s: ((0 if s["screen"]["feature_families"][fam]["title"] else 1),) + key(s)
    chosen, seen = [], set()
    for fam in families or LX.FEATURE_FAMILIES:
        pool = sorted((s for s in eligible if fam in s["screen"]["feature_families"]), key=fam_key(fam))
        for s in pool[:per_family]:
            if s["candidate_id"] not in seen and len(chosen) < maximum:
                chosen.append(s)
                seen.add(s["candidate_id"])
    for s in sorted(eligible, key=key):
        if len(chosen) >= maximum:
            break
        if s["candidate_id"] not in seen:
            chosen.append(s)
            seen.add(s["candidate_id"])
    return sorted(chosen, key=key)


def write_csv(path, fields, rows):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fields, lineterminator="\n")
        w.writeheader()
        for r in rows:
            w.writerow(r)


def _sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def jsonl_bytes(rows):
    return "".join(json.dumps(r, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n"
                   for r in rows).encode("utf-8")


def json_bytes(obj):
    return (json.dumps(obj, ensure_ascii=False, sort_keys=True, indent=2) + "\n").encode("utf-8")


def atomic_write(path, data):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(path.name + ".tmp")
    tmp.write_bytes(data)
    os.replace(tmp, path)


def candidates_storage(gz):
    """Run-manifest entry for the committed .gz (``candidates_sha256`` stays the hash of the content)."""
    return {"file": CANDIDATES_GZ, "storage_version": S.STORAGE_VERSION, "format": S.FORMAT,
            "gz_sha256": hashlib.sha256(gz).hexdigest(),
            "note": "candidates_sha256, and every hash recorded elsewhere under the name candidates.jsonl (inputs of "
                    "later runs, screening/manifest.json, the v3 evidence map), is the SHA-256 of the uncompressed "
                    "content of this file; gz_sha256 is the SHA-256 of its committed bytes"}


# --- pool audit sample -------------------------------------------------------------------

def pool_index_rows(root):
    with (Path(root) / POOL_INDEX).open(encoding="utf-8", newline="") as f:
        return list(csv.DictReader(f))


AUDIT_STRATA_V2 = {**AUDIT_STRATA,  # screening v1.1 adds a check of the publication-type rule
                   ("pool_7580", "excluded_publication_type"): 10, ("v3_run", "excluded_publication_type"): 10}


def audit_strata(run):
    return AUDIT_STRATA if run.audit_design == "v1" else AUDIT_STRATA_V2


def pool_sample(root, run=RUN_1_0):
    index = pool_index_rows(root)
    rows = [r for r in index if r.get("eligibility", "eligible") == "eligible"]
    excluded = [r for r in index if r.get("eligibility", "eligible") == "excluded_publication_type"]
    picked = []
    for (source, stratum), n in audit_strata(run).items():
        if source != "pool_7580":
            continue
        base = excluded if stratum == "excluded_publication_type" else [r for r in rows if r["priority"] == stratum]
        cands = sorted(base, key=lambda r: rank_key(run.audit_seed, f"pool_7580:{stratum}", r["candidate_id"]))
        picked += [("random", stratum, r) for r in cands[:n]]
    have = {r["candidate_id"] for _, _, r in picked}
    by_doi = {ids.normalize_doi(r["doi"]): r for r in rows if r.get("doi")}
    for doi in AUDIT_NAMED_DOIS:
        r = by_doi.get(doi)
        if r and r["candidate_id"] not in have:
            picked.append(("purposive_named_in_research_audit", r["priority"], r))
    return picked


def pool_abstract_batches(root, run=RUN_1_0):
    """PMID batches (<= 50) for the pool audit-sample abstracts. Later runs first reuse the
    v1.0 batches, so their cache entries are not fetched twice, then batch the rest."""
    def chunks(pmids):
        return [pmids[i:i + 50] for i in range(0, len(pmids), 50)]

    def pmids(r):
        return {x["pmid"] for _, _, x in pool_sample(root, r) if x.get("pmid")}
    v1 = sorted(pmids(RUN_1_0))
    if run.legacy:
        return chunks(v1)
    batches, have = chunks(v1), set(v1)
    for prev in reversed(previous_runs(run)):  # oldest first, so every earlier batch keeps its cache key
        if prev.legacy:
            continue
        rest = sorted(pmids(prev) - have)
        batches, have = batches + chunks(rest), have | set(rest)
    return batches + chunks(sorted(pmids(run) - have))


def pool_abstracts(cache, sample, batches=None):
    """{SL-id: abstract} from cached Europe PMC EXT_ID lookups (by PMID)."""
    out = {}
    if batches is None:
        pmids = sorted({r["pmid"] for _, _, r in sample if r.get("pmid")})
        batches = [pmids[i:i + 50] for i in range(0, len(pmids), 50)]
    by_pmid = {}
    for batch in batches:
        env = cache.get("europepmc", "pool_abstracts:" + ",".join(batch))
        for res in ((((env or {}).get("body") or {}).get("resultList") or {}).get("result") or []):
            if res.get("abstractText") and res.get("pmid"):
                by_pmid[str(res["pmid"])] = clean_text(res["abstractText"])
    for _, _, r in sample:
        if r.get("pmid") in by_pmid:
            out[r["candidate_id"]] = by_pmid[r["pmid"]]
    return out


# --- commands ---------------------------------------------------------------------------------

def fetch_paged(client, cache, item, refresh=False):
    """Fetch every page of an 'all' request in order (each page needs the previous page's cursor).
    Returns (new pages fetched, error or None)."""
    lo, hi = item["bucket"].split("..")
    new = 0
    if refresh:  # drop this bucket's cached pages first, so that page n always follows page n-1's cursor
        _pages, _ = epmc_pages(cache, item)
        for key, _env in _pages:
            cache.path(item["service"], key).unlink(missing_ok=True)
    while True:
        _pages, need = epmc_pages(cache, item)
        if need is None:
            return new, None
        key, cursor = need
        url = epmc_search_request(item["query"], (lo, hi), page_size=EPMC_PAGE_MAX, cursor=cursor)
        env, did = get_json(client, cache, item["service"], key, url, trim_epmc_paged)
        if "fetch_error" in env:
            return new, env["fetch_error"]
        new += 1 if did else 0
        if not did:  # the page was cached after all; avoid a loop on a corrupt cache
            return new, None


def run_fetch(root, freeze, refresh=False, client=None, log=print, openalex_max=OPENALEX_MAX, run=RUN_1_0):
    cache = F.Cache(cache_dir(root, freeze, run))
    bases = [(prev, F.Cache(cache_dir(root, freeze, prev))) for prev in previous_runs(run)]
    client = client or F.Client(mailto=os.environ.get("SELA_RESEARCH_MAILTO"))
    errors, fetched, reused = [], collections.Counter(), collections.Counter()
    reused_by_run = collections.defaultdict(collections.Counter)

    def reuse(service, key):
        """Copy an identical earlier request (same service and key, so same URL) into this run's cache: from
        the newest earlier run that has it. The earlier caches are only read, never written."""
        if not bases or refresh or cache.get(service, key) is not None:
            return False
        for prev, base in bases:
            env = base.get(service, key)
            if env is not None:
                cache.put(service, key, env)
                reused[service] += 1
                reused_by_run[prev.run_version][service] += 1
                return True
        return False

    plan = search_plan(freeze, run)
    for n, item in enumerate(plan, start=1):
        if item.get("paged"):
            new, err = fetch_paged(client, cache, item, refresh=refresh)
            if err:
                errors.append({"key": item["key"], "error": err})
            fetched["europepmc"] += new
        else:
            reuse(item["service"], item["key"])
            trim = trim_epmc if item["source"] == "europepmc" else trim_crossref
            env, did = get_json(client, cache, item["service"], item["key"], item["url"], trim, refresh=refresh)
            if "fetch_error" in env:
                errors.append({"key": item["key"], "error": env["fetch_error"]})
            elif did:
                fetched[item["service"]] += 1
        if n % 20 == 0 or n == len(plan):
            log(f"[{n}/{len(plan)}] searches; new: {dict(fetched)}; reused from earlier runs: {dict(reused)}; "
                f"errors: {len(errors)}")
    # pool audit-sample abstracts (Europe PMC by PMID, batched)
    for batch in pool_abstract_batches(root, run):
        key = "pool_abstracts:" + ",".join(batch)
        reuse("europepmc", key)
        env, did = get_json(client, cache, "europepmc", key, epmc_ext_ids_request(batch), trim_epmc, refresh=refresh)
        if "fetch_error" in env:
            errors.append({"key": "pool_abstracts", "error": env["fetch_error"]})
        elif did:
            fetched["europepmc_pool_abstracts"] += 1
    # OpenAlex singleton lookups (free) for screened-in candidates: language and retraction signal
    cands, abstracts, _stats, missing, _plan = build_candidates(root, freeze, cache, run)
    lexicon = None if run.legacy else run.lexicon()
    screened = [dict(c, screen=screen_one(c["title"], abstracts.get(c["candidate_id"]), c["mesh_age_groups"],
                                          c["publication_type_eligibility"], lexicon=lexicon)) for c in cands]
    targets = [s["doi"] for s in sorted(screened, key=lambda s: (-s["screen"]["score"], s["candidate_id"]))
               if s["screen"]["priority"] in ("high", "medium") and s["doi"]][:openalex_max]
    if bases:  # also carry over every earlier OpenAlex lookup for a DOI of this run
        for c in cands:
            if c["doi"]:
                reuse("openalex", F.openalex_request(c["doi"])[0])
    budget_stop = None
    for doi in targets:
        key, url = F.openalex_request(doi)
        if cache.get("openalex", key) is not None and not refresh:
            continue
        status, raw, error = client.get("openalex", url, attempts=1)
        if status == 429:
            budget_stop = F.utc_now()
            log("OpenAlex answered 429 (keyless budget); remaining singleton lookups skipped")
            break
        if status == 200:
            body = json.loads(raw.decode("utf-8"))
            body = {k: body.get(k) for k in ("id", "doi", "language", "type", "is_retracted", "publication_date")}
            cache.put("openalex", key, F.envelope("openalex", key, url, F.utc_now(), 200, "ok", body=body))
            fetched["openalex"] += 1
        elif status == 404:
            cache.put("openalex", key, F.envelope("openalex", key, url, F.utc_now(), 404, "not_found"))
        else:
            errors.append({"key": key, "error": error or f"HTTP {status}"})
    report = {"finished_at": F.utc_now(), "freeze_date": freeze, "run_version": run.run_version,
              "new_responses": dict(sorted(fetched.items())), "http_requests": client.requests,
              "errors": errors, "openalex_targets": len(targets), "openalex_budget_stop": budget_stop,
              "search_requests_planned": len(plan), "crossref_pool": "polite" if client.mailto else "public"}
    if run is RUN_1_1:
        report["reused_from_v1_0_cache"] = dict(sorted(reused.items()))
    elif not run.legacy:
        report["reused_from_earlier_runs"] = {v: dict(sorted(c.items())) for v, c in sorted(reused_by_run.items())}
    if not run.legacy:
        prev = cache.root / report_name(run)
        if prev.exists():  # keep the first fetch's OpenAlex budget observation and reuse counts across re-runs
            old = json.loads(prev.read_text(encoding="utf-8"))
            report["openalex_budget_stop"] = report["openalex_budget_stop"] or old.get("openalex_budget_stop")
            reuse_key = "reused_from_v1_0_cache" if run is RUN_1_1 else "reused_from_earlier_runs"
            report["previous_runs"] = old.get("previous_runs", []) + [{k: old.get(k) for k in (
                "finished_at", "new_responses", "http_requests", reuse_key)}]
            if run is not RUN_1_1:  # a re-run finds everything cached: keep the first run's reuse counts
                first = next((p for p in report["previous_runs"] if p.get(reuse_key)), None)
                if first and not report[reuse_key]:
                    report[reuse_key] = first[reuse_key]
    cache.root.mkdir(parents=True, exist_ok=True)
    (cache.root / report_name(run)).write_text(json.dumps(report, indent=1, sort_keys=True) + "\n",
                                                encoding="utf-8")
    return 1 if errors else 0


def out_dir(root, freeze, out=None, run=None):
    return Path(out) if out else Path(root) / OUT_BASE / (run or RUN_1_0).dir_name(freeze)


def retrieval_table(stats):
    """Per-query hits, rows retrieved and share (audit: the search was 'bounded' but never said how much)."""
    agg = {}
    for s in stats:
        k = (s["family"], s["source"], s["query_id"])
        a = agg.setdefault(k, {"family": s["family"], "source": s["source"], "query_id": s["query_id"],
                               "requests": 0, "hit_count": 0, "retrieved": 0, "capped_requests": 0,
                               "cap": s.get("cap", EPMC_BUCKET_CAP if s["source"] == "europepmc" else CROSSREF_ROWS)})
        a["requests"] += 1
        a["hit_count"] += s["hit_count"] or 0
        a["retrieved"] += s["returned"]
        if a["cap"] == "all":  # every hit requested: a bucket counts when it still came back incomplete
            a["pages"] = a.get("pages", 0) + s.get("pages", 1)
            if not s.get("complete"):
                a["capped_requests"] += 1
        elif s["returned"] >= a["cap"]:
            a["capped_requests"] += 1
        if s.get("indonesian_title_cue_by_rank_band"):
            a["indonesian_title_cue_by_rank_band"] = s["indonesian_title_cue_by_rank_band"]
    rows = []
    for a in agg.values():
        a["share_retrieved"] = round(a["retrieved"] / a["hit_count"], 4) if a["hit_count"] else None
        if a["source"] == "crossref":
            a["note"] = ("Crossref total-results counts every work that matches any query word; the top rows by "
                         "relevance were kept, so the share is not a recall estimate")
        rows.append(a)
    return rows


def _run_keys(root, freeze, prev):
    """{doi:/pmid:/title: key -> V3 id} of an earlier run, when its candidates file is present."""
    p = Path(root) / OUT_BASE / prev.dir_name(freeze) / CANDIDATES_GZ
    if not p.exists():
        return None, None
    keys = {}
    for line in S.read_text(p).splitlines():
        r = json.loads(line)
        for k in (("doi:" + r["doi"]) if r.get("doi") else None, ("pmid:" + r["pmid"]) if r.get("pmid") else None,
                  "title:" + norm_title_key(r["title"], (r.get("first_publication_date") or "")[:4])):
            if k:
                keys.setdefault(k, r["candidate_id"])
    return keys, p


def _previous_run_keys(root, freeze, run):
    """[(earlier run, keys, candidates .gz path)] for every earlier run whose candidates file is present."""
    if run.legacy:
        return []
    out = []
    for prev in previous_runs(run):
        keys, path = _run_keys(root, freeze, prev)
        if keys is not None:
            out.append((prev, keys, path))
    return out


def _annotate_previous(cands, prevs):
    for c in cands:
        year = (c["first_publication_date"] or "")[:4]
        for prev, keys, _path in prevs:
            c[prev.in_run_field] = next((keys[k] for k in (
                ("doi:" + c["doi"]) if c["doi"] else None, ("pmid:" + c["pmid"]) if c["pmid"] else None,
                "title:" + norm_title_key(c["title"], year)) if k and k in keys), None)
    return cands


RUN_1_2_CHANGES = [
    "every hit, not the relevance top 50 per year bucket, for the D-048 gap queries listen_core (v1.1: 271 of "
    "663, 40.9%), journal_core (265 of 742, 35.7%), qpl_core (280 of 774, 36.2%), teen_consent (299 of 1,883, "
    "15.9%) and older_digital (1,181 of 5,994, 19.7%), and for continuity_core (9.5%), safety_v3 (10.3%) and "
    "help_v3 (10.6%): pageSize 1000 with Europe PMC cursorMark pagination until the last page",
    "a11y_core_v2 keeps its v1.1 request: it was already complete at 200 per bucket (665 of 665)",
    "the Indonesian-language Crossref queries (id_kesehatan_mental_aplikasi, id_konseling_daring, id_pdp, "
    "id_remaja_privasi_cr, id_lansia_cr, id_dewasa_pekerja_cr, ai_id_mh_cr) keep 1,000 rows (Crossref's largest "
    "page) instead of 40; per_query_retrieval records how many rows per rank band of 40 have an Indonesian title "
    "cue, so the depth at which the relevance ranking stops finding Indonesian records can be read off",
    "query strings, year buckets and every other request are unchanged from v1.1; identical requests reuse the "
    "v1.1 (or v1.0) response, copied into this run's cache with its original retrieval time",
    "candidates carry in_run_1_1 as well as in_run_1_0",
]


def _manifest_run_1_2(manifest, run, freeze, report, prevs):
    """Manifest text of run v1.2 (the v1.1 text is written by run_build and left unchanged for v1.1)."""
    manifest["supersedes"] = {
        "run_version": RUN_1_1.run_version, "run_dir": (OUT_BASE / RUN_1_1.dir_name(freeze)).as_posix(),
        "note": "runs v1.1 and v1.0 and their screenings stay frozen and rebuild byte for byte from their own "
                "caches (`v3_gap_search.py --run <version> build|screen`); this run never rewrites them"}
    manifest["changes_from_previous_run"] = RUN_1_2_CHANGES
    epmc_q = [q for s in run.families.values() for q, _ in s["europepmc"]]
    cr_q = [q for s in run.families.values() for q, _ in s["crossref"]]
    manifest["sources"]["europepmc"]["per_query_caps"] = {q: run.cap(q) for q in sorted(epmc_q)}
    manifest["sources"]["europepmc"]["page_size_when_all"] = EPMC_PAGE_MAX
    manifest["sources"]["europepmc"]["pagination"] = (
        "cap 'all': cursorMark from '*', following nextCursorMark until a page returns fewer than "
        f"{EPMC_PAGE_MAX} rows, the rows reach hitCount, or the cursor repeats")
    manifest["sources"]["crossref"]["rows_per_query"] = {q: run.crossref_cap(q) for q in sorted(cr_q)}
    manifest["sources"]["openalex"]["use"] = (
        "free singleton lookups for screened-in candidates (language, retraction signal), plus every earlier "
        "lookup (runs v1.1 and v1.0) for a DOI of this run; OpenAlex list/search queries were not used (the "
        "keyless daily budget was exhausted at 2026-10-26T16:49Z during run v1.0) and no API key was requested")
    manifest["sources"]["openalex"].pop("reused_from_v1_0_cache", None)
    reused = report.get("reused_from_earlier_runs") or {}
    manifest["sources"]["openalex"]["reused_from_earlier_runs"] = {v: c.get("openalex") for v, c in reused.items()
                                                                  if c.get("openalex")}
    manifest.pop("requests_reused_from_v1_0", None)
    manifest["requests_reused_from_earlier_runs"] = reused
    manifest["limitations"] = manifest["limitations"] + [
        "Europe PMC queries without cap 'all' still keep their v1.1 depth (per_query_retrieval shows hits, "
        "retrieved rows and share); candidates that were never retrieved are outside the screening and the "
        "false-negative audit, so their loss cannot be measured.",
        "Crossref relevance ranking is fuzzy at depth: the rank bands in per_query_retrieval show how many rows of "
        "each Indonesian query still look Indonesian; Garuda/SINTA remain a manual gap.",
    ]


def run_build(root, freeze, out=None, run=RUN_1_0):
    cache = F.Cache(cache_dir(root, freeze, run))
    cands, abstracts, stats, missing, plan = build_candidates(root, freeze, cache, run)
    if missing:
        raise SystemExit(f"build failed: {len(missing)} search responses missing from the cache (run fetch), "
                         f"e.g. {missing[:3]}")
    od = out_dir(root, freeze, out, run)
    prevs = _previous_run_keys(root, freeze, run)
    _annotate_previous(cands, prevs)
    data = jsonl_bytes(cands)
    gz = S.write(od / CANDIDATES_GZ, data)  # deterministic gzip; an unchanged file keeps its bytes
    if (od / CANDIDATES).exists():
        (od / CANDIDATES).unlink()  # the uncompressed file is never committed (storage sela-jsonl-gzip-v1)
    report = {}
    rp = cache.root / report_name(run)
    if rp.exists():
        report = json.loads(rp.read_text(encoding="utf-8"))
    counts = {
        "candidates": len(cands),
        "by_family_retrieved": dict(sorted(collections.Counter(f for c in cands for f in c["families_retrieved"]).items())),
        "source_records_by_source": dict(sorted(collections.Counter(
            s.split(":")[0] for c in cands for s in c["source_records"]).items())),
        "candidates_by_source": {
            "europepmc": sum(1 for c in cands if any(s.startswith("europepmc:") for s in c["source_records"])),
            "crossref": sum(1 for c in cands if any(s.startswith("crossref:") for s in c["source_records"])),
            "both": sum(1 for c in cands if any(s.startswith("europepmc:") for s in c["source_records"])
                        and any(s.startswith("crossref:") for s in c["source_records"]))},
        "only_crossref": sum(1 for c in cands if all(s.startswith("crossref:") for s in c["source_records"])),
        "publication_language": dict(sorted(collections.Counter(str(c["publication_language"]) for c in cands).items())),
        "publication_language_basis": dict(sorted(collections.Counter(
            c["publication_language_detail"]["basis"] for c in cands).items())),
        "within_window": dict(sorted(collections.Counter(str(c["within_window"]).lower() for c in cands).items())),
        "year": dict(sorted(collections.Counter((c["first_publication_date"] or "none")[:4] for c in cands).items())),
        "eligibility": dict(sorted(collections.Counter(c["publication_type_eligibility"]["status"] for c in cands).items())),
        "abstract_available": sum(1 for c in cands if c["abstract_available"]),
        "title_suggests_indonesian": sum(1 for c in cands if c["publication_language_detail"]["title_suggests_indonesian"]),
        "title_cue_disagrees_with_language_metadata": sum(
            1 for c in cands if c["publication_language_detail"]["title_cue_disagrees"]),
        "in_register": sorted({c["in_register"] for c in cands if c["in_register"]}),
        "in_pool_7580": sum(1 for c in cands if c["in_pool_7580"]),
        "openalex_enriched": sum(1 for c in cands if c["openalex"]),
    }
    for prev, _keys, _path in prevs:
        counts[prev.in_run_field] = sum(1 for c in cands if c.get(prev.in_run_field))
    manifest = {
        "dataset": "Sela v3 gap search: bibliographic candidates for the v3 evidence gaps (metadata only, unreviewed)",
        "run_version": RUN_VERSION,
        "freeze_date": freeze, "cutoff_date": D.cutoff_for(freeze),
        "window_rule": "cutoff_date <= first publication date <= freeze_date (inclusive; partial dates must fit)",
        "status": "candidate_metadata_only_unreviewed",
        "sources": {
            "europepmc": {"endpoint": F.EPMC_SEARCH, "resultType": "core", "filter": "(SRC:MED OR SRC:PMC) AND "
                          "FIRST_PDATE per year bucket", "per_bucket_cap": EPMC_BUCKET_CAP},
            "crossref": {"endpoint": "https://api.crossref.org/works", "filter": "from-pub-date/until-pub-date, "
                         "type:journal-article", "rows_per_query": CROSSREF_ROWS, "sort": "relevance",
                         "pool": report.get("crossref_pool", "public")},
            "openalex": {"endpoint": F.OPENALEX_WORK, "use": "free singleton lookups for screened-in candidates "
                         "(language, retraction signal); OpenAlex list/search queries were not used: the keyless "
                         "daily budget was exhausted (HTTP 429 observed at 2026-10-26T16:49Z) and no API key was requested",
                         "lookups_planned": report.get("openalex_targets"),
                         "budget_stop": report.get("openalex_budget_stop")},
        },
        "query_families": {f: {"purpose": s["purpose"], "europepmc": dict(s["europepmc"]), "crossref": dict(s["crossref"])}
                           for f, s in QUERY_FAMILIES.items()},
        "year_buckets": [f"{a}..{b}" for a, b in year_buckets(D.cutoff_for(freeze), freeze)],
        "requests": stats,
        "retrieval_window": {"first": min((s["retrieved_at"] for s in stats if s["retrieved_at"]), default=None),
                             "last": max((s["retrieved_at"] for s in stats if s["retrieved_at"]), default=None)},
        "counts": counts,
        "candidates_sha256": hashlib.sha256(data).hexdigest(),
        "candidates_storage": candidates_storage(gz),
        "inputs": {REGISTER.as_posix(): _sha(Path(root) / REGISTER), POOL.as_posix(): _sha(Path(root) / POOL)},
        "rights_policy": {
            "committed": "bibliographic metadata, identifiers, dates, publication language, licence strings, "
                         "abstract SHA-256 and availability; no abstract or full text",
            "local_cache_only": "search responses including abstracts (data/research/corpus/.cache/v3_search/, "
                                "gitignored); used only for machine screening",
            "training_use": "none",
        },
        "limitations": [
            "Searches are bounded (per-bucket and per-query caps) and ranked by each API; this is not an exhaustive "
            "or systematic search, and hit counts are not evidence counts.",
            "Crossref keyword search is fuzzy and noisy; Crossref-only records have no MeSH age groups.",
            "Publication language comes from Europe PMC (indexer) or Crossref (publisher deposit); OpenAlex language "
            "is machine-inferred and flagged. Some Indonesian-titled Crossref records are deposited as 'en' or carry "
            "no language: title_suggests_indonesian / title_cue_disagrees flag them; the value is not changed.",
            "No Garuda/SINTA harvest: those portals have no documented free API; listed as a manual gap.",
            "Nothing here is screened by a person; status is candidate_metadata_only_unreviewed.",
        ],
    }
    if not run.legacy:
        manifest["run_version"] = run.run_version
        manifest["run_dir"] = (OUT_BASE / run.dir_name(freeze)).as_posix()
        manifest["supersedes"] = {
            "run_version": RUN_1_0.run_version, "run_dir": (OUT_BASE / RUN_1_0.dir_name(freeze)).as_posix(),
            "note": "run v1.0 and its screening v1.0 stay frozen and rebuild byte for byte from their own cache "
                    "(`v3_gap_search.py --run sela-v3-gap-search-v1.0 build|screen`); this run never rewrites them"}
        manifest["changes_from_previous_run"] = [
            "id_digital (no mental-health term; 300 of 15,653 hits retrieved, 1.9%, mostly general telemedicine and "
            "AI) is replaced by id_digital_mh (Indonesia, a mental-health term and a digital term, all in "
            "title/abstract) plus id_health_data_law (Indonesia and health-data privacy, for UU PDP)",
            "a11y_core (300 of 7,042, 4.3%; 'accessible' alone matched access to services) is replaced by the "
            "narrowed a11y_core_v2",
            "new family working_age_adults (dewasa: working-age and mid-life adults), the brief topic without a "
            "family in v1.0",
            "per-bucket cap raised from 50 to 200 for the queries retrieved at 5% or less in v1.0 (checkin_core, "
            "teen_digital, older_digital) and for the new or narrowed queries",
            "Indonesian Crossref queries added for adolescents' confidentiality and for older adults (lansia)",
            "requests identical to v1.0 (same query, bucket and cap) reuse the v1.0 response, copied into this "
            "run's cache with its original retrieval time",
        ]
        manifest["sources"]["europepmc"]["per_query_caps"] = dict(sorted(run.caps.items()))
        manifest["sources"]["openalex"]["use"] = (
            "free singleton lookups for screened-in candidates (language, retraction signal), plus every v1.0 "
            "lookup for a DOI of this run; OpenAlex list/search queries were not used (the keyless daily budget "
            "was exhausted at 2026-10-26T16:49Z during run v1.0) and no API key was requested")
        manifest["sources"]["openalex"]["reused_from_v1_0_cache"] = (report.get("reused_from_v1_0_cache") or {}).get(
            "openalex")
        manifest["query_families"] = {f: {"purpose": s["purpose"], "europepmc": dict(s["europepmc"]),
                                          "crossref": dict(s["crossref"]),
                                          "europepmc_per_bucket_cap": {q: run.cap(q) for q, _ in s["europepmc"]}}
                                      for f, s in run.families.items()}
        manifest["per_query_retrieval"] = retrieval_table(stats)
        manifest["requests_reused_from_v1_0"] = (report.get("reused_from_v1_0_cache") or {})
        for _prev, _keys, prev_path in prevs:  # recorded under the logical name, hash of the content
            manifest["inputs"][S.logical_path(prev_path).relative_to(root).as_posix()] = S.content_sha256(prev_path)
        if run is not RUN_1_1:
            _manifest_run_1_2(manifest, run, freeze, report, prevs)
    atomic_write(od / "manifest.json", json_bytes(manifest))
    return manifest


def run_screen(root, freeze, out=None, run=RUN_1_0):
    if run.legacy:
        return _run_screen_v1_0(root, freeze, out)
    return _run_screen_v1_1(root, freeze, out, run)


def _run_screen_v1_0(root, freeze, out=None):
    cache = F.Cache(cache_dir(root, freeze))
    od = out_dir(root, freeze, out)
    cands = [json.loads(x) for x in S.read_text(od / CANDIDATES_GZ).splitlines()]
    _c, abstracts, _s, missing, _p = build_candidates(root, freeze, cache)
    if missing:
        raise SystemExit("screen failed: search cache incomplete (run fetch)")
    built = {c["candidate_id"]: c for c in _c}
    for c in cands:  # the committed file must be what the cache builds
        if built.get(c["candidate_id"]) != c:
            raise SystemExit(f"screen failed: candidates.jsonl is stale for {c['candidate_id']} (run build)")
    screened = []
    for c in cands:
        s = screen_one(c["title"], abstracts.get(c["candidate_id"]), c["mesh_age_groups"],
                       c["publication_type_eligibility"], within_window=c["within_window"])
        s["population_band"] = population_band(s)
        if c["within_window"] is None:
            s["date_window_unclear"] = True
        s["ruleset"] = SCREEN_VERSION
        screened.append(dict(c, screen=s))
    sd = od / "screening"
    index_rows = [{
        "candidate_id": s["candidate_id"], "doi": s["doi"] or "", "pmid": s["pmid"] or "",
        "first_publication_date": s["first_publication_date"] or "", "publication_language": s["publication_language"] or "",
        "title": s["title"], "priority": s["screen"]["priority"], "score": s["screen"]["score"],
        "feature_families": ";".join(s["screen"]["feature_families"]),
        "population_band": s["screen"]["population_band"],
        "possible_null_or_negative_finding": ";".join(s["screen"]["possible_null_or_negative_finding"]),
        "abstract_used": s["screen"]["abstract_used"], "eligibility": s["publication_type_eligibility"]["status"],
        "within_window": "" if s["within_window"] is None else str(s["within_window"]).lower(),
        "in_register": s["in_register"] or "", "in_pool_7580": s["in_pool_7580"] or "",
        "ruleset": SCREEN_VERSION, "review_status": "machine_title_abstract_screen_unreviewed",
    } for s in screened]
    write_csv(sd / "index.csv", list(index_rows[0]) if index_rows else ["candidate_id"], index_rows)
    queue = select_queue(screened)
    atomic_write(sd / "queue.jsonl", jsonl_bytes([{
        "candidate_id": s["candidate_id"], "title": s["title"], "doi": s["doi"], "pmid": s["pmid"],
        "journal": s["journal"], "first_publication_date": s["first_publication_date"],
        "publication_language": s["publication_language"], "publication_types": s["publication_types"],
        "in_register": s["in_register"], "in_pool_7580": s["in_pool_7580"], "screen": s["screen"],
    } for s in queue]))
    ws_fields = ["candidate_id", "doi", "title", "machine_priority", "feature_families", "population_band",
                 "reviewer_1", "reviewer_1_date", "reviewer_1_decision", "reviewer_2", "reviewer_2_date",
                 "reviewer_2_decision", "disagreement_resolved_by", "final_decision", "population_verified",
                 "design_verified", "full_text_appraised", "license_reuse_verified", "rag_ready", "notes"]
    write_csv(sd / "worksheet.csv", ws_fields, [{
        "candidate_id": s["candidate_id"], "doi": s["doi"] or "", "title": s["title"],
        "machine_priority": s["screen"]["priority"], "feature_families": ";".join(s["screen"]["feature_families"]),
        "population_band": s["screen"]["population_band"],
        "reviewer_1": "", "reviewer_1_date": "", "reviewer_1_decision": "NOT_REVIEWED", "reviewer_2": "",
        "reviewer_2_date": "", "reviewer_2_decision": "NOT_REVIEWED", "disagreement_resolved_by": "",
        "final_decision": "NOT_REVIEWED", "population_verified": "NO", "design_verified": "NO",
        "full_text_appraised": "NO", "license_reuse_verified": "NO", "rag_ready": "NO", "notes": "",
    } for s in queue])
    # false-negative (and false-positive) audit sample: criterion 5 of doc 14
    audit = []
    by_pri = collections.defaultdict(list)
    for s in screened:
        if s["publication_type_eligibility"]["status"] == "eligible":
            by_pri[s["screen"]["priority"]].append(s)
    for (source, stratum), n in AUDIT_STRATA.items():
        if source != "v3_run":
            continue
        rows = sorted(by_pri[stratum], key=lambda s: rank_key(AUDIT_SEED, f"v3_run:{stratum}", s["candidate_id"]))
        for s in rows[:n]:
            audit.append({"source": "v3_run", "selection": "random", "stratum": stratum, "candidate_id": s["candidate_id"],
                          "doi": s["doi"] or "", "title": s["title"], "machine_priority": stratum,
                          "v3_rescreen_priority": s["screen"]["priority"],
                          "v3_feature_families": ";".join(s["screen"]["feature_families"]),
                          "abstract_used": s["screen"]["abstract_used"],
                          "machine_flag": ""})
    sample = pool_sample(root)
    pabs = pool_abstracts(cache, sample)
    for selection, stratum, r in sample:
        s = screen_one(r["title"], pabs.get(r["candidate_id"]), None, {"status": "eligible"})
        flag = ("possible_false_negative" if stratum == "deferred" and s["priority"] in ("high", "medium") else
                "possible_false_positive_for_v3" if stratum in ("high", "medium") and s["priority"] == "deferred"
                else "")
        audit.append({"source": "pool_7580", "selection": selection, "stratum": stratum,
                      "candidate_id": r["candidate_id"], "doi": r["doi"] or "", "title": r["title"],
                      "machine_priority": f"{r['priority']} (sela-title-screen-v1.1)",
                      "v3_rescreen_priority": s["priority"], "v3_feature_families": ";".join(s["feature_families"]),
                      "abstract_used": s["abstract_used"], "machine_flag": flag})
    audit_fields = ["sample_id", "source", "selection", "stratum", "candidate_id", "doi", "title", "machine_priority",
                    "v3_rescreen_priority", "v3_feature_families", "abstract_used", "machine_flag",
                    "auditor", "audit_date", "auditor_relevance", "auditor_notes"]
    for n, a in enumerate(audit, start=1):
        a.update({"sample_id": f"FNA-{n:03d}", "auditor": "", "audit_date": "", "auditor_relevance": "NOT_AUDITED",
                  "auditor_notes": ""})
    write_csv(sd / "fn_audit_sample.csv", audit_fields, audit)
    pool_def = [a for a in audit if a["source"] == "pool_7580" and a["stratum"] == "deferred" and a["selection"] == "random"]
    flagged = [a for a in pool_def if a["machine_flag"] == "possible_false_negative"]
    pc = collections.Counter(s["screen"]["priority"] for s in screened)
    fam_counts = collections.Counter(f for s in screened if s["screen"]["priority"] in ("high", "medium")
                                     for f in s["screen"]["feature_families"])
    manifest = {
        "screening_version": SCREEN_VERSION, "lexicon_version": LX.LEXICON_VERSION,
        "eligibility_rule_version": ELIG.RULE_VERSION,
        "screening_unit": "title + abstract (abstract read from the local cache only; never committed)",
        "status": "machine_title_abstract_screen_unreviewed",
        "is_appraisal": False, "llm_used": False, "human_screening_done": False,
        "candidates": len(screened), "priority_counts": dict(sorted(pc.items())),
        "screened_in_by_feature_family": dict(sorted(fam_counts.items())),
        "population_band_counts_screened_in": dict(sorted(collections.Counter(
            s["screen"]["population_band"] for s in screened if s["screen"]["priority"] in ("high", "medium")).items())),
        "possible_null_or_negative_screened_in": sum(1 for s in screened if s["screen"]["priority"] in ("high", "medium")
                                                     and s["screen"]["possible_null_or_negative_finding"]),
        "queue_count": len(queue), "queue_rule": "per feature family up to 30 (title hit for that family first, then "
                                                 "high before medium, score desc), then the remaining best rows up "
                                                 "to 400",
        "queue_family_counts": dict(sorted(collections.Counter(f for s in queue for f in s["screen"]["feature_families"]).items())),
        "audit_sample": {
            "criterion": "doc 14 acceptance criterion 5 (manual false-positive / false-negative audit)",
            "design": {f"{a}:{b}": n for (a, b), n in AUDIT_STRATA.items()},
            "selection": f"first n rows by sha256('{AUDIT_SEED}|<source>:<stratum>|<candidate_id>'); stable across "
                         "Python versions; plus deferred pool titles named in the research audit (purposive)",
            "rows": len(audit),
            "machine_pre_audit_pool_deferred": {"sampled": len(pool_def), "v3_rescreen_high_or_medium": len(flagged),
                                                "note": "a machine re-screen with the v3 lexicon, NOT the manual "
                                                        "audit; the auditor columns are blank"},
            "status": "prepared_not_audited",
        },
        "outputs": {name: _sha(sd / name) for name in ("index.csv", "queue.jsonl", "worksheet.csv", "fn_audit_sample.csv")},
        "inputs": {CANDIDATES: S.content_sha256(od / CANDIDATES_GZ), POOL_INDEX.as_posix(): _sha(Path(root) / POOL_INDEX)},
        "limitations": [
            "Lexical families miss paraphrases and over-match generic words (for example 'trust', 'stress'); every "
            "label is a prompt for a human screener, not a relevance or quality judgement.",
            "Population bands come from words, MeSH groups and explicit ages in titles/abstracts; adult and "
            "adolescent signals are kept apart and are unverified.",
            "possible_null_or_negative_finding is a cue list for finding counter-evidence, not a result.",
            "The worksheet supports dual screening (two reviewers, disagreement resolution); all decisions are blank.",
        ],
    }
    atomic_write(sd / "manifest.json", json_bytes(manifest))
    return manifest


ID_AGE_TITLE_TERMS = re.compile(r"\b(?:remaja|anak(?:-anak)?|siswa|pelajar|lansia|lanjut\s+usia)\b", re.I)
AUDIT_SHEET_FIELDS = ["audit_id", "title", "doi", "auditor", "audit_date", "auditor_relevance",
                      "auditor_publication_kind", "auditor_notes"]
AUDIT_SHEET_MACHINE_FIELDS = ["audit_id", "title", "doi"]
AUDIT_KEY_FIELDS = ["audit_id", "source", "selection", "stratum", "candidate_id", "machine_priority",
                    "v3_priority_if_eligible", "v3_feature_families", "abstract_used", "eligibility",
                    "eligibility_basis", "machine_flag"]
AUDIT_RELEVANCE = ("NOT_AUDITED", "relevant", "not_relevant", "unsure")
AUDIT_KINDS = ("NOT_AUDITED", "study_report", "protocol", "correction_or_erratum", "editorial",
               "retraction_notice_or_retracted", "other")


def _with_previous_run(root, freeze, cands, run):
    """Annotate built candidates exactly as run_build does (in_run_1_0, ...), so screen can compare them."""
    return _annotate_previous(cands, _previous_run_keys(root, freeze, run))


def private_dir(root, freeze, run):
    """Local, gitignored location of files withheld from the repository (the audit key of runs v1.1+)."""
    return cache_dir(root, freeze, run) / "private"


def csv_bytes(fields, rows):
    import io
    buf = io.StringIO()
    w = csv.DictWriter(buf, fieldnames=fields, lineterminator="\n")
    w.writeheader()
    for r in rows:
        w.writerow(r)
    return buf.getvalue().encode("utf-8")


def sheet_machine_columns_sha256(rows):
    """SHA-256 of the audit sheet's machine columns only (audit_id, title, doi): the auditor fills the other
    columns, which must not break verify."""
    return hashlib.sha256(csv_bytes(AUDIT_SHEET_MACHINE_FIELDS,
                                    [{k: r.get(k, "") for k in AUDIT_SHEET_MACHINE_FIELDS} for r in rows])).hexdigest()


def _eligibility_basis(e):
    return ";".join(f"{b['kind']} ({b['source']}: {b['term']})" for b in e.get("basis") or [])


def _run_screen_v1_1(root, freeze, out, run):
    """Screening v1.1: lexicon sela-v3-families-2, population band v2, and a BLINDED false-negative
    audit (sheet with id/title/DOI only; machine labels in a separate key joined after the audit)."""
    lexicon = run.lexicon()
    cache = F.Cache(cache_dir(root, freeze, run))
    od = out_dir(root, freeze, out, run)
    cands = [json.loads(x) for x in S.read_text(od / CANDIDATES_GZ).splitlines()]
    _c, abstracts, _s, missing, _p = build_candidates(root, freeze, cache, run)
    if missing:
        raise SystemExit("screen failed: search cache incomplete (run fetch)")
    built = {c["candidate_id"]: c for c in _with_previous_run(root, freeze, _c, run)}
    for c in cands:  # the committed file must be what the cache builds
        if built.get(c["candidate_id"]) != c:
            raise SystemExit(f"screen failed: candidates.jsonl is stale for {c['candidate_id']} (run build)")
    screened = []
    for c in cands:
        s = screen_one(c["title"], abstracts.get(c["candidate_id"]), c["mesh_age_groups"],
                       c["publication_type_eligibility"], within_window=c["within_window"], lexicon=lexicon)
        s["population_band"] = population_band_v2(s)
        if c["within_window"] is None:
            s["date_window_unclear"] = True
        s["ruleset"] = run.screen_version
        screened.append(dict(c, screen=s))
    sd = od / "screening"
    prev_fields = [p.in_run_field for p in reversed(previous_runs(run))]  # oldest first: in_run_1_0, in_run_1_1
    index_rows = [{
        "candidate_id": s["candidate_id"], "doi": s["doi"] or "", "pmid": s["pmid"] or "",
        "first_publication_date": s["first_publication_date"] or "", "publication_language": s["publication_language"] or "",
        "title": s["title"], "priority": s["screen"]["priority"], "score": s["screen"]["score"],
        "feature_families": ";".join(s["screen"]["feature_families"]),
        "population_band": s["screen"]["population_band"],
        "possible_null_or_negative_finding": ";".join(s["screen"]["possible_null_or_negative_finding"]),
        "abstract_used": s["screen"]["abstract_used"], "eligibility": s["publication_type_eligibility"]["status"],
        "within_window": "" if s["within_window"] is None else str(s["within_window"]).lower(),
        "in_register": s["in_register"] or "", "in_pool_7580": s["in_pool_7580"] or "",
        **{f: s.get(f) or "" for f in prev_fields},
        "ruleset": run.screen_version, "review_status": "machine_title_abstract_screen_unreviewed",
    } for s in screened]
    write_csv(sd / "index.csv", list(index_rows[0]) if index_rows else ["candidate_id"], index_rows)
    queue = select_queue(screened, families=lexicon.FEATURE_FAMILIES)
    atomic_write(sd / "queue.jsonl", jsonl_bytes([{
        "candidate_id": s["candidate_id"], "title": s["title"], "doi": s["doi"], "pmid": s["pmid"],
        "journal": s["journal"], "first_publication_date": s["first_publication_date"],
        "publication_language": s["publication_language"], "publication_types": s["publication_types"],
        "in_register": s["in_register"], "in_pool_7580": s["in_pool_7580"], **{f: s.get(f) for f in prev_fields},
        "screen": s["screen"],
    } for s in queue]))
    ws_fields = ["candidate_id", "doi", "title", "machine_priority", "feature_families", "population_band",
                 "reviewer_1", "reviewer_1_date", "reviewer_1_decision", "reviewer_2", "reviewer_2_date",
                 "reviewer_2_decision", "disagreement_resolved_by", "final_decision", "population_verified",
                 "design_verified", "full_text_appraised", "license_reuse_verified", "rag_ready", "notes"]
    write_csv(sd / "worksheet.csv", ws_fields, [{
        "candidate_id": s["candidate_id"], "doi": s["doi"] or "", "title": s["title"],
        "machine_priority": s["screen"]["priority"], "feature_families": ";".join(s["screen"]["feature_families"]),
        "population_band": s["screen"]["population_band"],
        "reviewer_1": "", "reviewer_1_date": "", "reviewer_1_decision": "NOT_REVIEWED", "reviewer_2": "",
        "reviewer_2_date": "", "reviewer_2_decision": "NOT_REVIEWED", "disagreement_resolved_by": "",
        "final_decision": "NOT_REVIEWED", "population_verified": "NO", "design_verified": "NO",
        "full_text_appraised": "NO", "license_reuse_verified": "NO", "rag_ready": "NO", "notes": "",
    } for s in queue])
    # --- blinded false-negative / false-positive / eligibility audit (doc 14 criterion 5) ---
    strata = audit_strata(run)
    picks = []
    by_pri = collections.defaultdict(list)
    for s in screened:
        if s["publication_type_eligibility"]["status"] == "eligible":
            by_pri[s["screen"]["priority"]].append(s)
        else:
            by_pri["excluded_publication_type"].append(s)
    for (source, stratum), n in strata.items():
        if source != "v3_run":
            continue
        rows = sorted(by_pri[stratum], key=lambda s: rank_key(run.audit_seed, f"v3_run:{stratum}", s["candidate_id"]))
        for s in rows[:n]:
            if_eligible = s["screen"] if stratum != "excluded_publication_type" else screen_one(
                s["title"], abstracts.get(s["candidate_id"]), s["mesh_age_groups"], {"status": "eligible"},
                within_window=s["within_window"], lexicon=lexicon)
            picks.append({"source": "v3_run", "selection": "random", "stratum": stratum,
                          "candidate_id": s["candidate_id"], "doi": s["doi"] or "", "title": s["title"],
                          "machine_priority": s["screen"]["priority"],
                          "v3_priority_if_eligible": if_eligible["priority"],
                          "v3_feature_families": ";".join(if_eligible["feature_families"]),
                          "abstract_used": s["screen"]["abstract_used"],
                          "eligibility": s["publication_type_eligibility"]["status"],
                          "eligibility_basis": _eligibility_basis(s["publication_type_eligibility"])})
    sample = pool_sample(root, run)
    pabs = pool_abstracts(cache, sample, pool_abstract_batches(root, run))
    for selection, stratum, r in sample:
        sc = screen_one(r["title"], pabs.get(r["candidate_id"]), None, {"status": "eligible"}, lexicon=lexicon)
        picks.append({"source": "pool_7580", "selection": selection, "stratum": stratum,
                      "candidate_id": r["candidate_id"], "doi": r["doi"] or "", "title": r["title"],
                      "machine_priority": f"{r['priority']} (sela-title-screen-v1.1)",
                      "v3_priority_if_eligible": sc["priority"], "v3_feature_families": ";".join(sc["feature_families"]),
                      "abstract_used": sc["abstract_used"], "eligibility": r.get("eligibility") or "eligible",
                      "eligibility_basis": r.get("eligibility_basis") or ""})
    for a in picks:
        relevant = a["v3_priority_if_eligible"] in ("high", "medium")
        if a["stratum"] == "excluded_publication_type":
            a["machine_flag"] = ("excluded_by_title_cue_only" if "(title:" in a["eligibility_basis"]
                                 and "publication_type" not in a["eligibility_basis"] else "excluded_by_publication_type")
        elif a["stratum"] == "deferred" and relevant:
            a["machine_flag"] = "possible_false_negative"
        elif a["stratum"] in ("high", "medium") and a["v3_priority_if_eligible"] == "deferred":
            a["machine_flag"] = "possible_false_positive_for_v3"
        else:
            a["machine_flag"] = ""
    # blinding: one stable shuffle across all strata and sources; ids carry no machine information
    picks.sort(key=lambda a: rank_key(run.audit_seed, "blinded-order", f"{a['source']}:{a['candidate_id']}"))
    for n, a in enumerate(picks, start=1):
        a["audit_id"] = f"AUD-{n:03d}"
    sheet_rows = [{"audit_id": a["audit_id"], "title": a["title"], "doi": a["doi"], "auditor": "", "audit_date": "",
                   "auditor_relevance": "NOT_AUDITED", "auditor_publication_kind": "NOT_AUDITED", "auditor_notes": ""}
                  for a in picks]
    sheet = sd / "fn_audit_sheet.csv"
    if run.audit_design == "v3" and sheet.exists() and _read_csv(sheet)[1] and any(_audited(r) for r in _read_csv(sheet)[1]):
        # never overwrite an auditor's entries: keep the sheet if its machine columns are unchanged
        if sheet_machine_columns_sha256(_read_csv(sheet)[1]) != sheet_machine_columns_sha256(sheet_rows):
            raise SystemExit("screen failed: fn_audit_sheet.csv holds auditor entries for other rows; move it away "
                             "before regenerating the audit sample")
    else:
        write_csv(sheet, AUDIT_SHEET_FIELDS, sheet_rows)
    # The key (stratum, machine priority, flags) is held outside the repository until the audit is complete
    # (review round 3): an auditor with repository access must not be able to open it. It goes to the local,
    # gitignored private directory; release-audit-key copies it into screening/ after the audit.
    key_path = private_dir(root, freeze, run) / "fn_audit_key.csv"
    write_csv(key_path, AUDIT_KEY_FIELDS, [{k: a[k] for k in AUDIT_KEY_FIELDS} for a in picks])
    committed_key = sd / "fn_audit_key.csv"
    if committed_key.exists() and not (_sha(committed_key) == _sha(key_path) and
                                       all(_audited(r) for r in _read_csv(sheet)[1])):
        committed_key.unlink()  # a key next to an unaudited sheet would unblind it
    pool_def = [a for a in picks if a["source"] == "pool_7580" and a["stratum"] == "deferred"
                and a["selection"] == "random"]
    flagged = [a for a in pool_def if a["machine_flag"] == "possible_false_negative"]
    pc = collections.Counter(s["screen"]["priority"] for s in screened)
    fam_counts = collections.Counter(f for s in screened if s["screen"]["priority"] in ("high", "medium")
                                     for f in s["screen"]["feature_families"])
    id_age = [s for s in screened if ID_AGE_TITLE_TERMS.search(s["title"] or "")]
    manifest = {
        "screening_version": run.screen_version, "lexicon_version": lexicon.version,
        "run_version": run.run_version, "eligibility_rule_version": ELIG.RULE_VERSION,
        "screening_unit": "title + abstract (abstract read from the local cache only; never committed)",
        "status": "machine_title_abstract_screen_unreviewed",
        "is_appraisal": False, "llm_used": False, "human_screening_done": False,
        "supersedes": {"screening_version": SCREEN_VERSION, "lexicon_version": LX.LEXICON_VERSION,
                       "note": "screening v1.0 of run v1.0 stays frozen and reproducible; v1.1 only adds entries"},
        "changes_from_previous_screening": [
            "lexicon sela-v3-families-2: Indonesian age terms (remaja, anak, siswa, pelajar, lansia, lanjut usia, "
            "dewasa) as population signals; Indonesian feature and consent terms (menulis ekspresif, jurnal harian, "
            "privasi, kerahasiaan, persetujuan orang tua, bunuh diri, mencari bantuan, disabilitas, ...); a "
            "working_age_adults family; every families-1 entry unchanged",
            "Indonesian age phrases ('berusia 15-17 tahun', 'usia 60 tahun ke atas') are read as ages",
            "population band v2: includes_15_17_explicit only when the ages agree on one range or minimum that "
            "overlaps 15-17 and the record is not a review; ages that describe an advisory or working group do "
            "not count",
            "the false-negative audit is blinded (fn_audit_sheet.csv shows only audit id, title and DOI; "
            "fn_audit_key.csv holds the machine labels and is joined after the audit) and adds an "
            "excluded_publication_type stratum to check the eligibility rule",
        ],
        "candidates": len(screened), "priority_counts": dict(sorted(pc.items())),
        "screened_in_by_feature_family": dict(sorted(fam_counts.items())),
        "population_band_counts_screened_in": dict(sorted(collections.Counter(
            s["screen"]["population_band"] for s in screened if s["screen"]["priority"] in ("high", "medium")).items())),
        "population_band_rule": POPULATION_BAND_V2_RULE,
        "indonesian_age_term_titles": {
            "titles": len(id_age),
            "population_band": dict(sorted(collections.Counter(s["screen"]["population_band"] for s in id_age).items())),
            "screened_in_population_band_not_stated": sum(
                1 for s in id_age if s["screen"]["priority"] in ("high", "medium")
                and s["screen"]["population_band"] == "not_stated"),
            "note": "titles containing remaja, anak, siswa, pelajar, lansia or lanjut usia; a check that "
                    "Indonesian age words now give a population signal"},
        "possible_null_or_negative_screened_in": sum(1 for s in screened if s["screen"]["priority"] in ("high", "medium")
                                                     and s["screen"]["possible_null_or_negative_finding"]),
        "queue_count": len(queue), "queue_rule": "per feature family up to 30 (title hit for that family first, then "
                                                 "high before medium, score desc), then the remaining best rows up "
                                                 "to 400",
        "queue_family_counts": dict(sorted(collections.Counter(f for s in queue for f in s["screen"]["feature_families"]).items())),
        "audit_sample": {
            "criterion": "doc 14 acceptance criterion 5 (manual false-positive / false-negative audit), plus a check "
                         "of the publication-type eligibility rule",
            "design": {f"{a}:{b}": n for (a, b), n in strata.items()},
            "selection": f"first n rows by sha256('{AUDIT_SEED}|<source>:<stratum>|<candidate_id>'); stable across "
                         "Python versions; plus deferred pool titles named in the research audit (purposive)",
            "blinding": ("the auditor receives only fn_audit_sheet.csv: audit id, title and DOI, rows in one stable "
                         f"shuffle (sha256('{AUDIT_SEED}|blinded-order|<source>:<candidate_id>')) across all strata, "
                         "so neither the stratum nor the machine priority can be inferred from the order; "
                         "fn_audit_key.csv (stratum, machine priority, v3 priority if eligible, eligibility basis) "
                         "is joined on audit_id only after the audit"),
            "auditor_columns": {"auditor_relevance": list(AUDIT_RELEVANCE),
                                "auditor_publication_kind": list(AUDIT_KINDS)},
            "rows": len(picks),
            "machine_pre_audit_pool_deferred": {"sampled": len(pool_def), "v3_rescreen_high_or_medium": len(flagged),
                                                "note": "a machine re-screen with lexicon sela-v3-families-2, NOT the "
                                                        "manual audit; the auditor columns are blank"},
            "excluded_publication_type_rows": {
                "sampled": sum(1 for a in picks if a["stratum"] == "excluded_publication_type"),
                "excluded_by_title_cue_only": sum(1 for a in picks if a["machine_flag"] == "excluded_by_title_cue_only")},
            "status": "prepared_not_audited",
        },
        "outputs": {name: _sha(key_path if name == "fn_audit_key.csv" else sd / name)
                    for name in ("index.csv", "queue.jsonl", "worksheet.csv", "fn_audit_sheet.csv", "fn_audit_key.csv")},
        "inputs": {CANDIDATES: S.content_sha256(od / CANDIDATES_GZ), POOL_INDEX.as_posix(): _sha(Path(root) / POOL_INDEX)},
        "limitations": [
            "Lexical families miss paraphrases and over-match generic words (for example 'trust', 'stress', 'anak'); "
            "every label is a prompt for a human screener, not a relevance or quality judgement.",
            "Population bands come from words, MeSH groups and explicit ages in titles/abstracts; adult and "
            "adolescent signals are kept apart and are unverified.",
            "possible_null_or_negative_finding is a cue list for finding counter-evidence, not a result.",
            "The worksheet supports dual screening (two reviewers, disagreement resolution); all decisions are blank.",
            "Blinding is procedural: the key file sits next to the sheet and must not be given to the auditor.",
        ],
    }
    if run.audit_design == "v3":
        _screen_manifest_v3(manifest, root, freeze, run, sd, key_path, sheet_rows, screened)
    atomic_write(sd / "manifest.json", json_bytes(manifest))
    return manifest


def _read_csv(path):
    with Path(path).open(encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f)
        rows = list(reader)
        return reader.fieldnames, rows


def _audited(row):
    return row.get("auditor_relevance", "NOT_AUDITED") != "NOT_AUDITED" or \
        row.get("auditor_publication_kind", "NOT_AUDITED") != "NOT_AUDITED"


def _fully_audited(rows):
    return bool(rows) and all(r.get("auditor") and r.get("audit_date") and r.get("auditor_relevance") != "NOT_AUDITED"
                              and r.get("auditor_publication_kind") != "NOT_AUDITED" for r in rows)


def crossref_depth_yield(screened, run, width=200):
    """Per raised Crossref query: rows, Indonesian title cues and machine screened-in rows per rank band, so the
    yield of the deeper rows can be judged (a reading aid, not a recall estimate)."""
    out = {}
    for q in sorted(qid for qid, rows in run.crossref_rows.items() if rows > CROSSREF_ROWS):
        bands = collections.defaultdict(lambda: {"rows": 0, "indonesian_title_cue": 0, "screened_in": 0})
        for s in screened:
            for h in s["retrieval"]:
                if h["source"] == "crossref" and h["query_id"] == q:
                    lo = (h["rank"] - 1) // width * width + 1
                    b = bands[lo]
                    b["rows"] += 1
                    b["indonesian_title_cue"] += 1 if indonesian_title_cue(s["title"]) else 0
                    b["screened_in"] += 1 if s["screen"]["priority"] in ("high", "medium") else 0
        out[q] = [dict({"ranks": f"{lo}-{lo + width - 1}"}, **bands[lo]) for lo in sorted(bands)]
    return out


def _screen_manifest_v3(manifest, root, freeze, run, sd, key_path, sheet_rows, screened):
    """Screening v1.2 text: the key is withheld (hash only), the sheet is tracked by its machine columns."""
    prev = previous_runs(run)[0]
    manifest["supersedes"] = {"screening_version": prev.screen_version, "lexicon_version": prev.lexicon_version,
                              "note": f"screening {prev.screen_version} of run {prev.run_version} stays frozen and "
                                      "reproducible; this screening uses the same lexicon and rules"}
    manifest["changes_from_previous_screening"] = [
        "same lexicon (sela-v3-families-2), population band v2, priority rule and queue rule as screening v1.1, "
        "applied to the larger candidate set of run v1.2",
        "the audit key (fn_audit_key.csv) is withheld from the repository until the audit is complete: only its "
        "SHA-256 is recorded here (withheld_outputs); the file is written to the local, gitignored private "
        "directory and released into screening/ by `v3_gap_search.py release-audit-key` once every sheet row "
        "carries an auditor, a date and both auditor decisions",
        f"a run-specific seed ({run.audit_seed}), so the sampled rows differ from those whose strata the "
        "unblinded v1.0 sample (fn_audit_sample.csv) shows",
        "fn_audit_sheet.csv is tracked by the SHA-256 of its machine columns (audit_id, title, doi), so the "
        "auditor's entries do not break verify; screen never overwrites a sheet that holds auditor entries",
    ]
    manifest["outputs"] = {name: _sha(sd / name) for name in ("index.csv", "queue.jsonl", "worksheet.csv")}
    manifest["crossref_depth_yield"] = {
        "note": ("rows of each Indonesian-language Crossref query (1,000 rows by relevance) per rank band of 200: how "
                 "many carry an Indonesian title cue and how many the machine screen put in high or medium. A "
                 "Crossref record found by several queries counts in each; this is a reading aid, not recall"),
        "queries": crossref_depth_yield(screened, run)}
    manifest["audit_sheet"] = {
        "file": "fn_audit_sheet.csv", "machine_columns": AUDIT_SHEET_MACHINE_FIELDS,
        "machine_columns_sha256": sheet_machine_columns_sha256(sheet_rows),
        "blank_sheet_sha256": hashlib.sha256(csv_bytes(AUDIT_SHEET_FIELDS, sheet_rows)).hexdigest(),
        "rows": len(sheet_rows)}
    manifest["withheld_outputs"] = {"fn_audit_key.csv": {
        "sha256": _sha(key_path), "rows": len(sheet_rows),
        "held_at": "local private directory "
                   f"{(CACHE_BASE / run.dir_name(freeze) / 'private' / 'fn_audit_key.csv').as_posix()} (gitignored; "
                   "regenerated byte for byte by `screen` from the local cache)",
        "release": "after the audit: `v3_gap_search.py release-audit-key` checks that every sheet row is audited "
                   "and that the key matches this SHA-256, then copies it into screening/; verify then checks it "
                   "against this SHA-256 and fails if it is committed while any row is unaudited",
        "status": "withheld_until_audit_complete"}}
    a = manifest["audit_sample"]
    a["selection"] = (f"first n rows by sha256('{run.audit_seed}|<source>:<stratum>|<candidate_id>'); stable across "
                      "Python versions; plus deferred pool titles named in the research audit (purposive)")
    a["blinding"] = (
        "the auditor receives only fn_audit_sheet.csv: audit id, title and DOI, rows in one stable shuffle "
        f"(sha256('{run.audit_seed}|blinded-order|<source>:<candidate_id>')) across all strata; the key "
        "(stratum, machine priority, v3 priority if eligible, eligibility basis, machine flag) is not in the "
        "repository until the audit is complete (withheld_outputs)")
    a["machine_pre_audit_pool_deferred"]["note"] = ("a machine re-screen with lexicon sela-v3-families-2, NOT the "
                                                    "manual audit; the auditor columns are blank")
    manifest["limitations"] = manifest["limitations"][:-1] + [
        "Withholding the key removes the direct link from audit id to stratum, but the machine priority of every "
        "row can still be looked up by DOI or title in committed files (this run's screening/index.csv, the "
        "pool's data/research/sela_screening_index.csv, earlier runs' files). The auditor must work from the "
        "sheet alone, received out of band, and not consult the repository until the audit is done.",
    ]


def run_verify(root, freeze, out=None, run=None):
    """``freeze`` may also be a run directory name such as '2026-10-26_search-1.1'."""
    run = run or run_for_dir(str(freeze))
    name = str(freeze) if (run.legacy or str(freeze).endswith(run.dir_suffix)) else run.dir_name(freeze)
    od = Path(out) if out else Path(root) / OUT_BASE / name
    problems = []
    m = json.loads((od / "manifest.json").read_text(encoding="utf-8"))
    if m.get("run_version") != run.run_version:
        problems.append(f"manifest run_version {m.get('run_version')!r} does not match the run directory {name}")
    problems += verify_candidates_storage(od, m)
    data = S.read_bytes(od / CANDIDATES_GZ) if (od / CANDIDATES_GZ).exists() else b""
    if hashlib.sha256(data).hexdigest() != m["candidates_sha256"]:
        problems.append("candidates.jsonl sha256 (content of candidates.jsonl.gz) differs from manifest")
    rows = [json.loads(x) for x in data.decode("utf-8").splitlines()]
    allowed = {"candidate_id", "title", "authors", "journal", "doi", "pmid", "pmcid", "first_publication_date",
               "date_source", "within_window", "publication_types", "publication_type_eligibility",
               "publication_language", "publication_language_detail", "is_open_access", "license", "mesh_age_groups",
               "abstract_available", "abstract_source", "abstract_sha256", "openalex", "families_retrieved",
               "retrieval", "source_records", "in_register", "in_pool_7580", "status"}
    allowed |= {p.in_run_field for p in previous_runs(run)}
    seen = set()
    for r in rows:
        extra = set(r) - allowed
        if extra:
            problems.append(f"{r.get('candidate_id')}: unexpected fields {sorted(extra)} (abstracts must never be committed)")
        if r.get("status") != "candidate_metadata_only_unreviewed":
            problems.append(f"{r.get('candidate_id')}: status must stay candidate_metadata_only_unreviewed")
        if len(r.get("title") or "") > 1000 or len(r.get("authors") or "") > AUTHORS_MAX:
            problems.append(f"{r.get('candidate_id')}: overlong title/authors")
        key = r.get("doi") or r.get("pmid") or r.get("title")
        if key in seen:
            problems.append(f"{r.get('candidate_id')}: duplicate {key}")
        seen.add(key)
        if r.get("publication_language") is not None and not re.match(r"^[a-z]{2}$", r["publication_language"]):
            problems.append(f"{r.get('candidate_id')}: publication_language not ISO 639-1")
    for name, digest in m["inputs"].items():  # a candidates.jsonl input is the content of its .gz
        if not S.exists(Path(root) / name) or S.content_sha256(Path(root) / name) != digest:
            problems.append(f"input {name} changed since the run (rebuild or keep the old bytes)")
    sd = od / "screening"
    if (sd / "manifest.json").exists():
        sm = json.loads((sd / "manifest.json").read_text(encoding="utf-8"))
        for name, digest in sm["outputs"].items():
            path = sd / name
            if name == "fn_audit_key.csv" and not path.exists():
                # held outside the repository (moved out of screening/ in review round 3): checked when the
                # local private copy exists, otherwise there is nothing here to check
                path = private_dir(root, str(freeze)[:10], run) / name
                if not path.exists():
                    continue
            if not path.exists() or _sha(path) != digest:
                problems.append(f"screening/{name} sha256 differs from screening manifest")
        if sm["inputs"]["candidates.jsonl"] != m["candidates_sha256"]:
            problems.append("screening manifest is stale (candidates.jsonl changed)")
        if sm.get("status") != "machine_title_abstract_screen_unreviewed" or sm.get("is_appraisal") is not False:
            problems.append("screening must stay labelled machine_title_abstract_screen_unreviewed and not appraisal")
        with (sd / "worksheet.csv").open(encoding="utf-8", newline="") as f:
            for w in csv.DictReader(f):
                if w["final_decision"] != "NOT_REVIEWED" and not (w["reviewer_1"] and w["reviewer_2"]):
                    problems.append(f"worksheet {w['candidate_id']}: a final decision needs two named reviewers")
                if w["rag_ready"] != "NO" and w["full_text_appraised"] != "YES":
                    problems.append(f"worksheet {w['candidate_id']}: rag_ready without full-text appraisal")
        if sm.get("screening_version") != run.screen_version:
            problems.append(f"screening_version {sm.get('screening_version')!r} is not {run.screen_version!r}")
        if run.audit_design == "v1":
            with (sd / "fn_audit_sample.csv").open(encoding="utf-8", newline="") as f:
                for a in csv.DictReader(f):
                    if a["auditor_relevance"] != "NOT_AUDITED" and not (a["auditor"] and a["audit_date"]):
                        problems.append(f"audit {a['sample_id']}: an audit decision needs auditor and date")
        else:
            key = sd / "fn_audit_key.csv"
            if not key.exists():
                key = private_dir(root, str(freeze)[:10], run) / "fn_audit_key.csv"
            problems += verify_blinded_audit(sd, seed=run.audit_seed, key_path=key if key.exists() else None)
            if run.audit_design == "v3":
                problems += verify_withheld_key(sd, sm, private_dir(root, str(freeze)[:10], run))
    return problems


def verify_candidates_storage(od, m):
    """Storage sela-jsonl-gzip-v1: only the .gz is committed, in the deterministic format, with the recorded hash."""
    problems = []
    if (od / CANDIDATES).exists():
        problems.append("candidates.jsonl is present uncompressed; only candidates.jsonl.gz is committed (run build, "
                        "or delete the uncompressed copy)")
    gzp = od / CANDIDATES_GZ
    if not gzp.exists():
        return problems + ["candidates.jsonl.gz missing"]
    raw = gzp.read_bytes()
    _content, gz_problems = S.decode(raw)
    problems += [f"candidates.jsonl.gz: {p}" for p in gz_problems]
    st = m.get("candidates_storage") or {}
    if st.get("file") != CANDIDATES_GZ or st.get("storage_version") != S.STORAGE_VERSION:
        problems.append(f"manifest candidates_storage does not record {CANDIDATES_GZ} ({S.STORAGE_VERSION})")
    elif hashlib.sha256(raw).hexdigest() != st.get("gz_sha256"):
        problems.append("candidates.jsonl.gz sha256 differs from manifest candidates_storage.gz_sha256")
    return problems


def verify_withheld_key(sd, sm, private):
    """Audit design v3: the sheet's machine columns match the manifest; the key is either absent from the
    repository (withheld) or committed after a complete audit and equal to the recorded SHA-256."""
    problems = []
    info = (sm.get("withheld_outputs") or {}).get("fn_audit_key.csv")
    sheet_info = sm.get("audit_sheet") or {}
    if not info or not sheet_info:
        return ["screening manifest lacks withheld_outputs/audit_sheet (audit design v3)"]
    if "fn_audit_key.csv" in sm.get("outputs", {}) or "fn_audit_sheet.csv" in sm.get("outputs", {}):
        problems.append("audit design v3 records the key and the sheet outside 'outputs'")
    _h, sheet = _read_csv(sd / "fn_audit_sheet.csv")
    if sheet_machine_columns_sha256(sheet) != sheet_info.get("machine_columns_sha256") or \
            len(sheet) != sheet_info.get("rows"):
        problems.append("fn_audit_sheet.csv machine columns (audit_id, title, doi) differ from the screening manifest")
    if [r.get("audit_id") for r in sheet] != [f"AUD-{n:03d}" for n in range(1, len(sheet) + 1)]:
        problems.append("fn_audit_sheet.csv audit ids are not AUD-001.. in order")
    committed = sd / "fn_audit_key.csv"
    if committed.exists():
        if _sha(committed) != info.get("sha256"):
            problems.append("fn_audit_key.csv sha256 differs from the hash recorded when it was withheld")
        if not _fully_audited(sheet):
            problems.append("fn_audit_key.csv is committed before the audit is complete (this unblinds the sheet); "
                            "remove it: it stays in the private directory until release-audit-key")
    elif (private / "fn_audit_key.csv").exists() and _sha(private / "fn_audit_key.csv") != info.get("sha256"):
        problems.append("the private fn_audit_key.csv does not match the recorded sha256 (re-run screen)")
    return problems


def release_audit_key(root, freeze, out=None, run=None):
    """Copy the withheld key into screening/ once every sheet row is audited (audit design v3)."""
    run = run or LATEST
    if run.audit_design != "v3":
        raise SystemExit(f"release-audit-key: run {run.run_version} does not withhold its key (audit design "
                         f"{run.audit_design}); its audit sample is superseded, use run {LATEST.run_version}")
    sd = out_dir(root, freeze, out, run) / "screening"
    sm = json.loads((sd / "manifest.json").read_text(encoding="utf-8"))
    info = sm["withheld_outputs"]["fn_audit_key.csv"]
    private = private_dir(root, freeze, run) / "fn_audit_key.csv"
    if not private.exists():
        raise SystemExit(f"release-audit-key: {private} is missing; run `screen` to regenerate it from the cache")
    if _sha(private) != info["sha256"]:
        raise SystemExit("release-audit-key: the private key does not match the recorded sha256 (re-run screen)")
    _h, sheet = _read_csv(sd / "fn_audit_sheet.csv")
    pending = [r["audit_id"] for r in sheet if not _fully_audited([r])]
    if pending:
        raise SystemExit(f"release-audit-key: {len(pending)} sheet rows are not audited yet (auditor, date, "
                         f"relevance and publication kind), e.g. {pending[:3]}; the key stays withheld")
    atomic_write(sd / "fn_audit_key.csv", private.read_bytes())
    return sd / "fn_audit_key.csv"


def verify_blinded_audit(sd, seed=AUDIT_SEED, key_path=None):
    """The auditor sheet carries no machine label; the key (when available here) matches it row for row;
    decisions are attributed. ``key_path`` None means the key is withheld and not available locally."""
    problems = []
    with (sd / "fn_audit_sheet.csv").open(encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f)
        header = reader.fieldnames
        sheet = list(reader)
    if key_path is None and (sd / "fn_audit_key.csv").exists():
        key_path = sd / "fn_audit_key.csv"
    key = None
    if key_path is not None:
        with Path(key_path).open(encoding="utf-8", newline="") as f:
            key = list(csv.DictReader(f))
    if header != AUDIT_SHEET_FIELDS:
        problems.append(f"fn_audit_sheet.csv columns {header} are not the blinded set {AUDIT_SHEET_FIELDS}")
    if key is not None and [r["audit_id"] for r in sheet] != [r["audit_id"] for r in key]:
        problems.append("fn_audit_sheet.csv and fn_audit_key.csv do not list the same audit ids")
    for r in sheet:
        if r.get("auditor_relevance") not in AUDIT_RELEVANCE or r.get("auditor_publication_kind") not in AUDIT_KINDS:
            problems.append(f"audit {r.get('audit_id')}: unknown auditor value")
        if (r.get("auditor_relevance") != "NOT_AUDITED" or r.get("auditor_publication_kind") != "NOT_AUDITED") \
                and not (r.get("auditor") and r.get("audit_date")):
            problems.append(f"audit {r.get('audit_id')}: an audit decision needs auditor and date")
    if key is not None:
        order = sorted(key, key=lambda r: rank_key(seed, "blinded-order", f"{r['source']}:{r['candidate_id']}"))
        if [r["audit_id"] for r in order] != [r["audit_id"] for r in key]:
            problems.append("audit rows are not in the blinded shuffle order (rows must not be grouped by stratum)")
    return problems


def retrieval_markdown(manifest):
    rows = manifest.get("per_query_retrieval") or retrieval_table(manifest["requests"])
    out = ["| Family | Source | Query | Requests | Hits | Retrieved | Share | Requests at cap |",
           "| --- | --- | --- | --- | --- | --- | --- | --- |"]
    for r in rows:
        share = "n/a (fuzzy)" if r["source"] == "crossref" else (
            f"{100 * r['share_retrieved']:.1f}%" if r["share_retrieved"] is not None else "-")
        at_cap = (f"{r['capped_requests']} of {r['requests']} incomplete (all hits, {r.get('pages', 0)} pages)"
                  if r["cap"] == "all" else f"{r['capped_requests']} of {r['requests']} (cap {r['cap']})")
        out.append(f"| {r['family']} | {r['source']} | `{r['query_id']}` | {r['requests']} | {r['hit_count']:,} | "
                   f"{r['retrieved']:,} | {share} | {at_cap} |")
    return "\n".join(out)


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--root", type=Path, default=ROOT)
    p.add_argument("--freeze-date", default=DEFAULT_FREEZE)
    p.add_argument("--run", default=LATEST.run_version, choices=sorted(RUNS),
                   help=f"frozen run version (default {LATEST.run_version}; earlier runs stay reproducible)")
    p.add_argument("--out", type=Path, default=None,
                   help="output directory (default data/research/v3_gap_search/<freeze><run suffix>)")
    sub = p.add_subparsers(dest="command", required=True)
    pf = sub.add_parser("fetch")
    pf.add_argument("--refresh", action="store_true")
    pf.add_argument("--openalex-max", type=int, default=OPENALEX_MAX)
    sub.add_parser("build")
    sub.add_parser("screen")
    sub.add_parser("verify")
    sub.add_parser("plan", help="print the search plan (no network)")
    sub.add_parser("release-audit-key", help="after a complete audit: copy the withheld audit key into screening/")
    sub.add_parser("retrieval-table", help="print hits, retrieved rows and share per query (from the manifest)")
    a = p.parse_args(argv)
    dt.date.fromisoformat(a.freeze_date)
    run = RUNS[a.run]
    if a.command == "plan":
        for item in search_plan(a.freeze_date, run):
            print(item["key"], item["url"])
        return 0
    if a.command == "fetch":
        return run_fetch(a.root, a.freeze_date, refresh=a.refresh, openalex_max=a.openalex_max, run=run)
    if a.command == "build":
        m = run_build(a.root, a.freeze_date, a.out, run=run)
        print(json.dumps(m["counts"], indent=1, ensure_ascii=False))
        return 0
    if a.command == "screen":
        m = run_screen(a.root, a.freeze_date, a.out, run=run)
        print(json.dumps({k: m[k] for k in ("priority_counts", "screened_in_by_feature_family", "queue_count",
                                            "population_band_counts_screened_in", "audit_sample")}, indent=1))
        return 0
    if a.command == "release-audit-key":
        print(f"released {release_audit_key(a.root, a.freeze_date, a.out, run)}")
        return 0
    if a.command == "retrieval-table":
        od = out_dir(a.root, a.freeze_date, a.out, run)
        print(retrieval_markdown(json.loads((od / "manifest.json").read_text(encoding="utf-8"))))
        return 0
    if a.command == "verify":
        problems = run_verify(a.root, a.freeze_date, a.out, run=run)
        for x in problems:
            print("VIOLATION:", x)
        print(f"v3 gap search verify ({run.run_version}): {len(problems)} violation(s)")
        return 1 if problems else 0
    return 2


if __name__ == "__main__":
    sys.exit(main())
