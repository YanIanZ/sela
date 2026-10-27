#!/usr/bin/env python3
"""Leak check: no committed file may carry a verbatim run of 8 or more tokens taken from
an abstract or a full text that the project may not redistribute.

Two levels:

  local (exact; needs the gitignored caches; REQUIRED before every commit that touches
  scripts/research, tests/research, data/research or docs/research):
      python3 scripts/research/leak_check.py --require-cache
    Every cached register abstract, every cached CC full text whose licence does not allow
    redistribution, every out-of-register source abstract, and every abstract of every
    gap-search run (and of the pool audit sample) is compared with the committed files.
    It also checks that the committed fingerprint file (below) is current.

  CI (no cache; run by corpus_pipeline.py verify):
      python3 scripts/research/leak_check.py --fingerprints-only
    data/research/corpus/leak_fingerprints.json holds, for every protected register or
    doc-10 source abstract, the truncated SHA-256 of each 8-token run (no text). CI checks
    the agent-written files against it. It does not cover full texts or gap-search
    abstracts: those need the local check.

Protected text: an abstract or full text of a register record whose licence does not allow
redistribution (CC BY-NC, BY-NC-ND, publisher, TDM-only, none), an out-of-register source
abstract, and every gap-search abstract (licence unknown). Committed files: agent-written
files (docs, specs, evidence and claim maps, manifests, tests, fixtures, scripts) and
machine outputs (records, knowledge base, snapshots, gap-search candidates and screening).

Exempt: bibliographic titles, journal names and author lists (register, sources, pool, every committed
candidate), locator section headings, identifiers (URLs, DOIs, PMCIDs), runs with fewer than 5 word
tokens (statistics are facts, not text), the few overlaps a person reviewed as coincidental
(REVIEWED_COINCIDENCES, by file and run hash, each with its reason; agent-judged, a person should confirm), and runs that also occur in a text the
project may redistribute (a CC BY / BY-SA / CC0 register abstract or full text, which the
pipeline may quote with its locator). A run of 8+ tokens from a CC BY abstract inside an
agent-written file is reported separately (allowed with the citation, but an abstract is
never committed whole). Tokens are case-folded word characters. Stdlib only.
"""
from __future__ import annotations

import argparse
import collections
import hashlib
import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from corpus import extract as X  # noqa: E402
from corpus import fetch as F  # noqa: E402
from corpus import gzstore as S  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]
RUN_TOKENS = 8
MIN_WORD_TOKENS = 5  # a run with fewer word tokens (statistics such as "OR 0.34, 95% CI 0.13-0.88") is a fact, not text
CORPUS = Path("data/research/corpus")
FULLTEXT_CACHE = CORPUS / ".cache"
FINGERPRINTS = CORPUS / "leak_fingerprints.json"
FINGERPRINT_VERSION = "sela-leak-fingerprints-v1"
HASH_SALT = "sela-leak-v1|"
HASH_CHARS = 12  # 48 bits: with ~10^4 stored runs and ~10^6 checked runs a chance collision is ~10^-5
TOKEN = re.compile(r"\w+", re.UNICODE)
# identifiers are metadata, not text: URLs, DOIs and PMC ids are removed before tokenising
IDENTIFIER = re.compile(r"https?://\S+|\bdoi:\s*\S+|\b10\.\d{4,9}/[^\s\"'<>|]+|\bPMC\d+\b", re.I)

# Files written by people or agents (prose, specs, code, synthetic fixtures). Paths are globs from the root.
AGENT_WRITTEN = (
    "docs/research/**/*.md", "docs/research/**/*.json", "docs/research/**/*.bib",
    "data/research/README.md", "data/research/corpus/*_spec.json", "data/research/corpus/evidence_map_v3.json",
    "data/research/corpus/claim_check_v3.json", "data/research/corpus/evidence_map.json",
    "data/research/corpus/eval_fixtures/*.json", "data/research/register_inputs/**/*",
    "data/research/screening_history/*.json", "data/research/sela_*manifest.json",
    "data/research/v3_gap_search/**/manifest.json",
    "tests/research/**/*.py", "tests/research/fixtures/**/*", "scripts/research/**/*.py",
)
# Machine outputs that legitimately hold titles and (CC BY only) short snippets with locators. A ``.gz`` file
# (the gap-search candidates, corpus.gzstore) is checked as its decompressed text.
MACHINE_OUTPUTS = (
    "data/research/corpus/records.jsonl", "data/research/corpus/knowledge_base.jsonl",
    "data/research/corpus/manifest.json", "data/research/corpus/snapshots/**/*",
    "data/research/v3_gap_search/**/*.jsonl", "data/research/v3_gap_search/**/*.jsonl.gz",
    "data/research/v3_gap_search/**/*.csv",
    "data/research/sela_*.csv", "data/research/sela_*.jsonl",
)
NEVER_SCANNED = {FINGERPRINTS.as_posix()}  # hashes only
REDISTRIBUTABLE = re.compile(r"^(?:CC-BY(?:-SA)?(?:-\d(?:\.\d)?)?|CC0(?:-1\.0)?)$", re.I)


def tokens(text):
    return [t.casefold() for t in TOKEN.findall(IDENTIFIER.sub(" ", text or ""))]


_HAS_LETTER = re.compile(r"[^\W\d_]")


def shingles(toks, n=RUN_TOKENS):
    return {" ".join(toks[i:i + n]) for i in range(len(toks) - n + 1)}


def is_text_run(shingle):
    """A run counts as text only when at least MIN_WORD_TOKENS of its tokens contain a letter."""
    return sum(1 for t in shingle.split(" ") if _HAS_LETTER.search(t)) >= MIN_WORD_TOKENS


# Overlaps judged coincidental, keyed by file and the SHA-256 of the 8-token run (no text). Each entry needs its
# reason; anything else fails the check. The one entry was judged by the AI research agent on 2026-10-26 (unreviewed):
# a person should confirm it.
REVIEWED_COINCIDENCES = {
    ("tests/research/fixtures/corpus/fulltext_single.xml",
     "db5f1eefd4c0e254bb52f0e66195b118565bcde12f847671d508691b7077797a"): (
        "synthetic fixture sentence written for the pipeline tests in commit 884c839, before any v3 search; a generic "
        "methods phrase (a cross-sectional survey of 312 people) that coincides with an unrelated gap-search abstract "
        "on exercise in later life; the fixture is pinned by the legacy golden hashes in test_corpus_v3.py"),
}


def run_sha256(shingle):
    return hashlib.sha256(shingle.encode("utf-8")).hexdigest()


def shingle_hash(shingle):
    return hashlib.sha256((HASH_SALT + shingle).encode("utf-8")).hexdigest()[:HASH_CHARS]


@dataclass
class Source:
    """One text. ``scope`` is 'all' (protected: no committed file may carry it) or 'agent_written'
    (redistributable: only reported when it appears in an agent-written file)."""
    source_id: str
    kind: str  # abstract | fulltext | gap_search_abstract | pool_abstract
    licence: str | None
    scope: str
    text: str


@dataclass
class Leak:
    source_id: str
    kind: str
    licence: str | None
    path: str
    run_tokens: int
    example: str = field(repr=False)  # printed on the console only, never written to a file


def _text_files(root, patterns, exclude=()):
    root = Path(root)
    out = {}
    for pat in patterns:
        for p in sorted(root.glob(pat)):
            rel = p.relative_to(root).as_posix()
            if (not p.is_file() or "/.cache/" in f"/{rel}" or "__pycache__" in rel or rel in exclude
                    or rel in NEVER_SCANNED):
                continue
            try:
                out[rel] = S.read_text(p) if p.name.endswith(S.SUFFIX) else p.read_text(encoding="utf-8")
            except UnicodeDecodeError:  # binary (an unreadable .gz raises: it must not go unchecked)
                continue
    return out


def committed_files(root, machine_outputs=True):
    """{'agent_written': {path: text}, 'machine_outputs': {path: text}}"""
    agent = _text_files(root, AGENT_WRITTEN)
    machine = _text_files(root, MACHINE_OUTPUTS, exclude=set(agent)) if machine_outputs else {}
    return {"agent_written": agent, "machine_outputs": machine}


def _records(root):
    p = Path(root) / CORPUS / "records.jsonl"
    if not p.exists():
        return {}
    return {r["id"]: r for r in map(json.loads, p.read_text(encoding="utf-8").splitlines())}


def _redistributable(rec):
    return bool(rec and rec.get("redistribution_allowed") and REDISTRIBUTABLE.match(str(rec.get("license") or "")))


def register_sources(root):
    """Register and doc-10 abstracts and cached CC full texts, split by licence:
    (protected, redistributable, notes)."""
    import v3_evidence as VE
    root = Path(root)
    recs = _records(root)
    protected, free, notes = [], [], []
    if not (root / VE.CACHE).exists():
        notes.append(f"skipped register abstracts: no local cache at {VE.CACHE}")
    else:
        for sid, a in sorted(VE.load_abstracts(root, every_register_record=True).items()):
            if not a.get("abstract"):
                continue
            rec = recs.get(sid)
            if _redistributable(rec):
                free.append(Source(sid, "abstract", rec.get("license"), "agent_written", a["abstract"]))
            else:
                protected.append(Source(sid, "abstract", (rec or {}).get("license"), "all", a["abstract"]))
    cache = F.Cache(root / FULLTEXT_CACHE)
    if not (root / FULLTEXT_CACHE / "fulltext").exists():
        notes.append(f"skipped full texts: no local cache at {FULLTEXT_CACHE / 'fulltext'}")
        return protected, free, notes
    for rid, r in sorted(recs.items()):
        if not r.get("pmcid"):
            continue
        key, _ = F.fulltext_request(r["pmcid"])
        xml = cache.get_text("fulltext", key)
        if not xml:
            continue
        text = "\n".join(p["text"] for p in X.parse_fulltext(xml)["paragraphs"])
        if _redistributable(r):
            free.append(Source(rid, "fulltext", r.get("license"), "agent_written", text))
        else:
            protected.append(Source(rid, "fulltext", r.get("license"), "all", text))
    return protected, free, notes


def gap_search_sources(root):
    """Every cached abstract of every committed gap-search run (licence unknown), plus the pool audit
    abstracts. Returns (sources, notes)."""
    import v3_gap_search as G
    root = Path(root)
    out, notes = [], []
    for run in G.RUNS.values():
        name = run.dir_name(G.DEFAULT_FREEZE)
        cdir = G.cache_dir(root, G.DEFAULT_FREEZE, run)
        if not (root / G.OUT_BASE / name / "manifest.json").exists():
            continue
        if not cdir.exists():
            notes.append(f"skipped gap-search run {run.run_version}: no local cache at {cdir.relative_to(root)}")
            continue
        cache = F.Cache(cdir)
        _c, abstracts, _s, missing, _p = G.build_candidates(root, G.DEFAULT_FREEZE, cache, run)
        if missing:
            notes.append(f"skipped part of gap-search run {run.run_version}: {len(missing)} cache entries missing")
        for cid, text in sorted(abstracts.items()):
            out.append(Source(f"{run.run_version}:{cid}", "gap_search_abstract", None, "all", text))
        sample = G.pool_sample(root, run)
        for sl, text in sorted(G.pool_abstracts(cache, sample, G.pool_abstract_batches(root, run)).items()):
            out.append(Source(f"{run.run_version}:pool:{sl}", "pool_abstract", None, "all", text))
    return out, notes


def metadata_shingles(root):
    """Shingles that are bibliographic metadata or locators and may be committed: every register and
    source title, every pool title, every title, journal name and author list (group authors included)
    in a committed gap-search candidates file, the pool or the register, and every locator section
    heading in records.jsonl. Needs no cache except for the source titles."""
    import v3_evidence as VE
    root = Path(root)
    recs = _records(root)
    texts = [r.get("title") or "" for r in recs.values()] + [r.get("journal") or "" for r in recs.values()]
    texts += [", ".join(r.get("authors") or []) for r in recs.values()]
    for r in recs.values():
        for e in r.get("evidence") or []:
            texts.append((e.get("locator") or {}).get("section") or "")
        for c in (r.get("country_evidence_fulltext") or {}).get("candidates") or []:
            texts.append((c.get("first_locator") or {}).get("section") or "")
    if (root / VE.CACHE).exists():
        texts += [a.get("title") or "" for a in VE.load_abstracts(root, every_register_record=True).values()]
    for p in [root / "data/research/sela_research_candidates_7580.jsonl"] + sorted(
            (root / "data/research/v3_gap_search").glob("*/candidates.jsonl.gz")):
        if p.exists():
            for x in S.read_text(p).splitlines():
                c = json.loads(x)
                authors = c.get("authors") or ""
                texts += [c.get("title") or "", c.get("journal") or "",
                          ", ".join(authors) if isinstance(authors, list) else str(authors)]
    out = set()
    for t in texts:
        out |= shingles(tokens(t))
    return out


def find_leaks(sources, files, allowed=frozenset(), n=RUN_TOKENS):
    """[Leak] for every (source, committed file) pair sharing a run of n or more tokens that is not
    an allowed shingle. A protected source (scope 'all') is checked against every file; a
    redistributable one only against agent-written files."""
    index = collections.defaultdict(set)  # shingle -> {(group, path)}
    for group, files_ in files.items():
        for path, text in files_.items():
            for s in shingles(tokens(text), n):
                if s not in allowed and is_text_run(s) and (path, run_sha256(s)) not in REVIEWED_COINCIDENCES:
                    index[s].add((group, path))
    leaks = []
    for src in sources:
        toks = tokens(src.text)
        hits = collections.defaultdict(list)  # path -> [start positions in the source]
        for i in range(len(toks) - n + 1):
            for group, path in index.get(" ".join(toks[i:i + n]), ()):
                if src.scope == "all" or group == "agent_written":
                    hits[path].append(i)
        for path, starts in sorted(hits.items()):
            best, best_start, run_start, prev = 0, starts[0], starts[0], starts[0]
            for st in starts[1:] + [None]:  # merge overlapping windows into runs; report the longest
                if st is not None and st == prev + 1:
                    prev = st
                    continue
                if prev - run_start + n > best:
                    best, best_start = prev - run_start + n, run_start
                if st is not None:
                    run_start = prev = st
            leaks.append(Leak(src.source_id, src.kind, src.licence, path, best,
                              " ".join(toks[best_start:best_start + best])))
    return leaks


# --- committed fingerprints (CI) ---------------------------------------------------------------

def build_fingerprints(root):
    """The fingerprint document: for every protected register/doc-10 abstract, hashes of its 8-token runs
    that are not exempt (titles, locators, identifiers, runs also in a redistributable text). No text."""
    root = Path(root)
    protected, free, notes = register_sources(root)
    if any(n.startswith("skipped register abstracts") for n in notes):
        raise SystemExit("fingerprints: the register abstract cache is missing (run v3_evidence.py fetch-abstracts)")
    exempt = metadata_shingles(root)
    for src in free:
        exempt |= shingles(tokens(src.text))
    sources = {}
    for src in protected:
        if src.kind != "abstract":
            continue
        kept = sorted({shingle_hash(s) for s in shingles(tokens(src.text)) if s not in exempt and is_text_run(s)})
        sources[src.source_id] = {"kind": "abstract", "licence": src.licence,
                                  "text_sha256": hashlib.sha256(src.text.encode("utf-8")).hexdigest(),
                                  "runs": len(kept), "run_hashes": kept}
    return {
        "version": FINGERPRINT_VERSION,
        "purpose": ("lets CI check committed agent-written files for verbatim runs from protected abstracts without "
                    "the local cache; hashes only, no text"),
        "rule": (f"every {RUN_TOKENS}-token run (case-folded \\w+ tokens, URLs/DOIs/PMCIDs removed) of each protected "
                 f"abstract with at least {MIN_WORD_TOKENS} word tokens, minus exempt runs (titles, journal names, "
                 "author lists, locator headings, runs that also occur in a CC BY / BY-SA / CC0 register text); "
                 f"sha256('{HASH_SALT}' + run) truncated to {HASH_CHARS} hex characters"),
        "protected": ("register abstracts whose licence does not allow redistribution (CC BY-NC, BY-NC-ND, publisher, "
                      "TDM-only, none) and the out-of-register abstracts cited in doc 10"),
        "not_covered_in_ci": ("full texts and gap-search abstracts: run `python3 scripts/research/leak_check.py "
                              "--require-cache` locally before committing"),
        "run_tokens": RUN_TOKENS,
        "sources": sources,
    }


def fingerprint_bytes(doc):
    return (json.dumps(doc, ensure_ascii=False, sort_keys=True, indent=1) + "\n").encode("utf-8")


def check_fingerprints(root):
    """CI check: problems for committed agent-written files that contain a fingerprinted run."""
    root = Path(root)
    p = root / FINGERPRINTS
    if not p.exists():
        return [f"{FINGERPRINTS} missing (run `leak_check.py --write-fingerprints` where the cache exists)"]
    doc = json.loads(p.read_text(encoding="utf-8"))
    if doc.get("version") != FINGERPRINT_VERSION or doc.get("run_tokens") != RUN_TOKENS:
        return [f"{FINGERPRINTS}: unexpected version or run length"]
    owner = {}
    for sid, info in doc["sources"].items():
        for h in info["run_hashes"]:
            owner.setdefault(h, sid)
    problems = []
    for path, text in committed_files(root, machine_outputs=False)["agent_written"].items():
        toks = tokens(text)
        found = collections.Counter()
        for i in range(len(toks) - RUN_TOKENS + 1):
            sid = owner.get(shingle_hash(" ".join(toks[i:i + RUN_TOKENS])))
            if sid:
                found[sid] += 1
        for sid, n in sorted(found.items()):
            problems.append(f"{path}: {n} run(s) of {RUN_TOKENS}+ tokens from the protected abstract of {sid} "
                            f"({doc['sources'][sid]['licence'] or 'no licence'}); paraphrase it")
    return problems


def run_check(root, include_gap_search=True):
    """(leaks, cc_by_abstract_runs, summary) of the exact local check."""
    root = Path(root)
    files = committed_files(root)
    protected, free, notes = register_sources(root)
    if include_gap_search:
        gsrc, gnotes = gap_search_sources(root)
        protected += gsrc
        notes += gnotes
    meta = metadata_shingles(root)
    redistributable = set()
    for src in free:
        redistributable |= shingles(tokens(src.text))
    leaks = find_leaks(protected, files, meta | redistributable)
    cc_by = find_leaks([s for s in free if s.kind == "abstract"], {"agent_written": files["agent_written"]}, meta)
    counts = collections.Counter(s.kind + (":protected" if s.scope == "all" else ":redistributable")
                                 for s in protected + free)
    summary = {"sources_checked": dict(sorted(counts.items())),
               "committed_files": {k: len(v) for k, v in files.items()},
               "run_tokens": RUN_TOKENS, "leaks": len(leaks), "cc_by_abstract_runs": len(cc_by), "notes": notes}
    return leaks, cc_by, summary


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--root", type=Path, default=ROOT)
    p.add_argument("--require-cache", action="store_true",
                   help="exit 1 when a cache is missing or the committed fingerprints are stale (before committing)")
    p.add_argument("--no-gap-search", action="store_true", help="skip the gap-search abstracts (faster)")
    p.add_argument("--fingerprints-only", action="store_true", help="the CI check (no cache needed)")
    p.add_argument("--write-fingerprints", action="store_true", help=f"rewrite {FINGERPRINTS} from the cache")
    a = p.parse_args(argv)
    if a.write_fingerprints:
        data = fingerprint_bytes(build_fingerprints(a.root))
        (a.root / FINGERPRINTS).write_bytes(data)
        print(f"wrote {FINGERPRINTS} ({len(data)} bytes)")
    fp_problems = check_fingerprints(a.root)
    for x in fp_problems:
        print("LEAK (fingerprint):", x)
    if a.fingerprints_only:
        print(f"leak check (committed fingerprints): {len(fp_problems)} problem(s)")
        return 1 if fp_problems else 0
    leaks, cc_by, summary = run_check(a.root, include_gap_search=not a.no_gap_search)
    for lk in leaks:
        print(f"LEAK: {lk.path}: {lk.run_tokens}-token run from {lk.kind} {lk.source_id} "
              f"(licence {lk.licence or 'unknown'}): \"{lk.example}\"")
    for lk in cc_by:
        print(f"CC-BY RUN (allowed with the citation; paraphrase where possible): {lk.path}: {lk.run_tokens}-token "
              f"run from abstract {lk.source_id} ({lk.licence}): \"{lk.example}\"")
    stale = False
    if not any(n.startswith("skipped register abstracts") for n in summary["notes"]) and \
            (a.root / FINGERPRINTS).exists():
        stale = fingerprint_bytes(build_fingerprints(a.root)) != (a.root / FINGERPRINTS).read_bytes()
        if stale:
            print(f"STALE: {FINGERPRINTS} does not match the cache; run --write-fingerprints and commit it")
    for n in summary["notes"]:
        print("NOTE:", n)
    print(json.dumps({k: v for k, v in summary.items() if k != "notes"}, sort_keys=True))
    print(f"leak check: {len(leaks)} leak(s), {len(fp_problems)} fingerprint hit(s)")
    if a.require_cache and (stale or any(n.startswith("skipped") for n in summary["notes"])):
        print("leak check: run this where every cache exists, with current fingerprints")
        return 1
    return 1 if (leaks or fp_problems) else 0


if __name__ == "__main__":
    sys.exit(main())
