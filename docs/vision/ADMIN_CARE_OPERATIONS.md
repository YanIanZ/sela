# Sela | Planned Assistance Operations

**Status:** target workflow specification. A **local planned-care demo** now exists: a sourced public directory, editorial draft/submit/verify/reject/unpublish and an isolated labelled synthetic sandbox (2026-10-30, §10). No real hospital booking, referral exchange, authenticated partner workspace or production directory is deployed.
**Boundary:** admin publishes trustworthy directories; **users choose** professions and facilities without automatic location. Sela does not assign diagnoses or reach out uninvited.

## 1. Two kinds of assistance

| Journey | Primary entry | What admin manages | Safety rule |
| --- | --- | --- | --- |
| Immediate help | User-facing "Butuh bantuan sekarang" | Source-verified official help entries, freshness and corrections | Does **not** depend on Redis, models, admin being online or partner availability |
| Planned professional assistance | Manual profession/facility/name search | Source-verified directory, affiliation, service descriptions, optional future signed handoff | No location needed; no auto-referral, clinical triage, invented appointment or real-time availability |

**"Planned" means non-emergency, user-chosen support—not an automatic scheduled medical appointment.** A request awaiting a provider is not an accepted appointment and must be labelled accordingly.

## 2. Staff workflow — curate and publish providers

~~~mermaid
flowchart TD
    E["Directory editor creates draft"] --> S["Attach official source + credentials/affiliation status"]
    S --> R["Independent reviewer checks source/date"]
    R -->|insufficient| J["Rejected / needs update"]
    J --> E
    R -->|verified| P["Publish versioned public directory entry"]
    P --> F["Scheduled freshness review"]
    F -->|stale / withdrawn| Q["Unpublish or mark stale; invalidate Redis"]
    F -->|current| P
~~~

Minimum fields: organization name and ID, facility name and publicly published city, official source URL, last verified date, services/profession categories and modality only if sourced, practitioner name/credential verification status **only when independently sourced**, affiliation record, contact URL and responsible reviewer. Unsupported fields stay "unknown". No synthetic person/facility is published to the real directory.

**Moderation guards:** review identity != edit identity; reverify-by interval and evidence are required to publish; publish/unpublish events use transactional outbox and versioned public Redis key invalidation. An operator cannot directly manipulate search ranking to conceal sourcing limitations.

## 3. User workflow — no GPS

~~~mermaid
sequenceDiagram
    participant User
    participant Sela as Sela Personal Space
    participant Catalog as Go Directory
    participant Summary as Care Context Capsule
    User->>Sela: Choose profession OR facility OR name
    opt Optional input
       User->>Sela: Type a city/province manually
    end
    Sela->>Catalog: Query verified public entries
    Catalog-->>Sela: Sourced results, last verified timestamp
    Sela-->>User: Choose, compare, or open official contact
    opt Prepare user-owned note
       User->>Summary: Pick approved source versions
       Summary-->>User: Editable, source-linked preview
       User->>Summary: Approve and export personally
    end
~~~

The UI never invokes browser geolocation, silently infers IP-based city, or requires address. Public search results do not imply live schedules. The user decides whether and how to contact a provider; Sela does not send anything autonomously.

## 4. Care request management: simulation vs future real integrations

**Now — real Sela functionality:** search directory, prepare source-versioned consultation summary, export and manually initiate contact through verified links.

**Hackathon partner sandbox:** operator may use fictional entities and fictional appointments to show an extensible, multi-organization workflow. Every record and screen is stamped `synthetic=true` and visibly shows **"Simulasi — tidak dikirim ke fasilitas kesehatan"**.

**Future actual partner system:** only after legal agreement, authenticated operator/professional roles, technical integration, contractual retention and purpose-scoped consent. A user-selected disclosure may be transmitted securely only when the user explicitly authorizes that exact recipient, exact capsule version, and purpose. A provider's confirmed receipt is not a confirmation that care has occurred.

~~~mermaid
stateDiagram-v2
    [*] --> Draft
    Draft --> Prepared: user approves contents
    Prepared --> Cancelled: user cancels
    Prepared --> Exported: user downloads
    Prepared --> SyntheticPending: sandbox-only demonstration
    SyntheticPending --> SyntheticReceived: fictional partner event
    SyntheticReceived --> SyntheticAcknowledged: fictional acknowledgement
    SyntheticPending --> SyntheticRevoked: user cancels
    SyntheticReceived --> SyntheticRevoked: user cancels future access
    SyntheticAcknowledged --> SyntheticClosed: synthetic completion
    Exported --> [*]
    SyntheticClosed --> [*]
    SyntheticRevoked --> [*]
    Cancelled --> [*]
~~~

In the partner sandbox, statuses are **never mapped to a real facility**. The future real implementation needs its own separately reviewed state machine (requested, accepted, declined, needs information, expired, revoked, confirmed appointment) with externally signed evidence and explicit user communications.

## 5. Data ownership

- Public provider data belongs to versioned, sourced directory records.
- Personal story, medical-sensitive details and proposed summaries remain under anonymous owner encryption.
- A Care Context Capsule is version-bound user-owned content; staff have no implicit read access.
- Partner access (future) checks tenant, verified professional, exact recipient, grant scope, expiration, revocation, source validity and event-specific purpose on every request.
- **No list of anonymous help seekers** appears in Admin Console. A user choosing a profession does not create an operator-visible case automatically.
- Appointment reminder/notification is disabled unless a real service and user-initiated communication channel exist and have explicit consent; no fake email or SMS promises.

## 6. Suggested data and service boundaries

| Entity / command | Authority and content |
| --- | --- |
| directory_organization, directory_facility, practitioner_public_profile, practitioner_role | PostgreSQL public metadata + provenance |
| directory_revision, reviewer_decision | PostgreSQL immutable review trail |
| support_service | Approved public service taxonomy / descriptions |
| care_capsule | Encrypted owner record and source-version hash |
| partner_tenant, partner_user (future) | Separate authenticated partner identities |
| partner_grant (future) | Signed, consented, versioned access scope |
| partner_request (future) | Durable transactional request with audit, **not Redis-only** |
| synthetic_request | Isolated sandbox namespace with fake IDs and mocked side effects |
| cache:public:directory | Only non-sensitive publicly sourced derived results, rev-stamped + TTL |

## 7. Admin pages and task-specific permissions

- **Directory / Organizations:** drafts, publication, official source and version history.
- **Directory / Professionals:** credential/affiliation verification state (no automatic AI endorsement).
- **Directory / Services:** taxonomy, modalities, source links, freshness.
- **Verification queue:** independent approval, conflicting source reviews, expiry.
- **Partner sandbox:** synthetic organizations and case transitions; no production tokens.
- **Future partner onboarding:** agreement checklist, legal review, certificate/token management, tenant isolation, test-only endpoints.
- **Audit & incidents:** who changed a listing, why, when, what public version changed; never show private journal text.

## 8. Acceptance and adverse cases

1. A search for a profession with a blank city must work; browsers denying location permission behave identically.
2. A practitioner belonging to two facilities displays correct source-backed roles without duplicate identity confusion.
3. The operator can unpublish an incorrectly sourced listing and ensure the public view no longer claims it is verified despite a warm cache.
4. User consent revocation/source edit makes a previously approved capsule stale and blocks further server sharing.
5. A synthetic partner's `received` event never triggers real email, webhook or medical booking call.
6. An operator viewing aggregate planned-care usage cannot recover a user's original text, IP or private destination choices.
7. No actual partner action is marked successful without authenticated response provenance.
8. Emergency help remains available with Meta, Redis, admin and partner simulator all offline.

## 9. Future standards

HL7 FHIR Organization, Practitioner, PractitionerRole, Consent, ServiceRequest and Appointment can guide interoperability. Sela is **not** currently integrated with SATUSEHAT, licensed as telemedicine, or partnered with any named hospital. Do not claim otherwise.

## 10. Implemented local demo (2026-10-30)

- Migration `0700_care_directory`: `care_directory_entries`, `care_sandbox_requests` and `care_audit`. No table references a consumer user or stores a story.
- Public `GET /api/v3/care-directory` and the `/support/directory` page: search by text, kind and an optional manually typed city; no geolocation; only verified entries whose re-verify date is current; source publisher, source date and official links shown; no request, booking or contact action.
- Operator `/operator`: directory list, create draft, edit, submit, verify/reject/unpublish with `expectedVersion`; a real entry requires creator ≠ reviewer, while a labelled synthetic demo entry may be self-verified. The `/api/admin/v1/directory/audit` view and panel show content-free care events.
- Synthetic sandbox: `care_sandbox_requests` and `/api/admin/v1/sandbox/requests`; states `prepared → pending → received → acknowledged → closed`, or revoked. Every row is `synthetic = true` and the UI says **Simulasi**. No email, webhook, booking or facility call exists.
- `sela care seed-demo --admin USERNAME` inserts three fictional entries and two fictional requests in development/test/demo only; production and staging refuse it.
- Tests: `internal/care` (editorial flow, public filter, self-verify guard, sandbox transitions, seed), HTTP `TestCareDirectoryEditorialFlowAndSandbox`, web planned-care and operator-panel tests, and a browser screenshot review.

Not done: real sourced partner data, professional credential/affiliation verification, freshness scheduler, Redis cache invalidation for the public directory, partner onboarding, patient-approved capsule transfer and any booking. The demo does not claim a hospital partnership.
