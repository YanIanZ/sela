# Licence notice for `data/content/reading/`

**The card text in [`cards_who.json`](cards_who.json) is not under the repository's MIT licence.** It adapts four World Health Organization (WHO) publications and is licensed under the **Creative Commons Attribution-NonCommercial-ShareAlike 3.0 IGO** licence (**CC BY-NC-SA 3.0 IGO**):

- Licence: <https://creativecommons.org/licenses/by-nc-sa/3.0/igo>
- Legal code: <https://creativecommons.org/licenses/by-nc-sa/3.0/igo/legalcode>

The [MIT licence](../../../LICENSE) at the repository root does not apply to this text, and it grants no commercial rights in it, because WHO never granted any. To copy, publish, translate or ship `cards_who.json`, or any card text from it, you must use CC BY-NC-SA 3.0 IGO and follow the terms below. Use is **non-commercial only**: a commercial launch is blocked until **B-14** is resolved (PM decision **D-045**).

The licence is WHO's. This notice only explains how this library meets it. It was written on 2026-10-26 for content set `sela-reading-v1`, version 1.1.0, and revised the same day for version 1.2.0. It is not legal advice; the legal review under B-10 still applies.

## Which files this covers

| File | Licence of its text |
| --- | --- |
| `cards_who.json` (22 cards) | **CC BY-NC-SA 3.0 IGO.** The cards are adaptations and translations of WHO works by the Dengar.in and Sela team. |
| `internal/fixtures/data/self_help_cards.v1.json` (outside this directory) | Its 22 entries whose `source.license` is CC BY-NC-SA 3.0 IGO are the same adaptations in their Dengar.in v1 form, so the same licence applies to them. A notice in this directory cannot exempt that file from the root MIT notice. Adding a carve-out to the root `LICENSE` and `README.md` is an owner action (open point 2 in `docs/content/READING_LIBRARY.md`). **The validator fails until that file carries the licence URI, the IRIS links and the adaptation notice, or no longer holds the text** (repository licence scan, below). The compressed pack built from it, `tests/carebridge/dataset/self_help_cards.v1.pb.zst`, holds the same text; the scan cannot read compressed files, so that pack needs the same decision by its owner. |
| `cards_kemenkes.json` | The Dengar.in team's paraphrase, under the repository's MIT licence. The Kemenkes pages state no licence. Attribute Kemenkes, and get legal confirmation before a public or commercial release. The v1 team's statement that no text was copied is checked by machine for the Keslan page (longest shared run 3 words) and is **not yet verified** for the two Ayo Sehat pages, which were unreachable on 2026-10-26 (`sources.json`, `verbatim_check`, due by 2026-11-26). |
| `cards_cdc.json` | The Dengar.in team's adaptation, under the repository's MIT licence. CDC "About Sleep" is a US public-domain work; attribute CDC and imply no endorsement. One card also paraphrases Kemenkes Keslan, so it needs the same legal confirmation. |
| `cards_safety_plan.json` | The Dengar.in and Sela team's own description, under the repository's MIT licence. It describes the structure only; the Stanley & Brown form and quick guide are copyright Stanley & Brown and are not reproduced. Get legal confirmation before commercial use. |
| `guides_sela.json`, `reading_item.schema.json`, `set.json`, `content_gates.json`, `sources.json`, this file | Sela's own text, under the repository's MIT licence. `sources.json` and this file reproduce WHO's suggested citations and WHO's translation disclaimer, because WHO asks adapters to show them. |

## The WHO works adapted

Each licence statement was read on the copyright page of the English PDF retrieved from WHO IRIS on 2026-10-26, and matches IRIS `dc.rights`. Every IRIS link answered HTTP 200 on 2026-10-26.

| Source id (`sources.json`) | Cards | WHO's suggested citation (verbatim) | Link |
| --- | --- | --- | --- |
| `who-dwmm-2020` | 10 | Doing what matters in times of stress: an illustrated guide. Geneva: World Health Organization; 2020. Licence: CC BY-NC-SA 3.0 IGO. | <https://iris.who.int/handle/10665/331901> |
| `who-pmplus-2018` | 8 | Problem Management Plus (PM+): individual psychological help for adults impaired by distress in communities exposed to adversity (generic field-trial version 1.1). Geneva: World Health Organization; 2018. Licence: CC BY-NC-SA 3.0 IGO. | <https://iris.who.int/handle/10665/375604> |
| `who-mh-work-2022` | 3 | WHO guidelines on mental health at work. Geneva: World Health Organization; 2022. Licence: CC BY-NC-SA 3.0 IGO. | <https://iris.who.int/handle/10665/363177> |
| `who-loneliness-2025` | 1 | From loneliness to social connection - charting a path to healthier societies: report of the WHO Commission on Social Connection. Geneva: World Health Organization; 2025. Licence: CC BY-NC-SA 3.0 IGO. | <https://iris.who.int/handle/10665/381746> |

WHO's translation disclaimer, which WHO asks to be added to any translation together with the suggested citation (verbatim, identical in all four works):

> This translation was not created by the World Health Organization (WHO). WHO is not responsible for the content or accuracy of this translation. The original English edition shall be the binding and authentic edition

## What the licence and WHO require, and how this library meets it

The "Where" column gives the licence's legal code section or WHO's copyright page; the terms are summarised in our own words.

| Requirement | Where | How this library meets it |
| --- | --- | --- |
| **Non-commercial use only** | Legal code 4(c); WHO copyright page | Each card and source records `commercial_use: "blocked"` and `blocker: "B-14"`. A commercial build must drop `cards_who.json` (D-045). |
| **Cite the work**: title, WHO's attribution, and a credit that shows the card is adapted from it | Legal code 4(d); WHO copyright page ("Suggested citation") | Each card's `attribution` (Indonesian and English) starts "Diadaptasi dari:" / "Adapted from:", followed by WHO's citation and the part used. |
| **Give the licence URI with every copy of each adaptation** | Legal code 4(a) and 4(b)(I) | Each card's `attribution` contains <https://creativecommons.org/licenses/by-nc-sa/3.0/igo>, and so do `file_licence.notice` and this file. |
| **Give the URI associated with the work**, as far as practicable | Legal code 4(d)(iii) | Each card's `attribution` links its source's WHO IRIS record, which carries the licence information. |
| **Share-alike**: license the adaptation under the same or an equivalent licence | Legal code 4(b); WHO copyright page | Each card records `item_text_licence: "CC BY-NC-SA 3.0 IGO"` and `share_alike: true`, and this file licenses the cards that way. |
| **Translation disclaimer** | WHO copyright page | Each card's `adaptation_notice` (Indonesian and English) says that the Dengar.in and Sela team, not WHO, made the adaptation and translation; that WHO is not responsible for its content or accuracy and does not endorse Sela; that the original English edition is the binding and authentic edition; and that the adaptation is licensed CC BY-NC-SA 3.0 IGO. WHO's own wording is quoted above and kept in `sources.json` (`translation_disclaimer_verbatim`). |
| **No suggestion that WHO endorses** Sela or any organisation, product or service | WHO copyright page; legal code 4(d) | The `adaptation_notice` says WHO does not endorse Sela. No wording, layout or credit may imply a connection with WHO. |
| **No WHO logo** | WHO copyright page | Sela never shows the WHO logo or emblem with these cards. |
| **Keep the notices intact, add no restricting terms or technical measures** | Legal code 4(b)(II)–(IV) | The app shows `attribution` and `adaptation_notice` with the card text in every view and in every export ("Sumber & lisensi", design screen S20). No term of service or technical measure may stop a recipient from using the card text under the licence. |
| **Disputes** go to WIPO mediation | WHO copyright page | Noted; no action in the product. |
| **No warranty**: WHO offers the works as is | Legal code 5; WHO copyright page (general disclaimers) | The cards are not reviewed health advice. Every card stays `machine_drafted_unreviewed` and unpublishable until a human editor approves it. |

`tests/content/reading_validator.py` enforces the parts that can be checked in data. Each WHO card must carry:

- the licence;
- the attribution with the licence URI and its source's IRIS link;
- the adaptation notice;
- share-alike;
- the B-14 blocker.

The validator also requires this file, its suggested citations, the translation disclaimer and the links above.

**Repository licence scan (content set 1.2.0).** The validator also reads every text file in the repository that git tracks or would track (binary files and files over 32 MB are counted, not read). A file that shares a run of 8 words with the current or v1 text of any card in `cards_who.json` must contain:

- the licence URI <https://creativecommons.org/licenses/by-nc-sa/3.0/igo>;
- the IRIS link of the source of every card whose text it holds;
- the adaptation notice ("WHO" and "not responsible" or "tidak bertanggung jawab").

Otherwise the validator fails and names the file. On 2026-10-26 the scan found card text in two files: `cards_who.json`, which passes, and `internal/fixtures/data/self_help_cards.v1.json`, which fails. The check is per file, not per entry.

## Commercial use: blocked (B-14)

B-14 ("WHO card licence for commercial use") clears in one of two ways:

- **(a)** WHO grants permission for the intended commercial use, through the licensing route linked from each PDF's copyright page;
- **(b)** the 22 cards are replaced by content that Sela owns or licenses for commercial use.

Until then, the non-commercial pilot may use the cards only with their attribution and notice (D-045).

## WHO's own wording in `cards_who.json`

The cards are the Dengar.in v1 team's adaptation, not WHO's text. A machine verbatim check run on 2026-10-26 (`sela-verbatim-overlap-v1`, unreviewed) compared each card's English text with its source PDF. It was re-run for content set 1.2.0 with `tests/content/verbatim_overlap.py`, which reproduces the first result exactly, and the result is now stored per source in `sources.json` (`verbatim_check`, counts and locators only). It found the following:

- **One full sentence and one example.** `lepas-kaitan-sadari-namai-fokus`:
  - step 1 (English) is WHO's sentence word for word;
  - the second example in step 2 (English) is one of WHO's naming examples word for word.

  Both are from DWMM, Summary, Tool 2, PDF p. 125. The card's `language_notes.en` and `adaptation_note` say so.
- **Five other cards share an everyday phrase of five or six words** with their source and nothing longer. The list is in `docs/content/READING_LIBRARY.md`.
- **One title.** Version 2 of `gerak-tubuh-setelah-kerja` names its source by title ("WHO's guidelines on mental health at work"), a run of 6 words with PDF p. 1. In content set 1.1.0 this card's 'why' shared a 5-word phrase with PDF p. 64 instead; that sentence was reworded because it misstated the guideline.

The Indonesian text is a translation by the v1 team; IRIS lists no Indonesian edition of these works. Reusing WHO's wording is permitted under the licence, because `cards_who.json` is itself licensed CC BY-NC-SA 3.0 IGO.

No images, illustrations, figures or tables from the WHO works are used. The caution about third-party material on WHO's copyright pages therefore does not arise.

## Changing or adding WHO material

Material adapted from a CC BY-NC-SA source goes in `cards_who.json`, or in a new file covered by this notice. The new material needs:

- the attribution, including the licence URI and the source link;
- the adaptation notice;
- `share_alike: true`;
- `commercial_use: "blocked"`.

Update the table above and run `python3 tests/content/reading_validator.py`. See `docs/content/READING_LIBRARY.md`, "How to add or change an item".
