# Sela v2 product

Status: **implemented** in `web/` and `/api/v2` (demo scope); see [implementation status](IMPLEMENTATION_STATUS.md) for what is verified. Authority: [SPEC](../SPEC.md) §G, C1–C14, I.web, V1–V8, V13.

Sela is a bridge to professional support, not an AI therapist. It helps a person put their story into words they understand and approve, choose a practical next step, rehearse how to start, share only what they choose, and see what actually happened after a (simulated) handoff. Everything in the demo is synthetic or fictional.

## Journey

| Stage (ID / EN) | What the user does | What the system guarantees |
| --- | --- | --- |
| Mulai / Start | Reads what Sela is and is not; picks language | Synthetic-demo banner; Indonesian and English everywhere |
| Sebelum mulai / Before you start | Reads privacy and boundaries; anonymous or pseudonym; adult attestation | Versioned privacy notice; no real identity collected |
| Izin / Permissions | Decides AI processing and retention separately | Neither implies the other or sharing; changeable in Settings |
| Ceritaku / My story | Chooses a fictional sample (Indonesian, English, code-switched, crisis test) or writes a fictional practice story | Safety gate before anything else; contact details rejected; memory-only |
| Catatanku / My note | AI draft (streamed, cancelable, retryable) or manual selection; reviews each item: own words vs Sela's wording, qualifiers, notes; edits/removes; answers or skips one clarification; approves version *n* | Only validated claims; every edit creates a new version and clears approval |
| Langkah / Next step | Sets language, format, cost, time and one optional barrier | Deterministic fictional matches with reasons, near matches, honest empty state, non-handoff steps first when "not ready" |
| Latihan / Practice | Drafts opening lines (AI or template), rewrites them, rehearses | Lines grounded in the approved note; feedback is "mentioned / not yet", never a score; rehearsal not stored |
| Tinjau berbagi / Review sharing | Picks fields and duration, sees the exact recipient preview, consents explicitly | Version-bound, recipient-bound, field-minimised projection; idempotent request |
| Perspektif penerima / Recipient (fictional) | Second window: link + access code; acknowledges, asks for clarification, declines or completes a scripted consultation | Only the projection; demo authorization, not real identity |
| Perjalananku / My journey | Sees true status, timeline, expiry; revokes; reads the fixture plan; checks in | "Sent" is never "care received"; revoke blocks future access |
| Pengaturan / Settings | Language, identity, permissions, retention, delete everything | Deletion purges session, shares and recipient access immediately |
| Bantuan segera / Immediate help | Verified Indonesian services with sources and verification dates | Crisis route stops the ordinary journey |
| Riset / Research | Reviewer search of the evidence map | No session data; metadata and annotations only |

Feature lineage: source-grounded Care Passport, clarification and opening-lines logic come from the inherited Dengar.in v1 and were ported and extended (Meaning Lock, versioned approval, recipient handoff). The fictional recipient perspective, share tokens + codes, receipts, follow-up and the Go/Meta runtime are new in v2. See [provenance](SELA_PROVENANCE.md).

## Excluded

Forum, gamification, general chatbot, diagnosis/prescribing/triage, therapy roleplay, payments, real scheduling or professionals, live microphone, health-record integration and model training.

Optional, separately consented Anthropic Sonnet 5.5 source-fidelity review (EvidenceGuard, C12) is a standalone prototype, never a substitute for Meta generation or human approval, and not part of the current journey ([contract](CLAUDE_VERIFIER.md)).
