"""Tests for the leak check (review round 3): no committed file may carry a verbatim run of 8 or more
tokens from an abstract or full text that the project may not redistribute.

The rule tests use synthetic text written for these tests. The committed-fingerprint tests run in CI
(no cache). The exact checks against the real abstracts and full texts need the local, gitignored caches,
so they are skipped in CI: run them locally before every commit (see CORPUS_PIPELINE.md).
"""
import json
import re
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
sys.path.insert(0, str(REPO / "scripts" / "research"))

import leak_check as LC  # noqa: E402
import v3_evidence as VE  # noqa: E402

PROTECTED = ("Participants described how the waiting room felt cold and unwelcoming, and several said that "
             "the leaflets on the wall made them feel judged before anyone spoke to them.")


def src(text, scope="all", sid="S1"):
    return LC.Source(sid, "abstract", "CC-BY-NC-ND-4.0", scope, text)


class TokenAndRunTests(unittest.TestCase):
    def test_tokens_case_fold_and_drop_identifiers(self):
        self.assertEqual(LC.tokens("See https://doi.org/10.1/x and DOI 10.5555/abc.d, PMC123 Mixed-Case"),
                         ["see", "and", "doi", "mixed", "case"])

    def test_an_eight_token_run_is_a_leak_and_seven_is_not(self):
        eight = "the leaflets on the wall made them feel"
        seven = "the leaflets on the wall made them"
        files = {"agent_written": {"a.md": f"Our note: {eight}.", "b.md": f"Our note: {seven} uneasy."}}
        leaks = LC.find_leaks([src(PROTECTED)], files)
        self.assertEqual([(lk.path, lk.run_tokens) for lk in leaks], [("a.md", 8)])

    def test_longest_run_is_reported(self):
        run = "described how the waiting room felt cold and unwelcoming and several said"
        leaks = LC.find_leaks([src(PROTECTED)], {"agent_written": {"a.md": run}})
        self.assertEqual(leaks[0].run_tokens, len(LC.tokens(run)))

    def test_titles_and_redistributable_text_are_exempt(self):
        text = "the leaflets on the wall made them feel judged"
        files = {"agent_written": {"a.md": text}}
        self.assertEqual(LC.find_leaks([src(PROTECTED)], files, allowed=LC.shingles(LC.tokens(text))), [])

    def test_scope_redistributable_text_is_checked_only_in_agent_written_files(self):
        files = {"machine_outputs": {"records.jsonl": PROTECTED}, "agent_written": {"a.md": "unrelated words"}}
        self.assertEqual(LC.find_leaks([src(PROTECTED, scope="agent_written")], files), [])
        self.assertEqual(len(LC.find_leaks([src(PROTECTED, scope="all")], files)), 1)

    def test_statistics_and_reviewed_coincidences_are_not_text(self):
        stats = "odds ratio OR 0.34, 95% CI 0.13-0.88, p = .026"
        self.assertFalse(LC.is_text_run(" ".join(LC.tokens("OR 0.34, 95% CI 0.13-0.88"))))
        self.assertEqual(LC.find_leaks([src("We found OR 0.34, 95% CI 0.13-0.88, p = .026 overall.")],
                                       {"agent_written": {"a.md": stats}}), [])
        for (path, digest), reason in LC.REVIEWED_COINCIDENCES.items():
            self.assertRegex(digest, r"^[0-9a-f]{64}$")
            self.assertTrue((REPO / path).exists(), path)
            self.assertGreater(len(reason), 60, path)  # every reviewed exception states why

    def test_compressed_machine_outputs_are_checked_as_text(self):
        """The gap-search candidates are committed as .gz; the check reads their decompressed text."""
        from corpus import gzstore as S
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            run = root / "data/research/v3_gap_search/2026-10-26_search-9.9"
            run.mkdir(parents=True)
            (run / "candidates.jsonl.gz").write_bytes(S.gzip_bytes((json.dumps({"title": PROTECTED}) + "\n").encode()))
            files = LC.committed_files(root)
            rel = "data/research/v3_gap_search/2026-10-26_search-9.9/candidates.jsonl.gz"
            self.assertIn(PROTECTED, files["machine_outputs"][rel])
            self.assertEqual([lk.path for lk in LC.find_leaks([src(PROTECTED)], files)], [rel])
            # and a committed candidate title is metadata: exempt
            self.assertEqual(LC.find_leaks([src(PROTECTED)], files, LC.metadata_shingles(root)), [])

    def test_paraphrase_passes(self):
        para = "People said the waiting area felt unfriendly and that posters made them feel judged early."
        self.assertEqual(LC.find_leaks([src(PROTECTED)], {"agent_written": {"a.md": para}}), [])


class FingerprintRuleTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(tempfile.mkdtemp())
        (self.root / "docs/research").mkdir(parents=True)
        (self.root / LC.CORPUS).mkdir(parents=True)
        hashes = sorted({LC.shingle_hash(s) for s in LC.shingles(LC.tokens(PROTECTED))})
        doc = {"version": LC.FINGERPRINT_VERSION, "run_tokens": LC.RUN_TOKENS,
               "sources": {"R999": {"kind": "abstract", "licence": "CC-BY-NC-ND-4.0", "runs": len(hashes),
                                    "text_sha256": "0" * 64, "run_hashes": hashes}}}
        (self.root / LC.FINGERPRINTS).write_bytes(LC.fingerprint_bytes(doc))

    def tearDown(self):
        shutil.rmtree(self.root, ignore_errors=True)

    def test_fingerprints_catch_a_copied_run_without_the_text(self):
        (self.root / "docs/research/NOTE.md").write_text("A row line: several said that the leaflets on the wall "
                                                         "made them feel judged.", encoding="utf-8")
        problems = LC.check_fingerprints(self.root)
        self.assertEqual(len(problems), 1)
        self.assertIn("R999", problems[0])
        self.assertNotIn("leaflets", problems[0])  # the report names the source, never the text
        (self.root / "docs/research/NOTE.md").write_text("A row line: people felt judged by posters.", encoding="utf-8")
        self.assertEqual(LC.check_fingerprints(self.root), [])

    def test_fingerprint_file_holds_no_text(self):
        raw = (self.root / LC.FINGERPRINTS).read_text(encoding="utf-8")
        self.assertNotIn("leaflets", raw)


@unittest.skipUnless((REPO / LC.FINGERPRINTS).exists(), "leak fingerprints not committed")
class CommittedFingerprintTests(unittest.TestCase):
    """Runs in CI without the cache: the agent-written files carry no fingerprinted run."""

    def setUp(self):
        self.doc = json.loads((REPO / LC.FINGERPRINTS).read_text(encoding="utf-8"))

    def test_structure_is_hashes_only(self):
        self.assertEqual(self.doc["version"], LC.FINGERPRINT_VERSION)
        self.assertEqual(self.doc["run_tokens"], 8)
        hexes = re.compile(r"^[0-9a-f]{%d}$" % LC.HASH_CHARS)
        for sid, info in self.doc["sources"].items():
            self.assertEqual(set(info), {"kind", "licence", "text_sha256", "runs", "run_hashes"}, sid)
            self.assertTrue(all(hexes.match(h) for h in info["run_hashes"]), sid)
            self.assertEqual(info["runs"], len(info["run_hashes"]))
        # R031 (CC BY-NC-ND) is protected; its abstract was the source of the round-2 leak
        self.assertIn("R031", self.doc["sources"])
        self.assertIn("NC-ND", self.doc["sources"]["R031"]["licence"])
        self.assertGreater(self.doc["sources"]["R031"]["runs"], 100)
        recs = VE.records(REPO)
        for sid in self.doc["sources"]:
            if sid in recs:
                self.assertFalse(LC._redistributable(recs[sid]), sid)  # CC BY abstracts are not fingerprinted

    def test_committed_agent_written_files_carry_no_protected_abstract_run(self):
        self.assertEqual(LC.check_fingerprints(REPO), [])


@unittest.skipUnless((REPO / VE.CACHE).exists() and (REPO / LC.FULLTEXT_CACHE / "fulltext").exists(),
                     "LOCAL ONLY: register abstract and full-text caches not present (always skipped in CI)")
class LocalExactLeakTests(unittest.TestCase):
    """The exact check needs the gitignored caches. It is a required step before committing."""

    def test_no_register_abstract_or_protected_full_text_run_in_committed_files(self):
        leaks, cc_by, summary = LC.run_check(REPO, include_gap_search=False)
        self.assertEqual([(lk.source_id, lk.kind, lk.path, lk.run_tokens) for lk in leaks], [])
        self.assertGreaterEqual(summary["sources_checked"].get("abstract:protected", 0)
                                + summary["sources_checked"].get("abstract:redistributable", 0), 100)
        self.assertGreater(summary["sources_checked"].get("fulltext:protected", 0), 10)

    def test_committed_fingerprints_are_current(self):
        self.assertEqual(LC.fingerprint_bytes(LC.build_fingerprints(REPO)), (REPO / LC.FINGERPRINTS).read_bytes(),
                         "run `leak_check.py --write-fingerprints` and commit the file")


if __name__ == "__main__":
    unittest.main()
