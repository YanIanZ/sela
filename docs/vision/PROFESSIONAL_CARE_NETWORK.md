# Sela | Professional Care Network

**Status:** target design; actual hospital/professional partnership integrations **not present**.  
**Core decision:** users choose the profession, provider and facility manually, **without GPS, background location or IP geolocation**.

## 1. Purpose

Sela should not stop at urgent-help links. It must also let a person find **non-urgent, planned professional support** on their own terms and prepare a user-owned summary. Help seeking is voluntary; Sela must not diagnose the need for a particular profession or automatically make referrals.

## 2. User flows

### A. Planned support (real product)
1. Open **Cari bantuan profesional**.
2. Choose **professional type** (e.g., clinical psychologist, psychiatrist, general practitioner, verified counseling service), **facility name**, or **practitioner name**.
3. Optionally type/select city or province; blank is valid.
4. Browse **real, source-verified directory records**, including source, verification date, professional license/affiliation verification status if available, and available public contact information.
5. Manually open the facility's verified official contact page/number or export a personal summary; **never present unknown booking availability**.
6. Independently prepare a Care Context Capsule consisting of *only* selected personal notes.

### B. Immediate help (real product)
Deterministic safety routing and source-verified help information are available without AI, GPS, a booking system or a partner. No auto-contact or invented emergency service promises.

### C. Partner collaboration (synthetic until contracts)
Use sandbox-only fictional hospital, clinic, practitioner and appointment records to show prepared → consented → simulated sent → simulated received → simulated acknowledged/revoked/expired. Display **SIMULASI — TIDAK DIKIRIM KE FASILITAS KESEHATAN** throughout. No real patient transfer or booking occurs.

## 3. Conceptual records

| Entity | Key fields | Rule |
| --- | --- | --- |
| Organization | org_id, legal_name, kind, source_url, verified_at | Verified identity and publicly sourced metadata |
| Facility | facility_id, organization_id, name, manual_city, services, source | Multiple facilities may belong to one org |
| Practitioner | practitioner_id, name, professional_type, verified_credentials_status | Don't call someone verified without evidence |
| PractitionerRole | practitioner_id, facility_id, role, affiliation_date, source | One practitioner may work at multiple facilities |
| Service | service_id, facility_id, modality, public_contact, update_date | Online/in-person only when sourced; costs/times may be unknown |
| CareContextCapsule | owner, selected_record_version_ids, approved_text_hash, created_at | User's notes, not medical record; approved and exportable |
| ConsentGrant | owner, exact scope, recipient, expiry, revoked_at | Consent by purpose; no implied access |
| SimulatedCareRequest | sandbox_id, synthetic_facility_id, state | Never maps to real facility identifier or endpoint |

HL7 FHIR concepts (Organization, Practitioner, PractitionerRole, Consent, ServiceRequest, Appointment) are **interoperability references**, not a claim of SATUSEHAT certification or a deployed clinical API.

## 4. Search contract

~~~mermaid
flowchart LR
    U["User chooses criteria"] --> Q["Manual query, optional typed city"]
    Q --> D["Go directory search"]
    D --> P["Source-verified public results"]
    P --> C["Choose profession / facility manually"]
    C --> E["Export own consultation summary"]
    C -. "Sandbox only" .-> S["Simulated partner exchange"]
~~~

- Browser geolocation API prohibited; no default nearest-me view and no hidden IP-based geolocation.
- Search supports blank city and nationwide results, pagination, normalized profession labels, ID/EN accessibility.
- A result may be **unverified or stale**, but then must be labelled and not represented as an available/endorsed practitioner.
- Search **does not imply real-time reservation availability**. Partner booking APIs, if ever added, require explicit authorized integration and confirmed schedules.
- Directory entries should not be fabricated by Meta; Meta may help describe profession types but cannot invent providers.

## 5. Care Context Capsule (user-owned handoff artifact)

What user selects: relevant note passages, dates, context they want to communicate, questions to ask, optional goals. Each selected statement must be linked to a current approved source version.

~~~mermaid
sequenceDiagram
    participant U as Anonymous user
    participant A as Sela Go API
    participant M as Meaning Integrity
    participant F as Care directory
    participant S as Synthetic partner sandbox
    U->>F: Search manually, no location permission
    F-->>U: Sourced facility / practitioner choices
    U->>A: Select records for summary
    A->>M: Revalidate sources and user approval
    M-->>U: Editable preview with source links
    U->>A: Approve export
    A-->>U: Downloadable personal notes
    opt Demonstrate future partner workflow
      U->>S: Submit synthetic example only
      S-->>U: Clearly simulated status
    end
~~~

No real facility receives the user's story from this simulation. User-owned exports may be shown to a chosen professional **outside Sela** at the user's discretion.

## 6. Future partner interface contract

Only after agreements, authority and security review: authenticated organizations and verified practitioner identities; least-privilege role scope; accepted payload versions, purpose/expiry, audit, revocation, retention and incident response; separation of multiple organizations; signed webhook events, idempotency and replay protection; explicit receipt acknowledgement vs actual care.

A user can remain pseudonymous inside Sela but a real facility may legally require patient identity for actual care. That identity should be collected by the facility or an approved transition—not automatically merged into a hidden Sela profile.

## 7. Security and data minimization

- Do not transfer complete journals or entire conversation history by default.
- Show an exact redaction and content preview immediately before user approval.
- Consent identifies recipient, purpose, precise fields, expiry and revocation method; downloading a prior copy cannot be undone.
- Synthetic sandbox uses a separate namespace/database or strict tenant barrier and never has production provider credentials.
- Keep provider directory provenance and periodic reverification logs (without user health content).
- No integration to real SATUSEHAT, hospital information systems, appointment confirmation, or clinician messaging is claimed.

## 8. Acceptance matrix

| Test | Expected |
| --- | --- |
| Browser blocks geolocation permission | Entire directory flow still works |
| User leaves city blank | Finds unfiltered, source-verified directory results |
| Practitioner affiliated with two facilities | Both roles shown without duplicates |
| Facility absent or stale | "Not verified/unavailable" displayed; not fabricated |
| Unapproved journal passage | Excluded from capsule/export |
| Source edited after approval | Capsule becomes stale before export |
| Consent revoked during send | Transaction aborts; no subsequent delivery |
| Simulated partner action | Synthetic banner and sandbox-only audit |
| No real partner exists | Manual public contact/export still available |
| Screen reader and mobile | End-to-end keyboard and accessible status labels |

## 9. External sources and legal gates

- [HL7 FHIR](https://hl7.org/fhir/) is a technical design reference only.
- [SATUSEHAT platform](https://satusehat.kemkes.go.id/platform/) needs separate onboarding/authorization; never assume open access to partner/patient data.
- Applicable Indonesian privacy, healthcare and data-sharing laws, provider contracts and professional credential verification require separate legal evaluation. This document is not legal certification.

## 10. Operational ownership and the Admin Console

Publication of real facility/practitioner entries requires an **authenticated operator editorial workflow** and an independent reviewer with cited evidence; it is not a generative AI task. Sela operators curate the **public directory only**, while individual user browsing and journal data remain private. Care requests are not created merely because a user searches for a professional. There is no automated assignment, professional booking or operator-visible "patient queue" without a lawful, explicit, future partner workflow.

The sandbox supports fictional facilities and fictional request state transitions; it must never trigger real webhooks, emails or appointments. Redis may cache published, revisioned public metadata but cannot store care capsules, partner credentials, approval or actual case state.

[Admin Console](ADMIN_CONSOLE.md) · [Planned Care Operations](ADMIN_CARE_OPERATIONS.md) · [Redis policy](REDIS_AI_COST_ENGINEERING.md).
