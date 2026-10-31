# Public Sela information portal — local validation

2026-10-28. Owner clarified: the dashboard should contain more information before personal Home; confirmed “Sebelum masuk/login, sebagai portal informasi Sela”. This supersedes the earlier assumption that dashboard and personal Home were one surface.

## Implemented

- `/` for visitors and `/dashboard` render the public information portal. Personal `/home` remains separate; existing signed-in root navigation still resolves to Home.
- Compact heading/entry action, facts strip, six feature descriptions grouped by purpose, three-step start flow, storage/AI/manual-memory and summary controls, honest recipient-sharing limit, reading topics from the existing WHO/NHS catalogue, help and policy links.
- No personal records or conversation loading, account creation, mutation or AI call in DashboardPage. Shared session initialization only reads public policy/options and current-session state. The public portal test checks that no personal-record API or mutation is called.
- “Masuk ke Beranda” starts the existing age/guest-or-account/consent flow for a visitor. It does not bypass onboarding or permission requirements. Authenticated visitors can go directly to Home.
- No-login development preview remains http://localhost:5173/dashboard-preview.html, starts with null session, and supports in-memory Dashboard–Home navigation. Home uses labelled synthetic fixtures; other product actions are blocked. No account/API/AI calls or stored personal content. Default production build excludes the preview HTML and fixtures.
- Public information uses the incumbent Manrope/cobalt/cool-ground identity, a2.3fr/1fr reading grid and dense semantic rows; mobile reflows into one column. Private Home and backend features remain unchanged.

## Fresh verification

- Typecheck,13 test files /66 tests, production build passed. Updated the public-root test and added English `/dashboard` coverage with no-login and no-personal-data/mutation assertions.
- Docker image `sela:info-portal`, version `d638255-info-portal`, replaces only `sela-v3-core-testing` at8080. PostgreSQL16 volume and the other8081 stack preserved. Ready endpoint: database ok/schema current.
- Final retained browser journey:100/100 checks,39 axe scans,0 browser errors, exit0. The start control assertion now matches the information portal, then exercises real guest creation, note/consent/summary/download/deletion paths.
- Separate preview:12/12 checks, including7 axe/overflow conditions (1440,1024,390,320,dark,contrast-dark,150% text), preview-to-Home transition, blocked actions,0 API requests,0 page errors and first-focus skip link.
- Final layout detector:0 findings. Captures and preview-results.json are in `.impeccable/review/portal/`; full browser results remain `e2e/results/v3-frontend/results.json`.
- Approved structure reconciled into DESIGN.md, schema-v2 sidecar and a dedicated Read-mode dashboard surface brief. Identity tokens preserved.
- Independent five-section finish review: disposition ship, no material visual or flow findings in this public-portal scope. Report `.impeccable/review/portal/finish-review.md`; this is not exhaustive backend or clinical approval.

No public deployment, clinical validation, recipient-share service, Git commit or push is claimed. Backend and live AI behavior were not changed by this portal correction.
