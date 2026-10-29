# Reading library (content set `sela-reading-v1`)

Status: **content set v1 drafted on 2026-10-26 (version 1.0.0) and revised twice the same day after review (versions 1.1.0 and 1.2.0, see [Version history](#version-history)); implemented as data and verified locally by its validator; not reviewed by a person.** The validator passes on the library itself and **fails on purpose on one file outside it**, `packages/config/src/selfHelpCards.v1.json`, which holds the 22 WHO adaptations without their licence notice (open point 2). All 38 items are `machine_drafted_unreviewed`, and **none is publishable** until a human editor approves its exact text. Supports SPEC C21, I.data, V36, V39 and T19 (backlog S-140, S-141) and decisions D-033, D-038, D-044, D-045 and D-048. This document is non-normative; the SPEC stays authoritative.

**Where the references live.** SPEC ids from C16 and V26 upward, the D-0xx decisions and the design screens (S10–S26 in `docs/design/SCREENS.md`, `docs/design/CONTENT_STYLE.md`) are in the v3 SPEC and design documents on branch `feat/sela-v3-pilot` (working copy of 2026-10-26). This research branch's `SPEC.md` predates them, so they are named here without links.

The library is the public content behind the "Bacaan & latihan kecil" / "Reading & small exercises" surface (design screen S20) and the static "Langkah kecil untuk beberapa menit ke depan" section of the Immediate help page (S21). It is **public content** (D-033): it holds no personal data and no one's story, it is never copied into a user's notes, and a bookmark stores only an item id.

## What this is not

- **Not treatment, diagnosis or a health claim.** No item claims to cure, treat, diagnose, prevent or work. Every text is checked against the product claim gates and the reading gates below. In content set 1.1.0 two migrated cards failed; version 2 of each rewords the failing sentence, and no item fails in 1.2.0.
- **Not evidence that Sela works.** Items explain skills from WHO, Kemenkes and CDC material and explain Sela's own features. The research corpus is not cited, and no R-id appears in any item.
- **Not reviewed.** Every machine output is labelled `machine_*_unreviewed`. The authoring-time verifier (V36) has not run, because the Claude verifier still needs B-8. A verifier verdict would never replace the human sign-off.
- **Not teen content by default.** Every migrated card is `adult` only. Adult material is not generalised to 15–17.

## Files (`data/content/reading/`)

| File | Content |
| --- | --- |
| `reading_item.schema.json` | JSON Schema (draft 2020-12) for the items files, plus `$defs/sourcesFile` and `$defs/setFile` |
| `set.json` | Content-set manifest: version `1.1.0` with its change log (`changes`), file list with item ids, counts, a fingerprint over every item's text hash, the methods, and the v1 baseline (`packages/config/src/selfHelpCards.v1.json`, SHA-256 and commit) |
| `sources.json` | 17 sources: licence, licence evidence, suggested citation, the population the source addresses, what was read, the date checked, a re-verify-by date and, for every source whose text is adapted or paraphrased, the stored verbatim check (`verbatim_check`) |
| `content_gates.json` | 7 reading-specific forbidden-claim gates (`sela-reading-content-gates-v1`) |
| [`LICENSE.md`](../../data/content/reading/LICENSE.md) | Licence notice for this directory: `cards_who.json` is CC BY-NC-SA 3.0 IGO (licence URI, WHO's suggested citations and translation disclaimer, the terms, B-14); the licence of every other file |
| `cards_who.json` | 22 migrated WHO cards. **This file's text is licensed CC BY-NC-SA 3.0 IGO, not MIT** |
| `cards_kemenkes.json` | 4 migrated cards paraphrased from Kemenkes pages |
| `cards_cdc.json` | 2 migrated sleep cards from CDC "About Sleep" |
| `cards_safety_plan.json` | 2 migrated safety-plan structure cards (Stanley & Brown 2012, checked against the open SPRC quick guide) |
| `guides_sela.json` | 8 new guides about Sela's own features |

Validator, verbatim-overlap tool and tests: [tests/content/reading_validator.py](../../tests/content/reading_validator.py), [tests/content/verbatim_overlap.py](../../tests/content/verbatim_overlap.py) and [tests/content/test_reading_library.py](../../tests/content/test_reading_library.py). Python 3.10+ standard library only.

```sh
# offline tests (no network); 79 tests
python3 -m unittest discover -s tests/content -p 'test_*.py'

# validate the library, then scan the repository for WHO card text without its notice; exit 1 on any error
# (exits 1 today: one licence error, packages/config/src/selfHelpCards.v1.json, open point 2)
python3 tests/content/reading_validator.py

# after editing text: recompute hashes, reading time, plain-language and claim-gate results
# (never touches review fields), then re-run the validator
python3 tests/content/reading_validator.py --update

# list sources and contacts past their re-verify-by date and verbatim checks past their due date; exit 1 if any
python3 tests/content/reading_validator.py --check-stale --today 2026-12-01

# verbatim check of card text against a local copy of its source (counts and locators; --show-text is local only)
python3 tests/content/verbatim_overlap.py --items data/content/reading/cards_kemenkes.json \
    --ids jeda-dan-batas-waktu-belajar,menerima-diri-dan-tujuan-realistis --lang id --source /path/to/page.html
```

All of these work now. They were run on 2026-10-26 under Python 3.10, 3.12 and 3.13, with identical output, for versions 1.0.0 and 1.1.0, and under Python 3.11, 3.12 and 3.13 for version 1.2.0 (3.10 is not installed in this environment). `--update` is idempotent: run on unchanged files, it leaves them byte-identical. **No CI workflow runs the content tests yet**, so a change to the content gates, the WHO notices, the audience fields or the help catalogue could merge without them; a ready-to-paste job is in open point 1.

## Item structure

Every item has these fields:

- **Identity:** `id` (kebab-case, kept from v1 for migrated cards), `version`, `kind`, `topics`, `surfaces`, `ai_may_offer`.
  - `kind` is `exercise` (a "latihan kecil" with 2–5 steps), `safety_plan` (2–6 steps, so that it can keep the six steps of the Stanley & Brown structure), or `guide` (sections of paragraphs). A title or summary that states a number of steps ("enam langkah", "six steps") must match the steps, or carry an open editor flag.
  - `surfaces` is `reading` or `help_page`.
- **Language:** `languages` is always `["id", "en"]`. `language_notes` says who wrote or translated each language. No translation is an official WHO, Kemenkes or CDC translation.
- **Audience:** `audience` is `adult`, `15-17` or `all`, with `audience_basis`. A 15–17 item also needs `availability_note`.
- **Content:** `content` holds `title`, `summary` and then either:
  - `steps`, `why`, `seek_help_when` and optional `cautions`; or
  - `sections[].paragraphs[]`. Each paragraph carries its `basis`: `source` (with `source_id`, `locator` and `support`), `product_basis` (SPEC, decision or design ids, with `implementation_check`), `general_advice` (labelled "saran umum" / "general advice") or `evidence_gap`.
- **Rights:** `source` (`source_ids`, `relation` and `locator`), `licence`, `attribution`, `adaptation_notice` (required for CC BY-NC-SA material) and `adaptation_note` (what was changed, for reviewers).
- **Contacts:** `contacts` declares every phone number or web address the text mentions. Each entry gives the help-catalogue id, the sources, the date checked and the re-verify-by date.
- **Source claims:** `source_claims`, one entry for every `summary` or `why` that names a source (WHO, CDC, Kemenkes, Stanley & Brown or "health agencies"). Each entry gives the source ids, a locator, `source_says` (Sela's own English paraphrase of what the cited passage says, never its wording), a `quote_check` (the paraphrase shares fewer than 5 words in a row with the source), a `support` verdict (`supported`, `partly_supported`, `not_supported`, `not_checked`), a note, and an `editor_confirmation` that stays `pending` until a human editor confirms the entry for the exact text.
- **Machine checks:** `reading_time_minutes`, plus `checks` (`date_checked`, `plain_language`, `claims` with `editor_flags`, `source_fidelity`).
- **Provenance and review:** `origin` (authors, and for migrated cards the v1 id, the v1 text hash, the v1 source object and the v1 evidence note verbatim), `review` and `text_sha256`.

How the app should use an item:

- Show `attribution`, and `adaptation_notice` where present, under "Sumber & lisensi" on every view and in every export. For the WHO cards, the attribution text contains the licence URI and the source's IRIS link in parentheses. Render it in full, and preferably make both links clickable; do not shorten it to the licence name.
- An `adult` item that has not been reviewed for teens is labelled "Untuk 18 tahun ke atas" for the 15–17 band (S20); it is not hidden.
- Items in `help_page` (the two safety-plan cards) appear only on Immediate help, below the contacts, and only after review.
- As in v1, the model may only **pick an item id** where `ai_may_offer` is true; the text always comes from this data. The server filters by audience and age band before the model sees the catalogue. It never offers `safety_plan` items.
- Exercises are optional and have no day counters, streaks or badges (D-037, D-038). A content gate enforces this.

## Content set v1

| Group | Items | Audience | Source licence | Commercial use |
| --- | --- | --- | --- | --- |
| WHO cards (migrated) | 22 | adult | CC BY-NC-SA 3.0 IGO | **blocked (B-14)** |
| Kemenkes cards (migrated) | 4 | adult | no licence statement (public information) | needs legal confirmation |
| CDC cards (migrated) | 2 | adult | US public domain | allowed (1); needs legal confirmation (1, which also paraphrases Kemenkes Keslan) |
| Safety-plan cards (migrated) | 2 | adult | paywalled article, structure only | needs legal confirmation |
| Sela guides (new) | 8 | all (7), 15-17 (1) | Sela's own text | allowed |

The 8 guides are:

| Id | Topic | Audience |
| --- | --- | --- |
| `didengarkan-di-sela` | What listening in Sela is and is not; AI can be wrong (WHO 2024); crisis screening is open about what it does | all |
| `menulis-catatan-dan-melihat-kembali` | Notes and reflection; no promise that writing helps (recorded evidence gap) | all |
| `menyiapkan-konsultasi` | Preparing for a consultation and what to bring | all |
| `apa-yang-terjadi-pada-yang-kamu-simpan` | Storage choice, encryption, guest retention, AI providers, export and deletion | all |
| `berbagi-dan-mencabut-akses` | What sharing and revoking mean, and what they cannot do | all |
| `kerahasiaan-usia-16-17` | Confidentiality and its limits for 16–17 | 15-17 |
| `butuh-bantuan-sekarang` | When and how to get help now (Healing119, local emergency services, IGD) | all |
| `tampilan-dan-aksesibilitas` | Display settings and the WCAG 2.2 AA target, without claiming conformance | all |

### Migration of the 30 Dengar.in cards

- **Text kept identical in 23 cards.** Every title, summary, step, "why", "seek help" and caution of 23 cards is identical to `packages/config/src/selfHelpCards.v1.json`. The test compares them field by field, and `origin.v1_text_sha256` pins each card.
  - That file entered this repository in commit `7d898ae` and is byte-identical to `zlxrnn/dengar.in-2.0` at commit `9ee21e0`.
  - The v1 `source` object and `evidenceNote` are kept verbatim in `origin`.
- **Six cards changed in content set 1.2.0, each as version 2: their "why" credited the source with something the source does not say.** Only the "why" changed. Each card records `origin.text_changed: true`, says in `adaptation_note` what changed in version 2 and why, and carries an open editor flag that asks the editor to confirm or revert. See [Source claims](#source-claims-what-a-named-source-says).
  - `gerak-tubuh-setelah-kerja`: v1 said WHO recommends leisure-time activity "because evidence shows benefits for mental health". WHO's work guideline makes recommendation 8B conditional, on very low-certainty evidence, made mainly for work-related outcomes, and says the mental-health findings did not turn out as expected (PDF pp. 60, 64). The source contradicts v1.
  - `mulai-bergerak-lagi-sedikit-demi-sedikit`: v1 said "WHO programmes find ... directly linked to improved mood". PM+ Chapter 8 states the strategy's aim and reports no finding (PDF pp. 58-59), and "directly linked" reads as causal.
  - `jadwal-tidur-konsisten`: v1 gave CDC a reason ("because the body clock works best with a regular pattern") that the CDC page does not give. Version 2 drops the clause.
  - `khawatir-soal-tubuh-dan-kesehatan`: v1 credited WHO with the advice that a health worker's assessment settles the question. The guide does not say this; version 2 keeps that advice as the card's own, unattributed sentence.
  - `bersikap-baik-pada-diri-sendiri`: v1 said WHO's guide promises more energy "to help yourself and others". The guide speaks of helping others (PDF pp. 107, 127); version 2 drops "yourself".
  - `kesepian-langkah-kecil-terhubung`: v1 said the report "shows loneliness affects health". For physical and mental health the report describes associations (PDF pp. 109, 111), so version 2 says the report links loneliness to poorer health.
- **One card changed on purpose in content set 1.1.0: `rencana-keselamatan-enam-langkah`, version 2.**
  - The v1 card was titled "six steps" but listed five, because its step 3 merged two steps of the original.
  - Version 2 splits that step, so the card follows the six steps of the open SPRC quick guide, and drops an unsourced sentence (see [Safety-plan structure](#safety-plan-structure-checked-against-the-sprc-quick-guide)).
  - The card records `origin.text_changed: true` and keeps `origin.v1_text_sha256` as the record of the v1 text. Its `adaptation_note` says what changed and why, and two open editor flags ask the human editor to confirm or revert.
  - The v1 file in `packages/config` is unchanged.
- **Attribution.** The cards come from the Dengar.in v1 team ("Kata Justin Nama Timnya Gokil Udah", Telkom University Surabaya).
  - They were committed by I Wayan Bagus Arya Darma Wiguna (@YanIanZ) under SCRUM-46 and SCRUM-45 on 2026-10-15.
  - The v1 knowledge base ([docs/archive/v1/CAREBRIDGE_KNOWLEDGE_BASE.md](../archive/v1/CAREBRIDGE_KNOWLEDGE_BASE.md)) records that Justin, the Safety Owner, approved their runtime use. No content review of the card text is recorded.
- **Audience.** All 30 are `adult`:
  - The v1 cards were written for adults 18+ (knowledge base, section 1).
  - PM+ addresses adults and the work guideline addresses workers.
  - The two sleep durations are adult figures: CDC and Kemenkes Keslan give 8–10 hours for teens.
  - Widening a card's audience needs a human teen review, and the validator enforces this.
- **Not migrated: the 8 v1 daily missions** (`DOMAIN_MISSION_TEMPLATES`). They cite no source. D-038 merges the missions into "latihan kecil", and the sourced exercise cards fill that role.

### The WHO URL fix

The discovery report flagged the loneliness card's URL, `https://www.who.int/publications/i/item/978240112360`, as one digit short of the ISBN. That is correct, but the short slug is the one **WHO itself publishes**. On 2026-10-26:

- that URL answered HTTP 200 and is the page's canonical link;
- the 13-digit form `.../item/9789240112360` answered **HTTP 404**.

Adding the digit would therefore break the link. Instead, the source now records:

- WHO's working landing URL;
- the persistent IRIS handle `https://iris.who.int/handle/10665/381746`;
- the correct ISBN, 978-92-4-011236-0 (electronic) and 978-92-4-011237-7 (print).

The v1 text is unchanged. The validator rejects the 404 form anywhere in the library.

## Licences

| Material | Licence | What Sela must do |
| --- | --- | --- |
| WHO adaptations (22 cards) | CC BY-NC-SA 3.0 IGO (<https://creativecommons.org/licenses/by-nc-sa/3.0/igo>), read on each PDF's copyright page and in IRIS `dc.rights` | Non-commercial use only. Cite WHO's suggested citation. Give the licence URI with every copy (legal code 4(b)(I)) and the source link (4(d)(iii)); both are in each card's attribution. Show the adaptation and translation notice. License the adaptation under the same licence (share-alike). Do not use the WHO logo. Never suggest that WHO endorses Sela. Any commercial launch is **blocked until B-14 is resolved** (D-045). The full notice is [`data/content/reading/LICENSE.md`](../../data/content/reading/LICENSE.md) |
| Kemenkes paraphrases (4 cards, plus part of 1 CDC card) | No licence statement. The v1 team relied on UU 28/2014 Pasal 43 huruf b and paraphrased without copying | Attribute Kemenkes. Get legal confirmation before a public or commercial release (v1 knowledge base, section 5.6). Not legal advice |
| CDC "About Sleep" (2 cards) | US public domain | Attribute CDC. Imply no endorsement |
| Stanley & Brown 2012 (2 cards) | Paywalled article; the form is copyright Stanley & Brown 2008, 2021 | Describe the structure only and reproduce no form text. Get legal confirmation before commercial use |
| SPRC "Safety Planning Guide: A Quick Guide for Clinicians" (2 cards, structure check) | Copyright 2008 Stanley & Brown, reprinted by WICHE and SPRC with the authors' permission; free to read, no reproduction without written permission | Read only, to check the structure. Nothing is reproduced; locators only |
| Sela guides (8) | Repository licence (MIT) | Short facts are cited with the source and a locator; no source text is reproduced |
| WHO 2024 AI guidance; WHO 2025 adolescent standards; W3C WCAG 2.2 and WAI page | Cited only | The 2025 adolescent standards PDF says CC BY-NC-SA **4.0** IGO while IRIS says 3.0 IGO. The discrepancy is recorded; only a short fact is cited |

The attribution shown with each WHO card follows WHO's suggested citation, then gives the source link and the licence URI in parentheses, in WHO's reference style. Example (`grounding-saat-badai-emosi`):

- **English:** "Adapted from: Doing what matters in times of stress: an illustrated guide, Tool 1: Grounding. Geneva: World Health Organization (WHO); 2020 (https://iris.who.int/handle/10665/331901). Licence: CC BY-NC-SA 3.0 IGO (https://creativecommons.org/licenses/by-nc-sa/3.0/igo)."
- **Indonesian:** the same, starting "Diadaptasi dari:".

The validator requires every WHO card's attribution, in both languages, to contain the licence URI and the IRIS link of each cited source. It also requires `LICENSE.md`, with the URI, the legal code, the scope, B-14, D-045, the terms, every suggested citation and the translation disclaimer.

The WHO notice shown with each WHO card:

- **English:** "This adaptation and its translation were made by the Dengar.in and Sela team, not by the World Health Organization (WHO). WHO is not responsible for the content or accuracy of this adaptation or translation, and does not endorse Sela. The original English edition is the binding and authentic edition. This adaptation is licensed under CC BY-NC-SA 3.0 IGO."
- **Indonesian:** the same notice in `adaptation_notice.id`.

WHO's verbatim translation disclaimer is stored in each WHO source as `translation_disclaimer_verbatim`.

### Commercial-use blocker B-14

B-14 ("WHO card licence for commercial use") blocks any commercial use of the 22 WHO cards. Each card records `licence.commercial_use: "blocked"` and `licence.blocker: "B-14"`. The non-commercial pilot may use them only with the attribution and the notice.

B-14 clears in one of two ways:

- **(a)** WHO grants permission for the intended commercial use (requests go through WHO's licensing page, linked from each PDF);
- **(b)** the cards are replaced by content Sela owns or licenses for commercial use.

Until then, a commercial build must filter on `commercial_use` and drop the WHO file. Seven more items need legal confirmation before commercial use (Kemenkes, Keslan and Stanley & Brown); that is a separate question from B-14, recorded here as open point 3.

## Sources and verification (checked 2026-10-26)

Full texts were read only through official open routes: WHO IRIS, CDC, the Kemenkes sites, healing119.id, W3C and, for the safety-plan structure, SPRC (which publishes the Stanley & Brown quick guide with the authors' permission). No paywall was bypassed. The Stanley & Brown article was not accessed; only its DOI was resolved.

Downloaded PDFs and pages stayed in a local scratch directory and are **not committed**. Items and reviewer notes hold Sela's own wording plus locators (PDF page numbers or section names), never quoted text from the CC BY-NC-SA sources. The 22 WHO cards themselves are adaptations, which D-045 allows in the non-commercial pilot.

**Verbatim WHO text in the repository.** WHO's own wording appears in these places:

- **Required notices.** WHO's translation disclaimer, which WHO's licence asks adapters to add, is kept in `sources.json` (`translation_disclaimer_verbatim`) and in `LICENSE.md`. WHO's four suggested citations are reproduced the same way.
- **One sentence and one example in a card.** `lepas-kaitan-sadari-namai-fokus`:
  - step 1 (English) is WHO's sentence word for word;
  - the second example in step 2 (English) is one of WHO's naming examples word for word.

  Both are from DWMM, Summary, Tool 2, PDF p. 125. Reuse is allowed, because `cards_who.json` is itself licensed CC BY-NC-SA 3.0 IGO (share-alike). The card's `language_notes.en` and `adaptation_note` now say so.
- **Short everyday phrases.** Six other cards share a phrase of five or six words with their source (for one of them, the source's title), and nothing longer. The machine-readable list is `sources.json` `verbatim_check`:

| Card | Field (English) | Shared run | Source page |
| --- | --- | --- | --- |
| `grounding-saat-badai-emosi` | `steps[3]`, `steps[4]` | 5 and 6 words | DWMM PDF pp. 41, 69 |
| `bertindak-sesuai-nilai` | `title`, `steps[1]` | 5 words each | DWMM PDF pp. 42, 22 |
| `gerak-tubuh-setelah-kerja` (version 2) | `why` | 6 words: the guideline's title, "guidelines on mental health at work". In 1.1.0 the v1 `why` shared a 5-word phrase with PDF p. 64 instead | Mental health at work PDF p. 1 |
| `berduka-memberi-ruang-dan-berbaik-hati` | `summary` | 5 words | DWMM PDF p. 53 |
| `jeda-dulu-sebelum-bicara` | `steps[2]` | 6 words | DWMM PDF p. 40 |
| `konflik-keluarga-nilai-dan-satu-tindakan-baik` | `steps[3]` | 5 words | DWMM PDF p. 98 |

How this was found: a machine check (`sela-verbatim-overlap-v1`, run on 2026-10-26, unreviewed) looked for the longest run of identical words between each English text field of the 22 WHO cards and the text of its source PDF, reporting runs of 5 or more words. Since content set 1.2.0 the check is a committed tool, `tests/content/verbatim_overlap.py`; re-run on the v1 text it reproduces the 1.1.0 result exactly. The result is stored per source in `sources.json` (`verbatim_check`: the copy's URL, date and SHA-256, the longest run, and every run of 5 or more words with its locator). The PDFs are not committed, so the validator checks the stored result's consistency but cannot recompute it. The Indonesian text was not compared, because IRIS lists no Indonesian edition of these works. The same check covers the Sela guides that cite a WHO work: one guide sentence (`butuh-bantuan-sekarang`) shared 6 words with DWMM PDF p. 11 and was reworded in 1.2.0.

Earlier wording of this document said that the translation disclaimer was the only verbatim WHO text. That was wrong; the check above corrects it.

`sources.json` records what was read for each source. Some details:

- **WHO PDFs.** For DWMM, PM+, the mental health at work guideline and the loneliness report, the copyright page and every page a card cites were read.
- **Healing119.** The Kemenkes Ditjen Kesmas article of 7 August 2025 and the healing119.id front page support these facts:
  - 119 extension 8;
  - WhatsApp through the site;
  - free and anonymous;
  - initial support and referral;
  - "morning to night" hours.

  The contact is also in the help catalogue (`web/src/data/help.fallback.json`, entry `healing119-kemenkes`, re-verify by 2027-01-14). The validator checks that every contact the text mentions is declared and matches that catalogue (V39: one catalogue).
- **Ayo Sehat.** Both Ayo Sehat pages answered **HTTP 503 (site under maintenance)** twice in the morning and again at 20:07 UTC. The Internet Archive's availability API lists a snapshot of each page (2026-10-02 and 2026-06-11), but `web.archive.org` itself could not be reached from this environment, so no copy was read. Their three cards keep the v1 verification date (2026-10-15), carry an open `source_unreachable` editor flag, and have a re-verify-by date of 2026-11-26. **Their "paraphrased, no text copied" rests only on the v1 team's statement**: the verbatim check has not run (`verbatim_check.status: "not_run"`, due by 2026-11-26), the texts now say "not yet verified", and the cards stay `needs_legal_confirmation` and unpublishable until it runs.
- **Keslan.** The Ditjen Kesehatan Lanjutan page answered HTTP 200 on 2026-10-26. The verbatim check compared the Indonesian text of its two cards with it: the longest shared run is 3 words for `tidur-cukup-tujuh-sampai-sembilan-jam` and 4 for `kamar-dan-layar-sebelum-tidur`. The page is a news item from RSUP Dr. M. Djamil Padang reporting one of its doctors' talks; a new editor flag notes that "Kemenkes advises" simplifies this.
- **CDC and SPRC.** CDC "About Sleep" (public domain) shares one 5-word run with `kamar-dan-layar-sebelum-tidur` ("at least 30 minutes before"). The SPRC quick guide, which may not be reproduced, shares at most 3 words with the safety-plan cards.
- **SPRC quick guide.** The SPRC resource page answered HTTP 200 on 2026-10-26. The older PDF path cited in some listings answered HTTP 404, so the PDF (2 pages) was taken from the link on the resource page; its SHA-256 is in `sources.json`. Both pages were read. Nothing from it is reproduced; it carries "no reproduction without written permission".
- **Re-verification cadence.** Other sources must be re-verified by 2027-04-26, following the v1 rule of at least every six months. Healing119 sources follow the help catalogue's date.

## Machine checks and results

All checks are labelled `machine_checked_unreviewed`. The validator recomputes every stored value, so a stale check fails the tests.

### Plain language (`sela-plain-language-v1`)

- **Method.** Body text is split into sentences at `. ! ?`. The check flags:
  - any sentence over 20 words;
  - an average over 15 words per sentence;
  - an English Flesch–Kincaid grade over 8, using a vowel-group syllable heuristic.
- **These are the targets of `docs/design/CONTENT_STYLE.md` section 3** (on `feat/sela-v3-pilot`). No validated Indonesian readability formula exists, so Indonesian gets sentence-length checks only. Readers from the target groups still have to check the text in owner testing.
- **Result.** The 8 guides are within targets (English Flesch–Kincaid grade 2.9 to 5.9). **28 of the 30 migrated cards need an editor**, mostly for sentences over 20 words. Their text was kept faithful, so splitting those sentences is an editorial change: the editor bumps the item version and reviews the result.

### Claims (`sela-claim-gates-v2` + `sela-reading-content-gates-v1`)

- **What is scanned.** Every user-facing string, the attribution, the notice and the internal `evidence_note`.
- **The gates.**
  - The 13 product-claim gates are used exactly as published in [claim_gates.json](../../data/research/corpus/eval_fixtures/claim_gates.json); that file is not modified.
  - The 7 reading gates cover cure or heal, guaranteed outcome, efficacy or certainty, causal benefit, condition labels, medication, and engagement pressure. They are wrapped with the same negation-window prefix, which the validator derives from the published patterns. Disclaimers such as "Sela tidak memberi diagnosis" therefore pass.
  - Migrated cards also get the Dengar.in v1 forbidden-term list.
- **Hits need flags.** Every hit must be covered by an `editor_flag`. A hit is a prompt for review, not a verdict.

Results:

| Item | Finding | Proposed fix (not applied) |
| --- | --- | --- |
| `gerak-tubuh-setelah-kerja` | 1.1.0: gate hit, efficacy or certainty ("evidence shows benefits"). The 1.2.0 source check found more: WHO made recommendation 8B *conditional, on very low-certainty evidence*, mainly for work-related outcomes, and says the mental-health findings did not turn out as expected (PDF pp. 60, 64). The 1.1.0 proposal ("may be considered for workers' mental health") misstated the reason too | **Applied in version 2**: the "why" now gives the recommendation's strength (conditional), the certainty of the evidence (very low) and WHO's main reason (benefits at work), and says the mental-health results were not as expected. See the card and its `source_claims` entry. The editor confirms or reverts |
| `mulai-bergerak-lagi-sedikit-demi-sedikit` | Gate hits, efficacy and causal: "WHO programmes find … directly linked". PM+ states a rationale, not a finding, and "directly linked" reads as causal | **Applied in version 2**: the "why" now says what PM+ does with the technique (small, gradual steps, aimed at the cycle of inactivity and low mood), not what it found. The editor confirms or reverts |
| `jadwal-tidur-konsisten` | (1) No gate hit; source mismatch. "7 to 9 hours" is Kemenkes Keslan's adult figure; CDC says "7 or more" for ages 18–60. (2) 1.2.0: the "why" gave CDC a reason the page does not give. (3) 1.2.0: the summary's body-rhythm rationale is not on the page | (1) Cite Keslan as well, or use CDC's figure. (2) **Applied in version 2**: the "because" clause is dropped. (3) Label it as general advice, reword or cite a source |
| `khawatir-soal-tubuh-dan-kesehatan`, `bersikap-baik-pada-diri-sendiri`, `kesepian-langkah-kecil-terhubung` (1.2.0) | The "why" credited WHO with a statement its source does not make, or (loneliness) stated an association as an effect | **Applied in version 2** (see [Migration](#migration-of-the-30-dengarin-cards)). The editor confirms or reverts |
| `kamar-dan-layar-sebelum-tidur` (1.2.0) | The "why" puts "keep the body alert" next to "basic advice from both the CDC and Indonesia's Ministry of Health"; neither source gives that explanation | "Turning screens off, skipping heavy meals and alcohol before bed, and avoiding caffeine later in the day are on CDC's list of sleep habits." |
| `tidur-cukup-tujuh-sampai-sembilan-jam` (1.2.0) | The summary says Kemenkes advises; the source is a hospital news item quoting a doctor, published on the Ditjen Keslan site | "An article on the Ministry of Health's Ditjen Kesehatan Lanjutan site gives adults 7 to 9 hours of sleep, with a calming routine before bed." |
| 3 Ayo Sehat cards | Source unreachable on 2026-10-26; the verbatim check could not run | Re-verify and run the verbatim check before 2026-11-26 |
| `rencana-keselamatan-enam-langkah` (2 flags, content set 1.1.0) | Titled "six steps" but listed five (v1 step 3 merged the original steps 3 and 4). Its "why" also said the structure is widely used by crisis services in many countries, which nothing that was read supports | **Applied in version 2**: step 3 split into two; the unsourced clause dropped. The editor confirms or reverts |
| `masalah-uang-enam-langkah` (1.1.0) | Step-count check: titled "the six PM+ steps" but lists five; step 2 combines PM+'s "choose" and "define" (PDF p. 53) | Retitle: "Masalah uang: langkah-langkah PM+" / "Money problems: the PM+ steps". An exercise has at most 5 steps, so splitting is not proposed |
| `lepas-kaitan-sadari-namai-fokus` (1.1.0) | Step-count check: the summary says three steps, but the card lists four (DWMM's three plus a "repeat" step from the v1 team) | "Sadari, namai, fokus kembali, lalu ulangi: …" / "Notice, name, refocus, then repeat: …" |

The 8 new guides have **no gate hits**, and since content set 1.2.0 no item has one. There are **17 open editor flags** in total: 8 concern the 7 changed cards, where the editor confirms or reverts the change (the six-step safety plan has 2); 5 propose a fix that was not applied; 3 mark the unreachable Ayo Sehat pages; and 1 marks the CDC sleep-duration figure.

### Source claims: what a named source says

A review of content set 1.1.0 found that some cards' "why" credited WHO or CDC with things the cited source does not say, and the source-fidelity spot-checks had not caught them. Since 1.2.0 every `summary` or `why` that names a source carries a `source_claims` entry (18 entries in 14 items), and the validator fails if one is missing.

- **What an entry holds.** The source ids, a locator (PDF page or page section), `source_says`, a `support` verdict and a note on any difference. `source_says` is Sela's own paraphrase of the cited passage, written so that an editor can compare the card with the source without opening it. It is quote-free: it has no quotation marks, and the verbatim tool found no run of 5 or more words shared with the source (longest: 4).
- **Results (machine draft, unreviewed).** 12 `supported`, all after the 1.2.0 rewording; 2 `partly_supported` (`kamar-dan-layar-sebelum-tidur` "why", `tidur-cukup-tujuh-sampai-sembilan-jam` summary), each with an open editor flag; 4 `not_checked` (the Ayo Sehat cards), each with the open `source_unreachable` flag.
- **Who confirms.** Every entry is `editor_confirmation.status: "pending"`. Only a human editor confirms one, for the exact current text (`confirmed_text_sha256`), and only on an item whose review status is no longer `machine_drafted_unreviewed`. A later text change withdraws the confirmation. An item is publishable only when all its entries are `supported` and confirmed.

### Source fidelity

- **Spot-checked against the source pages (15 cards, plus the "why" or summary of every card in `source_claims`).** Examples: slow breathing against PM+ Chapter 6, grounding against the DWMM Tool 1 summary, the loneliness figures against the report (PDF pp. 18 and 44), the sleep cards against CDC and Keslan, and the two safety-plan cards against the SPRC quick guide (see below).
- **Found during the spot-check.** The grounding card tells the reader to breathe as usual, which is gentler than the guide's own instruction to slow the breath and empty the lungs fully (DWMM PDF p. 40). This is an adaptation choice for the editor to confirm.
- **Not compared (the other 15 cards).** Their `source_fidelity` says so. The editor compares them with the cited pages.

#### Safety-plan structure, checked against the SPRC quick guide

Until version 1.1.0, both safety-plan cards were `not_checked`, because the 2012 article is paywalled. Yet their `evidence_note` already named the free SPRC quick guide (Stanley & Brown 2008, published by WICHE and SPRC), which needs no paywall. On 2026-10-26 the guide was read, and the cards were compared with it.

- **The six steps (PDF p. 2):**
  1. warning signs;
  2. internal coping strategies;
  3. social contacts and settings that distract;
  4. family or friends who may help, whom the person, unlike in step 3, tells about the crisis;
  5. professionals and agencies;
  6. making the environment safe.

  Each step is used when the one before has not resolved the crisis.
- **`rencana-keselamatan-enam-langkah`.** The v1 card listed five steps under the title "six steps": its step 3 covered both steps 3 and 4. Version 2 splits them in Sela's own words:
  - step 3: people and places that distract;
  - step 4: people you can ask for help, whom you tell that you are struggling.

  The v1 "why" also said that the structure "is widely used by crisis services in many countries". Neither the guide nor anything else that was read says this, so version 2 drops the clause. Re-adding it needs a cited source.

  Details left to the editor, all in `source_fidelity`:
  - step 1's "situations" is not in the guide's list of warning signs;
  - "struggling" is gentler than telling someone you are in crisis;
  - "prepared while calm" is not stated in the guide.
- **`rencana-keselamatan-cara-menggunakan`.** Consistent with the guide: build the plan with a clinician, use the steps in order, decide where the plan is kept, and review it when circumstances change (PDF pp. 1–2). The text is unchanged. Its `source_fidelity` lists the wording the guide does not state ("while calm", "someone you trust", "every few months", "works best").
- **Rights.** The guide says that no part of it may be reproduced without the authors' written permission. The cards describe the structure only, and the repository holds locators and the PDF's SHA-256, not its text.

### Evidence boundaries

- **No adult evidence for teens.** `kerahasiaan-usia-16-17` cites only an adolescent-specific WHO source (Statement 3.2, PDF p. 43) and Sela's own design. The validator rejects an `all` or `15-17` item that adapts or paraphrases an adult-only source.
- **No correlation presented as causation.** The causal-benefit gate exists for this, and both flagged cards are examples of it.
- **No benefit claims without evidence.** The journaling guide says plainly that Sela does not promise writing helps, because the research register holds no journaling evidence (D-048 gap).
- **Product statements describe planned behaviour.** They come from the v3 SPEC, the PM decisions and the design screens on `feat/sela-v3-pilot` (working copy of 2026-10-26). Each carries `implementation_check: "pending"` and must be checked against the built pilot before publishing.
- **Legal wording.** Two paragraphs use wording from the minors design about operator access and legal disclosure. They carry `pending_review` for B-10.

## Review status and publishing rule

Every item starts and stays `review.status: "machine_drafted_unreviewed"` with `publishable: false` until a human editor reviews it. The validator allows `publishable: true` only when all of these hold:

1. `status` is `human_approved`, with `human_editor` and `reviewed_on` set;
2. `approved_text_sha256` equals the current `text_sha256`, so any later text change withdraws the approval;
3. the claims result is `pass` and no `editor_flag` is open;
4. the authoring-time verifier verdict is `supported` (V36);
5. for teen content, a human teen review is recorded (`teen_review`). Only `human_approved` items may carry `reviewed_suitable`;
6. every `source_claims` entry is `supported` and confirmed by the editor for the current text;
7. the verbatim check of every source the item adapts or paraphrases has run.

The editor also checks every product statement against the implementation (`implementation_check`), every cited locator, and the Indonesian text with readers.

## How to add or change an item

1. Choose a source from an official, approved host. The validator enforces these hosts: who.int, iris.who.int, cdc.gov, the kemkes.go.id sites, healing119.id, w3.org, sprc.org (the authorised open copy of the Stanley & Brown quick guide), and doi.org for article identifiers.
2. Read its licence on the document itself. Do not use a mirror, and do not bypass a paywall. Add the source to `sources.json` with what you read, the date and a re-verify-by date.
3. Write the item in both languages, in the "kamu" register, using the product glossary in `docs/design/CONTENT_STYLE.md` section 4 (on `feat/sela-v3-pilot`):
   - one idea per sentence, at most 20 words;
   - no quotations longer than 15 words from any source;
   - no personal data and no one's story;
   - no diagnosis, cure, treatment, efficacy, guarantee or medication wording;
   - no day counters;
   - 2–5 steps for an exercise and 2–6 for a safety plan. If the title or summary says how many steps there are, it must match.
4. Give the item the right audience. Use `all` or `15-17` only when no adult-only source is adapted or paraphrased. Give every guide paragraph its basis, and label general advice.
5. Material under CC BY-NC-SA goes in `cards_who.json`, or in a new file under the same licence that is covered by `LICENSE.md`. It needs:
   - the attribution, with the source's IRIS link and the licence URI in parentheses;
   - the adaptation notice;
   - `share_alike: true`;
   - `commercial_use: "blocked"`.

   If you add a WHO source, add its suggested citation and link to `LICENSE.md`.
6. Declare every phone number or web address under `contacts`. It must match the help catalogue.
7. If a `summary` or `why` names a source, say only what that source says, and add a `source_claims` entry: the source ids, the locator, a paraphrase of the passage in your own words, its `quote_check` (run `tests/content/verbatim_overlap.py --text "..." --source <local copy>`; under 5 shared words), the support verdict, and `editor_confirmation.status: "pending"`. A reason the source does not give is either dropped or written as the card's own sentence, without the source's name. Anything but `supported` needs an open editor flag.
8. For a source whose text is adapted or paraphrased, run `tests/content/verbatim_overlap.py` against a legitimate copy (the publisher's page or PDF, or an archived copy) and record the result in the source's `verbatim_check`: the copy's URL, date and SHA-256, the longest run and every run of 5 or more words with its locator. Never commit the copy or the matched words. If the check cannot run, record the attempts and a due date, and do not write "no text copied" without "not yet verified".
9. Run `python3 tests/content/reading_validator.py --update`, then the tests. Add an `editor_flag` for every claim-gate hit you cannot reword away.
10. Leave `review` as `machine_drafted_unreviewed` with `publishable: false`. Only the human editor changes it.
11. To change a migrated card's text:
   - set `origin.text_changed: true`;
   - bump `version`;
   - keep `origin.v1_text_sha256` as the record of the original;
   - record in `adaptation_note` what changed "in version N" and why;
   - open an editor flag, so that the human editor confirms or reverts the change.

   The validator enforces all five.
12. A schema change, or any change to the set, bumps the content set version (`set.json`) and adds an entry to `set.json` `changes`. A frozen content set is never rewritten silently.

## Known limitations

- **The checks are approximate.** The plain-language metrics are heuristics: syllable counts are approximate, and semicolon-joined clauses count as one sentence. The gates miss paraphrases, and an unrelated negation within six tokens can hide a claim (the blind spot documented for `sela-claim-gates-v2`).
- **Partial checks.** Source fidelity was spot-checked for 15 cards only. The English text of the migrated cards is the v1 team's translation. The verbatim-overlap check is a stored result: without the PDFs, the validator cannot re-run it.
- **Step-count check.** It reads number words from 2 to 10 and digits before "steps" or "langkah" in titles and summaries only. It misses counts written any other way.
- **Link to the help catalogue.** The contact check reads `web/src/data/help.fallback.json` and skips itself if that file moves. The catalogue is owned by the help-directory work, not by this library.
- **Planned behaviour.** The guides describe planned v3 behaviour that is not yet implemented and verified.
- **Source claims are machine drafts.** The paraphrases and support verdicts were written by an AI agent from the source pages on 2026-10-26. Names are detected by a word list (WHO, CDC, Kemenkes, Ministry of Health, Kementerian Kesehatan, Stanley, "health agencies" / "lembaga kesehatan"), in summaries and "why" only; an attribution phrased any other way, or in a step, is missed. Steps were not given source claims.
- **Stored, not recomputed.** The verbatim results and the quote checks are stored results; the source copies are not committed, so the validator checks their consistency but cannot re-run them.
- **Repository licence scan.** It matches runs of 8 words, so a shorter excerpt or a light paraphrase of a WHO card is not found. It checks the notice per file, not per entry. It cannot read binary or compressed files, such as `tests/carebridge/dataset/self_help_cards.v1.pb.zst`, which holds the v1 card text. It scans what git tracks or would track, so ignored files are skipped.

## Version history

The machine-readable history is `set.json` `changes`.

- **1.0.0 (2026-10-26).** First draft: 30 migrated Dengar.in v1 cards, with text unchanged, and 8 new Sela guides.
- **1.1.0 (2026-10-26), after review findings.**
  - **Licence.**
    - Added `data/content/reading/LICENSE.md`.
    - Every WHO card's attribution now gives its source's IRIS link and the licence URI.
    - `cards_who.json` `file_licence.notice` gives the URI and points to `LICENSE.md`.
    - The validator requires all three.
  - **Safety plan.**
    - `rencana-keselamatan-enam-langkah` version 2 has six steps; the unsourced "widely used" clause is dropped.
    - New source `sprc-safety-planning-guide-2008` (source kind `practice_guide`, host sprc.org).
    - Both safety-plan cards were spot-checked against the SPRC guide.
  - **Step counts.**
    - Safety plans may have 2–6 steps.
    - A stated step count must match the steps.
    - New editor flags on `masalah-uang-enam-langkah` and `lepas-kaitan-sadari-namai-fokus`.
  - **Verbatim WHO text.** It is now recorded accurately (section [Sources and verification](#sources-and-verification-checked-2026-10-26)).
  - **Schema.** `set.json` gains `changes`, and sources gain the kind `practice_guide`.

  The fingerprint changed only because `rencana-keselamatan-enam-langkah` changed (new version and text hash). The attributions are outside the hashed `content`. The research corpus and its frozen 2026-10-26 snapshot are not touched.
- **1.2.0 (2026-10-26), after review findings.**
  - **Source support of the "why".** Six migrated cards reworded to what their source says, each as version 2 with an open confirm-or-revert flag (see [Migration](#migration-of-the-30-dengarin-cards)). Three more open flags where a named source only partly supports the text.
  - **Source claims.** Every summary or "why" that names a source carries a locator, a quote-free paraphrase and a support verdict, pending an editor's confirmation (18 entries). The validator requires them and blocks publishing until each is supported and confirmed.
  - **Verbatim checks.** A committed tool, `tests/content/verbatim_overlap.py`, and a stored `verbatim_check` for every adapted or paraphrased source (and the SPRC guide). Keslan, CDC, SPRC and the four WHO PDFs were checked; the two Ayo Sehat pages could not be, so their "no text copied" now says "not yet verified" (due by 2026-11-26).
  - **Guide wording.** `butuh-bantuan-sekarang` version 2: one English sentence reworded because it shared 6 words with DWMM.
  - **Repository licence scan.** The validator fails when a repository file holds CC BY-NC-SA card text without the licence URI, the IRIS link and the adaptation notice. It fails today on `packages/config/src/selfHelpCards.v1.json` (open point 2).
  - **Schema.** Items gain `source_claims`; sources gain `verbatim_check`; editor flags gain the issues `attribution_not_in_source` and `causation_overstated`.

  The fingerprint changed because 7 items changed (new versions and text hashes). The research corpus and its frozen 2026-10-26 snapshot are not touched.

  **ID renumbering (2026-10-26, same version).** After main was merged, the `SPEC:` and `DECISION:` basis references inside item `content` and the IDs in this document, `LICENSE.md` and the validator were renumbered to the repository's IDs (v3 C15–C25 → C16–C26, V24–V40 → V26–V42, decisions D-025–D-049 → D-026–D-050). No reader-facing sentence changed. The text hashes and the fingerprint changed only because the references sit inside the hashed `content`; no item was approved, so no approval is invalidated.

## Open points for the PM and other owners

1. **CI (workflow owner).** No workflow runs `tests/content` or the validator, so a change to the content gates, the WHO notices, the audience fields or the help catalogue can merge without the 79 tests. The validator step below **fails until open point 2 is resolved**, because the repository licence scan fails on `packages/config/src/selfHelpCards.v1.json`; that is intended. The unit tests pass, because they accept that one known failure and no other. This library does not own `.github/workflows/`. A proposed `.github/workflows/reading-library.yml`, following the conventions of `research-corpus.yml` (same action versions, read-only permissions, offline):

   ```yaml
   name: Sela reading library

   # Offline checks for data/content/reading/ (no network): unit tests and the validator.
   # A weekly run lists sources and contacts past their re-verify-by date.
   on:
     pull_request:
       paths:
         - 'data/content/**'
         - 'tests/content/**'
         - 'data/research/corpus/eval_fixtures/claim_gates.json'
         - 'web/src/data/help.fallback.json'
         - 'packages/config/src/selfHelpCards.v1.json'
         - '.github/workflows/reading-library.yml'
     push:
       branches: [main]
       paths:
         - 'data/content/**'
         - 'tests/content/**'
         - 'data/research/corpus/eval_fixtures/claim_gates.json'
         - 'web/src/data/help.fallback.json'
         - 'packages/config/src/selfHelpCards.v1.json'
     schedule:
       - cron: '17 6 * * 1'
     workflow_dispatch:

   permissions:
     contents: read

   concurrency:
     group: reading-library-${{ github.ref }}
     cancel-in-progress: true

   jobs:
     offline:
       name: Content tests and validator (Python ${{ matrix.python }})
       runs-on: ubuntu-latest
       timeout-minutes: 10
       strategy:
         fail-fast: false
         matrix:
           python: ['3.10', '3.12']
       steps:
         - uses: actions/checkout@v7
         - uses: actions/setup-python@v7
           with:
             python-version: ${{ matrix.python }}
         - name: Unit tests (no network)
           run: python3 -m unittest discover -s tests/content -p 'test_*.py'
         - name: Validate the library
           run: python3 tests/content/reading_validator.py

     stale:
       name: Sources and contacts past their re-verify-by date
       if: github.event_name == 'schedule' || github.event_name == 'workflow_dispatch'
       runs-on: ubuntu-latest
       timeout-minutes: 5
       steps:
         - uses: actions/checkout@v7
         - uses: actions/setup-python@v7
           with:
             python-version: '3.12'
         - run: python3 tests/content/reading_validator.py --check-stale
   ```

   `packages/config/src/selfHelpCards.v1.json` is in the path filter because the validator compares every migrated card with it and scans it for WHO card text. The licence scan reads the whole repository, so a path-filtered run can miss a new copy of the card text elsewhere; the weekly run catches it. The `stale` job will start failing after 2026-11-26 until the Ayo Sehat pages are re-verified and their verbatim checks have run, which is intended.
2. **Licence carve-out (owner) and the v1 card file (packages/config owner).** Status on 2026-10-26: **not resolved; the content validator now fails on it.** The root `LICENSE` grants MIT, and the README shows an MIT badge. Anyone who reads only those files is told they have commercial rights in the WHO card text, which WHO never granted. `data/content/reading/LICENSE.md` now gives the correct notice for that directory, but it cannot carve the text out of the root notice, and it cannot cover `packages/config/src/selfHelpCards.v1.json`, which holds the same 22 adaptations. Proposed text, for the owner to add:

   - At the end of `LICENSE`:

     ```text
     Exception: text licensed under CC BY-NC-SA 3.0 IGO

     The MIT licence above does not apply to the self-help card text adapted from
     World Health Organization publications. That text is licensed under the
     Creative Commons Attribution-NonCommercial-ShareAlike 3.0 IGO licence
     (https://creativecommons.org/licenses/by-nc-sa/3.0/igo):

     - data/content/reading/cards_who.json (see data/content/reading/LICENSE.md);
     - the 22 entries of packages/config/src/selfHelpCards.v1.json whose
       source.license is "CC BY-NC-SA 3.0 IGO".

     It may be used for non-commercial purposes only, with its attribution and
     adaptation notice. WHO does not endorse this project. Commercial use is
     blocked until permission is obtained or the text is replaced (B-14).
     ```

   - In `README.md`, under "Provenance and license": "Exception: the WHO-adapted self-help cards (`data/content/reading/cards_who.json` and 22 entries of `packages/config/src/selfHelpCards.v1.json`) are CC BY-NC-SA 3.0 IGO, not MIT, and non-commercial only until B-14 clears; see [data/content/reading/LICENSE.md](data/content/reading/LICENSE.md)." Consider changing the badge to "MIT (code); some content CC BY-NC-SA 3.0 IGO".

   The carve-out alone does not make the v1 file compliant, and this library cannot change files outside `data/content`, `tests/content` and `docs/content`. The owner of `packages/config` chooses one of these:

   - **(a) Add the notice to the v1 file.** Give each of the 22 CC BY-NC-SA entries an attribution with the licence URI and its source's IRIS link, plus the adaptation notice, in the same words as the matching card's `attribution` and `adaptation_notice` in `cards_who.json`, as new fields; leave `source`, `evidenceNote` and the text fields unchanged, because the library pins them. Expose the new fields through `selfHelpAttribution()`. This changes the file's bytes, so run `python3 tests/content/reading_validator.py --update` afterwards to refresh `set.json` `v1_baseline.sha256`, and record the new commit there.
   - **(b) Stop loading the v1 text.** Make `packages/config/src/selfHelpCards.ts` read `data/content/reading/cards_who.json` (and the other card files), which already carry the notices, and remove the v1 file, or keep it out of every build and export. The library's migration tests skip themselves when the v1 file is absent; `origin.v1_text_sha256` still records the v1 text.

   Either way, `tests/carebridge/dataset/self_help_cards.v1.pb.zst` holds the same text in compressed form, which the scan cannot read; it needs the same decision. Once the scan passes, the validator exits 0.
3. Legal confirmation for commercial use of the Kemenkes paraphrases and the safety-plan structure: 7 items, separate from B-14. Decide whether to record it as a new blocker.
4. A human editor should:
   - resolve the 17 open editor flags, including confirming or reverting the 7 changed cards;
   - confirm, correct or reject the 18 `source_claims` entries against the cited pages (each records its locator);
   - run the verbatim check of the two Ayo Sehat pages before 2026-11-26 (live page, or the Internet Archive snapshot if the site is still down), and record it in `sources.json`;
   - split the long sentences in 28 migrated cards;
   - re-verify the Ayo Sehat pages after maintenance (by 2026-11-26);
   - check every product statement against the implementation;
   - run the V36 verifier once B-8 clears.
5. Teen review: decide which exercise cards, if any, may be offered to 16–17, and record a human `teen_review`. No Indonesian source on adolescent confidentiality in health services has been checked yet (D-048 gap).
6. Healing119 is the only contact in the content. Emergency numbers, such as PSC 119 for medical emergencies, belong in the help catalogue after verification there, not in reading items.
