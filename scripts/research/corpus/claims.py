"""Curated FORBIDDEN product claims (ID/EN) for scanning Sela product copy.

Derived from the 'Do not claim' guidance in
docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md and the limits in
06_RESEARCH_METHOD.md / 07_EVIDENCE_TO_FEATURE.md / 08_ROADMAP_AND_VALIDATION.md
(plus 13_JUDGING_AND_DEMO_PACKAGE.md where it states the same limit).

Each gate lists bare AFFIRMATIVE claim regexes (``claim_patterns``). The
exported ``patterns`` wrap every claim in a negation-window prefix, so that a
pattern matches only when none of the NEGATION_WINDOW_TOKENS tokens before the
claim in the same clause is a negation cue ("not", "doesn't", "cannot", "no",
"tidak", "bukan", "belum", "tanpa" ...). Clauses end at . ! ? ; : followed by
whitespace, a line break, an em/en dash or a spaced hyphen; a contrastive
conjunction ("but", "however", "tetapi", "namun", "melainkan" ...) also starts
a new scope, so "not X but an AI therapist" still matches. Disclaimers such as
"does not mean you have received care" or "Kami tidak mengklaim Sela terbukti
efektif" therefore pass, while affirmative claims match.

The prefix is plain regex that compiles in Python ``re`` (fixed-width
lookbehinds only) and in JavaScript ``new RegExp(p, 'iu')`` (as used by
web/src/test/claims.test.ts), so every consumer gets the same negation
handling without reimplementing it. Go's RE2 has no lookaround and cannot use
these patterns. Token characters are spelled out explicitly (ASCII letters,
digits, '_' and U+00C0-U+024F) instead of ``\\w`` because ``\\w`` is Unicode in
Python but ASCII in JavaScript.

A match is a prompt for human review of the copy, not an automatic verdict.
Known blind spot: a negation word within six tokens that belongs to a
different idea ("Never feel alone with your AI therapist") suppresses the hit.
"""
from __future__ import annotations

import re

GATES_VERSION = "sela-claim-gates-v2"
NEGATION_WINDOW_TOKENS = 6

# --- negation-window prefix (portable: Python re and JavaScript 'iu') -----------

NEGATION_CUES = (
    "not", "no", "never", "nor", "cannot", "unable", "without", "neither", "none", "nobody", "nothing", "non",
    "tidak", "tak", "bukan", "bukanlah", "belum", "tanpa", "jangan", "enggak", "nggak", "ngga", "gak", "tiada",
)
NEGATION_CUE_PATTERNS = (r"[a-z]+n['’]t",)  # doesn't, isn't, can't, won't, don't (straight or curly apostrophe)
SCOPE_BREAKERS = ("but", "however", "whereas", "tetapi", "tapi", "namun", "melainkan", "sedangkan", "padahal")

_L = r"0-9A-Za-z_À-ɏ"                       # token letters, identical in both engines
_AP = r"'’"                                      # apostrophes (inside a token only between letters)
_TOKEN = rf"[{_L}]+(?:[{_AP}][{_L}]+)*"
_END = rf"(?![{_L}]|[{_AP}][{_L}])"                   # a token is maximal: "doesn't" cannot split into "doesn" + "t"
_BEGIN = rf"(?<![{_L}])(?<![{_L}][{_AP}])"            # a token starts at a token boundary
# Separator characters between tokens. Clause boundaries are NOT separators:
# [.!?;:] followed by whitespace/end, newline, em dash, en dash, spaced hyphen.
# The five alternatives are disjoint (no ambiguous backtracking on '-----').
_SEP_CH = (rf"(?:[^{_L}{_AP}.!?;:\n—–-]|[{_AP}]|[.!?;:](?!\s|$)|(?<!\s)-|(?<=\s)-(?!\s))")
_NEG = "(?:" + "|".join(NEGATION_CUES + NEGATION_CUE_PATTERNS) + ")"
_BREAK = "(?:" + "|".join(SCOPE_BREAKERS) + ")"
_FREE_TOKEN = rf"(?!(?:{_NEG}|{_BREAK}){_END}){_TOKEN}{_END}{_SEP_CH}+"   # a token that is neither cue nor breaker
_CLAUSE_START = (rf"(?:^|(?<=[.!?;:])(?=\s)|(?<=[\n—–])|(?<=\s-)(?=\s)|{_BEGIN}{_BREAK}{_END})")
NEGATION_PREFIX = (
    rf"(?:{_CLAUSE_START}{_SEP_CH}*(?:{_FREE_TOKEN}){{0,{NEGATION_WINDOW_TOKENS}}}"   # clause start within window
    rf"|{_BEGIN}{_TOKEN}{_END}{_SEP_CH}+(?:{_FREE_TOKEN}){{{NEGATION_WINDOW_TOKENS}}})"  # or a full cue-free window
)


def wrap(claim_pattern):
    """Exported pattern: negation-window prefix + the claim as capture group 1."""
    return NEGATION_PREFIX + "(" + claim_pattern + ")"


D10 = "docs/research/sela-relay/10_EVIDENCE_APPRAISAL_AND_CLAIMS.md"
D06 = "docs/research/sela-relay/06_RESEARCH_METHOD.md"
D07 = "docs/research/sela-relay/07_EVIDENCE_TO_FEATURE.md"
D08 = "docs/research/sela-relay/08_ROADMAP_AND_VALIDATION.md"
D13 = "docs/research/sela-relay/13_JUDGING_AND_DEMO_PACKAGE.md"

# Words that end a "Sela is a/an ... therapist" phrase: "a bridge to a therapist" is navigation, not a claim.
_EN_STOP = r"(?!(?:to|for|with|and|or|of|between|before|from|towards?|alongside|beside|like|than)\b)"
_ID_STOP = (r"(?!(?:menuju|ke|untuk|dengan|dan|atau|bagi|kepada|pada|antara|bersama|dari|sebelum|seperti|"
            r"daripada)\b)")

CLAIM_GATES = [
    {
        "id": "clinical_validation",
        "claim_en": "Sela is clinically validated / clinically proven",
        "claim_id": "Sela tervalidasi / terbukti secara klinis",
        "claim_patterns": [
            r"\bclinically[- ](?:validated|proven|tested|approved)\b",
            r"\bclinical[- ]grade\b",
            r"\bter(?:validasi|bukti|uji)\s+(?:secara\s+)?klinis\b",
            r"\b(?:sudah|telah)\s+divalidasi\s+(?:secara\s+)?klinis\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 2 (Clinical language generation is not clinical readiness)"},
            {"doc": D06, "section": "Langkah appraisal sebelum klaim ilmiah kuat, item 6"},
            {"doc": D13, "section": "Evaluation protocol: compare against meaningful baselines"},
        ],
        "forbidden_examples": ["Sela is clinically validated for young adults.",
                               "Aplikasi ini sudah teruji klinis.",
                               "Sela terbukti secara klinis membantu Anda."],
        "allowed_examples": ["Sela is not clinically validated.",
                             "Sela belum tervalidasi secara klinis.",
                             "Validasi klinis Sela tetap membutuhkan studi tersendiri.",
                             "We make no claim that Sela is clinically validated."],
    },
    {
        "id": "efficacy",
        "claim_en": "Sela is proven effective / has demonstrated efficacy",
        "claim_id": "Sela terbukti efektif",
        "claim_patterns": [
            r"\bproven\s+(?:to\s+be\s+)?effective\b",
            r"\b(?:clinically|scientifically)\s+effective\b",
            r"\b(?:demonstrated|proven)\s+efficacy\b",
            r"\bterbukti\s+(?:efektif|ampuh|manjur|bermanfaat)\b",
            r"\befektif\s+(?:mengatasi|menyembuhkan|mengobati)\b",
        ],
        "sources": [
            {"doc": D07, "section": "Peta bukti → keputusan produk (R097; R100: 'Efek produk lain tidak dapat diklaim sebagai efikasi Sela')"},
            {"doc": D10, "section": "Claim-to-feature evidence gate ('design hypotheses, not causal validation')"},
            {"doc": D13, "section": "Evaluation protocol ('Do not claim clinical efficacy')"},
        ],
        "forbidden_examples": ["Sela is proven to be effective.", "Sela terbukti efektif untuk mahasiswa.",
                               "Our approach has demonstrated efficacy."],
        "allowed_examples": ["Efficacy has not been studied.",
                             "Sela tidak terbukti efektif dan tidak mengklaim manfaat klinis.",
                             "Kami tidak mengklaim Sela terbukti efektif."],
    },
    {
        "id": "reduces_symptoms",
        "claim_en": "Sela reduces/treats/cures depression, anxiety or other symptoms",
        "claim_id": "Sela mengurangi/mengatasi/menyembuhkan depresi, kecemasan atau gejala",
        "claim_patterns": [
            r"\b(?:reduces?|reducing|lowers?|decreases?|alleviates?|relieves?|cures?|treats?|heals?|eliminates?|"
            r"overcomes?|overcoming|beats?|fights?|combats?)\s+"
            r"(?:your\s+|the\s+|their\s+)?(?:symptoms\s+of\s+)?(?:depression|depressive\s+symptoms|anxiety|"
            r"symptoms|ptsd|trauma|burnout|loneliness)\b",
            r"\b(?:mengurangi|menurunkan|meredakan|menyembuhkan|mengobati|menghilangkan|mengatasi|menangani|"
            r"memulihkan|melawan)\s+(?:gejala\s+)?(?:depresi|kecemasan|ansietas|cemas|stres|trauma|gejala|"
            r"gangguan\s+(?:jiwa|mental|kecemasan))\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 2 ('Do not claim ... treatment')"},
            {"doc": D06, "section": "Jangan hitung bukti dua kali"},
            {"doc": D13, "section": "Evaluation protocol ('Do not claim clinical efficacy')"},
        ],
        "forbidden_examples": ["Sela reduces depression in two weeks.", "Sela membantu menurunkan kecemasan.",
                               "Use Sela to relieve anxiety.", "Sela membantu mengatasi depresi."],
        "allowed_examples": ["Sela does not treat depression or anxiety.",
                             "Sela tidak mengobati depresi dan bukan layanan klinis."],
    },
    {
        "id": "suicide_prevention",
        "claim_en": "Sela prevents suicide / saves lives",
        "claim_id": "Sela mencegah bunuh diri",
        "claim_patterns": [
            r"\b(?:prevents?|preventing|stops?)\s+suicides?\b",
            r"\bsaves?\s+lives\b",
            r"\bmencegah\s+(?:tindakan\s+|upaya\s+)?bunuh\s+diri\b",
            r"\bmenyelamatkan\s+nyawa\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 5 ('These external evaluations do not prove Sela is safe')"},
            {"doc": D10, "section": "Five design conclusions, item 2 ('Do not claim ... risk assessment')"},
            {"doc": D07, "section": "Peta bukti → keputusan produk (R099; R075; R081)"},
        ],
        "forbidden_examples": ["Sela prevents suicide among students.", "Fitur ini mencegah bunuh diri.",
                               "An app that saves lives."],
        "allowed_examples": ["If you are in danger, contact local emergency services; Sela does not provide crisis care.",
                             "Sela tidak mencegah bunuh diri; hubungi layanan darurat setempat."],
    },
    {
        "id": "safe_or_certified",
        "claim_en": "Sela is proven/guaranteed safe, certified or compliant",
        "claim_id": "Sela dijamin aman, tersertifikasi atau patuh regulasi",
        "claim_patterns": [
            r"\b(?:proven|guaranteed|certified|clinically)\s+safe\b",
            r"\bsafety[- ]certified\b",
            r"\b100\s?%\s+(?:safe|secure|private|accurate)\b",
            r"\bzero\s+risk\b",
            r"\b(?:hipaa|gdpr|uu\s*pdp|iso\s*27001)[- ](?:compliant|certified)\b",
            r"\bfully\s+compliant\b",
            r"\b(?:terbukti|dijamin|pasti)\s+aman\b",
            r"\b(?:tersertifikasi|bersertifikat|bersertifikasi)\b",
            r"\b(?:aman|privat|akurat)\s+100\s?%",
            r"\b100\s?%\s+(?:aman|privat|akurat|terjamin)\b",
            r"\btanpa\s+risiko\b",
            r"\b(?:patuh|sesuai|memenuhi)\s+(?:dengan\s+)?(?:uu\s*pdp|regulasi\s+privasi)\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 5 ('do not prove Sela is safe')"},
            {"doc": D07, "section": "Peta bukti → keputusan produk ('Rule gate dan pembatasan scope bukan sertifikasi keselamatan'; 'Framework consent bukan jaminan kepatuhan Indonesia')"},
            {"doc": D13, "section": "Evaluation protocol ('Do not claim ... safety certification')"},
        ],
        "forbidden_examples": ["Sela is guaranteed safe.", "Data Anda 100% aman dan privat.",
                               "Sela sudah tersertifikasi.", "A HIPAA-compliant companion."],
        "allowed_examples": ["Sela is a demo with synthetic data and makes no safety certification claim.",
                             "Sela tidak tersertifikasi dan tidak dijamin aman untuk penggunaan klinis."],
    },
    {
        "id": "diagnosis",
        "claim_en": "Sela diagnoses / gives a diagnosis",
        "claim_id": "Sela mendiagnosis / memberi diagnosis",
        "claim_patterns": [
            r"\b(?:sela|we|it|the\s+app|our\s+(?:app|ai|model))\s+(?:can\s+|will\s+)?diagnos(?:e|es)\b",
            r"\b(?:get|receive|obtain)\s+(?:a\s+|your\s+)?diagnosis\b",
            r"\bdiagnostic\s+(?:tool|assessment|test|screening)\b",
            r"\bmendiagnosis\b",
            r"\b(?:dapatkan|mendapat(?:kan)?|memberi(?:kan)?)\s+diagnosis\b",
            r"\balat\s+diagnosis\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 2 ('Do not claim autonomous counseling, diagnosis, risk assessment or treatment')"},
            {"doc": D06, "section": "Apa yang sudah dilakukan ('Pemanfaatan hasil yang berkaitan dengan diagnosis atau terapi dibatasi')"},
        ],
        "forbidden_examples": ["Sela can diagnose anxiety.", "Get your diagnosis in minutes.",
                               "Sela mendiagnosis kondisi Anda.", "Dapatkan diagnosis instan."],
        "allowed_examples": ["Sela does not diagnose.", "Sela tidak mendiagnosis.",
                             "Sela bukan untuk mendiagnosis kondisi apa pun.",
                             "Sela is not a diagnostic tool."],
    },
    {
        "id": "therapy_or_counselling",
        "claim_en": "Sela is a therapist / provides therapy, counselling or treatment / replaces professionals",
        "claim_id": "Sela adalah terapis / memberikan terapi atau konseling / menggantikan profesional",
        "claim_patterns": [
            r"\b(?:ai|virtual|digital|personal|robot|chatbot)\s+(?:therapist|counsel(?:l)?or|psychologist|"
            r"psychiatrist|psychotherapist)s?\b",
            r"\b(?:sela|this\s+app|the\s+app|our\s+app)\s+(?:is|acts\s+as|works\s+as|serves\s+as|becomes)\s+"
            r"(?:a|an|your)\s+(?:" + _EN_STOP + r"[a-z-]+\s+){0,2}?"
            r"(?:therapist|counsel(?:l)?or|psychologist|psychiatrist|psychotherapist|doctor|clinician)\b",
            r"\b(?:provides?|offers?|delivers?)\s+(?:online\s+)?(?:therapy|psychotherapy|counsel(?:l)?ing|treatment)\b",
            r"\b(?:replaces?|replacing|substitutes?\s+for)\s+(?:a\s+|your\s+)?(?:therapist|psychologist|psychiatrist|"
            r"professional|clinician)s?\b",
            r"\b(?:terapis|psikolog|psikiater|konselor)\s+(?:ai|virtual|digital|pribadi)\b",
            r"\b(?:sela|aplikasi\s+ini)\s+(?:adalah|merupakan|ialah|berperan\s+sebagai|bertindak\s+sebagai|menjadi)\s+"
            r"(?:(?:seorang|sebuah|satu)\s+)?(?:" + _ID_STOP + r"[a-z-]+\s+){0,2}?"
            r"(?:terapis|psikolog|psikiater|psikoterapis|konselor|dokter)\b",
            r"\b(?:memberikan|menyediakan|menawarkan)\s+(?:layanan\s+)?(?:terapi|psikoterapi|konseling|pengobatan)\b",
            r"\b(?:pengganti|menggantikan)\s+(?:terapis|psikolog|psikiater|tenaga\s+profesional|profesional)\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 2 ('Do not claim autonomous counseling ... or treatment')"},
            {"doc": D08, "section": "Kartu uji owner O12 ('tidak ada obat/terapi dari model')"},
            {"doc": D07, "section": "Peta bukti → keputusan produk (R099: 'Batasi model sebagai drafting/navigation assistant')"},
        ],
        "forbidden_examples": ["Meet your AI therapist.", "Sela provides therapy.", "Sela menggantikan psikolog.",
                               "Sela memberikan konseling 24 jam.", "Sela adalah terapis.", "Sela adalah psikolog AI.",
                               "Sela is an evidence-based therapist.", "Sela is not a chatbot but an AI therapist.",
                               "Sela bukan aplikasi biasa, melainkan terapis AI."],
        "allowed_examples": ["Sela is not a therapist and does not provide therapy.",
                             "Sela bukan terapis dan tidak memberikan terapi.",
                             "Sela helps you prepare to talk with a professional.",
                             "Sela is a bridge to professional support, not an AI therapist.",
                             "Sela is not a substitute for a therapist.",
                             "Sela doesn't replace your psychologist.",
                             "Sela doesn’t replace your psychologist.",
                             "Share this note with your therapist.",
                             "Sela adalah jembatan menuju psikolog."],
    },
    {
        "id": "risk_assessment_or_triage",
        "claim_en": "Sela assesses suicide/mental-health risk or triages",
        "claim_id": "Sela menilai risiko atau melakukan triase",
        "claim_patterns": [
            r"\b(?:assess(?:es)?|assessing|evaluates?|predicts?|scores?)\s+(?:your\s+)?(?:suicide\s+|suicidal\s+|"
            r"crisis\s+|mental[- ]health\s+|clinical\s+)?risk\b",
            r"\brisk\s+scor(?:e|es|ing)\b",
            r"\b(?:ai|clinical|medical|automated)\s+triage\b",
            r"\bmenilai\s+(?:tingkat\s+)?risiko\b",
            r"\bskor\s+risiko\b",
            r"\b(?:melakukan\s+)?triase\s+(?:klinis|medis|otomatis|ai)\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 2 ('Do not claim ... risk assessment')"},
            {"doc": D07, "section": "Peta bukti → keputusan produk (R099; R075; R081)"},
        ],
        "forbidden_examples": ["Sela assesses your suicide risk.", "Get a personal risk score.",
                               "Sela menilai risiko Anda secara otomatis."],
        "allowed_examples": ["Sela does not assess risk; it shows help information.",
                             "Sela tidak menilai risiko dan tidak melakukan triase.",
                             "Sela cannot assess your suicide risk."],
    },
    {
        "id": "prescribing",
        "claim_en": "Sela prescribes or recommends medication",
        "claim_id": "Sela meresepkan atau merekomendasikan obat",
        "claim_patterns": [
            r"\b(?:prescribes?|prescribing|recommends?)\s+(?:a\s+)?(?:medication|medicine|drugs?|antidepressants?|dosage)\b",
            r"\b(?:meresepkan|merekomendasikan|menyarankan)\s+(?:obat|antidepresan|dosis)\b",
        ],
        "sources": [
            {"doc": D08, "section": "Kartu uji owner O12 ('Rencana hanya dari fixture; tidak ada obat/terapi dari model')"},
            {"doc": D10, "section": "Five design conclusions, item 2"},
        ],
        "forbidden_examples": ["Sela recommends medication based on your story.", "Sela meresepkan obat."],
        "allowed_examples": ["Sela never recommends medication.", "Sela tidak meresepkan obat.",
                             "Sela does not diagnose, treat or prescribe medication."],
    },
    {
        "id": "care_received_or_booking",
        "claim_en": "Care was received / a real appointment was booked",
        "claim_id": "Bantuan telah diterima / janji temu nyata telah dipesan",
        "claim_patterns": [
            r"\byou\s+(?:have\s+)?received\s+(?:care|treatment|therapy)\b",
            r"\byour\s+(?:real\s+)?appointment\s+(?:is|has\s+been)\s+(?:confirmed|booked)\b",
            r"\b(?:anda|kamu)\s+(?:telah|sudah)\s+(?:menerima|mendapat(?:kan)?)\s+(?:bantuan|perawatan|terapi)\b",
            r"\bjanji\s+temu\s+(?:anda\s+|kamu\s+)?(?:telah|sudah)\s+(?:dikonfirmasi|dipesan)\b",
        ],
        "sources": [
            {"doc": D08, "section": "Kartu uji owner O10 ('UI belum menyebut bantuan diterima/konsultasi selesai')"},
            {"doc": D10, "section": "Five design conclusions, item 4 (all actors and events remain simulated)"},
        ],
        "forbidden_examples": ["You have received care from Dr. Rina.", "Janji temu Anda telah dikonfirmasi.",
                               "Request sent. You have received care."],
        "allowed_examples": ["This is a simulated handoff; no real appointment was booked.",
                             "Ini simulasi; tidak ada janji temu nyata.",
                             "A sent request does not mean you have received care.",
                             "Sent or acknowledged does not mean you have received care. All of this is simulated.",
                             "Permintaan yang terkirim belum berarti kamu sudah mendapat perawatan.",
                             "Terkirim atau diterima belum berarti kamu sudah mendapat perawatan. Semua ini simulasi."],
    },
    {
        "id": "evidence_count_overclaim",
        "claim_en": "Sela is backed/proven by 100 studies; research proves Sela; Sela is evidence-based",
        "claim_id": "Sela didukung/dibuktikan oleh 100 studi; penelitian membuktikan Sela; Sela berbasis bukti",
        "claim_patterns": [
            r"\b(?:backed|supported|proven|validated)\s+by\s+(?:over\s+|more\s+than\s+)?(?:100|one\s+hundred|\d{2,})\s+"
            r"(?:studies|papers|publications|trials|rcts)\b",
            r"\b(?:100|one\s+hundred)\s+(?:studies|papers|trials|rcts)\s+(?:show|prove|confirm)\b",
            r"\b(?:research|studies|science)\s+(?:shows?|proves?)\s+(?:that\s+)?sela\b",
            r"\bsela\s+is\s+(?:an?\s+)?(?:evidence[- ]based|scientifically\s+proven|research[- ]backed|science[- ]backed)\b",
            r"\b(?:didukung|dibuktikan|divalidasi)\s+(?:oleh\s+)?(?:lebih\s+dari\s+)?(?:100|seratus|\d{2,})\s+"
            r"(?:studi|penelitian|publikasi|jurnal|riset)\b",
            r"\b(?:100|seratus)\s+(?:studi|penelitian|publikasi)\s+membuktikan\b",
            r"\b(?:penelitian|riset|studi|sains)\s+membuktikan\s+(?:bahwa\s+)?sela\b",
            r"\bsela\s+(?:adalah\s+)?(?:berbasis\s+bukti|terbukti\s+secara\s+ilmiah)\b",
        ],
        "sources": [
            {"doc": D06, "section": "Jangan hitung bukti dua kali ('Seratus publikasi bukan ... seratus pembuktian Sela')"},
            {"doc": D06, "section": "Kriteria kesiapan riset produk ('daripada menempelkan angka 100 pada landing page')"},
            {"doc": D10, "section": "Verified local register integrity ('cannot be called 100 independent demonstrations of efficacy')"},
            {"doc": D07, "section": "Peta bukti → keputusan produk ('Sumber tidak menguji Sela')"},
        ],
        "forbidden_examples": ["Sela is backed by 100 studies.", "Sela didukung oleh 100 penelitian.",
                               "Research proves Sela works.", "Sela is an evidence-based therapist.",
                               "Sela berbasis bukti ilmiah."],
        "allowed_examples": ["The design was informed by a 100-publication evidence map that does not test Sela.",
                             "Peta bukti 100 publikasi tidak menguji Sela."],
    },
    {
        "id": "solves_access_barriers",
        "claim_en": "Sela solves affordability, provider shortages or all barriers",
        "claim_id": "Sela mengatasi biaya, kekurangan tenaga profesional atau semua hambatan",
        "claim_patterns": [
            r"\bsolves?\s+(?:the\s+)?(?:affordability|cost|access|provider\s+shortages?|shortages?)\b",
            r"\b(?:eliminates?|removes?)\s+(?:all\s+)?(?:barriers|stigma)\b",
            r"\bmengatasi\s+(?:masalah\s+)?(?:biaya|keterjangkauan|kekurangan\s+(?:tenaga|psikolog|psikiater|profesional))\b",
            r"\bmenghapus\s+(?:semua\s+)?(?:hambatan|stigma)\b",
        ],
        "sources": [
            {"doc": D10, "section": "Five design conclusions, item 1 ('Do not claim that a directory solves affordability or provider shortages')"},
            {"doc": D07, "section": "Peta bukti → keputusan produk (R098; R019: 'Sebuah app tidak menghapus kekurangan tenaga profesional')"},
        ],
        "forbidden_examples": ["Sela solves the provider shortage.", "Sela menghapus semua hambatan menuju layanan."],
        "allowed_examples": ["Sela cannot fix provider shortages or costs.", "Sela tidak menghapus hambatan biaya."],
    },
    {
        "id": "statistical_or_adherence",
        "claim_en": "Statistically significant results / improved treatment adherence from demo data",
        "claim_id": "Hasil signifikan secara statistik / meningkatkan kepatuhan pengobatan",
        "claim_patterns": [
            r"\bstatistically\s+significant\b",
            r"\bimproves?\s+(?:treatment\s+|therapy\s+|medication\s+)?adherence\b",
            r"\bsignifikan\s+secara\s+statistik\b",
            r"\bmeningkatkan\s+kepatuhan\s+(?:pengobatan|terapi|minum\s+obat)\b",
        ],
        "sources": [
            {"doc": D13, "section": "Evaluation protocol ('Do not claim ... statistical significance, improved treatment adherence')"},
            {"doc": D07, "section": "Rancangan evaluasi ('tidak ada klaim signifikansi atau power')"},
        ],
        "forbidden_examples": ["Users showed statistically significant improvement.", "Sela meningkatkan kepatuhan pengobatan."],
        "allowed_examples": ["Small demo samples are descriptive only.", "Sampel demo kecil hanya deskriptif."],
    },
]


def exported_gates(gates=None):
    """Gates in their published form: ``patterns`` = wrapped ``claim_patterns``."""
    out = []
    for g in gates or CLAIM_GATES:
        e = dict(g)
        if "claim_patterns" in g:
            e["patterns"] = [wrap(p) for p in g["claim_patterns"]]
        out.append(e)
    return out


def compiled_gates(gates=None):
    """[(gate_id, [wrapped patterns], [bare claim patterns])]; accepts internal or exported gates."""
    out = []
    for g in exported_gates(gates):
        bare = [re.compile(p, re.IGNORECASE) for p in g.get("claim_patterns", [])]
        out.append((g["id"], [re.compile(p, re.IGNORECASE) for p in g["patterns"]], bare))
    return out


def scan_text(text, gates=None, include_negated=False):
    """Return [{'gate','pattern_index','match','start'}] for every forbidden-claim hit.

    ``match``/``start`` are the claim itself (capture group 1), not the window
    prefix. With ``include_negated``, bare claim matches that the negation
    window suppressed are added with ``negated: True`` (informational only).
    """
    text = text or ""
    hits = []
    for gate_id, patterns, bare in compiled_gates(gates):
        for i, pattern in enumerate(patterns):
            starts = set()
            for m in pattern.finditer(text):
                g = 1 if m.re.groups else 0
                starts.add(m.start(g))
                hits.append({"gate": gate_id, "pattern_index": i, "match": m.group(g), "start": m.start(g)})
            if include_negated and i < len(bare):
                for m in bare[i].finditer(text):
                    if m.start() not in starts:
                        hits.append({"gate": gate_id, "pattern_index": i, "match": m.group(0), "start": m.start(),
                                     "negated": True})
    return sorted(hits, key=lambda h: (h["start"], h["gate"], h["pattern_index"], h.get("negated", False)))


def export_document():
    return {
        "version": GATES_VERSION,
        "purpose": ("Forbidden product claims for Sela copy (ID/EN). Case-insensitive regex; "
                    "a hit requires human review and rewording. Not exhaustive."),
        "flags": "IGNORECASE",
        "usage": ("Apply each entry of 'patterns' as a case-insensitive regex (Python re.IGNORECASE; JavaScript "
                  "new RegExp(p, 'iu')); any match is a hit and capture group 1 is the claim text. "
                  "'claim_patterns' are the bare claims, for reading only: they have no negation handling."),
        "negation_handling": {
            "rule": (f"a claim matches only if none of the {NEGATION_WINDOW_TOKENS} tokens before it in the same "
                     "clause is a negation cue; a scope breaker starts a new scope"),
            "window_tokens": NEGATION_WINDOW_TOKENS,
            "negation_cues": list(NEGATION_CUES) + ["<word>n't (straight or curly apostrophe)"],
            "scope_breakers": list(SCOPE_BREAKERS),
            "clause_boundaries": ". ! ? ; : followed by whitespace or end; line break; em dash; en dash; spaced hyphen",
            "known_blind_spot": ("a negation cue in the window that belongs to another idea suppresses the hit, "
                                 "e.g. 'Never feel alone with your AI therapist'"),
            "engines": "Python re and JavaScript (lookbehind); not Go RE2 (no lookaround)",
        },
        "gates": exported_gates(),
    }
