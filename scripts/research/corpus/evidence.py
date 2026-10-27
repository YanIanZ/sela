"""Parse the evidence-to-feature tables (docs 07 and 10) and doc 10's year claim."""
from __future__ import annotations

import re

_RID = re.compile(r"\bR(\d{3})\b")
_RANGE = re.compile(r"\bR(\d{3})\s*[–-]\s*R(\d{3})\b")


def expand_rids(text):
    """'R098; R019' -> ['R019', 'R098']; 'R098–R100' -> ['R098', 'R099', 'R100']."""
    out = set()
    for m in _RANGE.finditer(text):
        a, b = int(m.group(1)), int(m.group(2))
        if a <= b and b - a <= 100:
            out.update(f"R{i:03d}" for i in range(a, b + 1))
    out.update("R" + m.group(1) for m in _RID.finditer(text))
    return sorted(out)


def _cells(line):
    return [c.strip() for c in line.strip().strip("|").split("|")]


def markdown_tables(text):
    """Yield (heading, header_cells, rows) for every pipe table in ``text``."""
    heading = None
    lines = text.splitlines()
    i = 0
    while i < len(lines):
        line = lines[i]
        if line.startswith("#"):
            heading = line.lstrip("#").strip()
        if line.strip().startswith("|") and i + 1 < len(lines) and re.match(r"^\s*\|[\s:|-]+\|\s*$", lines[i + 1]):
            header = _cells(line)
            rows = []
            j = i + 2
            while j < len(lines) and lines[j].strip().startswith("|"):
                rows.append(_cells(lines[j]))
                j += 1
            yield heading, header, rows
            i = j
            continue
        i += 1


def _strip_md(text):
    return re.sub(r"\*\*|`", "", text).strip()


def parse_feature_tables(doc_name, text, rid_column, feature_column):
    """Rows of every table whose header has both named columns."""
    entries = []
    for heading, header, rows in markdown_tables(text):
        if rid_column not in header or feature_column not in header:
            continue
        ri, fi = header.index(rid_column), header.index(feature_column)
        for n, row in enumerate(rows, start=1):
            if len(row) != len(header):
                continue
            entries.append({
                "source_doc": doc_name, "section": heading, "row": n,
                "feature": _strip_md(row[fi]),
                "r_ids": expand_rids(row[ri]),
                "columns": {h: _strip_md(c) for h, c in zip(header, row)},
            })
    return entries


def build_evidence_map(doc07_name, doc07_text, doc10_name, doc10_text, corpus_ids, status_by_id):
    entries = (parse_feature_tables(doc07_name, doc07_text, "Bukti pada register", "Keputusan Sela")
               + parse_feature_tables(doc10_name, doc10_text, "Registry starting points", "Feature or claim"))
    corpus_ids = set(corpus_ids)
    missing = set()
    for e in entries:
        e["missing_r_ids"] = [r for r in e["r_ids"] if r not in corpus_ids]
        e["r_id_screening_status"] = {r: status_by_id.get(r) for r in e["r_ids"] if r in corpus_ids}
        e["relies_on_non_included"] = sorted(
            r for r, s in e["r_id_screening_status"].items() if s != "included_title_abstract_v0")
        missing.update(e["missing_r_ids"])
    batch = []
    m = re.search(r"\*\*First detailed appraisal batch:\*\*\s*(.+)", doc10_text)
    if m:
        batch = expand_rids(m.group(1).split(".")[0])
    features_by_rid = {}
    for e in entries:
        for r in e["r_ids"]:
            features_by_rid.setdefault(r, set()).add(e["feature"])
    return {
        "entries": entries,
        "missing_r_ids": sorted(missing),
        "all_r_ids_present": not missing,
        "first_appraisal_batch": {"source_doc": doc10_name, "r_ids": batch,
                                  "missing_r_ids": [r for r in batch if r not in corpus_ids]},
        "features_by_r_id": {r: sorted(v) for r, v in sorted(features_by_rid.items())},
        "note": ("Design hypotheses copied from our own docs; mapping a paper to a feature is not "
                 "causal validation of Sela."),
    }


def annotate_v3(emap, records_by_id):
    """v3: show which feature rows lean on records with a design-label caution, a
    needs_check date or adult-only / unverified-age populations (no row changes)."""
    for e in emap["entries"]:
        recs = [records_by_id[r] for r in e["r_ids"] if r in records_by_id]
        e["relies_on_study_type_caution"] = sorted(r["id"] for r in recs if r.get("study_type_caution"))
        e["relies_on_needs_check_documented"] = sorted(r["id"] for r in recs if r.get("needs_check_evidence"))
        e["r_id_age_band"] = {r["id"]: r["age_band"]["summary"] for r in recs}
        e["r_id_reviewer_date_override"] = {r["id"]: r["publication_date_reviewer_override"]["date"]
                                            for r in recs if r.get("publication_date_reviewer_override")}
    emap["v3_note"] = ("age bands and cautions are machine-derived and unreviewed; a row backed only by adult "
                       "evidence says nothing about ages 15-17")
    return emap


_YEAR_CLAIM = re.compile(r"\|\s*First-publication year\s*\|\s*([^|]+)\|")


def parse_year_claim(doc10_text):
    """'2023: 2; 2024: 3; 2025: 86; 2026: 9' -> {'2023': 2, ...} (or None)."""
    m = _YEAR_CLAIM.search(doc10_text)
    if not m:
        return None, None
    raw = m.group(1).strip()
    counts = {y: int(n) for y, n in re.findall(r"(\d{4})\s*:\s*(\d+)", raw)}
    return counts, raw


def compare_distribution(claimed, observed):
    years = sorted(set(claimed) | set(observed))
    diff = {y: observed.get(y, 0) - claimed.get(y, 0) for y in years if observed.get(y, 0) != claimed.get(y, 0)}
    return {"observed": dict(sorted(observed.items())), "matches_claim": not diff, "difference_observed_minus_claimed": diff}
