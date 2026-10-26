"""Offline tests for the R-9 claim check and the v3 evidence map (research audit R-8, R-9).

The checker logic runs on synthetic abstracts. The committed-output tests run in
CI without the abstract cache: labels, staleness, the adult/minor separation,
declared gaps and the generated document.
"""
import copy
import json
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
sys.path.insert(0, str(REPO / "scripts" / "research"))

import v3_evidence as VE  # noqa: E402

V3_FEATURES = {"listening_room", "journal", "check_in", "timeline", "memory_correction", "consultation_summary",
               "help_seeking", "sharing_and_revocation", "minors_mode", "accessibility", "older_adults",
               "indonesian_context", "safety", "privacy", "working_age_adults"}
# Review round 3: the help-seeking records the research audit lists (section 5).
HELP_SEEKING_CITED = {"R010", "R011", "R012", "R013", "R014", "R015", "R016", "R017", "R018", "R019", "R040", "R047",
                      "R088", "R098"}


def assertion(sources, terms, **kw):
    return dict({"id": "a1", "claim_part": "x", "sources": sources,
                 "terms": [{"label": t, "pattern": t} for t in terms]}, **kw)


class LanguageDisplayTests(unittest.TestCase):
    def test_language_cell_shows_basis_and_title_cue(self):
        def cand(lang, basis, cue, disagrees):
            return {"publication_language": lang, "publication_language_detail": {
                "basis": basis, "title_suggests_indonesian": cue, "title_cue_disagrees": disagrees}}
        self.assertEqual(VE.language_display(cand("en", "machine_inferred_only", True, True)),
                         "en (inferred; title suggests id)")
        self.assertEqual(VE.language_display(cand("en", "publisher_deposit", True, True)),
                         "en (deposited; title suggests id)")
        self.assertEqual(VE.language_display(cand("en", "machine_inferred_only", False, False)), "en (inferred)")
        self.assertEqual(VE.language_display(cand(None, "not_stated", True, False)), "not stated (title suggests id)")
        self.assertEqual(VE.language_display(cand(None, "not_stated", False, False)), "not stated")
        self.assertEqual(VE.language_display(cand("en", "curated", False, False)), "en")


class ClaimCheckLogicTests(unittest.TestCase):
    ABS = {"S1": {"abstract": "Help-seeking intention did not improve, but literacy improved."},
           "S2": {"abstract": "Stigma and cost were reported barriers."},
           "S3": {"abstract": None}}

    def check(self, a):
        return VE.check_assertion(a, self.ABS)

    def test_statuses(self):
        self.assertEqual(self.check(assertion(["S1"], ["literacy improved"]))[0], "terms_found")
        self.assertEqual(self.check(assertion(["S1"], ["literacy", "symptoms"]))[0], "partially_found")
        self.assertEqual(self.check(assertion(["S1"], ["symptoms"]))[0], "not_found")
        self.assertEqual(self.check(assertion(["S3"], ["anything"]))[0], "abstract_unavailable")
        contra = assertion(["S1"], ["literacy"], contradiction=[{"label": "null", "pattern": "did not improve"}])
        self.assertEqual(self.check(contra)[0], "contradiction_cue_found")
        self.assertEqual(self.check(assertion(["S1", "S2"], ["Indonesia"], expect="absent"))[0], "absent_as_expected")
        self.assertEqual(self.check(assertion(["S1", "S2"], ["stigma"], expect="absent", support_mode="all"))[0],
                         "unexpectedly_present")

    def test_support_modes(self):
        self.assertEqual(self.check(assertion(["S1", "S2"], ["stigma"]))[0], "terms_found")  # any
        self.assertEqual(self.check(assertion(["S1", "S2"], ["stigma"], support_mode="all"))[0], "not_found")
        best, per = self.check(assertion(["S1", "S2"], ["stigma"]))
        self.assertEqual(per["S1"]["status"], "not_found")
        self.assertEqual(len(per["S2"]["abstract_sha256"]), 64)
        self.assertNotIn("abstract", per["S2"])  # only a hash of the abstract is reported

    def test_terms_are_matched_in_the_title_as_well(self):
        """Audit fix (07-r11): R087's title says 'Termination Report'; its abstract does not repeat it."""
        abstracts = {"R087": {"title": "A chatbot trial for youth on waiting lists: Randomized Controlled Study "
                                       "Termination Report.",
                              "abstract": "All participants withdrew or were excluded; no evaluable data."}}
        best, per = VE.check_assertion(assertion(["R087"], [r"\bterminat\w*", "no evaluable data"]), abstracts)
        self.assertEqual(best, "terms_found")
        self.assertEqual(per["R087"]["matched_in_title"], [r"\bterminat\w*"])
        self.assertEqual(per["R087"]["searched"], ["title", "abstract"])
        abstracts["R087"]["abstract"] = None  # a title alone is not an abstract check
        self.assertEqual(VE.check_assertion(assertion(["R087"], [r"\bterminat\w*"]), abstracts)[0],
                         "abstract_unavailable")


@unittest.skipUnless((REPO / VE.CLAIM_OUT).exists(), "claim check output not present")
class CommittedClaimCheckTests(unittest.TestCase):
    def setUp(self):
        self.doc = json.loads((REPO / VE.CLAIM_OUT).read_text(encoding="utf-8"))
        self.spec = json.loads((REPO / VE.CLAIM_SPEC).read_text(encoding="utf-8"))

    def test_every_doc07_and_doc10_row_is_checked(self):
        emap = json.loads((REPO / VE.CORPUS / "evidence_map.json").read_text(encoding="utf-8"))
        rows07 = {e["row"] for e in emap["entries"] if e["source_doc"].endswith("07_EVIDENCE_TO_FEATURE.md")}
        rows10 = {e["row"] for e in emap["entries"] if e["source_doc"].endswith("10_EVIDENCE_APPRAISAL_AND_CLAIMS.md")}
        ids = {s["id"] for s in self.doc["statements"]}
        self.assertEqual({f"07-r{n:02d}" for n in rows07} - ids, set())
        self.assertEqual({f"10-g{n}" for n in rows10} - ids, set())
        cited = {r for e in emap["entries"] for r in e["r_ids"]}
        checked = {c for s in self.doc["statements"] for c in s["cited"]}
        self.assertEqual(cited - checked, set())
        self.assertIn("nature_medicine_2026_audit", self.doc["sources"])
        self.assertIn("not peer reviewed", self.doc["sources"]["stanford_news_2025"]["note"])

    def test_labels_mismatches_and_no_abstract_text(self):
        self.assertEqual(self.doc["status"], "machine_checked_unreviewed")
        self.assertFalse(self.doc["is_appraisal"])
        self.assertEqual(self.doc["spec_sha256"], VE._sha(REPO / VE.CLAIM_SPEC))
        statuses = [r["status"] for s in self.doc["statements"] for r in s["assertions"]]
        listed = {(m["statement"], m["assertion"]) for m in self.doc["mismatches"]}
        expected = {(s["id"], r["id"]) for s in self.doc["statements"] for r in s["assertions"]
                    if r["status"] not in VE.CONSISTENT}
        self.assertEqual(listed, expected)
        self.assertEqual(self.doc["counts"]["mismatches"], len(expected))
        self.assertIn("abstract_unavailable", statuses)  # R100 has no open abstract: recorded, not guessed
        for sid, src in self.doc["sources"].items():
            self.assertNotIn("abstract", src, sid)
        for s in self.doc["statements"]:
            for r in s["assertions"]:
                for per in r["per_source"].values():
                    self.assertEqual(set(per) - {"status", "matched_terms", "missing_terms", "contradiction_cues",
                                                 "abstract_sha256", "matched_in_title", "searched"}, set())

    def test_r087_termination_is_consistent_not_a_wording_issue(self):
        r11 = next(s for s in self.doc["statements"] if s["id"] == "07-r11")
        self.assertIsNone(r11["wording_issue"])
        self.assertEqual(r11["overall"], "terms_found")
        a4 = next(a for a in r11["assertions"] if a["id"] == "a4")
        self.assertEqual(a4["per_source"]["R087"]["matched_in_title"], ["termination"])
        self.assertEqual(self.doc["counts"]["wording_issues"], ["07-r01"])
        r05 = next(s for s in self.doc["statements"] if s["id"] == "07-r05")
        self.assertNotIn("(adults)", r05["agent_reading"])  # R047's undergraduates have no stated ages

    def test_world_psychiatry_review_is_register_record_r070(self):
        c2 = next(s for s in self.doc["statements"] if s["id"] == "10-c2")
        self.assertIn("R070", c2["cited"])
        self.assertNotIn("world_psychiatry_review", self.doc["sources"])


@unittest.skipUnless((REPO / VE.EMAP_OUT).exists(), "v3 evidence map not present")
class CommittedEvidenceMapTests(unittest.TestCase):
    def setUp(self):
        self.emap = json.loads((REPO / VE.EMAP_OUT).read_text(encoding="utf-8"))
        self.rows = {r["feature_id"]: r for r in self.emap["features"]}

    def test_one_row_per_v3_feature_with_gaps_and_boundaries(self):
        self.assertEqual(set(self.rows), V3_FEATURES)
        for fid, r in self.rows.items():
            self.assertTrue(r["claim_boundary"], fid)
            self.assertTrue(r["declared_gaps"], fid)
            self.assertEqual(r["status"], "machine_assembled_agent_drafted_unreviewed")
            for s in r["register_support"]:
                self.assertIn(s["relation"], ("direct", "adjacent", "indirect"))
                self.assertIn("age_band", s)
                self.assertIn("design_machine_label", s)

    def test_minors_row_never_uses_adult_evidence(self):
        recs = VE.records(REPO)
        for s in self.rows["minors_mode"]["register_support"]:
            self.assertIn(s["role"], VE.MINOR_ROLES, s["id"])
            if s["role"] == "supports_for_minors":  # only an accepted explicit 15-17 range of a primary study
                self.assertEqual(s["age_band"], "includes_15_17_explicit", s["id"])
                self.assertIsNotNone(recs[s["id"]]["age_range"], s["id"])
            self.assertTrue(s["minor_signal"], s["id"])
            self.assertFalse(s["adult_only"], s["id"])
        self.assertIn("Adult evidence", self.rows["minors_mode"]["minors_rule"])
        self.assertIn("0 register studies on adolescent consent", " ".join(self.rows["minors_mode"]["declared_gaps"]))

    def test_gap_features_are_declared(self):
        no_direct = set(self.emap["counts"]["features_without_direct_register_support"])
        for fid in ("journal", "consultation_summary", "accessibility", "minors_mode", "older_adults", "listening_room"):
            self.assertIn(fid, no_direct)

    def test_every_help_seeking_theme_record_is_mapped_or_deliberately_excluded(self):
        recs = VE.records(REPO)
        theme = {r for r, rec in recs.items() if rec["theme"] == "HELP_SEEKING"}
        self.assertEqual(len(theme), 10)
        row = self.rows["help_seeking"]
        cited = {s["id"] for s in row["register_support"]} | {s["id"] for s in row["counter_evidence"]}
        excluded = {e["id"] for e in row["theme_coverage"]["deliberately_excluded"]}
        self.assertEqual(theme - cited - excluded, set())
        self.assertEqual(row["theme_coverage"]["uncovered"], [])
        self.assertLessEqual(HELP_SEEKING_CITED, {s["id"] for s in row["register_support"]})
        self.assertEqual(set(self.emap["brief_topics"]["help-seeking"]["register_ids"]), HELP_SEEKING_CITED)

    def test_help_seeking_keeps_ages_apart_and_reads_mediation_as_association(self):
        row = self.rows["help_seeking"]
        by = {s["id"]: s for s in row["register_support"]}
        for rid in ("R014", "R015"):
            self.assertEqual(by[rid]["inference"], "mediation_on_cross_sectional_data", rid)
            self.assertIn("associations only, not causal", by[rid]["inference_label"])
        self.assertEqual(by["R016"]["inference"], "association_cross_sectional")
        groups = row["support_age_groups"]
        self.assertEqual(groups["includes_15_17_explicit"], ["R016"])  # 16-25, apart from adult-only evidence
        self.assertNotIn("R016", groups["adults_only"])
        for rid in ("R011", "R012", "R015", "R018", "R019", "R088", "R098"):
            self.assertIn(rid, groups["adults_only"], rid)
        for rid in ("R013", "R017", "R040", "R047"):
            self.assertIn(rid, groups["minors_signal_ages_unverified"], rid)
        boundary = row["claim_boundary"]
        self.assertIn("must not be generalised to minors", boundary)
        self.assertIn("cannot show causal pathways", boundary)
        self.assertIn("R047", {s["id"] for s in row["counter_evidence"]})

    def test_brief_topics_come_from_per_record_tags_not_the_union_of_rows(self):
        topics = self.emap["brief_topics"]
        for rid in ("R065", "R068", "R069", "R096"):  # clinician EHR summaries
            self.assertNotIn(rid, topics["continuity"]["register_ids"], rid)
            self.assertNotIn(rid, topics["help-seeking"]["register_ids"], rid)
            self.assertIn(rid, topics["consultation preparation"]["register_ids"], rid)
        for rid in ("R057", "R064", "R065", "R067", "R068", "R096"):  # memory-correction records
            self.assertNotIn(rid, topics["listening"]["register_ids"], rid)
        self.assertEqual(self.rows["memory_correction"]["brief_topics"], [])
        self.assertTrue(self.rows["memory_correction"]["brief_topics_note"])
        self.assertEqual(set(self.emap["brief_section_9_topics"]) - set(topics), set())
        for t in self.emap["brief_section_9_topics"]:
            self.assertEqual(topics[t]["rule"], "per-record topic tags", t)
            self.assertTrue(topics[t]["features"], t)

    def test_candidate_language_shows_its_basis_and_the_title_cue(self):
        shown = [q for r in self.rows.values() for q in r["v3_candidates_for_human_screening"]["top"][:5]]
        for q in shown:
            if q["title_cue_disagrees"]:
                self.assertIn("title suggests id", q["language_display"], q["candidate_id"])
            if q["publication_language_basis"] == "machine_inferred_only":
                self.assertIn("inferred", q["language_display"], q["candidate_id"])
        c = self.emap["counts"]
        self.assertEqual(c["candidates_shown_language_disagrees_with_indonesian_title"],
                         sorted({q["candidate_id"] for q in shown if q["title_cue_disagrees"]}))
        doc = (REPO / VE.EMAP_DOC).read_text(encoding="utf-8")
        self.assertIn("title suggests id", doc)

    def test_round_3_rewordings(self):
        spec = json.loads((REPO / VE.EMAP_SPEC).read_text(encoding="utf-8"))
        rows = {f["id"]: f for f in spec["features"]}
        safety_gaps = " ".join(rows["safety"]["declared_gaps"])
        self.assertIn("34 adolescents aged 15-19", safety_gaps)
        self.assertNotIn("qualitative synthesis", safety_gaps)
        r073 = next(c for c in rows["safety"]["counter_evidence"] if c["id"] == "R073")["reason"]
        self.assertIn("uncontrolled evaluation", r073)
        self.assertNotIn("feasible", r073)
        r092 = next(c for c in rows["check_in"]["counter_evidence"] if c["id"] == "R092")["reason"]
        self.assertIn("1,093", r092)
        r020 = next(c for c in rows["timeline"]["counter_evidence"] if c["id"] == "R020")["reason"]
        self.assertIn("Sela's design inference", r020)

    def test_verify_passes_and_doc_is_regenerated(self):
        self.assertEqual(VE.verify(REPO), [])
        doc = (REPO / VE.EMAP_DOC).read_text(encoding="utf-8")
        self.assertIn(f"evidence_map_v3.json sha256 `{VE._sha(REPO / VE.EMAP_OUT)}`", doc)
        self.assertIn("machine-assembled, agent-drafted, unreviewed", doc)


class EvidenceMapVerifyRuleTests(unittest.TestCase):
    """verify() on a scratch copy: adult evidence for minors, missing gaps and stale inputs are violations."""

    @classmethod
    def setUpClass(cls):
        cls.ok = (REPO / VE.EMAP_OUT).exists() and (REPO / VE.CLAIM_OUT).exists()

    def setUp(self):
        if not self.ok:
            self.skipTest("v3 evidence outputs not present")
        self.root = Path(tempfile.mkdtemp())
        for rel in (VE.EMAP_OUT, VE.CLAIM_OUT, VE.CLAIM_SPEC, VE.EMAP_SPEC, VE.CORPUS / "records.jsonl", VE.EMAP_DOC,
                    VE.GAP_QUEUE, VE.GAP_RUN / "candidates.jsonl.gz",
                    Path("docs/research/sela-relay/07_EVIDENCE_TO_FEATURE.md"),
                    Path("docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md")):
            dst = self.root / rel
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(REPO / rel, dst)
        self.assertEqual(VE.verify(self.root), [])

    def tearDown(self):
        shutil.rmtree(getattr(self, "root", ""), ignore_errors=True)

    def _edit(self, fn):
        p = self.root / VE.EMAP_OUT
        doc = json.loads(p.read_text(encoding="utf-8"))
        fn(doc)
        p.write_text(json.dumps(doc), encoding="utf-8")

    def test_adult_record_in_minors_row_is_a_violation(self):
        def add_adult(doc):
            row = next(r for r in doc["features"] if r["feature_id"] == "minors_mode")
            adult = next(s for r in doc["features"] for s in r["register_support"] if s["id"] == "R098")
            row["register_support"].append(dict(copy.deepcopy(adult), role="context_for_minors"))
        self._edit(add_adult)
        self.assertTrue(any("adult evidence mapped to minors" in p for p in VE.verify(self.root)))

    def test_review_or_conflicting_band_cannot_support_minors(self):
        """Audit fix: R071-like review eligibility and R031-like conflicting ranges are not 15-17 evidence."""
        def promote(doc):
            row = next(r for r in doc["features"] if r["feature_id"] == "minors_mode")
            for s in row["register_support"]:
                if s["id"] in ("R040", "R017"):  # review eligibility / conflicting ranges
                    s["role"] = "supports_for_minors"
        self._edit(promote)
        problems = VE.verify(self.root)
        for rid in ("R040", "R017"):
            self.assertTrue(any(f"{rid} supports minors without an accepted explicit" in p for p in problems), rid)

    def test_missing_declared_gap_is_a_violation(self):
        def drop(doc):
            next(r for r in doc["features"] if r["feature_id"] == "journal")["declared_gaps"] = []
        self._edit(drop)
        self.assertTrue(any("no direct support and no declared gap" in p for p in VE.verify(self.root)))

    def test_stale_inputs_and_hand_edited_doc_are_violations(self):
        rec = self.root / VE.CORPUS / "records.jsonl"
        rec.write_text(rec.read_text(encoding="utf-8") + "\n", encoding="utf-8")
        problems = VE.verify(self.root)
        self.assertTrue(any("records.jsonl changed" in p for p in problems))
        self._edit(lambda doc: doc.update(map_version="edited"))  # the JSON changes, the doc does not
        self.assertTrue(any("not regenerated" in p for p in VE.verify(self.root)))

    def test_candidates_input_is_the_content_of_the_compressed_file(self):
        """The map records the candidates under their logical name with the hash of the uncompressed content."""
        import gzip
        import hashlib
        name = (VE.GAP_RUN / "candidates.jsonl").as_posix()
        emap = json.loads((self.root / VE.EMAP_OUT).read_text(encoding="utf-8"))
        gz = self.root / VE.GAP_RUN / "candidates.jsonl.gz"
        content = gzip.decompress(gz.read_bytes())
        self.assertEqual(emap["inputs"][name], hashlib.sha256(content).hexdigest())
        gz.write_bytes(gzip.compress(content, compresslevel=1, mtime=0))  # other bytes, same content: still current
        self.assertEqual(VE.verify(self.root), [])
        gz.write_bytes(gzip.compress(content + b"\n", mtime=0))
        self.assertTrue(any(f"input {name} changed" in p for p in VE.verify(self.root)))
        gz.unlink()
        self.assertTrue(any(f"input {name} changed" in p for p in VE.verify(self.root)))

    def _edit_spec(self, fn):
        p = self.root / VE.EMAP_SPEC
        spec = json.loads(p.read_text(encoding="utf-8"))
        fn({f["id"]: f for f in spec["features"]}, spec)
        p.write_text(json.dumps(spec), encoding="utf-8")

    def test_untagged_record_in_a_multi_topic_row_is_a_violation(self):
        def drop(rows, spec):
            entry = next(e for e in rows["consultation_summary"]["register_support"] if e["id"] == "R065")
            del entry["topics"]
        self._edit_spec(drop)
        self.assertTrue(any("R065 needs topics" in p for p in VE.verify(self.root)))

    def test_help_seeking_record_neither_mapped_nor_excluded_is_a_violation(self):
        def drop(rows, spec):
            row = rows["help_seeking"]
            row["register_support"] = [e for e in row["register_support"] if e["id"] != "R014"]
        self._edit_spec(drop)
        self.assertTrue(any("HELP_SEEKING records neither mapped nor listed" in p and "R014" in p
                            for p in VE.verify(self.root)))

        def exclude(rows, spec):
            rows["help_seeking"]["theme_coverage"]["deliberately_excluded"] = [{"id": "R014", "reason": ""}]
        self._edit_spec(exclude)
        self.assertTrue(any("exclusion of R014 needs a reason" in p for p in VE.verify(self.root)))

    def test_unknown_inference_and_topic_are_violations(self):
        def bad(rows, spec):
            rows["help_seeking"]["register_support"][0]["inference"] = "causal_effect"
            rows["journal"]["brief_topics"] = ["journaling", "wellness"]
        self._edit_spec(bad)
        problems = VE.verify(self.root)
        self.assertTrue(any("unknown inference 'causal_effect'" in p for p in problems))
        self.assertTrue(any("'wellness' is not a brief section 9 topic" in p for p in problems))

    def test_brief_topics_built_as_a_union_are_a_violation(self):
        self._edit(lambda doc: doc["brief_topics"]["continuity"]["register_ids"].append("R065"))
        self.assertTrue(any("brief topic 'continuity'" in p for p in VE.verify(self.root)))

    def test_relabelling_as_appraisal_is_a_violation(self):
        p = self.root / VE.CLAIM_OUT
        doc = json.loads(p.read_text(encoding="utf-8"))
        doc["status"] = "reviewed"
        p.write_text(json.dumps(doc), encoding="utf-8")
        self.assertTrue(any("machine_*" in x for x in VE.verify(self.root)))


if __name__ == "__main__":
    unittest.main()
