# Sela | Meta AI, research evidence and safety

**Status:** architectural contract / planned integration, 29 October 2026.  
**Scope:** text only; adult synthetic evaluation until real-data handling is authorized.

## 1. Responsibilities are different

| Component | Allowed job | Must never do |
| --- | --- | --- |
| Go safety rules | Consent gate, deterministic crisis routing, bounded input, output release | Let any model silently downgrade a crisis or bypass a refusal |
| Meta primary | Listening response; structured reflection; source-anchored extraction; propose memory and consultation wording | Diagnose, treat, write a medical record, approve memory/share, invent citations |
| Go Meaning Integrity | Quote/negation/time/certainty checks, stale-version protection, consent and lineage | Treat an AI interpretation as a user assertion |
| Claude Sonnet 5.5 | Independent source-fidelity review for selected sensitive tasks | Act as truth oracle, replacement generator, or silently send more data than consented |
| DeepSeek fallback | Optional substitute generator **only** for Meta availability outage | Safety fallback, refusal laundering, default vendor change |
| Research library | Curated educational context and reproducible citations | Establish a user's diagnosis; train on private notes |
| User | Accept, edit, withhold, export, delete | Be compelled to explain or disclose |

## 2. Model decision

- **Target hosted Meta primary:** evaluate Muse Spark 1.3 and an optional Muse Wonder tier *if the account, rules and available API support them*.
- **Existing code path:** Meta Muse Glimmer 30B through Modal/vLLM; existing pin and adapter remain historical/operational facts. No live call is demonstrated by this document.
- **Do not pretend a rename changes a model:** record actual provider, model ID/revision, region, latency, token usage and deployment mode.
- **Model policy review is required** before sending wellbeing or sensitive health-related data. No processing of under-18 data until applicable model age/policy and legal gates are cleared.
- **Use Meta directly for meaningful operations:** propose_reflection, extract_source_anchors, compare_uncertainty, retrieve_evidence, draft_user_owned_summary and explain_limitations. These are proposed task identifiers, not current public API names.
- **Never use a generative model as the final crisis classifier.** If Meta offers moderation capabilities, they may *raise* an internal concern for human/safe handling but never reduce Go's safety classification.

## 3. Trust flow

~~~mermaid
flowchart TD
    A["Text input"] --> B["Go: safety, consent, age/policy, redaction"]
    B -->|blocked| H["Help / manual mode (no provider calls)"]
    B -->|allowed| M["Meta structured task"]
    M --> G["Go: schema, quote, meaning, evidence, source and version checks"]
    M -. "Eligible outage before any response" .-> X{"Separate DeepSeek consent?"}
    X -->|yes, enabled| D["DeepSeek fallback"]
    X -->|no| H
    D --> G
    G -->|failed| H
    G -->|pass; required verifier| C["Claude Sonnet 5.5"]
    G -->|pass; verifier not required| U["Unconfirmed user-visible proposal"]
    C -->|pass| U
    C -->|unavailable/reject| H
    U --> E["User edits / approves / rejects"]
    E -->|explicitly approved + consent| P["Encrypted personal records"]
~~~

A DeepSeek-generated listening reply follows the stricter verifier requirement from the existing ADR; if the verifier cannot run, abstain.

## 4. Fallback truth table

| Situation | DeepSeek? | Action |
| --- | --- | --- |
| Meta timeout, transient 5xx or bounded 429, before first content | Only if separately enabled/consented and data eligible | Attempt once within budget; label vendor |
| Model refusal, safety flag, suspected harm | **No** | Stop and handle appropriately |
| Invalid citation, schema or unsupported claim | **No** | Reject or user manual path |
| Meta already started streaming content | **No** | Discard incomplete result |
| No provider consent or revoked consent | **No** | User-owned manual path |
| DeepSeek unavailable/Claude required but unavailable | **No further downgrade** | Show unavailable; do not release verifier-gated output |

Consent to Meta does **not** imply consent to Claude or DeepSeek. Provider data retention/region statements must cite current contractual documentation, not assume zero retention.

## 5. Two knowledge stores, never conflated

**Personal journal**: encrypted user-owned source, with versions; only approved fragments are eligible for memory or external model retrieval. Never training data, public research, or shared by default.

**Research library**: public licensed research metadata plus curated passages with license, DOI/source URL, publication date, study type, population, research limitations, screening and reviewer status. An article title or a BM25 metadata hit **does not prove a clinical claim**.

The current internal/research index searches metadata (BM25) and supports reviewer exploration; it is **not** a fully validated RAG or a patient-specific clinical engine. The existing 100-publication register was screened at title/abstract level, not independently appraised full text; the 5,000-candidate pipeline is collection, not 5,000 validated studies.

## 6. Evidence pipeline

~~~mermaid
flowchart LR
    S["WHO / NHS / Europe PMC / licensed publishers"] --> I["Ingest metadata + license"]
    I --> SC["Deduplicate, screen, human quality appraisal"]
    SC --> C["Curate authorized passages + anchors"]
    C --> R["Retrieve only for question type"]
    R --> M["Meta synthesizes general information"]
    M --> V["Go verifies citation IDs + source spans"]
    V --> CL["Claude verifies selected sensitive claims"]
    CL --> O["Educational response with limitations"]
~~~

Do not bulk copy copyrighted articles, scrape in violation of terms or cite papers unseen by the model/validator. Explicitly label "limited evidence", "population differs" and "unknown" where relevant.

## 7. Privacy and clinical boundary

- Anonymous-first does not mean invisible to network observers or exempt from provider telemetry.
- Go may decrypt user-approved text to provide service, therefore Sela should **not** claim end-to-end encryption between users.
- Model-dependent workflows need minimal purpose-specific content, location/legal transfer notice and separate opt-in.
- Use verified official help resources; do not offer unverifiable emergency telephone numbers.
- No mental-health diagnosis, treatment protocol, medication advice, automated triage or severity scores.
- Meaning Integrity prevents semantic distortions, but **does not establish medical or clinical safety**. Seek independent expert and legal review before real-user/pilot use.
- The draft safety ruleset in PR #8 has unresolved independent findings; green CI alone must not authorize merge or change crisis release policy.

## 8. Evaluation gates

- Run every reported adversarial corpus with committed fixtures, without claiming unavailable external test sets were executed.
- Guardrail: no model call along deterministic crisis block path; no safety-to-provider bypass; no unauthorized provider call after consent revocation.
- Accuracy: check quote fidelity, source span, health claims, uncertainty, correction persistence and lineage invalidation in ID/EN/code-switch inputs.
- Reliability: provider outage simulations, transient retry behavior, cancellation, deadline and stale-result handling.
- Privacy: redaction, no story content in logs, isolation of synthetic/real data, retention/deletion tests.
- Model access: demonstrate real provider response with recorded model ID, timestamp, non-sensitive tracing, without uploading real patient information.
- Human review: editorial appraisal for public content and qualified safety/legal review for sensitive deployment.

## 9. Primary references

- [Existing AI policy](../AI_POLICY.md), [AI routing ADR](../adr/ADR-0003-ai-routing-verifier-fallback.md), [Claude verifier contract](../CLAUDE_VERIFIER.md), [DeepSeek fallback](../DEEPSEEK_FALLBACK.md).
- [Meta developer documentation](https://dev.meta.ai/docs/), [Anthropic API documentation](https://platform.claude.com/docs/), [DeepSeek API documentation](https://api-docs.deepseek.com/).
- [Research expansion licensing and caveats](../research/sela-relay/RESEARCH_EXPANSION_PIPELINE.md), [Safety](../SAFETY.md), [Blockers](../BLOCKERS.md).
