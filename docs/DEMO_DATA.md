# Synthetic demo data

Status: **implemented** in `internal/fixtures/data/*.json` (version `sela-fixtures-2026.10.26-2`), embedded in the binary and validated at start-up. Authority: [SPEC](../SPEC.md) I.data, C3/C7/C8, V1/V2/V6/V7/V8/V11.

## Fixture families

| File | Contents | Validation at load |
| --- | --- | --- |
| `scenarios.json` | Four fictional adults: Nadia (Indonesian, overwhelm, cost), Sam (English, shift work, minimal sharing), Dimas (code-switched), and a crisis-routing test with a content warning. Each seeded scenario has gold facts (quote, kind, negation, subject, time, certainty, importance) and forbidden patterns for evaluation | Both languages present; quotes occur in the story; patterns compile; no contact-like data |
| `professionals.json` | Six visibly fictional profiles ("(fiktif)"/"(fictional)") across categories, languages, formats, cost tiers and availability, with slots on a fixed demo clock; one service has only full slots to show an honest empty state | Fictional names, valid enums, slots on the demo clock, plans exist |
| `plans.json` | Three fixture follow-up plans attributed to fictional professionals; one states explicitly that the demo has no prescription or treatment decision | Translations complete |
| `options.json` | Demo clock, check-in options, recipient clarification/decline reasons, non-handoff next steps per barrier, clarification question templates | Translations complete |
| `help.json` | Real Indonesian services copied only from v1 entries marked `verified_official` (Healing119, LISA Helpline, Yayasan Pulih) with sources, access dates (2026-10-14/15), verifier and re-verification date | Only `verified_official`; https sources with dates |
| `policies.json` | Versioned privacy, AI-processing, storage and sharing texts that consent decisions reference | Versions present, translations complete |

Language changes select translations of the same records; story text and quotes stay in the persona's own language. The demo clock (2026-11-06, slots 10–15 November) is shown as a fixed demo date; session expiry uses real time.

## Practice text

Besides the fixtures, a user may write a fictional practice story (D-003): 20–3000 characters, adult attestation, safety gate first, obvious contact details rejected, memory-only. It is not used for evaluation or training.

## Isolation

All session state is per visitor, in memory, under the retention policy; reset, deletion and expiry purge it with its shares and recipient principals. Fictional recipients are instantiated per owner session, so no cross-visitor inbox or directory exists. A stale model result (changed story, ledger or consent) is discarded.
