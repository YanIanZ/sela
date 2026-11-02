# Life Context Timeline — product and technical specification

**Status: proposed / not implemented in this PR.** Task IDs BW-002–BW-004. Reference [Build for Your World roadmap](../roadmap/BUILD_FOR_YOUR_WORLD_2026.md), [MIE](../vision/MEANING_INTEGRITY_ENGINE.md).

## 1. User story and differentiator

“As someone who finds it hard to explain what has changed, I want to see the original experiences I chose to save in order, and optionally review a tentative reflection about a difference—without the AI telling me what diagnosis, risk or progress score I have.”

This is **not** the existing notes list renamed. The timeline adds source provenance, relevant changes, version-and-correction state, approved user-selected themes and preparation links. It **reuses** the existing notes/check-ins/conversation/weekly structures; it must not create duplicate diaries or become a health history.

## 2. Information architecture

Proposed route: `/notes/context` with a link under `/notes`, not a replacement for the existing timeline. Keep `/notes/patterns` (check-in patterns) separate and unmodified. Text:
- ID: **Perjalanan ceritaku** / “Lihat pengalaman yang kamu simpan, berdasarkan waktunya—tanpa tebakan tentang kondisimu.”
- EN: **My story over time** / “See the experiences you chose to save in chronological order—without guesses about your condition.”
- Empty state: “Belum ada cerita tersimpan. Kamu bisa menulis atau memilih untuk menyimpan percakapan.” Link to `/notes/write`.
- One primary CTA: **Tinjau yang berubah / Review changes** (optional Meta processing, requires explicit consent and user confirmation).
- Secondary CTA: **Pilih untuk ringkasan / Select for summary** → `/support/summaries/new` with owner-approved ID/version selections, not copied private text in URL.

### Cards / data representation
A timeline card contains: local display date/time (source timestamp from server, stable UTC stored), user-controlled type (note/check-in/saved conversation/weekly review), title or neutral label, original excerpt behind privacy preview, provenance (“Tulisanku” versus “Usulan Sela”), source version, status (current/stale/review pending), and optional **what changed** clue only if evidence is source anchored and reviewable.

No unrequested “you are deteriorating” / “your anxiety rose 40%” / “risk stage” from this timeline. Existing Condition Map stays an **explicit, optional separate consent** and is not repackaged as timeline interpretation.

### Interaction states
1. Default read-only chronological list without AI.
2. Filter by record type and an owner-selected bounded window (7/30/90 days); date range itself does not imply longitudinal diagnosis.
3. Expand exact original source; query detail through owner API, no sensitive text in URL/analytics.
4. Review a proposed explanation of difference; show both source quotes and MIE meaning check. Reject/correct/accept or ignore.
5. Correct original source: existing MIE invalidation causes affected interpretations to be stale; refresh list and show **“Perlu ditinjau ulang”**.
6. Export/share path only via approved Consultation Summary / Care Passport, *never* share entire timeline automatically.
7. Delete source via existing deletion API; update timeline and show real dependency impact before choice.

### Accessibility
Keyboard list navigation, semantic headings and buttons, progressive loading (no infinite focus jump), text/no-color-only stale chip, logical article chronology, adjustable contrast, reduced motion, 320px reflow, ID/EN, screen-reader announcements for revised items. No forced chart: a textual timeline is the primary rendering.

## 3. Data contract

**Read projection** should be computed from existing owner-scoped records, not a new model-generated profile. Proposed endpoint (to validate against API_SPEC_V3 and router conventions): `GET /api/v3/context-timeline?from=<RFC3339>&to=<RFC3339>&limit=<1..50>&cursor=<opaque>`.

Example *proposed* response:

```json
{
  "items": [
    {
      "source": {"recordId": "opaque-uuid", "version": 2, "kind": "note"},
      "occurredAt": "2026-10-27T09:00:00Z",
      "label": "Catatan",
      "excerpt": "Sepulang kuliah aku merasa lelah ...",
      "provenance": "user_original",
      "meaningStatus": "current",
      "selectedForReview": false
    }
  ],
  "nextCursor": null,
  "count": 1,
  "aiUsed": false
}
```

Final schema must reuse existing `Entry` and MIE status vocabulary. A derivative item adds `sourceRefs` with exact versions and `reviewStatus`, not a new opaque inference. Pagination cursor must not embed user text or a guessable owner ID. Only allowed fields are returned—no date-of-birth, real identity, IP, or hidden session metadata.

### Derived optional “change” proposal
- Use existing AT-3/Meaning Diff and confirmed-memory conventions where possible. A proposed comparison may cite a pair of *approved owned source versions* and a bounded excerpt but **must not** be silently stored.
- Source correlation is a **narrative observation**, not causation. Wording: “Dari dua catatan yang kamu pilih, kamu menyebut ... pada tanggal ...”; do not claim cause.
- No off-by-default background inference; no timer, no push notifications, no periodic surveillance.
- If source deleted/changed, proposal becomes stale, cannot feed memory, passport or partner request without a new user review.

## 4. Architecture & queries

- Reuse `internal/account.Records`, `Record` fields, MIE `mie_edges` and owner-visible stale APIs, rather than calculating private data client-side from unbounded record downloads.
- Server enforces session + owner/guest principal; no public cache/Redis, no shared operator access and `Cache-Control: no-store`.
- Limit number of events/period before decryption; avoid leaking data through status counts. Database indexed query by `owner_id` + `created_at`; verify existing schema before adding indices.
- Optional Meta summary requires consent check before call and a second check before release/save; quote validation deterministic.
- Source kind eligibility: explicit saved note, check-in, saved conversation, weekly review note. Never include unsaved conversation holds, community contributions, operator moderation records, outside literature, and deleted or guardian-restricted data.
- Respect private-name masking and editing across records. Exact quote display to owner may be allowed, but provider receives minimized excerpts only.

## 5. Acceptance criteria (BW-002/BW-003)

- LT-01: timeline is useful with AI disabled and with zero stored data.
- LT-02: owner A cannot infer or read owner B's event IDs/content; unknown and foreign IDs indistinguishable.
- LT-03: no event appears if session/retention/consent makes it unreadable.
- LT-04: stable pagination and chronological ordering across date boundaries and concurrent inserts.
- LT-05: original quote and version exactly match the server source; all timezone conversions explicit.
- LT-06: stale derivative is visibly marked and excluded from new approved shares/context.
- LT-07: revising/deleting source updates timeline on the next owner read; no unsafe stale client cache.
- LT-08: 320px keyboard, 200% zoom, ID/EN, assistive technology manual tasks complete.
- LT-09: no AI provider call when timeline is merely viewed; independent consent for review proposal.
- LT-10: no unsupported diagnosis, score or claimed causality shown as a fact.
- LT-11: “Select for summary” sends **identifiers with versions** through an owner-scoped navigation state or server draft, never private excerpt in query.
- LT-12: use only approved MIE-derived items for any downstream “change story” export.

## 6. Edge cases

Clock skew, local date rollover at midnight, multiple records same timestamp, DST for non-Indonesia users, record edited during a selected-item review, one source referenced by many summaries, source deleted with history kept, no consent, private name added after old model output, and empty history. Reuse existing MIE atomic correction tests and add LT tests, not a new stale authority.

## 7. Planned file map (not yet created)

- `internal/httpapi/v3_context_timeline.go` handler and tests.
- `internal/account/context_timeline.go` projection from owned saved records and MIE status.
- `web/src/v3/screens/ContextTimeline.tsx` + localized CSS; route `/notes/context` and entry point under Notes.
- `web/src/test/contextTimeline.test.tsx` unit + Playwright/Node E2E task.
- Changes to relevant API spec and policy copy after implementation review.

**No UI/API implementation exists from this document alone.** Existing standard notes timeline remains functional; do not label this projected view as shipped until its tests pass.
