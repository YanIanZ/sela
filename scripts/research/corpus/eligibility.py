"""Publication-type eligibility shared by the title screener, the v3 gap search
and the corpus pipeline (rule ``sela-pubtype-eligibility-v1``).

Protocols, corrections/errata, editorials and retraction notices (or retracted
publications) are not study reports, so they never enter a review queue or the
evidence map. The publication type decides; an explicit title cue is a
fallback for records whose type omits the kind. Excluded rows are never
deleted: they keep every other label and carry the basis of the exclusion.
"""
from __future__ import annotations

import re

RULE_VERSION = "sela-pubtype-eligibility-v1"
# Europe PMC pubType / pubTypeList tokens (lower-cased).
INELIGIBLE_PUBTYPES = {
    "protocol": {"clinical trial protocol", "study protocol"},
    "correction_or_erratum": {"published erratum", "correction", "erratum", "corrigendum"},
    "editorial": {"editorial"},
    "retraction_notice_or_retracted": {"retraction notice", "retraction of publication", "retracted publication"},
}
INELIGIBLE_TITLE_CUES = {
    "protocol": (
        r"\b(?:study|trial|review|research|evaluation|pilot|feasibility|scoping|implementation)\s+protocol\b",
        r"(?:^|[:;.]\s*|\(\s*)(?:a\s+|an\s+|the\s+)?protocol\s+(?:for|of)\b",
        r"\bprotocol\s+for\s+(?:a|an|the)\s+(?:[\w-]+\s+){0,7}?(?:trial|study|review|evaluation|synthesis|"
        r"analysis|investigation|meta-analysis)\b",
    ),
    "correction_or_erratum": (r"^\s*(?:correction|erratum|errata|corrigendum)s?\s*(?::|to\b|for\b|\[)",),
    "editorial": (r"^\s*editorial\s*(?::|[-\u2013\u2014]\s)",),
    "retraction_notice_or_retracted": (
        r"^\s*(?:retraction|retracted(?:\s+article)?|withdrawn)\s*(?::|notice\b|note\b|of\b|\[)",),
}
INELIGIBLE_TITLE_PATTERNS = {
    kind: tuple(re.compile(r, re.IGNORECASE) for r in rx) for kind, rx in INELIGIBLE_TITLE_CUES.items()
}
RULE = (
    "excluded_publication_type when the publication type names a protocol, correction/erratum, editorial or "
    "retraction notice/retracted publication, or (fallback) the title carries an explicit cue ('study protocol', "
    "'Protocol for a ... trial', 'Correction:', 'Editorial:', 'Retraction:'); excluded rows keep their other "
    "labels but never enter a review queue"
)


def eligibility(publication_types, title):
    """``publication_types``: iterable of type strings (any case). Returns a dict."""
    tokens = sorted({str(t).strip().lower() for t in publication_types or [] if str(t).strip()})
    basis = []
    for kind, names in INELIGIBLE_PUBTYPES.items():
        for tok in tokens:
            if tok in names:
                basis.append({"kind": kind, "source": "publication_type", "term": tok})
    for kind, patterns in INELIGIBLE_TITLE_PATTERNS.items():
        for pattern in patterns:
            m = pattern.search(title or "")
            if m:
                basis.append({"kind": kind, "source": "title", "term": m.group(0).strip()})
                break
    kinds = sorted({b["kind"] for b in basis})
    return {
        "status": "excluded_publication_type" if kinds else "eligible",
        "excluded_kinds": kinds,
        "basis": sorted(basis, key=lambda b: (b["kind"], b["source"], b["term"])),
    }
