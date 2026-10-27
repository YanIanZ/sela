"""Offline tests for corpus pipeline 1.2.0 (schema v3) and the frozen 1.1.0 snapshot.

Covers audit problems P1-P5 / tasks R-2, R-6, R-11: the legacy profile still
produces the exact pre-1.2.0 bytes, ``reproduce`` rebuilds a frozen snapshot
byte for byte (with frozen copies of inputs edited after the freeze), and the
schema-v3 fields (publication language, age range with locators, MeSH age
groups, age band, reviewer-promotable country evidence, study-type caution,
publication-type eligibility, needs_check documentation and the human-only
reviewer fields) behave as documented. No network.
"""
import hashlib
import io
import json
import shutil
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from urllib.error import HTTPError

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
sys.path.insert(0, str(REPO / "scripts" / "research"))
sys.path.insert(0, str(HERE))

import corpus_pipeline as P  # noqa: E402
import test_corpus_pipeline as T  # noqa: E402  (fixture cache + mini register)
from corpus import export as E  # noqa: E402
from corpus import fetch as F  # noqa: E402
from corpus import profiles as PF  # noqa: E402
from corpus import reviews as RV  # noqa: E402
from corpus import snapshots as SN  # noqa: E402
from corpus import v3fields as V3  # noqa: E402
from corpus import verify as V  # noqa: E402

FREEZE = T.FREEZE
CORPUS = REPO / "data" / "research" / "corpus"

# SHA-256 of the fixture build made with the pre-1.2.0 code (commit e9711af),
# recorded before any 1.2.0 change. The legacy profile must keep producing them.
LEGACY_FIXTURE_GOLDEN = {
    "eval_fixtures/claim_gates.json": "89f6f0312880d77c98f9dd60de685c9d0dfbec9a18e0098266c3c51b4889e81e",
    "evidence_map.json": "fd656ebfb721adeb00ba66a43dceecccda2b8441493aad8aa313a14dc2c293e5",
    "knowledge_base.jsonl": "b34ff2d627c65cd162b4d661744448a69b0a825b5b0031d53d513e8e8ddd4f67",
    "manifest.json": "5959b30442be45044b57ef96fc25b483c11d21eae7dec7a35c269f4988175202",
    "records.jsonl": "0f8715d5e2f8bb441b03eb7c0d4280cb085893a82e9563e95fd01a27d99c7ca2",
    "snapshots/2026-10-26/manifest.json": "5959b30442be45044b57ef96fc25b483c11d21eae7dec7a35c269f4988175202",
    "snapshots/2026-10-26/records.jsonl": "0f8715d5e2f8bb441b03eb7c0d4280cb085893a82e9563e95fd01a27d99c7ca2",
}
# The committed frozen snapshot of the real register (pipeline 1.1.0, 2026-10-26).
COMMITTED_LEGACY_SNAPSHOT = {
    "manifest.json": "56969da476ef1e250de0b1425f4ac8662f74fc62a8565aae5b111e7586a62e1f",
    "records.jsonl": "69d368bbf7efe06fecc8bf3d051c03df614353c7128ba11a6ccf1937dbb0c616",
}

# The 1.2.0 snapshot as rebuilt on 2026-10-26 after the age-band and claim-check corrections (never
# committed before that rebuild). Once committed it is frozen too: a change needs a new version.
COMMITTED_CURRENT_SNAPSHOT = {
    "manifest.json": "9d03c998410033215c610308ca523e5df9856a6199482db1d2ab65c0fe6c2153",
    "records.jsonl": "e7aaa751a31c7a0ab1651b50e14682a2232c5801f912fc4ee299272fb51a0e19",
}


def tree_hashes(root):
    root = Path(root)
    return {p.relative_to(root).as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in sorted(root.rglob("*")) if p.is_file()}


class V3Case(T.BuildCase):
    def recs(self, out_name="out"):
        return self.records(out_name)


class LegacyProfileTests(V3Case):
    def test_legacy_profile_is_byte_identical_to_pre_1_2_0_code(self):
        self.build(profile=PF.LEGACY)
        self.assertEqual(tree_hashes(self.tmp / "out"), LEGACY_FIXTURE_GOLDEN)
        rec = self.recs()["R001"]
        for field in ("publication_language", "age_range", "reviewer_name", "findings", "schema_version"):
            self.assertNotIn(field, rec)
        self.assertEqual(rec["pipeline_version"], "1.1.0")

    def test_profiles_and_snapshot_names(self):
        self.assertEqual(PF.LEGACY.snapshot_name(FREEZE), "2026-10-26")
        self.assertEqual(PF.CURRENT.snapshot_name(FREEZE), "2026-10-26_pipeline-1.2.0")
        self.assertIs(PF.snapshot_profile("2026-10-26"), PF.LEGACY)
        self.assertIs(PF.snapshot_profile("2026-10-26_pipeline-1.2.0"), PF.CURRENT)
        with self.assertRaises(ValueError):
            PF.for_version("0.9")

    def test_current_and_legacy_snapshots_coexist_and_verify(self):
        self.build(profile=PF.LEGACY)
        legacy = (self.tmp / "out" / "snapshots" / "2026-10-26" / "records.jsonl").read_bytes()
        m = self.build()  # current profile into the same out dir: a NEW snapshot, the old one untouched
        self.assertEqual(m["pipeline_version"], "1.2.0")
        self.assertEqual(m["snapshot"], "snapshots/2026-10-26_pipeline-1.2.0")
        self.assertEqual((self.tmp / "out" / "snapshots" / "2026-10-26" / "records.jsonl").read_bytes(), legacy)
        violations, _ = V.verify_outputs(self.tmp / "out", root=T.MINI)
        self.assertEqual(violations, [])
        # tampering with the frozen snapshot is caught even though the current outputs are fine
        p = self.tmp / "out" / "snapshots" / "2026-10-26" / "records.jsonl"
        p.write_bytes(p.read_bytes().replace(b"AI_SUPPORT", b"AI_SUPPORX"))
        violations, _ = V.verify_outputs(self.tmp / "out")
        self.assertTrue(any("snapshot 2026-10-26: records.jsonl sha256" in v for v in violations), violations)


class ReproduceTests(V3Case):
    def _repo_copy(self):
        root = self.tmp / "repo"
        shutil.copytree(T.MINI, root)
        return root

    def test_reproduce_uses_frozen_copy_of_an_input_edited_after_the_freeze(self):
        root = self._repo_copy()
        out = self.tmp / "committed"
        P.run_build(P.Inputs(root), self.cache_dir, out, FREEZE, profile=PF.LEGACY)
        doc10 = root / "docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md"
        original = doc10.read_bytes()
        report = P.run_reproduce(root, out, self.cache_dir, "2026-10-26", log=lambda m: None)
        self.assertTrue(report["identical"])
        self.assertEqual(set(report["inputs"].values()), {"live"})
        # a dated note is added to doc 10 after the freeze: without a frozen copy the snapshot is not reproducible
        doc10.write_bytes(original + b"\n> Superseded note (2026-10-26): test.\n")
        with self.assertRaises(P.NotReproducible):
            P.run_reproduce(root, out, self.cache_dir, "2026-10-26", log=lambda m: None)
        violations, _ = V.verify_outputs(out, root=root)
        self.assertTrue(any("no longer reproducible" in v for v in violations), violations)
        frozen = SN.frozen_path(root, "2026-10-26", "docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md")
        self.assertTrue(frozen.name.endswith(".md.frozen"))
        frozen.parent.mkdir(parents=True)
        frozen.write_bytes(original)
        report = P.run_reproduce(root, out, self.cache_dir, "2026-10-26", log=lambda m: None)
        self.assertTrue(report["identical"])
        self.assertEqual(report["inputs"]["docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md"],
                         "frozen copy")
        self.assertEqual(V.verify_outputs(out, root=root)[0], [])

    def test_reproduce_detects_a_different_rebuild(self):
        root = self._repo_copy()
        out = self.tmp / "committed"
        P.run_build(P.Inputs(root), self.cache_dir, out, FREEZE, profile=PF.LEGACY)
        snap = out / "snapshots" / "2026-10-26" / "records.jsonl"
        snap.write_bytes(snap.read_bytes() + b"\n")
        report = P.run_reproduce(root, out, self.cache_dir, "2026-10-26", log=lambda m: None)
        self.assertFalse(report["identical"])

    def test_reproduce_cli_and_current_snapshot(self):
        root = self._repo_copy()
        out = self.tmp / "committed"
        P.run_build(P.Inputs(root), self.cache_dir, out, FREEZE)
        buf = io.StringIO()
        with redirect_stdout(buf):
            code = P.main(["--root", str(root), "--out", str(out), "--cache", str(self.cache_dir), "reproduce",
                           "--snapshot", "2026-10-26_pipeline-1.2.0"])
        self.assertEqual(code, 0, buf.getvalue())
        self.assertIn("BYTE-IDENTICAL", buf.getvalue())
        with redirect_stderr(io.StringIO()), redirect_stdout(io.StringIO()):
            self.assertEqual(P.main(["--root", str(root), "--out", str(out), "--cache", str(self.cache_dir),
                                     "reproduce", "--snapshot", "2026-10-26"]), 1)  # no such snapshot


class CommittedSnapshotTests(unittest.TestCase):
    """Runs in CI without the cache: the frozen snapshot bytes and its inputs stay available."""

    def test_frozen_1_1_0_snapshot_bytes_are_unchanged(self):
        snap = CORPUS / "snapshots" / "2026-10-26"
        self.assertEqual({n: hashlib.sha256((snap / n).read_bytes()).hexdigest() for n in COMMITTED_LEGACY_SNAPSHOT},
                         COMMITTED_LEGACY_SNAPSHOT)

    def test_1_2_0_snapshot_bytes_are_pinned(self):
        snap = CORPUS / "snapshots" / "2026-10-26_pipeline-1.2.0"
        self.assertEqual({n: hashlib.sha256((snap / n).read_bytes()).hexdigest() for n in COMMITTED_CURRENT_SNAPSHOT},
                         COMMITTED_CURRENT_SNAPSHOT)

    def test_every_committed_snapshot_can_still_be_rebuilt(self):
        for d in sorted(p for p in (CORPUS / "snapshots").iterdir() if p.is_dir()):
            self.assertEqual(SN.check_snapshot_dir(d, PF.snapshot_profile, root=REPO), [], d.name)

    def test_committed_current_outputs_are_schema_v3(self):
        m = json.loads((CORPUS / "manifest.json").read_text(encoding="utf-8"))
        self.assertEqual(m["pipeline_version"], PF.CURRENT.pipeline_version)
        self.assertEqual(m["supersedes"]["snapshot"], "snapshots/2026-10-26")
        recs = {r["id"]: r for r in map(json.loads, (CORPUS / "records.jsonl").read_text(encoding="utf-8").splitlines())}
        self.assertEqual(sorted(r for r in recs if recs[r]["study_type_caution"]), ["R032", "R083"])
        for rid in ("R032", "R083"):
            self.assertIn("study_type_label_conflict", {f["id"] for f in recs[rid]["flags"]})
            self.assertEqual(recs[rid]["study_type"], "randomized_controlled_trial")  # machine label not hand-edited
            self.assertIsNone(recs[rid]["study_type_reviewer_override"])
        self.assertEqual(sorted(r for r in recs if recs[r]["needs_check_evidence"]), ["R002", "R013", "R065", "R100"])
        for r in recs.values():
            self.assertEqual(r["reviewer_status"], "not_reviewed")
            self.assertTrue(all(r[f] is None for f in V3.REVIEWER_FIELDS), r["id"])
            self.assertIn(r["publication_language"], ("en", None))


class SnapshotInputLineEndingTests(unittest.TestCase):
    """Audit fix: a checkout with core.autocrlf=true must not turn snapshot inputs into CRLF.

    Every input recorded by a committed snapshot manifest (and every frozen copy) must be pinned by
    .gitattributes (eol=lf, or -text for .frozen copies, which keeps their bytes), and the bytes on
    disk must still hash to what the manifest recorded."""

    def _inputs(self):
        out = set()
        for d in sorted(p for p in (CORPUS / "snapshots").iterdir() if p.is_dir()):
            m = json.loads((d / "manifest.json").read_text(encoding="utf-8"))
            out.update(m.get("inputs", {}))
        return sorted(out)

    def test_every_snapshot_input_is_pinned_to_lf(self):
        import subprocess
        try:
            res = subprocess.run(["git", "-C", str(REPO), "check-attr", "text", "eol", "--"] + self._inputs(),
                                 capture_output=True, text=True, check=True)
        except (OSError, subprocess.CalledProcessError):
            self.skipTest("git is not available or this is not a git work tree")
        attrs = {}
        for line in res.stdout.splitlines():
            path, attr, value = (x.strip() for x in line.rsplit(":", 2))
            attrs.setdefault(path, {})[attr] = value
        for rel in self._inputs():
            self.assertEqual(attrs[rel].get("eol"), "lf", f"{rel} is not pinned to LF: {attrs.get(rel)}")
            self.assertEqual(attrs[rel].get("text"), "set", rel)
        frozen = sorted(str(p.relative_to(REPO)) for p in (REPO / SN.FROZEN_INPUTS).rglob("*.frozen"))
        if frozen:
            res = subprocess.run(["git", "-C", str(REPO), "check-attr", "text", "--"] + frozen,
                                 capture_output=True, text=True, check=True)
            for line in res.stdout.splitlines():
                self.assertTrue(line.endswith(": text: unset"), line)  # -text: bytes are never converted

    def test_input_bytes_have_no_crlf_and_hashes_match(self):
        for d in sorted(p for p in (CORPUS / "snapshots").iterdir() if p.is_dir()):
            m = json.loads((d / "manifest.json").read_text(encoding="utf-8"))
            found, problems = SN.resolve_frozen_inputs(REPO, d.name, m.get("inputs", {}))
            self.assertEqual(problems, [], d.name)
            for rel, path in found.items():
                self.assertNotIn(b"\r\n", Path(path).read_bytes(), f"{d.name}: {rel}")


class RepositoryVerifyCommandTests(unittest.TestCase):
    def test_verify_command_on_the_repository_reports_no_violation(self):
        """The documented check `corpus_pipeline.py verify` (corpus, both snapshots' inputs, v3 outputs)."""
        buf = io.StringIO()
        with redirect_stdout(buf):
            code = P.main(["verify"])
        self.assertEqual(code, 0, buf.getvalue())
        self.assertIn("verify: 0 violation(s)", buf.getvalue())


class V3FieldTests(V3Case):
    def test_build_adds_v3_fields_and_keeps_machine_labels(self):
        m = self.build()
        recs = self.recs()
        r1 = recs["R001"]
        self.assertEqual(r1["schema_version"], "sela-corpus-schema-v3")
        self.assertEqual(r1["publication_language_detail"]["status"], "machine_copied_from_index_unreviewed")
        self.assertEqual(r1["country_evidence_fulltext"]["candidates"][0]["label"], "Indonesia")
        self.assertIsNone(r1["country_evidence_fulltext"]["promoted_labels"])
        self.assertEqual(r1["publication_type_eligibility"]["status"], "eligible")
        self.assertIsNone(r1["needs_check_evidence"])
        for r in recs.values():
            self.assertTrue(all(r[f] is None for f in V3.REVIEWER_FIELDS))
            self.assertEqual(V.validate_record(r, FREEZE, m["cutoff_date"], v3=True), [], r["id"])
        self.assertEqual(m["counts"]["v3"]["reviewer_status"], {"not_reviewed": 5})
        kb = [json.loads(x) for x in (self.tmp / "out" / "knowledge_base.jsonl").read_text().splitlines()]
        self.assertEqual(set(kb[0]), set(E.KB_FIELDS_V3))

    def test_needs_check_is_documented_not_resolved(self):
        self.build()
        recs = self.recs()
        r4, r5 = recs["R004"], recs["R005"]
        self.assertEqual(r4["screening_status"], "needs_check")
        nce = r4["needs_check_evidence"]
        self.assertEqual(nce["status"], "machine_documented_unreviewed")
        self.assertEqual(nce["resolution"], "unresolved_pending_reviewer")
        self.assertEqual(nce["earliest_day_precision_publication_candidate"], "2024-03-15")
        self.assertIn("landing_page.citation_online_date", {c["source"] for c in nce["candidate_dates"]})
        self.assertIsNone(r4["publication_date"])  # canonical-date rule unchanged
        self.assertIn("needs_check_documented", {f["id"] for f in r4["flags"]})
        self.assertEqual(r5["needs_check_evidence"]["landing_page"]["status"], "blocked")
        self.assertIsNone(r5["needs_check_evidence"]["earliest_day_precision_publication_candidate"])

    def test_publication_language_rules(self):
        v, d, flags = V3.publication_language({"language": "eng"}, {"language": "en"}, {"language": "it"})
        self.assertEqual((v, d["basis"], flags), ("en", "curated", []))
        self.assertTrue(d["notes"])  # the OpenAlex disagreement is recorded, not used
        v, d, flags = V3.publication_language({}, {"language": "id"}, None)
        self.assertEqual((v, d["basis"]), ("id", "publisher_deposit"))
        v, d, flags = V3.publication_language(None, None, {"language": "id"})
        self.assertEqual((v, flags), ("id", ["publication_language_inferred_only"]))
        v, d, flags = V3.publication_language({"language": "ind"}, {"language": "en"}, None)
        self.assertEqual((v, flags), ("id", ["publication_language_sources_disagree"]))
        self.assertEqual(V3.publication_language(None, None, None)[0], None)
        self.assertIsNone(V3.normalise_language("und"))
        self.assertEqual(V3.normalise_language("en-US"), "en")

    def test_age_range_patterns_locators_and_conflicts(self):
        def para(text, section="Methods > Participants", part="body", n=1):
            return {"part": part, "section": section, "paragraph": n, "text": text}
        url = "https://example.org/x"
        items = V3.extract_age_candidates([
            para("Participants aged 15-17 years were recruited from schools."),
            para("Adolescents aged between 15 and 17 years (mean age 16.1) took part.", n=2),
            para("Infants aged 6-18 months were excluded.", n=3),
            para("Young people (aged 15-30) rarely seek help.", section="Background", part="abstract"),
        ], url)
        self.assertEqual(sorted({i["value"] for i in items}), ["mean:16.1", "range:15-17"])
        self.assertTrue(all(i["locator"]["section"] == "Methods > Participants" for i in items))
        self.assertTrue(all(i["status"] == "machine_extracted_unreviewed" and i["snippet_sha256"] for i in items))
        value, detail = V3.age_range_detail(items, is_review=False)
        self.assertEqual(value, {"min": 15, "max": 17, "unit": "years", "kind": "range"})
        conflict = V3.extract_age_candidates([para("aged 18 years or older"), para("aged 18-25 years", n=2)], url)
        value, detail = V3.age_range_detail(conflict, is_review=False)
        self.assertIsNone(value)
        self.assertTrue(detail["note"].startswith("conflicting"))
        value, detail = V3.age_range_detail(items, is_review=True)
        self.assertIsNone(value)

    def test_age_band_keeps_adults_and_minors_apart(self):
        def c(v):
            return {"value": v}

        def band(values, mesh=None, pops=None, is_review=False):
            items = [c(v) for v in values]
            value, _ = V3.age_range_detail(items, is_review=is_review)
            return V3.age_band(items, mesh, pops, accepted=value, is_review=is_review)
        self.assertEqual(band(["range:12-18"])["summary"], "includes_15_17_explicit")
        self.assertEqual(band(["min:14"])["summary"], "includes_15_17_explicit")
        adult = band(["range:18-32"], ["Adolescent", "Young Adult"])
        self.assertEqual(adult["summary"], "adults_only_explicit")
        self.assertTrue(adult["notes"])  # MeSH Adolescent also covers 18-year-olds
        self.assertEqual(band([], ["Adolescent"])["summary"], "minors_possible_unverified")
        self.assertEqual(band([], None, ["young people or youth"])["summary"], "minors_possible_unverified")
        self.assertEqual(band([], ["Adult", "Middle Aged"])["summary"], "adults_indexed_or_named_only")
        self.assertEqual(band([])["summary"], "not_stated")
        older = band(["min:60"], ["Aged"], ["older adults"])
        self.assertTrue(older["signals"]["older_adults_50_plus"])
        self.assertEqual(band(["mean:21.7"])["summary"], "not_stated")
        # an explicit band comes only from the ACCEPTED age_range, never from raw candidates
        raw = V3.age_band([c("range:16-30")], None, None)  # a candidate that was not accepted
        self.assertEqual(raw["summary"], "minors_possible_unverified")  # a signal, never an explicit band
        conflict = band(["range:16-30", "range:18-30"], ["Adolescent", "Young Adult"])
        self.assertEqual(conflict["summary"], "conflicting_explicit_ranges")
        self.assertEqual(conflict["candidates_overlapping_15_17"], ["range:16-30"])
        self.assertEqual(band(["range:18-29", "range:18-64"])["summary"], "conflicting_explicit_ranges")
        # review-type records describe eligibility rules or included studies, not one sample
        self.assertEqual(band(["min:14"], is_review=True)["summary"], "review_eligibility_includes_minors")
        self.assertEqual(band(["min:18"], is_review=True)["summary"], "review_eligibility_adults_only")
        self.assertEqual(band([], ["Adolescent"], is_review=True)["summary"], "minors_possible_unverified")

    def _age_fields(self, paragraphs, is_review=False, mesh=None, pops=None):
        items = V3.extract_age_candidates(paragraphs, "https://example.org/x")
        value, detail = V3.age_range_detail(items, is_review=is_review)
        return items, value, detail, V3.age_band(items, mesh, pops, accepted=value, is_review=is_review)

    @staticmethod
    def _para(text, section="Methods > Participants and recruitment", part="body", n=1):
        return {"part": part, "section": section, "paragraph": n, "text": text}

    def test_r031_regression_advisory_group_ages_do_not_make_an_adolescent_band(self):
        """Regression for R031 (CC BY-NC-ND): a study sample of adults only, plus an advisory group whose
        age range overlaps 15-17 and is described in the sentence after the one that names the group.
        The text below is invented for this test: its wording, numbers and group name are not R031's;
        only the structure (adult sample range, a 'This panel ...' sentence with an overlapping range,
        a mean age) is reproduced."""
        items, value, detail, band = self._age_fields([
            self._para("Sampling was opportunistic. Anyone aged between 19 and 29 years who had used a campus "
                       "counselling service in the past year could enrol."),
            self._para("Interview prompts were piloted with the Peer Advisory Panel. This panel comprised "
                       "volunteers (aged 15-28) drawn from a community youth network.",
                       section="Methods > Data collection > Interviews", n=4),
            self._para("Twenty-two people (mean age = 23.4 years) completed the interviews.", section="Results",
                       part="abstract"),
        ], mesh=["Adolescent", "Adult", "Young Adult"], pops=["young adults"])
        self.assertEqual(sorted(i["value"] for i in items), ["mean:23.4", "range:15-28", "range:19-29"])
        advisory = next(i for i in items if i["value"] == "range:15-28")
        self.assertIn("non-sample group", advisory["non_voting"])
        self.assertEqual(detail["candidate_values"], ["range:19-29"])
        self.assertEqual([n["value"] for n in detail["non_voting_candidates"]], ["range:15-28"])
        self.assertEqual(value, {"min": 19, "max": 29, "unit": "years", "kind": "range"})
        self.assertEqual(band["summary"], "adults_only_explicit")
        self.assertNotIn("fulltext:range:15-28", band["signals"]["minors"])
        # without the group rule the same ranges conflict: still never an explicit 15-17 band
        plain = [dict(i) for i in items]
        for i in plain:
            i.pop("non_voting", None)
        v2, _ = V3.age_range_detail(plain, is_review=False)
        self.assertIsNone(v2)
        self.assertEqual(V3.age_band(plain, None, None, accepted=v2)["summary"], "conflicting_explicit_ranges")

    def test_non_sample_group_rule_keeps_sample_sentences_voting(self):
        reason = V3.non_sample_group_reason
        text = "Participants aged 15-17 were recruited with help from our youth advisory group."
        self.assertIsNone(reason(text, text.index("aged")))
        text = "Members of the steering committee (aged 40-60) reviewed the protocol."
        self.assertIsNotNone(reason(text, text.index("aged")))
        text = "We convened a Delphi panel. Panellists aged 15-25 years took part in three rounds."
        self.assertIsNone(reason(text, text.index("aged")))  # a plain panel can be the sample

    def test_r071_regression_review_eligibility_is_not_an_adolescent_sample(self):
        """R071 is a meta-analysis whose eligibility rule was 'aged 14 years or older' (synthetic text)."""
        items, value, detail, band = self._age_fields([
            self._para("Population: studies of individuals aged 14 years or older were eligible, with or without "
                       "symptoms of depression or anxiety.", section="Methods > Inclusion and Exclusion Criteria", n=2),
        ], is_review=True)
        self.assertEqual([i["value"] for i in items], ["min:14"])
        self.assertIsNone(value)
        self.assertTrue(detail["note"].startswith("review-type"))
        self.assertEqual(band["summary"], "review_eligibility_includes_minors")
        self.assertNotEqual(band["summary"], "includes_15_17_explicit")

    def test_verify_rejects_an_explicit_band_without_an_accepted_range(self):
        self.build()
        rec = self.recs()["R001"]
        forged = json.loads(json.dumps(rec))
        forged["age_range"] = None
        forged["age_band"]["summary"] = "includes_15_17_explicit"
        self.assertTrue(any("without an accepted age_range" in e for e in V.validate_record_v3(forged)))
        forged["age_range"] = {"min": 18, "max": 30, "unit": "years", "kind": "range"}
        self.assertTrue(any("disagrees with age_range" in e for e in V.validate_record_v3(forged)))

    def test_mesh_age_groups_are_copied(self):
        epmc = {"meshHeadingList": {"meshHeading": [{"descriptorName": "Adolescent"}, {"descriptorName": "Humans"},
                                                     {"descriptorName": "Young Adult"}]}}
        self.assertEqual(V3.age_groups_indexed(epmc), ["Adolescent", "Young Adult"])
        self.assertIsNone(V3.age_groups_indexed({}))

    def test_study_type_caution_for_index_only_trial_labels(self):
        def stype(primary, basis):
            return {"primary": primary, "labels": sorted({b["label"] for b in basis}), "basis": basis}
        epmc_rct = {"source": "europepmc.pubTypeList", "term": "Randomized Controlled Trial",
                    "label": "randomized_controlled_trial"}
        qual = {"source": "title", "term": "Qualitative", "label": "qualitative"}
        r032 = V3.study_type_caution(stype("randomized_controlled_trial", [epmc_rct, qual]),
                                     "Young People's Experiences: Lived Experience-Led Qualitative Study")
        self.assertEqual(r032["flag"], "study_type_label_conflict")
        self.assertEqual(r032["status"], "machine_flag_unreviewed")
        r083 = V3.study_type_caution(stype("randomized_controlled_trial", [epmc_rct]),
                                     "Demographic, clinical, and linguistic features associated with engagement")
        self.assertIn("association", r083["title_signals"][0])
        title_rct = {"source": "title", "term": "Randomized", "label": "randomized_trial"}
        self.assertIsNone(V3.study_type_caution(stype("randomized_controlled_trial", [epmc_rct, title_rct]),
                                                "Randomized Trial of a Chatbot, associated with outcomes"))
        self.assertIsNone(V3.study_type_caution(stype("randomized_controlled_trial", [epmc_rct]),
                                                "A chatbot for depression"))
        self.assertIsNone(V3.study_type_caution(stype("qualitative", [qual]), "A qualitative study"))

    def test_publication_type_exclusion_status(self):
        rec = V3.publication_type_eligibility({"pubTypeList": {"pubType": ["Published Erratum"]}}, {},
                                              "Correction: something")
        self.assertEqual(rec["status"], "excluded_publication_type")
        self.assertEqual(rec["rule_version"], "sela-pubtype-eligibility-v1")
        self.assertEqual(V3.publication_type_eligibility({}, {"publication_types_from_index": ["Journal Article"]},
                                                         "A cohort study")["status"], "eligible")


class ReviewerFieldTests(V3Case):
    REVIEW = {"id": "R001", "reviewer_name": "Dewi Lestari", "human_attestation": True, "review_date": "2026-11-02",
              "review_level": "full_text", "decision": "include", "appraisal_tool": "AXIS",
              "study_type_override": "cross_sectional", "publication_date_override": None,
              "country_promoted": ["Indonesia"], "findings": "Reviewer's own summary.", "notes": None}
    SECOND = {"reviewer_name": "Ai Nurhasanah", "human_attestation": True, "review_date": "2026-11-03",
              "decision": "include"}

    def _write_reviews(self, root, reviews):
        f = root / "data/research/register_inputs/reviewer_decisions.json"
        f.write_text(json.dumps({"schema": RV.SCHEMA, "reviews": reviews}), encoding="utf-8")

    def test_validation_refuses_machine_or_incomplete_reviews(self):
        ids = {"R001", "R002"}
        ok = RV.validate_entry(dict(self.REVIEW), ids)
        self.assertEqual(ok["reviewer_name"], "Dewi Lestari")
        self.assertIsNone(ok["review_disagreement"])  # single review
        for bad in ({"reviewer_name": "Claude (AI agent)"}, {"reviewer_name": "pipeline"}, {"reviewer_name": "Claude"},
                    {"reviewer_name": "Claude Opus 4"}, {"reviewer_name": "ChatGPT"}, {"reviewer_name": "gpt-4o"},
                    {"reviewer_name": "Gemini 1.5 Pro"}, {"reviewer_name": "DeepSeek V3"}, {"reviewer_name": "LLM"},
                    {"reviewer_name": "Full Name"}, {"reviewer_name": "Reviewer 1"}, {"reviewer_name": "AI assistant"},
                    {"human_attestation": False}, {"human_attestation": "yes"}, {"id": "R999"},
                    {"review_date": "26/10/2026"}, {"review_level": "abstract"}, {"decision": "maybe"},
                    {"review_level": "title_abstract"},  # findings need a full-text review
                    {"appraisal_tool": None},  # a full-text review must name its appraisal tool
                    {"publication_date_override": {"date": "2025-05-05"}}, {"surprise": 1}):
            with self.assertRaises(RV.ReviewError, msg=str(bad)):
                RV.validate_entry(dict(self.REVIEW, **bad), ids)
        no_attestation = dict(self.REVIEW)
        del no_attestation["human_attestation"]
        with self.assertRaises(RV.ReviewError):
            RV.validate_entry(no_attestation, ids)
        with self.assertRaises(RV.ReviewError):
            RV.load_reviews(json.dumps({"schema": RV.SCHEMA, "reviews": [self.REVIEW, self.REVIEW]}).encode(), ids)
        with self.assertRaises(RV.ReviewError):
            RV.load_reviews(json.dumps({"schema": "sela-register-review-v1", "reviews": []}).encode(), ids)

    def test_model_tool_and_placeholder_names_are_refused(self):
        """Review round 3: AI model names (including this project's own generator and verifier) and
        template placeholders that the first refusal list accepted."""
        ids = {"R001"}
        for name in ("Muse Glimmer", "Muse Spark Open", "Muse Spark", "Muse", "Llama", "Meta Llama",
                     "Meta Llama 3.1 70B", "Sonnet 4.5", "Opus 4.1", "Haiku 3", "Gemma 2", "gemma-2b", "Gemma",
                     "Kimi K2", "Kimi", "Codex", "OpenAI Codex", "Phi-3", "phi 4", "Cohere Command R",
                     "Command R", "Command R+", "Sela Verifier", "Sela AI", "Sela",
                     "Reviewer A", "reviewer b", "Reviewer Two", "Anonymous Reviewer", "Human Reviewer",
                     "Second Person", "Reviewer Name", "Dr. Jane Doe", "John Doe", "Jane Q. Doe"):
            with self.assertRaises(RV.ReviewError, msg=name):
                RV.validate_entry(dict(self.REVIEW, reviewer_name=name), ids)
            with self.assertRaises(RV.ReviewError, msg=f"reviewer_2 {name}"):
                RV.validate_entry(dict(self.REVIEW, reviewer_2=dict(self.SECOND, reviewer_name=name)), ids)

    def test_real_names_that_share_a_token_with_a_tool_are_accepted(self):
        ids = {"R001"}
        for name in ("Ai Nurhasanah", "Claude Martin", "Agen Siregar", "Model Wijaya", "Bot Rahayu",
                     "Gemma Collins", "Kimi Lindqvist", "Sela Ward", "Phi Nguyen", "Opus Daniel", "Muse Adeyemi"):
            self.assertEqual(RV.validate_entry(dict(self.REVIEW, reviewer_name=name), ids)["reviewer_name"], name)
        title_only = dict(self.REVIEW, review_level="title_abstract", appraisal_tool=None, findings=None)
        self.assertIsNone(RV.validate_entry(title_only, ids)["appraisal_tool"])

    def test_dual_review_disagreement_needs_a_third_person(self):
        ids = {"R001"}
        agreed = RV.validate_entry(dict(self.REVIEW, reviewer_2=dict(self.SECOND)), ids)
        self.assertEqual((agreed["review_disagreement"], agreed["final_decision"]), (False, "include"))
        disagree = dict(self.REVIEW, reviewer_2=dict(self.SECOND, decision="exclude"))
        with self.assertRaises(RV.ReviewError):
            RV.validate_entry(disagree, ids)  # no adjudication
        adj = {"adjudicated_by": "Rina Pratiwi", "human_attestation": True, "adjudication_date": "2026-11-05",
               "decision": "exclude", "notes": None}
        done = RV.validate_entry(dict(disagree, adjudication=adj), ids)
        self.assertEqual((done["review_disagreement"], done["final_decision"]), (True, "exclude"))
        for bad in (dict(disagree, adjudication=dict(adj, adjudicated_by="Dewi Lestari")),  # not a third person
                    dict(disagree, adjudication=dict(adj, human_attestation=False)),
                    dict(self.REVIEW, reviewer_2=dict(self.SECOND, reviewer_name="dewi  lestari")),  # same person
                    dict(self.REVIEW, reviewer_2=dict(self.SECOND, reviewer_name="Claude Sonnet")),
                    dict(self.REVIEW, reviewer_2=dict(self.SECOND), adjudication=adj)):  # nothing to adjudicate
            with self.assertRaises(RV.ReviewError):
                RV.validate_entry(bad, ids)

    def test_a_human_review_is_an_attributed_layer_over_machine_fields(self):
        root = self.tmp / "repo"
        shutil.copytree(T.MINI, root)
        self.build("before", inputs=P.Inputs(root))
        before = self.recs("before")["R001"]
        self._write_reviews(root, [self.REVIEW])
        m = self.build("after", inputs=P.Inputs(root))
        r = self.recs("after")["R001"]
        self.assertEqual((r["reviewer_name"], r["review_date"], r["review_decision"]),
                         ("Dewi Lestari", "2026-11-02", "include"))
        self.assertEqual(r["reviewer_status"], "human_reviewed_full_text")
        self.assertEqual(r["findings"], "Reviewer's own summary.")
        self.assertEqual(r["study_type_reviewer_override"], "cross_sectional")
        self.assertEqual(r["country_or_language_reviewer_promoted"], ["Indonesia"])
        self.assertEqual(r["country_evidence_fulltext"]["promoted_labels"], ["Indonesia"])
        for field in ("study_type", "country_or_language", "population", "sample_size", "publication_date"):
            self.assertEqual(r[field], before[field], field)  # machine fields untouched
        self.assertEqual(r["provenance"]["findings"]["source"], "human_reviewer")
        self.assertEqual(r["appraisal_tool"], "AXIS")
        self.assertIsNone(r["reviewer_2_name"])
        self.assertIsNone(r["review_disagreement"])
        self.assertEqual(m["counts"]["v3"]["reviewer_status"], {"human_reviewed_full_text": 1, "not_reviewed": 4})
        self.assertEqual(V.verify_outputs(self.tmp / "after")[0], [])
        # a reviewer field written by anything but the reviewer file is a violation
        forged = dict(before, findings="machine summary")
        self.assertTrue(any("no human review" in e for e in V.validate_record_v3(forged)))
        forged = dict(before, reviewer_status="human_reviewed_full_text", reviewer_name="X Y", review_date="2026-11-01",
                      review_decision="include")
        self.assertTrue(any("human reviewer file" in e for e in V.validate_record_v3(forged)))


class V3FetchTests(unittest.TestCase):
    PUBMED_XML = b"""<?xml version="1.0"?><PubmedArticleSet><PubmedArticle><MedlineCitation><PMID>123</PMID>
      <Article PubModel="Print-Electronic"><Journal><JournalIssue><PubDate><Year>2025</Year><Month>Jul</Month>
      </PubDate></JournalIssue></Journal><Abstract><AbstractText>SECRET ABSTRACT</AbstractText></Abstract>
      <ArticleDate DateType="Electronic"><Year>2025</Year><Month>05</Month><Day>05</Day></ArticleDate>
      <Language>eng</Language></Article></MedlineCitation><PubmedData><History>
      <PubMedPubDate PubStatus="entrez"><Year>2025</Year><Month>5</Month><Day>5</Day></PubMedPubDate>
      </History></PubmedData></PubmedArticle></PubmedArticleSet>"""

    def test_pubmed_history_keeps_dates_only(self):
        body = F.parse_pubmed_history(self.PUBMED_XML, "123")
        self.assertEqual(body["article_dates"], [{"type": "Electronic", "date": "2025-05-05"}])
        self.assertEqual(body["journal_issue_date"], "2025-07")
        self.assertEqual(body["history"], [{"status": "entrez", "date": "2025-05-05"}])
        self.assertNotIn("SECRET", json.dumps(body))
        self.assertIsNone(F.parse_pubmed_history(self.PUBMED_XML, "999"))

    def test_landing_meta_only_date_tags_and_tdm_reservation(self):
        page = (b'<html><head><meta name="citation_online_date" content="2025/08/07">'
                b'<meta name="citation_title" content="T"><meta name="tdm-reservation" content="1">'
                b'<meta property="article:published_time" content="2025-08-07T00:00:00Z"></head>'
                b'<body>full text that must not be cached</body></html>')
        meta, tdm = F.parse_landing_meta(page)
        self.assertEqual(meta, {"article:published_time": "2025-08-07T00:00:00Z", "citation_online_date": "2025/08/07"})
        self.assertEqual(tdm, "1")

    def test_blocked_landing_page_is_recorded_once_and_never_retried(self):
        calls = []

        def challenge(req, timeout):
            calls.append(req.full_url)
            raise HTTPError("u", 403, "Just a moment...", {}, None)
        client = F.Client(opener=challenge, sleep=lambda s: None, log=lambda m: None)
        with tempfile.TemporaryDirectory() as td:
            env, did = F.fetch_landing_meta(client, F.Cache(td), "10.1001/x")
            self.assertTrue(did)
            self.assertEqual((env["status"], env["http_status"]), ("blocked", 403))
            self.assertEqual(len(calls), 1)
            stored = "".join(p.read_text() for p in Path(td).rglob("*.json"))
            self.assertNotIn("Just a moment", stored)
            again, did_again = F.fetch_landing_meta(client, F.Cache(td), "10.1001/x")
            self.assertFalse(did_again)
            self.assertEqual(len(calls), 1)


if __name__ == "__main__":
    unittest.main()
