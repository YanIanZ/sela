"""Offline tests for the Sela reading library, content set sela-reading-v1 (no network).

Run: python3 -m unittest discover -s tests/content -p 'test_*.py'

The validator (reading_validator.py) holds every rule; these tests run it on the committed
data, pin the facts that the content set must keep (30 v1 cards, 23 of them with unchanged text
and 7 changed cards that say so, 22 WHO cards with the licence URI, source link, notice and the
B-14 blocker, the LICENSE.md notice, adult-only audience for adult material, the WHO URL fix, the
six-step safety plan, a source claim for every summary or "why" that names a source, a verbatim
check for every adapted or paraphrased source, the repository licence scan, every item
machine_drafted_unreviewed), and mutate copies of real items to prove that each rule fails when it
should.
"""
import copy
import json
import re
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import reading_validator as RV  # noqa: E402
import verbatim_overlap as VO  # noqa: E402


def _context():
    schema, set_doc, sources, files = RV.load_library()
    return {
        "schema": schema, "set": set_doc, "sources": sources, "files": files,
        "sources_by_id": {s["id"]: s for s in sources["sources"]},
        "gate_sets": RV.load_gate_sets(),
        "v1": RV.load_v1_cards(),
        "help": RV.help_catalogue_by_id(),
    }


CTX = _context()
ITEMS = {item["id"]: (path, item) for path, item in RV.all_items(CTX["files"])}
# Migrated cards whose text was changed on purpose, with the version that changed it: the six-step
# safety plan in set 1.1.0, and six cards whose "why" credited its source with something the source
# does not say in set 1.2.0.
WHY_REWORDED = ("gerak-tubuh-setelah-kerja", "mulai-bergerak-lagi-sedikit-demi-sedikit", "jadwal-tidur-konsisten",
                "khawatir-soal-tubuh-dan-kesehatan", "bersikap-baik-pada-diri-sendiri", "kesepian-langkah-kecil-terhubung")
CHANGED_V1 = {"rencana-keselamatan-enam-langkah": 2, **{i: 2 for i in WHY_REWORDED}}
SIX_STEPS = "rencana-keselamatan-enam-langkah"
SPRC = "sprc-safety-planning-guide-2008"


def errors_for(item, path=None):
    path = path or ITEMS[item["id"]][0]
    return RV.item_errors(item, file_path=path, file_doc=CTX["files"][path], sources_by_id=CTX["sources_by_id"],
                          gate_sets=CTX["gate_sets"], v1_cards=CTX["v1"], help_catalogue=CTX["help"])


def mutated(item_id, change):
    item = copy.deepcopy(ITEMS[item_id][1])
    change(item)
    return errors_for(item)


def refreshed(item):
    """Recompute the machine-check fields, as --update would, so only the rule under test fails."""
    item["text_sha256"] = RV.text_sha256(item)
    item["reading_time_minutes"] = RV.reading_time(item)
    item["checks"]["plain_language"] = RV.plain_language(item)
    item["checks"]["claims"] = RV.claims_check(item, CTX["gate_sets"], item["checks"]["claims"])
    return item


class LibraryIsValid(unittest.TestCase):
    def test_validator_reports_no_errors(self):
        errors = RV.library_errors()
        self.assertEqual(errors, [], "\n".join(errors))

    def test_schema_is_draft_2020_12_and_files_follow_it(self):
        self.assertEqual(CTX["schema"]["$schema"], "https://json-schema.org/draft/2020-12/schema")
        for path, doc in CTX["files"].items():
            self.assertEqual(RV.validate_schema(doc, CTX["schema"], CTX["schema"]), [], path)

    def test_schema_subset_rejects_bad_documents(self):
        doc = copy.deepcopy(CTX["files"]["guides_sela.json"])
        doc["items"][0]["audience"] = "teens"
        doc["items"][0]["unexpected"] = True
        del doc["items"][0]["review"]
        msgs = "\n".join(RV.validate_schema(doc, CTX["schema"], CTX["schema"]))
        self.assertIn("'teens' not in", msgs)
        self.assertIn("unexpected property 'unexpected'", msgs)
        self.assertIn("missing required 'review'", msgs)


class ContentSetV1(unittest.TestCase):
    def test_counts(self):
        counts = CTX["set"]["counts"]
        self.assertEqual(counts["items"], 38)
        self.assertEqual(counts["origin:migrated_v1"], 30)
        self.assertEqual(counts["origin:new_v3"], 8)
        self.assertEqual(counts["licence:CC BY-NC-SA 3.0 IGO"], 22)
        self.assertEqual(counts["kind:safety_plan"], 2)
        self.assertEqual(counts["editor_flags:open"], 17)
        self.assertEqual(counts["claims:hits"], 0)
        self.assertEqual(counts["origin:text_changed"], 7)
        self.assertEqual(counts["source_claims"], 18)
        self.assertEqual(counts["source_claims:editor_confirmed"], 0)

    def test_set_version_is_recorded_in_its_change_log(self):
        self.assertEqual(CTX["set"]["version"], "1.2.0")
        self.assertEqual([c["version"] for c in CTX["set"]["changes"]], ["1.0.0", "1.1.0", "1.2.0"])
        for method in ("verbatim_overlap", "source_claims", "licence_scan"):
            self.assertIn(method, CTX["set"]["methods"])

    def test_every_item_is_machine_drafted_unreviewed_and_not_publishable(self):
        for item_id, (_path, item) in ITEMS.items():
            self.assertEqual(item["review"]["status"], "machine_drafted_unreviewed", item_id)
            self.assertFalse(item["review"]["publishable"], item_id)
            self.assertIsNone(item["review"]["human_editor"], item_id)
            self.assertEqual(item["review"]["verifier"]["status"], "not_run", item_id)
            self.assertTrue(item["checks"]["plain_language"]["status"].startswith("machine_"), item_id)
            self.assertTrue(item["checks"]["claims"]["status"].startswith("machine_"), item_id)

    def test_every_item_is_bilingual_and_dated(self):
        for item_id, (_path, item) in ITEMS.items():
            self.assertEqual(item["languages"], ["id", "en"], item_id)
            self.assertEqual(item["checks"]["date_checked"], "2026-10-26", item_id)
            for _p, lang, text in RV.user_texts(item):
                self.assertTrue(text.strip(), f"{item_id}: empty {lang} text")

    def test_new_guides_cover_the_requested_topics(self):
        guides = {i["id"]: i for _p, i in ITEMS.values() if i["kind"] == "guide"}
        topics = {t for g in guides.values() for t in g["topics"]}
        for t in ("listening", "journaling", "consultation_preparation", "privacy", "sharing", "confidentiality",
                  "urgent_help", "accessibility"):
            self.assertIn(t, topics)


class FaithfulMigration(unittest.TestCase):
    def setUp(self):
        if CTX["v1"] is None:
            self.skipTest("internal/fixtures/data/self_help_cards.v1.json not present")

    def test_all_30_v1_cards_are_migrated_and_23_keep_identical_text(self):
        self.assertEqual(len(CTX["v1"]) - len(CHANGED_V1), 23)
        self.assertEqual(len(CTX["v1"]), 30)
        for card_id, card in CTX["v1"].items():
            self.assertIn(card_id, ITEMS)
            item = ITEMS[card_id][1]
            self.assertEqual(item["origin"]["v1_text_sha256"], RV.sha256(RV.canonical(RV.v1_content(card))), card_id)
            self.assertEqual(item["origin"]["v1_source"], card["source"], card_id)
            self.assertEqual(item["evidence_note"], card["evidenceNote"], card_id)
            if card_id in CHANGED_V1:
                self.assertTrue(item["origin"]["text_changed"], card_id)
                self.assertEqual(item["version"], CHANGED_V1[card_id], card_id)
                self.assertNotEqual(item["content"], RV.v1_content(card), card_id)
                self.assertIn(f"version {item['version']}", item["adaptation_note"], card_id)
                self.assertTrue(any(f["status"] == "open" for f in item["checks"]["claims"]["editor_flags"]), card_id)
            else:
                self.assertEqual(item["content"], RV.v1_content(card), card_id)
                self.assertFalse(item["origin"]["text_changed"], card_id)
                self.assertEqual(item["version"], 1, card_id)

    def test_a_declared_change_must_bump_the_version_and_differ_from_v1(self):
        errors = "\n".join(mutated(SIX_STEPS, lambda i: i.update(version=1)))
        self.assertIn("bumps its version", errors)

        def back_to_v1(item):
            item["content"] = RV.v1_content(CTX["v1"][SIX_STEPS])
            refreshed(item)
        errors = "\n".join(mutated(SIX_STEPS, back_to_v1))
        self.assertIn("text_changed is true but the text equals the v1 card", errors)

        errors = "\n".join(mutated(SIX_STEPS, lambda i: i.update(adaptation_note="Changed.")))
        self.assertIn("must record what changed in version 2", errors)

    def test_a_text_change_is_caught(self):
        def change(item):
            item["content"]["steps"]["id"][0] = "Teks yang diubah diam-diam."
            refreshed(item)
        errors = "\n".join(mutated("napas-lambat-tiga-hitungan", change))
        self.assertIn("not identical to the v1 card", errors)
        self.assertIn("text differs from the v1 hash", errors)

    def test_baseline_hash_matches_the_v1_file(self):
        self.assertEqual(CTX["set"]["v1_baseline"]["sha256"], RV.sha256(RV.V1_CARDS_PATH.read_bytes()))


class WhoLicence(unittest.TestCase):
    def who_items(self):
        return [i for _p, i in ITEMS.values() if RV.WHO_LICENCE in i["licence"]["source_licence"]]

    def test_22_who_cards_carry_attribution_notice_share_alike_and_b14(self):
        who = self.who_items()
        self.assertEqual(len(who), 22)
        for item in who:
            self.assertEqual(ITEMS[item["id"]][0], "cards_who.json")
            self.assertEqual(item["licence"]["item_text_licence"], RV.WHO_LICENCE)
            self.assertTrue(item["licence"]["share_alike"])
            self.assertEqual(item["licence"]["commercial_use"], "blocked")
            self.assertEqual(item["licence"]["blocker"], "B-14")
            self.assertIn("WHO", item["attribution"]["en"])
            self.assertIn("not responsible", item["adaptation_notice"]["en"])
            self.assertIn("tidak bertanggung jawab", item["adaptation_notice"]["id"])
            self.assertIn("does not endorse", item["adaptation_notice"]["en"])

    def test_who_file_declares_its_licence_is_not_mit(self):
        notice = CTX["files"]["cards_who.json"]["file_licence"]
        self.assertEqual(notice["text_licence"], RV.WHO_LICENCE)
        self.assertIn("not under the repository's MIT licence", notice["notice"])
        self.assertIn("B-14", notice["notice"])
        self.assertIn(RV.WHO_LICENCE_URL, notice["notice"])
        self.assertIn("LICENSE.md", notice["notice"])

    def test_every_who_attribution_gives_the_licence_uri_and_the_source_link(self):
        self.assertEqual(RV.WHO_LICENCE_URL, "https://creativecommons.org/licenses/by-nc-sa/3.0/igo")
        for item in self.who_items():
            (sid,) = item["source"]["source_ids"]
            link = CTX["sources_by_id"][sid]["persistent_url"]
            self.assertTrue(link.startswith("https://iris.who.int/handle/10665/"), sid)
            for lang in RV.LANGS:
                att = item["attribution"][lang]
                self.assertIn(f"({RV.WHO_LICENCE_URL})", att, (item["id"], lang))
                self.assertIn(f"({link})", att, (item["id"], lang))
                self.assertEqual(CTX["sources_by_id"][sid]["licence_url"], RV.WHO_LICENCE_URL)

    def test_missing_licence_uri_or_source_link_is_caught(self):
        def drop_uri(item):
            item["attribution"]["en"] = item["attribution"]["en"].replace(f" ({RV.WHO_LICENCE_URL})", "")
        self.assertIn("must give the licence URI", "\n".join(mutated("grounding-saat-badai-emosi", drop_uri)))

        def wrong_link(item):
            item["attribution"]["id"] = item["attribution"]["id"].replace("10665/331901)", "10665/3319012)")
        self.assertIn("must link its source who-dwmm-2020", "\n".join(mutated("grounding-saat-badai-emosi", wrong_link)))

        def notice_without_uri(item, path="cards_who.json"):
            doc = copy.deepcopy(CTX["files"][path])
            doc["file_licence"]["notice"] = doc["file_licence"]["notice"].replace(RV.WHO_LICENCE_URL, "")
            return RV.item_errors(item, file_path=path, file_doc=doc, sources_by_id=CTX["sources_by_id"],
                                  gate_sets=CTX["gate_sets"], v1_cards=CTX["v1"], help_catalogue=CTX["help"])
        errors = "\n".join(notice_without_uri(copy.deepcopy(ITEMS["grounding-saat-badai-emosi"][1])))
        self.assertIn("file_licence.notice must give", errors)

    def test_url_match_is_exact(self):
        self.assertTrue(RV._contains_url("x (https://a.org/b).", "https://a.org/b"))
        self.assertTrue(RV._contains_url("x https://a.org/b.", "https://a.org/b"))
        self.assertFalse(RV._contains_url("x (https://a.org/b2).", "https://a.org/b"))
        self.assertFalse(RV._contains_url("x https://a.org/b/c", "https://a.org/b"))
        self.assertFalse(RV._contains_url("x https://a.org/b.html", "https://a.org/b"))


class LicenceNotice(unittest.TestCase):
    """data/content/reading/LICENSE.md: the CC BY-NC-SA 3.0 IGO notice the MIT LICENSE cannot give."""

    def text(self):
        return RV.LICENCE_NOTICE_PATH.read_text(encoding="utf-8")

    def test_notice_exists_and_meets_the_validator(self):
        self.assertTrue(RV.LICENCE_NOTICE_PATH.exists())
        self.assertEqual(RV.licence_notice_errors(self.text(), CTX["sources"]), [])

    def test_notice_gives_licence_uri_scope_terms_and_citations(self):
        text = self.text()
        for needle in (RV.WHO_LICENCE_URL, RV.WHO_LEGALCODE_URL, "cards_who.json",
                       "internal/fixtures/data/self_help_cards.v1.json", "B-14", "D-045", "WHO logo", "endorse"):
            self.assertIn(needle, text)
        for sid in ("who-dwmm-2020", "who-pmplus-2018", "who-mh-work-2022", "who-loneliness-2025"):
            s = CTX["sources_by_id"][sid]
            self.assertIn(s["suggested_citation"], text, sid)
            self.assertIn(s["persistent_url"], text, sid)
        self.assertIn(CTX["sources_by_id"]["who-dwmm-2020"]["translation_disclaimer_verbatim"], text)

    def test_a_weakened_notice_is_caught(self):
        text = self.text()
        dwmm = CTX["sources_by_id"]["who-dwmm-2020"]
        weakened = text.replace(RV.WHO_LICENCE_URL, "").replace(dwmm["suggested_citation"], "").replace("logo", "")
        msgs = "\n".join(RV.licence_notice_errors(weakened, CTX["sources"]))
        self.assertIn(repr(RV.WHO_LICENCE_URL), msgs)
        self.assertIn("who-dwmm-2020's suggested_citation", msgs)
        self.assertIn("'logo'", msgs)

    def test_notice_states_the_verbatim_wording_found(self):
        text = self.text()
        self.assertIn("lepas-kaitan-sadari-namai-fokus", text)
        self.assertIn("PDF p. 125", text)
        lepas = ITEMS["lepas-kaitan-sadari-namai-fokus"][1]
        self.assertIn("keep WHO's own wording", lepas["language_notes"]["en"])
        self.assertIn("word for word", lepas["adaptation_note"])

    def test_missing_notice_or_commercial_use_is_caught(self):
        errors = "\n".join(mutated("grounding-saat-badai-emosi", lambda i: i.pop("adaptation_notice")))
        self.assertIn("need an adaptation_notice", errors)
        errors = "\n".join(mutated("grounding-saat-badai-emosi", lambda i: i["licence"].update(commercial_use="allowed")))
        self.assertIn("blocked for commercial use", errors)

    def test_adapted_who_sources_keep_citation_and_translation_disclaimer(self):
        for sid in ("who-dwmm-2020", "who-pmplus-2018", "who-mh-work-2022", "who-loneliness-2025"):
            s = CTX["sources_by_id"][sid]
            self.assertIn("Licence: CC BY-NC-SA 3.0 IGO.", s["suggested_citation"])
            self.assertIn("WHO is not responsible", s["translation_disclaimer_verbatim"])
            self.assertTrue(s["persistent_url"].startswith("https://iris.who.int/handle/10665/"))


class WhoUrlFix(unittest.TestCase):
    def test_loneliness_source_has_iris_handle_and_isbn_and_no_404_url(self):
        s = CTX["sources_by_id"]["who-loneliness-2025"]
        self.assertEqual(s["persistent_url"], "https://iris.who.int/handle/10665/381746")
        self.assertEqual(s["identifiers"]["isbn_electronic"], "978-92-4-011236-0")
        self.assertEqual(s["url"], "https://www.who.int/publications/i/item/978240112360")
        self.assertIn("HTTP 404", s["url_note"])
        blob = json.dumps(CTX["files"], ensure_ascii=False) + json.dumps(CTX["sources"], ensure_ascii=False)
        self.assertNotIn(RV.WHO_404_URL, blob)

    def test_the_404_form_is_rejected(self):
        errors = "\n".join(mutated("kesepian-langkah-kecil-terhubung",
                                   lambda i: i.update(adaptation_note=i["adaptation_note"] + " " + RV.WHO_404_URL)))
        self.assertIn("404 URL", errors)


class Audience(unittest.TestCase):
    def test_migrated_cards_are_adult_only(self):
        for _p, item in ITEMS.values():
            if item["origin"]["kind"] == "migrated_v1":
                self.assertEqual(item["audience"], "adult", item["id"])
                self.assertEqual(item["review"]["teen_review"], "not_reviewed", item["id"])

    def test_teen_item_is_gated(self):
        teen = [i for _p, i in ITEMS.values() if i["audience"] == "15-17"]
        self.assertEqual([i["id"] for i in teen], ["kerahasiaan-usia-16-17"])
        self.assertIn("B-10", teen[0]["availability_note"])
        sources = {b["source_id"] for s in teen[0]["content"]["sections"] for p in s["paragraphs"]
                   for b in p["basis"] if b["kind"] == "source"}
        for sid in sources:
            self.assertNotIn(CTX["sources_by_id"][sid]["stated_population"], RV.ADULT_ONLY_POPULATIONS, sid)

    def test_adult_material_cannot_be_widened(self):
        for item_id in ("napas-lambat-tiga-hitungan", "tidur-cukup-tujuh-sampai-sembilan-jam"):
            for audience in ("all", "15-17"):
                errors = "\n".join(mutated(item_id, lambda i: i.update(audience=audience, availability_note="x")))
                self.assertIn("v1 cards were written for adults 18+", errors)
        errors = "\n".join(mutated("napas-lambat-tiga-hitungan", lambda i: i.update(audience="all")))
        self.assertIn("cannot be adapted from adult-only sources", errors)

    def test_guide_cannot_paraphrase_an_adult_only_source(self):
        def change(item):
            item["content"]["sections"][0]["paragraphs"][3]["basis"][0]["support"] = "paraphrase"
        errors = "\n".join(mutated("menulis-catatan-dan-melihat-kembali", change))
        self.assertIn("paraphrases an adult-only source", errors)


class ClaimGates(unittest.TestCase):
    def test_both_gate_files_behave_as_declared(self):
        self.assertEqual(RV.gate_examples_errors(CTX["gate_sets"]), [])

    def test_content_gates_use_the_published_negation_prefix(self):
        claim_gates = RV.load_json(RV.CLAIM_GATES_PATH)
        prefix = RV.negation_prefix(claim_gates)
        self.assertTrue(prefix and prefix.startswith("(?:"))
        versions = [v for v, _ in CTX["gate_sets"]]
        # the product gates are used exactly as published, whatever their version; a new version
        # makes the stored checks stale, so `--update` and an editor review of new hits follow
        self.assertEqual(versions, [claim_gates["version"], "sela-reading-content-gates-v1"])

    def test_former_hits_are_reworded_and_await_the_editor(self):
        # In 1.1.0 these two cards hit the efficacy and causal gates; version 2 rewords their "why".
        with_hits = {i["id"] for _p, i in ITEMS.values() if i["checks"]["claims"]["result"] == "hits"}
        self.assertEqual(with_hits, set())
        for item_id in ("mulai-bergerak-lagi-sedikit-demi-sedikit", "gerak-tubuh-setelah-kerja"):
            item = ITEMS[item_id][1]
            self.assertEqual(item["version"], 2)
            flags = {f["path"]: f for f in item["checks"]["claims"]["editor_flags"]}
            self.assertEqual(flags["content.why"]["status"], "open", item_id)
            self.assertIn("confirms version 2 or reverts it", flags["content.why"]["detail"], item_id)

    def test_new_guides_have_no_hits(self):
        for _p, item in ITEMS.values():
            if item["kind"] == "guide":
                self.assertEqual(item["checks"]["claims"]["hits"], [], item["id"])

    def test_disclaimers_pass_and_claims_hit(self):
        gs = CTX["gate_sets"]
        for ok in ("Sela tidak memberi diagnosis, tidak memberi saran obat.", "Sela doesn't diagnose.",
                   "A summary that was sent or opened doesn't mean you have received care.",
                   "Sela tidak menjanjikan bahwa menulis catatan akan membuatmu merasa lebih baik.",
                   "Kartu jaminan kesehatan, kalau kamu memakainya."):
            self.assertEqual(RV.scan_text(ok, gs), [], ok)
        for bad, gate in (("Latihan ini menyembuhkan kecemasan.", "cure_or_heal"), ("Sela is an AI therapist.", "therapy_or_counselling"),
                          ("Kamu mengalami depresi.", "condition_label"), ("Evidence shows this works.", "efficacy_or_certainty"),
                          ("Sela mendiagnosis.", "diagnosis"), ("Hari ke-3 dari 14.", "engagement_pressure")):
            self.assertIn(gate, {h[1] for h in RV.scan_text(bad, gs)}, bad)

    def test_a_new_claim_needs_an_editor_flag(self):
        def change(item):
            item["content"]["sections"][0]["paragraphs"][0]["id"] = "Latihan ini terbukti menyembuhkan stres."
            refreshed(item)
        errors = "\n".join(mutated("didengarkan-di-sela", change))
        self.assertIn("has no editor flag", errors)

    def test_stale_checks_are_caught(self):
        def change(item):
            item["content"]["summary"]["en"] = item["content"]["summary"]["en"] + " Extra words here."
        errors = "\n".join(mutated("didengarkan-di-sela", change))
        self.assertIn("text_sha256 is stale", errors)
        self.assertIn("checks.plain_language is stale", errors)


class StatedStepCounts(unittest.TestCase):
    """A title or summary that says how many steps an item has must match its steps."""

    def test_number_words_are_read_in_both_languages(self):
        item = copy.deepcopy(ITEMS[SIX_STEPS][1])
        self.assertEqual(RV.stated_step_counts(item), [("content.title.id", 6), ("content.title.en", 6)])
        item = copy.deepcopy(ITEMS["masalah-uang-enam-langkah"][1])
        self.assertEqual(RV.stated_step_counts(item), [("content.title.id", 6), ("content.title.en", 6)])

    def test_known_mismatches_carry_open_editor_flags_with_proposals(self):
        mismatched = {}
        for _p, item in ITEMS.values():
            if "steps" in item["content"]:
                for path, n in RV.stated_step_counts(item):
                    if n != len(item["content"]["steps"]["id"]):
                        mismatched.setdefault(item["id"], set()).add(path.rsplit(".", 1)[0])
        self.assertEqual(mismatched, {"masalah-uang-enam-langkah": {"content.title"},
                                      "lepas-kaitan-sadari-namai-fokus": {"content.summary"}})
        for item_id, paths in mismatched.items():
            flags = {f["path"]: f for f in ITEMS[item_id][1]["checks"]["claims"]["editor_flags"]}
            for path in paths:
                self.assertEqual(flags[path]["status"], "open", item_id)
                self.assertIn("proposed_revision", flags[path], item_id)
                self.assertEqual(RV.stated_step_counts({"content": {"title": flags[path]["proposed_revision"],
                                                                    "summary": flags[path]["proposed_revision"]}}),
                                 [], item_id)

    def test_an_unflagged_mismatch_is_caught(self):
        def unflag(item):
            item["checks"]["claims"]["editor_flags"] = [f for f in item["checks"]["claims"]["editor_flags"]
                                                        if f["path"] != "content.title"]
        errors = "\n".join(mutated("masalah-uang-enam-langkah", unflag))
        self.assertIn("content.title.en states 6 steps but the item has 5", errors)

        def merge_back(item):
            for lang in RV.LANGS:
                item["content"]["steps"][lang][2:4] = [item["content"]["steps"][lang][2]]
            refreshed(item)
        errors = "\n".join(mutated(SIX_STEPS, merge_back))
        self.assertIn("content.title.id states 6 steps but the item has 5", errors)


class PlainLanguage(unittest.TestCase):
    def test_new_guides_are_within_targets(self):
        for _p, item in ITEMS.values():
            if item["kind"] == "guide":
                pl = item["checks"]["plain_language"]
                self.assertEqual(pl["result"], "within_targets", (item["id"], pl["flags"]))
                self.assertLessEqual(pl["metrics"]["en"]["fk_grade"], RV.MAX_FK_GRADE)

    def test_metric_helpers(self):
        self.assertEqual(RV.sentences("Satu dua. Tiga empat lima! Enam?"), ["Satu dua.", "Tiga empat lima!", "Enam?"])
        self.assertEqual(RV.words("HP-mu, doesn't 16–17"), ["HP-mu", "doesn't", "16", "17"])
        self.assertEqual(RV.syllables("the"), 1)
        self.assertEqual(RV.syllables("breathing"), 2)
        self.assertEqual(RV.syllables("consultation"), 4)

    def test_reading_time_rounds_up_per_180_words(self):
        item = copy.deepcopy(ITEMS["didengarkan-di-sela"][1])
        n = sum(len(RV.words(t)) for _p, lang, t in RV.user_texts(item) if lang == "id")
        self.assertEqual(item["reading_time_minutes"]["id"], max(1, -(-n // 180)))


class GuidesBasis(unittest.TestCase):
    def test_every_guide_paragraph_has_a_basis(self):
        for _p, item in ITEMS.values():
            for section in item["content"].get("sections", []):
                for par in section["paragraphs"]:
                    self.assertTrue(par["basis"], item["id"])

    def test_general_advice_is_labelled_and_sources_resolve(self):
        def change(item):
            item["content"]["sections"][0]["paragraphs"][0]["basis"] = [{"kind": "general_advice"}]
        self.assertIn("general advice carries the label", "\n".join(mutated("menulis-catatan-dan-melihat-kembali", change)))

        def change2(item):
            item["content"]["sections"][2]["paragraphs"][0]["basis"][0]["source_id"] = "made-up-source"
        self.assertIn("is not in sources.json", "\n".join(mutated("didengarkan-di-sela", change2)))

    def test_product_statements_are_pending_implementation_check(self):
        for _p, item in ITEMS.values():
            for section in item["content"].get("sections", []):
                for par in section["paragraphs"]:
                    for b in par["basis"]:
                        if b["kind"] == "product_basis":
                            self.assertEqual(b["implementation_check"], "pending", item["id"])

    def test_legal_wording_is_marked_pending_b10(self):
        for item_id in ("apa-yang-terjadi-pada-yang-kamu-simpan", "kerahasiaan-usia-16-17"):
            item = ITEMS[item_id][1]
            pending = [b for s in item["content"]["sections"] for p in s["paragraphs"] for b in p["basis"] if b.get("pending_review")]
            self.assertTrue(pending and all("B-10" in b["pending_review"] for b in pending), item_id)


class SafetyAndPersonalData(unittest.TestCase):
    def test_safety_plan_cards_are_static_help_page_content(self):
        safety = [i for _p, i in ITEMS.values() if i["kind"] == "safety_plan"]
        self.assertEqual(sorted(i["id"] for i in safety), ["rencana-keselamatan-cara-menggunakan", "rencana-keselamatan-enam-langkah"])
        for item in safety:
            self.assertEqual(item["surfaces"], ["help_page"])
            self.assertFalse(item["ai_may_offer"])
            self.assertEqual(item["source"]["relation"], "structure_only")
        errors = "\n".join(mutated("rencana-keselamatan-enam-langkah", lambda i: i.update(ai_may_offer=True)))
        self.assertIn("may never offer a safety_plan item", errors)

    def test_six_step_card_has_the_six_steps_of_the_original(self):
        item = ITEMS[SIX_STEPS][1]
        for lang in RV.LANGS:
            self.assertEqual(len(item["content"]["steps"][lang]), 6, lang)
        self.assertIn("enam langkah", item["content"]["title"]["id"])
        self.assertIn("six steps", item["content"]["title"]["en"])
        steps_en = item["content"]["steps"]["en"]
        self.assertTrue(steps_en[2].startswith("People and places that distract"))
        self.assertTrue(steps_en[3].startswith("People you can ask for help"))
        self.assertTrue(steps_en[5].startswith("A safer environment"))
        self.assertIn(SPRC, item["source"]["source_ids"])
        self.assertEqual(item["checks"]["source_fidelity"]["status"], "machine_spot_checked_unreviewed")

    def test_unsourced_widely_used_claim_is_gone_and_flagged(self):
        item = ITEMS[SIX_STEPS][1]
        blob = json.dumps(item["content"], ensure_ascii=False)
        self.assertNotIn("widely used", blob)
        self.assertNotIn("banyak dipakai", blob)
        flags = {f["path"]: f for f in item["checks"]["claims"]["editor_flags"]}
        self.assertEqual(set(flags), {"content.steps", "content.why"})
        self.assertTrue(all(f["status"] == "open" for f in flags.values()))
        self.assertEqual(flags["content.steps"]["source_id"], SPRC)

    def test_sprc_guide_is_an_approved_open_source_read_without_reproduction(self):
        s = CTX["sources_by_id"][SPRC]
        self.assertEqual(s["kind"], "practice_guide")
        self.assertEqual(RV._host(s["url"]), "sprc.org")
        self.assertIn("sprc.org", RV.APPROVED_HOSTS)
        self.assertEqual(s["stated_population"], "clinical_population")
        self.assertIn("reproduce", s["licence"])
        for item_id in ("rencana-keselamatan-enam-langkah", "rencana-keselamatan-cara-menggunakan"):
            self.assertIn(SPRC, ITEMS[item_id][1]["source"]["source_ids"])

    def test_step_limits_depend_on_kind(self):
        def seven(item):
            for lang in RV.LANGS:
                item["content"]["steps"][lang].append(item["content"]["steps"][lang][0])
            refreshed(item)
        self.assertIn("safety_plan steps must number 2-6, got 7", "\n".join(mutated(SIX_STEPS, seven)))

        def six(item):
            for lang in RV.LANGS:
                item["content"]["steps"][lang].append(item["content"]["steps"][lang][0])
            refreshed(item)
        self.assertIn("exercise steps must number 2-5, got 6", "\n".join(mutated("masalah-uang-enam-langkah", six)))

    def test_contacts_are_declared_and_in_the_help_catalogue(self):
        if CTX["help"] is None:
            self.skipTest("web/src/data/help.fallback.json not present")
        for _p, item in ITEMS.values():
            text = "\n".join(t for _x, _l, t in RV.user_texts(item))
            if "119" in re.sub(r"Healing119|healing119", "", text):
                self.assertTrue(item["contacts"], item["id"])
            for contact in item["contacts"]:
                self.assertIn(contact["catalogue_id"], CTX["help"])
                self.assertLessEqual(contact["reverify_by"], CTX["help"][contact["catalogue_id"]]["reverifyBy"])

    def test_undeclared_numbers_addresses_and_formal_register_are_caught(self):
        def change(item):
            item["content"]["sections"][0]["paragraphs"][0]["id"] = "Hubungi Anda di 0812-3456-7890 atau ke contoh@mail.com dan www.contoh.com."
            refreshed(item)
        errors = "\n".join(mutated("didengarkan-di-sela", change))
        self.assertIn("formal 'Anda'", errors)
        self.assertIn("personal data", errors)
        self.assertIn("undeclared web address", errors)
        self.assertIn("undeclared number", errors)

    def test_publishing_needs_a_human_approval_of_the_exact_text(self):
        errors = "\n".join(mutated("didengarkan-di-sela", lambda i: i["review"].update(publishable=True)))
        self.assertIn("never publishable", errors)
        self.assertIn("needs a human approval", errors)
        self.assertIn("verifier verdict 'supported'", errors)

        def approved(item):
            item["review"].update(status="human_approved", human_editor="Editor Name", reviewed_on="2026-11-01",
                                  approved_text_sha256=item["text_sha256"], publishable=True,
                                  verifier={"status": "supported", "note": "ok"})
        self.assertEqual(mutated("didengarkan-di-sela", approved), [])
        errors = "\n".join(mutated("gerak-tubuh-setelah-kerja", approved))
        self.assertIn("clean claims check and no open editor flags", errors)


def claim(item, path):
    return next(sc for sc in item["source_claims"] if sc["path"] == path)


class SourceClaims(unittest.TestCase):
    """A summary or 'why' that names a source says only what that source says, with a locator and a paraphrase."""

    def test_every_field_that_names_a_source_has_a_pending_entry(self):
        entries = 0
        for item_id, (_p, item) in ITEMS.items():
            named = RV.named_sources(item)
            paths = sorted(sc["path"] for sc in item.get("source_claims", []))
            self.assertEqual(paths, sorted(named), item_id)
            for sc in item.get("source_claims", []):
                entries += 1
                self.assertEqual(sc["status"], "machine_checked_unreviewed", item_id)
                self.assertEqual(sc["editor_confirmation"]["status"], "pending", item_id)
                self.assertTrue(sc["locator"].strip(), item_id)
                if sc["support"] != "not_checked":
                    self.assertTrue(sc["source_says"], item_id)
                    self.assertFalse(any(m in sc["source_says"] for m in RV.QUOTE_MARKS), item_id)
                    self.assertLess(sc["quote_check"]["longest_shared_run_words"], RV.MIN_SHARED_RUN, item_id)
        self.assertEqual(entries, 18)
        # every 'why' that names WHO or CDC is covered
        for item_id, (_p, item) in ITEMS.items():
            why = item["content"].get("why")
            if why and re.search(r"\b(?:WHO|CDC)\b", why["id"] + " " + why["en"]):
                self.assertEqual(claim(item, "content.why")["path"], "content.why", item_id)

    def test_the_three_reviewed_rationales_now_say_what_their_source_says(self):
        work = ITEMS["gerak-tubuh-setelah-kerja"][1]["content"]["why"]
        self.assertNotIn("evidence shows", work["en"])
        self.assertNotIn("bukti menunjukkan", work["id"])
        self.assertIn("conditional recommendation", work["en"])
        self.assertIn("very low certainty", work["en"])
        self.assertIn("bersyarat", work["id"])
        self.assertIn("benefits at work", work["en"])
        move = ITEMS["mulai-bergerak-lagi-sedikit-demi-sedikit"][1]["content"]["why"]
        for gone in ("find", "directly linked"):
            self.assertNotIn(gone, move["en"])
        for gone in ("menemukan", "berkaitan langsung"):
            self.assertNotIn(gone, move["id"])
        sleep = ITEMS["jadwal-tidur-konsisten"][1]["content"]["why"]
        self.assertNotIn("because", sleep["en"])
        self.assertNotIn("body clock", sleep["en"])
        self.assertNotIn("karena", sleep["id"])
        self.assertNotIn("jam biologis", sleep["id"])
        for item_id in WHY_REWORDED:
            sc = claim(ITEMS[item_id][1], "content.why")
            self.assertEqual(sc["support"], "supported", item_id)
            self.assertIn("Version 2 text", sc["note"], item_id)

    def test_partly_supported_and_unchecked_claims_keep_open_flags(self):
        partly = {(i, sc["path"]) for i, (_p, item) in ITEMS.items() for sc in item.get("source_claims", [])
                  if sc["support"] == "partly_supported"}
        self.assertEqual(partly, {("kamar-dan-layar-sebelum-tidur", "content.why"),
                                  ("tidur-cukup-tujuh-sampai-sembilan-jam", "content.summary")})
        unchecked = {i for i, (_p, item) in ITEMS.items() for sc in item.get("source_claims", [])
                     if sc["support"] == "not_checked"}
        self.assertEqual(unchecked, {"kiat-harian-kelola-stres", "jeda-dan-batas-waktu-belajar", "menerima-diri-dan-tujuan-realistis"})

    def test_a_missing_or_quoting_entry_is_caught(self):
        def drop(item):
            item["source_claims"] = [sc for sc in item["source_claims"] if sc["path"] != "content.why"]
        self.assertIn("content.why names CDC but has no source_claims entry", "\n".join(mutated("jadwal-tidur-konsisten", drop)))

        def quoting(item):
            claim(item, "content.why")["source_says"] = 'CDC says "go to bed at the same time".'
        self.assertIn("must not quote the source", "\n".join(mutated("jadwal-tidur-konsisten", quoting)))

        def long_run(item):
            claim(item, "content.why")["quote_check"]["longest_shared_run_words"] = 7
        self.assertIn("shares 7 words in a row", "\n".join(mutated("jadwal-tidur-konsisten", long_run)))

        def no_says(item):
            claim(item, "content.why")["source_says"] = None
        self.assertIn("needs source_says", "\n".join(mutated("jadwal-tidur-konsisten", no_says)))

        def wrong_names(item):
            claim(item, "content.why")["names"] = ["WHO"]
        self.assertIn("do not match the names in the text", "\n".join(mutated("jadwal-tidur-konsisten", wrong_names)))

        def foreign_source(item):
            claim(item, "content.why")["source_ids"] = ["who-dwmm-2020"]
        self.assertIn("is not one of the item's sources", "\n".join(mutated("jadwal-tidur-konsisten", foreign_source)))

    def test_a_new_attribution_needs_an_entry(self):
        def attribute(item):
            item["content"]["why"]["en"] = "CDC says so. " + item["content"]["why"]["en"]
            refreshed(item)
        errors = "\n".join(mutated("tidur-cukup-tujuh-sampai-sembilan-jam", attribute))
        self.assertIn("content.why names CDC but has no source_claims entry", errors)

    def test_partial_support_and_unchecked_sources_need_open_flags(self):
        def close_flags(item):
            for f in item["checks"]["claims"]["editor_flags"]:
                f["status"] = "resolved"
        errors = "\n".join(mutated("kamar-dan-layar-sebelum-tidur", close_flags))
        self.assertIn("support 'partly_supported' needs an open editor flag on content.why", errors)
        errors = "\n".join(mutated("jeda-dan-batas-waktu-belajar", close_flags))
        self.assertIn("not_checked needs an open source_unreachable editor flag", errors)

    def test_only_a_human_editor_confirms_and_only_the_exact_text(self):
        def machine_confirms(item):
            claim(item, "content.why")["editor_confirmation"].update(
                status="confirmed", editor="Editor Name", confirmed_on="2026-11-01",
                confirmed_text_sha256=RV.field_sha256(item, "content.why"))
        errors = "\n".join(mutated("kelelahan-kerja-kenali-dan-pulihkan", machine_confirms))
        self.assertIn("only a human editor confirms a source claim", errors)

        def approve(item, *, change_text=False):
            for sc in item["source_claims"]:
                sc["editor_confirmation"].update(status="confirmed", editor="Editor Name", confirmed_on="2026-11-01",
                                                 confirmed_text_sha256=RV.field_sha256(item, sc["path"]))
            if change_text:
                item["content"]["why"]["en"] += " Extra."
                refreshed(item)
            item["review"].update(status="human_approved", human_editor="Editor Name", reviewed_on="2026-11-01",
                                  approved_text_sha256=item["text_sha256"], publishable=True,
                                  verifier={"status": "supported", "note": "ok"})
        self.assertEqual(mutated("beban-kerja-bicara-dan-atur-batas", approve), [])
        errors = "\n".join(mutated("beban-kerja-bicara-dan-atur-batas", lambda i: approve(i, change_text=True)))
        self.assertIn("the text changed after the editor confirmed it", errors)

    def test_publishing_needs_every_claim_supported_and_confirmed(self):
        def approve_without_claims(item):
            item["review"].update(status="human_approved", human_editor="Editor Name", reviewed_on="2026-11-01",
                                  approved_text_sha256=item["text_sha256"], publishable=True,
                                  verifier={"status": "supported", "note": "ok"})
        errors = "\n".join(mutated("beban-kerja-bicara-dan-atur-batas", approve_without_claims))
        self.assertIn("publishable needs every source claim supported and confirmed by the editor (content.summary)", errors)
        self.assertIn("(content.why)", errors)

    def test_a_changed_migrated_card_keeps_an_open_flag(self):
        def close(item):
            for f in item["checks"]["claims"]["editor_flags"]:
                f["status"] = "resolved"
        errors = "\n".join(mutated("bersikap-baik-pada-diri-sendiri", close))
        self.assertIn("keeps an open editor flag until the editor confirms or reverts", errors)


def sources_copy():
    return copy.deepcopy(CTX["sources"])


def by_id(sources, sid):
    return next(s for s in sources["sources"] if s["id"] == sid)


class VerbatimChecks(unittest.TestCase):
    """Every adapted or paraphrased source records sela-verbatim-overlap-v1 (counts and locators only)."""

    def test_every_adapted_or_paraphrased_source_has_a_check(self):
        for s in CTX["sources"]["sources"]:
            if s["use"].startswith(("adapted", "paraphrased")):
                self.assertIn("verbatim_check", s, s["id"])
                self.assertEqual(s["verbatim_check"]["method"], VO.METHOD, s["id"])
        self.assertEqual(RV.verbatim_check_errors(CTX["sources"], CTX["files"]), [])

    def test_ayo_sehat_checks_are_recorded_as_not_run_and_due(self):
        for sid in ("kemenkes-ayosehat-stres-2024", "kemenkes-ayosehat-10cara-2025"):
            vc = CTX["sources_by_id"][sid]["verbatim_check"]
            self.assertEqual(vc["status"], "not_run", sid)
            self.assertEqual(vc["due_by"], "2026-11-26", sid)
            self.assertTrue(any("HTTP 503" in a["what"] for a in vc["attempts"]), sid)
            self.assertIn("not yet verified", vc["no_text_copied_basis"], sid)
            self.assertIn("not yet verified", CTX["sources_by_id"][sid]["licence"], sid)
            for item_id in vc["items_compared"]:
                item = ITEMS[item_id][1]
                self.assertEqual(item["licence"]["commercial_use"], "needs_legal_confirmation", item_id)
                self.assertIn("not yet verified", item["adaptation_note"], item_id)
        self.assertEqual(sorted(i for s in ("kemenkes-ayosehat-stres-2024", "kemenkes-ayosehat-10cara-2025")
                                for i in CTX["sources_by_id"][s]["verbatim_check"]["items_compared"]),
                         ["jeda-dan-batas-waktu-belajar", "kiat-harian-kelola-stres", "menerima-diri-dan-tujuan-realistis"])

    def test_checks_that_ran_are_recorded_with_their_copy(self):
        keslan = CTX["sources_by_id"]["kemenkes-keslan-tidur-2025"]["verbatim_check"]
        self.assertEqual(keslan["status"], "machine_checked_unreviewed")
        self.assertLess(keslan["longest_shared_run_words"], RV.MIN_SHARED_RUN)
        self.assertEqual(keslan["languages_compared"], ["id"])
        dwmm = CTX["sources_by_id"]["who-dwmm-2020"]["verbatim_check"]
        runs = {(r["item_id"], r["field"]): (r["words"], r["locator"]) for r in dwmm["runs_at_or_over_5_words"]}
        # the 1.1.0 finding: one full WHO sentence in lepas-kaitan-sadari-namai-fokus, DWMM PDF p. 125
        self.assertEqual(runs[("lepas-kaitan-sadari-namai-fokus", "content.steps.en[0]")], (10, "PDF p. 125"))
        for s in CTX["sources"]["sources"]:
            vc = s.get("verbatim_check")
            if vc and vc["status"] == "machine_checked_unreviewed":
                self.assertRegex(vc["copy"]["sha256"], r"^[0-9a-f]{64}$", s["id"])
                self.assertTrue(RV._host_ok(vc["copy"]["url"], vc["copy"]["kind"]), s["id"])

    def test_missing_or_inconsistent_checks_are_caught(self):
        sources = sources_copy()
        del by_id(sources, "kemenkes-keslan-tidur-2025")["verbatim_check"]
        self.assertIn("kemenkes-keslan-tidur-2025: its text is paraphrased, so it needs a verbatim_check",
                      "\n".join(RV.verbatim_check_errors(sources, CTX["files"])))

        sources = sources_copy()
        vc = by_id(sources, "who-dwmm-2020")["verbatim_check"]
        vc["longest_shared_run_words"] = 3
        self.assertIn("longest_shared_run_words must equal the longest listed run",
                      "\n".join(RV.verbatim_check_errors(sources, CTX["files"])))

        sources = sources_copy()
        by_id(sources, "cdc-about-sleep")["verbatim_check"]["copy"]["url"] = "https://mirror.example.org/sleep.html"
        self.assertIn("is neither the publisher's approved host", "\n".join(RV.verbatim_check_errors(sources, CTX["files"])))

        sources = sources_copy()
        by_id(sources, "cdc-about-sleep")["verbatim_check"]["items_compared"] = ["jadwal-tidur-konsisten"]
        self.assertIn("items_compared must be exactly the items that cite this source",
                      "\n".join(RV.verbatim_check_errors(sources, CTX["files"])))

        sources = sources_copy()
        by_id(sources, "kemenkes-keslan-tidur-2025")["verbatim_check"]["runs_at_or_over_5_words"] = [
            {"item_id": "tidur-cukup-tujuh-sampai-sembilan-jam", "field": "content.steps.id[0]", "words": 6, "locator": "block 48 of the page text"}]
        by_id(sources, "kemenkes-keslan-tidur-2025")["verbatim_check"]["longest_shared_run_words"] = 6
        self.assertIn("shares 6 words with a source that states no licence; it needs an open editor flag",
                      "\n".join(RV.verbatim_check_errors(sources, CTX["files"])))

        sources = sources_copy()
        by_id(sources, "kemenkes-ayosehat-10cara-2025")["verbatim_check"]["attempts"] = []
        self.assertIn("a check that has not run records the attempts", "\n".join(RV.verbatim_check_errors(sources, CTX["files"])))

    def test_an_unverified_no_text_copied_claim_is_caught(self):
        files = copy.deepcopy(CTX["files"])
        item = next(i for i in files["cards_kemenkes.json"]["items"] if i["id"] == "kiat-harian-kelola-stres")
        item["adaptation_note"] = "Paraphrased into steps by the Dengar.in v1 team (2026-10-15); no text copied."
        item["licence"]["commercial_use"] = "allowed"
        item["checks"]["claims"]["editor_flags"] = []
        errors = "\n".join(RV.verbatim_check_errors(CTX["sources"], files))
        self.assertIn("adaptation_note says no text was copied, but the verbatim check", errors)
        self.assertIn("commercial use cannot be allowed while the verbatim check", errors)
        self.assertIn("needs an open editor flag for kemenkes-ayosehat-stres-2024", errors)

        sources = sources_copy()
        by_id(sources, "kemenkes-ayosehat-stres-2024")["licence"] = "No licence statement; paraphrased, no text copied."
        self.assertIn("kemenkes-ayosehat-stres-2024: says no text was copied", "\n".join(RV.verbatim_check_errors(sources, CTX["files"])))

    def test_publishing_needs_the_check_to_have_run(self):
        sources_by_id = {s["id"]: s for s in CTX["sources"]["sources"]}
        item = copy.deepcopy(ITEMS["menerima-diri-dan-tujuan-realistis"][1])
        item["review"]["publishable"] = True
        errors = "\n".join(RV.source_claim_errors(item, "x", sources_by_id))
        self.assertIn("publishable needs the verbatim check of kemenkes-ayosehat-10cara-2025 to have run", errors)


class VerbatimOverlapTool(unittest.TestCase):
    """tests/content/verbatim_overlap.py, the sela-verbatim-overlap-v1 implementation."""

    def source(self, name, text):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path = Path(tmp.name) / name
        path.write_text(text, encoding="utf-8")
        return VO.Source(path)

    def test_tokens_lower_case_and_split_hyphens(self):
        self.assertEqual(VO.tokens("Sehari-hari, WHO’s guide: 7–9 jam."), ["sehari", "hari", "who's", "guide", "7", "9", "jam"])

    def test_longest_run_and_page_locator(self):
        src = self.source("s.txt", "first page words here\fthe quick brown fox jumps over the lazy dog\fend")
        rows = VO.compare(src, [("a", "A quick brown fox jumps over a fence"), ("b", "nothing shared at all"), ("c", "lazy")])
        self.assertEqual(rows[0]["longest_shared_run_words"], 5)
        self.assertEqual(rows[0]["locator"], "PDF p. 2")
        self.assertNotIn("run_text", rows[0])
        self.assertEqual(rows[1]["longest_shared_run_words"], 0)
        self.assertEqual(rows[2]["longest_shared_run_words"], 1)
        self.assertNotIn("locator", rows[2])
        shown = VO.compare(src, [("a", "A quick brown fox jumps over a fence")], show_text=True)
        self.assertEqual(shown[0]["run_text"], "quick brown fox jumps over")

    def test_html_blocks_drop_scripts_and_split_blocks(self):
        src = self.source("p.html", "<html><head><script>var a = 'tidur yang cukup itu penting';</script></head>"
                                    "<body><p>Satu dua tiga empat lima</p><div>enam tujuh</div></body></html>")
        self.assertEqual(src.longest_run("tidur yang cukup itu penting")[0], 0)
        rows = VO.compare(src, [("x", "satu dua tiga empat lima enam")])
        # runs may continue across blocks (and PDF pages), so the count errs on the side of more
        self.assertEqual(rows[0]["longest_shared_run_words"], 6)
        self.assertEqual(rows[0]["locator"], "block 1 of the page text")


class LicenceScan(unittest.TestCase):
    """No repository file may carry CC BY-NC-SA card text without the licence URI, IRIS link and notice."""

    @classmethod
    def setUpClass(cls):
        cls.errors, cls.report = RV.licence_scan()

    def card_text(self, item_id):
        item = ITEMS[item_id][1]
        return " ".join(item["content"]["steps"]["en"]) + " " + item["content"]["summary"]["id"]

    def scan(self, text, name="copy.md"):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path = Path(tmp.name) / name
        path.write_text(text, encoding="utf-8")
        return RV.licence_scan(paths=[path], files=CTX["files"], sources_by_id=CTX["sources_by_id"], v1_cards=CTX["v1"])

    def test_repository_card_text_is_only_where_expected(self):
        found = set(self.report["with_card_text"])
        self.assertIn("data/content/reading/cards_who.json", found)
        self.assertLessEqual(found, {"data/content/reading/cards_who.json", "internal/fixtures/data/self_help_cards.v1.json"})
        failing = {e.split(":", 1)[0] for e in self.errors}
        # cards_who.json carries its notices; the v1 file fails until its owner adds them (open point 2)
        self.assertNotIn("data/content/reading/cards_who.json", failing)
        self.assertLessEqual(failing, {"internal/fixtures/data/self_help_cards.v1.json"})

    def test_a_copy_without_the_notice_fails_and_with_it_passes(self):
        text = self.card_text("grounding-saat-badai-emosi")
        errors, report = self.scan(text)
        self.assertEqual(len(errors), 1)
        self.assertIn("without the licence URI", errors[0])
        self.assertIn("https://iris.who.int/handle/10665/331901", errors[0])
        self.assertIn("the adaptation notice", errors[0])
        item = ITEMS["grounding-saat-badai-emosi"][1]
        errors, _ = self.scan(text + "\n" + item["attribution"]["en"] + "\n" + item["adaptation_notice"]["en"])
        self.assertEqual(errors, [])

    def test_v1_text_of_a_changed_card_is_still_detected(self):
        if CTX["v1"] is None:
            self.skipTest("internal/fixtures/data/self_help_cards.v1.json not present")
        v1 = CTX["v1"]["gerak-tubuh-setelah-kerja"]
        old_why = v1["why"]["en"]
        self.assertNotIn(old_why, json.dumps(ITEMS["gerak-tubuh-setelah-kerja"][1]["content"]))
        errors, report = self.scan(old_why)
        self.assertEqual(list(report["with_card_text"].values()), [["gerak-tubuh-setelah-kerja"]])
        self.assertEqual(len(errors), 1)

    def test_short_phrases_and_other_licences_do_not_trigger(self):
        errors, report = self.scan("Notice what you are thinking. " + self.card_text("jadwal-tidur-konsisten"))
        self.assertEqual((errors, report["with_card_text"]), ([], {}))

    def test_binary_files_are_counted_not_scanned(self):
        errors, report = self.scan("\0binary" + self.card_text("grounding-saat-badai-emosi"), name="pack.bin")
        self.assertEqual(errors, [])
        self.assertEqual(len(report["binary_or_large"]), 1)


class Staleness(unittest.TestCase):
    def test_nothing_is_stale_on_the_freeze_date(self):
        self.assertEqual(RV.stale_entries("2026-10-26"), [])

    def test_unreachable_kemenkes_pages_come_due_first(self):
        stale = RV.stale_entries("2026-12-01")
        self.assertTrue(any("kemenkes-ayosehat" in s for s in stale))
        self.assertFalse(any("healing119" in s for s in stale))
        self.assertIn("source kemenkes-ayosehat-stres-2024: verbatim_check not run, due_by 2026-11-26", stale)


if __name__ == "__main__":
    unittest.main()
