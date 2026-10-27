"""Offline tests for the v3 gap search, its machine title/abstract screening and the
false-negative audit sample (research audit R-3, R-4; doc 14 criterion 5).

Synthetic cache fixtures only (no network). The committed-output tests run in CI
without the cache and check what can be checked from the committed files.
"""
import csv
import gzip
import hashlib
import io
import json
import shutil
import struct
import sys
import tempfile
import unittest
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
sys.path.insert(0, str(REPO / "scripts" / "research"))

import leak_check as LC  # noqa: E402
import v3_gap_search as G  # noqa: E402
from corpus import fetch as F  # noqa: E402
from corpus import gzstore as S  # noqa: E402
from corpus import v3lexicon as LX  # noqa: E402

FREEZE = "2026-10-26"
RUN = REPO / G.OUT_BASE / FREEZE
RUN11 = REPO / G.OUT_BASE / G.RUN_1_1.dir_name(FREEZE)
RUN12 = REPO / G.OUT_BASE / G.RUN_1_2.dir_name(FREEZE)
SECRET = "SECRET ABSTRACT SENTENCE that must never reach a committed file"


def read_candidates(run_dir):
    """Rows of a run's committed candidates.jsonl.gz."""
    return [json.loads(x) for x in S.read_text(Path(run_dir) / G.CANDIDATES_GZ).splitlines()]


def committed_text(path):
    """Text of a committed file as the leak check reads it (a .gz decompressed)."""
    return S.read_text(path) if path.name.endswith(S.SUFFIX) else path.read_text(encoding="utf-8")


def epmc_result(pmid, title, abstract=None, date="2024-05-01", lang="eng", doi=None, pubtypes=("Journal Article",),
                mesh=()):
    r = {"id": pmid, "source": "MED", "pmid": pmid, "doi": doi, "title": title, "authorString": "Author A, Author B.",
         "journalInfo": {"journal": {"title": "J Test"}}, "firstPublicationDate": date, "language": lang,
         "pubTypeList": {"pubType": list(pubtypes)}, "isOpenAccess": "N"}
    if abstract:
        r["abstractText"] = abstract
    if mesh:
        r["meshHeadingList"] = {"meshHeading": [{"descriptorName": m} for m in mesh]}
    return r


def assert_no_abstract_runs(case, run, run_dir):
    """Every cached abstract of ``run`` (and of its pool audit sample) against every committed file of the
    run: no shared run of 8 or more tokens outside titles (leak_check rules). Replaces the earlier check of
    two 80-character windows of every 50th abstract."""
    cache = F.Cache(G.cache_dir(REPO, FREEZE, run))
    cands, abstracts, _s, missing, _p = G.build_candidates(REPO, FREEZE, cache, run)
    case.assertEqual(missing, [])
    pool = G.pool_abstracts(cache, G.pool_sample(REPO, run), G.pool_abstract_batches(REPO, run))
    sources = [LC.Source(cid, "gap_search_abstract", None, "all", t) for cid, t in abstracts.items()]
    sources += [LC.Source(sl, "pool_abstract", None, "all", t) for sl, t in pool.items()]
    files = {"machine_outputs": {p.relative_to(REPO).as_posix(): committed_text(p)
                                 for p in run_dir.rglob("*") if p.is_file()}}
    leaks = LC.find_leaks(sources, files, LC.metadata_shingles(REPO))
    case.assertEqual([(lk.source_id, lk.path, lk.run_tokens) for lk in leaks], [])
    case.assertGreater(len(sources), 1000)


def make_repo(root):
    """Minimal repository: register, pool, pool index (v1.1 columns)."""
    (root / "docs/research/sela-relay").mkdir(parents=True)
    (root / "docs/research/sela-relay/research_100.json").write_text(json.dumps({"papers": [
        {"id": "R001", "doi": "10.5555/registered", "index_url": "https://pubmed.ncbi.nlm.nih.gov/111/"}]}))
    (root / "data/research").mkdir(parents=True)
    pool = [{"title": "Expressive writing for students", "doi": "10.5555/pool-a", "pmid": "901",
             "first_publication_date": "2025-01-01", "screening_status": "candidate_metadata_only_unreviewed"},
            {"title": "Unrelated agriculture survey", "doi": "10.5555/pool-b", "pmid": "902",
             "first_publication_date": "2025-01-01", "screening_status": "candidate_metadata_only_unreviewed"},
            {"title": "Correction: Journaling app for adolescents", "doi": "10.5555/pool-c", "pmid": "903",
             "first_publication_date": "2025-01-01", "screening_status": "candidate_metadata_only_unreviewed"}]
    (root / G.POOL).write_text("".join(json.dumps(r) + "\n" for r in pool))
    with (root / G.POOL_INDEX).open("w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=["candidate_id", "doi", "pmid", "title", "priority", "eligibility"],
                           lineterminator="\n")
        w.writeheader()
        w.writerow({"candidate_id": "SL-00001", "doi": "10.5555/pool-a", "pmid": "901",
                    "title": "Expressive writing for students", "priority": "deferred", "eligibility": "eligible"})
        w.writerow({"candidate_id": "SL-00002", "doi": "10.5555/pool-b", "pmid": "902",
                    "title": "Unrelated agriculture survey", "priority": "deferred", "eligibility": "eligible"})
        w.writerow({"candidate_id": "SL-00003", "doi": "10.5555/pool-c", "pmid": "903",
                    "title": "Correction: Journaling app for adolescents", "priority": "deferred",
                    "eligibility": "excluded_publication_type"})


def fill_cache(root, results_by_key=None, crossref_items=None, run=G.RUN_1_0):
    cache = F.Cache(G.cache_dir(root, FREEZE, run))
    for item in G.search_plan(FREEZE, run):
        if item["source"] == "europepmc":
            res = (results_by_key or {}).get(item["key"], [])
            body = G.trim_epmc({"hitCount": len(res), "resultList": {"result": res}})
        else:
            items = (crossref_items or {}).get(item["key"], [])
            body = G.trim_crossref({"message": {"total-results": len(items), "items": items}})
        cache.put(item["service"], item["key"], F.envelope(item["service"], item["key"], item["url"],
                                                           "2026-10-26T17:00:00Z", 200, "ok", body=body))
    pool_env = G.trim_epmc({"resultList": {"result": [epmc_result("901", "Expressive writing for students",
                                                                  abstract="A randomised trial of expressive writing "
                                                                           "for depression in university students.")]}})
    for batch in G.pool_abstract_batches(root, run):
        key = "pool_abstracts:" + ",".join(batch)
        cache.put("europepmc", key, F.envelope("europepmc", key, "u", "2026-10-26T17:00:00Z", 200, "ok",
                                               body=pool_env))
    return cache


class PlanTests(unittest.TestCase):
    def test_every_required_family_and_both_sources(self):
        plan = G.search_plan(FREEZE)
        fams = {p["family"] for p in plan}
        for required in ("listening_feeling_heard", "journaling_expressive_writing", "consultation_preparation",
                         "continuity_after_consultation", "adolescents_15_17", "older_adults_50_plus",
                         "accessibility_disability", "indonesia_digital_mh_privacy", "ai_bahasa_code_switching",
                         "help_seeking", "safety", "privacy_sharing_revocation"):
            self.assertIn(required, fams)
        for fam in fams:
            self.assertEqual({p["source"] for p in plan if p["family"] == fam}, {"europepmc", "crossref"}, fam)
        epmc = [p for p in plan if p["source"] == "europepmc"]
        self.assertTrue(all("FIRST_PDATE" in p["url"] and "resultType=core" in p["url"] for p in epmc))
        self.assertEqual(G.year_buckets("2021-10-26", FREEZE)[0], ("2021-10-26", "2021-12-31"))
        self.assertEqual(G.year_buckets("2021-10-26", FREEZE)[-1], ("2026-01-01", "2026-10-26"))
        cr = [p for p in plan if p["source"] == "crossref"]
        self.assertTrue(all("from-pub-date%3A2021-10-26" in p["url"] and "select=" not in p["url"] for p in cr))
        self.assertIn("id_pdp", {p["query_id"] for p in cr})  # Indonesian-language Crossref queries

    def test_cache_trim_keeps_abstract_locally_and_drops_bulk(self):
        body = G.trim_epmc({"hitCount": 1, "resultList": {"result": [dict(epmc_result("1", "T", abstract=SECRET),
                                                                          fullTextUrlList={"x": 1}, authorList={"y": 2})]}})
        r = body["resultList"]["result"][0]
        self.assertEqual(r["abstractText"], SECRET)  # local cache only, for screening
        self.assertNotIn("fullTextUrlList", r)
        self.assertNotIn("authorList", r)
        cr = G.trim_crossref({"message": {"items": [{"DOI": "10.1/x", "reference": [1], "abstract": "<jats:p>a</jats:p>",
                                                      "author": [{"family": "F", "given": "Given", "ORCID": "o"}]}]}})
        self.assertNotIn("reference", cr["message"]["items"][0])
        self.assertEqual(cr["message"]["items"][0]["author"], [{"family": "F", "given": "G"}])


class BuildAndScreenTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(tempfile.mkdtemp())
        make_repo(self.root)
        plan = G.search_plan(FREEZE)
        k_journal = next(p["key"] for p in plan if p["query_id"] == "journal_core" and "2024" in p["key"])
        k_teen = next(p["key"] for p in plan if p["query_id"] == "teen_consent" and "2025" in p["key"])
        k_listen = next(p["key"] for p in plan if p["query_id"] == "listen_core" and "2024" in p["key"])
        epmc = {
            k_journal: [
                epmc_result("201", "Expressive writing app for depression: a randomised trial", doi="10.5555/a",
                            abstract=SECRET + " Expressive writing did not improve depression versus control."),
                epmc_result("202", "Correction: Expressive writing app", doi="10.5555/corr",
                            pubtypes=("Published Erratum",)),
                epmc_result("111", "Registered paper on journaling and mental health", doi="10.5555/registered"),
            ],
            k_teen: [epmc_result("301", "Adolescent confidentiality and parental access to patient portals in mental "
                                        "health care", doi="10.5555/b", date="2025-03-03",
                                 abstract="Adolescents aged 15-17 years described confidentiality concerns.",
                                 mesh=("Adolescent", "Humans"))],
            k_listen: [epmc_result("201", "Expressive writing app for depression: a randomised trial", doi="10.5555/A")],
        }
        crossref = {"crossref:id_kesehatan_mental_aplikasi": [{
            "DOI": "10.5555/ID-1", "title": ["Aplikasi kesehatan mental untuk remaja di Indonesia"],
            "container-title": ["Jurnal Uji"], "published-online": {"date-parts": [[2023, 2, 1]]},
            "type": "journal-article", "language": "en",
            "abstract": "<jats:p>Penelitian aplikasi kesehatan mental remaja dengan fitur jurnal harian.</jats:p>"}],
            "crossref:revocation_cr": [{"DOI": "10.5555/notice", "title": ["Dynamic consent platforms"],
                                        "type": "journal-article", "published-print": {"date-parts": [[2024, 1]]},
                                        "update-to": [{"type": "correction"}]}]}
        self.cache = fill_cache(self.root, epmc, crossref)

    def tearDown(self):
        shutil.rmtree(self.root, ignore_errors=True)

    def build(self):
        G.run_build(self.root, FREEZE)
        return {r["doi"]: r for r in read_candidates(self.root / G.OUT_BASE / FREEZE)}

    def test_build_merges_sources_records_language_and_never_commits_abstracts(self):
        rows = self.build()
        a = rows["10.5555/a"]
        self.assertEqual(len(a["retrieval"]), 2)  # same DOI from two families merged (case-insensitive DOI)
        self.assertEqual(a["families_retrieved"], ["journaling_expressive_writing", "listening_feeling_heard"])
        self.assertEqual(a["publication_language"], "en")
        self.assertTrue(a["abstract_available"])
        self.assertEqual(len(a["abstract_sha256"]), 64)
        self.assertEqual(rows["10.5555/registered"]["in_register"], "R001")
        self.assertEqual(rows["10.5555/corr"]["publication_type_eligibility"]["status"], "excluded_publication_type")
        self.assertEqual(rows["10.5555/notice"]["publication_type_eligibility"]["status"], "excluded_publication_type")
        self.assertTrue(rows["10.5555/notice"]["within_window"])  # month precision 2024-01 fits the window
        ind = rows["10.5555/id-1"]
        self.assertEqual(ind["publication_language"], "en")  # deposited value kept...
        self.assertTrue(ind["publication_language_detail"]["title_suggests_indonesian"])  # ...but flagged
        self.assertTrue(ind["publication_language_detail"]["title_cue_disagrees"])
        committed = "".join(committed_text(p) for p in (self.root / G.OUT_BASE).rglob("*") if p.is_file())
        self.assertNotIn("SECRET ABSTRACT", committed)
        self.assertNotIn("jurnal harian", committed)
        self.assertEqual(G.run_verify(self.root, FREEZE), [])

    def test_screening_labels_queue_worksheet_and_audit_sample(self):
        self.build()
        m = G.run_screen(self.root, FREEZE)
        self.assertEqual(m["status"], "machine_title_abstract_screen_unreviewed")
        self.assertFalse(m["is_appraisal"])
        self.assertFalse(m["human_screening_done"])
        sd = self.root / G.OUT_BASE / FREEZE / "screening"
        index = {r["doi"]: r for r in csv.DictReader((sd / "index.csv").open(newline=""))}
        self.assertEqual(index["10.5555/corr"]["priority"], "excluded_publication_type")
        self.assertIn("journaling_expressive_writing", index["10.5555/a"]["feature_families"])
        self.assertEqual(index["10.5555/a"]["possible_null_or_negative_finding"], "did not improve")
        self.assertEqual(index["10.5555/b"]["population_band"], "includes_15_17_explicit")
        self.assertIn("adolescent_consent_confidentiality", index["10.5555/b"]["feature_families"])
        queue = [json.loads(x) for x in (sd / "queue.jsonl").read_text().splitlines()]
        self.assertTrue(queue)
        self.assertTrue(all(q["screen"]["priority"] in ("high", "medium") for q in queue))
        ws = list(csv.DictReader((sd / "worksheet.csv").open(newline="")))
        self.assertTrue(all(w["reviewer_1_decision"] == w["reviewer_2_decision"] == w["final_decision"]
                            == "NOT_REVIEWED" and w["rag_ready"] == "NO" for w in ws))
        audit = list(csv.DictReader((sd / "fn_audit_sample.csv").open(newline="")))
        pool = [a for a in audit if a["source"] == "pool_7580"]
        self.assertEqual({a["candidate_id"] for a in pool}, {"SL-00001", "SL-00002"})
        sl1 = next(a for a in pool if a["candidate_id"] == "SL-00001")
        self.assertEqual(sl1["machine_flag"], "possible_false_negative")  # v3 rescreen with the cached abstract
        self.assertEqual(sl1["abstract_used"], "True")
        self.assertTrue(all(a["auditor_relevance"] == "NOT_AUDITED" and not a["auditor"] for a in audit))
        self.assertEqual(G.run_verify(self.root, FREEZE), [])
        committed = "".join(p.read_text() for p in sd.rglob("*") if p.is_file())
        self.assertNotIn("SECRET ABSTRACT", committed)
        self.assertNotIn("randomised trial of expressive writing for depression in university", committed)

    def test_verify_catches_tampering_abstract_leaks_and_unattributed_decisions(self):
        self.build()
        G.run_screen(self.root, FREEZE)
        od = self.root / G.OUT_BASE / FREEZE
        rows = read_candidates(od)
        rows[0]["abstract"] = SECRET
        (od / G.CANDIDATES_GZ).write_bytes(S.gzip_bytes("".join(json.dumps(r) + "\n" for r in rows).encode()))
        problems = G.run_verify(self.root, FREEZE)
        self.assertTrue(any("sha256" in p for p in problems))
        self.assertTrue(any("abstracts must never be committed" in p for p in problems))
        G.run_build(self.root, FREEZE)
        G.run_screen(self.root, FREEZE)
        ws = od / "screening" / "worksheet.csv"
        text = ws.read_text().replace("NOT_REVIEWED,NO", "INCLUDE,NO", 1)
        ws.write_text(text)
        problems = G.run_verify(self.root, FREEZE)
        self.assertTrue(any("two named reviewers" in p or "sha256" in p for p in problems))

    def test_candidates_are_stored_as_deterministic_gzip_and_rebuild_byte_identical(self):
        self.build()
        od = self.root / G.OUT_BASE / FREEZE
        self.assertFalse((od / G.CANDIDATES).exists())  # only the .gz is written
        raw = (od / G.CANDIDATES_GZ).read_bytes()
        content = gzip.decompress(raw)
        self.assertEqual(raw, S.gzip_bytes(content))
        self.assertEqual(raw[:10], S.HEADER)  # no file name, MTIME 0
        m = json.loads((od / "manifest.json").read_text())
        self.assertEqual(m["candidates_sha256"], hashlib.sha256(content).hexdigest())  # the content, as before
        st = m["candidates_storage"]
        self.assertEqual((st["file"], st["storage_version"]), (G.CANDIDATES_GZ, S.STORAGE_VERSION))
        self.assertEqual(st["gz_sha256"], hashlib.sha256(raw).hexdigest())
        G.run_screen(self.root, FREEZE)
        sm = json.loads((od / "screening" / "manifest.json").read_text())
        self.assertEqual(sm["inputs"]["candidates.jsonl"], m["candidates_sha256"])  # logical name, content hash
        manifest = (od / "manifest.json").read_bytes()
        G.run_build(self.root, FREEZE)
        self.assertEqual((od / G.CANDIDATES_GZ).read_bytes(), raw)
        self.assertEqual((od / "manifest.json").read_bytes(), manifest)
        self.assertEqual(G.run_verify(self.root, FREEZE), [])
        # an uncompressed copy next to the .gz is a violation, and build removes it
        (od / G.CANDIDATES).write_bytes(content)
        self.assertTrue(any("present uncompressed" in p for p in G.run_verify(self.root, FREEZE)))
        G.run_build(self.root, FREEZE)
        self.assertFalse((od / G.CANDIDATES).exists())
        self.assertEqual(G.run_verify(self.root, FREEZE), [])
        # same content with a file name and a time stamp in the gzip header: a violation, and build restores it
        buf = io.BytesIO()
        with gzip.GzipFile(filename="candidates.jsonl", mode="wb", fileobj=buf, mtime=1700000000) as f:
            f.write(content)
        (od / G.CANDIDATES_GZ).write_bytes(buf.getvalue())
        problems = G.run_verify(self.root, FREEZE)
        self.assertTrue(any("header is not the deterministic one" in p for p in problems))
        self.assertTrue(any("gz_sha256" in p for p in problems))
        self.assertFalse(any(p.startswith("candidates.jsonl sha256") for p in problems))  # the content is unchanged
        G.run_build(self.root, FREEZE)
        self.assertEqual((od / G.CANDIDATES_GZ).read_bytes(), raw)
        (od / G.CANDIDATES_GZ).unlink()
        self.assertTrue(any("candidates.jsonl.gz missing" in p for p in G.run_verify(self.root, FREEZE)))

    def test_audit_sampling_is_hash_ranked_and_stable(self):
        a = G.rank_key(G.AUDIT_SEED, "pool_7580:deferred", "SL-00001")
        self.assertEqual(a, hashlib.sha256(f"{G.AUDIT_SEED}|pool_7580:deferred|SL-00001".encode()).hexdigest())
        self.assertNotEqual(a, G.rank_key(G.AUDIT_SEED, "pool_7580:deferred", "SL-00002"))


class CandidatesStorageTests(unittest.TestCase):
    """corpus.gzstore (sela-jsonl-gzip-v1): deterministic gzip; recorded hashes are of the uncompressed content."""
    DATA = "".join(json.dumps({"candidate_id": f"V3-{i:05d}", "title": f"Title {i} of a candidate"}) + "\n"
                   for i in range(500)).encode()

    def test_gzip_bytes_are_deterministic_and_carry_no_name_or_time(self):
        a = S.gzip_bytes(self.DATA)
        self.assertEqual(a, S.gzip_bytes(self.DATA))
        self.assertEqual(a[:4], b"\x1f\x8b\x08\x00")  # deflate; FLG 0: no file name, comment or extra field
        self.assertEqual(a[4:8], b"\x00\x00\x00\x00")  # MTIME 0
        self.assertEqual(a[8:10], b"\x02\xff")  # XFL 2 (level 9), OS 255
        self.assertEqual(gzip.decompress(a), self.DATA)
        self.assertEqual(S.decode(a), (self.DATA, []))
        self.assertLess(len(a), len(self.DATA) // 4)

    def test_decode_names_every_departure_from_the_format(self):
        buf = io.BytesIO()
        with gzip.GzipFile(filename="candidates.jsonl", mode="wb", fileobj=buf, mtime=1234) as f:
            f.write(self.DATA)
        content, problems = S.decode(buf.getvalue())
        self.assertEqual(content, self.DATA)
        self.assertTrue(any("header" in p for p in problems))
        two = S.gzip_bytes(self.DATA[:100]) + S.gzip_bytes(self.DATA[100:])
        self.assertTrue(any("more than one gzip member" in p for p in S.decode(two)[1]))
        bad = bytearray(S.gzip_bytes(self.DATA))
        bad[-1] ^= 1  # size field
        self.assertTrue(any("trailer" in p for p in S.decode(bytes(bad))[1]))
        self.assertEqual(S.decode(b"not gzip")[0], None)
        self.assertTrue(S.decode(S.gzip_bytes(self.DATA)[:-20])[1])  # truncated

    def test_write_keeps_bytes_whose_content_is_unchanged(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "candidates.jsonl"
            # the same content from another deflate encoder (level 1 here) in the same container is kept as is,
            # so a rebuild never rewrites committed bytes where the local zlib compresses differently
            c = zlib.compressobj(1, zlib.DEFLATED, -zlib.MAX_WBITS)
            other = S.HEADER + c.compress(self.DATA) + c.flush() + struct.pack("<II", zlib.crc32(self.DATA),
                                                                                 len(self.DATA))
            self.assertNotEqual(other, S.gzip_bytes(self.DATA))
            S.gz_path(p).write_bytes(other)
            self.assertEqual(S.write(p, self.DATA), other)
            self.assertEqual(S.gz_path(p).read_bytes(), other)
            new = self.DATA + b'{"candidate_id": "V3-99999"}\n'
            self.assertEqual(S.write(p, new), S.gzip_bytes(new))  # new content: written deterministically
            self.assertEqual(S.read_bytes(p), new)  # the logical name resolves to the .gz
            self.assertEqual(S.content_sha256(p), hashlib.sha256(new).hexdigest())
            self.assertEqual(S.logical_path(S.gz_path(p)), p)
            self.assertFalse(p.exists())
            self.assertTrue(S.exists(p))


class ScreenRuleTests(unittest.TestCase):
    ELIGIBLE = {"status": "eligible"}

    def test_feature_families_gates_and_priorities(self):
        s = G.screen_one("Expressive writing app for depression", "A randomised trial in university students.",
                         None, self.ELIGIBLE)
        self.assertEqual(s["priority"], "high")
        self.assertEqual(s["status"], "machine_title_abstract_screen_unreviewed")
        # consent words alone do not make a minors hit; an adolescent signal is required
        s = G.screen_one("Confidentiality in adult psychotherapy", "Adults discussed confidentiality.", None, self.ELIGIBLE)
        self.assertNotIn("adolescent_consent_confidentiality", s["feature_families"])
        s = G.screen_one("Parental consent for adolescent mental health apps", "", None, self.ELIGIBLE)
        self.assertIn("adolescent_consent_confidentiality", s["feature_families"])
        # language family needs an AI/NLP term
        self.assertNotIn("ai_language_code_switching",
                         G.screen_one("Teaching Bahasa Indonesia in schools", "", None, self.ELIGIBLE)["feature_families"])
        self.assertIn("ai_language_code_switching",
                      G.screen_one("Code-switching Indonesian-English text and large language models for depression",
                                   "", None, self.ELIGIBLE)["feature_families"])
        self.assertEqual(G.screen_one("Expressive writing", "", None, {"status": "excluded_publication_type"})["priority"],
                         "excluded_publication_type")
        self.assertEqual(G.screen_one("Expressive writing and depression", "", None, self.ELIGIBLE,
                                      within_window=False)["priority"], "excluded_out_of_window")
        self.assertEqual(G.screen_one("A survey of agriculture", "", None, self.ELIGIBLE)["priority"], "deferred")

    def test_accessibility_means_digital_access_not_service_access(self):
        self.assertNotIn("accessibility_disability", LX.family_hits("Accessibility of mental health services in rural "
                                                                    "areas", ""))
        self.assertIn("accessibility_disability", LX.family_hits("Digital accessibility of mental health apps for "
                                                                 "blind users", ""))

    def test_population_bands_keep_adults_and_minors_apart(self):
        teen = G.screen_one("Chatbot for adolescents", "Participants aged 15-17 years.", None, self.ELIGIBLE)
        self.assertEqual(G.population_band(teen), "includes_15_17_explicit")
        adult = G.screen_one("Chatbot for adults with depression", "Adults aged 18-65 years took part.", None,
                             self.ELIGIBLE)
        self.assertEqual(G.population_band(adult), "adults_signal")
        mid = G.screen_one("App for depression", "", ["Middle Aged", "Adult"], self.ELIGIBLE)
        self.assertEqual(G.population_band(mid), "adults_signal")  # MeSH Middle Aged is not a 50+ signal
        old = G.screen_one("App for older adults with depression", "", ["Aged"], self.ELIGIBLE)
        self.assertEqual(G.population_band(old), "older_adults_signal")

    def test_queue_is_stratified_by_family(self):
        rows = []
        for i in range(40):
            rows.append({"candidate_id": f"V3-{i:05d}", "screen": G.screen_one(
                "Safety of chatbots in suicide crisis for depression", "", None, self.ELIGIBLE)})
        rows.append({"candidate_id": "V3-09999", "screen": G.screen_one(
            "Question prompt list before a mental health consultation", "", None, self.ELIGIBLE)})
        q = G.select_queue(rows, maximum=10, per_family=5)
        self.assertIn("V3-09999", {r["candidate_id"] for r in q})
        self.assertLessEqual(len(q), 10)


class RunV11PlanTests(unittest.TestCase):
    """Run v1.1 is a new frozen run: v1.0's plan is untouched, v1.1 fixes the review findings."""

    def test_v1_0_plan_is_unchanged(self):
        plan = G.search_plan(FREEZE)
        self.assertEqual(len(plan), 112)
        self.assertTrue(all(":n" not in p["key"] and "cap" not in p for p in plan))
        self.assertIn("id_digital", {p["query_id"] for p in plan})

    def test_v1_1_queries_caps_and_new_family(self):
        plan = G.search_plan(FREEZE, G.RUN_1_1)
        qids = {p["query_id"] for p in plan}
        self.assertNotIn("id_digital", qids)  # replaced: it required no mental-health term
        self.assertNotIn("a11y_core", qids)  # replaced by the narrowed query
        for q in ("id_digital_mh", "id_health_data_law", "a11y_core_v2", "working_adults_digital",
                  "working_adults_cr", "id_dewasa_pekerja_cr", "id_lansia_cr", "id_remaja_privasi_cr"):
            self.assertIn(q, qids)
        self.assertIn("working_age_adults", {p["family"] for p in plan})
        mh = G.QUERY_FAMILIES_1_1["indonesia_digital_mh_privacy"]["europepmc"][0][1]
        self.assertIn('"mental health"', mh)
        for p in plan:
            if p["source"] == "europepmc":
                self.assertIn(f"pageSize={p['cap']}", p["url"])
                self.assertEqual(p["key"].endswith(":n200"), p["cap"] == 200, p["key"])
        for q in ("checkin_core", "teen_digital", "older_digital"):  # retrieved at <= 5% in v1.0
            self.assertEqual(G.RUN_1_1.cap(q), 200)
        unchanged = next(p for p in plan if p["query_id"] == "listen_core")
        self.assertEqual(unchanged["key"], next(p["key"] for p in G.search_plan(FREEZE)
                                                 if p["query_id"] == "listen_core"))  # reusable cache key
        self.assertEqual(G.run_for_dir("2026-10-26_search-1.1"), G.RUN_1_1)
        self.assertEqual(G.run_for_dir("2026-10-26"), G.RUN_1_0)
        self.assertNotEqual(G.cache_dir(REPO, FREEZE, G.RUN_1_1), G.cache_dir(REPO, FREEZE))

    def test_retrieval_table_reports_hits_retrieved_and_share(self):
        stats = [{"family": "f", "source": "europepmc", "query_id": "q", "hit_count": 1000, "returned": 50},
                 {"family": "f", "source": "europepmc", "query_id": "q", "hit_count": 10, "returned": 10}]
        row = G.retrieval_table(stats)[0]
        self.assertEqual((row["hit_count"], row["retrieved"], row["capped_requests"]), (1010, 60, 1))
        self.assertAlmostEqual(row["share_retrieved"], 60 / 1010, places=4)


class LexiconV2Tests(unittest.TestCase):
    """sela-v3-families-2 adds Indonesian age, feature and consent terms; families-1 is unchanged."""
    ELIGIBLE = {"status": "eligible"}
    L2 = LX.LEXICONS["sela-v3-families-2"]

    def band(self, title, abstract="", lexicon=None, mesh=None):
        s = G.screen_one(title, abstract, mesh, self.ELIGIBLE, lexicon=lexicon)
        return G.population_band_v2(s) if lexicon else G.population_band(s)

    def test_indonesian_age_terms_give_population_signals(self):
        self.assertEqual(self.band("GAMBARAN KESEHATAN MENTAL PADA REMAJA"), "not_stated")  # v1.0 behaviour kept
        self.assertEqual(self.band("GAMBARAN KESEHATAN MENTAL PADA REMAJA", lexicon=self.L2),
                         "adolescent_or_youth_unverified")
        self.assertEqual(self.band("Depresi pada lansia di panti werdha", lexicon=self.L2), "older_adults_signal")
        self.assertEqual(self.band("Stres kerja pada karyawan dewasa", lexicon=self.L2), "adults_signal")
        self.assertEqual(self.band("Kesehatan mental siswa", "Siswa berusia 15-17 tahun mengisi kuesioner.",
                                   lexicon=self.L2), "includes_15_17_explicit")
        self.assertNotIn("adolescents", self.L2.family_hits("Spinal muscular atrophy (SMA) and caregiver stress", ""))

    def test_indonesian_feature_and_consent_terms(self):
        title = "Persetujuan orang tua dan kerahasiaan konseling remaja"
        v1 = G.screen_one(title, "", None, self.ELIGIBLE)
        v2 = G.screen_one(title, "", None, self.ELIGIBLE, lexicon=self.L2)
        self.assertNotIn("adolescent_consent_confidentiality", v1["feature_families"])
        self.assertIn("adolescent_consent_confidentiality", v2["feature_families"])
        for title, fam in (("Menulis ekspresif untuk mahasiswa", "journaling_expressive_writing"),
                           ("Jurnal harian dan kesehatan mental", "journaling_expressive_writing"),
                           ("Privasi data aplikasi kesehatan mental", "privacy"),
                           ("Ide bunuh diri pada remaja", "safety"),
                           ("Digital mental health app for employees", "working_age_adults")):
            self.assertIn(fam, G.screen_one(title, "", None, self.ELIGIBLE, lexicon=self.L2)["feature_families"],
                          title)
        # a v1 entry is never changed by v2: every families-1 label is still present
        for fam, entries in LX.FAMILIES.items():
            self.assertEqual([e[0] for e in entries], [e[0] for e in self.L2.FAMILIES[fam]][:len(entries)], fam)

    def test_band_v2_needs_one_consistent_range_and_no_review(self):
        L2 = self.L2
        self.assertEqual(self.band("Chatbot", "Participants aged 15-17 years.", L2), "includes_15_17_explicit")
        self.assertEqual(self.band("Chatbot", "Participants aged 16-30 years; others aged 18-30 years.", L2),
                         "adolescent_or_youth_unverified")  # conflicting ranges: never explicit
        self.assertEqual(self.band("A systematic review of chatbots", "Studies of people aged 14 years or older.", L2),
                         "adolescent_or_youth_unverified")  # review eligibility, not one sample
        self.assertEqual(self.band("Chatbot", "All people aged between 18 and 30 years were invited. The working "
                                              "group consisted of young people (aged 16-30) with lived experience.",
                                   L2), "adults_signal")  # the advisory group's ages do not count
        # v1.0 band unchanged for the same text
        self.assertEqual(self.band("Chatbot", "Participants aged 16-30 years; others aged 18-30 years."),
                         "includes_15_17_explicit")


class RunV11BuildScreenTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(tempfile.mkdtemp())
        make_repo(self.root)
        plan = G.search_plan(FREEZE, G.RUN_1_1)
        k_journal = next(p["key"] for p in plan if p["query_id"] == "journal_core" and "2024" in p["key"])
        k_teen = next(p["key"] for p in plan if p["query_id"] == "teen_consent" and "2025" in p["key"])
        k_work = next(p["key"] for p in plan if p["query_id"] == "working_adults_digital" and "2023" in p["key"])
        epmc = {
            k_journal: [epmc_result(str(500 + i), f"Expressive writing study number {i} for depression",
                                    doi=f"10.5555/w{i}", abstract=SECRET + " Expressive writing and depression.")
                        for i in range(30)] + [
                epmc_result("202", "Correction: Expressive writing app", doi="10.5555/corr",
                            pubtypes=("Published Erratum",)),
                epmc_result("203", "Study protocol for a trial of journaling", doi="10.5555/prot")],
            k_teen: [epmc_result("301", "Kerahasiaan dan persetujuan orang tua pada konseling remaja", doi="10.5555/b",
                                 date="2025-03-03", abstract="Remaja berusia 15-17 tahun diwawancarai.")],
            k_work: [epmc_result("401", "A digital mental health app for employees with burnout", doi="10.5555/c",
                                 date="2023-04-04", abstract="Workers aged 25-60 years used the app for stress.")],
        }
        fill_cache(self.root, epmc, run=G.RUN_1_1)
        self.od = self.root / G.OUT_BASE / G.RUN_1_1.dir_name(FREEZE)

    def tearDown(self):
        shutil.rmtree(self.root, ignore_errors=True)

    def build_and_screen(self):
        m = G.run_build(self.root, FREEZE, run=G.RUN_1_1)
        sm = G.run_screen(self.root, FREEZE, run=G.RUN_1_1)
        return m, sm

    def test_new_run_directory_versions_and_retrieval_table(self):
        m, sm = self.build_and_screen()
        self.assertEqual(m["run_version"], "sela-v3-gap-search-v1.1")
        self.assertEqual(sm["screening_version"], "sela-v3-tiab-screen-v1.1")
        self.assertEqual(sm["lexicon_version"], "sela-v3-families-2")
        self.assertFalse((self.root / G.OUT_BASE / FREEZE).exists())  # v1.0 directory not touched
        self.assertTrue(m["per_query_retrieval"])
        self.assertTrue(all("share_retrieved" in r for r in m["per_query_retrieval"]))
        self.assertIn("working_age_adults", sm["screened_in_by_feature_family"])
        self.assertEqual(G.run_verify(self.root, FREEZE, run=G.RUN_1_1), [])
        self.assertEqual(G.run_verify(self.root, G.RUN_1_1.dir_name(FREEZE)), [])

    def test_blinded_audit_sheet_and_key(self):
        self.build_and_screen()
        sd = self.od / "screening"
        self.assertFalse((sd / "fn_audit_sample.csv").exists())
        with (sd / "fn_audit_sheet.csv").open(newline="") as f:
            reader = csv.DictReader(f)
            self.assertEqual(reader.fieldnames, G.AUDIT_SHEET_FIELDS)
            sheet = list(reader)
        self.assertFalse((sd / "fn_audit_key.csv").exists())  # review round 3: the key is not next to the sheet
        key_path = G.private_dir(self.root, FREEZE, G.RUN_1_1) / "fn_audit_key.csv"
        with key_path.open(newline="") as f:
            key = list(csv.DictReader(f))
        sm = json.loads((sd / "manifest.json").read_text())
        self.assertEqual(sm["outputs"]["fn_audit_key.csv"], G._sha(key_path))  # same bytes, same recorded hash
        self.assertEqual([r["audit_id"] for r in sheet], [r["audit_id"] for r in key])
        for col in ("stratum", "machine_priority", "machine_flag", "candidate_id", "source"):
            self.assertNotIn(col, sheet[0])
        self.assertTrue(all(r["auditor_relevance"] == r["auditor_publication_kind"] == "NOT_AUDITED" for r in sheet))
        strata = {r["stratum"] for r in key}
        self.assertIn("excluded_publication_type", strata)
        excluded = [r for r in key if r["stratum"] == "excluded_publication_type"]
        self.assertEqual({r["source"] for r in excluded}, {"v3_run", "pool_7580"})
        self.assertTrue(all(r["eligibility"] == "excluded_publication_type" for r in excluded))
        order = sorted(key, key=lambda r: G.rank_key(G.AUDIT_SEED, "blinded-order", f"{r['source']}:{r['candidate_id']}"))
        self.assertEqual([r["audit_id"] for r in order], [r["audit_id"] for r in key])
        committed = "".join(p.read_text() for p in sd.rglob("*") if p.is_file())
        self.assertNotIn("SECRET ABSTRACT", committed)

    def test_verify_catches_unblinded_sheets_and_unattributed_audits(self):
        self.build_and_screen()
        sd = self.od / "screening"
        sheet = sd / "fn_audit_sheet.csv"
        original = sheet.read_text()
        sheet.write_text(original.replace("audit_id,title,doi,", "audit_id,stratum,title,doi,", 1))
        self.assertTrue(any("blinded set" in p for p in G.verify_blinded_audit(sd)))
        sheet.write_text(original.replace("NOT_AUDITED,NOT_AUDITED", "relevant,study_report", 1))
        self.assertTrue(any("needs auditor and date" in p for p in G.verify_blinded_audit(sd)))
        key = G.private_dir(self.root, FREEZE, G.RUN_1_1) / "fn_audit_key.csv"
        lines = key.read_text().splitlines(keepends=True)
        key.write_text(lines[0] + "".join(sorted(lines[1:], key=lambda x: x.split(",")[3])))
        sheet.write_text(original)
        self.assertTrue(G.verify_blinded_audit(sd, key_path=key))
        self.assertTrue(any("fn_audit_key.csv sha256" in p for p in G.run_verify(self.root, FREEZE, run=G.RUN_1_1)))

    def test_v1_1_never_writes_into_the_v1_0_cache(self):
        before = sorted(p.name for p in G.cache_dir(self.root, FREEZE).rglob("*")) \
            if G.cache_dir(self.root, FREEZE).exists() else []
        self.build_and_screen()
        after = sorted(p.name for p in G.cache_dir(self.root, FREEZE).rglob("*")) \
            if G.cache_dir(self.root, FREEZE).exists() else []
        self.assertEqual(before, after)


@unittest.skipUnless((RUN11 / "manifest.json").exists(), "v3 gap search run v1.1 outputs not present")
class CommittedRunV11Tests(unittest.TestCase):
    def test_committed_run_v1_1_verifies_and_is_blinded(self):
        self.assertEqual(G.run_verify(REPO, RUN11.name), [])
        m = json.loads((RUN11 / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(m["run_version"], G.RUN_1_1.run_version)
        self.assertEqual(set(m["query_families"]), set(G.QUERY_FAMILIES_1_1))
        self.assertTrue(m["per_query_retrieval"])
        sm = json.loads((RUN11 / "screening" / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(sm["screening_version"], G.RUN_1_1.screen_version)
        self.assertEqual(sm["audit_sample"]["status"], "prepared_not_audited")
        with (RUN11 / "screening" / "fn_audit_sheet.csv").open(encoding="utf-8", newline="") as f:
            self.assertEqual(csv.DictReader(f).fieldnames, G.AUDIT_SHEET_FIELDS)

    def test_committed_v1_1_candidates_are_metadata_only(self):
        for r in read_candidates(RUN11):
            self.assertNotIn("abstract", r)
            self.assertEqual(r["status"], "candidate_metadata_only_unreviewed")

    @unittest.skipUnless((REPO / G.CACHE_BASE / G.RUN_1_1.dir_name(FREEZE)).exists(), "local v1.1 cache not present")
    def test_no_cached_abstract_text_in_committed_v1_1_files(self):
        """LOCAL ONLY (needs the gitignored cache; skipped in CI): every abstract, 8-token runs."""
        assert_no_abstract_runs(self, G.RUN_1_1, RUN11)

    def test_v1_1_audit_key_is_not_committed(self):
        """Review round 3: the key is held outside the repository; its hash stays in the manifest."""
        self.assertFalse((RUN11 / "screening" / "fn_audit_key.csv").exists())
        sm = json.loads((RUN11 / "screening" / "manifest.json").read_text(encoding="utf-8"))
        self.assertRegex(sm["outputs"]["fn_audit_key.csv"], r"^[0-9a-f]{64}$")


@unittest.skipUnless((RUN / "manifest.json").exists(), "v3 gap search outputs not present")
class CommittedRunTests(unittest.TestCase):
    """CI (no cache): the frozen run and its machine screening keep their labels and invariants."""

    def test_committed_run_verifies(self):
        self.assertEqual(G.run_verify(REPO, FREEZE), [])

    def test_run_manifest(self):
        m = json.loads((RUN / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual((m["freeze_date"], m["cutoff_date"]), ("2026-10-26", "2021-10-26"))
        self.assertEqual(m["status"], "candidate_metadata_only_unreviewed")
        self.assertEqual(set(m["query_families"]), set(G.QUERY_FAMILIES))
        self.assertGreater(m["counts"]["only_crossref"], 0)
        self.assertGreater(m["counts"]["candidates"], 1000)
        self.assertEqual(m["counts"]["within_window"].get("false", 0), 0)
        self.assertTrue(all(s["returned"] <= (G.EPMC_BUCKET_CAP if s["source"] == "europepmc" else G.CROSSREF_ROWS)
                            for s in m["requests"]))

    def test_committed_candidates_are_metadata_only(self):
        for r in read_candidates(RUN):
            self.assertNotIn("abstract", r)
            self.assertEqual(r["status"], "candidate_metadata_only_unreviewed")
            self.assertTrue(r["publication_language_detail"]["status"].startswith("machine_"))
            self.assertIn(r["within_window"], (True, None))

    def test_committed_screening_is_machine_labelled_and_unreviewed(self):
        sd = RUN / "screening"
        sm = json.loads((sd / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(sm["screening_version"], G.SCREEN_VERSION)
        self.assertEqual(sm["audit_sample"]["status"], "prepared_not_audited")
        self.assertLessEqual(sm["queue_count"], 400)
        for w in csv.DictReader((sd / "worksheet.csv").open(encoding="utf-8", newline="")):
            self.assertEqual((w["final_decision"], w["rag_ready"], w["full_text_appraised"]), ("NOT_REVIEWED", "NO", "NO"))
        audit = list(csv.DictReader((sd / "fn_audit_sample.csv").open(encoding="utf-8", newline="")))
        deferred_pool = [a for a in audit if a["source"] == "pool_7580" and a["stratum"] == "deferred"
                         and a["selection"] == "random"]
        self.assertEqual(len(deferred_pool), G.AUDIT_STRATA[("pool_7580", "deferred")])
        self.assertTrue(all(a["auditor_relevance"] == "NOT_AUDITED" for a in audit))
        for q in map(json.loads, (sd / "queue.jsonl").read_text(encoding="utf-8").splitlines()):
            self.assertEqual(q["screen"]["status"], "machine_title_abstract_screen_unreviewed")
            self.assertIn(q["screen"]["priority"], ("high", "medium"))

    @unittest.skipUnless((REPO / G.CACHE_BASE / FREEZE).exists(), "local v3 cache not present")
    def test_no_cached_abstract_text_in_committed_files(self):
        """LOCAL ONLY (needs the gitignored cache; skipped in CI): every abstract, 8-token runs."""
        assert_no_abstract_runs(self, G.RUN_1_0, RUN)


@unittest.skipUnless((RUN / "manifest.json").exists(), "v3 gap search outputs not present")
class CommittedStorageTests(unittest.TestCase):
    """CI (no cache): every run commits its candidates only as deterministic gzip, with both hashes recorded."""

    def test_every_run_commits_only_the_compressed_candidates(self):
        runs = [d for d in (RUN, RUN11, RUN12) if (d / "manifest.json").exists()]
        for d in runs:
            self.assertFalse((d / G.CANDIDATES).exists(), d.name)
            raw = (d / G.CANDIDATES_GZ).read_bytes()
            content, problems = S.decode(raw)
            self.assertEqual(problems, [], d.name)
            m = json.loads((d / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(m["candidates_sha256"], hashlib.sha256(content).hexdigest(), d.name)
            self.assertEqual(m["candidates_storage"]["gz_sha256"], hashlib.sha256(raw).hexdigest(), d.name)
            sm = json.loads((d / "screening" / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(sm["inputs"]["candidates.jsonl"], m["candidates_sha256"], d.name)
        self.assertEqual(sorted(p.parent.name for p in (REPO / G.OUT_BASE).glob(f"*/{G.CANDIDATES}")), [])

    @unittest.skipUnless((RUN12 / "manifest.json").exists(), "v3 gap search run v1.2 outputs not present")
    def test_earlier_runs_recorded_as_inputs_by_content_hash(self):
        m = json.loads((RUN12 / "manifest.json").read_text(encoding="utf-8"))
        for d in (RUN, RUN11):
            name = (d / G.CANDIDATES).relative_to(REPO).as_posix()
            self.assertEqual(m["inputs"][name], S.content_sha256(REPO / name), name)


class FakeClient:
    """Serves Europe PMC cursor pages: ``pages[n]`` is the result list of page n (1-based)."""

    def __init__(self, pages, hit_count):
        self.pages, self.hit_count, self.urls, self.requests, self.mailto = pages, hit_count, [], 0, None

    def get(self, service, url, accept="application/json", attempts=None):
        from urllib.parse import parse_qs, urlparse
        self.urls.append(url)
        self.requests += 1
        cursor = parse_qs(urlparse(url).query)["cursorMark"][0]
        n = 1 if cursor == "*" else int(cursor.split("-")[1])
        res = self.pages.get(n, [])
        body = {"hitCount": self.hit_count, "nextCursorMark": f"c-{n + 1}" if res else cursor,
                "resultList": {"result": res}}
        return 200, json.dumps(body).encode(), None


class RunV12PlanTests(unittest.TestCase):
    """Run v1.2 (review round 3) retrieves every hit of the D-048 gap queries; v1.0 and v1.1 are unchanged."""

    def test_v1_2_queries_all_hits_and_crossref_rows(self):
        plan = G.search_plan(FREEZE, G.RUN_1_2)
        p11 = {p["key"]: p for p in G.search_plan(FREEZE, G.RUN_1_1)}
        for q in ("listen_core", "journal_core", "qpl_core", "teen_consent", "older_digital", "continuity_core",
                  "safety_v3", "help_v3"):
            items = [p for p in plan if p["query_id"] == q]
            self.assertEqual(len(items), 6, q)
            for p in items:
                self.assertEqual((p["cap"], p["paged"]), ("all", True))
                self.assertTrue(p["key"].endswith(":all"))
                self.assertIn("pageSize=1000", p["url"])
                self.assertIn("cursorMark=%2A", p["url"])
        a11y = [p for p in plan if p["query_id"] == "a11y_core_v2"]
        self.assertTrue(all(p["key"] in p11 and p["cap"] == 200 for p in a11y))  # complete at 200: v1.1 request kept
        for q in G.CROSSREF_ID_QUERIES:
            p = next(p for p in plan if p["query_id"] == q)
            self.assertEqual((p["cap"], p["key"]), (1000, f"crossref:{q}:n1000"))
            self.assertIn("rows=1000", p["url"])
        en = next(p for p in plan if p["query_id"] == "listen_cr")
        self.assertIn(en["key"], p11)  # English Crossref queries keep 40 rows and their cache key
        self.assertEqual(G.run_for_dir("2026-10-26_search-1.2"), G.RUN_1_2)
        self.assertEqual(G.LATEST, G.RUN_1_2)
        self.assertNotEqual(G.RUN_1_2.audit_seed, G.RUN_1_1.audit_seed)

    @unittest.skipUnless((RUN11 / "manifest.json").exists(), "v3 gap search run v1.1 outputs not present")
    def test_v1_1_plan_keys_are_what_its_manifest_recorded(self):
        m = json.loads((RUN11 / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual([p["key"] for p in G.search_plan(FREEZE, G.RUN_1_1)], [r["key"] for r in m["requests"]])
        m10 = json.loads((RUN / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual([p["key"] for p in G.search_plan(FREEZE)], [r["key"] for r in m10["requests"]])

    def test_paged_fetch_follows_the_cursor_to_the_last_page(self):
        item = next(p for p in G.search_plan(FREEZE, G.RUN_1_2) if p["query_id"] == "listen_core")
        pages = {n: [epmc_result(f"{n}{i:04d}", f"Paper {n}-{i}") for i in range(size)]
                 for n, size in ((1, 1000), (2, 1000), (3, 300))}
        client = FakeClient(pages, hit_count=2300)
        with tempfile.TemporaryDirectory() as td:
            cache = F.Cache(td)
            new, err = G.fetch_paged(client, cache, item)
            self.assertEqual((new, err), (3, None))
            self.assertEqual([u.split("cursorMark=")[1].split("&")[0] for u in client.urls], ["%2A", "c-2", "c-3"])
            got, need = G.epmc_pages(cache, item)
            self.assertIsNone(need)
            self.assertEqual([k for k, _ in got], [item["key"], item["key"] + ":p2", item["key"] + ":p3"])
            rows, stats, missing = G.raw_hits(cache, [item])
            self.assertEqual((len(rows), missing), (2300, []))
            self.assertEqual((stats[0]["pages"], stats[0]["complete"], stats[0]["cap"]), (3, True, "all"))
            self.assertEqual([r[1] for r in rows[998:1002]], [999, 1000, 1001, 1002])  # ranks continue over pages
            table = G.retrieval_table(stats)[0]
            self.assertEqual((table["retrieved"], table["capped_requests"], table["pages"]), (2300, 0, 3))
            # a missing middle page is reported, never silently dropped
            cache.path(item["service"], item["key"] + ":p2").unlink()
            self.assertEqual(G.epmc_pages(cache, item)[1], (item["key"] + ":p2", "c-2"))
            self.assertEqual(G.raw_hits(cache, [item])[2], [item["key"] + ":p2"])
            new, err = G.fetch_paged(client, cache, item)  # refetch resumes from the stored cursor
            self.assertEqual((new, err), (1, None))

    def test_incomplete_bucket_counts_in_the_retrieval_table(self):
        stats = [{"family": "f", "source": "europepmc", "query_id": "q", "hit_count": 1500, "returned": 1000,
                  "cap": "all", "pages": 1, "complete": False}]
        row = G.retrieval_table(stats)[0]
        self.assertEqual(row["capped_requests"], 1)
        self.assertIn("1 of 1 incomplete", G.retrieval_markdown({"per_query_retrieval": [row]}))


class RunV12BuildScreenTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(tempfile.mkdtemp())
        make_repo(self.root)
        epmc = {}
        for run in (G.RUN_1_1, G.RUN_1_2):
            plan = G.search_plan(FREEZE, run)
            k_journal = next(p["key"] for p in plan if p["query_id"] == "journal_core" and "2024" in p["key"])
            k_help = next(p["key"] for p in plan if p["query_id"] == "help_v3" and "2025" in p["key"])
            epmc[run.run_version] = {
                k_journal: [epmc_result(str(500 + i), f"Expressive writing study number {i} for depression",
                                        doi=f"10.5555/w{i}", abstract=SECRET + " Expressive writing and depression.")
                            for i in range(30)] + [epmc_result("202", "Correction: Expressive writing app",
                                                               doi="10.5555/corr", pubtypes=("Published Erratum",))],
                k_help: [epmc_result("601", "Help-seeking for mental health among adolescents online", doi="10.5555/h",
                                     date="2025-05-05", abstract="Adolescents aged 15-17 years sought help online.")],
            }
        fill_cache(self.root, epmc["sela-v3-gap-search-v1.1"], run=G.RUN_1_1)
        fill_cache(self.root, epmc["sela-v3-gap-search-v1.2"], run=G.RUN_1_2)
        G.run_build(self.root, FREEZE, run=G.RUN_1_1)
        self.m = G.run_build(self.root, FREEZE, run=G.RUN_1_2)
        self.sm = G.run_screen(self.root, FREEZE, run=G.RUN_1_2)
        self.sd = self.root / G.OUT_BASE / G.RUN_1_2.dir_name(FREEZE) / "screening"

    def tearDown(self):
        shutil.rmtree(self.root, ignore_errors=True)

    def _sheet(self):
        with (self.sd / "fn_audit_sheet.csv").open(newline="") as f:
            return list(csv.DictReader(f))

    def _audit_all(self):
        rows = self._sheet()
        for r in rows:
            r.update(auditor="Dewi Lestari", audit_date="2026-11-10", auditor_relevance="unsure",
                     auditor_publication_kind="study_report")
        G.write_csv(self.sd / "fn_audit_sheet.csv", G.AUDIT_SHEET_FIELDS, rows)

    def test_manifest_versions_previous_runs_and_all_hit_queries(self):
        self.assertEqual(self.m["run_version"], "sela-v3-gap-search-v1.2")
        self.assertEqual(self.m["supersedes"]["run_version"], "sela-v3-gap-search-v1.1")
        self.assertEqual(self.m["sources"]["europepmc"]["per_query_caps"]["listen_core"], "all")
        self.assertEqual(self.m["sources"]["crossref"]["rows_per_query"]["id_pdp"], 1000)
        rows = read_candidates(self.sd.parent)
        w0 = next(r for r in rows if r["doi"] == "10.5555/w0")
        self.assertTrue(w0["in_run_1_1"])  # the same record's V3 id in run v1.1
        self.assertNotIn("in_run_1_0", w0)  # only earlier runs whose candidates file exists are annotated
        self.assertIn("in_run_1_1", self.m["counts"])
        prev = (G.OUT_BASE / G.RUN_1_1.dir_name(FREEZE) / G.CANDIDATES).as_posix()  # logical name, content hash
        self.assertEqual(self.m["inputs"][prev], S.content_sha256(self.root / prev))
        self.assertEqual(self.m["inputs"][prev], json.loads(
            (self.root / G.OUT_BASE / G.RUN_1_1.dir_name(FREEZE) / "manifest.json").read_text())["candidates_sha256"])
        self.assertEqual(self.sm["screening_version"], "sela-v3-tiab-screen-v1.2")
        self.assertEqual(G.run_verify(self.root, FREEZE, run=G.RUN_1_2), [])
        self.assertFalse((self.root / G.OUT_BASE / FREEZE).exists())  # v1.0 directory never written

    def test_audit_key_is_withheld_until_the_audit_is_complete(self):
        self.assertFalse((self.sd / "fn_audit_key.csv").exists())
        private = G.private_dir(self.root, FREEZE, G.RUN_1_2) / "fn_audit_key.csv"
        info = self.sm["withheld_outputs"]["fn_audit_key.csv"]
        self.assertEqual(info["sha256"], G._sha(private))
        self.assertNotIn("fn_audit_key.csv", self.sm["outputs"])
        self.assertNotIn("fn_audit_sheet.csv", self.sm["outputs"])
        self.assertIn(G.RUN_1_2.audit_seed, self.sm["audit_sample"]["selection"])
        # committing the key next to an unaudited sheet unblinds it: a violation
        shutil.copyfile(private, self.sd / "fn_audit_key.csv")
        self.assertTrue(any("before the audit is complete" in p for p in G.run_verify(self.root, FREEZE, run=G.RUN_1_2)))
        (self.sd / "fn_audit_key.csv").unlink()
        with self.assertRaises(SystemExit):
            G.release_audit_key(self.root, FREEZE, run=G.RUN_1_2)  # nothing audited yet
        # the auditor's entries do not break verify (the sheet is tracked by its machine columns)
        self._audit_all()
        self.assertEqual(G.run_verify(self.root, FREEZE, run=G.RUN_1_2), [])
        released = G.release_audit_key(self.root, FREEZE, run=G.RUN_1_2)
        self.assertEqual(G._sha(released), info["sha256"])
        self.assertEqual(G.run_verify(self.root, FREEZE, run=G.RUN_1_2), [])
        # a tampered released key is caught
        released.write_text(released.read_text().replace("deferred", "high", 1))
        self.assertTrue(any("differs from the hash recorded" in p for p in G.run_verify(self.root, FREEZE, run=G.RUN_1_2)))
        with self.assertRaises(SystemExit):
            G.release_audit_key(self.root, FREEZE, run=G.RUN_1_1)  # v1.1 is superseded, not released

    def test_machine_columns_of_the_sheet_cannot_change_and_screen_keeps_audits(self):
        self._audit_all()
        audited = (self.sd / "fn_audit_sheet.csv").read_bytes()
        G.run_screen(self.root, FREEZE, run=G.RUN_1_2)  # re-running screen never overwrites the auditor's entries
        self.assertEqual((self.sd / "fn_audit_sheet.csv").read_bytes(), audited)
        rows = self._sheet()
        rows[0]["title"] = "A different title"
        G.write_csv(self.sd / "fn_audit_sheet.csv", G.AUDIT_SHEET_FIELDS, rows)
        self.assertTrue(any("machine columns" in p for p in G.run_verify(self.root, FREEZE, run=G.RUN_1_2)))
        committed = "".join(committed_text(p) for p in self.sd.parent.rglob("*") if p.is_file())
        self.assertNotIn("SECRET ABSTRACT", committed)


@unittest.skipUnless((RUN12 / "manifest.json").exists(), "v3 gap search run v1.2 outputs not present")
class CommittedRunV12Tests(unittest.TestCase):
    def test_committed_run_v1_2_verifies_withholds_the_key_and_retrieves_all_gap_hits(self):
        self.assertEqual(G.run_verify(REPO, RUN12.name), [])
        self.assertFalse((RUN12 / "screening" / "fn_audit_key.csv").exists())
        m = json.loads((RUN12 / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(m["run_version"], G.RUN_1_2.run_version)
        table = {r["query_id"]: r for r in m["per_query_retrieval"]}
        for q in sorted(G.EPMC_ALL_1_2) + ["a11y_core_v2"]:
            self.assertEqual(table[q]["capped_requests"], 0, q)  # every bucket complete
            self.assertGreaterEqual(table[q]["share_retrieved"], 0.99, q)
        for q in G.CROSSREF_ID_QUERIES:
            self.assertEqual(table[q]["cap"], 1000, q)
            self.assertTrue(table[q]["indonesian_title_cue_by_rank_band"], q)
        sm = json.loads((RUN12 / "screening" / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(sm["withheld_outputs"]["fn_audit_key.csv"]["status"], "withheld_until_audit_complete")
        with (RUN12 / "screening" / "fn_audit_sheet.csv").open(encoding="utf-8", newline="") as f:
            self.assertEqual(csv.DictReader(f).fieldnames, G.AUDIT_SHEET_FIELDS)

    def test_committed_v1_2_candidates_are_metadata_only(self):
        for r in read_candidates(RUN12):
            self.assertNotIn("abstract", r)
            self.assertEqual(r["status"], "candidate_metadata_only_unreviewed")

    @unittest.skipUnless((REPO / G.CACHE_BASE / G.RUN_1_2.dir_name(FREEZE)).exists(), "local v1.2 cache not present")
    def test_no_cached_abstract_text_in_committed_v1_2_files(self):
        """LOCAL ONLY (needs the gitignored cache; skipped in CI): every abstract, 8-token runs."""
        assert_no_abstract_runs(self, G.RUN_1_2, RUN12)


if __name__ == "__main__":
    unittest.main()
