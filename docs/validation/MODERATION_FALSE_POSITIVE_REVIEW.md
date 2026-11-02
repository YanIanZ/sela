# Community moderation: evaluation set and false-positive review (S-036)

Date: 2026-11-02. Status: engineering evaluation on **synthetic** text only. Every case was written by one author (the AI engineer) for this evaluation; no real post was used. It does not measure real-user moderation quality, and it says nothing about clinical safety.

## What was evaluated

The community publishing pipeline of D-072/S-031, exactly as `internal/httpapi/v3_community.go` runs it:

1. `community.Prepare`: the safety router (`internal/safety`). A crisis text returns `422 crisis` and quick-help mode; it is never stored.
2. `community.Rules`: deterministic rules (insults and contact details). A match is hidden, and Meta never sees it.
3. `community.MetaModerator`: Meta labels the text. Any harm label hides it; no label publishes it; an unavailable or out-of-schema answer holds it for an operator.

Two configurations were scored:

- **rules**: steps 1–2 only (offline, deterministic). Text the rules let through counts as published, because in production Meta would read it next.
- **full (live)**: steps 1–3 with the Meta Model API (`meta_api`, `muse-spark-1.3`, `reasoning_effort=minimal`, endpoint `api.meta.ai`), with the production prompt and schema unchanged (prompt sha256 `30b64feb…577c`).

## Dataset

`eval/moderation/cases.jsonl`: 222 cases (sha256 `e4cf3ba0e2ff8a92fd69fbdd81d240dcb7d9fdd6d917222f3243331dce355b13`), split by paraphrase family. A family never spans two splits, and splits were assigned by family position before any scoring.

| Stratum | Expected | dev | tune | heldout | Total |
| --- | --- | --- | --- | --- | --- |
| bullying (aimed at a person) | hide | 12 | 9 | 12 | 33 |
| harassment (exposure, threats, sexual remarks, hate, "go die", stalking) | hide | 8 | 6 | 6 | 20 |
| spam_contact (phones, links, emails, handles, doxxing, sales) | hide | 7 | 5 | 5 | 17 |
| banter (friendly teasing) | publish | 7 | 7 | 7 | 21 |
| quote_support (quoting an insult to ask for support) | publish | 8 | 7 | 9 | 24 |
| reclaimed_self (self-description, reclaimed words) | publish | 8 | 5 | 8 | 21 |
| venting_profanity (profanity not aimed at anyone present) | publish | 9 | 5 | 7 | 21 |
| negated_reported (negated or reported speech, talk about words) | publish | 9 | 5 | 8 | 22 |
| supportive_reply | publish | 9 | 5 | 7 | 21 |
| crisis (disclosures that must route to crisis) | crisis | 8 | 7 | 7 | 22 |
| **Total** | | **85** | **61** | **76** | **222** |

Languages: Indonesian 123 (formal and slang registers are marked in `register`), English 83, mixed 16. Replies carry the post they answer in `context`, which Meta receives as the API sends it. `hold` is allowed in the schema, but no case expects it: hold only happens when Meta is unavailable or unclear.

**Definitions** (pre-registered in `eval/thresholds.json`, version `sela-eval-thresholds-2026.11.02-2` — registered as `-1` and renamed on merge, unchanged; result files written before the merge carry the old name — before held-out was scored):

- **Benign false positive:** a publish case that ends hidden, held or crisis-routed.
- **Harmful false negative:** a hide case that ends published.
- **Crisis routing recall:** crisis cases that `Prepare` stops with `ErrCrisis`.

## Protocol and runs

The harness is `cmd/sela-eval`: `-mode moderation` for rules, and `-mode moderation_live -splits … -max-calls N` for the full pipeline. The offline suite also runs the rules step. Live runs count every provider HTTP request, retries included. The harness refuses requests beyond `-max-calls`, and it stops before a case could need more requests than remain.

| Run | Config | Splits | Provider calls | Files (`eval/results/`, metadata only) |
| --- | --- | --- | --- | --- |
| Rules before | rules `e41f0e8f…`, safety `2026.11.01-2` | all | 0 | `20261102T012959Z-moderation*` |
| Smoke test | after | 1 dev case | 1 | scratch only, not kept |
| Live dev/tune | rules after, safety `2026.11.02-1`, prompt unchanged | dev, tune | 114 | `20261102T014003Z-moderation_live*` |
| Rules after (final) | rules `aa82f39b…`, safety `2026.11.02-1` | all | 0 | `20261102T014331Z-moderation*` |
| Live held-out (single final run) | as live dev/tune | heldout | 61 | `20261102T014338Z-moderation_live*` |

Total provider calls: **176** of the 800 cap, with no retries recorded and no held cases.

The rules were fixed using dev and tune cases only. The held-out failure lists were not read until both final runs were done.

**Meta prompt:** not changed. Meta made zero false positives and zero false negatives on dev and tune, so no dev/tune evidence supported a prompt change.

**How "full before" was derived:** the old rules first, then the Meta verdict from the live runs for every case the old rules let through. The pipeline is sequential and the prompt is unchanged, so Meta's verdict on a case does not depend on which rules ran first. Two dev crisis cases (CRI04-1, CRI07-2) were routed by the new safety ruleset in the live run, so they have no recorded Meta verdict. They count as crisis misses in "before" whatever Meta would have said.

## Results (95% Wilson intervals)

### Held-out (judged against the thresholds)

| Metric | Before | After | Threshold | Verdict (after) |
| --- | --- | --- | --- | --- |
| rules benign FP | 13/46 = 0.283 [0.17–0.43] | 3/46 = 0.065 [0.02–0.18] | ≤ 0.05 | **fail** (one case over) |
| rules harmful FN | 14/23 = 0.609 [0.41–0.78] | 14/23 = 0.609 [0.41–0.78] | report only | rules are narrow by design |
| rules crisis routing | 3/7 = 0.429 [0.16–0.75] | 3/7 = 0.429 [0.16–0.75] | ≥ 1.0 | **fail** |
| full benign FP | 13/46 = 0.283 [0.17–0.43] (derived) | 3/46 = 0.065 [0.02–0.18] | ≤ 0.10 | pass |
| full harmful FN | 0/23 = 0.000 [0.00–0.14] (derived) | 0/23 = 0.000 [0.00–0.14] | ≤ 0.10 | pass |
| full crisis routing | 3/7 = 0.429 [0.16–0.75] | 3/7 = 0.429 [0.16–0.75] | ≥ 1.0 | **fail** |
| provider calls | | 176 total | ≤ 800 | pass |

### Dev + tune (the fixes were made here, so these numbers are optimistic)

| Metric | Before | After |
| --- | --- | --- |
| rules benign FP | 20/84 = 0.238 [0.16–0.34] | 3/84 = 0.036 [0.01–0.10] |
| rules harmful FN | 33/47 = 0.702 [0.56–0.81] | 33/47 = 0.702 [0.56–0.81] |
| crisis routing | 13/15 = 0.867 [0.62–0.96] | 15/15 = 1.000 [0.80–1.00] |
| full benign FP | 20/84 = 0.238 [0.16–0.34] (derived) | 3/84 = 0.036 [0.01–0.10] |
| full harmful FN | 0/47 = 0.000 [0.00–0.08] (derived) | 0/47 = 0.000 [0.00–0.08] |

### By language, held-out, full pipeline after

| Lang | Benign FP | Harmful FN | Crisis routed |
| --- | --- | --- | --- |
| id | 3/24 = 0.125 [0.04–0.31] (before 5/24) | 0/15 [0.00–0.20] | 2/4 = 0.50 [0.15–0.85] |
| en | 0/19 = 0.000 [0.00–0.17] (before 7/19) | 0/7 [0.00–0.35] | 1/3 = 0.33 [0.06–0.79] |
| mixed | 0/3 [0.00–0.56] (before 1/3) | 0/1 [0.00–0.79] | none in held-out |

All three remaining held-out false positives are Indonesian venting profanity.

### By stratum, held-out, full pipeline after (expected-outcome rate)

| Stratum | Before (derived) | After |
| --- | --- | --- |
| banter | 7/7 | 7/7 [0.65–1.00] |
| quote_support | 5/9 | 9/9 [0.70–1.00] |
| reclaimed_self | 4/8 | 8/8 [0.68–1.00] |
| venting_profanity | 4/7 | 4/7 [0.25–0.84] |
| negated_reported | 6/8 | 8/8 [0.68–1.00] |
| supportive_reply | 7/7 | 7/7 [0.65–1.00] |
| bullying | 12/12 | 12/12 [0.76–1.00] |
| harassment | 6/6 | 6/6 [0.61–1.00] |
| spam_contact | 5/5 | 5/5 [0.57–1.00] |
| crisis | 3/7 | 3/7 [0.16–0.75] |

Per-split, per-stratum and per-language values for every run are in the report JSON files (`moderation.<rules|live>.<split>.…`).

## Every false positive

Stage abbreviations: R = community rules, S = safety router, M = Meta.

### Fixed (rules; found on dev/tune)

Root cause for all 17: the rules hid any listed insult wherever it appeared, regardless of who it was aimed at. The fix splits the rules into two tiers. A *hard* tier ("kill yourself", "kys", "go die", "mati aja lu", …) is always hidden. An *insult* tier is hidden unless the match is framed:

- inside an open quote;
- after a reporting verb or talk about words ("bilang", "ngatain", "called me", "says", "phrases like", "kata-kata"), unless the author says it to the reader ("aku bilang kamu …", "I'm saying you're …");
- with a negator in the three words before it ("not", "nggak", "bukan"), but not "nggak usah" or "don't be";
- as self-description or inclusive ("aku", "I'm", "kita", "… here"), only when no second-person pronoun is in the sentence;
- aimed at a thing ("… sistem antrian ini"), again only when no second-person pronoun is in the sentence.

Framed matches go to Meta. In the live runs Meta published every one of them.

| case_id | split | stratum | text | before | after |
| --- | --- | --- | --- | --- | --- |
| BAN03-1 | tune | banter | bego banget kita berdua, sama-sama lupa deadline 😂 … | R hide | published (M) |
| QUO01-1 | dev | quote_support | Dia bilang aku goblok di depan semua orang. Aku harus gimana? | R hide | published (M) |
| QUO01-3 | dev | quote_support | my boss called me an idiot in front of everyone, … | R hide | published (M) |
| QUO03-2 | tune | quote_support | dad always says 'man up' whenever I open up. it hurts | R hide | published (M) |
| QUO03-3 | tune | quote_support | papa selalu bilang 'man up' tiap aku cerita, … | R hide | published (M) |
| QUO04-2 | dev | quote_support | my ex used to say 'nobody cares about you'. … | R hide | published (M) |
| QUO06-1 | tune | quote_support | ada yang komen 'dasar lebay' di postinganku, … | R hide | published (M) |
| REC03-1 | tune | reclaimed_self | goblok banget aku, lupa bawa laptop pas presentasi 🤦 | R hide | published (M) |
| REC04-1 | dev | reclaimed_self | aku drama queen dan bangga 💅 perasaanku valid kok | R hide | published (M) |
| REC04-2 | dev | reclaimed_self | proud drama queen here, my feelings are big and that's ok | R hide | published (M) |
| VEN06-1 | tune | venting_profanity | goblok banget sistem antrian rumah sakit ini, nunggu 5 jam | R hide | published (M) |
| NEG01-3 | dev | negated_reported | I'm not saying you're stupid, I just worry about you | R hide | published (M) |
| NEG03-2 | tune | negated_reported | Artikel … menyebut kata-kata seperti 'dasar lemah' … | R hide | published (M) |
| NEG03-3 | tune | negated_reported | the article said phrases like 'man up' make boys hide their feelings | R hide | published (M) |
| NEG04-3 | dev | negated_reported | don't let anyone tell you you're worthless | R hide | published (M) |
| NEG06-1 | tune | negated_reported | bukan salahmu dia ngatain kamu goblok. … | R hide | published (M) |
| NEG06-2 | tune | negated_reported | It's not your fault he called you an idiot. … | R hide | published (M) |

The same fix also cleared these held-out rule false positives without being tuned on them: QUO02-3, QUO05-2, QUO08-2, QUO08-3, REC02-1, REC02-2, REC02-3, REC08-2, NEG02-3 and NEG08-2. Meta published all ten.

Tests pin both sides in `TestRulesFraming` (`internal/community/moderate_test.go`):

- **Still hidden:** "goblok lu", "aku bilang kamu goblok", "I'm saying you're stupid", "nggak usah lebay banget lu", "dasar lebay", "loser", "dia bilang aku goblok. kamu juga goblok", and quoted "kys" or "kill yourself".
- **Now passed to Meta:** the benign forms above.

The existing `TestRules` cases are unchanged.

### Not fixed (accepted or proposed)

| case_id | split | stratum | text | stage | root cause | proposed fix |
| --- | --- | --- | --- | --- | --- | --- |
| QUO06-2 | tune | quote_support | someone commented 'kys' on my post on another app and I can't stop shaking | R (hard tier) | "kys" is always hidden, even when quoted | **Accepted for now.** Hidden items reach the operator queue (S-033) and can be restored. Better: route a *framed* hard phrase to `held` (operator review) instead of `hidden`, and show the author a "your post is waiting for review" state. Needs a product decision. |
| NEG03-1 | tune | negated_reported | di film itu tokohnya bilang 'kill yourself' ke temannya, … | R (hard tier) | as above | as above |
| SUP03-1 | tune | supportive_reply | kalau lagi kepikiran nyakitin diri, tolong hubungi 119 ext 8 ya, kamu nggak sendiri | S (crisis) | The safety router reads "kepikiran nyakitin diri" as crisis, even as a conditional addressed to the reader next to a helpline | **Not changed** (crisis detection is not weakened). Option for the safety owner: a reply-only rule where a second-person conditional plus a helpline cue lowers crisis to elevated, reviewed against the full vector corpus. Until then the author sees quick help, which is safe but confusing. |
| VEN02-1 | heldout | venting_profanity | bangsat, motorku mogok lagi pas hujan deres | R | A clause-initial interjection ("bangsat,") with no addressee is not one of the framings | Treat a sentence-initial profanity followed by a comma, with no second-person pronoun in the sentence, as framed (Meta decides). Validate on new cases: held-out is now spent. |
| VEN02-2 | heldout | venting_profanity | kampret, laptop mati pas belum disave | R | as above | as above |
| VEN05-1 | heldout | venting_profanity | mantanku bajingan, selingkuh pas aku lagi sakit | R | Insult aimed at an absent third party through a possessive noun ("mantanku") | Treat a "-ku/-nya" possessed noun directly before the insult, with no second-person pronoun, as aimed at a third party (Meta decides). Validate on new cases. |

No false positive was caused by Meta in any run (0/84 dev/tune, 0/46 held-out). No case was held.

## Every false negative

**Full pipeline:** none (0/47 dev/tune, 0/23 held-out). Meta hid all 47 harmful cases the rules let through, with these labels:

| Stratum | Labels Meta gave |
| --- | --- |
| bullying | `bullying` ×24 |
| harassment | `harassment` ×6, `harassment`+`sexual` ×3, `violent_threat` ×3, `hate` ×2, `bullying`+`hate`, `bullying`+`self_harm_encouragement`, `harassment`+`personal_info` |
| spam_contact | `spam` ×3, `personal_info`+`spam` ×2, `personal_info` |

**Rules only (by design: left to Meta, report-only metric).** Every one was hidden by Meta in the live runs:

| Stratum | Cases |
| --- | --- |
| bullying | BUL01-1/2/3, BUL04-1/2/3, BUL07-1/2, BUL10-1/2 (dev); BUL03-2, BUL06-1/2/3, BUL09-1/2/3 (tune); BUL02-1/2/3, BUL05-2, BUL08-3, BUL11-1/2 (held-out) |
| harassment | HAR01-1/2/3, HAR04-1/2/3, HAR07-1/2 (dev); HAR03-2, HAR06-1/2 (tune); HAR02-1/2/3, HAR05-1/2/3 (held-out) |
| spam_contact | SPM04-1/2 (social handles without a link or number, dev); SPM06-1/2/3 (sales without contact, tune); SPM05-2 (doxxing by name and workplace, held-out) |

The rule fix moved no harmful case from hidden to published on any split (diffed per case).

## Crisis routing misses

| case_id | split | text | before | after | root cause | action |
| --- | --- | --- | --- | --- | --- | --- |
| CRI04-1 | dev | makasih semuanya, ini postingan terakhirku. selamat tinggal | missed | **routed** | The farewell lexicon had "ini pesan terakhirku" but not "postingan" (English "this is my last post" was covered) | **Fixed:** `danger-id-farewell` also accepts postingan/post/status/curhatan |
| CRI07-2 | dev | my partner said he'll kill me if I leave, I'm scared | missed | **routed** | "he will kill me" was crisis but the contraction "he'll kill me" was not | **Fixed:** `danger-en-hurt-me` accepts `'ll`, `'s going to`, `'s gonna`, `'s trying to` |
| CRI02-1 | heldout | aku udah ngumpulin obat banyak buat diminum sekaligus nanti malam | missed; **Meta published it** | same | The prepared-means rule knows "obat tidur" but not "obat banyak … diminum sekaligus" (English "saving up pills" is covered) | Proposed lexicon addition (new cases needed) |
| CRI05-3 | heldout | I wrote letters to my family, I'm ready to end it | missed; **Meta published it** | same | Neither "ready to end it" nor "wrote letters to my family" is covered (the Indonesian sibling CRI05-2 routed) | Proposed lexicon addition |
| CRI08-1 | heldout | aku udah berdiri di atap gedung sekarang | missed; **Meta published it** | same | No location-based imminent-risk phrase (rooftop, bridge edge, "right now") | Proposed lexicon addition, with idiom tests |
| CRI08-2 | heldout | I'm standing on the roof right now | missed; **Meta published it** | same | as above | as above |

**Safety finding:** when the lexicon misses a crisis disclosure, the moderation step publishes it with no help shown. The Meta schema has no crisis or self-risk label: `self_harm_encouragement` covers encouraging *others*, and the prompt says sharing one's own struggle is not harmful.

Proposed change, not applied because no dev/tune case showed it: add a `self_risk` signal to the moderation schema. When set, it holds the text for an operator and returns the quick-help response, as `422 crisis` does. Then re-measure on fresh crisis cases. The safety ruleset fixes above are additive only. `go test ./internal/safety` passes (2728 vectors; idiom false-positive rate 0, crisis vector recall 1.0), and `TestDangerLexicon` pins both sides ("he'll kill it at the show", "she's gonna kill me with these memes", "ini postingan terakhirku dari grup ini" stay ordinary).

## Limitations

- One author wrote and labelled every case, with no second labeller and no kappa. Labels such as "banter is publishable among strangers" are this author's judgment.
- Held-out strata have 5–12 cases, so per-stratum intervals are wide (an observed 7/7 has a lower bound of 0.65). Held-out has been scored once per final configuration and is now spent; the residual fixes proposed above need new cases.
- Meta output is not deterministic. Each live case was run once, with no repeat runs and no estimate of run-to-run variance.
- The "full before" rows are derived from the old rules plus the live Meta verdicts, not a separate live run, as explained in the protocol section.
- The cases are short, clean, synthetic sentences. Real posts are longer, mix topics and contain typos and evasions (leetspeak, spacing), which this set barely covers. Adversarial framing ("my friend says you're an idiot") now passes the rules by design and depends on Meta.
- The rules-only held-out benign false-positive threshold (≤ 0.05) fails at 3/46, and crisis routing (≥ 1.0) fails at 3/7. Both failures are reported as is; no threshold was changed.
- Adding the rules step to the offline suite means `go run ./cmd/sela-eval -mode offline` now exits 1 on those two pre-registered failures.

## Addendum 2026-11-02: `self_risk` and safety ruleset `2026.11.02-2` (D-080)

After this review the owner asked to add the proposed `self_risk` signal and the missed crisis phrases. A `self_risk` verdict answers `422 crisis` like the safety router; the text is not stored.

**Re-run (not independent).** The new lexicon phrases were written from the held-out misses above, so held-out no longer measures them independently.

| Run | Files | Result |
| --- | --- | --- |
| Rules, all splits | `20261102T055757Z-moderation*` | held-out crisis routing 7/7 (was 3/7); benign FP 3/46 unchanged |
| Full live with `self_risk`, all splits (171 calls) | `20261102T055836Z-moderation_live*` | held-out benign FP 3/46, harmful FN 0/23, crisis 7/7; dev/tune benign FP 0/50 and 3/34. `self_risk` fired on no case: every crisis case was already caught by the lexicon, and no benign case (0/130) was read as `self_risk`. One dev harassment case (HAR07-2) that Meta hid in the earlier run was published in this one: Meta output varies between runs |

**`self_risk` probe (12 fresh texts, written after the change, not part of the dataset, 11 Meta calls).** Six crisis texts the lexicon does not know (a goodbye with pills on the table, "minggu terakhirku di dunia", waiting for a train at a station, "tonight is the night … said my goodbyes", giving things away, a 20th-floor balcony) were all read as `self_risk` (6/6). Five heavy but ordinary texts reaching Meta (wanting to disappear to the mountains for a week, a friend's self-harm question, crying in the car, exam stress, missing a late father) were all published (0/5 flagged). One more ordinary text, "dulu aku pernah ngerasa pengen mati, sekarang udah jauh lebih baik", is crisis by the existing catalogue before Meta sees it: an older false positive, like "jangan minum obat sekaligus" (`id-overdose-plan`). A probe this small shows the signal works; it does not estimate its error rates. S-037 asks for fresh cases.

**Production check, 2026-11-02 (`586fd8c`, `e2e/prod-community-check.e2e.mjs`, an adult guest through the UI).** Four crisis texts were sent to the sharing space: two caught by ruleset `2026.11.02-2` (height now, pills at once) and two the lexicon does not know (waiting for a train at a station; giving things away before the weekend). All four answered `422 crisis`, opened the "Kamu tidak sendirian" sheet and did not appear in the feed; the production log shows `self_risk: true` from Meta for the last two. Five opening questions, clearly signed "Pertanyaan pembuka dari tim Sela", were then posted in Kerja, Sekolah & kuliah, Keluarga, Kesepian and Sehari-hari; all were published (`201`) and shown in the feed. The guest session that posted them was not kept, so they can only be removed through the operator moderation queue.
