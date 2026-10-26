# Research corpus pipeline

Status: implemented and run on 2026-10-26 (pipeline `1.1.0`, rules `sela-corpus-rules-v2`, claim gates `sela-claim-gates-v2`, index version `87902c81efe30d05`). Supports SPEC C15, I.research, V24 and decisions D-015 and D-016. This document is non-normative; [SPEC.md](../../SPEC.md) stays authoritative.

## Purpose

The pipeline rebuilds Sela's existing 100-publication register ([research_100.json](sela-relay/research_100.json)) into a corpus that can be reproduced and audited. For each record it:

- normalises identifiers;
- checks dates against Europe PMC, Crossref and OpenAlex;
- applies a rolling five-year cutoff;
- records the licence and full-text access status, plus retractions and corrections;
- records provenance for every field.

Where a Creative Commons licence explicitly allows it, the pipeline also extracts a few facts with deterministic patterns. Each extracted fact keeps a locator. Unknown values stay `null`. Discrepancies with earlier documents are reported, not smoothed over.

## What this is NOT

- **Not a systematic review.** No new search was run. The 100 records were selected earlier, and this pipeline re-verifies them; it does not search again or decide whether they belong in the register.
- **Not 100 full-text appraisals.** Every record keeps `reviewer_status: "not_reviewed"`. No person reviewed any field produced here. The inherited screening is title/abstract only.
- **Not clinical evidence.** Nothing here supports a claim that Sela is safe, effective or validated. The feature mappings are design hypotheses.
- **Not training data.** The corpus is never used to train or fine-tune a model, and product answers never retrieve it at inference time (see the last section).

## Commands

Python 3.10+ standard library only. The commands work from any directory.

```sh
# offline tests (fixtures only, no network)
python3 -m unittest discover -s tests/research -p 'test_*.py'

# 1. fill the local cache (network; sequential, paced, retried)
python3 scripts/research/corpus_pipeline.py fetch --freeze-date 2026-10-26
#    --offline: no network, only report missing cache entries (exit 1 if any)
#    --only R001,R002: limit to some records    --refresh: re-fetch cached entries

# 2. build outputs offline from the cache (byte-for-byte reproducible)
python3 scripts/research/corpus_pipeline.py build --freeze-date 2026-10-26

# 3. re-hash outputs and check schema/invariants (exit 1 on any violation)
python3 scripts/research/corpus_pipeline.py verify

# optional: scan product copy for forbidden claims (exit 1 on any hit)
python3 scripts/research/corpus_pipeline.py scan-claims path/to/copy.ts
#    --show-negated: also list claim wordings that the negation window suppressed (not counted)
```

- **Contact address for Crossref.** Crossref's polite pool uses a contact address. It is read only from the environment variable `SELA_RESEARCH_MAILTO`, sent only to Crossref, and never written to the cache, logs or outputs. Without it, the public pool is used at no more than 2 requests per second.
- **Missing cache entries.** `build` refuses to run while cache entries are missing, so a network failure can never be published as a column of `null`s.
- **Workflow.** [.github/workflows/research-corpus.yml](../../.github/workflows/research-corpus.yml) runs the offline tests and `verify` on pull requests (Python 3.10 and 3.12). Changes to `web/src/i18n/` also trigger it, because the tests scan that copy with the claim gates. A manual run can also fetch and rebuild. It uploads the outputs as an artifact and never commits them; the cache and full text are not uploaded.

Code: [scripts/research/corpus_pipeline.py](../../scripts/research/corpus_pipeline.py) (CLI) and the helper package [scripts/research/corpus/](../../scripts/research/corpus/). Tests: [tests/research/test_corpus_pipeline.py](../../tests/research/test_corpus_pipeline.py) with fixtures in [tests/research/fixtures/corpus/](../../tests/research/fixtures/corpus/). The fixtures are trimmed real metadata responses (no abstracts, no author lists) and synthetic JATS XML written for these tests. A synthetic 5-record mini register under `fixtures/corpus/repo/` drives the build and snapshot tests.

## Steps

Every step is a pure function. Network access happens only in `corpus/fetch.py`, always through the cache.

1. **Ingest** (`ingest.py`)
   - Parses `research_100.json` and validates the R-ids.
   - Cross-checks it against `research_100.bib` (keys, DOIs, titles, years).
   - Parses the `09_RESEARCH_REGISTER.md` annotations by R-id (title, date, theme, DOI link, index link, note, limitation).
   - Records the SHA-256 of every input in the manifest.
2. **Normalise identifiers** (`ids.py`)
   - DOI: lowercase, trimmed, `https://doi.org/` / `dx.doi.org` / `doi:` prefixes stripped, trailing punctuation and unbalanced `)` removed.
   - PMID: digits only (parsed from the PubMed index URL, or taken from Europe PMC).
   - PMCID: `PMC<digits>`.
3. **De-duplicate** by normalised DOI, PMID, PMCID and normalised title+year. This runs twice: on the register fields, and again after enrichment.
4. **Verify dates** (`dates.py`)
   - **Canonical date.** `canonical_publication_date` is the earliest **day-precision** date from a primary source: Crossref `published-online`, or Europe PMC `firstPublicationDate` / `electronicPublicationDate`.
   - **Dates that are stored but never used as the canonical date.** Print/issue dates, Crossref `issued`, OpenAlex and the register's own dates are stored and compared but never chosen. Crossref `created` is a deposit timestamp: it is stored and never compared.
   - **Known padding is removed.** OpenAlex `publication_date` is month-padded, so only its year is compared. Europe PMC print dates are cut to the precision of `journalInfo.dateOfPublication` (for example "2025 Jan-Dec" gives year precision). A Europe PMC `firstPublicationDate` copied from a padded print date is downgraded.
   - **Discrepancies.** A discrepancy is recorded when the gap is more than 31 days, the year differs, or a primary source has only month/year precision. Each discrepancy is labelled `primary`, `secondary` or `register`.
   - **Conflicts.** A conflict (forcing `needs_check`) means two primary day dates differ by more than 31 days or fall on different sides of the window.
5. **Provenance**
   - `provenance{field -> {source, endpoint, retrieved_at, rule}}`.
   - `sources{service -> {endpoint, url, retrieved_at, http_status, status}}`.
   - `retrieval_date` and `pipeline_version`.
   - Retrieval timestamps come from the cache, so a rebuild never changes them.
6. **Licence and access** (`rights.py`)
   - **Licence choice.** The licence is taken from Crossref `license[]`: content-version `vor` or `unspecified`, with a start date on or before the freeze date. The freeze date stands in for "today" so the build stays reproducible. It is cross-checked with Europe PMC's `license`. If the two disagree, the more restrictive licence is used and the disagreement is flagged.
   - **TDM-only licences.** A licence declared only for text-and-data-mining is reported as `publisher-tdm` and is never treated as a permission. It counts as Creative Commons only when Europe PMC states the same CC licence for the article.
   - **`access_status`** takes one of three values:
     - `open_access_licensed`: a CC licence plus Europe PMC open-access full text (`pmcid`, `inEPMC=Y`, `isOpenAccess=Y`);
     - `open_access_unclear`: some open-access signal without that combination;
     - `closed_or_unknown`: everything else.
   - **`text_processing_allowed`** is true only for an explicit Creative Commons licence. NC/ND restrictions are noted.
   - **`redistribution_allowed`** is true only for CC BY, CC BY-SA or CC0, and only when the sources agree.
   - **Retractions and corrections** come from Crossref `updated-by` (as on the original work), `update-to` (the record is itself a notice) and `relation`, plus OpenAlex `is_retracted`, Europe PMC `pubTypeList` and Europe PMC `commentCorrectionList`.
7. **Screening**
   - **Window.** `cutoff_date` = freeze date minus five calendar years. 29 February maps to 28 February, so a freeze date of 2028-02-29 gives a cutoff of 2023-02-28. `within_cutoff` = `cutoff_date <= canonical <= freeze_date`, inclusive as specified. Note that [06_RESEARCH_METHOD.md](sela-relay/06_RESEARCH_METHOD.md) words the bound as "after 26 October 2021". The difference matters only for a record dated exactly on the cutoff, and there is none.
   - **Status, in order of precedence:**
     1. `excluded_retracted`;
     2. `needs_check`: no day-precision primary date, conflicting primary dates, a DOI that is unresolvable or unconfirmed, a partial retraction, an update notice, or an unknown retraction status;
     3. `excluded_out_of_window`;
     4. `included_title_abstract_v0`: the inherited screening, still inside the window.
   - **`window_by_any_source`** tells a reviewer whether every source date of any precision falls inside the window. It is informational only and never changes a status.
   - **Population and country/language** come only from explicit terms in the title and the register's Indonesian annotation. The matched term and field are recorded (`population_detail`, `country_or_language_detail`). These are word matches, not verified study populations.
     - The annotation is our own transfer commentary and often names a country or group only to say the study does *not* carry over ("bukan panduan hukum Indonesia", "relevansi target dewasa bersifat tidak langsung"). The annotation is therefore split into clauses at `; . ! ? :`, spaced dashes and contrastive conjunctions (`tetapi`, `namun`, `melainkan`, …).
     - A clause is not used when it contains a negation marker (`bukan`, `tidak`, `tak`, `belum`, `tanpa`, `jangan`, `not`, `no`, …), a transfer/difference marker (`berbeda`, `relevansi`, `relevan`, `berlaku`, `perlu`, …), a Sela-context marker (`Sela`, `target`) or a conditional (`jika`, `bila`, …).
     - Hyphenated population compounds (`dokter-pasien`, `Physician- and … Generated`, `patient-reported`) and `non-`-prefixed countries/languages (`Non-English`) are not labels, in the title or in the annotation.
     - Every match that is not used is kept under `*_detail.skipped` with its reason (and clause), so nothing disappears silently. A field is `null` when no usable match remains. The rule errs towards `null`: it can skip a true mention in a clause that is negative for another reason.
8. **Traceable evidence extraction** (`extract.py`). This step runs only when `text_processing_allowed`, a PMCID, open-access full text and a CC licence statement inside the XML are all present.
   - **Fetching and parsing.** The pipeline fetches `https://www.ebi.ac.uk/europepmc/webservices/rest/{PMCID}/fullTextXML` into `data/research/corpus/.cache/fulltext/` (gitignored). It walks `<abstract>` and `<body>` only. Front matter, references, tables and captions are skipped, and citation markers (`<xref ref-type="bibr">`, numeric `<sup>`) are removed. ElementTree does not resolve external entities, and the XML is treated as untrusted data.
   - **`sample_size` patterns.** `n = N`, `total of N <people>` and `N <people>`, where up to two words may sit between the number and the noun. The search covers the abstract and body sections under a top-level Methods heading. Measures, analysis, setting, coder/rater, positionality and ethics subsections are excluded.
   - **Candidates that are kept but do not vote:**
     - subgroups (`n = 11, 3%`);
     - approximate counts (`more than 700,000 individuals`);
     - pre-enrolment counts (`96 eligible individuals`);
     - research-team counts (`rated by 2 …`, `3 physicians independently annotated`, `panel of 8 physicians (A.B., …)`);
     - `n` of non-person units (`per app (n=10)`).
   - **When `sample_size` is set.** A value is set only if all remaining candidates agree. Otherwise it stays `null` and every candidate is kept with a note. Review-type records always stay `null`, because pooled counts are not one study's sample.
   - **Design phrases and country names** are recorded at their first in-scope occurrence, with an occurrence count.
   - **`intervention_or_exposure`, `comparator`, `outcomes`.** These hold the first paragraph of an explicitly titled section in the abstract or under Methods. Otherwise they are `null`.
   - **`limitations`** holds the first paragraph of a section explicitly titled "Limitations" or "Strengths and limitations". This is a deliberate exception to the abstract/Methods rule, because journals usually put that section inside the Discussion.
   - **What each extraction stores:** the value, a verbatim whitespace-normalised snippet of 240 characters or fewer, its `snippet_sha256`, a locator (`part`, `section` path, 1-based `paragraph`), the source URL, and `status: "machine_extracted_unreviewed"`. Findings are never paraphrased.
   - **`study_type`** is built from Europe PMC `pubTypeList`, Crossref `type`, the register's index types and title cues. Each label keeps its basis. `primary` follows a fixed precedence (protocol > meta-analysis > systematic review > …).
9. **Flags.** These are design-based caution labels, all `assigned_by: "machine"`. Examples: `review_overlap_possible`, `observational`, `abstract_only_screening`, `population_not_indonesian`, `population_includes_minors_or_youth`, `print_year_differs`, `licence_restricts_redistribution`, `sample_size_conflicting_candidates` and `corrected`. A flag is a prompt for a reviewer, not a quality grade.
10. **Export** (`export.py`). Output uses sorted keys, compact JSON Lines, UTF-8 and a trailing newline, and records are sorted by id.
    - **Snapshots.** `snapshots/<freeze_date>/` holds a copy of the manifest and records. An existing snapshot with different content makes `build` fail *before* anything is written.
    - **`index_version`** is the first 16 hex characters of the SHA-256 over the knowledge-base rows serialised **without** the `index_version` field. A file cannot contain a hash of itself; `verify` recomputes it the same way.

## Outputs (`data/research/corpus/`)

| File | Content |
| --- | --- |
| `records.jsonl` | Full record per R-id. Required fields: `id, title, authors[], publication_date, doi, source_url, study_type, population, country_or_language, sample_size, intervention_or_exposure, comparator, outcomes, limitations, relevance_to_sela, license, access_status, screening_status, extraction_source, retrieval_date, reviewer_status`, plus `pmid, pmcid, journal, theme, dates{source->date}, date_precision, date_discrepancies[], within_cutoff, window_by_any_source, retraction_status, retraction_detail, text_processing_allowed, redistribution_allowed, flags[], evidence[], provenance{}, sources{}`, and `*_detail` objects that hold the basis of each simple value. |
| `knowledge_base.jsonl` | Reviewer search index used by the Go reviewer API (`SELA_RESEARCH_KB`): `id, title, authors, year, publication_date, journal, doi, url, theme, note_id, transfer_limitation_id, study_type, population, country_or_language, sample_size, license, access_status, screening_status, reviewer_status, within_cutoff, flags[] (ids), evidence[], index_version`. |
| `manifest.json` | Freeze and cutoff dates, versions, `index_version`, input and output SHA-256s, counts by status, year, access, licence, flag and more, every date discrepancy, the year-distribution check, duplicate and cross-check results, source endpoints and response counts, the retrieval window, and the rights policy. |
| `snapshots/2026-10-26/` | Immutable copy of `manifest.json` and `records.jsonl` for this freeze date. |
| `evidence_map.json` | Feature → R-id rows parsed from the tables in [07](sela-relay/07_EVIDENCE_TO_FEATURE.md) and [10](sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md). Each row records its R-ids, any missing ones, each R-id's screening status, and the rows that rely on records that are not included. Also holds doc 10's first appraisal batch. |
| `eval_fixtures/claim_gates.json` | 13 FORBIDDEN product-claim gates in Indonesian and English: `clinical_validation`, `efficacy`, `reduces_symptoms`, `suicide_prevention`, `safe_or_certified`, `diagnosis`, `therapy_or_counselling`, `risk_assessment_or_triage`, `prescribing`, `care_received_or_booking`, `evidence_count_overclaim`, `solves_access_barriers`, `statistical_or_adherence`. Each gate has its source document section, examples that must match and disclaimers that must not. `patterns` are self-contained case-insensitive regexes for Python `re` and JavaScript `new RegExp(p, 'iu')` (the form `web/src/test/claims.test.ts` uses); capture group 1 is the claim. Each pattern carries a negation window: a claim matches only if none of the 6 tokens before it in the same clause is a negation cue (`not`, `no`, `never`, `cannot`, `…n't`, `tidak`, `bukan`, `belum`, `tanpa`, `tak`, …). Clauses end at `. ! ? ; :` before whitespace, line breaks, em/en dashes and spaced hyphens. `but`/`however`/`tetapi`/`namun`/`melainkan` start a new scope, so "not a chatbot but an AI therapist" still matches. `claim_patterns` are the bare claims, for reading only. Go's RE2 cannot compile the patterns (no lookaround). A hit means a person rewords the copy; it is not an automatic verdict. |

Field interpretation:

- `null` means unknown or not extracted. It never means "absent" or "zero".
- `screening_status: included_title_abstract_v0` means the record keeps its earlier title/abstract screening and its verified date is inside the window. It is not a quality judgement.
- Entries in `evidence[]` are locations a reviewer can open. They are not findings. When the licence does not allow redistribution (CC BY-NC, CC BY-NC-ND), committed files hold the locator and `snippet_sha256` only; the snippet is `null` and `snippet_withheld` explains why.
- `relevance_to_sela` is our own register text (abstract-based, not full-text appraisal) plus the features that cite the record.

## Rights policy: six layers kept separate

| Layer | Where | Rule |
| --- | --- | --- |
| Bibliographic metadata (titles, identifiers, dates, licence statements) | Committed outputs | From public APIs, with source and retrieval time per field. Abstracts and reference lists are dropped before caching. |
| Our annotations (`note_id`, `transfer_limitation_id`, feature rows) | Committed outputs | Sela's own text from the register and docs 07/10. |
| Licensed full text | `data/research/corpus/.cache/fulltext/` only (gitignored, about 18 MB) | Fetched only under an explicit CC licence; never committed, never uploaded by CI. |
| Retrieval documents (short evidence snippets) | Committed outputs | At most 240 verbatim characters with a locator, only for CC BY / BY-SA / CC0; otherwise locator plus hash only. |
| Synthetic evaluation cases (`claim_gates.json`, test fixtures) | Committed | Written by us; no third-party text. |
| Training data | None | This corpus is not, and must not become, a training set. |

Licence fields reflect metadata that Crossref and Europe PMC publish. They are not legal advice.

## Re-running with a new freeze date

```sh
python3 scripts/research/corpus_pipeline.py fetch --freeze-date 2027-04-30   # only fetches what the cache lacks
python3 scripts/research/corpus_pipeline.py fetch --freeze-date 2027-04-30 --refresh   # re-verify metadata online
python3 scripts/research/corpus_pipeline.py build --freeze-date 2027-04-30
python3 scripts/research/corpus_pipeline.py verify
```

- **Cutoff.** The new cutoff is the freeze date minus five calendar years (here 2022-04-30). Records older than that become `excluded_out_of_window`, and nothing is deleted.
- **New snapshot.** A new `snapshots/<freeze_date>/` is written. Existing snapshots are never overwritten: rebuilding a past freeze date with different content fails.
- **Licences** are re-evaluated against the new freeze date (start dates).
- **Commit.** Commit the outputs only after `verify` exits 0.
- **Adding papers** means editing the register first. The pipeline never adds papers on its own.

## Observed results (run of 2026-10-26)

**Run**
- Fetch: 497 HTTP requests between 04:24:55Z and 04:28:44Z, plus one full-text request at 04:36:25Z after a licence-rule fix for R049.
- Crossref public pool, because `SELA_RESEARCH_MAILTO` was not set. Crossref answered 14 requests with HTTP 429, and all of them succeeded on retry after the `Retry-After` delay. There were 0 failed requests.
- Two builds from the cache were byte-identical, and a build under Python 3.10.20 was byte-identical to the one under Python 3.13.
- `verify`: 0 violations and 3 warnings (see the evidence map below).

**Identifiers and integrity**
- 100 records. All 100 DOIs resolve via doi.org.
- Crossref holds 99. R016 (`10.25646/13549`, Journal of Health Monitoring) is not a Crossref DOI, so Crossref returned 404.
- Europe PMC matched 98. R099 (ACM FAccT) and R100 (NEJM AI) are not indexed there. 89 records have a PMCID.
- **Duplicates: 0**, both on the register fields and after enrichment.
- JSON, BibTeX and register Markdown agree on every id, DOI, title, year, date, theme and note. The only exception is that the BibTeX `author` fields of R099 and R100 are comma-separated, so BibTeX would read each list as one name.

**Dates**
- Verified canonical dates: **96**. **Out of window: 0.** All 96 fall between 2021-10-26 and 2026-10-26.
- **needs_check: 4**, none of them for a primary-date conflict: no two primary day-precision dates disagreed by more than 31 days. The cause is that no primary source has a day-precision online date. Every source date of each of the four falls in 2025 (`window_by_any_source: all_inside`).
  - R002 (Stud Health Technol Inform): Crossref print date only; Europe PMC gives a month.
  - R013 (J Adolesc Health): month only.
  - R065 (JAMA Intern Med): Europe PMC gives a month; Crossref gives a print date only.
  - R100 (NEJM AI): Crossref `published-print` 2025-03-27 only.
- The register's own `first_publication_date` for R002, R013 and R065 (2025-08-01, 2025-09-01, 2025-07-01) looks month-padded.
- **Date discrepancies:** 57 entries across 21 records.

  | By type | Count |
  | --- | --- |
  | `year_mismatch` | 37 |
  | `month_precision` | 9 |
  | `gt_31_days` | 7 (all of them print/issue dates against online-first dates) |
  | `missing_primary_day_date` | 4 |

  | By kind | Count |
  | --- | --- |
  | `primary` | 15 (in 10 records) |
  | `register` | 8 |
  | `secondary` | 34 |

**Year-distribution check (doc 10)**
- Doc 10 states "First-publication year 2023: 2; 2024: 3; 2025: 86; 2026: 9".
- That **equals the JSON `publication_year` field** (issue year).
- It **differs** from the year of the JSON `first_publication_date` (2023: 2, 2024: 3, 2025: 94, 2026: 1).
- It **differs** from the verified canonical dates (2023: 2, 2024: 3, 2025: 90, 2026: 1, plus 4 unverified, all from 2025).
- Eight records counted as 2026 (R018, R019, R021, R022, R025, R026, R083, R089) were first published online in 2025; 2026 is their print issue year. The doc 10 label "first-publication year" is therefore inaccurate. The manifest records this under `year_distribution_check`; the document itself is left unchanged.

**Retractions:** 0. **Corrections or errata:** 3 (R034, R044, R061). These stay included and carry the flag `corrected`.

**Licences**

| Licence | Records |
| --- | --- |
| CC BY | 73 (33 with version 4.0 from Crossref; 40 version-less from Europe PMC) |
| CC BY-NC | 7 |
| CC BY-NC-ND | 6 |
| Publisher TDM-only | 5 |
| Publisher-specific | 3 |
| None found | 6 |

- `text_processing_allowed`: 86. `redistribution_allowed`: 73.
- Access: `open_access_licensed` 84, `open_access_unclear` 11, `closed_or_unknown` 5.

**Full text and extraction**
- **84 open-access full texts** were fetched and processed. Each XML licence statement was re-checked and all 84 were CC. None was committed.
- **`sample_size`, 8 extracted:** R001 428, R032 73, R035 24, R042 38, R052 12, R054 20, R086 527, R098 90.
- The other 76 processed texts: 31 left `null` because candidates conflicted, 30 with no explicit pattern, 15 review-type records left `null`.
- Labelled sections: `limitations` 38, `outcomes` 4, `intervention_or_exposure` 2, `comparator` 2.
- 694 evidence items, all `machine_extracted_unreviewed`. 602 committed with snippets (CC BY); 92 with locator and hash only (CC BY-NC or BY-NC-ND).
- During development, the AI agent that built this pipeline read the snippets behind each extracted value. The first rule set extracted 15 values, and 7 of them were not the study sample: interview minutes, a catchment population ("more than 700,000"), coders/raters/positionality counts, a count of outcomes, and an eligible-but-not-enrolled count. The next rule set added 4 more false values, then 1 more. Each one became an exclusion rule plus a regression test. The 8 remaining values match their snippets. This was a development check, not a review, and `reviewer_status` stays `not_reviewed`.

**Population and country (word matches)**
- A population term was found in 49 records; the most common are young people or youth (15), college or university students (10), and clinicians or health professionals (9).
- A country or language term was found in 24 records, Indonesia in **9** (R047–R054, R098). `population_not_indonesian`: 15. `population_country_not_stated`: 76. `population_includes_minors_or_youth`: 20.
- 11 mentions in 9 records were not used and are listed under `*_detail.skipped`: negated annotation clauses in R041, R044 and R057 ("Indonesia") and R054 ("dewasa"), hyphenated compounds in R011 and R036 (`dokter-pasien`, `pasien-klinisi`) and R065 (`Physician-`), `Non-English` in R057, plus clauses in R043 and R074 whose titles still supply the label.
- Correction: an earlier build for this freeze date matched terms inside negated annotation clauses. It labelled R041, R044 and R057 as Indonesian (counting Indonesia in 12, dropping their `population_not_indonesian` flag), R054 as adults and R011 as patients. That build was never committed. Its `snapshots/2026-10-26/` copy was deleted and rebuilt with rules v2. Only the population/country fields and their flags changed. Date, licence, screening and extraction results are unchanged.

**Evidence map**
- 20 feature rows (14 from doc 07, 6 from doc 10). 0 missing R-ids.
- Three rows cite a `needs_check` record:
  - R065 in doc 07 row 9 and doc 10 "Story Ledger and Meaning Lock";
  - R100 in doc 07 row 14.

  Resolve their dates by hand before relying on them in public claims.

**Claim gates:** 13 gates, 69 patterns. Every pattern compiles in Python `re` and in Node 22 `new RegExp(p, 'iu')`, and every example behaves as declared in both.
- `scan-claims` over `web/src/i18n/en.ts`, `web/src/i18n/id.ts`, `docs/SAFETY.md` and `docs/PRODUCT.md` gives 0 hits. Seven disclaimers are suppressed by the negation window, for example "A sent request does not mean you have received care", "… belum berarti kamu sudah mendapat perawatan", "Sela … tidak memberi diagnosis" and "… not an AI therapist".
- The 880 i18n strings, checked the way `web/src/test/claims.test.ts` checks them, also give 0 hits.
- Gates v1 used one-word lookbehinds and flagged 5 of those disclaimers (4 strings under the JavaScript check). It also missed "Sela adalah terapis", "Sela adalah psikolog AI", "Sela membantu mengatasi depresi" and "Sela is an evidence-based therapist". All of these are now examples, and `tests/research` scans the i18n copy and expects 0 hits.

## Known limitations

- Term lexicons, design phrases and sample-size patterns miss paraphrases, numbers written as words and non-English text. Precision was checked only on the 8 extracted values.
- Population terms are not checked for role. "klinisi" as raters or care providers (R009, R025, R032) still counts as a population term, because it appears in an affirmative clause.
- The claim gates' negation window can hide a real claim when an unrelated negation sits within six tokens ("Never feel alone with your AI therapist"). The window can also flag a disclaimer whose negation sits further back in a long list. The gates are a prompt for review, not a guarantee.
- 16 records have no processed Creative Commons full text, so their evidence fields stay `null` by design. For the other 84, only the abstract and Methods are searched.
- Licence, date and retraction metadata can change after the retrieval window (04:24–04:37 UTC on 2026-10-26). Re-run with `--refresh` and a new freeze date.
- The Europe PMC REST documentation page (https://europepmc.org/RestfulWebService) returned a Cloudflare 403 to this container. Instead, the `fullTextXML` endpoint was verified directly (HTTP 200 `application/xml` for an open-access PMCID).

Other references checked on 2026-10-26:

- Crossref pools and `mailto`: https://crossref.org/documentation/retrieve-metadata/rest-api/access-and-authentication/
- Crossref retractions in `update-to` and `updated-by`: https://www.crossref.org/blog/retraction-watch-retractions-now-in-the-crossref-api/
- OpenAlex costs (singleton lookups free, list/filter queries metered): https://help.openalex.org/access/example-costs/
- DOI handle API `responseCode` values: https://www.doi.org/the-identifier/resources/factsheets/doi-resolution-documentation
- Europe PMC open-access licences differ per article: https://europepmc.org/downloads/openaccess
- Claim-gate portability: Python lookbehinds must be fixed-length (https://docs.python.org/3/library/re.html); JavaScript lookbehind may be variable-length (https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Regular_expressions/Lookbehind_assertion); RE2 does not support lookaround (https://github.com/google/re2/wiki/Syntax)
- Workflow `paths` filter (runs if at least one changed file matches; `**` covers subdirectories): https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax

## Why there is no fine-tuning and no inference-time RAG (D-016)

The owner set six prerequisites for fine-tuning. None is met:

| Criterion | State |
| --- | --- |
| Measurable goal | No task metric exists on which a tuned model is expected to beat the current one. |
| Licensed dataset | 13 of 100 records are NC/ND and 14 have no CC licence. No task-labelled dataset with training rights exists. The register annotations are notes, not training targets. |
| Proven baseline gap | No measured failure of the base Meta model that tuning would fix. |
| Train/validation/test split | There is no real-distribution data to split. The synthetic fixtures are too few, and the product forbids real user stories. |
| Bilingual evaluation | No held-out ID/EN/code-switch evaluation for a tuned model exists. |
| Risk/cost analysis | Not done. Tuning would add GPU cost, licence risk and safety-regression risk without a stated benefit. |

Inference-time retrieval is not used either. Product answers never need literature: features are mapped to evidence statically in `evidence_map.json`. Retrieving third-party text at runtime would break the licence separation above. It would also put corpus text next to user stories in a model prompt, and V24 requires that corpus text stay data, never instructions. The corpus serves only the reviewer search API and people reading the evidence.
