# Local v3 core increment — 27 October 2026

> Historical checkpoint. The frontend described here was replaced by the full v3
> shell and screens on 2026-10-28; current evidence is in
> [V3_FRONTEND_VALIDATION.md](V3_FRONTEND_VALIDATION.md). The backend behaviour
> and local commands below remain the basis for the listening/records tests.

Source: branch `feat/v3-core-journey`, based on `e62506a`. Status: in progress with recorded local evidence, not deployed and not ready for a real-user pilot. All inference/browser examples are synthetic. The owner supplied API access for local tests; no key or session token is included here.

## Delivered behaviour

The default browser entry uses `/api/v3` for guest/accounts, registration with ten one-time recovery codes, sign-in/recovery, home, listening, encrypted notes and revisions, check-ins, timeline, support and privacy/export/deletion. `/demo` preserves the v2 demo and `/r` preserves its fictional recipient entry. Guest upgrade is exposed in Privacy & data. Both Indonesian and English are available. The v3 app uses the existing `Ruang & Catatan` design tokens; the synthetic-demo banner remains only on the separate v2 journey.

Record ownership comes only from the v3 session cookie. Text is encrypted per user; create/update require `storageGranted:true`. Each check-in is unique by owner and browser-local date. Updating keeps previous encrypted versions; export includes both current records and all revisions, and deletion cascades to revisions. Conversation snapshots use the same owned/encrypted record store. Unsaved conversations are bounded to 64 turns, 5 live rooms per session, 500 total, 30 inactive minutes and 2 hours maximum; timers purge inactive holds even without further requests. End/sign-out/session revocation/account deletion cancel or purge affected holds.

Listening uses `internal/ai`, with minimisation, crisis screening, output guards, per-provider consent, a 90-second operation budget, per-owner allowance (8 AI operations per 10 minutes), provider admission and stale-session/consent/version checks. A crisis keeps the user's text and pauses AI until explicit continuation; a saved pause survives reopen. Deleting its saved record ends the corresponding room. AI failures retain the user's words and show an unavailable/manual state. Meta replies are labelled with model/provenance; verifier/fallback eligibility stays governed by the existing router (DeepSeek is off and synthetic-only by default). Saved replies carry the router's content-free provenance record.

## Local configuration and repeatable commands

Existing ignored files: `.env.pg-testing` (PostgreSQL identity/password), `.env.docker-testing` (development configuration, DB URL, per-user wrapping key and provider keys), `.env.unit-testing` (separate synthetic test DB). Permissions are `0600`. Never paste, commit or print their contents. Preserve the KEK together with the local database; generating a different key does not decrypt existing data.

- Start/rebuild: `bash scripts/run-local-v3.sh`. Uses Docker image `sela:v3-core`, network `sela-testing`, PostgreSQL 16 container `sela-postgres-testing`, persistent volume `sela-postgres-testing-data`, app `sela-v3-core-testing` on loopback port 8080. Database env URL points to that container. Development-only auto-migration applies schema 0200. No public hosting action.
- Frontend: `cd web && npm run typecheck && npm test && npm run build` (Node 22). The tests need the complete repository's `internal/` and `data/` fixtures when run in Docker.
- Backend: `go vet ./... && SELA_REQUIRE_DB_TESTS=1 go test -race -count=1 ./...` (Go 1.26.10, PostgreSQL 16, client tools 16). For the local Docker database, the test container shares PostgreSQL's network namespace and uses a loopback test URL; no production TLS rule is bypassed. Test schemas/DB are separate from the application.
- V3 browser regression: `E2E_LIVE_AI=1 E2E_BASE_URL=http://127.0.0.1:8080 node e2e/v3-core.e2e.mjs`. Requires Playwright/Chromium and axe-core; optional `CHROMIUM_PATH`. Creates synthetic guest records, authorises one Meta call, exercises export and deletes its guest. Real inference requires the explicit flag.
- V2 regression: build `build/sela`, `build/fakellm`, and `web/dist`, then `node e2e/journey.e2e.mjs`. Uses test doubles, not paid providers; now enters `/demo` explicitly. Optional `CHROMIUM_PATH` for system Chromium.

Browser evidence lives in ignored `e2e/results/v3-core/` (desktop 1365×900 and mobile 320×740, ID/EN). It contains synthetic examples only. Automated axe checks cover serious/critical WCAG 2.0/2.1/2.2 violations and horizontal overflow. Visual screenshots were inspected. This is not a full manual WCAG audit. The design detector's side-border warnings refer to the documented original-word/notice rules in `docs/design/DESIGN_DIRECTION.md`, retained for meaning and provenance.

## Recorded local results

27 October 2026, final workspace/image `sela:v3-core` (`e62506a-dirty`):

| Check | Observed result |
| --- | --- |
| Go vet and full PostgreSQL-required race suite | Exit 0, every package passed; isolated test DB/schema, PostgreSQL 16 |
| Frontend typecheck, Vitest and production build | Exit 0; 51 tests in 11 files passed |
| V3 browser journey against the final Docker image | Guest → note save/reload → check-in/reload → live Meta listening → export → deletion; passed. 10 page/viewport/language axe checks, no serious/critical finding and no horizontal overflow |
| Account browser flow | Registration, ten one-time recovery codes, acknowledgement, logout, recovery-code password reset and account deletion passed on synthetic account |
| Existing v2 browser suite | 14/14 passed, including ID/EN sharing/revocation, crisis, cancellation, other-tab replacement, keyboard, mobile and content-free logs |
| Saved crisis pause regression | New test failed before the fix; passed after persistence/reopen and associated-room deletion fixes |
| Docker launcher | `bash scripts/run-local-v3.sh` completed; health/readiness 200, PostgreSQL ok, schema current |
| Whitespace/shell checks | `git diff --check` and `bash -n scripts/run-local-v3.sh` exit 0 |

The compatibility endpoints `/api/v2/health` and `/api/v2/ready` continue to describe the retained **v2** journey (`synthetic_demo`, `store:memory`, `persistent:false`). They also report database/schema health, but their old persistence field is not the persistence of the v3 personal records. The default frontend uses `/api/v3/records` and `/api/v3/conversations`; those saved records use PostgreSQL.

## Remaining work

T15, T16 and T18 stay in progress. Memory and reflection review; full consultation summary/sharing/recipient/follow-up v3; reading-library UI with authoring verification; streaming listening events; session/audit management screens, full minor/guardian/display/restore-notice flows; fallback/verifier evaluation; hosted operations and legal/pilot gates remain open. Existing API provider smoke tests do not prove a completed verifier evaluation. The configured remote model revision remains unverified. No effectiveness, clinical quality or public-release claim.
