# Evidence-led refinement: 100-study register and claims policy

**Status:** research audit and engineering proposal, 26 October 2026. This is not a new systematic review, a clinical efficacy study or a completed app feature. [SPEC.md](../../../SPEC.md) remains authoritative.

## Verified local register integrity

Source: [100-publication JSON](research_100.json), [annotated register](09_RESEARCH_REGISTER.md), [methods](06_RESEARCH_METHOD.md).

| Local integrity check | Observation |
| --- | --- |
| Records | 100, identifiers R001–R100 |
| DOI field | 100 populated DOI fields |
| Duplicate normalized DOI strings | 0 detected in the local register |
| First-publication year | 2023: 2; 2024: 3; 2025: 86; 2026: 9 |
| Scope | Literature indexed under AI support, help-seeking, navigation, continuity, privacy, Indonesia, language equity, summarization, safety, engagement and anchor studies |
| Review status | Metadata and available-abstract screening; full-text appraisal pending |

The above are **file-level integrity checks**, not an independent revalidation of every publisher record. The corpus is heavily skewed to 2025, includes different designs and overlapping systematic reviews, and cannot be called 100 independent demonstrations of efficacy. All first-publication dates in the local register fall within the requested five-year window on 26 October 2026. Any future addition must respect the 26 October 2021–26 October 2026 date window for this snapshot.

Spot-checks against PubMed have corroborated the metadata of R001 (PMID 39839954, DOI 10.1177/20552076241313401) and R002 (PMID 40776275, DOI 10.3233/SHTI251258). This is not verification of the other 98 works.

## Five design conclusions, and how to avoid overclaiming

1. **Help-seeking has multiple barriers, not just a lack of emotional conversation.** The ASEAN-focused systematic review by Syafitri et al. (DOI [10.1177/00207640251367289](https://doi.org/10.1177/00207640251367289)) describes social/cultural, personal and structural barriers. Proposal: ask one optional practical barrier and present one intelligible next step. **Do not claim** that a directory solves affordability or provider shortages.
2. **Clinical language generation is not clinical readiness.** Reviews of generative LLMs in mental-health contexts describe sparse reliable clinical evaluation and notable gaps; see [npj Digital Medicine scoping review](https://doi.org/10.1038/s41746-025-01611-4) and [World Psychiatry systematic review](https://doi.org/10.1002/wps.21352). Proposal: constrain Meta to drafting, translation proposals and navigation explanations from synthetic sources. **Do not claim** autonomous counseling, diagnosis, risk assessment or treatment.
3. **Source fidelity is a product property, not a prompt aspiration.** Link every drafted claim to a fixture evidence span and require a user confirmation step. Reject inserted diagnoses, inverted negation, wrong time period, wrong speaker and unsupported facts before publication or sharing. An approved version is immutable for sharing.
4. **Handoff is not completed when a user clicks Send.** Proposal: a recipient-bound, version-bound receipt distinguishing submitted, acknowledged, clarification requested, declined, expired and revoked states. In this hackathon all actors and events remain **simulated**, not external referrals.
5. **Safety failures can occur across multi-turn interactions.** A 2026 Nature Medicine study on auditing chatbot behavior emphasizes cumulative interactions, and a 2025 Stanford investigation documents failure modes in AI therapy chatbots. Sela should minimize unconstrained interaction and test state transitions and model outputs separately. These external evaluations **do not prove Sela is safe**.

## Claim-to-feature evidence gate

| Feature or claim | Registry starting points | Counterevidence / transfer limits | Acceptable hackathon claim | Required evaluation |
| --- | --- | --- | --- | --- |
| Story Ledger and Meaning Lock | R065, R069, R096 | Summaries can omit details and hallucinate; synthetic texts differ from live care | Reconstructs grounded draft claims on held-out fixtures | Unsupported-claim rate, negation/time accuracy, coverage |
| Practical Next Step | R012, R019, R038, R047, R098 | Literacy gains may not translate to help-seeking | Reduces steps needed in a simulated navigation task | Completion/time versus a static directory |
| Controlled disclosure | R031, R043, R045 | Consent comprehension is not established by checking a box | Limits displayed fields and versions in the demo | Consent-comprehension questions; access-denial test |
| Handoff Receipt | R023, R027, R035 | Referral studies have mixed results and specific care contexts | Preserves accurate status across simulated handoff | State correctness, recovery task success |
| Bilingual parity | R057, R058 | Evidence from English/Korean may not transfer to Bahasa Indonesia | ID/EN labels and source-visible bilingual brief | Separate ID/EN/code-switch eval and human review |
| Safety boundary | R075, R081, R099 | Deterministic checks miss some variants; model safety is not proven | Crisis fixture interrupts before model/handoff | Negative-route tests; zero unsafe transitions in tested fixtures |

These mappings are **design hypotheses**, not causal validation. Full-text appraisal is required before stronger outcome claims.

## Reproducible appraisal worksheet

For each of R001–R100, future reviewers should record: full citation and stable DOI; first-publication date; paper type and population; country/language; sample size; study design and comparator; measured outcomes; adverse-event reporting; risks of bias or confounding; financing/conflicts; implementation context; whether a systematic review overlaps included primary studies; Sela relevance; explicit non-transferability; conclusion and reviewer/date. Never promote a title/abstract annotation to a full-text-reviewed label without reading and documenting that review.

**First detailed appraisal batch:** R023, R027, R031, R038, R047, R057, R065, R069, R087 and R098–R100. Prioritize contradictions, negative/null findings and Indonesia/ASEAN applicability. De-duplicate overlapping primary trials across reviews before any evidence-count claim.

**Quality gates:** (A) DOI metadata and status independently resolvable; (B) correct research design and risk-of-bias instrument; (C) claim maps to real measured outcome, not marketing extrapolation; (D) human professional review for any clinical interpretation; (E) separate ID/EN wording verification.

## Ethical experimentation and release gates

Use synthetic adults, fictional professionals and prepared outcomes for hackathon experiments. A small usability test with people evaluating a fictional task is not a clinical trial. Do not collect their mental-health histories, imply clinical benefit, or deploy autonomous clinical functions. Obtain appropriate consent and review if conducting human-subject research. Before moving beyond demo: establish licensed professional oversight, privacy/legal analysis, provider verification, redress, data-governance review and prospective safety evidence.

## Independent sources inspected for this refinement

- PubMed R001: https://pubmed.ncbi.nlm.nih.gov/39839954/
- PubMed R002: https://pubmed.ncbi.nlm.nih.gov/40776275/
- ASEAN help-seeking review: https://pubmed.ncbi.nlm.nih.gov/40928181/
- LLM scoping review: https://pubmed.ncbi.nlm.nih.gov/40307331/
- World Psychiatry review: https://doi.org/10.1002/wps.21352
- 2026 clinical audit study: https://www.nature.com/articles/s41591-026-04577-2
- Stanford summary of chatbot limitations: https://news.stanford.edu/stories/2025/06/ai-mental-health-care-tools-dangers-risks

The register retains the canonical 100 IDs; this document does not silently add seven papers to that count.