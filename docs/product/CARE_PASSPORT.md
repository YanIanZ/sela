# Care Passport — owner-approved consultation preparation

**Status: proposed, not implemented in this PR.** Task IDs BW-005/006/009. Related [Care Network API](../api/API_SPEC_CARE_NETWORK.md), [MIE](../vision/MEANING_INTEGRITY_ENGINE.md) and [Build for Your World plan](../roadmap/BUILD_FOR_YOUR_WORLD_2026.md).

## 1. Goal and terminology

**Care Passport** is the product nickname for a **personal, user-approved consultation preparation sheet**. It is **not** an official health passport, interoperable EHR, patient chart, medically verified record, proof of clinical condition, diagnosis or provider-issued document. UI should lead with “Ringkasan persiapan konsultasi / Consultation preparation sheet”; “Care Passport” may appear as secondary branding with this explanation.

Problem: before a professional conversation, a person may struggle to choose the relevant parts of weeks of notes. The useful outcome is **a short document in the person's own words with traceable source quotes and questions**, which can be kept private or optionally shared through existing revocable mechanisms.

## 2. Core five-step flow (no extra AI required)

1. **Select**: from owner-eligible saved notes, check-ins, saved conversation excerpts, confirmed memory, and approved timeline comparison if a versioned claim exists. A source is eligible only if owner can still access it; MIE stale items are excluded.
2. **Compose**: start from the existing `consultation_summary` builder and `prepared{goal,questions,bring}`. Limit 1–12 pinned source records and existing max-36 quote-backed `items[]`, unless reviewed schema changes explicitly raise caps.
3. **Verify**: show a source quote, author/provenance label, version, and Meaning Diff for each AI-suggested claim; show a conspicuous “This is your preparation, not a diagnosis” boundary. User edits any unsupported text.
4. **Approve**: bind approval to **exact summary version**, source-ref versions, source-quote validation, active sharing consent when actually sharing, and current MIE graph revision. Draft can be saved but not sent as approved.
5. **Use or revoke**: download with current HTML/Markdown export, create a share requiring link + code, or create a care request to a **clearly synthetic** partner. Users may keep it completely private. Revising a contributing source invalidates affected summary, ends future share reads and cancels dependent open care requests (D-078).

### What Care Passport does not do
No automatic recipient selection, background messaging, direct outreach or clinical risk score; no obligation to seek care; no claim of automatic professional authentication; no real booking guaranteed; no special access for operators to owner journals. If the person chooses to call the actual support number, that's their action outside Sela.

## 3. Screen and export UX (additive over current Summary UI)

Proposed owner path: `/support/passport` as task entry to **existing** summaries and `/support/passport/:summaryId` as optional preview route. Avoid creating an unrelated duplicate persistence model; the summary remains source of truth.

Screen sections:
- **My purpose** (optional): “Aku ingin bisa menjelaskan ...” editable, length-bounded, no automatic clinical objective.
- **What I want to share**: explicit select/unselect cards with source versions; unselected notes never included in snapshot.
- **In my words**: user-edited statement cards with exact source quote and provenance (user original/correction, pending AI suggestion blocked from approval until review).
- **What I want to ask**: 1–5 questions chosen or written by user; a draft may use current `prepared.questions`, without invented clinical recommendations.
- **What helps me explain**: optional user-supplied preferences about how to communicate; cannot infer disability/diagnosis.
- **Privacy preview**: precise fields that recipient will receive, masking configured private names and withholding owner ID, direct account/profile identifiers or unselected notes.
- **Approval and status**: “Draft / Approved version / Needs review / Revoked or expired”; never color-only.
- **Recheck source**: one tap from item to owner source and lineage; outdated view forces refresh.

### Example copy
- ID title: **Ringkasan untuk dibawa**; subtitle: “Pilih bagian ceritamu yang ingin kamu jelaskan. Kamu bebas mengubah atau tidak membagikannya.”
- EN title: **My consultation preparation**; subtitle: “Choose the parts of your story you want to explain. You can edit or keep this private.”
- Warning: “Sela tidak memeriksa atau menetapkan diagnosis. Penerima bisa menyimpan salinan yang sudah dilihat; pencabutan memblokir akses berikutnya, bukan salinan yang sudah diunduh.”
- Synthetic badge: “SIMULASI — penerima fiktif; tidak ada konsultasi sungguhan terjadi.”

## 4. Data and API approach

**Prefer reusing existing APIs**, not introducing a mandatory new table:
- Existing `POST /api/v3/consultation-summaries` with source refs/prepared/items.
- Existing `PATCH /api/v3/consultation-summaries/{id}` for versioned owner edits.
- Existing approve/export APIs and `POST /api/v3/shares` or `POST /api/v3/care-requests`.
- Existing `GET /api/v3/meaning/impact` and lineage/endorsement if available.
- For display only, a proposed `GET /api/v3/passports/{summaryId}/preview` can aggregate approved summary, source validity, sharing-safe projection and graph revision; it must never bypass the current share projection functions. If current summary detail already returns all needed data, do not add endpoint.
- Prefer a **rendering preset** of current summary (HTML/Markdown) to a separate PDF pipeline; include metadata explaining personal, nonclinical nature. Do not export private source quotes beyond selected/approved fields.

Example *proposed view-model*, not an existing API:

```json
{
  "passport": {
    "summaryId": "opaque-uuid",
    "summaryVersion": 4,
    "status": "approved",
    "title": "What I want to explain",
    "prepared": {
      "goal": "Explain what I have noticed",
      "questions": "What might be useful to discuss first?",
      "bring": "My own notes, if I choose"
    },
    "items": [
      {
        "statement": "I sometimes need quiet time after class.",
        "sourceRef": {"recordId": "opaque-uuid", "version": 2},
        "sourceQuote": "Kadang aku ingin sendiri setelah kuliah.",
        "provenance": "user_correction",
        "approved": true
      }
    ],
    "sharingSafe": true,
    "synthetic": true
  }
}
```

Do not attach real names, government IDs, medical diagnoses, proof of clinician qualification or provider endorsements. Use server-side HTML escaping and content security rules for exports. Screen labels must distinguish owner-only source quote from recipient-projected quote when those differ.

## 5. MIE and sharing invariants

- CP-01: User controls exact sources, wording, questions and recipient decision; unselected sources cannot enter export, snapshots or model prompt.
- CP-02: Only **approved current** summary version is shareable and exportable as approved.
- CP-03: Editing/rejecting a contributing claim/source makes dependent approved summary stale; new share/recipient reads fail after commit; outstanding care request cancelled.
- CP-04: Existing shares are immutable frozen projections; correction never silently overwrites what the recipient previously saw.
- CP-05: Only future access can be revoked; information already viewed outside Sela cannot be removed.
- CP-06: Owner can always choose private export or no export; nothing is auto-shared, no mandatory professional pathway.
- CP-07: Care Network continues to reject synthetic organisations from being described as real; partner_staff cannot read summary content; cross-organization isolation enforced.
- CP-08: Source quote cannot be fabricated or paraphrased into an exact quote; quoting must match the pinned version.
- CP-09: Meta-suggested text, where allowed, is *proposed* and must be reviewed; use existing Meta-only verified route and deterministic schema gate.
- CP-10: Every displayed version/consent/recipient status is server-authoritative; no stale client assumption.

## 6. Test matrix / definition of done

| Test | Setup | Expected |
| --- | --- | --- |
| CP-T01 | Draft with 3 sources and 1 unselected source | Preview/export excludes unrelated record |
| CP-T02 | Attempt approval with pending AI assertion | 409/validation until reviewed |
| CP-T03 | Approve v3, edit quote-backed claim v2 | Summary becomes stale; export and partner read refuse |
| CP-T04 | Share to synthetic partner then edit original | Future share reads blocked and care request cancelled |
| CP-T05 | Wrong user requests own/other summary | Other user's id indistinguishable from not found |
| CP-T06 | Consent withdrawn during share creation | Creation denied; no recipient token |
| CP-T07 | Print/export to HTML/Markdown with special characters | No injection; exact snapshot text |
| CP-T08 | Frontend reload mid-approval | Version conflict handled, no false success |
| CP-T09 | Browser on 320px with screen reader | Entire preview and approve/revoke accessible |
| CP-T10 | A recipient already downloaded a copy | Owner UI honestly acknowledges this limitation |
| CP-T11 | Partner not real / absent | Cannot claim booked care or professional acknowledgement |
| CP-T12 | AI model offline/no permission | Full manual Passport creation/approval possible |

## 7. Implementation map (future code PR)

- `web/src/v3/screens/Continuity.tsx`: add “Care Passport” reading/approval preset instead of duplicating source composition.
- `web/src/v3/screens/CarePlan.tsx`: select approved current passport/summary and clearly label synthetic partner.
- `web/src/v3/screens/Support.tsx` and `App.tsx`: route/entry only after implementation.
- `internal/account/mie.go`, `shares.go`, `carerequests.go`: protect exact version in every downstream read and gate.
- `internal/httpapi/v3_summary_export.go` for safe reusable preview projection where useful.
- Add `web` tests + `internal/httpapi` PostgreSQL tests; manually replay synthetic scenarios.

**No new medical record store should be created.** Existing source-bound summary is the durable authority.
