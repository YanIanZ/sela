# Repository alignment audit — 2026-10-29

> **Update 2026-10-29 (D-060):** the voice-room, dictation and read-aloud work described below was removed. The v3 product is text-only; this audit remains as the historical record of that pass.

Scope: full-repository stabilization, reconciliation and architectural alignment
(branches, PRs, CI, safety engine, voice/ASR contract, AI routing, docs).
Method: fresh GitHub inspection, four parallel read-only subagent audits, then
verified fixes on the affected branches. Nothing was merged to `main`; no
history was rewritten; no force-push.

## 0. Verified baseline (fresh inspection, 2026-10-29)

- Remote branches: `main` `0a741a5`, `feat/sela-wellbeing-themes` `63e13dc`,
  `fix/safety-ruleset-5` `0f729ee`. (The earlier audit's branch list holds.)
- PRs: 15 total; 13 merged; **#8 draft** (safety ruleset -5), **#15 open**
  (751 files, +29042/−7119, 17 ahead / 9 behind, was CONFLICTING).
- Local working tree held uncommitted Meta→Amazon dictation wording (SPEC C25,
  BACKLOG S-025, Settings consent labels) from a prior session — completed and
  committed as part of F-02 below.
- Key structural fact: the branch's commit `cc1a55f` has a tree **byte-identical**
  to `main` `0a741a5` (empty `git diff`), so PR #15's branch already contained
  all of main; its 51 merge conflicts were rewritten-history artifacts, not
  divergent content.

## 1. Findings register

| ID | Severity | Component | Finding | Root cause | Corrective action | Status | Evidence |
|----|----------|-----------|---------|------------|-------------------|--------|----------|
| F-01 | P0 safety | `internal/safety` (PR #8) | Denial/idiom/brief-break exemptions weighed against the rest of the **sentence** only; a weak method/threat/later-act/absence tail opening a new sentence never took the exemption back ("I'm not suicidal, just need a break... with the pills" → `none`; comma form → `crisis`). PR #8's blocker (91 inputs), major (30+10) and minor (4) classes. | `sentenceAfter` truncates at `. ! ? \n`; the "leave this world" idiom pattern was not wide; no not-coming-back rule; skip-equivalence key ignored post-unit text. | `restAfter` (rest of message, bounded) at all five qualification sites; wide `en-passive-death-wish`; new rule `id-brief-farewell`; `mayHoldDenial` in the unit key. | **Fixed** `278f335`; the later review findings A/B/C and P1 variants are fixed in `223dc65`; PR #8 merged as `99b60fd` on 2026-10-29 after an independent review returned READY with no P0/P1 | 2728 vectors pass; -4/-5 differential on the 1329 shared vectors: **zero lower, zero higher**; race suite with database tests, skip/window equivalence and performance pass |
| F-02 | P0 | Voice/ASR transport | Dictation frontend produced **24 kHz** PCM; Sonic relay declared **16 kHz** input and forwarded unconverted (audio arrives 1.5× slow). | `ASR_SAMPLE_RATE = 24000` in `web/src/v3/asr.ts`; no resampling in `asrSonic`. | Frontend now produces 16 kHz mono little-endian PCM; explicit `DataView` LE encoding; stale comment corrected. | **Fixed** `1b8949a` | `web` asr tests 10/10; Go ASR tests pass |
| F-03 | P0 | `internal/httpapi/v3_asr.go` | Successful final transcript immediately followed by contradictory `error: asr_no_transcript` (masked only by client session-nulling). | Missing `else` in the flush path. | `else` + comment; regression tests for both terminal outcomes. | **Fixed** `1b8949a` | `TestASRSonicSingleCompletionWith/WithoutTranscript` |
| F-04 | P0 safety | `internal/httpapi/v3_voice.go` | Assistant audio released on **speculative** text pass; a failing **final** text could not recall already-streamed audio; no per-turn binding. | Gate ignored final-text failure after `gateOpen`. | Final-text failure after release now emits `interrupted` (client stops playback, drops queue) and shuts the gate for the rest of the turn; invariant documented (Sonic orders final text after audio; the speculative check bounds the window). | **Fixed** `1b8949a` | `TestVoiceGateUnsafeFinalTextRecallsReleasedAudio` |
| F-05 | P1 | `internal/httpapi/v3_asr.go` | Sonic dictation path had no total-audio cap (only per-frame + 5-min ctx). | Missing byte counter (Meta path had one). | `asrAudioMaxBytes` enforced on the Sonic path. | **Fixed** `1b8949a` | code review + existing tests |
| F-06 | P1 | `internal/httpapi/v3_asr.go` | Dead `asrMetaURL` helper; stale "push-to-talk" comment misdescribing Sonic semantics. | Leftover from the Meta relay. | Removed; comment rewritten. | **Fixed** `1b8949a` | — |
| F-07 | P0 CI | `.github/workflows/v2.yml`, `internal/account/store.go` | PR #15 CI failed at **gofmt** (`store.go`), then would fail the **dependency allowlist** (AWS SDK + gorilla/websocket never added). | Allowlist predated the voice work; unformatted file. | `gofmt -w`; allowlist extended to the exact 7 approved direct requirements; ADR-0002 §1 + F-46 updated with written reasons; PM approval recorded as **D-059**. | **Fixed** `ab206f2` | allowlist command verified locally; CI Go job green after F-10 |
| F-08 | P1 | `docs/API_SPEC_V3.md`, `docs/META_INTEGRATION.md`, `web/src/v3/asr.ts` | Docs and client naming still described Meta dictation after the committed Amazon migration (`policies.json` `asr-aws-2026-10-29.1`). | Migration docs landed without the code/doc sweep. | API spec row rewritten (dual-stack, 16 kHz, single terminal outcome); META_INTEGRATION exception marked superseded; `useMetaDictation`→`useDictation`, `downsampleTo24k`→`downsampleTo16k`. | **Fixed** `1b8949a` | web typecheck + tests |
| F-09 | P1 | `internal/httpapi` tests | New completion tests skipped locally (need `SELA_TEST_DATABASE_URL`); under CI `-race` they timed out on the 15 s flush window. | Wall-clock test design vs the handler's flush. | `Options.ASRFlushWait` test hook (500 ms); deterministic partial-transcript wait. | **Fixed** `f98900e` | `-race` + `SELA_REQUIRE_DB_TESTS=1`: 1.4 s / 0.7 s; full race suite green; CI Go job pass |
| F-10 | P0 | PR #15 integration | 51 conflicts vs main (18 content, 31 add/add, 2 modify/delete). | Rewritten PR #14 commits on the branch (same trees, different SHAs). | Resolved all with the branch side after verifying tree identity; merged tree **byte-identical to the pre-merge branch**; PR #15 now MERGEABLE. | **Fixed** `e3d06f7` | `git diff` empty vs pre-merge; Go+web+build green |
| F-16 | P1 a11y | `web/src/v3/base.css` | Desktop header overflowed 1365 px by 3 px at 130% text + dark (WCAG 2.2 AA 1.4.10 Reflow); failed the v3 journey's "no horizontal overflow display dark 130%". | rem-based header budget never compacts in the ≥75 em range when text scale grows. | Compact-header budget extended to `data-text-scale` 130/150 at 75 em+. | **Fixed** `bab3557` | Full v3 e2e journey re-run locally: all checks pass |
| F-17 | P1 a11y | `web/src/v3/base.css` (mobile shell) | Fixed `.s-shell-inner` bottom padding (4.75rem) under-cleared the fixed place-nav when it wraps taller (130% text, 320 px): the nav covered the footer Policies link on the policy detail page (axe target-size, WCAG 2.2 AA 2.5.8). Latent branch defect — the container journey never ran on the branch before this pass (earlier CI died at gofmt). | Padding ignored the measured `--bottom-bar-h` the shell already publishes. | `padding-bottom: max(4.75rem, --bottom-bar-h)`; verified `--bottom-bar-h: 165px` engages on the detail page. | **Fixed** `01b7402` | Local full journey ×3 zero failures; CI container journey pass |
| F-11 | P1 product | v2 demo contract | Branch removed the v2 demo UI (`/demo`, `/r`, `web/src/pages/**`, `e2e/journey.e2e.mjs`) while SPEC C24/V42 and status docs still describe a demo running every feature on synthetic data. | Sampul-only refactor (aaf1c36) made a product decision the docs never recorded. | **Owner decision needed**: restore a demo surface or amend the contract docs. Flagged on PR #15. | **Open — owner** | PR #15 integration comment |
| F-12 | P2 repo | `.impeccable/` | 547 of PR #15's 751 files are design-artifact boards. | Iterative design assets committed to the feature branch. | Recommend moving to review archives (separate cleanup PR) to shrink the review surface; provenance retained. | **Open — low** | conflict-map audit |
| F-13 | P2 docs | `docs/IMPLEMENTATION_STATUS.md`, `docs/design/DESIGN_DIRECTION.md` | Status checkpoint says the UI authority is "cobalt/open-S" while the branch implements the sage/cream Sampul Buku identity in `web/DESIGN.md`. | Branch redesign outran the doc checkpoint. | To reconcile when the owner confirms the canonical identity (with F-11). | **Open — owner-adjacent** | doc read |
| F-14 | P1 safety process | PR #8 corpora | The 91/30/10/4 regression corpora (1953-input, 17,992-input sweep) are **not in the repo**; exact counts unverifiable. | External re-verification harness. | Committed vectors now cover every reproduced class; recommend the owner attach the corpora to the PR or accept the committed coverage. | **Partially addressed** | vectors.json +27 |
| F-15 | P2 | repo hygiene | Stash `stash@{0}` ("Teleport auto-stash", docs-only, +228) and worktree `.worktrees/v2-go` on a deleted remote branch. | Prior sessions. | Left untouched; recommend owner review/drop. | **Open — low** | `git stash list` |

## 2. What was preserved

- PR #8 remains **draft** — not merged, because GitHub mergeability ≠ safety
  acceptance. Its independent re-verification is still owed.
- The dual-stack ASR design (Amazon Sonic `bedrock` + legacy Meta `meta`
  behind `SELA_ASR_MODE`) is preserved; the `asr_meta` consent purpose key is
  unchanged.
- The voice room's approved constraints (D-058): Meta stays the primary text
  generator; Amazon only in the voice room, labelled, 18+, consent-gated,
  `SELA_VOICE_MODE=disabled` default, nothing stored, one session per person,
  7 min 50 s cap.
- Main's PR #11–#14 content (tree-identical subset of the branch) and the
  branch's Sampul redesign, well-being themes, voice work and a11y audit.
- Contributor history: no force-push, no rewritten published commits; the
  merge used a normal merge commit.

## 3. Branch / PR state after this pass

| PR | State before | State after | Notes |
|----|--------------|-------------|-------|
| #15 | CONFLICTING, CI red | **MERGEABLE, CLEAN; all 7 active checks pass** on head `01b7402` (24 commits) | Resolution + fixes documented in a PR comment |
| #8  | draft, blocker documented | draft + rounds 8–9 fix `278f335`; **all CI green** (Go race 5m34s, e2e, container) | Independent re-verification still required before un-drafting |

## 4. Remaining blockers (owner / external)

1. **Owner decision (F-11):** v2 demo removal vs the C24/V42 demo contract.
2. **Owner decision (F-13):** canonical design identity (cobalt/open-S docs vs
   Sampul sage/cream code) — reconcile `IMPLEMENTATION_STATUS.md` +
   `DESIGN_DIRECTION.md` once decided.
3. **Independent safety re-verification (PR #8):** by someone other than the
   implementer; optionally attach the external corpora (F-14).
4. **Cleanup (F-12, F-15):** `.impeccable` archive PR; stash/worktree review.
5. Live AWS/Bedrock and Modal/provider testing was **not** performed (test
   doubles only); B-1, B-3, B-8, B-9, B-10, B-15 stand as in `docs/BLOCKERS.md`.

## 5. Release-readiness statement

No release-readiness claim is made. This pass repaired P0 correctness/safety
defects and made the integration branch mergeable with evidence; it does not
equate merged PRs with readiness, and every provider claim above is labelled
with the doubles-vs-live distinction.
