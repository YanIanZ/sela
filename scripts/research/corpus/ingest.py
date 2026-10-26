"""Step 1: ingest the 100-publication register and cross-check its copies."""
from __future__ import annotations

import hashlib
import json
import re

from . import ids

_RID = re.compile(r"^R\d{3}$")


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def sha256_file(path):
    return sha256_bytes(path.read_bytes())


def load_register_json(raw_bytes):
    """Parse research_100.json; validate ids; return (header, papers)."""
    obj = json.loads(raw_bytes.decode("utf-8"))
    papers = obj.get("papers")
    if not isinstance(papers, list) or not papers:
        raise ValueError("research_100.json: 'papers' list missing or empty")
    seen = set()
    for i, paper in enumerate(papers, start=1):
        rid = paper.get("id")
        if not isinstance(rid, str) or not _RID.match(rid):
            raise ValueError(f"research_100.json: paper {i} has invalid id {rid!r}")
        if rid in seen:
            raise ValueError(f"research_100.json: duplicate id {rid}")
        if not paper.get("title"):
            raise ValueError(f"research_100.json: {rid} has no title")
        seen.add(rid)
    if isinstance(obj.get("count"), int) and obj["count"] != len(papers):
        raise ValueError(f"research_100.json: count {obj['count']} != {len(papers)} papers")
    header = {k: v for k, v in obj.items() if k != "papers"}
    return header, papers


def split_authors(value):
    """'Rackoff GN, Zhang ZZ, Newman MG.' -> (['Rackoff GN', ...], truncated)."""
    text = " ".join(str(value or "").split()).rstrip(".")
    parts = [p.strip() for p in re.split(r",|\band\b", text) if p.strip()]
    truncated = False
    out = []
    for p in parts:
        if p.lower().rstrip(".") in {"et al", "et al"}:
            truncated = True
            continue
        out.append(p)
    return out, truncated


_BIB_ENTRY = re.compile(r"@(\w+)\{([^,\s]+),\s*\n(.*?)\n\}", re.DOTALL)
_BIB_FIELD = re.compile(r"^\s*(\w+)\s*=\s*\{(.*)\},?\s*$")


def parse_bib(text):
    """Minimal parser for the register's one-field-per-line BibTeX."""
    entries = {}
    for m in _BIB_ENTRY.finditer(text):
        entry_type, key, body = m.group(1), m.group(2), m.group(3)
        fields = {}
        for line in body.splitlines():
            fm = _BIB_FIELD.match(line)
            if fm:
                fields[fm.group(1).lower()] = fm.group(2)
        rid = key.split("_")[-1]
        entries[rid] = {"key": key, "entry_type": entry_type.lower(), "fields": fields}
    return entries


_MD_SECTION = re.compile(r"^## (R\d{3}) — (.+?)\s*$", re.MULTILINE)
_MD_META = re.compile(
    r"^(?P<authors>.*?) \*\*(?P<journal>.+?)\*\*\. Publikasi pertama: (?P<date>[0-9-]+)\. "
    r"Tema: (?P<theme>[A-Z_]+)\. Jenis: (?P<jenis>.+?)\s*$",
    re.MULTILINE,
)
_MD_LINKS = re.compile(r"^\[DOI\]\((?P<doi>[^)]+)\) · \[Sumber/indeks\]\((?P<index>[^)]+)\)", re.MULTILINE)
_MD_NOTE = re.compile(r"^\*\*Catatan:\*\* (?P<note>.+?)\s*$", re.MULTILINE)
_MD_LIMIT = re.compile(r"^\*\*Batas:\*\* (?P<limit>.+?)\s*$", re.MULTILINE)


def parse_register_md(text):
    """Parse 09_RESEARCH_REGISTER.md annotations keyed by R-id."""
    out = {}
    matches = list(_MD_SECTION.finditer(text))
    for i, m in enumerate(matches):
        rid = m.group(1)
        body = text[m.end(): matches[i + 1].start() if i + 1 < len(matches) else len(text)]
        entry = {"title": m.group(2)}
        meta = _MD_META.search(body)
        if meta:
            entry.update({k: meta.group(k) for k in ("authors", "journal", "date", "theme", "jenis")})
        links = _MD_LINKS.search(body)
        if links:
            entry["doi_url"] = links.group("doi")
            entry["index_url"] = links.group("index")
        note = _MD_NOTE.search(body)
        if note:
            entry["note"] = note.group("note")
        limit = _MD_LIMIT.search(body)
        if limit:
            entry["limitation"] = limit.group("limit")
        out[rid] = entry
    return out


def _title_key(value):
    return ids.normalize_title(value)


def cross_check(papers, bib, register_md):
    """Compare research_100.json with the .bib and register .md copies.

    Returns a sorted list of issue dicts; an empty list means all copies agree
    on ids, DOIs, titles, years, dates, themes and annotations.
    """
    issues = []

    def issue(rid, source, field, expected, found):
        issues.append({"id": rid, "source": source, "field": field,
                       "json_value": expected, "found": found})

    json_ids = {p["id"] for p in papers}
    for rid in sorted(set(bib) - json_ids):
        issue(rid, "research_100.bib", "id", None, "entry without JSON record")
    for rid in sorted(set(register_md) - json_ids):
        issue(rid, "09_RESEARCH_REGISTER.md", "id", None, "section without JSON record")
    for p in papers:
        rid = p["id"]
        b = bib.get(rid)
        if b is None:
            issue(rid, "research_100.bib", "id", rid, None)
        else:
            f = b["fields"]
            if ids.normalize_doi(f.get("doi")) != ids.normalize_doi(p.get("doi")):
                issue(rid, "research_100.bib", "doi", p.get("doi"), f.get("doi"))
            if _title_key(f.get("title")) != _title_key(p.get("title")):
                issue(rid, "research_100.bib", "title", p.get("title"), f.get("title"))
            if str(f.get("year") or "") != str(p.get("publication_year") or ""):
                issue(rid, "research_100.bib", "year", p.get("publication_year"), f.get("year"))
            author = f.get("author") or ""
            if "," in author and " and " not in author and len(split_authors(p.get("authors"))[0]) > 1:
                issue(rid, "research_100.bib", "author_format",
                      "BibTeX 'and'-separated list", "comma-separated (parsed by BibTeX as one name)")
        r = register_md.get(rid)
        if r is None:
            issue(rid, "09_RESEARCH_REGISTER.md", "id", rid, None)
            continue
        checks = [
            ("title", p.get("title"), r.get("title"), _title_key),
            ("first_publication_date", p.get("first_publication_date"), r.get("date"), str),
            ("theme", p.get("theme"), r.get("theme"), str),
            ("doi", p.get("doi"), r.get("doi_url"), ids.normalize_doi),
            ("index_url", p.get("index_url"), r.get("index_url"), str),
            ("abstract_based_note_id", p.get("abstract_based_note_id"), r.get("note"), str),
        ]
        for field, expected, found, norm in checks:
            if found is None or norm(expected) != norm(found):
                issue(rid, "09_RESEARCH_REGISTER.md", field, expected, found)
        limitation = r.get("limitation") or ""
        if not limitation.startswith(p.get("transfer_limitation_id") or "\0"):
            issue(rid, "09_RESEARCH_REGISTER.md", "transfer_limitation_id",
                  p.get("transfer_limitation_id"), limitation)
    return sorted(issues, key=lambda x: (x["id"], x["source"], x["field"]))
