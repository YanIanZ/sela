# Requirements traceability

Updated: 2026-10-28. §1 maps the v3 program (owner brief v3 of 2026-10-26) to planned code and planned evidence, including the delivery rules (§1.3), the quality gates (§1.4) and the v3 Spec constraints and invariants (§1.5); §2 keeps the v2 mapping, which stays valid for v2 scope. Normative wording lives in [SPEC.md](../SPEC.md); decisions in the [decision log](DECISION_LOG.md); items and acceptance criteria in the [backlog](BACKLOG.md); open external dependencies in [blockers](BLOCKERS.md); latest status in [implementation status](IMPLEMENTATION_STATUS.md).

## 1. v3 scope (owner brief v3)

**2026-10-28 frontend delivery.** Branch `feat/v3-experience` implements the v3
frontend against the existing `/api/v3` routes and records the 28 screen groups,
their API dependencies and their blocked states in
[`V3_FRONTEND_VALIDATION.md`](V3_FRONTEND_VALIDATION.md). That is local frontend
evidence, not a change to any requirement or backlog status; nothing is verified
live.

**Status values (v3).** The status words of [SPEC §G](../SPEC.md#g-goal), their single definition; the gate for each level is in [roadmap §2](ROADMAP.md#2-release-ladder). Each backlog row's status is copied from [backlog §5](BACKLOG.md#5-index); a DoD row names the items whose work has started. Today the **backend foundation is verified locally, and its CI run on draft PR #9 passed** (commit `0ff3341`, ADR-0002 F-01 to F-47; recorded local and CI runs in [implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands); [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26)): S-173 and S-260 are `verified locally`, and S-002 to S-005, S-171, S-172, S-174 to S-177, S-211 and S-261, and criterion 1 of S-262, have their backend part verified locally and stay `in progress` for the rest (for S-262, the restore notice and status-page post of its criterion 3 are planned and its hosted drill is blocked externally). Other `in progress` work is not evidence: the design and planning documents (S-190, S-273), the ruleset `-5` fix loop (S-150; a WIP snapshot in draft PR #8 to main), the frontend (S-153, S-191, S-193; implemented and locally checked on `feat/v3-experience`; owner/browser acceptance pending, see [V3_FRONTEND_VALIDATION.md](V3_FRONTEND_VALIDATION.md)) and, on separate branches that are not merged, the AI provider layer (S-220 to S-224, S-226 to S-228; committed on `feat/sela-v3-ai` as `44845d3` and `63a5c98`, then `02051ae`, which merges `feat/sela-v3-pilot` at `59dba14`, and open as draft PR #10 against `feat/sela-v3-pilot`), research and reading content (S-140, S-240, S-242 to S-245; committed on `feat/sela-v3-research` as `268a101` and `64aa3f6`, and open as draft PR #11 against main), the P3 listening backend (S-020, S-050, S-060, S-070, S-080) and operations (S-265). Nothing in v3 is verified live. A row also names its external blocker where live verification or the item itself depends on one.

Package names marked *(proposed)* are proposals for the milestone owner, not existing code. `F-` criteria are in [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md#acceptance-criteria-foundation); `EV-`, `VC-` and routing thresholds in [ADR-0003](adr/ADR-0003-ai-routing-verifier-fallback.md); `BASE-` criteria in [backlog §2](BACKLOG.md#2-shared-acceptance-criteria). "Ported" means a v2 test re-pointed at the v3 store and routes.

### 1.1 Definition of Done capabilities (brief §15)

The brief also requires the final status to separate implemented, verified locally, verified live, ready for controlled pilot and public-release blockers; that ladder and its gates are in the [roadmap](ROADMAP.md#2-release-ladder).

| ID | Capability | Backlog items | Planned code areas | Planned evidence | Status |
| --- | --- | --- | --- | --- | --- |
| DoD-1 | Tell their story and get a relevant response ("bercerita dan menerima respons yang relevan") | S-020, S-021, S-022, S-023, S-024, S-150, S-151, S-221, S-223 | `/api/v3/conversations` in `internal/httpapi`; `internal/conversation` *(proposed)*, migrations 0100–0199; `internal/airoute` *(proposed)* over `internal/llm`; `internal/safety`; conversation room in `web/` | e2e talk journey with the test double; guard unit tests in ID, EN and mixed text; EV-7 routing thresholds; manual screen-reader check of streaming; live EV-1 and EV-4 on staging | in progress: S-150 (ruleset -5); S-020, S-221 and S-223 on separate branches, not merged; the rest planned; live blocked externally (B-1) |
| DoD-2 | Choose how their notes are stored ("memilih penyimpanan") | S-002, S-003, S-006, S-020, S-170, S-171 | `internal/account` (guest, accounts, consents); per-conversation storage choice and the bounded in-memory hold for "don't save" turns (D-051); settings in `web/` | F-30 to F-33, F-35; DB tests asserting zero content rows for every non-storing choice, during and after a "don't save" conversation; fake-clock and restart tests of the hold's purges; e2e onboarding with storage declined | in progress: S-002, S-003, S-171 (backend verified locally); S-020 (backend, separate branch, not merged); S-006, S-170 planned |
| DoD-3 | See and correct their personal context | S-023, S-050, S-080 | `internal/memory` *(proposed)*, migrations 0300–0399; journal records, migrations 0200–0299; review status and versions | Tests: a correction keeps the earlier version; a forgotten item is absent from the next reply request; unconfirmed cards are never used; e2e memory journey | in progress: S-050 and S-080 (backend, separate branch, not merged); S-023 planned |
| DoD-4 | Access a timeline | S-070, S-071 | `/api/v3/timeline` read model over owned records | Cross-user test; a deleted record disappears at once; e2e timeline journey; axe list semantics | in progress: S-070 (backend, separate branch, not merged); S-071 planned |
| DoD-5 | Prepare a sourced consultation summary | S-090, S-091, S-092 | `internal/summary` *(proposed)* reusing the v2 approval-by-hash and `internal/ledger` Meaning Lock; migrations 0400–0499; verifier for AI wording | Selected-items-only test; stale-on-source-deletion test; ported `TestAIDraftNeedsReviewEditInvalidatesApproval`; VC-8 per-verdict tests; e2e manual summary with AI off | planned; AI wording live blocked externally (B-8) |
| DoD-6 | Control sharing | S-100, S-101, S-104, S-120 | `internal/share` *(proposed)* reusing v2 handoff and projection, with a purpose; recipient principal and cookie; migrations 0400–0499 | Preview-equals-projection test; purpose-required test; ported `TestTokenAloneIsNotEnoughAndLockout` and `TestIdempotentHandoffUnderConcurrency`; e2e share journey | planned |
| DoD-7 | Stop further access | S-101, S-120, S-174 | Revocation, expiry and deletion paths ending recipient sessions | Revocation race tests; e2e revoke then recipient denied; copy test for "cannot recall copies" | in progress: S-174 (account deletion backend verified locally); S-101, S-120 planned |
| DoD-8 | Export and delete their data | S-174, S-175 | Exporter registry and deletion path in `internal/account`; per-feature exporters and cascades; backup windows in `scripts/ops/` | F-08, F-14, F-39, F-40; per-feature derivative-deletion tests; export injection tests; e2e export and delete | in progress: S-174, S-175 (backend foundation verified locally); feature sections planned |
| DoD-9 | Continue reflecting after a consultation | S-050, S-130, S-131 | After-consultation journal entry type linked to summaries and shares | e2e with and without a share; copy test that nothing implies care was given | in progress: S-050 (backend, separate branch, not merged); S-130, S-131 planned |
| DoD-10 | Use Indonesian and English | S-193, BASE-2 on every user-facing item | `web/src/i18n`; server message tables; reply-language handling | i18n parity and hard-coded string tests; F-42 pattern for every v3 error; code-switched tests in safety, guards and Meaning Lock; fluent-speaker review | in progress: S-193 (implemented on `feat/v3-experience`; fluent-speaker review pending); live bilingual quality blocked externally (B-1) |
| DoD-11 | Use the main journeys with different accessibility needs | S-022, S-190, S-192, S-194, S-195, S-196, BASE-3 to BASE-7, BASE-11 and BASE-12 | `docs/design/` accessibility criteria; design system and app shell in `web/`; text-only surface; `e2e/` gate; `eval/thresholds.json` accessibility metric | axe with `wcag22aa` at any impact; 320 px reflow, text-spacing, unique-title and target-size checks; manual review with NVDA, JAWS, VoiceOver and TalkBack; screenshots per journey | in progress: S-190 (design documents); the rest planned |

### 1.2 Backlog items, pilot-critical and pilot-important

Priorities follow [backlog §1](BACKLOG.md#1-how-to-read-this). The Spec IDs that each item serves are mapped in §1.5.

| ID | Requirement | Priority | Planned code areas | Planned evidence | Status |
| --- | --- | --- | --- | --- | --- |
| S-001 | Welcome: what Sela is and is not | pilot-critical | Welcome page and copy in `web/` | e2e zero rows after a page load; claim gates; BASE | planned |
| S-002 | Guest start with honest storage notice | pilot-critical | Guest lifecycle and purge in `internal/account`; `POST /api/v3/auth/guest`; guest notice in `internal/fixtures/v3`; notice in `web/` | F-30, F-31, F-33; `web:` guest-notice test; e2e guest start | in progress (backend verified locally: F-30, F-31, F-33) |
| S-003 | Account, sign-in and recovery without email | pilot-critical | Credentials, sessions and recovery in `internal/account`; API_SPEC_V3 §5 routes; forms in `web/` | F-16 to F-29; `web:` auth-forms test (paste, `autocomplete`); e2e idle-expiry warning | in progress (backend verified locally: F-16 to F-29) |
| S-004 | Guest upgrade keeps everything | pilot-important | `POST /api/v3/auth/guest/upgrade` | F-32; e2e upgrade | in progress (backend verified locally: F-32) |
| S-005 | Age band at the start | pilot-critical | `users.age_band`, `SELA_ALLOWED_AGE_BANDS` | F-36; under-15 zero-row test | in progress (backend verified locally: F-36) |
| S-006 | Consents during onboarding | pilot-critical | Onboarding in `web/`; consent API; policy texts | e2e with every AI consent declined; DB test with storage declined; disclosure-content tests | planned |
| S-007 | Optional conversation intent | pilot-important | Conversation field; AT-1 input; picker in `web/` | e2e first message without the picker | planned |
| S-020 | Conversation room | pilot-critical | `internal/conversation` *(proposed)*; migrations 0100–0199; `/api/v3/conversations`; bounded in-memory hold for "don't save" turns (D-051); room in `web/` | "Don't save" DB tests (no message row during or after); fake-clock purge tests (End, sign-out, 30 minutes idle, 2 hours) and a restart test; reload-reopen e2e; route-table screening test; derivative-deletion test; e2e talk journey | in progress (backend, separate branch, not merged) |
| S-021 | Replies that listen | pilot-critical | Versioned listening prompt; S-223 guards; conversation set in `eval/` | Guard tests in ID, EN and mixed text; question-count and template-repetition checks; no-push-to-a-professional guard cases; pre-registered relevance metric | planned; live blocked externally (B-1) |
| S-022 | Screen-reader-friendly streaming and Stop | pilot-critical | Message log, status region and the "announce replies to screen readers" (`aria-live`) setting, on by default, in `web/` | Manual screen-reader matrix; e2e focus and Stop checks; one announcement per reply with the setting on, none with it off (ST-1, ST-3) | planned |
| S-025 | Voice dictation | retired | Removed 2026-10-29 (D-060); no microphone or ASR surface in `web/` | — | retired |
| S-026 | Read aloud | retired | Removed 2026-10-29 (D-060); no speech synthesis in `web/` | — | retired |
| S-023 | "Sela menangkap…" cards | pilot-critical | AT-2 through the router; card records with provenance and review status | Unconfirmed-never-used test; grounding checks; e2e confirm, correct, remove | planned |
| S-024 | AI off or unavailable | pilot-critical | Conversation states; `internal/llm` disabled mode; test doubles | Zero calls when declined; outage e2e | planned |
| S-040 | Personal home | pilot-important | Home in `web/` | e2e states; copy test with no streak or score | planned |
| S-050 | Notes (journal) inside Catatan pribadiku | pilot-critical | `internal/journal` *(proposed)*; migrations 0200–0299; `/api/v3/journal` | Crisis-matching save test; version-history test; derivative-deletion test; e2e | in progress (backend, separate branch, not merged) |
| S-051 | Reflection prompts | pilot-important | Prompt data; `web/` | e2e save without a prompt; source-label test | planned |
| S-052 | Reflect on an entry with Sela | pilot-important | AT-2 over journal segments | S-023 tests on journal cards | planned |
| S-060 | Check in any time | pilot-important | Check-in records, migrations 0200–0299; `/api/v3/check-ins`; `web/` | Crisis-note save test; manual 2.5.7 and 1.4.1 checks; e2e | in progress (backend, separate branch, not merged) |
| S-070 | Timeline across sessions | pilot-critical | `/api/v3/timeline` read model | Cross-user test; deletion test; e2e | in progress (backend, separate branch, not merged) |
| S-071 | Find things in my timeline | pilot-important | Per-user search in `web/` | e2e result-count announcement | planned |
| S-080 | See and correct what Sela remembers | pilot-critical | `internal/memory` *(proposed)*; migrations 0300–0399; `personal_memory` consent | Forgotten-item-absent-from-request test; DM-4 test; minors opt-in test; e2e | in progress (backend, separate branch, not merged) |
| S-081 | Memory suggestions from Sela | pilot-important | AT-3 through the router and verifier | Verifier-unavailable shows nothing (RR-11) | planned; live blocked externally (B-8) |
| S-090 | Build a "Ringkasan untuk konsultasi" | pilot-critical | `internal/summary` *(proposed)*; migrations 0400–0499 | Selected-only test; stale marker test; ported approval tests; e2e manual summary | planned |
| S-091 | AI wording for summary items, verified | pilot-important | AT-4 through the router; VC-8 release rules | Per-verdict fallback tests; abstain test | planned; live blocked externally (B-8) |
| S-092 | Print or download one summary | pilot-important | Print view in `web/`; export renderer | Injection test; audit-event test | planned |
| S-100 | Preview and consent before sharing | pilot-critical | `internal/share` *(proposed)*; purpose field; consent record | Preview-equals-projection test; refusal without purpose; e2e | planned |
| S-101 | Share link, code, expiry, revocation, status | pilot-critical | Share records, recipient principals, status machine, audit events | Ported recipient and idempotency tests; revocation race tests; plaintext token scan | planned |
| S-102 | Opening lines | pilot-important | `internal/ledger` opening validation reused; AT-5 | Ported `TestValidateOpening`; template-path e2e | planned |
| S-103 | Destination checklists | pilot-important | Checklist data with sources and re-verify-by | Stale-flag test | planned; sources need B-5 |
| S-104 | Private names and redaction preview | pilot-important | `internal/ai/minimize` (ported v1 PII patterns) plus a future `internal/redact` preview surface | Captured-request tests; projection tests | partial: patterns ported; preview not built |
| S-120 | What the recipient sees | pilot-critical | `/api/v3/recipient`; `__Secure-sela_recipient` cookie | Principal-separation tests; uniform access-ended test; e2e on a phone viewport | planned |
| S-121 | Acknowledgement and consultation status | pilot-important | Recipient action; owner status action | Audit test; claim gates | planned |
| S-130 | Reflect after a consultation | pilot-critical | After-consultation entry type; links to shares | e2e with and without a share; copy test | planned |
| S-131 | My own next steps | pilot-important | Person-written steps | Production serves no fixture plan (config test) | planned |
| S-140 | Reading library | pilot-important | `/api/v3/reading`; migrations 0600–0699; card data from `internal/fixtures/data/self_help_cards.v1.json` | Licence-fields test; cards-by-ID-only test; verification record per item | in progress (content, separate branch, not merged) |
| S-141 | Small exercises ("latihan kecil") | pilot-important | Exercise data in the reading store; Reading & support in `web/` | Source-or-team-written label test; no-day-counter copy test; verification record per exercise | planned |
| S-150 | Crisis screening on every text | pilot-critical | `internal/safety` ruleset -5; every v3 write route | Route-table screening test; `safety.model_calls_on_crisis_path = 0`; ruleset version stored with each assessment; new vector tags; linear-time benchmark | in progress (ruleset -5: WIP snapshot `f619677` on `fix/safety-ruleset-5`, draft PR #8 to main, re-verification pending; D-034 status) |
| S-151 | Crisis support that keeps the words | pilot-critical | Per-conversation crisis hold; journal and check-in save paths; help card in `web/` | e2e without navigation; zero calls during the hold; risk-flag tests | planned |
| S-152 | Immediate help page and verified directory | pilot-critical | Help data with enforced re-verify-by in `internal/fixtures`; help route; page in `web/` | Fake-clock re-verify-by test; no-model-call test; one-catalogue test (bundled copy equals the server catalogue) | planned |
| S-153 | "Butuh bantuan sekarang / Need help now" and Quick Exit everywhere | pilot-critical | App shell in `web/` | e2e on every route; Esc toggle test; 320 px check | in progress (uncommitted) |
| S-154 | Re-verify help lines | pilot-critical | Successor of `internal/fixtures/data/help.json` | Decisions recorded with sources; dates updated | blocked externally (B-5) |
| S-170 | Choose how Sela keeps my notes | pilot-critical | Storage settings; per-surface storage rules | No-row tests; policy-duration equality test | planned |
| S-171 | Consent records per purpose and provider | pilot-critical | Consents and purpose registry in `internal/account` | F-35; withdrawal-mid-operation test | in progress (foundation verified locally: F-35) |
| S-172 | See and end my sessions | pilot-important | Session routes; settings in `web/` | F-28 | in progress (backend verified locally: F-28) |
| S-173 | Encryption at rest and crypto-shredding | pilot-critical | `internal/vault` | F-12 to F-15; plaintext scan per feature | verified locally (F-12 to F-15; CI passed on PR #9) |
| S-174 | Deletion with derivatives | pilot-critical | Deletion path in `internal/account`; per-feature derivative rules | F-08, F-14, F-40; per-feature derivative tests | in progress (account deletion backend verified locally: F-08, F-14, F-40) |
| S-175 | Export my data | pilot-critical | Exporter registry and renderers | F-39; injection test | in progress (foundation sections verified locally: F-39) |
| S-176 | Access audit | pilot-critical | `audit_events`; recipient-open events; list in `web/` | F-38; recipient-open test | in progress (foundation events verified locally: F-38) |
| S-177 | Policy texts and disclosures | pilot-critical | `internal/fixtures/v3/policies.json` | Pasal 21 element test; forbidden-phrase test; equality with configuration | in progress (foundation texts verified locally; `terms-privacy-2026-11-01.2` and `guest-storage-2026-11-01.2` come with the integration pass, committed on `feat/sela-v3-pilot` (draft PR #9); the PM confirms the `terms_privacy` restore sentence as written and the `guest_storage` text, [D-056](DECISION_LOG.md#d-056)); legal review blocked externally (B-10) |
| S-178 | Legal readiness | pilot-critical | DPIA, processing record, processor list, breach runbook, registration checklist (documents) | Recorded owner decisions | blocked externally (B-10, B-15) |
| S-190 | Design direction, IA, screens, content style | pilot-critical | `docs/design/` | `docs/design/tools/contrast.py`; recorded PM review | in progress (design documents) |
| S-191 | Design system and app shell | pilot-critical | Components, layout and router in `web/src` | axe; unique-title test; navigation-order test | in progress (uncommitted) |
| S-192 | Display settings | pilot-important | Settings and pre-paint script in `web/`; preferences in `localStorage` on the device (non-sensitive only) | Reload-persistence test; storage-blocked test; 200 % check | planned |
| S-193 | Indonesian and English everywhere | pilot-critical | `web/src/i18n`; server message tables | Parity tests; F-42 pattern; code-switch tests | in progress (uncommitted) |
| S-194 | Automated WCAG 2.2 AA gate | pilot-critical | `e2e/`; `eval/thresholds.json` | A seeded violation fails CI | planned |
| S-195 | Manual accessibility review and screenshots | pilot-critical | Review records in each milestone PR | Screen-reader matrix report; screenshots in ID and EN | planned |
| S-196 | Slow connections and small devices | pilot-important | Bundle budget in CI; throttled e2e | CI budget check; throttled e2e | planned |
| S-210 | Experience for ages 15–17 | pilot-important | Age-band rules across features; young-person copy | `routing.deepseek_calls_minor_bands = 0`; F-10 production refusal; no-guardian-route test; `unknown` gets the `15_17` defaults | planned; production blocked externally (B-10) |
| S-211 | Guardian consent record | pilot-important | Guardian consents in `internal/account` | F-37 | in progress (backend verified locally: F-37) |
| S-212 | Legal and provider gate for minors | pilot-important | `SELA_ALLOWED_AGE_BANDS` per decision; an age question that tells 15 from 16–17 | Recorded decisions; a test that a 15-year-old is refused before `15_17` is enabled anywhere ([conflict matrix](CONFLICT_MATRIX.md) #49) | blocked externally (B-10) |
| S-213 | Evaluation and evidence for young people | pilot-important | Personas in `internal/fixtures`; safety vector tags; EV-0 cases | Fixture and tag presence tests | planned |
| S-220 | Routing core with test doubles | pilot-critical | `internal/airoute` *(proposed)*; migrations 0500–0599; `/api/v3/ai` | EV-7 thresholds at zero; provenance DB test; own-rate-limit-never-falls-back test | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified) |
| S-221 | Meta as primary generator | pilot-critical | `internal/llm` Modal mode through the router; `modal/` | Served-model pin tests; live smoke test, EV-1, EV-4 | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified); live blocked externally (B-1) |
| S-222 | Claude Sonnet 5.5 verifier | pilot-critical | `internal/verifier` *(proposed)*, grown from `prototypes/evidenceguard` | VC-6 and VC-8 tests; no-reasoning storage scan; live EV-2, EV-3 | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified); live blocked externally (B-8) |
| S-223 | Output guards and health-claim detector | pilot-critical | `internal/ledger/clinical.go` extended; guard and flag lists; claim gates | Bilingual positive and negative tests; same-guards-for-every-generator test | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified) |
| S-224 | DeepSeek fallback generator | pilot-important | `internal/llm` DeepSeek mode | Routing thresholds; no-consent test; live EV-5 | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified); live blocked externally (B-9) |
| S-226 | Data minimisation and redaction | pilot-critical | `internal/redact` *(proposed)*; request builders | Captured-request tests; log canary | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified) |
| S-227 | Meta-only demo profile | pilot-important | `SELA_AI_PROFILE` configuration | Zero Anthropic and DeepSeek calls under the profile | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified); organizers' answer pending (B-12) |
| S-228 | AI evaluation | pilot-critical | `cmd/sela-eval`; `eval/` | EV-0 and threshold version committed before the first verifier run; offline, test-double and live reports | in progress (committed on `feat/sela-v3-ai`, not merged; nothing verified); live blocked externally (B-1, B-8, B-9) |
| S-240 | Corpus hygiene | pilot-important | `scripts/research/`; CI path filter; `web/src/pages/Research.tsx` | Manifest hashes verify after checkout; `verify` 0 violations | in progress (LF pins and dated notes in `268a101`, draft PR #11; CI path filter and `Research.tsx` flags not done) |
| S-241 | Unchecked dates and design labels | pilot-important | Reviewer overrides in `data/research/corpus/` | `verify` without warnings for cited records | planned |
| S-242 | Corpus schema fields | pilot-important | `scripts/research/corpus/` (language, age range, country evidence, reviewer-only findings and limitations, reviewer fields) | Byte-reproducible rebuild; null-when-unknown checks; reviewer-only check for findings and limitations | in progress (separate branch, not merged) |
| S-243 | v3 gap searches | pilot-important | Collector and screening scripts; new frozen run with the v2 families carried over and the v3 families added, help-seeking and safety among them | Frozen queries and manifest; reproducible screening | in progress (separate branch, not merged) |
| S-244 | v3 evidence map | pilot-important | `data/research/corpus/evidence_map.json` new version: feature, brief §9 topic and population rows | Population label on every row; a test that every listed row exists; declared gaps | in progress (separate branch, not merged) |
| S-245 | Claim support and reading verification | pilot-important | Claim-check records; AT-6 through the verifier | Recorded check per statement and reading item | in progress (separate branch, not merged); AT-6 live blocked externally (B-8) |
| S-246 | Human appraisal | pilot-important | Reviewer worksheet | Reviewer, date, tool and result per record | blocked externally (B-4) |
| S-260 | PostgreSQL and migrations | pilot-critical | `internal/db`; `cmd/sela` migrate commands; CI service | F-01 to F-09, F-46 | verified locally (F-01 to F-09, F-46; CI passed on PR #9) |
| S-261 | Separate environments | pilot-critical | `internal/config`; startup guards in `cmd/sela` | F-10 (counts once ADR-0002 §12 is aligned with D-046, [conflict matrix](CONFLICT_MATRIX.md) row 52), F-11; demo e2e: AI on live providers with synthetic data or on labelled test doubles, never shown as live; demo fixtures refused outside the demo | in progress (guards verified locally: F-10, F-11; row 52 and the demo criteria open) |
| S-262 | Backup and restore | pilot-critical | `scripts/ops/`; `sela restore reconcile` | F-44; hosted drill record in the runbook; e2e of the after-restore notice in ID and EN, and the status-page post (S-262 criterion 3) | in progress (criterion 1 verified locally: F-44, CI passed on PR #9; the in-app restore notice, P2, and the status-page post, P7, planned; the hosted drill blocked externally, B-3, B-15) |
| S-263 | Deployment and rollback | pilot-critical | Container image and deployment notes (successor of `deploy/v2/Dockerfile`) | Staging smoke test; recorded rollback rehearsal | blocked externally (B-3, B-15) |
| S-264 | Meta model on Modal | pilot-critical | `modal/sela_llm.py`, `modal/smoke_test.py` | Deploy, smoke test and cold-start measurement | blocked externally (B-1) |
| S-265 | Observability without content | pilot-important | Metrics route; alerts | F-41, F-43; staged alert tests | in progress (documents, separate branch, not merged) |
| S-266 | Security review before the pilot | pilot-critical | Review report over v3 routes, cookies, encryption, logs, provider calls and the operator audit | Every finding fixed or decided by the owner; outbound-destination check | planned |
| S-267 | Legacy Dengar.in deployments | pilot-important | Owner actions outside this repository | Recorded decision and sunset steps | blocked externally (B-13) |
| S-270 | `OWNER_TESTER.md` | pilot-critical | `OWNER_TESTER.md` | Owner's recorded results for the pilot-critical scenarios | planned |
| S-271 | Pilot readiness report and plan | pilot-critical | Readiness report | Every DoD capability on the release ladder with evidence links | planned |
| S-273 | Spec and documentation alignment | pilot-critical | SPEC.md and `docs/` | Link and anchor check (run 2026-10-26 on the tree of the integration pass, now committed on `feat/sela-v3-pilot` in draft PR #9: 0 broken over 139 files apart from one known false positive; to be re-run on that commit, [implementation status §1](IMPLEMENTATION_STATUS.md#1-where-v3-stands)); document inventory | in progress |
| S-274 | Operator access audit | pilot-critical | Operator audit record written by the `cmd/sela` operator commands (`keys`, `restore` and `restore reconcile`, `ops canary`, data migrations); store location to be set in ADR-0002 §13 *(proposed; [conflict matrix](CONFLICT_MATRIX.md) row 57)*; mirror of operator changes into `audit_events` | Per-command record test; append-only test; mirror-into-owner-audit test; log canary; no-decrypted-content route and command test; hosted database access log in the runbook | planned; hosted part blocked externally (B-3) |

### 1.3 Delivery and GitHub (brief §12)

The v2 rows GH1 to GH4 (§2) describe PR #6, which was merged into main on 2026-10-26 (`7fc6678`). The rows below apply to the v3 delivery branch `feat/sela-v3-pilot`. The v3 commits (`4cadbfd`, `0ff3341`, `59dba14`, and the integration pass committed on top of them) are pushed to `origin/feat/sela-v3-pilot` and open as **draft PR #9** against main (opened 2026-10-26); the AI provider layer is draft PR #10 against `feat/sela-v3-pilot`, the research and reading work is draft PR #11 against main, and ruleset `-5` is draft PR #8 against main. A row whose evidence is the PR is checked against PR #9.

| ID | Requirement | Planned evidence | Status |
| --- | --- | --- | --- |
| DL-1 | Commits use the verified YanIanZ identity: no invented email and no agent attribution or co-author lines (D-021) | `git log --format='%an <%ae>%n%b'` over the branch, checked at every PR update | in progress: the v3 commits `4cadbfd`, `0ff3341` and `59dba14` use the same name and email as the owner's commits on main and carry no co-author line (checked 2026-10-26); re-checked at every PR update |
| DL-2 | Commits are unsigned, and no document, PR or report claims a Verified signature | `git log --show-signature`; review of the PR text | in progress: the v3 commits `4cadbfd`, `0ff3341` and `59dba14` are unsigned (`%G?` = `N`; GitHub reports `verified: false` for those three commits of PR #9, checked 2026-10-26); the description of PR #9 claims no signature; re-checked at every PR update |
| DL-3 | Each PR states the problem, the outcome, the acceptance criteria met, the verification evidence, migrations and limitations | The PR description. The [PR template](../.github/PULL_REQUEST_TEMPLATE.md) has Change, Validation and Review notes sections, but no "acceptance criteria met" or "limitations" section yet | in progress: the description of draft PR #9 states the problem, the outcome, the validation, the migration and the residual risks, and the PM adds the acceptance criteria met, ADR-0002 F-01 to F-47 for the backend, with the web parts of F-17 and F-33 following with the frontend PR (read from GitHub on 2026-10-26: "Acceptance criteria met: ADR-0002 F-01 to F-47, backend part"); the SPEC §G implemented-gate point is closed, confirmed by the PM ([D-056](DECISION_LOG.md#d-056); [SPEC §G foundation status note](../SPEC.md#foundation-status-note-2026-10-26)); the template sections are still missing |
| DL-4 | Draft PR from `feat/sela-v3-pilot` (D-034). D-034 planned it stacked on PR #6 (base `feat/v2-go-meta-runtime`); PR #6 is merged, and the PM opened the v3 PR against main (D-034 status, [conflict matrix](CONFLICT_MATRIX.md#documentation-and-contract-follow-ups-open) row 63) | The PR's base branch | in progress: draft PR #9, base main, opened 2026-10-26 and not merged, with the integration pass committed on top of `59dba14`; the AI provider layer is draft PR #10, base `feat/sela-v3-pilot` |
| DL-5 | No self-review or agent review is presented as independent approval; reviews say who reviewed and how | Review records in the PR; S-195 criterion 3; S-266 criterion 2 | planned |
| DL-6 | Branch protection is respected: no force-push, no history rewrite, no overwrite of the owner's work | Branch history (additive commits only) | planned |
| DL-7 | Merge only with valid authorization | The merge record names the authorization | planned |

### 1.4 Quality gates (brief §13)

Each gate needs a recorded run (command, date and result) in [implementation status](IMPLEMENTATION_STATUS.md) before an item that depends on it is `verified locally`; an agent's report alone is not evidence. Engineering tests on synthetic data are never presented as clinical evidence, and agent reviews are never presented as user research. The v2 baseline for each gate is in §2 and in [implementation status §4](IMPLEMENTATION_STATUS.md#4-v2-evidence-that-remains-valid-v2-scope).

| ID | Gate (brief §13) | v3 evidence required | Items | Status |
| --- | --- | --- | --- | --- |
| QG-1 | Build, lint and type checks | `gofmt -l cmd internal`, `go vet ./...`, the approved-module check (F-46), web `npm run typecheck` and `npm run build`, locally and in CI (`.github/workflows/v2.yml`) | S-260, every code item | foundation verified locally: `gofmt` and `go vet` clean at `0ff3341` and F-46 holds (implementation status §1); the CI run on draft PR #9 passed (`v2.yml` run 40405475962); the web checks run with the frontend |
| QG-2 | Unit and integration tests | `go test -race -count=1 ./...` against PostgreSQL 16 with `SELA_REQUIRE_DB_TESTS=1` (F-09); Vitest; `tests/research`; `tests/modal` | F-01 to F-47; each item's criteria | foundation verified locally: `go test -race ./...` against PostgreSQL 16 passes in every package at `0ff3341` (implementation status §1); the CI run on draft PR #9 passed (`v2.yml` run 40405475962); the feature tests come with each item |
| QG-3 | End-to-end flow | `e2e/journey.e2e.mjs` extended with the P2, P3 and P4 exit journeys of [backlog §4.2](BACKLOG.md#42-items-and-exit-criteria), on the labelled test double | S-194 and the journey items | planned |
| QG-4 | Data isolation and authorization | Cross-user reads, updates and deletes return not found for every feature (F-34, BASE-10); owner and recipient principals are separated (S-120); eval `authorization.violations = 0`; operator commands are audited and never show decrypted content (S-274, SPEC V32) | S-120, S-266, S-274, every owned record | foundation verified locally: F-34 (`internal/account/isolation_test.go`, `TestV3CrossUserNotFound`) and F-45 pass; recipients, the features and S-274 planned |
| QG-5 | Consent across providers | F-35; `routing.provider_calls_without_consent = 0`, including withdrawal mid-operation; `routing.deepseek_calls_minor_bands = 0` (EV-7) | S-171, S-220, S-224, S-210 | in progress: the consent records of the foundation are verified locally (F-35); routing is in progress on a separate branch, not merged |
| QG-6 | Sharing, expiry and revocation | Ported `TestTokenAloneIsNotEnoughAndLockout`, `TestRevokeExpiryAndDeleteEndRecipientAccess` and `TestIdempotentHandoffUnderConcurrency`; revocation race tests; the P4 e2e journey | S-100, S-101, S-120 | planned |
| QG-7 | Export and deletion | F-08, F-14, F-39, F-40; per-feature derivative-deletion tests; export injection tests; a restore honours every deletion recorded in the newest key dump (F-44), with the residual window of an account deleted after it accepted for a controlled pilot (D-055) | S-174, S-175, S-262 | foundation verified locally: F-08, F-14, F-39, F-40 and F-44 pass at `0ff3341` (implementation status §1); per-feature tests planned |
| QG-8 | Memory correction | A correction keeps the earlier version; a forgotten item is absent from the captured next request; unconfirmed cards are never used; "don't save" conversations never feed memory (DM-4) and never reach the database (D-051) | S-020, S-023, S-080 | planned |
| QG-9 | Model, verifier and fallback failure | Fault tests for outage, timeout, malformed output, refusal, truncation and database failure; no fallback after a content failure; verifier unavailable means abstain; the AI-off and outage paths in e2e | S-024, S-220, S-222, S-224 (EV-7) | planned |
| QG-10 | Citation grounding | Meaning Lock checks on cards and summary wording; VC-6 and VC-8 tests; every `supported` verdict cites its own span; claim-support records for research statements and reading items | S-023, S-091, S-222, S-245 | planned |
| QG-11 | Prompt injection | EV-0 with at least 40 prompt-injection items; user text and documents treated as data in every prompt; export and print injection tests | S-228, S-092, S-175 | planned |
| QG-12 | Indonesian and English | BASE-2; i18n parity and hard-coded string tests; F-42 for every v3 error code; code-switched tests in safety, guards and Meaning Lock; fluent-speaker review | S-193 | in progress: the foundation's error messages are verified locally (F-42); the frontend strings are in progress, uncommitted |
| QG-13 | Accessibility | BASE-3 to BASE-7 and BASE-12; the S-194 gate (axe `wcag22aa` at any impact, 320 px reflow, text spacing, unique titles, target sizes); the S-195 manual review | S-190, S-194, S-195 | in progress: the accessibility criteria document (S-190); the gate and the review planned |
| QG-14 | Browser and mobile review | BASE-1: screenshots at 320 CSS px and desktop, in ID and EN, of every screen a milestone ships, attached to its PR | S-195 (P2 to P8) | planned |
| QG-15 | Migrations and rollback | F-01 to F-07 and F-46 (up, status, dry-run, down-last; data-migration gates: backup, dry run, ownership validation, rollback; D-027); restore drill (F-44); a rollback rehearsal on staging | S-260, S-262, S-263 | foundation verified locally: F-01 to F-07, F-46 and F-44, the migration round trip and the restore drill pass at `0ff3341` (implementation status §1); the CI run on draft PR #9 passed (`v2.yml` run 40405475962); the hosted rehearsal is blocked externally (B-3, B-15) |

### 1.5 Spec constraints and invariants (v3)

Each v3 constraint (C16 to C26) and invariant (V26 to V42) of [SPEC.md](../SPEC.md#c-constraints), plus the owner's fallback constraint C13 and invariants V19–V21 that the v3 router must also satisfy, maps to the backlog items that satisfy it and the planned tests that prove it. Test names are planned, not existing, unless an ADR or this document names them; "AC n" is acceptance criterion n of the item. **Gap** marks a part of a Spec ID that no backlog criterion or planned test covers yet; each gap names who closes it.

| Spec ID | Requirement (short) | Backlog items | Planned tests and checks | Gap or note |
| --- | --- | --- | --- | --- |
| C13 | Owner's DeepSeek availability fallback: opt-in, its own consent, OFF by default (hackathon mode too), never silent, never after unsafe output (main) | S-224, S-171, S-227 | EV-7 routing thresholds; the owner's validation matrix and the ported `prototypes/generationfallback` cases (ADR-0003 §17 O-11) | The prototype is mock-tested only; the v2 runtime has no DeepSeek route (T5b open); the v3 route is planned (T18) |
| V19–V21 | Fallback only after upstream Meta availability errors; every fallback output validated and labelled `live_deepseek_fallback`; recheck before persisting; data limits (main) | S-220, S-221, S-224 | `routing.fallback_after_content_failure = 0`; S-224 AC1 to AC3; a fallback listening reply is verified, ADR-0003's means of meeting V20 for it (S-224 AC5; RR-8, §17 O-6); `routing.deepseek_listening_calls_without_claude_route = 0` (S-224 AC6; RR-4 (i)) | Real-user text to DeepSeek waits for B-10 and the owner (ADR-0003 §17 O-3) |
| C16 | Persistence and encryption; no content or secret in browser storage | S-173, S-174, S-260, S-261, S-262; BASE-10, BASE-11 | F-01 to F-09, F-12 to F-15, F-44, F-46; F-10 (memory store refused in production, `sslmode`); the BASE-11 browser-storage scan after each journey | — |
| C17 | Accounts and guest mode; no absolute anonymity | S-002, S-003, S-004, S-172; S-179 (post-pilot, B-11) | F-16 to F-33; S-002 AC2 forbidden-phrase check | — |
| C18 | Multi-provider AI with a consent per provider; every output names its provider | S-006, S-171, S-177, S-220, S-221, S-222, S-224, S-226, S-227 | F-35; EV-7 `routing.provider_calls_without_consent = 0`, including withdrawal mid-operation; S-006 AC2 disclosure test against `GET /api/v3/policies`; S-221 AC2 provider label; S-227 AC1 zero Anthropic and DeepSeek calls | Live parts blocked externally (B-1, B-8, B-9) |
| C19 | Minors built but gated; under 15 refused | S-005, S-210, S-211, S-212, S-213 | F-36, F-37; F-10 (production refuses `15_17`); `routing.deepseek_calls_minor_bands = 0`; S-210 AC3 no-guardian-route test; S-212 AC3 15-year-old refusal | Production decision blocked externally (B-10); 15 versus 16–17 open ([conflict matrix](CONFLICT_MATRIX.md) row 49) |
| C20 | No engagement pressure; no push notifications in the pilot | BASE-8; S-040, S-041, S-053, S-131, S-141 | BASE-8 claim-gate scan and the Notification and Push API code scan; S-040 AC2 and S-141 AC3 copy tests; S-053 AC1 period-only test | — |
| C21 | Content separation (public, personal, community, demo, research, evaluation, training) | S-140, S-142, S-228, S-229, S-240 to S-246, S-261; BASE-10 | S-140 AC2 cards by ID only; S-261 AC4 (demo fixtures load only in the demo); F-34 per owned record | **Gap:** no planned test that the research corpus and the evaluation sets hold no personal record and are never joined to one; separation is by construction (separate stores, D-033). The AI engineer and the research engineer add a check with S-228 and S-244 |
| C22 | Honest terms; recipients unverified; no invented examples | BASE-8; S-001, S-090, S-100, S-101, S-120, S-121, S-130 | Claim gates (forbidden terms); S-090 AC5 and S-120 AC1 labels; S-100 AC3 "identity not verified"; S-101 AC4 and S-121 AC2 status wording; S-130 AC3 | — |
| C23 | No clinical claims or telemedicine; never contacts anyone automatically | BASE-8; S-021, S-090, S-151, S-222, S-223; S-122 (post-pilot) | S-223 AC1 hard-guard tests (diagnosis, medication directive, professional persona); S-021 AC1; claim gates; S-151 AC2 zero provider calls during a crisis hold; S-266 AC3 outbound-destination check ("never contacts anyone automatically") | — |
| C24 | Real use; synthetic data only in tests, evaluation and the demo; five environments | S-131, S-261, S-263 | F-10, F-11; S-261 AC2 and AC3 (the demo works for real; AI on live providers with synthetic data or labelled test doubles); S-131 AC1 no fixture plan in production | F-10 counts only once ADR-0002 §12 is aligned with D-046 ([conflict matrix](CONFLICT_MATRIX.md) row 52) |
| C25 | Pilot scope with reasons; nothing cut silently | S-025, S-026, S-027 (retired, D-060); S-142 (paused); S-182 (deferred); S-220 (no reply cache); S-229 | [Backlog §6](BACKLOG.md#6-feature-matrix-coverage) maps every feature-matrix row to an item or a recorded retirement (document review); voice/TTS retired by D-060 | — |
| C26 | WCAG 2.2 AA on every surface and for every age | BASE-1 to BASE-7, BASE-12; S-190, S-191, S-194, S-195, S-196 | S-194 automated gate; S-195 manual review; [accessibility criteria](design/ACCESSIBILITY_CRITERIA.md) §6 and §7 | The independent accessibility audit is a public-release gate ([roadmap §2](ROADMAP.md#2-release-ladder)) |
| V26 | Listening: no profile, optional intent, replies that listen, End in one action, correctable cards, never pushed to a professional, no canned empathy | S-007, S-020, S-021, S-023, S-024 | S-020 AC1 (at most four screens, no profile) and AC3 (End); S-007 AC1; S-021 AC1 to AC3 and AC6; S-023 AC2 to AC4; S-024 AC2 (honest unavailable state) and AC3 (no canned reply) | Live reply quality blocked externally (B-1) |
| V27 | Storage choice before any content; "don't save" held only in bounded server memory and purged (D-051) | S-001, S-006, S-020, S-050, S-060, S-170, S-080, S-226; BASE-11 | S-001 AC3 and S-006 AC4 zero rows; S-020 AC2 (no message row during or after; fake-clock and restart purge tests; reload reopen); S-050 AC1; S-170 AC1; S-080 AC4 (DM-4); the BASE-11 browser-storage scan | — |
| V28 | Provenance classes, versions and review status; originals never overwritten | BASE-10; S-023, S-050, S-080, S-090, S-130 | BASE-10 reserved-class constraint test; S-023 AC2; S-050 AC3; S-080 AC1 and AC2; S-090 AC2; S-130 AC2 | — |
| V29 | Transparent, correctable memory; no hidden profile | S-080, S-081, S-210; S-020, S-050 (derivative deletion) | S-080 AC1 to AC5 (forgotten item absent from the captured request; no consent, no memory); S-081 AC1; S-020 AC5 and S-050 AC4 | — |
| V30 | Sharing consent by content, recipient and purpose; statuses that never imply care | S-090, S-100, S-101, S-120, S-121 | S-100 AC1 to AC3; S-090 AC1, AC4 and AC6; S-101 AC1, AC4 and AC7; S-120 AC1 | — |
| V31 | Revocation ends future access and says copies cannot be recalled | S-092, S-101, S-120, S-174, S-175, S-177 | S-101 AC2 (race test) and AC3; S-120 AC3; S-175 AC3; S-092 AC2 | — |
| V32 | Access audit, including operator access (D-052) | S-092, S-101, S-121, S-175, S-176, S-274 | F-38; S-176 AC1 recipient-open test; S-101 AC5; S-121 AC1; S-274 AC1 to AC6 | Where the operator record is kept is open ([conflict matrix](CONFLICT_MATRIX.md) row 57); the hosted access log needs B-3 |
| V33 | Isolation: owner only from the cookie; another environment's database or key refused | BASE-10; S-120, S-261, S-266 | F-34 per owned record; F-45; S-120 AC2 principal separation; F-10 key-id prefix; F-11 environment marker | — |
| V34 | Deletion with derivatives; key destroyed; backups age out; a restore honours every deletion in the newest key dump, with the stated residual window | S-020, S-050, S-090, S-174, S-177, S-262 | F-08, F-14, F-40, F-44 (verified locally); S-174 AC4 per-feature derivative tests; S-020 AC5; S-050 AC4; S-090 AC6; `TestTermsPrivacyStatesBackupWindows` (the privacy text states the window) | Reconciled: V34 now states the window of [ADR-0002](adr/ADR-0002-persistence-accounts-and-encryption.md) §11 (an account deleted after the newest key dump can come back after a total database loss until a deletion journal exists), accepted for a controlled pilot and revisited before a public release (D-055); the journal needs B-3 |
| V35 | Routing and provenance; DeepSeek only after an availability failure; never silent | S-024, S-220, S-221, S-224, S-227 | EV-7 thresholds at zero; S-220 AC2 to AC5; S-221 AC1 and AC2; S-224 AC1 to AC3 | Live parts blocked externally (B-1, B-9) |
| V36 | Verifier: own consent, release only `supported`, no model verifies itself, no chain-of-thought | S-081, S-091, S-222, S-245; S-225 (post-pilot) | VC-6 and VC-8 tests (S-222 AC1, AC4); `routing.self_verification = 0`; `routing.verifier_gated_released_without_check = 0`; `routing.deepseek_listening_calls_without_claude_route = 0` (S-224 AC6); S-222 AC3 no-reasoning storage scan; S-091 AC1; live EV-2, EV-3 | Live parts blocked externally (B-8) |
| V37 | Minors: self-declared band; `unknown` gets the protective defaults; no guardian access; no DeepSeek | S-005, S-080, S-081, S-210, S-211, S-212, S-213 | F-36, F-37; S-005 AC1 and AC2; S-210 AC1 to AC5; S-080 AC5; S-081 AC3; S-212 AC2 and AC3 | Production use blocked externally (B-10) |
| V38 | Crisis handling: screen before storage and AI; keep the words; pause AI; second safeguards | S-150, S-151, S-152, S-153, S-222, S-223 | S-150 AC1, AC2 (`safety.model_calls_on_crisis_path = 0`) and AC5 (ruleset version recorded); S-151 AC1 to AC5; S-153 AC1; BASE-9 | Ruleset -5 is in progress: a WIP snapshot is committed on `fix/safety-ruleset-5` (`f619677`, draft PR #8 to main), re-verification pending, and the fix loop continues in the working tree of that branch (D-034 status; S-150) |
| V39 | Verified help resources; server-enforced re-verify-by; one catalogue | S-103, S-152, S-154, S-210 | S-152 AC2 (fake-clock test), AC3 and AC5 (one catalogue); S-103 AC1 | Re-verification blocked externally (B-5) |
| V40 | Export: readable and structured, escaped, audited, no secrets, copies cannot be recalled | S-092, S-175 | F-39; S-175 AC2 injection test and AC3; S-092 AC1; F-38 export event | — |
| V41 | Data minimisation for AI providers | S-080, S-104, S-226 | S-226 AC1 to AC3 (captured requests, placeholders, log canary); S-104 AC2; S-080 AC4 (DM-4) | — |
| V42 | Environment separation; the demo works for real on synthetic data | S-261, S-263 | F-10, F-11; S-261 AC2 to AC4; S-263 AC2 | F-10 counts only once ADR-0002 §12 is aligned with D-046 ([conflict matrix](CONFLICT_MATRIX.md) row 52) |

## 2. v2 scope (branch `feat/v2-go-meta-runtime`)

The rows below map every in-scope requirement of the v2 implementation brief to code, tests and evidence on the v2 branch (PR #6, merged into main as `7fc6678` on 2026-10-26). They stay true for v2 and are the baseline that v3 keeps passing or replaces explicitly. Rows A3, A6, A7, B1, B3, F7, BE8, BE9 and M13 describe v2 limits (anonymous demo identity, memory-only retention, synthetic adults, fictional practice text, no chat surface, fictional professionals, memory store, Meta-only) that v3 supersedes as product scope (D-026, D-027, D-029, D-030, D-035); the v3 requirements are in §1.

**Status values (v2).** `planned` (not built) · `implemented` (code exists, no automated proof) · `tested locally` (automated tests or evaluation pass in this repository; CI runs the same commands except `sela-eval`) · `verified in deployment` (observed on a deployed environment) · `blocked externally` (needs an owner action, budget or account; see [blockers](BLOCKERS.md)). Nothing is deployed yet, so no row is `verified in deployment`. "Test double" means `cmd/fakellm`, which is never Meta inference.

Test names are Go tests unless prefixed: `web:` = Vitest in `web/src/test`, `e2e:` = `e2e/journey.e2e.mjs`, `eval:` = `cmd/sela-eval` metric, `py:` = Python unittest.

### A. Onboarding, consent and data

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| A1 | Calm, accessible, mobile-first start | `web/src/pages/Welcome.tsx`, `Start.tsx`, `styles.css` | e2e: public pages axe WCAG 2.1 A/AA = 0 violations, 390 px no horizontal scroll; e2e: keyboard-only start | tested locally |
| A2 | Indonesian and English everywhere | `web/src/i18n/{id,en}.ts`, `internal/httpapi/errors.go` | web: `i18n.test.ts` (key parity, placeholders, error codes), `hardcoded.test.ts`; `TestLocalizedErrorsAndSessionMissing`; e2e ID and EN journeys | tested locally |
| A3 | Anonymous or pseudonym; no real identity | `journey/session.go` (`validIdentity`), `Settings.tsx` | `TestCreateSessionValidation` (journey) | tested locally |
| A4 | Honest data explanation | `fixtures/data/policies.json`, `Privacy.tsx`, `Start.tsx` | Policies versioned and validated at load (`fixtures` tests); e2e start flow | tested locally |
| A5 | Separate consents for AI processing, storage and sharing | `journey/session.go` (`SetConsent`), `handoff.go` (share consent + policy version) | `TestConsentsAreIndependentAndRetentionWorks`, `TestShareRequiresExplicitConsentAndCurrentPolicy`, `TestNoAIConsentManualPathReachesFollowUp` | tested locally |
| A6 | Working retention and deletion | `journey/store.go`, `store/memory.go` (sweeper, idle/absolute expiry), `DeleteSession` | `TestSessionIdleExpiryPurges`, `TestRevokeExpiryAndDeleteEndRecipientAccess`, store tests, e2e settings deletion | tested locally |
| A7 | Synthetic adult demo only (adult attestation, fictional practice text) | `CreateSession` (`AdultAttested`), `SubmitStory` practice bounds, D-003 | `TestCreateSessionValidation`, `TestPracticeTextBoundsAndContactDetails`, e2e contact-detail rejection | tested locally |

### B. Story room

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| B1 | Text input (sample scenarios or fictional practice story) | `journey/story.go`, `Story.tsx` | e2e journeys; `TestPracticeTextBoundsAndContactDetails` | tested locally |
| B2 | Streaming, cancel, retry, timeout, honest errors | `journey/ops.go`, `httpapi/sse.go`, `web/src/lib/operation.ts`, `Ledger.tsx` | `TestFullJourneyOverHTTP` (SSE replay with Last-Event-ID), `TestCancelOperation`, `TestStaleAIResultIsDiscarded`, `TestProviderUnavailableAndDisabled`, `TestModalColdStartRetries`; eval fault battery (timeout, stall, 503/429/401, truncated) | tested locally (test double) |
| B3 | No emotional dependence; no chatbot persona | No chat surface; copy in i18n; prompts (`journey/prompts.go`) | web: `claims.test.ts` (forbidden-claim gates incl. therapy/counselling) | tested locally (see note in status) |
| B4 | Does not impersonate professionals | Recipient view labelled fictional; `fixtures` require "(fiktif)/(fictional)" names | `fixtures` validation tests; e2e recipient view label | tested locally |

### C. Story Ledger (Care Passport)

Lineage: the source-grounded Care Passport and clarification flow come from Dengar.in v1 (`services/validator/src/carePassport.ts`); v2 ports and extends them ([provenance](SELA_PROVENANCE.md)).

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| C1 | Structured, sourced summary with verbatim quotes and offsets | `internal/ledger` (`ValidateDraft`, `SplitSegments`), `journey/ledger_ops.go` | ledger unit + fuzz tests; `TestAIDraftNeedsReviewEditInvalidatesApproval`; eval `ledger.residual_unsupported_rate` | tested locally |
| C2 | Separate user facts, model interpretation and unknowns | `Item.Quote` vs `Item.Wording` + `WordingOrigin`, `Ledger.Unknowns` | `Ledger.tsx` labels; journey tests | tested locally |
| C3 | Clarification question | `ledger.ComputeUnknowns`, `journey.Clarify` | `TestClarificationAnswerAndSkip`, `TestClarificationRequestRequiresNewShare` | tested locally |
| C4 | Correct, delete, approve | `ReviewItem` (confirm/use_quote/edit/remove/set_kind), `Approve` | `TestAIDraftNeedsReviewEditInvalidatesApproval`, e2e ID journey | tested locally |
| C5 | Versioning invalidates approval and stale shares | `changeLedger`, approval hash, `CreateHandoff` ledger-version check | `TestAIDraftNeedsReviewEditInvalidatesApproval`, `TestStaleVersionRejectedWithoutSideEffects` | tested locally |
| C6 | No invented diagnosis, history or events (Meaning Lock) | `ledger/validate.go`, `clinical.go`, `qualifiers.go` | ledger tests; `TestMeaningLockRejectsFabricatedAndUnsafeModelOutput`; eval `meaning_lock.*` and fault battery; adversarial review findings fixed (see status) | tested locally |

### D. Next Step

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| D1 | Deterministic options with simple rationale | `journey/nextstep.go` | `TestNextStepsAreDeterministicAndHonestWhenEmpty` | tested locally |
| D2 | No diagnosis, triage or treatment | No model in Next Step; fixtures; claim gates | web: `claims.test.ts` (triage/diagnosis gates) | tested locally |
| D3 | User decides; honest empty state; non-handoff steps | `NextStep.tsx`, `nextSteps` fixtures | `TestNextStepsAreDeterministicAndHonestWhenEmpty`, e2e | tested locally |

### E. Opening Practice

Lineage: the opening-lines pipeline comes from Dengar.in v1 (`services/validator/src/openingLines.ts`), ported and extended.

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| E1 | Grounded in the approved note | `ledger.ValidateOpening`, `TemplateOpening`, `journey/opening.go` | ledger opening tests; `TestOpeningPracticeTemplateAndCoverage`; e2e ID journey practice step | tested locally |
| E2 | ID/EN and code-switching | ledger opening tests (code-switched cases) | ledger tests | tested locally |
| E3 | Non-manipulative (no pressure language) | `ledger/opening.go` pressure lexicon | ledger opening tests | tested locally |
| E4 | Not therapy; rehearsal not stored, no score | `journey.Practice` (attempt not stored), `Practice.tsx` ("mentioned / not yet") | journey test; e2e | tested locally |

### F. Handoff and continuity

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| F1 | Exact recipient preview | `PreviewHandoff`, `Share.tsx`, `components/Projection.tsx` | `TestFullJourneyOverHTTP` (preview equals stored projection) | tested locally |
| F2 | Explicit, scoped, versioned share consent | `HandoffInput.ShareConsent`, `SharingPolicy`, field selection | `TestShareRequiresExplicitConsentAndCurrentPolicy` | tested locally |
| F3 | Link token with expiry and revocation | 256-bit token in URL fragment, `RevokeShare`, 1/24/72 h | `TestRevokeExpiryAndDeleteEndRecipientAccess`, e2e revoke | tested locally |
| F4 | Recipient authorization; token alone is not access | token + 8-character code, scoped recipient cookie, lockout | `TestTokenAloneIsNotEnoughAndLockout`; eval `authorization.violations = 0` | tested locally |
| F5 | Receipt and acknowledgement | `HandoffView`, `RecipientAct` | e2e ID journey (ack, complete) | tested locally |
| F6 | Follow-up never equates sent with treated | `CareReceived` always false, copy | e2e ("sent does not mean care"); web: `claims.test.ts` | tested locally |
| F7 | Fictional professionals and synthetic data labelled | fixtures validation, demo banner | `fixtures` tests; e2e | tested locally |
| F8 | Idempotent handoff creation | `Idempotency-Key`, payload hash | `TestIdempotentHandoffUnderConcurrency`, HTTP replay test | tested locally |
| F9 | Never contacts real professionals | No outbound integration exists; recipients are per-owner fixtures | Architecture review; `TestSessionsAreIsolated` | tested locally |

### G. Safety

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| G1 | Safety routing before inference | `journey.assessText` before any op; `startOp` crisis gate | `TestCrisisStoryInterruptsBeforeAnyModelCallOrHandoff`; eval `safety.model_calls_on_crisis_path = 0` | tested locally |
| G2 | Deterministic path for crisis and model failure | `internal/safety`, manual ledger path | `TestNoAIConsentManualPathReachesFollowUp`, `TestProviderUnavailableAndDisabled`, e2e crisis | tested locally |
| G3 | Classifier is never the sole safeguard | Lexicon + interruption + human review of every item + no model in routing (D-017) | `internal/safety` tests | tested locally |
| G4 | No diagnosis or safety promises | Meaning Lock clinical checks; copy | ledger clinical tests; web: `claims.test.ts` | tested locally |
| G5 | Verified, maintained local references; no invented numbers | `fixtures/data/help.json` (verified_official only, dates, re-verify by) | `fixtures` tests; re-verification due 2027-01-14 ([B-5](BLOCKERS.md)) | tested locally |
| G6 | Negation, quotes, slang, code-switching, ambiguity tested | `internal/safety/testdata/vectors.json` (1329 vectors, ruleset `sela-safety-2026.10.26-4`) | `TestVectors*`, `TestV1Parity`; eval `safety.*` | tested locally |
| G7 | No sensitive content in operational logs | request log fields only; `llm.Observer` metadata; counters instead of safety logs | log canary in `TestFullJourneyOverHTTP` and e2e | tested locally |

### Backend (Go)

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| BE1 | Modules with clear boundaries | `internal/{config,fixtures,journey,ledger,llm,research,safety,store,httpapi}` | [architecture](ARCHITECTURE.md) | implemented |
| BE2 | Documented API | [API spec](API_SPEC.md) | `TestFullJourneyOverHTTP` exercises it | tested locally |
| BE3 | Validation and structured errors | strict JSON decode, `{"error":{code,message,field}}` | `TestSessionCookieAttributesAndStrictJSON`, `TestLocalizedErrorsAndSessionMissing` | tested locally |
| BE4 | Context cancellation, timeouts, limited retry | `journey/ops.go` (`context.WithCancelCause`, op timeout, cold-start retry), `internal/llm` (attempt timeout, retries before first token only) | `TestCancelOperation`, `TestModalColdStartRetries`, `internal/llm` tests | tested locally |
| BE5 | Idempotency | handoff idempotency keys | `TestIdempotentHandoffUnderConcurrency` | tested locally |
| BE6 | Authentication and authorization | session cookie (HttpOnly, SameSite=Strict, Path), recipient principal, same-origin guard | `TestMutationGuardRejectsCrossSite`, `TestSessionCookieAttributesAndStrictJSON`, eval authorization battery | tested locally |
| BE7 | Consent versioning, revocation and deletion | `Consent.PolicyVersion`, `Revision`, withdrawal cancels ops | `TestConsentsAreIndependentAndRetentionWorks`, `TestStaleAIResultIsDiscarded` | tested locally |
| BE8 | Storage abstraction with memory store; no implicit sensitive persistence | `journey.Store`, `store.Memory` (bounded, LRU idle eviction) | store tests, `TestStoreCapacityNeverEvictsActiveSessions`, `TestLiveSessionsArePerClientBounded` (store) and `TestLiveSessionsPerClientAreBounded` (HTTP, 429 `rate_limited`) | tested locally |
| BE9 | Meta/Modal provider abstraction, no silent non-Meta fallback | `llm.Provider`; only `modal`, `test_double` (non-production only), `disabled` | `internal/llm` tests; config tests (`TestTestDoubleIsRefusedInProductionAndWithoutOptIn`) | tested locally |
| BE10 | Sanitised logging | `httpapi/server.go` request log, `cmd/sela` observer | log canary tests | tested locally |
| BE11 | Health and readiness | `/api/v2/health`, `/api/v2/ready` | `TestFullJourneyOverHTTP`; CI container job | tested locally |
| BE12 | Concurrency and race tests | per-session locks, admission | `go test -race ./...`, `TestIdempotentHandoffUnderConcurrency`, store race tests | tested locally |
| BE13 | Static frontend cannot serve files outside its directory | `httpapi/static.go` (`os.Root`, `fs.ValidPath`, SPA fallback) | `TestStaticServingAndSPAFallback`, `TestStaticServingRefusesSymlinkEscape` | tested locally |

### Frontend

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| FE1 | Rebuilt from the journey | `web/src/pages/*` (React 19 + TypeScript + Vite) | e2e journeys | tested locally |
| FE2 | Responsive and keyboard accessible | `styles.css` (44 px targets, focus-visible, reduced motion) | e2e axe on 15 pages, 390 px, keyboard start | tested locally |
| FE3 | Loading, empty, error, expired, revoked and offline states | `session.tsx`, `Layout.tsx` banners, `Recipient.tsx` (access ended), `SessionEnded` | e2e revoke/deletion; web component tests | tested locally |
| FE4 | i18n with no stray strings | i18n dictionaries | web: `i18n.test.ts`, `hardcoded.test.ts` | tested locally |
| FE5 | No placeholders shown as features | Only implemented features are routed; AI footer reflects the real provider mode | e2e; review | tested locally |

### Meta on Modal

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| M1 | ADR with pinned model ID and revision, licence, capability, cost, hackathon fit | [ADR-0001](adr/ADR-0001-meta-open-weight-model-on-modal.md), [model notes](../modal/MODEL_NOTES.md) | Sources cited; hackathon tooling eligibility unverified ([B-2](BLOCKERS.md)) | implemented |
| M2 | Reproducible Modal image | `modal/sela_llm.py` (pinned vLLM image digest, pinned client) | py: `tests/modal` (72) | tested locally (build blocked externally, [B-1](BLOCKERS.md)) |
| M3 | Safe model download and cache | `download_weights` with per-file digests and a verified marker | py: `tests/modal` | tested locally (run blocked externally) |
| M4 | Secrets via a secret manager | Modal Secret `sela-llm-api`; Go reads env only; no secrets committed | py: `tests/modal`; config tests | tested locally |
| M5 | Authenticated endpoint | Modal proxy auth + vLLM key | `modal/smoke_test.py` checks; live layered-auth unverified | blocked externally |
| M6 | Streaming | `internal/llm/stream.go` (SSE, `delta.reasoning` ignored) | `internal/llm` tests with the test double | tested locally (test double) |
| M7 | Limits, concurrency, timeout, retry | `max_containers=1`, `max_concurrency=8`, Go admission, attempt timeouts | `internal/llm` tests, `TestModalColdStartRetries` | tested locally |
| M8 | Scale to zero and container cap | `min_containers=0`, `max_containers=1`, `scaledown_window=300` | py: `tests/modal` | tested locally (deployment blocked) |
| M9 | Cold start handling | `journey.generate` retries Modal 503 within budget; UI "warming" state | `TestModalColdStartRetries` | tested locally |
| M10 | dev/staging environments | `modal deploy -e dev\|staging`, `.github/workflows/modal-deploy.yml` (manual) | runbook | implemented |
| M11 | Smoke test and rollback | `modal/smoke_test.py`, [runbook](RUNBOOK.md) | py tests against a local fake server; live run blocked | blocked externally |
| M12 | Owner configuration guide | [modal/README.md](../modal/README.md), [runbook](RUNBOOK.md) | — | implemented |
| M13 | No silent fallback to non-Meta providers | provider modes; `live_meta` provenance only when the server reports the pinned served name | `internal/llm` tests (model mismatch), eval fault `wrong_model` | tested locally |
| M14 | Meta used substantively (extraction, wording, opening lines) | `journey/prompts.go`, ledger and opening operations | test double only; live quality pending | blocked externally |

### Research

| ID | Requirement | Code | Evidence | Status |
| --- | --- | --- | --- | --- |
| R1 | Reproducible pipeline, steps 1–10 | `scripts/research/corpus_pipeline.py`, `scripts/research/corpus/` | py: `tests/research` (74); offline build byte-identical twice; `verify` 0 violations | tested locally |
| R2 | Rolling cutoff = freeze date − 5 years; snapshots kept | `manifest.json` (`freeze_date`, `cutoff_date`), `snapshots/2026-10-26/` | py tests (snapshot conflict refuses overwrite) | tested locally |
| R3 | Minimal schema, null when unknown, no guessed results | `records.jsonl` | `verify` schema/invariant checks | tested locally |
| R4 | Licence and access status; no paywall bypass | `rights.py`; only open-access full text from Europe PMC | `records.jsonl` fields; py tests | tested locally |
| R5 | No copyrighted full text in Git | full text only in the gitignored cache; snippets ≤240 chars only for CC BY/BY-SA/CC0 | `.gitignore`; `verify` | tested locally |
| R6 | Traceable extraction (value, snippet, locator, status) | `extract.py` | py tests; 694 evidence items, all `machine_extracted_unreviewed` | tested locally |
| R7 | Reviewer status recorded honestly | `reviewer_status = not_reviewed` for all records | `verify` | tested locally |
| R8 | No automatic fine-tuning | D-016; no training code | — | tested locally (by absence; documented) |
| R9 | Reviewer corpus search without conversation data | `internal/research` (BM25), `/api/v2/research/*`, `Research.tsx` | `internal/research` tests; `TestResearchUnavailableWithoutIndexAndPublicEndpoints`; eval `retrieval.*` | tested locally |
| R10 | Documents are data, not instructions; citation validity | Research data never reaches a model; eval citation check over sela-relay docs | eval `retrieval.citation_validity = 1.0` | tested locally |
| R11 | Human appraisal of evidence | — | Not done; needs a qualified reviewer ([B-4](BLOCKERS.md)) | blocked externally |

### Evaluation

Thresholds pre-registered in `eval/thresholds.json` (commit e23e0e0) before any final run.

| ID | Metric family | Evidence | Status |
| --- | --- | --- | --- |
| EV1 | Extraction accuracy, unsupported statements, grounding | eval `ledger.*`, `meaning_lock.*` (offline, test double) | tested locally; live coverage blocked ([B-1](BLOCKERS.md)) |
| EV2 | Correction/approval | journey tests | tested locally |
| EV3 | Cross-session leakage | eval `authorization.violations`, `TestSessionsAreIsolated` | tested locally |
| EV4 | Sharing scope, expiry, revocation | eval authorization battery | tested locally |
| EV5 | ID/EN consistency | i18n parity; live bilingual wording blocked | partly blocked externally |
| EV6 | Safety routing and model failure | eval `safety.*`, fault battery | tested locally |
| EV7 | Retrieval relevance and citation validity | eval `retrieval.*` | tested locally |
| EV8 | Prompt injection | fault `prompt_injection_echo`; prompts treat input as data | tested locally (test double) |
| EV9 | Latency, cold start, cost | eval live mode only | blocked externally |
| EV10 | Accessibility | e2e axe | tested locally |
| EV11 | Full scenario frontend → Go → Meta on Modal → approval → handoff → acknowledgement | e2e with test double; live run blocked | blocked externally (test double passes) |

### GitHub workflow

| ID | Requirement | Evidence | Status |
| --- | --- | --- | --- |
| GH1 | Verified YanIanZ identity, unsigned, no agent co-author lines | D-021; `git log` of the PR branch | implemented |
| GH2 | Feature branch from latest main, small commits | `feat/v2-go-meta-runtime` | implemented |
| GH3 | Checks before push; PR with problem, changes, validation, limitations | PR description; CI `v2.yml` | implemented |
| GH4 | No force-push, no history rewrite, no merge without authorization | branch history | implemented |
