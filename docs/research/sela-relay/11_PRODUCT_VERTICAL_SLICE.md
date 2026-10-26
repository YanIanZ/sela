# Sela Relay: a complete, consent-controlled journey

**Status: PROPOSED refinement**, 26 October 2026. This elaborates the existing [Sela Relay proposal](02_PRODUCT_STRATEGY.md), [journey](03_JOURNEY_AND_UX.md) and [SPEC.md](../../../SPEC.md); none of the new mechanisms is implemented merely because it appears here.

## North star

**ID:** Sela bukan AI terapis. Sela adalah jembatan yang membantu orang mempersiapkan bantuan profesional, menjaga makna ceritanya, dan mengetahui apa yang benar-benar terjadi pada proses serah terima.

**EN:** Sela is not an AI therapist. It is a bridge to professional support: helping people prepare what they want to say, keep control of their story, and understand what actually happened during a handoff.

Initial audience: Indonesian young adults preparing for a first professional conversation; demonstrate the same system in English. The app should be useful to a person who declines AI or does not want to share identifying details.

## The coherent capability: Sela Relay

This is one end-to-end experience, not a bundle of unrelated AI features:

| Capability | User job | Engineering differentiator | Priority |
| --- | --- | --- | --- |
| **Story Ledger** | Check how a story became a note | Each draft claim links to an exact synthetic source span; corrections are visible | P0 |
| **Meaning Lock** (new proposal) | Prevent meaning changes before sharing | Version-locked checks for speaker, date, negation, uncertainty and ID/EN translation differences | P0 |
| **Next Step** | Find a practical way forward | Explicit, deterministic filters for fictional language, location, format, timing and cost | P0 |
| **Opening Practice** | Know how to begin a consultation | Optional, source-bound two-sentence opening; no roleplay therapist | P1 |
| **Handoff Receipt** | Know whether the intended recipient received the note | Double-sided snapshot and receipt, distinguish sent from acknowledged; revocable access | P0 |
| **Return Bridge** (new proposal) | Return to the next practical step | A scripted fictional-professional follow-up with status and next action, no treatment generation | P1 |

The contribution is the *combination* of source fidelity, user authority and two-sided continuity. This is a candidate product differentiator; do not claim worldwide originality, patents, scientific novelty or superior clinical outcomes.

### Meaning Lock design

A source-backed claim includes fixture ID, evidence span, literal excerpt, typed meaning (concern / reported experience / uncertainty / user goal), temporal qualifier, negation, draft wording in current language and confirmation state. Meta may propose wording, but the deterministic validator must reject claims without a matching source or with unsupported timing, negation, diagnoses or subject attribution.

Meaning Lock surfaces a human-readable change preview before approval: **what the source said**, **what Meta proposed**, **what is shared**. When a translated version would change what recipients read, expose the translation as a distinct reviewed snapshot. Any consequential edit clears approval; the exact brief version, chosen fields, recipient and consent are attached to a share receipt. This is stronger than a generic AI confidence score. Never present a numeric diagnostic confidence score.

### Handoff Receipt and Return Bridge

Proposed simulated states: composed → approved → share-consented → simulated-requested → simulated-acknowledged → scripted consultation complete → fixture follow-up available. Alternative terminal/recovery states: refused, clarification-needed, expired, revoked. Additional states require revising the normative API/state machine first; they are **not yet part of SPEC.md**.

A requested handoff is not care received. The fictional professional sees **only** the immutable, recipient-specific shared projection; requests for clarification must return to the user's review, create a new version, and require a new explicit consent. Refusal or expiry never silently switches professionals. Return Bridge offers one predefined follow-up choice tied to the completed fixture; it neither assesses severity nor generates prescriptions.

## Full journey and bilingual interface contract

~~~mermaid
flowchart TD
 A[Choose synthetic scenario / Pilih skenario] --> B[Story and AI consent / Cerita dan persetujuan AI]
 B --> C{Deterministic crisis gate}
 C -->|Interrupt| X[Immediate-help screen / Hentikan alur biasa]
 C -->|Continue, no AI| M[Manual source-backed note]
 C -->|Continue, consent| D[Meta structured draft]
 D --> E[Story Ledger and Meaning Lock]
 M --> E
 E --> F[Practical filters and fictional options]
 F --> G[Approval and field-level sharing preview]
 G --> H[Simulated handoff receipt]
 H --> I{Fictional recipient status}
 I -->|Ack| J[Scripted consultation]
 I -->|Clarify| E
 I -->|Decline/expire| F
 J --> K[Fixture follow-up / Return Bridge]
~~~

### Screen contracts

| Screen ID / EN | Primary action | Visible proof and non-happy-path |
| --- | --- | --- |
| Mulai / Start | Choose one seeded adult | Fictional-data banner; anonymous/pseudonymous choice optional |
| Ceritaku / My Story | Review synthetic passages; permit or decline Meta processing | Consent purpose; ability to continue manually; crisis interruption |
| Catatanku / My Brief | Inspect claim-to-source; correct or omit | Meaning Lock, unknown facts, translated-source comparison, approval invalidation |
| Pilihan / Options | Apply explicit practical preferences | Deterministic rationale, empty-state alternatives; no made-up credentials/slots |
| Tinjau berbagi / Share Preview | Select recipient and fields; approve exact version; give separate share consent | A readable snapshot and irreversible-disclosure explanation |
| Status / Journey | Read request versus acknowledgment status | Requested, acknowledged, declined, expired, revoked; reset and recovery |
| Demo profesional / Professional Demo | Inspect only the shared snapshot | Explicit fictional role switch; no patient diary or private history |
| Langkah berikutnya / Next Step | Review fixture follow-up after scripted completion | No AI-written therapy, no reminders without separate opt-in |

Required on every screen: ID/EN translation parity, visible focus/keyboard use, accessible state messages and consistent **fictional / simulated** labels. Never equate language with jurisdiction.

### Experience principles

Quiet editorial style, warm neutrals, generous spacing and a single primary task per mobile screen. A progress indicator describes **journey steps**, never a mental-health score. Avoid guilt/streak mechanics, emotional dependence cues, fake provider reviews, urgency countdowns and prompts to reveal more than required. Pause and exit are first-class options.

## MVP, stretch, and hard exclusions

**P0 shipped demo:** one Indonesian synthetic adult; basic English fixture; manual path; Meta source-backed structured draft; Meaning Lock claim inspection; three fictional options; versioned approval and selected-field consent; simulated request and acknowledgment; revocation, reset and crisis fixture. Every P0 state must render in both languages.

**P1 after P0 stability:** Opening Practice, clarification/refusal recovery, fixture follow-up and concise share-history view.

**P2 only if measured quality and time permit:** optional synthetic-audio transcription after verifying ID/EN accuracy and actual access. No live microphone or authentic medical record ingestion.

**Excluded from hackathon runtime:** real patient intake, real booking, clinical diagnosis or triage, medication/therapy advice, provider credential verification, payment, forum, gamification, psychological scoring, production identity management and autonomous follow-up to real contacts.

## A measurable impact hypothesis

Compared with a static directory/manual note, a participant completing a *fictional* first-consultation task should be able to (1) accurately review what will be shared; (2) finish preparing a brief; (3) choose a practical option; and (4) identify whether the recipient has actually acknowledged it. Measure task success, correction burden, source-fidelity errors and consent comprehension, not reduction of depression or anxiety. See [evidence gates](10_EVIDENCE_APPRAISAL_AND_CLAIMS.md) and [demo protocol](13_JUDGING_AND_DEMO_PACKAGE.md).

## Preserve attribution

The current v1 Care Passport already includes source grounding and opening-line logic. Sela Relay refines a journey and state contracts; do not relabel all inherited functionality as new work. Keep [project provenance](../../SELA_PROVENANCE.md), license and authorship intact.