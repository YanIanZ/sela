---
version: 1
slug: "web-src-v3-app-tsx"
primary_target: "web/src/v3/App.tsx"
related_targets: ["web/src/v3/screens/Continuity.tsx", "web/src/v3/screens/Weekly.tsx", "web/src/v3/screens/Support.tsx", "web/src/v3/screens/Reading.tsx"]
---

# Sela replacement interface

Mode: Operate. The public information portal has its own Read-mode surface brief.
Scope: web/src/v3, default /api/v3 journey; preserve owner authorization, consent, records, crisis help, ID/EN, display preferences and every supported API. New request supersedes the old warm editorial identity. Unsupported APIs remain honest blocked states.

## Direction contract

THESIS: Tidak harus rapi. Feelings need not be tidy before someone can talk; the interface makes one next action clear without assessing or scoring them.

OWN-WORLD: White and cool blue-grey surfaces, cobalt action/identity, coral and lilac in the original folded-paper S illustration. Self-hosted humanist sans, open S mark, crisp standard controls, subtle divisions and restrained 12–16px panels. No serif-paper editorial costume.

STORY: Arrive with an unfinished thought; choose talk, a private note, or a check-in; decide storage and AI independently; return to real saved records. Help and Quick exit stay reachable.

FIRST VIEWPORT: Public entry is the owner-requested information-dense portal before login; it has no personal data and is separate from Beranda. See web-src-v3-screens-dashboard-tsx.md. Authenticated non-focused pages use a fixed 15rem workspace rail at ≥64em, with primary places, daily tools and settings; the header carries context, Help and Quick exit. Home has a compact white title/greeting followed by the accepted cobalt invitation (talk/note/check-in and storage/AI hint), a right-hand weekly/summary continuation and feature index, then real recent records. The 1.6fr/1fr split becomes one column below54em. Mobile preserves its header and bottom places; 320px/text scaling uses2×2 navigation. Settings, journal, reading and support share the workspace frame; focused rooms retain their original protected writing area. No invented scores, missions or activity. The separate DEV dashboard preview needs no login, labels synthetic data, disables product actions and makes no account/API/AI requests.

FORM: Crouwel grid specimen translated under Operate limits (type, palette, density, one signature); selected by user on direction round 4 (reroll 3, bolder). seed:e10a74b613d8d165. Chosen sketch .impeccable/mocks/decision/belum-rapi.html; capture at captures/belum-rapi.png. Code-led, no approved comp owed. User subsequently delegated creative details freely and requested OM/OpenCode DeepSeek Flash 1017 sub-agent from DeepSeek API.

CONTINUITY CHECKPOINT (2026-10-28): User-authorized incumbent-world extension, local and uncommitted, not live. Memory is explicit manual CRUD with separate personal_memory consent, encrypted owner storage/revisions/export/delete; withdrawal hides normal reads and does not erase owner export/delete data. Manual summaries pin 1–12 current owned source versions, copy user turns only, allow wording and optional goals/questions/bring, invalidate approval on edits/source changes and recheck the server before an approved .txt download. Follow-ups are real records with edit/delete and parent-deletion cascade. Weekly uses local Monday–Sunday, note creation/check-in day and an ordinary-note reflection. Legacy /support/prepare and /support/after open the real summary list; /support/shares and recipient/token access stay unavailable. Four official WHO/NHS reading links use no referrer; Sela exercise-card editorial and clinical gates stay intact. No AI summary generation or memory injection is claimed.

EVIDENCE: source web/src/v3/{styles,tokens,controls,continuity}.css and screens/{Home,Intro,Continuity,Weekly,Support,Reading}.tsx, App.tsx; backend internal/account/continuity.go and records.go, API_SPEC_V3 manual-continuity section, migration 0300. Original choice and seed remain unchanged; exact CLI seed proof is at .impeccable/review/continuity/seed-check.txt from repo root. Local captures: 21 in .impeccable/review/continuity/ — 18 baseline full-page visual views across desktop, 390px and 320px plus three alias/sharing views. Validation at checkpoint: PostgreSQL 16 Go race suite and vet exited 0; 64 frontend tests, typecheck and production build passed; final Docker browser run exited 0 with 100/100 checks, 39 axe scans and zero browser errors. The independent finish verdict is ship for the three scored fixes, not exhaustive backend approval. Final evidence: [continuity validation](../../../docs/V3_CONTINUITY_VALIDATION.md) and .impeccable/review/continuity/finish-verdict.md from repo root. None of this establishes a live deployment, user research or clinical approval.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance


DASHBOARD CHECKPOINT (2026-10-28): Browser-minted live session59192355 accepted Today variant1, comfortable density1 and split composition. Carbonized into dashboard.css, completed and helper stopped; Vite remains5173. Preview entry dashboard-preview.html is outside the default production build. Sources: shell.tsx, workspace.css, dashboard.css, Home.tsx and design-preview.tsx. Reviewer fixes cover preview banner rail offset, stale summary copy and responsive system-theme updates. Evidence .impeccable/review/dashboard/; scope is local implementation, not publication or clinical approval.


PUBLIC PORTAL CORRECTION (2026-10-28): Owner explicitly confirmed dashboard is a public information portal BEFORE login. `/` and `/dashboard` render DashboardPage, while personal `/home` retains the prior accepted composition after authentication. Main CTA follows normal age/guest-or-account/consent flow. Preview now begins with null session profile and can switch to synthetic Home without making API calls. The earlier public illustration-hero instruction is superseded by this confirmed information-only portal scope.
