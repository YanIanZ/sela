# Dengar.in continuity increment — local validation

Date: 2026-10-28. Branch: `feat/sela-new-identity`, uncommitted working tree based on `d638255`.
Owner request: add inherited Dengar.in features and improve the current Sela design.

## Delivered behavior

- Explicit manual memory at `/notes/memory`, off by default with a separate `personal_memory` consent. Create, correct, delete, withdraw, and open an owner-scoped detail (`GET /api/v3/memory/{id}`). No automatic extraction. With `personal_memory` and `ai_processing` current, at most five confirmed/corrected items (600 runes) may be sent as AT-1 `memory` spans; otherwise none is sent. Withdrawal hides memory from normal list/detail/history; owner export and deletion remain available.
- Weekly reflection at `/notes/weekly`: local Monday–Sunday windows from original note creation dates and check-in local days. Edited old entries do not become this week's activity. Reflections save as ordinary encrypted notes only after permission. No invented mission completion, streak, score, percentage or clinical conclusion.
- Manual consultation summaries at `/support/summaries`: choose up to twelve current owned source records, copy only original user turns from conversations, edit the wording, save a draft, and approve its exact version. Source edits/deletion and summary edits invalidate approval. Download rechecks the server's effective approval before producing a plain-text file.
- Preparation goals, questions and things to bring are sealed with the summary. Follow-up notes are persistent, versioned and editable; deleting the summary removes its attached follow-ups.
- Support page, Home continuity shortcuts, source search/pagination, mobile reading layout and a concise memory permission flow use the incumbent cobalt/white identity and self-hosted Manrope.
- Reading catalogue links to official WHO/NHS publisher pages, with no referrer or personal content passed in the links. Internal clinical exercise cards retain their human editorial gate. No source text or unreviewed exercise instructions are republished.

## Implementation and migration

`0300_continuity_kinds` extends the existing encrypted personal-record kinds. Content, source links, preparation and follow-up parent ids remain inside the per-owner encrypted envelope. Existing account export and account deletion cover these records. Existing v2 demo and its cookie surface remain separate.

The actual delegated worker ran through OmniMemo -> OpenCode `deepseek/deepseek-flash`, using the configured DeepSeek API. Task `task_59432ba523eb9974`; scope was backend continuity. Parent integrated the frontend, route compatibility, documentation and regressions. No Git publication was performed.

## Verification

- Frontend typecheck, twelve test files / 64 tests, and production build passed. Four new tests cover weekly boundaries, edited timestamps, check-in days and derived-record exclusion.
- Full `go test -race -count=1 ./internal/... ./cmd/...` and `go vet ./internal/... ./cmd/...` passed against real PostgreSQL16 with required database tests. The disposable test image uses matching PostgreSQL16 dump/restore clients.
- Contrast checks passed for light, dark, contrast-light and contrast-dark.
- The expanded `e2e/v3-frontend.e2e.mjs` journey passed **100/100 checks and 39 axe scans**, with no serious/critical violations, horizontal overflow or browser errors. This includes actual downloads, edit/source invalidation, deletion cascade, both legacy aliases and truthful sharing limits. Results: `e2e/results/v3-frontend/results.json`; 21 captures: `.impeccable/review/continuity/`. Docker QA supplies the installed Chromium executable; the journey itself is now retained in the repository.
- Impeccable detector on changed continuity surfaces: zero findings (`.impeccable/review/continuity/detector.json`). Independent visual review opened the eighteen baseline captures and found no material visual defects; its subsequent `ship` verdict scores all three listed fixes resolved (seed proof, legacy route truth, persisted design/product records). This is the scope of the verdict, not an exhaustive backend audit. Reports: `.impeccable/review/continuity/finish-review.md` and `finish-verdict.md`. Fresh documentation checks confirmed DESIGN, its schema-v2 sidecar, surface contract and dated product/matrix checkpoints.

## Regressions found and corrected

- A withdrawal hid the dedicated memory list but generic detail originally bypassed consent. Generic list/detail/history now check current permission; regression assertions are in `TestContinuityMemoryConsentEncryptionAndIsolation` and SPEC BUG-4. Existing V29 covers the invariant.
- Follow-up correction preserves its sealed parent and checks ownership/version. The existing exported-store ownership test caught an unnecessary exported helper; the helper is private.
- Legacy `/support/prepare` and `/support/after` route to real summaries; sharing-only unavailable copy no longer claims summary APIs are missing.
- Initial full-suite environment failures came from absent/mismatched pg_dump/pg_restore tools; matching16 clients passed the restore tests. No restore implementation was changed.

## Scope limits

No recipient share links/access codes/revocation service, automatic AI memory use, AI summary generation or clinical editorial approval is claimed. Users can download an approved manual summary and decide how to carry it themselves. These local tests use synthetic data and do not claim public deployment, pilot readiness or clinical validation.
