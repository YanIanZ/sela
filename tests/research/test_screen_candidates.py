"""Offline contract and regression tests for metadata-only Sela screening."""
import csv
import hashlib
import json
import re
import tempfile
import unittest
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[2] / "scripts" / "research"))
from screen_candidates import (triage, screen, select_review_queue, publication_type_eligibility,
                               RULESET_V10, RULESET_V11)

REPO = Path(__file__).resolve().parents[2]
DATA = REPO / "data" / "research"


def _fixture_pool(d):
    records = [
        {"title":"Mental health help-seeking among Indonesian students","first_publication_date":"2024-01-01","doi":"10.1/a","screening_status":"candidate_metadata_only_unreviewed","publication_type":"research-article; journal article"},
        {"title":"Correction: Mental health help-seeking and referral","first_publication_date":"2025-01-01","doi":"10.1/b","screening_status":"candidate_metadata_only_unreviewed","publication_type":"published erratum; correction"},
        {"title":"Mental health care navigation for students: study protocol for a randomized trial","first_publication_date":"2025-02-01","doi":"10.1/c","screening_status":"candidate_metadata_only_unreviewed","publication_type":"research-article; journal article"},
        {"title":"Chatbots for anxiety and depression, a, \"quoted\" title","first_publication_date":"2025-03-01","doi":"10.1/d","screening_status":"candidate_metadata_only_unreviewed","publication_type":"journal article"},
    ]
    f = d / "candidates.jsonl"
    f.write_text("".join(json.dumps(r)+"\n" for r in records))
    return f

class TriageTest(unittest.TestCase):
    def test_pathway_mental_health_is_high(self):
        r=triage("Barriers to mental health help-seeking and referral in Indonesian students")
        self.assertEqual(r["priority"],"high")
        self.assertIn("mental_health",r["signals"])
        self.assertIn("care_pathway",r["signals"])

    def test_go_nogo_not_clinical_grade(self):
        r=triage("Systematic review of AI chatbots for depression and anxiety")
        self.assertEqual(r["priority"],"high")
        self.assertIn("systematic_review",r["study_design_title_clues"])
        self.assertEqual(r["screening_status"],"automated_title_only_unreviewed")

    def test_urology_search_false_positive(self):
        r=triage("Innovation and AI in Endourology: An Overview")
        self.assertEqual(r["priority"],"deferred")
        self.assertTrue(r["out_of_domain_flag"])

    def test_protocol_is_not_trial_outcome(self):
        a=triage("Study protocol: mental health care navigation for students")
        b=triage("Mental health care navigation for students")
        self.assertLess(a["score"],b["score"])
        self.assertIn("protocol",a["study_design_title_clues"])

    def test_safer_cross_domain_method(self):
        r=triage("Large language models and hallucination in clinical summarization")
        self.assertEqual(r["priority"],"medium")
        self.assertEqual(r["family"],"cross_domain_ai_methods")

    def test_empty_or_generic_deferred(self):
        self.assertEqual(triage("A survey of unrelated agriculture")["priority"],"deferred")

    def test_round_trip_and_manifest(self):
        with tempfile.TemporaryDirectory() as td:
            d=Path(td)
            input_file=d/"candidates.jsonl"
            records=[
                {"title":"Mental health help-seeking among Indonesian students","first_publication_date":"2024-01-01","doi":"10.1/a","screening_status":"candidate_metadata_only_unreviewed"},
                {"title":"Chatbots for anxiety and depression","first_publication_date":"2025-01-01","doi":"10.1/b","screening_status":"candidate_metadata_only_unreviewed"},
                {"title":"AI innovation in endourology","first_publication_date":"2026-01-01","doi":"10.1/c","screening_status":"candidate_metadata_only_unreviewed"},
            ]
            input_file.write_text("".join(json.dumps(r)+"\n" for r in records))
            m=screen(input_file,d,limit=2)
            self.assertEqual(m["source_count"],3)
            self.assertEqual(sum(m["priority_counts"].values()),3)
            self.assertEqual(m["queue_count"],2)
            with (d/"sela_screening_index.csv").open() as f:
                rows=list(csv.DictReader(f))
            self.assertEqual(len(rows),3)
            self.assertEqual(rows[2]["priority"],"deferred")
            raw=(d/"sela_priority_review_queue.jsonl").read_bytes()
            self.assertEqual(hashlib.sha256(raw).hexdigest(),m["queue_sha256"])
            self.assertEqual(len(raw.splitlines()),2)
            with (d/"sela_human_review_worksheet.csv").open(newline="") as f:
                review=list(csv.DictReader(f))
            self.assertEqual(len(review),2)
            self.assertTrue(all(r["abstract_screening"]=="NOT_REVIEWED" for r in review))
            self.assertTrue(all(r["rag_ready"]=="NO" for r in review))
            self.assertEqual(m["review_worksheet_count"],2)

    def test_duplicate_detection(self):
        with tempfile.TemporaryDirectory() as td:
            p=Path(td)/"x.jsonl"
            line=json.dumps({"title":"Mental health help-seeking","doi":"10.1/x","screening_status":"candidate_metadata_only_unreviewed"})+"\n"
            p.write_text(line*2)
            with self.assertRaises(ValueError):
                screen(p,Path(td))


class EligibilityTest(unittest.TestCase):
    """v1.1: protocols, corrections/errata, editorials and retraction notices leave the queue (audit P5)."""

    def test_publication_type_tokens(self):
        e = publication_type_eligibility("published erratum; correction", "Correction: some study")
        self.assertEqual(e["status"], "excluded_publication_type")
        self.assertEqual(e["excluded_kinds"], ["correction_or_erratum"])
        self.assertIn({"kind": "correction_or_erratum", "source": "publication_type", "term": "correction"}, e["basis"])
        self.assertEqual(publication_type_eligibility("clinical trial protocol; journal article", "X")["excluded_kinds"],
                         ["protocol"])
        self.assertEqual(publication_type_eligibility("editorial", "Mental health matters")["excluded_kinds"], ["editorial"])
        self.assertEqual(publication_type_eligibility("retracted publication; journal article", "X")["excluded_kinds"],
                         ["retraction_notice_or_retracted"])

    def test_title_cues_are_a_fallback(self):
        for title in ("User experience and safety of AI chatbots: Scoping review protocol.",
                      "Help-seeking in adolescents: a qualitative systematic review protocol.",
                      "Digital care: Protocol for a Pilot Feasibility Randomized Trial",
                      "Protocol for a cluster randomized controlled trial among young people",
                      "Effects of chatbots in Asia: protocol for a systematic review and meta-analysis"):
            self.assertEqual(publication_type_eligibility("research-article; journal article", title)["excluded_kinds"],
                             ["protocol"], title)
        self.assertEqual(publication_type_eligibility("journal article", "Editorial: AI in psychiatry")["excluded_kinds"],
                         ["editorial"])

    def test_research_articles_that_mention_these_words_stay_eligible(self):
        for title in ("Editorial stances on large language models in nursing publications: a cross-sectional analysis",
                      "Transdiagnostic treatment with the Unified Protocol for adolescents",
                      "Correcting misinformation about depression on social media",
                      "Retraction rates in psychiatry journals: a bibliometric study"):
            self.assertEqual(publication_type_eligibility("research-article; journal article", title)["status"],
                             "eligible", title)

    def test_v11_queue_excludes_ineligible_and_index_keeps_every_row(self):
        with tempfile.TemporaryDirectory() as td:
            d = Path(td)
            m = screen(_fixture_pool(d), d, limit=10)
            self.assertEqual(m["screening_version"], RULESET_V11)
            self.assertEqual(m["eligibility_counts"], {"eligible": 2, "excluded_publication_type": 2})
            with (d/"sela_screening_index.csv").open(newline="", encoding="utf-8") as f:
                rows = list(csv.DictReader(f))
            self.assertEqual(len(rows), 4)  # criterion 1: every row keeps exactly one priority
            self.assertEqual({r["priority"] for r in rows} <= {"high", "medium", "deferred"}, True)
            by = {r["doi"]: r for r in rows}
            self.assertEqual(by["10.1/b"]["eligibility"], "excluded_publication_type")
            self.assertIn("correction_or_erratum:publication_type:published erratum", by["10.1/b"]["eligibility_basis"])
            self.assertIn("protocol:title:", by["10.1/c"]["eligibility_basis"])
            queue = [json.loads(x) for x in (d/"sela_priority_review_queue.jsonl").read_text().splitlines()]
            self.assertTrue(queue)
            self.assertTrue(all(q["screening"]["eligibility"]["status"] == "eligible" for q in queue))
            self.assertNotIn("10.1/b", {q["doi"] for q in queue})
            self.assertNotIn("10.1/c", {q["doi"] for q in queue})


class LineEndingAndVersionTest(unittest.TestCase):
    """Audit P1: v1.1 hashes the LF bytes git stores; v1.0 is reproducible but hashed CRLF."""

    def test_v11_writes_lf_and_manifest_hashes_bytes_on_disk(self):
        with tempfile.TemporaryDirectory() as td:
            d = Path(td)
            m = screen(_fixture_pool(d), d, limit=10)
            for name, key in (("sela_screening_index.csv", "index_sha256"),
                              ("sela_human_review_worksheet.csv", "review_worksheet_sha256"),
                              ("sela_priority_review_queue.jsonl", "queue_sha256")):
                raw = (d/name).read_bytes()
                self.assertNotIn(b"\r", raw, name)
                self.assertEqual(hashlib.sha256(raw).hexdigest(), m[key], name)
            self.assertEqual(m["line_terminator"], "LF")
            self.assertEqual(m["supersedes"]["screening_version"], RULESET_V10)

    def test_v10_ruleset_reproduces_crlf_outputs(self):
        with tempfile.TemporaryDirectory() as td:
            d = Path(td)
            m = screen(_fixture_pool(d), d, limit=10, ruleset=RULESET_V10)
            raw = (d/"sela_screening_index.csv").read_bytes()
            self.assertIn(b"\r\n", raw)
            self.assertEqual(hashlib.sha256(raw).hexdigest(), m["index_sha256"])
            self.assertNotEqual(hashlib.sha256(raw.replace(b"\r\n", b"\n")).hexdigest(), m["index_sha256"])
            self.assertNotIn("eligibility_counts", m)
            with self.assertRaises(ValueError):
                screen(_fixture_pool(d), d, limit=10, ruleset="sela-title-screen-v9")

    def test_committed_outputs_match_their_manifest(self):
        m = json.loads((DATA/"sela_screening_manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(m["screening_version"], RULESET_V11)
        for name, key in (("sela_screening_index.csv", "index_sha256"),
                          ("sela_human_review_worksheet.csv", "review_worksheet_sha256"),
                          ("sela_priority_review_queue.jsonl", "queue_sha256"),
                          ("sela_research_candidates_7580.jsonl", "source_sha256")):
            self.assertEqual(hashlib.sha256((DATA/name).read_bytes()).hexdigest(), m[key], name)
        self.assertEqual(m["queue_count"], 400)
        queue = [json.loads(x) for x in (DATA/"sela_priority_review_queue.jsonl").read_text(encoding="utf-8").splitlines()]
        self.assertTrue(all(q["screening"]["eligibility"]["status"] == "eligible" for q in queue))
        self.assertFalse([q["title"] for q in queue if q["title"].lower().startswith("correction")])

    def test_real_pool_rebuilds_both_versions_byte_for_byte(self):
        """v1.1 reproduces the committed files; v1.0 reproduces its frozen manifest (CRLF hashes)."""
        pool = DATA / "sela_research_candidates_7580.jsonl"
        frozen = (DATA / "screening_history" / "sela_screening_manifest_v1.0.json").read_bytes()
        with tempfile.TemporaryDirectory() as td:
            d = Path(td)
            screen(pool, d / "v11", limit=400)
            for name in ("sela_screening_index.csv", "sela_human_review_worksheet.csv",
                         "sela_priority_review_queue.jsonl", "sela_screening_manifest.json"):
                self.assertEqual((d/"v11"/name).read_bytes(), (DATA/name).read_bytes(), name)
            screen(pool, d / "v10", limit=400, ruleset=RULESET_V10)
            self.assertEqual((d/"v10"/"sela_screening_manifest.json").read_bytes(), frozen)
            v10 = json.loads(frozen)
            idx = (d/"v10"/"sela_screening_index.csv").read_bytes()
            self.assertEqual(hashlib.sha256(idx).hexdigest(), v10["index_sha256"])

    def test_gitattributes_pins_lf(self):
        text = (DATA / ".gitattributes").read_text(encoding="utf-8")
        for pattern in ("*.csv", "*.json", "*.jsonl"):
            self.assertRegex(text, rf"(?m)^{re.escape(pattern)}\s+text\s+eol=lf\s*$")


if __name__=="__main__":
    unittest.main()
