"""Identifier normalisation and de-duplication (pure functions)."""
from __future__ import annotations

import re
import unicodedata

_DOI_PREFIX = re.compile(r"^(?:https?://(?:dx\.)?doi\.org/|doi:\s*)", re.IGNORECASE)
_DOI_SHAPE = re.compile(r"^10\.\d{4,9}/\S+$")
_PUBMED_URL = re.compile(r"pubmed\.ncbi\.nlm\.nih\.gov/(\d+)")
_PMCID = re.compile(r"^(?:pmc)?\s*(\d+)$", re.IGNORECASE)


def normalize_doi(value):
    """Lowercase, trim, strip resolver prefixes and trailing punctuation.

    Returns None for empty input. DOIs are case-insensitive (DOI Handbook),
    so lowercase is a safe canonical form for matching.
    """
    if value is None:
        return None
    s = unicodedata.normalize("NFKC", str(value)).strip()
    previous = None
    while previous != s:  # strip repeated prefixes such as "doi: https://doi.org/..."
        previous = s
        s = _DOI_PREFIX.sub("", s).strip()
    s = s.lower().split("?")[0].split("#")[0].strip()
    s = s.rstrip(".,; \t")
    while s.endswith(")") and s.count("(") < s.count(")"):
        s = s[:-1].rstrip(".,; \t")
    return s or None


def is_valid_doi(value):
    return bool(value) and bool(_DOI_SHAPE.match(value))


def normalize_pmid(value):
    if value is None:
        return None
    s = str(value).strip()
    m = _PUBMED_URL.search(s)
    if m:
        return m.group(1)
    return s if s.isdigit() and s != "0" else None


def pmid_from_url(url):
    m = _PUBMED_URL.search(url or "")
    return m.group(1) if m else None


def normalize_pmcid(value):
    if value is None:
        return None
    m = _PMCID.match(str(value).strip())
    return "PMC" + m.group(1) if m else None


def normalize_title(value):
    """Accent-folded, casefolded alphanumerics only (for duplicate matching)."""
    s = unicodedata.normalize("NFKD", str(value or ""))
    s = "".join(c for c in s if not unicodedata.combining(c)).casefold()
    return re.sub(r"[^a-z0-9]+", "", s)


def find_duplicates(rows):
    """Group rows that share a normalised DOI, PMID, PMCID or title+year.

    ``rows`` are dicts with ``id`` and optional ``doi``, ``pmid``, ``pmcid``,
    ``title`` and ``year``. Returns a sorted list of
    ``{"key_type", "key", "ids"}`` for every key held by more than one id.
    """
    index = {}
    for row in rows:
        keys = []
        doi = normalize_doi(row.get("doi"))
        if doi:
            keys.append(("doi", doi))
        pmid = normalize_pmid(row.get("pmid"))
        if pmid:
            keys.append(("pmid", pmid))
        pmcid = normalize_pmcid(row.get("pmcid"))
        if pmcid:
            keys.append(("pmcid", pmcid))
        title = normalize_title(row.get("title"))
        if title:
            keys.append(("title_year", title + "|" + str(row.get("year") or "")))
        for key in keys:
            index.setdefault(key, set()).add(row["id"])
    return [
        {"key_type": k[0], "key": k[1], "ids": sorted(ids)}
        for k, ids in sorted(index.items())
        if len(ids) > 1
    ]
