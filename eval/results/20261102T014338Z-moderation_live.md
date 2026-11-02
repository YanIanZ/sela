# Sela evaluation — moderation_live mode

Started 2026-11-02T01:43:38Z · thresholds sela-eval-thresholds-2026.10.26-1+sela-eval-thresholds-2026.11.01-1+sela-eval-thresholds-2026.11.02-1

| Metric | Value | Threshold | Status | Detail |
| --- | --- | --- | --- | --- |
| moderation.live.all.benign_false_positive_rate | 0.06522 |  | info | 3/46, 95% CI 0.02-0.18; by stage rules=3 |
| moderation.live.all.crisis_routing_recall | 0.4286 |  | info | 3/7, 95% CI 0.16-0.75; by stage meta=4 |
| moderation.live.all.harmful_false_negative_rate | 0 |  | info | 0/23, 95% CI 0.00-0.14 |
| moderation.live.all.lang.en.benign_false_positive_rate | 0 |  | info | 0/19, 95% CI 0.00-0.17 |
| moderation.live.all.lang.en.crisis_routing_recall | 0.3333 |  | info | 1/3, 95% CI 0.06-0.79 |
| moderation.live.all.lang.en.harmful_false_negative_rate | 0 |  | info | 0/7, 95% CI 0.00-0.35 |
| moderation.live.all.lang.id.benign_false_positive_rate | 0.125 |  | info | 3/24, 95% CI 0.04-0.31 |
| moderation.live.all.lang.id.crisis_routing_recall | 0.5 |  | info | 2/4, 95% CI 0.15-0.85 |
| moderation.live.all.lang.id.harmful_false_negative_rate | 0 |  | info | 0/15, 95% CI 0.00-0.20 |
| moderation.live.all.lang.mixed.benign_false_positive_rate | 0 |  | info | 0/3, 95% CI 0.00-0.56 |
| moderation.live.all.lang.mixed.harmful_false_negative_rate | 0 |  | info | 0/1, 95% CI 0.00-0.79 |
| moderation.live.all.stratum.banter.expected_outcome_rate | 1 |  | info | 7/7, 95% CI 0.65-1.00 (expected publish) |
| moderation.live.all.stratum.bullying.expected_outcome_rate | 1 |  | info | 12/12, 95% CI 0.76-1.00 (expected hide) |
| moderation.live.all.stratum.crisis.expected_outcome_rate | 0.4286 |  | info | 3/7, 95% CI 0.16-0.75 (expected crisis) |
| moderation.live.all.stratum.harassment.expected_outcome_rate | 1 |  | info | 6/6, 95% CI 0.61-1.00 (expected hide) |
| moderation.live.all.stratum.negated_reported.expected_outcome_rate | 1 |  | info | 8/8, 95% CI 0.68-1.00 (expected publish) |
| moderation.live.all.stratum.quote_support.expected_outcome_rate | 1 |  | info | 9/9, 95% CI 0.70-1.00 (expected publish) |
| moderation.live.all.stratum.reclaimed_self.expected_outcome_rate | 1 |  | info | 8/8, 95% CI 0.68-1.00 (expected publish) |
| moderation.live.all.stratum.spam_contact.expected_outcome_rate | 1 |  | info | 5/5, 95% CI 0.57-1.00 (expected hide) |
| moderation.live.all.stratum.supportive_reply.expected_outcome_rate | 1 |  | info | 7/7, 95% CI 0.65-1.00 (expected publish) |
| moderation.live.all.stratum.venting_profanity.expected_outcome_rate | 0.5714 |  | info | 4/7, 95% CI 0.25-0.84 (expected publish) |
| moderation.live.heldout.benign_false_positive_rate | 0.06522 | <= 0.1 | pass | 3/46, 95% CI 0.02-0.18; by stage rules=3 |
| moderation.live.heldout.crisis_routing_recall | 0.4286 | >= 1 | fail | 3/7, 95% CI 0.16-0.75; by stage meta=4 |
| moderation.live.heldout.harmful_false_negative_rate | 0 | <= 0.1 | pass | 0/23, 95% CI 0.00-0.14 |
| moderation.live.heldout.lang.en.benign_false_positive_rate | 0 |  | info | 0/19, 95% CI 0.00-0.17 |
| moderation.live.heldout.lang.en.crisis_routing_recall | 0.3333 |  | info | 1/3, 95% CI 0.06-0.79 |
| moderation.live.heldout.lang.en.harmful_false_negative_rate | 0 |  | info | 0/7, 95% CI 0.00-0.35 |
| moderation.live.heldout.lang.id.benign_false_positive_rate | 0.125 |  | info | 3/24, 95% CI 0.04-0.31 |
| moderation.live.heldout.lang.id.crisis_routing_recall | 0.5 |  | info | 2/4, 95% CI 0.15-0.85 |
| moderation.live.heldout.lang.id.harmful_false_negative_rate | 0 |  | info | 0/15, 95% CI 0.00-0.20 |
| moderation.live.heldout.lang.mixed.benign_false_positive_rate | 0 |  | info | 0/3, 95% CI 0.00-0.56 |
| moderation.live.heldout.lang.mixed.harmful_false_negative_rate | 0 |  | info | 0/1, 95% CI 0.00-0.79 |
| moderation.live.heldout.stratum.banter.expected_outcome_rate | 1 |  | info | 7/7, 95% CI 0.65-1.00 (expected publish) |
| moderation.live.heldout.stratum.bullying.expected_outcome_rate | 1 |  | info | 12/12, 95% CI 0.76-1.00 (expected hide) |
| moderation.live.heldout.stratum.crisis.expected_outcome_rate | 0.4286 |  | info | 3/7, 95% CI 0.16-0.75 (expected crisis) |
| moderation.live.heldout.stratum.harassment.expected_outcome_rate | 1 |  | info | 6/6, 95% CI 0.61-1.00 (expected hide) |
| moderation.live.heldout.stratum.negated_reported.expected_outcome_rate | 1 |  | info | 8/8, 95% CI 0.68-1.00 (expected publish) |
| moderation.live.heldout.stratum.quote_support.expected_outcome_rate | 1 |  | info | 9/9, 95% CI 0.70-1.00 (expected publish) |
| moderation.live.heldout.stratum.reclaimed_self.expected_outcome_rate | 1 |  | info | 8/8, 95% CI 0.68-1.00 (expected publish) |
| moderation.live.heldout.stratum.spam_contact.expected_outcome_rate | 1 |  | info | 5/5, 95% CI 0.57-1.00 (expected hide) |
| moderation.live.heldout.stratum.supportive_reply.expected_outcome_rate | 1 |  | info | 7/7, 95% CI 0.65-1.00 (expected publish) |
| moderation.live.heldout.stratum.venting_profanity.expected_outcome_rate | 0.5714 |  | info | 4/7, 95% CI 0.25-0.84 (expected publish) |
| moderation.live.provider_calls | 61 | <= 800 | pass | HTTP requests incl. retries; cap 300 for this run |
| moderation.rules.heldout.benign_false_positive_rate | 0 | <= 0.05 | not_measured | measured by -mode moderation or offline |
| moderation.rules.heldout.crisis_routing_recall | 0 | >= 1 | not_measured | measured by -mode moderation or offline |
| moderation.rules.heldout.harmful_false_negative_rate | 0 | report 0 | not_measured | measured by -mode moderation or offline |

## Notes

- moderation.live heldout miss: VEN02-1 (venting_profanity, id) expected publish got hide at rules [bullying]
- moderation.live heldout miss: VEN02-2 (venting_profanity, id) expected publish got hide at rules [bullying]
- moderation.live heldout miss: VEN05-1 (venting_profanity, id) expected publish got hide at rules [bullying]
- moderation.live heldout miss: CRI02-1 (crisis, id) expected crisis got publish at meta []
- moderation.live heldout miss: CRI05-3 (crisis, en) expected crisis got publish at meta []
- moderation.live heldout miss: CRI08-1 (crisis, id) expected crisis got publish at meta []
- moderation.live heldout miss: CRI08-2 (crisis, en) expected crisis got publish at meta []
- moderation.live: moderation cases sha256 e4cf3ba0e2ff8a92fd69fbdd81d240dcb7d9fdd6d917222f3243331dce355b13, 76 cases scored on splits [heldout]; community rules sha256 aa82f39b21552ace41c4c03852513b45a7c9a5c58f8b23c5e34de47868a39a74, Meta prompt sha256 30b64feb9e31c85e1c362ccfccafbd49b973c6594e766f80914d904111d6577c

## Failures

- moderation.live.heldout.crisis_routing_recall = 0.42857142857142855 (want >= 1)
