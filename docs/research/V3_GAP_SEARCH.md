# v3 gap search and machine title/abstract screening

Status: three frozen search runs on 2026-10-26, all separate from the 100-record register and from the 7,580-record pool. Everything in them is bibliographic metadata plus **machine** screening labels. Nobody has screened, read or appraised any of these records. Supports research audit tasks R-3 and R-4 and decision D-048. Non-normative; [SPEC.md](../../SPEC.md) stays authoritative.

| Run | Screening | Lexicon | Directory | Status |
| --- | --- | --- | --- | --- |
| `sela-v3-gap-search-v1.2` | `sela-v3-tiab-screen-v1.2` | `sela-v3-families-2` | `data/research/v3_gap_search/2026-10-26_search-1.2/` | **current**; read by the [v3 evidence map](EVIDENCE_MAP_V3.md) |
| `sela-v3-gap-search-v1.1` | `sela-v3-tiab-screen-v1.1` | `sela-v3-families-2` | `data/research/v3_gap_search/2026-10-26_search-1.1/` | frozen and superseded; still rebuilds byte for byte from its own cache |
| `sela-v3-gap-search-v1.0` | `sela-v3-tiab-screen-v1.0` | `sela-v3-families-1` | `data/research/v3_gap_search/2026-10-26/` | frozen and superseded; still rebuilds byte for byte from its own cache |

All use the eligibility rule `sela-pubtype-eligibility-v1`. A later run never rewrites an earlier one: each run has its own directory and its own local cache, and it copies the requests it shares with an earlier run from that run's cache (newest first) without writing into it.

> **Dated note (2026-10-26, review round 3).** Run v1.1's audit key `screening/fn_audit_key.csv` was moved out of the committed directory into the local, gitignored private directory `data/research/corpus/.cache/v3_search/2026-10-26_search-1.1/private/`. Its bytes and its SHA-256 (in `screening/manifest.json` `outputs`) are unchanged, and `screen --run sela-v3-gap-search-v1.1` still writes the identical file (now to that private directory); every other v1.1 file is byte-identical to before. The v1.1 manifest's limitation line "the key file sits next to the sheet" is superseded by this note. `verify` checks the private copy where it exists. Use the v1.2 audit sheet, not the v1.1 one.

> **Dated note (2026-10-26, storage).** The three runs' `candidates.jsonl` files (7.1, 13.1 and 44.2 MB of metadata) are now committed compressed, as `candidates.jsonl.gz` (1.1, 2.0 and 6.7 MB), in the deterministic format `sela-jsonl-gzip-v1`; the uncompressed files are no longer in the repository. Their content is byte-identical to before and every recorded hash keeps its value: `candidates_sha256`, and every hash recorded under the name `candidates.jsonl` elsewhere, is still the SHA-256 of the uncompressed content. Each run manifest gained one entry, `candidates_storage`, with the SHA-256 of the `.gz` bytes; every other committed file of the three runs is unchanged, and `build` and `screen` reproduce every file byte for byte. Details in [Storage of the candidates](#storage-of-the-candidates-sela-jsonl-gzip-v1).

## Why a new run

The register and the 7,580 pool were collected for the v2 "Relay" features (navigation, handoff, summaries, safety). The audit (problems P6 and P7) found that they hold no direct record on listening, journaling, consultation preparation or accessibility, and nothing on adolescent consent or on adults 50+ with digital tools. It also found that their searches were English-language Europe PMC MEDLINE only. Run v1.0 searched for those gaps, recorded publication language, and added Crossref so that Indonesian journals outside MEDLINE can appear.

A review of run v1.0 then found five problems, which run v1.1 fixes:

1. **Low and unreported retrieval.** Europe PMC retrieval was capped at 50 records per year bucket, and the document called the search "bounded" without saying how much it kept. Five queries kept 5% or less of their hits (table below).
2. **`id_digital` required no mental-health term** and searched every field, so its 15,653 hits were dominated by general telemedicine and AI papers, many with an Indonesian author affiliation.
3. **`a11y_core` was broad**: "accessible" alone matched access to services, and "cognitive impairment" pulled in dementia research.
4. **No family for working-age or mid-life adults** (the brief's topic "dewasa"), although the register is skewed to 18-30-year-old students.
5. **English-only population and feature words**, and an **unblinded** false-negative audit sample (see the screening sections).

### Run v1.2 (review round 3)

A review of run v1.1 found that the cap was raised only for queries at 5% or less, so the cheapest and most important D-048 gap queries were still truncated to the relevance top 50 per year bucket: `listen_core` 271 of 663 (40.9%), `journal_core` 265 of 742 (35.7%), `qpl_core` 280 of 774 (36.2%), `teen_consent` 299 of 1,883 (15.9%, the adolescent consent and confidentiality gap), plus `continuity_core`, `safety_v3` and `help_v3` at about 10%. Candidates that were never retrieved are outside the screening and the false-negative audit, so their loss cannot be measured. Indonesian-language coverage rested on the top 40 Crossref rows per query. The review also found that the audit key sat next to the blinded sheet (see the false-negative audit section).

Run v1.2 changes only the retrieval depth and the audit layout; the query strings, year buckets, lexicon and screening rules are those of v1.1:

- **Every hit** (Europe PMC `pageSize=1000` with `cursorMark` pagination to the last page) for `listen_core`, `journal_core`, `qpl_core`, `teen_consent`, `older_digital`, `continuity_core`, `safety_v3` and `help_v3`. `a11y_core_v2` keeps its v1.1 request: it was already complete at 200 per bucket (665 of 665).
- The seven Indonesian-language Crossref queries keep **1,000 rows** (Crossref's largest page) instead of 40. A first pass at 200 rows still found 35-40 Indonesian-titled rows in ranks 161-200 of most of them, so the run was raised to 1,000 before it was frozen.
- The audit key is withheld from the repository until the audit is complete, the audit sample uses a run-specific seed, and the sheet is tracked by the hash of its machine columns (section below).

### Europe PMC retrieval per query

Hits are Europe PMC's `hitCount` summed over the six year buckets; retrieved is what the run kept. Every Crossref query kept its top 40 rows by relevance; Crossref's `total-results` counts every work matching any query word (hundreds of thousands to millions), so a share is not meaningful there. `python3 scripts/research/v3_gap_search.py --run <version> retrieval-table` prints the full table, Crossref included, from the committed manifest.

Run v1.0 (cap 50 per bucket for every query):

| Family | Query | Hits | Retrieved | Share | Buckets at the cap |
| --- | --- | --- | --- | --- | --- |
| listening_feeling_heard | `listen_core` | 663 | 271 | 40.9% | 5 of 6 |
| journaling_expressive_writing | `journal_core` | 742 | 265 | 35.7% | 5 of 6 |
| check_in_self_monitoring | `checkin_core` | 5,500 | 300 | 5.5% | 6 of 6 |
| consultation_preparation | `qpl_core` | 774 | 280 | 36.2% | 5 of 6 |
| continuity_after_consultation | `continuity_core` | 3,168 | 300 | 9.5% | 6 of 6 |
| adolescents_15_17 | `teen_consent` | 1,883 | 299 | 15.9% | 5 of 6 |
| adolescents_15_17 | `teen_digital` | 6,111 | 300 | 4.9% | 6 of 6 |
| older_adults_50_plus | `older_digital` | 5,992 | 300 | 5.0% | 6 of 6 |
| accessibility_disability | `a11y_core` | 7,042 | 300 | 4.3% | 6 of 6 |
| indonesia_digital_mh_privacy | `id_digital` | 15,653 | 300 | 1.9% | 6 of 6 |
| ai_bahasa_code_switching | `ai_id_lang` | 965 | 271 | 28.1% | 5 of 6 |
| help_seeking | `help_v3` | 2,830 | 300 | 10.6% | 6 of 6 |
| safety | `safety_v3` | 2,501 | 258 | 10.3% | 5 of 6 |
| privacy_sharing_revocation | `privacy_v3` | 2,063 | 279 | 13.5% | 5 of 6 |
| privacy_sharing_revocation | `revocation_v3` | 1,071 | 282 | 26.3% | 5 of 6 |

Run v1.1 (cap 200 per bucket for the queries marked *; cap 50 otherwise):

| Family | Query | Hits | Retrieved | Share | Buckets at the cap |
| --- | --- | --- | --- | --- | --- |
| listening_feeling_heard | `listen_core` | 663 | 271 | 40.9% | 5 of 6 |
| journaling_expressive_writing | `journal_core` | 742 | 265 | 35.7% | 5 of 6 |
| check_in_self_monitoring | `checkin_core` * | 5,502 | 1,155 | 21.0% | 5 of 6 |
| consultation_preparation | `qpl_core` | 774 | 280 | 36.2% | 5 of 6 |
| continuity_after_consultation | `continuity_core` | 3,168 | 300 | 9.5% | 6 of 6 |
| adolescents_15_17 | `teen_consent` | 1,883 | 299 | 15.9% | 5 of 6 |
| adolescents_15_17 | `teen_digital` * | 6,111 | 1,148 | 18.8% | 5 of 6 |
| older_adults_50_plus | `older_digital` * | 5,994 | 1,181 | 19.7% | 5 of 6 |
| accessibility_disability | `a11y_core_v2` * (narrowed) | 665 | 665 | 100% | 0 of 6 |
| indonesia_digital_mh_privacy | `id_digital_mh` * (needs a mental-health term) | 282 | 282 | 100% | 0 of 6 |
| indonesia_digital_mh_privacy | `id_health_data_law` * (new) | 489 | 489 | 100% | 0 of 6 |
| ai_bahasa_code_switching | `ai_id_lang` | 965 | 271 | 28.1% | 5 of 6 |
| help_seeking | `help_v3` | 2,830 | 300 | 10.6% | 6 of 6 |
| safety | `safety_v3` | 2,501 | 258 | 10.3% | 5 of 6 |
| privacy_sharing_revocation | `privacy_v3` | 2,063 | 279 | 13.5% | 5 of 6 |
| privacy_sharing_revocation | `revocation_v3` | 1,071 | 282 | 26.3% | 5 of 6 |
| working_age_adults | `working_adults_digital` * (new) | 527 | 527 | 100% | 0 of 6 |

- Every query is now above 5%. The lowest are `continuity_core` (9.5%), `safety_v3` (10.3%) and `help_v3` (10.6%), which kept the v1.0 cap. Run v1.2 retrieves all three in full.
- The hit counts of `checkin_core` and `older_digital` grew by 2 between the v1.0 requests (17:26-17:37 UTC) and the v1.1 requests (19:04-19:07 UTC), because Europe PMC kept indexing. The queries whose cache entries were reused carry their v1.0 retrieval time.

Run v1.2 ("all hits" requests paginated; the others reuse their v1.1 or v1.0 request; from `retrieval-table`):

| Family | Source | Query | Requests | Hits | Retrieved | Share | Requests at cap |
| --- | --- | --- | --- | --- | --- | --- | --- |
| listening_feeling_heard | europepmc | `listen_core` | 6 | 664 | 664 | 100.0% | 0 of 6 incomplete (all hits, 6 pages) |
| journaling_expressive_writing | europepmc | `journal_core` | 6 | 742 | 742 | 100.0% | 0 of 6 incomplete (all hits, 6 pages) |
| check_in_self_monitoring | europepmc | `checkin_core` | 6 | 5,502 | 1,155 | 21.0% | 5 of 6 (cap 200) |
| consultation_preparation | europepmc | `qpl_core` | 6 | 774 | 774 | 100.0% | 0 of 6 incomplete (all hits, 6 pages) |
| continuity_after_consultation | europepmc | `continuity_core` | 6 | 3,171 | 3,171 | 100.0% | 0 of 6 incomplete (all hits, 6 pages) |
| adolescents_15_17 | europepmc | `teen_consent` | 6 | 1,883 | 1,883 | 100.0% | 0 of 6 incomplete (all hits, 6 pages) |
| adolescents_15_17 | europepmc | `teen_digital` | 6 | 6,111 | 1,148 | 18.8% | 5 of 6 (cap 200) |
| older_adults_50_plus | europepmc | `older_digital` | 6 | 5,995 | 5,995 | 100.0% | 0 of 6 incomplete (all hits, 10 pages) |
| accessibility_disability | europepmc | `a11y_core_v2` | 6 | 665 | 665 | 100.0% | 0 of 6 (cap 200) |
| indonesia_digital_mh_privacy | europepmc | `id_digital_mh` | 6 | 282 | 282 | 100.0% | 0 of 6 (cap 200) |
| indonesia_digital_mh_privacy | europepmc | `id_health_data_law` | 6 | 489 | 489 | 100.0% | 0 of 6 (cap 200) |
| ai_bahasa_code_switching | europepmc | `ai_id_lang` | 6 | 965 | 271 | 28.1% | 5 of 6 (cap 50) |
| help_seeking | europepmc | `help_v3` | 6 | 2,831 | 2,831 | 100.0% | 0 of 6 incomplete (all hits, 6 pages) |
| safety | europepmc | `safety_v3` | 6 | 2,506 | 2,506 | 100.0% | 0 of 6 incomplete (all hits, 7 pages) |
| privacy_sharing_revocation | europepmc | `privacy_v3` | 6 | 2,063 | 279 | 13.5% | 5 of 6 (cap 50) |
| privacy_sharing_revocation | europepmc | `revocation_v3` | 6 | 1,071 | 282 | 26.3% | 5 of 6 (cap 50) |
| working_age_adults | europepmc | `working_adults_digital` | 6 | 527 | 527 | 100.0% | 0 of 6 (cap 200) |

- Every D-048 gap query and `continuity_core`, `safety_v3` and `help_v3` are now complete: 0 of 6 buckets incomplete, 100% of hits (`older_digital` needed 10 pages, `safety_v3` 7). A few hit counts grew by 1 to 5 since v1.1 because Europe PMC kept indexing (requests 20:15-20:38 UTC).
- Still partial, as in v1.1: `checkin_core` (21.0%), `teen_digital` (18.8%), `ai_id_lang` (28.1%), `privacy_v3` (13.5%) and `revocation_v3` (26.3%). They were not part of the round-3 request; they are listed as a limit below.

Indonesian-language Crossref queries in run v1.2 (1,000 rows each; `total-results` counts any word match):

| Family | Source | Query | Requests | Hits | Retrieved | Share | Requests at cap |
| --- | --- | --- | --- | --- | --- | --- | --- |
| adolescents_15_17 | crossref | `id_remaja_privasi_cr` | 1 | 329,838 | 1,000 | n/a (fuzzy) | 1 of 1 (cap 1000) |
| older_adults_50_plus | crossref | `id_lansia_cr` | 1 | 887,817 | 1,000 | n/a (fuzzy) | 1 of 1 (cap 1000) |
| indonesia_digital_mh_privacy | crossref | `id_kesehatan_mental_aplikasi` | 1 | 789,659 | 1,000 | n/a (fuzzy) | 1 of 1 (cap 1000) |
| indonesia_digital_mh_privacy | crossref | `id_konseling_daring` | 1 | 567,726 | 1,000 | n/a (fuzzy) | 1 of 1 (cap 1000) |
| indonesia_digital_mh_privacy | crossref | `id_pdp` | 1 | 653,960 | 1,000 | n/a (fuzzy) | 1 of 1 (cap 1000) |
| ai_bahasa_code_switching | crossref | `ai_id_mh_cr` | 1 | 1,290,744 | 1,000 | n/a (fuzzy) | 1 of 1 (cap 1000) |
| working_age_adults | crossref | `id_dewasa_pekerja_cr` | 1 | 799,142 | 1,000 | n/a (fuzzy) | 1 of 1 (cap 1000) |

The run manifest records, per rank band of 40, how many rows carry an Indonesian title cue (`per_query_retrieval[].indonesian_title_cue_by_rank_band`), and the screening manifest records, per band of 200, how many the machine screen put in high or medium (`crossref_depth_yield`). Indonesian-titled rows do not thin out by rank 1,000: for example `id_konseling_daring` has 193-199 Indonesian-titled rows in every band of 200, and `ai_id_mh_cr` still has 68 screened-in rows in ranks 801-1,000 (97 in ranks 1-200). Crossref keyword search therefore cannot exhaust Indonesian-language coverage at any practical depth; see the limits.

## Results of run v1.2 (committed in `data/research/v3_gap_search/2026-10-26_search-1.2/`)

| File | Content |
| --- | --- |
| `candidates.jsonl.gz` | 29,508 deduplicated candidates, metadata only (`status: candidate_metadata_only_unreviewed`); `in_run_1_0` and `in_run_1_1` name the earlier candidate ids of the same record. 6.7 MB compressed (44 MB of JSONL); see [Storage of the candidates](#storage-of-the-candidates-sela-jsonl-gzip-v1) |
| `manifest.json` | as in v1.1, plus `per_query_caps` with `all`, the pagination rule, `rows_per_query` for Crossref, the Indonesian title cue per rank band, and the requests reused from earlier runs; `candidates_sha256` (uncompressed content) and `candidates_storage.gz_sha256` (committed `.gz` bytes) |
| `screening/index.csv`, `queue.jsonl`, `worksheet.csv` | as in v1.1 (machine priority and labels; 400-row queue; blank dual-screening worksheet) |
| `screening/fn_audit_sheet.csv` | **blinded** 263-row audit sheet: audit id, title and DOI only, blank auditor columns |
| `screening/manifest.json` | screening counts, rules, sample design, blinding, output hashes, `audit_sheet` (hash of the sheet's machine columns) and `withheld_outputs` (the audit key's SHA-256 only) |

The audit key `fn_audit_key.csv` is **not** in the repository (see the audit section).

Counts:

- **Fetch.** 128 planned searches: 53 new Europe PMC pages and 7 new Crossref requests; 58 Europe PMC and 19 Crossref responses were reused from run v1.1 with their original retrieval time. Requests 17:28-20:47 UTC (reused v1.0/v1.1 requests carry their earlier time); 425 HTTP requests over two fetch passes, 0 errors. OpenAlex: 450 singleton lookups planned for the top screened-in candidates, 354 new and 757 reused (1,111 candidates enriched); no HTTP 429.
- **Sources.** 22,655 candidates have a Europe PMC record and 6,970 a Crossref record (117 both); 6,853 were found only through Crossref.
- **Overlap.** 52 register records found again (v1.1: 38); all 8,712 v1.1 candidates and 4,394 v1.0 candidates are in v1.2; 1,871 are also in the 7,580 pool.
- **Years.** 2021 (last ten weeks): 1,085; 2022: 4,623; 2023: 4,641; 2024: 5,248; 2025: 6,955; 2026: 6,956.
- **Publication language** (metadata only): `en` 22,962; not stated 6,323 (mostly Crossref records); `id` 100 (OpenAlex inference or Crossref deposit); others 123. 160 languages come from OpenAlex inference only. 5,073 titles look Indonesian; 73 of them have a recorded language other than Indonesian (`title_cue_disagrees`).
- **Publication type.** 1,221 excluded (protocols, errata and corrections, editorials, retractions). 28,496 candidates had an abstract for machine screening.
- **Machine screening v1.2** (same rules as v1.1): high 5,798, medium 10,209, deferred 12,280, excluded by publication type 1,221. Population bands of the screened-in rows: `includes_15_17_explicit` 937, `adolescent_or_youth_unverified` 4,090, `older_adults_signal` 4,794, `adults_signal` 3,183, `not_stated` 3,003. 1,633 screened-in abstracts carry a null or negative cue. The 400-row queue holds at least 30 rows per feature family (help-seeking 56).

## What was searched (run v1.1)

- **Window.** Freeze date 2026-10-26; cutoff 2021-10-26. The rule is inclusive, as in the corpus pipeline: `cutoff <= first publication date <= freeze`. A partial date counts only if every day it can denote fits inside the window. Six records have a month-precision date that straddles the cutoff, so their `within_window` is `null`; they are kept and flagged.
- **14 query families**, each with Europe PMC and Crossref queries:
  - listening and feeling heard;
  - journaling and expressive writing, including digital;
  - check-in and self-monitoring;
  - consultation preparation (question prompt lists, pre-visit and patient-generated summaries);
  - continuity after a consultation (visit summaries, open notes, between-session continuity);
  - adolescents aged 15-17, including consent, confidentiality and parental or guardian access (plus, new, an Indonesian Crossref query on adolescents' confidentiality and parental consent);
  - older adults 50+ with digital mental health (plus, new, an Indonesian Crossref query on *lansia*);
  - accessibility and disability in mental-health apps (narrowed);
  - Indonesian digital mental health (now requires a mental-health term) and, as a separate query, Indonesian health-data privacy (UU PDP);
  - AI handling of Bahasa Indonesia and code-switching;
  - help-seeking;
  - safety;
  - privacy, sharing and revocation;
  - **new:** working-age and mid-life adults (*dewasa*) with digital mental-health tools, with an English and an Indonesian Crossref query.

  The exact query strings and per-query caps are in the run manifest (`query_families`, `sources.europepmc.per_query_caps`) and in [scripts/research/v3_gap_search.py](../../scripts/research/v3_gap_search.py).
- **Europe PMC.** `resultType=core`, sources `MED` or `PMC`. Each query is split into six year buckets (2021-10-26..2021-12-31, then 2022 to 2025, then 2026-01-01..2026-10-26). The buckets counter the newest-first bias of the earlier collections. 17 queries, 102 requests.
- **Crossref.** 26 keyword queries (`/works?query=`, `type:journal-article`, the same window, relevance order, 40 rows each), including Bahasa Indonesia queries such as "aplikasi kesehatan mental digital", "pelindungan data pribadi kesehatan rekam medis", "kesehatan mental lansia aplikasi digital teknologi" and "kesehatan mental pekerja dewasa intervensi digital aplikasi". The public pool applied (no contact address was sent), paced to at most one request per second.
- **Reuse.** 60 Europe PMC searches and 22 Crossref searches were identical to v1.0 requests (same query, bucket and cap); their v1.0 responses were copied into the v1.1 cache with their original retrieval time. 42 Europe PMC and 4 Crossref searches were new.
- **OpenAlex.** OpenAlex list and search queries were **not used** (the keyless daily budget was exhausted at 2026-10-26T16:49Z during run v1.0, and no API key was requested, because that would mean sending the owner's email to a third party). Free singleton lookups (`/works/doi:...`) were made for the top 450 screened-in candidates: 476 lookups were reused from v1.0 and 277 were new; no HTTP 429 occurred in this run. They record OpenAlex's language and retraction signal where cached (753 candidates enriched).
- **Not harvested.** Garuda and SINTA (Indonesian journal portals) have no documented free API. They stay a manual gap.
- **Etiquette and rights.** Requests were sequential and paced, with bounded retries and a descriptive User-Agent; the v1.1 fetch made 324 HTTP requests with 0 errors and finished at 19:11 UTC. No paywalled page was opened. Responses, abstracts included, are cached only locally (`data/research/corpus/.cache/v3_search/2026-10-26_search-1.1/`, gitignored), because the screening needs abstracts and must be reproducible offline. **No abstract text is committed.** The committed files hold only an abstract SHA-256 and an availability flag; a test checks for leaks when the cache is present.

## Results of run v1.1 (committed in `data/research/v3_gap_search/2026-10-26_search-1.1/`)

| File | Content |
| --- | --- |
| `candidates.jsonl.gz` | 8,712 deduplicated candidates (by DOI, then PMID, then normalised title + year), metadata only, `status: candidate_metadata_only_unreviewed`; `in_run_1_0` names the v1.0 candidate id where the same record was found before. 2.0 MB compressed (13 MB of JSONL) |
| `manifest.json` | queries and per-query caps, year buckets, every request with HTTP status, hit count and retrieval time, the per-query retrieval table (`per_query_retrieval`), changes from v1.0, counts, input hashes, rights policy, limitations |
| `screening/index.csv` | machine priority and labels for all 8,712 |
| `screening/queue.jsonl` | 400 screened-in candidates, stratified by feature family |
| `screening/worksheet.csv` | the same 400 with **blank** dual-screening columns (two reviewers, disagreement resolution, final decision, `rag_ready=NO`) |
| `screening/fn_audit_sheet.csv` | **blinded** 263-row audit sheet for the auditor: audit id, title and DOI only, with blank auditor columns |
| `screening/fn_audit_key.csv` | the machine labels of the same 263 rows (stratum, machine priority, v3 priority if eligible, eligibility basis, machine flag), joined on `audit_id` only after the audit. Since review round 3 it is held in the local private directory, not in the repository (its SHA-256 stays in the manifest; dated note at the top) |
| `screening/manifest.json` | screening counts, rules, sample design, blinding, output hashes |

Counts:

- **Sources.** 7,827 candidates have a Europe PMC record and 974 a Crossref record (89 have both). 885 were found **only** through Crossref.
- **Overlap with earlier collections.** 38 register records were found again (a recall check; v1.0 found 32). 731 candidates are also in the 7,580 pool, and 4,373 were already in run v1.0.
- **Years.** 2021 (last ten weeks only): 816; 2022: 1,513; 2023: 1,516; 2024: 1,533; 2025: 1,714; 2026: 1,620.
- **Publication language** (from metadata only):

  | Language | Records | Basis |
  | --- | --- | --- |
  | `en` | 8,188 | mostly the Europe PMC indexer language (7,826 records have a curated language) or a Crossref deposit (372) |
  | not stated | 472 | Crossref records without a language |
  | `id` | 19 | all from OpenAlex inference only (flagged `publication_language_inferred_only`) |
  | others (de 16, fr 6, zh 4, es 3, it, ko, sv, ng) | 33 | |

  - 42 languages come only from OpenAlex (machine-inferred) and are flagged.
  - 262 titles look Indonesian (two or more Indonesian function words), but their deposited language is usually missing or `en`; 17 of them have a deposited language other than Indonesian. The flags `title_suggests_indonesian` and `title_cue_disagrees` mark them; the metadata value is not changed.
- **Publication type.** 549 candidates are excluded as protocols, errata or corrections (including Crossref `update-to` notices), editorials, or retraction notices and retracted works (OpenAlex `is_retracted`).
- **Abstracts.** 8,080 candidates had an abstract available for machine screening.

## Machine screening v1.1 (`sela-v3-tiab-screen-v1.1`)

- **Unit.** Title plus abstract, read from the local cache. Each family is a list of explicit regular expressions ([scripts/research/corpus/v3lexicon.py](../../scripts/research/corpus/v3lexicon.py)). A hit records the family, the field and the entry *label*, never the matched text.
- **Lexicon `sela-v3-families-2`.** Every `sela-v3-families-1` entry is unchanged; v2 only adds entries:
  - Indonesian age words as population signals: *remaja*, *anak*, *siswa* (school students; bare "SMA/SMP/SMK" is not used, because "SMA" is also spinal muscular atrophy), *pelajar*, *sekolah menengah*, *lansia*, *lanjut usia*, *usia lanjut*, and adult terms (*dewasa*, *pekerja*, *karyawan*, *usia produktif*).
  - Indonesian age phrases are read as ages ("berusia 15-17 tahun", "usia 60 tahun ke atas").
  - Indonesian feature and consent terms: *menulis ekspresif*, *jurnal harian*, *buku harian*, *privasi*, *kerahasiaan*, *data pribadi*, *persetujuan* or *izin orang tua*, *akses orang tua*, *bunuh diri*, *melukai diri*, *mencari bantuan*, *disabilitas*, *tunanetra*, *pemantauan suasana hati*, *persiapan konsultasi*, and Indonesian distress terms (*depresi*, *kecemasan*, *stres*).
  - A `working_age_adults` family (working-age adults, employees and workplace, general adult population, *pekerja*, *karyawan*, *orang dewasa*; *dewasa muda*, young adults, counts as an adult signal but not as working age).
- **Gates** (unchanged). Consent and confidentiality words count for minors only together with an adolescent signal, which now includes the Indonesian words. The AI-language family counts only with an AI or NLP term. "Accessibility" counts only in its digital or design sense.
- **Priority** (unchanged rule). `high` needs a mental-health term plus at least one v3 feature family, with a title hit and enough score. `medium` is lower scores. `deferred` is everything else; it is **not** an exclusion. Results: high 1,803, medium 3,150, deferred 3,210, excluded by publication type 549.
- **Population band v2** (title/abstract words, MeSH age groups, explicit ages). Adult and adolescent signals are kept apart.
  - `includes_15_17_explicit` (284 screened-in) needs the ages in the title and abstract to agree on **one** range or minimum that overlaps 15-17, and the record must not be a review (a review's ages are eligibility rules, not one sample). Ages that describe an advisory or working group do not count. This is the same rule as the corpus `age_band`.
  - `adolescent_or_youth_unverified` (911) means youth words, or a minor age that is not such a single range.
  - `older_adults_signal` (1,195) needs MeSH "Aged" or explicit older-adult words or ages. MeSH "Middle Aged" counts only as an adult signal, because it is tagged on most adult studies.
  - `adults_signal` 1,348; `not_stated` 1,215.
- **Indonesian age-word check.** 111 titles contain *remaja*, *anak*, *siswa*, *pelajar*, *lansia* or *lanjut usia*; none of the screened-in ones has band `not_stated` any more (78 youth unverified, 11 explicit 15-17, 19 older adults, 3 adults). Under screening v1.0, 15 high-priority titles with *remaja* or *lansia* had band `not_stated` (for example "... KESEHATAN MENTAL PADA REMAJA" and "PAK DEDE ... LANSIA ..."); in v1.1 all 15 have a band. Indonesian-language adolescent studies can now reach the minors family: two Indonesian studies of parental involvement and adolescent mental health were among the top five candidates of the evidence map's minors row (machine-ranked, not screened by a person). With run v1.2 the map reads the v1.2 queue, where two such Indonesian studies (V3-22324, V3-25821) are again in the minors row's top five, shown with language "not stated (title suggests id)".
- **Counter-evidence cues.** A cue list ("did not improve", "no significant difference", "terminated", ...) marks 620 screened-in abstracts as `possible_null_or_negative_finding`, so that a person can find counter-evidence quickly. A cue is not a result.
- **Queue.** Up to 30 rows per feature family (17 families), ranked by a title hit for that family first, then priority and score; then filled to 400. The queue covers every v3 feature; working-age adults has 57 rows and the smallest families (memory correction, AI language) have 30.

## False-negative audit (doc 14, criterion 5), blinded, key withheld (run v1.2)

Criterion 5 asks for a manual false-positive and false-negative audit before a shortlist may be called relevant. The machine cannot do that audit. It prepares a deterministic sample and a machine pre-audit; the auditor columns stay blank (`NOT_AUDITED`).

Run v1.2 (`sela-v3-tiab-screen-v1.2`) uses the v1.1 design below with four changes (review round 3: in v1.1 the key sat in the same committed directory as the sheet, so anyone with repository access could open it):

- **The key is withheld.** `screen` writes `fn_audit_key.csv` to the local, gitignored private directory `data/research/corpus/.cache/v3_search/2026-10-26_search-1.2/private/`; the committed `screening/manifest.json` records only its SHA-256 (`withheld_outputs`). `screen` regenerates the identical file from the local cache at any time.
- **Release after the audit.** `python3 scripts/research/v3_gap_search.py release-audit-key` copies the key into `screening/` only when every sheet row carries an auditor, a date, a relevance decision and a publication kind, and only when the private key matches the recorded hash. `verify` fails if the key is committed while any row is unaudited, or if a committed key differs from the recorded hash. Commit the released key together with the completed sheet.
- **The sheet survives the audit.** It is tracked by the SHA-256 of its machine columns (audit id, title, DOI) in `audit_sheet`, not by the hash of the whole file, so the auditor's entries do not break `verify`; `screen` refuses to overwrite a sheet that holds auditor entries.
- **A run-specific seed** (`sela-fn-audit-2026-10-26-run-1.2`), so the sampled rows differ from those whose strata the unblinded v1.0 sample (`fn_audit_sample.csv`) shows. The three titles named by the research audit stay in as purposive rows.

Limits and the machine pre-audit of v1.2:

- **What withholding does not do.** The machine priority of every row can still be looked up by DOI or title in committed files: this run's `screening/index.csv`, the pool's `data/research/sela_screening_index.csv`, and the earlier runs' files. The auditor should receive the sheet alone, out of band, and not consult the repository until the audit is done. This is recorded in the screening manifest's limitations.
- **Machine pre-audit of v1.2.** 39 of the 100 randomly sampled deferred pool rows come out high or medium under `sela-v3-families-2` (they are flagged `possible_false_negative` in the withheld key); 2 of the 20 excluded rows were excluded by a title cue alone.

The v1.1 design (run v1.1; v1.2 keeps it apart from the four changes above, with its own run's candidates in the `v3_run` strata):

- **Design (263 rows).**
  - From the 7,580 pool (`sela-title-screen-v1.1`): 100 deferred rows (the false-negative audit), 20 high and 20 medium (the false-positive check), 10 rows excluded by publication type (a check of the eligibility rule), and the 3 deferred titles that the research audit named (purposive).
  - From run v1.1: 60 deferred, 20 high, 20 medium and 10 excluded by publication type.
- **Selection.** The first *n* rows by `sha256("sela-fn-audit-2026-10-26|<source>:<stratum>|<candidate id>")`. This is stable on every Python version, unlike `random`.
- **Blinding.** The auditor gets **only `fn_audit_sheet.csv`**: an audit id (`AUD-001` ...), the title and the DOI. The rows are in one stable shuffle across all strata and sources (`sha256("sela-fn-audit-2026-10-26|blinded-order|<source>:<candidate id>")`), so neither the stratum nor the machine verdict can be read from the order or the id. The auditor records `auditor_relevance` (relevant, not relevant, unsure) and `auditor_publication_kind` (study report, protocol, correction or erratum, editorial, retraction, other), with name and date. After the audit, `fn_audit_key.csv` is joined on `audit_id`. In v1.1 blinding was procedural only: the key sat next to the sheet (until the review-round-3 move described in the dated note at the top). `verify` fails if the sheet gains a machine column, if the key and sheet ids differ, if the rows are not in the shuffle order, or if a decision has no auditor and date.
- **Machine pre-audit (not the audit).** The pool rows were re-screened with `sela-v3-families-2` on title plus abstract. Their abstracts were fetched from Europe PMC by PMID and cached locally (the v1.0 batches were reused; one new batch for the excluded rows).
  - 36 of the 100 randomly sampled deferred pool rows come out high or medium under v1.1 (34 under v1.0). They are flagged `possible_false_negative` in the key.
  - All three titles named by the audit come out high: "I Really Feel Heard by AI", "Adolescent consent and Generation Alpha" and "Lifefulness" (older adults).
  - 5 of the 20 excluded rows were excluded by a title cue alone, with no publication type behind it; the auditor's publication-kind column checks them.
  - This supports the audit's problem P6: the v1 title screen was tuned to v2 topics. It is not an estimate of a false-negative rate until a person has audited the sample.

## Run v1.0 and screening v1.0 (frozen, superseded)

Kept unchanged in `data/research/v3_gap_search/2026-10-26/` so that it rebuilds byte for byte (`--run sela-v3-gap-search-v1.0 build` and `screen` reproduce all seven files; its candidates are `candidates.jsonl.gz`, 1.1 MB compressed, since the storage note at the top). Do not use its screening for new work:

- 13 families, 112 requests (90 Europe PMC, 22 Crossref), retrieval 17:26-17:37 UTC; 4,818 candidates (4,054 Europe PMC, 839 Crossref, 764 only Crossref); 32 register records found again; 249 excluded by publication type; 4,364 with an abstract.
- Screening v1.0 (`sela-v3-families-1`): high 814, medium 1,608, deferred 2,147. Its `population_band` counted `includes_15_17_explicit` (181 screened-in) whenever **any** abstract age candidate overlapped 15-17, including conflicting candidates and reviews, which the v1.1 band no longer does.
- Its `screening/fn_audit_sample.csv` (243 rows) is **not blinded**: each row shows the stratum, the machine priority, the v3 re-screen priority and `machine_flag` next to the blank auditor columns, and the rows are grouped by stratum. It has no excluded-publication-type stratum. Use the v1.1 blinded sheet for the audit instead.
- The first fetch attempt sent a `select` field list that Crossref rejects (it has no `language` select); those 22 requests failed with HTTP 400 and nothing was cached, and the second run sent them without `select`.

## Storage of the candidates (`sela-jsonl-gzip-v1`)

Each run's candidates are committed as `candidates.jsonl.gz` only; `candidates.jsonl` is the logical name under which their hashes are recorded. Sizes on 2026-10-26:

| Run | `candidates.jsonl` (content) | `candidates.jsonl.gz` (committed) |
| --- | --- | --- |
| v1.0 `2026-10-26/` | 7,072,654 bytes | 1,062,125 bytes |
| v1.1 `2026-10-26_search-1.1/` | 13,068,273 bytes | 1,983,477 bytes |
| v1.2 `2026-10-26_search-1.2/` | 44,164,111 bytes | 6,683,357 bytes |

- **Format.** One gzip member (RFC 1952), written by [scripts/research/corpus/gzstore.py](../../scripts/research/corpus/gzstore.py): a fixed 10-byte header with no file name, comment or extra field, MTIME 0, XFL 2 and OS 255; raw deflate at level 9 (window 15, memLevel 8, default strategy); then the CRC-32 and the size. The header is written by the script, not by Python's `gzip` module, whose header for `mtime=0` has differed between Python versions. On CPython 3.13 with zlib 1.3 the bytes equal `gzip.compress(data, compresslevel=9, mtime=0)`. GNU `gzip -9n` gives other bytes (its own deflate code, OS 3); do not recompress the files by hand.
- **Hashes.** Nothing recorded changed its value or its meaning. The run manifest's `candidates_sha256` is the SHA-256 of the uncompressed JSONL, as before. The new `candidates_storage` entry records the file name, the storage version, the format and `gz_sha256`, the SHA-256 of the committed `.gz` bytes. Every other hash recorded under the name `candidates.jsonl` is also the hash of the uncompressed content of the `.gz` in that directory: the `inputs` of runs v1.1 and v1.2 (the earlier runs' candidates), `inputs.candidates.jsonl` of each `screening/manifest.json`, and the gap-search input of `data/research/corpus/evidence_map_v3.json` (listed in the provenance of [EVIDENCE_MAP_V3.md](EVIDENCE_MAP_V3.md)). The scripts check such an input by decompressing the `.gz`.
- **Rebuilds stay byte-identical.** `build` writes the `.gz` only when the content changes: when a `.gz` in this format already holds the same content, its bytes are kept. A rebuild therefore leaves the committed file and the manifest unchanged even where the local deflate implementation (zlib-ng, for example) would compress the same content differently. A build into an empty directory on such a system would give other `.gz` bytes and another `gz_sha256` for the same content; `candidates_sha256` is the identity that does not depend on the platform. `build` also deletes an uncompressed `candidates.jsonl` left in the run directory.
- **What `verify` checks.** No uncompressed `candidates.jsonl` in a run directory; the `.gz` is one member with the fixed header and a matching CRC-32 and size; `candidates_storage.gz_sha256` matches the committed bytes; `candidates_sha256` matches the decompressed content; then the same content checks as before (allowed fields, no abstracts, labels, duplicates). The leak check (`leak_check.py`) reads the decompressed text of every `data/research/v3_gap_search/**/*.jsonl.gz`, both as a committed file to scan and for the candidate titles it exempts.
- **Reading the file.** `zcat data/research/v3_gap_search/2026-10-26_search-1.2/candidates.jsonl.gz | head -1`, or `gzip.open(path, "rt", encoding="utf-8")` in Python. `v3_gap_search.py`, `v3_evidence.py` and `leak_check.py` read the `.gz` directly. `data/research/.gitattributes` marks `*.gz` as binary.

## Commands

```sh
python3 scripts/research/v3_gap_search.py plan              # print every request of run v1.2 (no network)
python3 scripts/research/v3_gap_search.py fetch             # network; fills only what the run's local cache lacks
python3 scripts/research/v3_gap_search.py build             # offline: cache -> candidates.jsonl.gz + manifest.json
python3 scripts/research/v3_gap_search.py screen            # offline: screening/ outputs (audit key to the private directory)
python3 scripts/research/v3_gap_search.py verify            # offline: hashes (.gz bytes and content), labels, no abstract fields, blank human columns
python3 scripts/research/v3_gap_search.py retrieval-table   # hits, retrieved rows and share per query
python3 scripts/research/v3_gap_search.py release-audit-key # after a COMPLETE audit only: key -> screening/
# a frozen earlier run: add --run sela-v3-gap-search-v1.1 (or -v1.0) to any command
```

`corpus_pipeline.py verify` also runs `verify` for every run directory. In CI there is no cache, so `fetch`, `build` and `screen` cannot be re-run there. CI checks the committed files and the code paths on synthetic fixtures (`tests/research/test_v3_gap_search.py`). Note that the research workflow is not triggered by a pull request that changes only `data/research/v3_gap_search/**` (see [CORPUS_PIPELINE.md](CORPUS_PIPELINE.md#commands)); run `verify` locally before committing such a change.

**The abstract-leak tests run only locally.** The tests that compare cached abstracts with the committed files (`test_no_cached_abstract_text_in_committed*_files` for each run, and `tests/research/test_leak_check.py`) need the gitignored cache and are skipped in CI, so a CI pass says nothing about abstract text in these files. They now test every cached abstract of every run for any shared run of 8 or more tokens (titles and journal names excepted), instead of two 80-character windows of every 50th abstract. Running them, and `python3 scripts/research/leak_check.py --require-cache`, is a **required step before committing** (see [CORPUS_PIPELINE.md](CORPUS_PIPELINE.md#required-before-committing)).

## Limits

- The search is bounded and ranked by each API. Run v1.2 keeps every hit of 12 of its 17 Europe PMC queries; `checkin_core` (21.0%), `teen_digital` (18.8%), `ai_id_lang` (28.1%), `privacy_v3` (13.5%) and `revocation_v3` (26.3%) are still truncated to their relevance top rows per bucket, and their unretrieved hits are outside the screening and the false-negative audit. It keeps the top 40 of each English Crossref query and the top 1,000 of each Indonesian one. It is not exhaustive or systematic, and hit counts are not evidence counts.
- **Indonesian-language coverage is partial.** Indonesian-titled and screened-in rows do not thin out by Crossref rank 1,000 (see the depth bands above), Crossref keyword relevance is fuzzy, and Garuda and SINTA have no documented free API and remain a manual gap.
- Crossref keyword search is fuzzy. Crossref-only records have no MeSH groups, and their deposited language is often missing or wrong.
- The lexicons miss paraphrases and over-match some words (for example "safety", "privacy", "stress", and in Indonesian *anak* or *krisis*). Priority is a reading order for people, not relevance, quality or permission for any use.
- Nothing here is training data. The runs stay separate from user data (D-033) and from the register; promoting a candidate to the register needs a person to screen and review it.
