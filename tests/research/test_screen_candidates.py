"""Offline contract and regression tests for metadata-only Sela screening."""
import csv
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[2] / "scripts" / "research"))
from screen_candidates import triage, screen, select_review_queue

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

    def test_duplicate_detection(self):
        with tempfile.TemporaryDirectory() as td:
            p=Path(td)/"x.jsonl"
            line=json.dumps({"title":"Mental health help-seeking","doi":"10.1/x","screening_status":"candidate_metadata_only_unreviewed"})+"\n"
            p.write_text(line*2)
            with self.assertRaises(ValueError):
                screen(p,Path(td))

if __name__=="__main__":
    unittest.main()
