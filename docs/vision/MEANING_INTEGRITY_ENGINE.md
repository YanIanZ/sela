# SELA Meaning Integrity Engine (MIE)

**Proposed flagship technology** · version MIE-0.1 · 29 October 2026  
**Stage:** design; not a claimed invention patent, first-in-world feature, or finished runtime.

## 1. Core problem

An AI can subtly rewrite a person's meaning: "sometimes" becomes "always"; "I need solitude" becomes "I isolate myself"; tentative guesses turn into permanent memories. Merely displaying an Edit button does not prevent these mistakes from propagating to summaries and professional-care notes.

**MIE makes meaning a versioned, source-bound, user-governed dependency graph rather than an opaque model memory.**

## 2. Four distinct categories

| Category | Example | Rule |
| --- | --- | --- |
| user_original | "Kadang aku capek setelah kuliah." | Preserve verbatim with a stable source ID and content version |
| ai_interpretation | "You may be exhausted after class." | Provisional and untrusted; never a diagnosis or a new user fact |
| user_correction | "Bukan menarik diri; aku hanya perlu istirahat." | Becomes an explicit constraint against the rejected reading |
| external_evidence | A reviewed source on stress and sleep | Informational only; cannot establish the user's condition |

**Unknown** is a first-class state: a user has not stated, does not know, or does not wish to discuss a cause. Do not fill it with inference.

## 3. Data contract (conceptual)

~~~json
{
  "claim_id": "claim_random_id",
  "owner_subject": "internal_random_subject_only",
  "claim_type": "ai_interpretation",
  "source_refs": [
    { "record_id": "journal_abc", "version": 4, "span_start": 0, "span_end": 37 }
  ],
  "text": "User-proposed wording only",
  "certainty": "tentative",
  "review": "pending",
  "scope": ["reflection"],
  "model_provenance": "live_meta",
  "verifier_status": "pending",
  "derived_from": ["journal_abc@4"],
  "expires_or_stales_on_source_change": true
}
~~~

The JSON is a **design contract**, not a current endpoint or database schema. Never send owner_subject to AI. Spans must use one documented offset unit (prefer Unicode scalar indexes or UTF-8 byte indexes with strict tests), not mixed JS/Go length semantics.

## 4. Meaning rules

- MR-01: user wording is never overwritten by a generative rewrite; revisions are separate user-controlled versions.
- MR-02: unapproved AI interpretations are non-authoritative; default storage is no.
- MR-03: preserve subject, negation, time, modality, frequency and uncertainty; reuse existing Meaning Lock validators.
- MR-04: a rejected interpretation cannot be used as evidence for a future one.
- MR-05: correction invalidates *all* derived memory, pattern explanations, research-personalization proposals and consultation summaries referring to the superseded version.
- MR-06: consent and record deletion invalidate retrieval/continued processing; memory does not secretly persist sensitive negative preferences.
- MR-07: external research can be attached as educational background but may not become a private medical claim.
- MR-08: Claude review checks source fidelity where required; the user retains final interpretation authority; a verifier cannot create or approve a record.
- MR-09: stale asynchronous Meta/Claude/DeepSeek results are discarded rather than saved.
- MR-10: crash/retry/duplicate event delivery does not resurrect stale or deleted claims.

## 5. User-facing interaction

1. Show **what you said** with a source quote.
2. Show **what Sela thinks you might mean**, explicitly labelled provisional.
3. Provide **Correct / Accept / Reject / Don't save** and a no-AI route.
4. Display **what Sela does not know** rather than guessing causes.
5. Permit a simple **Why is this here?** explanation linking source and model task.
6. When an approved source changes, visibly mark previous derived material **needs review**, not silently reuse it.

## 6. Lineage and invalidation

~~~mermaid
flowchart TD
    J["Journal v4 — user original"] --> R["Reflection R1 — approved"]
    R --> M["Memory M1 — approved"]
    M --> S["Consultation summary S1 — approved"]
    J2["Journal revised to v5"] --> X["Invalidate descendants of v4"]
    X -.-> R
    X -.-> M
    X -.-> S
    J2 --> N["New proposals require review"]
~~~

**Transactional requirement:** revision + dependent invalidation must be atomic. Export/sharing reads must revalidate source hashes and approval versions under the same consistency model; otherwise an old summary may leak after a correction.

## 7. Assumption Firewall

This is the **policy layer** within MIE, not another model that can overrule Go. It checks:

- unsupported diagnosis, invented cause, categorical personality labels;
- unwanted certainty (always/never, fact vs hypothesis);
- reversal of negation, speaker or timeframe;
- missing source span, inaccurate quotation or fabricated literature link;
- use of a rejected or deleted claim;
- prohibited scope (e.g., journal memory used for professional sharing without consent).

Go performs deterministic checks. Meta may suggest labels or missing context. Claude verifies selected source-sensitive outputs. Unknown or invalid output **abstains**; it is not routed through a weaker provider to obtain approval.

## 8. Research questions and evaluation

Test against real-world-like **synthetic** bilingual/code-switched stories containing negation, indirect speech, slang, uncertain causality, temporal changes and contradictory statements.

Report:
- unsupported-inference rate (lower is better);
- source-span accuracy and citation fidelity;
- semantic preservation of subject, negation, certainty and time;
- corrected-claim reuse rate (target **zero** in a pinned suite);
- stale-derived export leakage (target **zero**);
- reviewer override and abstention cases, with false-positive and false-negative breakdown;
- accessibility success for reviewing a claim.

Establish a baseline against ordinary summarization without MIE. Fix failing adversarial cases before release; do not claim a zero error rate outside the tested corpus.

## 9. Originality boundary

Source-linked summaries, editable AI memory and correctable profiles exist in other products. MIE's **proposed differentiator** is end-to-end enforcement of meaning constraints and correction-driven invalidation across chat, personal memory, evidence, and care preparation. The claim must be demonstrated with an implementation and comparative tests, not marketing adjectives.
