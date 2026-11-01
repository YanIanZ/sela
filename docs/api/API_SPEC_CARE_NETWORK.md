# Care network API (D-076)

Status: **implemented and tested locally** (2026-11-01; migration `1100_care_network`, `internal/carenet`, `internal/account/carerequests.go`, `internal/httpapi/v3_care_requests.go`, `internal/httpapi/admin_network.go`; `TestCareNetwork*` with PostgreSQL). Simulated partners only in staging and production until the owner acknowledges real partners (`SELA_CARE_NETWORK_REAL_ACK=B-10-owner`); B-10 stays open. Authority: [SPEC.md](../../SPEC.md) T29, [D-076](../decisions/DECISION_LOG.md#d-076).

## 1. Model

- **Care organisation** (`care_orgs`): a clinic, hospital, puskesmas, practice or organisation, with modes (`online`, `in_person`), the condition-map stages it fits, `synthetic` and a status `pending → verified ↔ suspended`. A real organisation is never verified by the operator who created it; a synthetic one may be.
- **Partner accounts**: operator accounts (`admin_users`) with role `partner_admin`, `practitioner` or `partner_staff`, always bound to one organisation (`care_org_id`). Practitioners carry a display name, a professional type (`psychologist`, `psychiatrist`, `doctor`, `counsellor`, `nurse`, `social_worker`) and optionally a licence number (STR/SIP) that a different operator verifies against the official register. Accounts are provisioned out of band (`sela admin create-partner`), with password and TOTP like every operator.
- **Care request** (`care_requests`): one approved consultation summary sent to one organisation, optionally to one of its practitioners. It is a share (`shares`, purpose `consultation`, seven days) whose link token and access code are discarded, so the organisation reads the frozen projection only through the request and only while the share is active. States: `sent → proposed → confirmed → completed`, or `declined`, `cancelled`, `expired`. Revoking the share or letting it expire ends an open request.
- No free-text chat, no booking engine, and Sela never passes on the person's contact details: the organisation proposes up to three times, a mode and its own contact instructions, and the person confirms one time.

## 2. Person (`/api/v3`, signed-in owner session)

| Method and path | Body | Result |
| --- | --- | --- |
| `GET /care-network` | — | `{orgs:[{id,kind,name,city,description,modes,stages,synthetic,practitioners:[{id,displayName,professionalType,licenseVerified}]}], realPartners}`; verified organisations that accept requests (real ones only when allowed) |
| `GET /care-requests` | — | `{requests:[CareRequest]}`, newest first |
| `POST /care-requests` | `{summaryId, orgId, practitionerId?, mode: online\|in_person\|either}` | `201 {request}`; needs the `sharing` consent (`403 consent_required`), an approved current summary (`409 invalid_transition`) and an available organisation (`409 care_org_unavailable`) |
| `GET /care-requests/{id}` | — | `{request}` |
| `POST /care-requests/{id}/confirm` | `{slot, expectedVersion}` | `{request}`; `slot` must be one of the proposed future times |
| `POST /care-requests/{id}/cancel` | `{expectedVersion}` | `{request}`; also revokes the share |

`CareRequest`: `id, reference (SR-XXXXXX), orgId, orgName, orgSynthetic, practitionerId?, practitionerName?, shareId, summaryId, summaryVersion, mode, state, proposal? {slots[], mode, contact, note?}, confirmedSlot?, declineReason?, version, expiresAt, createdAt, updatedAt, respondedAt?`.

## 3. Operators (`/api/admin/v1`, operator session)

| Method and path | Permission | Notes |
| --- | --- | --- |
| `GET /network/orgs` | `network/manage` or `network/verify` | every organisation with member counts and `realPartners` |
| `POST /network/orgs` | `network/manage` | `{kind,name,city,description,modes[],stages[],synthetic}`; a real organisation needs the deployment acknowledgement (`403`) |
| `POST /network/orgs/{id}/status` | `network/verify` | `{status: verified\|suspended}`; the creator never verifies a real organisation |
| `GET /network/orgs/{id}/members` | `network/manage` or `network/verify` | role, display name, professional type, licence number and licence status |
| `POST /network/members/{username}/verify-license` | `network/verify` | practitioners with a licence number; never one's own; once |

Owner holds both network permissions; `directory_reviewer` holds `network/verify`.

## 4. Professional workspace (`/api/admin/v1/partner`, partner session)

Every route requires a partner role and filters by the caller's organisation; internal operators are refused (`403`), and another organisation's request is `404`.

| Method and path | Permission | Notes |
| --- | --- | --- |
| `GET /partner/org` | `care-requests/read` | the caller's organisation |
| `GET /partner/requests` | `care-requests/read` | no content |
| `GET /partner/requests/{id}` | `care-requests/read` | `summary` only with `care-requests/content` (partner administrator, practitioner) and while the share is active; each opening is a share event the person sees and an `admin_audit` `care_request.open` |
| `POST /partner/requests/{id}/respond` | `care-requests/respond` | `{decision: propose, proposal:{slots[1..3] (future, ≤ 60 days), mode: online\|in_person, contact (3–300), note? (≤ 500)}, expectedVersion}` or `{decision: decline, declineReason: no_capacity\|not_a_fit\|refer_elsewhere\|other, expectedVersion}`; a new proposal may replace an earlier one until the person confirms |
| `POST /partner/requests/{id}/complete` | `care-requests/respond` | confirmed → completed |

`partner_staff` holds only `care-requests/read`: status, times and reference, never the summary.

## 5. Audit and privacy

`admin_audit` records `network.org_create`, `network.org_verify`, `network.org_suspend`, `network.license_verify` (in the same transaction) and `care_request.open`, `care_request.propose`, `care_request.decline`, `care_request.complete`. The partner view carries no user, account or session id; the summary keeps the owner's private-name masking. Deleting the summary or the account deletes the share and the request (cascade).
