# Production QA report — SELA: Beyond Being Heard

Checked on **2026-11-02** against repository commit **`dfdffd0`**, which is also the version production reports at `https://sela.vertalt.my.id/api/v2/health` (`{"mode":"production","version":"dfdffd0"}`, fetched 2026-11-02 22:25 WIB). Status words:

- **Verified** — the product behaviour is implemented at `dfdffd0` and asserted by a named test; the footage is the real SELA UI.
- **Verified (illustrated)** — verified behaviour, shown as a motion diagram rather than a screen recording.
- **Proposed** — not implemented; would need a "proposed" label if shown. *(None remain — see below.)*
- **Missing asset** — a shot the brief asked for that this build does not contain.

## How the product footage was made (read this first)

| Item | Detail |
| --- | --- |
| UI source | `web/src` at `dfdffd0`, unchanged, bundled with esbuild by `capture/build-web.mjs` into `video/.cache/web` (never `web/dist`). |
| Data | Synthetic "Nadia" fixture (`capture/fixtures.mjs`, from `docs/demo/SYNTHETIC_NADIA_FIXTURE.md`), served to the real client through Playwright request interception. No account, no network, no database. |
| Why not the live site | This build sandbox cannot reach `sela.vertalt.my.id` (egress proxy refuses the host) and cannot install Go 1.26 or the npm registry, so a local backend could not run. The fixture router answers with the response shapes the client reads; **backend effects (stale, revoked, cancelled) are not claimed from the fixture** — they are claimed from the Go tests below. |
| Meaning Diff | The diff on screen is the real output of `internal/ledger.CompareMeaning` (stdlib-only package, run with Go 1.24) for the exact strings shown: `evidence/meaning-diff/main.go` → `output.txt`. |
| AI provenance | No Meta runtime was captured. The talk reply carries SELA's own test-double label "Sela · balasan uji (bukan AI)", and the film adds "TEST DOUBLE · NOT LIVE META OUTPUT". No shot claims a Meta-generated reply. |
| Display | Dark scheme (a real SELA setting), Indonesian locale, Asia/Jakarta time. Every product panel is captioned "SELA UI · source dfdffd0 · synthetic data". |

**Owner action before submission:** if you want live-backend footage, run the same capture against a local stack (`go run ./cmd/sela` + `capture/` pointed at `http://localhost:8080`) or a staging copy. The shot list and crops in `capture/capture.mjs` stay valid.

## Scene-by-scene status

| # | Scene | Status | Evidence | On-screen truth labels |
| --- | --- | --- | --- | --- |
| 01 | Every story matters (00:00–00:20) | Typography only | — | Fragments are fictional sentences from the Nadia fixture |
| 02 | Beyond listening (00:20–00:40) | **Verified** | Room + reflection cards: `TestV3ReflectionCardsReview`; reply label logic `web/src/v3/screens/Talk.tsx` `authorLine` | "Fictional adult · synthetic data", "TEST DOUBLE · NOT LIVE META OUTPUT"; UI copy "Ini tafsiran AI, bukan fakta" glossed in English |
| 03 | Meaning Integrity Engine (00:40–01:15) | **Verified** + **Verified (illustrated)** for the graph | Diff: `TestCompareMeaning` (`kadang`→`selalu` → `frequency`), plus `evidence/meaning-diff/output.txt`. Propagation: `TestMeaningCorrectionPropagates` (`internal/httpapi/meaning_test.go`) — preview 1 summary / 2 shares / 1 care request with no write; after correction recipient `401 invalid_share`, partner `cancelled`, summary back to `draft` with `claim_corrected`, re-approval/share/export of the stale version refused. Memory edit path invalidates: `UpdateMemory` → `invalidate(..., claim_corrected)` (`internal/account/continuity.go:743`). | "TEST FIXTURE · SIMULATED AI ERROR" (solid amber) shown **before** the wrong text; "asserted in TestMeaningCorrectionPropagates" |
| 04 | Context over time (01:15–01:40) | **Verified** — Life Context Timeline is implemented (D-081) | `TestV3ContextTimelineOwnerScopeNoAI` (0 generator calls), `TestContextTimelineReadOnly`, `TestV3ContextTimelineStaleAfterEdit`; route `/notes/context` | "NO SCORES. NO DIAGNOSIS."; UI line "Halaman ini tidak memakai AI…" glossed |
| 05 | Personal action bridge (01:40–02:00) | **Verified** — Action Bridge is implemented (D-081) | `TestActionTransitions`, `TestV3ActionStepsManualWithoutAI`, `TestV3ActionSuggestionIsProposalOnly`; vitest "never uses streak, score or success wording" | "Optional · written by Nadia"; "No reminders · No streaks · Not treatment" (real copy glossed) |
| 06 | Care passport (02:00–02:25) | **Verified** — Passport view is implemented (D-081) | `TestV3CarePassport` (recipient receives exactly the preview); `projectSummary` shared with shares/care requests; vitest "previews exactly the recipient projection and approves the exact version" | "PERSONAL PREPARATION SHEET · NOT A MEDICAL RECORD"; real UI "Bukan paspor medis" highlighted |
| 07 | Support, on your terms (02:25–02:45) | **Verified** (person side) / **Missing asset** (professional side) | `TestCareNetworkEndToEnd` ("Cancelling revokes the share"), `TestV3ShareLifecycle`, `TestRevokeExpiryAndDeleteEndRecipientAccess` | "SIMULATED PROFESSIONAL", "fictional partner · not a real clinic", "Demo only · no real appointment, partnership or care delivered", "Copies already viewed cannot be recalled" |
| 08 | Final reveal (02:45–03:00) | Brand | Logo = `web/public/icon.svg` (dark variant), wordmark in Lexend 700 as in `web/DESIGN.md` | "synthetic demo data · simulated partners · not a medical or crisis service" |

### Proposed features: all three now ship

The brief treated Life Context Timeline, Care Passport preset and Personal Action Bridge as proposals. They were merged in `360f3a7` (D-081) and are in the deployed `dfdffd0`, so the film shows the real screens instead of fallbacks. Note: `docs/demo/META_HACKATHON_2026_STORYBOARD.md` §6 still lists them as PROPOSED and `docs/status/IMPLEMENTATION_STATUS.md` says "not yet deployed" — both are out of date relative to the production version string. Manual accessibility, browser E2E and a live AT-7 suggestion run are still listed as not done for D-081.

### Missing or substituted assets

| Asset | Why | Substitute in the film |
| --- | --- | --- |
| Professional Workspace (`/operator`, partner view) | Needs a staff sign-in (password + TOTP); the offline capture does not fake operator auth | Person-side care plan with the "Simulasi" badge + an illustrated connection diagram labelled SIMULATED PROFESSIONAL |
| Live Meta reply provenance | No captured runtime evidence in this build | Test-double reply label; the Meta models are named only in the end card ("Built with Meta AI"), which matches README/PRODUCT configuration |
| Narration | Not recorded | Script + timed ID/EN subtitles (`docs/VO_SCRIPT_ID.md`, `subtitles/`) |
| Reflection-card correction propagating | **Not a product behaviour**: correcting a talk reflection card stores the user's wording but does not walk the graph. Only remembered meanings (memory claims) propagate. | Chapter 03 demonstrates propagation on "Yang Sela ingat" (memory), not on the chat card |

## Claim checklist (every sentence on screen)

| Claim on screen | Backing |
| --- | --- |
| "kadang → selalu · frequency · critical", "added word · review" | `CompareMeaning` output for the exact strings |
| "This will mark 1 summary for review, stop 2 share links, and cancel 1 request…" | SELA UI copy (`impactText`, `web/src/v3/meaning.tsx:241`) for the impact counts asserted in `TestMeaningCorrectionPropagates` |
| "one transaction · one invalidation event" | `invalidate` runs inside the caller's `s.tx`, writes one `mie_events` row (`internal/account/mie.go:246`) |
| "Your correction was carried through" | UI copy `Continuity.tsx:1246` |
| "Needs review again" / "Stopped because the meaning was corrected" | `meaning.tsx:237`, `Shares.tsx:225` |
| "This page doesn't use AI, doesn't score you…" | `ContextTimeline.tsx:290`; `TestV3ContextTimelineOwnerScopeNoAI` |
| "No reminders or day counts. This step is not treatment." | `ActionSteps.tsx:282` |
| "Not a medical passport" | `Passport.tsx` notice; `TestV3CarePassport` |
| "The recipient preview is exactly what is shared — same server projection" | `PassportPreview` → `projectSummary` (`internal/account/passport.go:79`, `shares.go:227`) |
| "Only an approved, current version can be shared" | `Shareable` only when approved; stale versions refused (`TestMeaningCorrectionPropagates`, `TestV3ShareNeedsConsentAndApproval`) |
| "The service can no longer read your summary" | UI copy in `CarePlan.tsx`; `CancelCareRequest` revokes the hidden share (`carerequests.go:352`) |
| "Copies already viewed cannot be recalled" | UI copy `Shares.tsx:515`, `Passport.tsx:840`, `meaning.tsx:251` |
| "Built with Meta AI" | README "Meta models in use" (Muse Spark generator, Muse Wonder verifier; production `ai: meta_api`). Configuration claim, not a claim about the film's footage. |

Not used anywhere in the film: benchmark numbers, clinical outcomes, partnership claims, "appointment booked", health scores, the opt-in condition map.

## Privacy

No passwords, tokens, API keys, real users, real records, share links, access codes or partner credentials appear. The fixture user is a guest with `synthetic: true`; names "Nadia", "Klinik Cerita Demo" and "Konselor Demo B" are fictional (from the repo's fixture spec). No production data was read or written; the deployed app was not modified.

## Technical QA

| Check | Result |
| --- | --- |
| Master | 1920×1080, 16:9, 30 fps, 5,400 frames = 180.0 s, H.264 High (yuv420p) + AAC 256 kb/s |
| Teaser | 1920×1080, 30 fps, 900 frames = 30.0 s |
| Loudness | −15.9 LUFS integrated, −1.7 dBTP (two-pass loudnorm) |
| Safe area | Headlines centred; labels and captions inside 140 px / 84 px margins |
| Readability | Smallest on-screen text 13.5 px mono metadata (graph footers); all story text ≥ 19 px; English glosses under every Indonesian UI line that carries a claim |
| Flashing | No full-frame flashes; light sweeps peak at ~22 % alpha; no strobe |
| Type check | Not completed here: `@types/react` could not be installed offline, so `tsc` stops on missing React types. Run `npm i && npm run typecheck` once. The bundle builds and every frame renders. |
| Frame review | Stills reviewed at the busiest moments of every chapter before the full render; rendered chapter files spot-checked mid-transition |
