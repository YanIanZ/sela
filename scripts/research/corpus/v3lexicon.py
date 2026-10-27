"""v3 lexicon families for MACHINE title/abstract screening (``sela-v3-tiab-screen-v1.0``).

Every family is a list of explicit regular expressions; a hit records the
family, the field (title or abstract) and the entry LABEL, never the matched
text, so committed outputs carry no abstract wording. The families follow the
v3 feature list (listening room, journal, check-in, timeline/continuity,
memory correction, consultation preparation, sharing and revocation, minors,
accessibility, older adults, Indonesian context, safety, privacy) plus the
section 9 topics (help-seeking, AI handling of Bahasa Indonesia and
code-switching, health-data privacy law).

This is lexical triage, not appraisal: a hit means "a person should read this
abstract", nothing more. Changing any entry requires a new version.
"""
from __future__ import annotations

import re

LEXICON_VERSION = "sela-v3-families-1"
I = re.IGNORECASE


def _c(entries):
    return tuple((label, re.compile(rx, I)) for label, rx in entries)


FAMILIES = {
    "listening_feeling_heard": _c((
        ("feel heard", r"\bfe(?:el|els|eling|lt)\s+(?:truly\s+|really\s+)?heard\b"),
        ("being heard", r"\bbeing\s+(?:heard|listened\s+to)\b"),
        ("feel understood", r"\bfe(?:el|els|eling|lt)\s+understood\b"),
        ("active/empathic listening", r"\b(?:active|empathic|reflective|compassionate|deep)\s+listening\b"),
        ("perceived empathy", r"\bperceived\s+empathy\b|\bempathic\s+(?:responses?|communication|accuracy)\b"),
        ("non-judgemental", r"\bnon[- ]?judg(?:e)?mental\b"),
        ("alliance", r"\b(?:therapeutic|working|digital)\s+alliance\b"),
    )),
    "journaling_expressive_writing": _c((
        ("expressive writing", r"\bexpressive\s+writing\b"),
        ("journaling", r"\bjournal(?:l)?ing\b"),
        ("reflective writing", r"\breflective\s+writing\b|\bwriting\s+(?:intervention|exercise|task)s?\b"),
        ("diary intervention", r"\b(?:diary|journal)\s+(?:intervention|writing|app|application|feature)s?\b"),
        ("gratitude journal", r"\bgratitude\s+(?:journal|diary|writing|letters?)\b"),
        ("emotional disclosure", r"\b(?:emotional|written)\s+disclosure\b"),
    )),
    "check_in_self_monitoring": _c((
        ("mood tracking", r"\bmood\s+(?:tracking|tracker|monitoring|diar(?:y|ies)|ratings?|logs?)\b"),
        ("self-monitoring", r"\bself[- ]monitoring\b|\bself[- ]tracking\b"),
        ("ecological momentary assessment", r"\becological\s+momentary\s+assessments?\b|\bexperience\s+sampling\b"),
        ("check-in", r"\bcheck[- ]ins?\b"),
        ("symptom tracking", r"\bsymptom\s+(?:tracking|monitoring|diar(?:y|ies))\b"),
        ("measurement-based care", r"\bmeasurement[- ]based\s+care\b|\broutine\s+outcome\s+monitoring\b"),
    )),
    "continuity_after_consultation": _c((
        ("visit summary", r"\b(?:after[- ]visit|post[- ]visit|visit|clinic(?:al)?\s+visit)\s+summar(?:y|ies)\b"),
        ("open notes", r"\bopen\s*notes\b|\bpatients?\s+(?:reading|accessing)\s+(?:their\s+)?(?:clinical\s+)?notes\b"),
        ("patient portal", r"\bpatient\s+portals?\b"),
        ("between sessions", r"\bbetween[- ](?:sessions?|visits?|appointments?)\b"),
        ("continuity of care", r"\bcontinuity\s+of\s+care\b|\bcare\s+transitions?\b"),
        ("patient-held record", r"\bpatient[- ]held\s+records?\b|\bpersonal\s+health\s+records?\b"),
    )),
    "memory_correction_source_fidelity": _c((
        ("hallucination", r"\bhallucinat(?:ion|ions|ed|es)\b|\bconfabulat\w*\b"),
        ("faithfulness", r"\bfaithful(?:ness)?\b|\bfactual(?:ity|\s+(?:accuracy|consistency|errors?))\b"),
        ("summary accuracy", r"\bsummar(?:y|ies|isation|ization)\s+(?:accuracy|errors?|quality)\b"),
        ("record errors", r"\berrors?\s+in\s+(?:their\s+)?(?:clinical\s+)?(?:notes|records|documentation)\b"),
        ("patient correction", r"\bpatients?\s+(?:report(?:ed)?|correct(?:ed)?|flag(?:ged)?)\s+(?:errors?|inaccuracies|mistakes)\b"),
    )),
    "consultation_preparation": _c((
        ("question prompt list", r"\bquestion\s+prompt\s+(?:lists?|sheets?)\b|\bprompt\s+lists?\b"),
        ("pre-visit", r"\bpre[- ]?(?:visit|consultation|appointment)\b"),
        ("preparing for a consultation", r"\bprepar(?:e|ing|ation)\s+(?:for\s+)?(?:a\s+|the\s+|their\s+)?"
                                         r"(?:consultations?|visits?|appointments?)\b"),
        ("agenda setting", r"\bagenda[- ]setting\b"),
        ("patient-generated summary", r"\bpatient[- ]generated\s+(?:summar(?:y|ies)|health\s+data|data)\b"),
        ("communication aid", r"\bcommunication\s+(?:aid|tool)s?\b|\bquestion\s+lists?\b"),
        ("shared decision-making", r"\bshared\s+decision[- ]making\b|\bdecision\s+aids?\b"),
    )),
    "sharing_and_revocation": _c((
        ("dynamic/granular consent", r"\b(?:dynamic|granular|tiered|broad)\s+consent\b"),
        ("consent withdrawal", r"\b(?:withdraw(?:al|ing)?\s+(?:of\s+)?consent|consent\s+(?:withdrawal|revocation))\b"),
        ("revocation of consent or access", r"\b(?:revoc(?:ation|able)|revok(?:e|ed|ing))\b[^.]{0,40}\b(?:consent|access|"
                                            r"data|permission|sharing)\b|\b(?:consent|access|permission)s?\b[^.]{0,40}"
                                            r"\b(?:revoc(?:ation|able)|revok(?:e|ed|ing))\b"),
        ("patient-controlled sharing", r"\bpatient[- ]controlled\s+(?:access|sharing|records?)\b"),
        ("data sharing preferences", r"\b(?:data|information)\s+sharing\s+(?:preferences|controls?|decisions)\b"),
        ("proxy access", r"\bproxy\s+access\b|\bselective\s+disclosure\b"),
    )),
    "adolescents": _c((
        ("adolescents", r"\badolescen(?:t|ts|ce)\b"),
        ("teenagers", r"\bteen(?:age|agers?|s)?\b"),
        ("school students", r"\b(?:high|secondary|middle)\s+schools?\b"),
        ("minors", r"\bminors\b|\bunder\s+18\b|\bunder[- ]age\b"),
        ("aged 12-17", r"\baged?\s+1[2-7]\b|\b1[2-7]\s*(?:-|–|to)\s*1[5-9]\s*(?:years?|y)\b"),
    )),
    "adolescent_consent_confidentiality": _c((
        ("parental consent", r"\b(?:parental|parent|guardian)\s+(?:consent|permission|notification|involvement)\b"),
        ("parental access", r"\b(?:parental|parent|guardian|proxy)\s+access\b"),
        ("assent", r"\bassent\b"),
        ("confidentiality", r"\bconfidential(?:ity)?\b"),
        ("minor consent", r"\b(?:mature\s+)?minors?['’]?\s+consent\b|\bage\s+of\s+consent\b|\bwithout\s+parental\b"),
    )),
    "older_adults_50_plus": _c((
        ("older adults", r"\bolder\s+(?:adults?|people|persons|patients|individuals|women|men)\b"),
        ("elderly", r"\belderly\b|\bseniors?\b|\bgeriatric\b"),
        ("late life", r"\blate[- ]life\b|\blater\s+life\b"),
        ("aged 50 plus", r"\baged?\s+(?:50|55|60|65)\b|\b(?:50|55|60|65)\s+years?\s+(?:and|or)\s+(?:older|over)\b"),
        ("middle-aged", r"\bmiddle[- ]aged\b|\bmidlife\b|\bmid[- ]life\b"),
    )),
    "accessibility_disability": _c((
        # 'accessibility' alone usually means access to services in health research; only
        # the digital/design sense counts here
        ("digital accessibility", r"\b(?:digital|web|app|application|mobile|interface|website|software)\s+"
                                  r"accessibility\b|\baccessibility\s+(?:guidelines?|standards?|features?|evaluation|"
                                  r"testing|audit|requirements|barriers\s+for)\b|\baccessible\s+(?:design|formats?|"
                                  r"interfaces?|technology)\b"),
        ("disability", r"\bdisabilit(?:y|ies)\b|\bdisabled\b"),
        ("sensory impairment", r"\bvisual(?:ly)?\s+impair\w*\b|\bblind(?:ness)?\b|\blow\s+vision\b|\bdeaf\w*\b|"
                               r"\bhard\s+of\s+hearing\b"),
        ("assistive technology", r"\bscreen\s+readers?\b|\bassistive\s+technolog\w*\b|\bwcag\b"),
        ("universal design", r"\buniversal\s+design\b|\binclusive\s+design\b"),
        ("cognitive accessibility", r"\bcognitive\s+(?:accessibility|load|impairments?\s+and\s+(?:app|technology|usability))\b"),
        ("literacy", r"\blow\s+(?:health\s+|digital\s+)?literacy\b|\bplain\s+language\b|\beasy[- ]read\b|\bdyslexi\w*\b"),
        ("readability", r"\breadability\b|\breading\s+level\b"),
    )),
    "indonesian_context": _c((
        ("Indonesia", r"\bindonesia(?:n)?\b"),
        ("Indonesian places", r"\b(?:jakarta|surabaya|yogyakarta|bandung|java|javanese|sumatra|bali|sulawesi|"
                              r"kalimantan|papua)\b"),
        ("Bahasa", r"\bbahasa\b"),
        ("Indonesian-language terms", r"\b(?:kesehatan\s+(?:mental|jiwa)|remaja|mahasiswa|aplikasi|layanan|"
                                      r"konseling|gangguan\s+(?:mental|jiwa))\b"),
    )),
    "health_data_privacy_law": _c((
        ("personal data protection law", r"\bpersonal\s+data\s+protection\b|\buu\s*pdp\b|"
                                         r"\b(?:pelindungan|perlindungan)\s+data\s+pribadi\b"),
        ("data protection regulation", r"\bdata\s+protection\s+(?:laws?|regulations?|acts?)\b|\bgdpr\b|\bhipaa\b"),
        ("health data governance", r"\bhealth\s+data\s+(?:privacy|protection|governance|security)\b"),
        ("cross-border data", r"\bcross[- ]border\s+data\b|\bdata\s+locali[sz]ation\b|\bdata\s+sovereignty\b"),
    )),
    "ai_language_code_switching": _c((
        ("code-switching", r"\bcode[- ]?(?:switch\w*|mix\w*)\b"),
        ("Bahasa Indonesia", r"\bbahasa\s+indonesia\b|\bindonesian\s+(?:language|text|tweets?|corpus)\b"),
        ("low-resource language", r"\blow[- ]resource\s+languages?\b|\bnon[- ]english\b|\bmultilingual\b|"
                                  r"\bcross[- ]lingual\b"),
    )),
    "help_seeking": _c((
        ("help-seeking", r"\bhelp[- ]seeking\b|\bseek(?:ing)?\s+(?:professional\s+)?help\b"),
        ("treatment seeking", r"\b(?:treatment|care)[- ]seeking\b"),
        ("barriers to care", r"\bbarriers?\s+to\s+(?:mental\s+health\s+)?(?:care|treatment|help|services)\b"),
        ("service use", r"\bservice\s+(?:use|utili[sz]ation)\b"),
    )),
    "safety": _c((
        ("suicide", r"\bsuicid\w*\b"),
        ("self-harm", r"\bself[- ]harm\w*\b|\bself[- ]injur\w*\b"),
        ("crisis", r"\bcris(?:is|es)\b"),
        ("risk detection", r"\brisk\s+(?:detection|assessment|identification)\b"),
        ("safety", r"\bsafety\b|\bharmful\b|\badverse\s+(?:events?|effects?)\b|\bred[- ]teaming\b"),
    )),
    "privacy": _c((
        ("privacy", r"\bprivacy\b"),
        ("confidentiality", r"\bconfidential(?:ity)?\b"),
        ("consent to data use", r"\bconsent\s+(?:to|for)\s+(?:data|sharing|use|the\s+use)\b|\bdata[- ]use\s+consent\b"),
        ("data sharing or security", r"\b(?:data|information)\s+(?:sharing|security|breach(?:es)?|governance|protection)\b"),
        ("personal or health data", r"\b(?:personal|sensitive|health)\s+(?:data|information)\b"),
    )),
}
MENTAL_HEALTH = _c((
    ("mental health", r"\bmental\s+(?:health|illness|disorders?|wellbeing|well-being)\b"),
    ("psychiatry/psychology", r"\bpsychiatr\w*\b|\bpsycholog\w*\b|\bpsychotherap\w*\b|\bcounsel(?:l)?ing\b"),
    ("depression/anxiety", r"\bdepress\w*\b|\banxi(?:ety|ous)\b"),
    ("wellbeing/distress", r"\bwell[- ]?being\b|\bdistress\b|\bloneliness\b|\bstress\b|\bemotion(?:al|s)?\b"),
    ("suicide/self-harm", r"\bsuicid\w*\b|\bself[- ]harm\w*\b"),
    ("Indonesian mental health terms", r"\bkesehatan\s+(?:mental|jiwa)\b|\bgangguan\s+(?:mental|jiwa)\b"),
))
DIGITAL = _c((
    ("app/mobile", r"\b(?:apps?|smartphones?|mobile|mhealth|m-health)\b"),
    ("digital/online", r"\bdigital\b|\bonline\b|\binternet\b|\bweb[- ]based\b|\btele(?:health|medicine|psychiatry)\b"),
    ("AI/chatbot", r"\bchatbots?\b|\bconversational\s+agents?\b|\blarge\s+language\s+models?\b|\bllms?\b|"
                   r"\bartificial\s+intelligence\b|\bgenerative\s+ai\b|\bnatural\s+language\s+processing\b"),
    ("patient portal", r"\bpatient\s+portals?\b|\belectronic\s+health\s+records?\b"),
))
DESIGN = _c((
    ("randomized trial", r"\brandomi[sz]ed\b"),
    ("systematic review", r"\bsystematic\s+review\b|\bmeta[- ]analys[ie]s\b|\bscoping\s+review\b"),
    ("qualitative", r"\bqualitative\b|\bfocus\s+groups?\b|\bsemi[- ]structured\s+interviews?\b"),
    ("cross-sectional", r"\bcross[- ]sectional\b|\bsurvey\b"),
    ("cohort/longitudinal", r"\bcohort\b|\blongitudinal\b|\bprospective\b|\bretrospective\b"),
    ("pilot/feasibility", r"\bpilot\b|\bfeasibility\b|\busability\b"),
))
NULL_OR_NEGATIVE = _c((
    ("no significant effect", r"\bno\s+(?:statistically\s+)?significant\s+(?:difference|effect|improvement|change|"
                              r"association|reduction|increase)s?\b"),
    ("did not improve", r"\bdid\s+not\s+(?:significantly\s+)?(?:improve|differ|reduce|increase|change|predict|reach)\b"),
    ("not significantly", r"\bnot\s+(?:statistically\s+)?significant(?:ly)?\b"),
    ("null finding", r"\bnull\s+(?:results?|findings?|effects?)\b|\bno\s+evidence\s+(?:of|that|for)\b"),
    ("harm", r"\b(?:worsen\w*|harmful|iatrogenic|negative\s+(?:effects?|outcomes?|impacts?))\b"),
    ("failure", r"\bfailed\s+to\b|\bineffective\b|\bterminated\b|\bdiscontinued\b"),
))
# Population signals (title/abstract words). Kept apart so adult and adolescent
# evidence are never merged.
POPULATION = {
    "adolescent_signal": FAMILIES["adolescents"],
    "older_adult_signal": FAMILIES["older_adults_50_plus"],
    "adult_signal": _c((
        ("adults", r"\badults?\b(?!\s+(?:and|or)\s+(?:adolescents|children))"),
        ("university students", r"\b(?:university|college|undergraduate)\s+students?\b|\bundergraduates?\b|"
                                r"\bmahasiswa\b"),
        ("aged 18 plus", r"\baged?\s+(?:18|19|2\d)\b|\b(?:18|19)\s*(?:-|–|to)\s*\d{2}\s*(?:years?|y)\b"),
        ("workers/employees", r"\bemployees?\b|\bworkers?\b"),
    )),
}
FEATURE_FAMILIES = tuple(k for k in FAMILIES if k not in ("adolescents",))


def hits(lexicon, text):
    return sorted({label for label, rx in lexicon if rx.search(text or "")})


def family_hits(title, abstract):
    """{family: {"title": [labels], "abstract": [labels]}} for families with any hit."""
    out = {}
    for fam, lex in FAMILIES.items():
        t, a = hits(lex, title), hits(lex, abstract)
        if t or a:
            out[fam] = {"title": t, "abstract": a}
    return out


# --- versioned lexicons ------------------------------------------------------------------
# ``sela-v3-families-1`` above stays exactly as it was (it reproduces screening v1.0).
# ``sela-v3-families-2`` (screening v1.1) adds Indonesian age, feature and consent terms
# and a working-age/mid-life adults family. Every v1 entry is kept unchanged; v2 only adds.

class Lexicon:
    """One frozen lexicon version: families, gates' word lists and population signals."""

    def __init__(self, version, families, mental_health, digital, design, null_or_negative, population,
                 feature_families):
        self.version = version
        self.FAMILIES = families
        self.MENTAL_HEALTH = mental_health
        self.DIGITAL = digital
        self.DESIGN = design
        self.NULL_OR_NEGATIVE = null_or_negative
        self.POPULATION = population
        self.FEATURE_FAMILIES = feature_families
        self.AI_TERMS = tuple(e for e in digital if e[0] == "AI/chatbot")

    def hits(self, lexicon, text):
        return hits(lexicon, text)

    def family_hits(self, title, abstract):
        out = {}
        for fam, lex in self.FAMILIES.items():
            t, a = hits(lex, title), hits(lex, abstract)
            if t or a:
                out[fam] = {"title": t, "abstract": a}
        return out


FAMILIES_1 = Lexicon(LEXICON_VERSION, FAMILIES, MENTAL_HEALTH, DIGITAL, DESIGN, NULL_OR_NEGATIVE, POPULATION,
                     FEATURE_FAMILIES)


def _extend(base, entries):
    return tuple(base) + _c(entries)


# Indonesian additions (audit finding: population signals and feature families were English-only,
# so Indonesian-titled studies of teenagers or older adults got population_band not_stated).
_ID_ADOLESCENT = (
    ("remaja (adolescents)", r"\bremaja\b"),
    ("anak (children)", r"\banak(?:-anak)?\b(?!\s+(?:perusahaan|buah)\b)"),
    # bare SMA/SMP/SMK are not used: 'SMA' is also spinal muscular atrophy in English abstracts
    ("siswa SMA/SMP/SMK (school students)", r"\bsiswa\b|\bsekolah\s+(?:menengah|dasar)\b|"
                                            r"(?-i:\b(?:SMA|SMP|SMK|MTs)\s+(?:Negeri|Swasta|N\s?\d|\d))"),
    ("pelajar (pupils)", r"\bpelajar\b"),
    ("usia 12-17 tahun", r"\b(?:usia|umur|berusia|berumur)\s+1[2-7]\s*(?:-|–|sampai|hingga)\s*1[5-9]\s*tahun\b"),
)
_ID_OLDER = (
    ("lansia (older people)", r"\blansia\b"),
    ("lanjut usia (older age)", r"\blanjut\s+usia\b|\busia\s+lanjut\b|\bwarga\s+senior\b"),
    ("usia 60 tahun ke atas", r"\b(?:usia|umur|berusia)\s+(?:50|55|60|65)\s+tahun\s+(?:ke\s+atas|atau\s+lebih)\b"),
)
_WORKING_AGE = (
    ("working-age adults", r"\bworking[- ]age\b|\bworking\s+adults?\b"),
    ("employees/workers", r"\bemployees?\b|\bworkers?\b|\bworkplace\b|\boccupational\b|\bburnout\b"),
    ("general adult population", r"\bgeneral\s+(?:adult\s+)?population\b|\bcommunity[- ]dwelling\s+adults\b"),
    # 'dewasa muda' / 'dewasa awal' (young or emerging adults) is an adult signal, not working age
    ("Indonesian adult terms", r"\b(?:orang\s+)?dewasa\b(?!\s+(?:muda|awal))|\bpekerja\b|\bkaryawan\b|"
                               r"\busia\s+produktif\b"),
)
FAMILIES_V2 = dict(FAMILIES)
FAMILIES_V2.update({
    "listening_feeling_heard": _extend(FAMILIES["listening_feeling_heard"], (
        ("merasa didengar", r"\bmerasa\s+(?:di)?dengar(?:kan)?\b|\bdidengarkan\b"),
        ("mendengarkan aktif/empatik", r"\bmendengar(?:kan)?\s+(?:aktif|empatik|secara\s+empatik)\b"),
    )),
    "journaling_expressive_writing": _extend(FAMILIES["journaling_expressive_writing"], (
        ("menulis ekspresif", r"\bmenulis\s+ekspresif\b|\bpenulisan\s+ekspresif\b|\bexpressive\s+writing\b"),
        ("jurnal harian / buku harian", r"\bjurnal\s+(?:harian|syukur|rasa\s+syukur|pribadi)\b|\bbuku\s+harian\b|"
                                        r"\bmenulis\s+jurnal\b|\bjournaling\b"),
    )),
    "check_in_self_monitoring": _extend(FAMILIES["check_in_self_monitoring"], (
        ("pemantauan suasana hati", r"\bpemantauan\s+(?:suasana\s+hati|mood|diri)\b|\bpelacakan\s+(?:mood|suasana\s+hati)\b"),
    )),
    "consultation_preparation": _extend(FAMILIES["consultation_preparation"], (
        ("persiapan konsultasi", r"\bpersiapan\s+(?:konsultasi|kunjungan)\b|\bdaftar\s+pertanyaan\b"),
    )),
    "continuity_after_consultation": _extend(FAMILIES["continuity_after_consultation"], (
        ("kesinambungan perawatan", r"\b(?:kesinambungan|kontinuitas)\s+(?:perawatan|layanan|pelayanan)\b"),
    )),
    "sharing_and_revocation": _extend(FAMILIES["sharing_and_revocation"], (
        ("penarikan persetujuan", r"\bpenarikan\s+persetujuan\b|\bmencabut\s+(?:persetujuan|izin|akses)\b"),
    )),
    "adolescents": _extend(FAMILIES["adolescents"], _ID_ADOLESCENT),
    "adolescent_consent_confidentiality": _extend(FAMILIES["adolescent_consent_confidentiality"], (
        ("persetujuan/izin orang tua", r"\b(?:persetujuan|izin|ijin)\s+(?:orang\s+tua|wali)\b"),
        ("kerahasiaan", r"\bkerahasiaan\b"),
        ("akses orang tua/wali", r"\bakses\s+(?:orang\s+tua|wali)\b|\bketerlibatan\s+orang\s+tua\b"),
    )),
    "older_adults_50_plus": _extend(FAMILIES["older_adults_50_plus"], _ID_OLDER),
    "accessibility_disability": _extend(FAMILIES["accessibility_disability"], (
        ("disabilitas", r"\b(?:penyandang\s+)?disabilitas\b|\btunanetra\b|\btunarungu\b|\btuli\b|"
                        r"\baksesibilitas\s+(?:digital|aplikasi|web|situs)\b|\bbahasa\s+sederhana\b"),
    )),
    "health_data_privacy_law": _extend(FAMILIES["health_data_privacy_law"], (
        ("rekam medis / data kesehatan", r"\b(?:keamanan|perlindungan|privasi)\s+(?:data|rekam\s+medis)\b|"
                                         r"\bdata\s+kesehatan\b"),
    )),
    "help_seeking": _extend(FAMILIES["help_seeking"], (
        ("mencari bantuan", r"\b(?:mencari|pencarian)\s+(?:bantuan|pertolongan)\b|\bperilaku\s+mencari\s+bantuan\b"),
    )),
    "safety": _extend(FAMILIES["safety"], (
        ("bunuh diri / melukai diri", r"\bbunuh\s+diri\b|\b(?:melukai|menyakiti)\s+diri\b|\bkrisis\b"),
    )),
    "privacy": _extend(FAMILIES["privacy"], (
        ("privasi / kerahasiaan", r"\bprivasi\b|\bkerahasiaan\b|\bdata\s+pribadi\b|\bkeamanan\s+data\b"),
    )),
    "working_age_adults": _c(_WORKING_AGE),
})
MENTAL_HEALTH_V2 = _extend(MENTAL_HEALTH, (
    ("Indonesian distress terms", r"\bdepresi\b|\bkecemasan\b|\bstres\b|\bkesejahteraan\s+(?:psikologis|mental)\b|"
                                  r"\bpsikologis\b|\bkonseling\b|\bbunuh\s+diri\b"),
))
DIGITAL_V2 = tuple(
    (label, re.compile(rx.pattern + extra, I)) for (label, rx), extra in zip(DIGITAL, (
        r"|\baplikasi\b", r"|\bdaring\b|\btelekonseling\b", r"|\bkecerdasan\s+buatan\b", "")))
POPULATION_V2 = {
    "adolescent_signal": FAMILIES_V2["adolescents"],
    "older_adult_signal": FAMILIES_V2["older_adults_50_plus"],
    "adult_signal": _extend(POPULATION["adult_signal"], _WORKING_AGE + (
        ("young adults (dewasa muda)", r"\bdewasa\s+(?:muda|awal)\b"),)),
}
FEATURE_FAMILIES_V2 = tuple(k for k in FAMILIES_V2 if k not in ("adolescents",))
FAMILIES_2 = Lexicon("sela-v3-families-2", FAMILIES_V2, MENTAL_HEALTH_V2, DIGITAL_V2, DESIGN, NULL_OR_NEGATIVE,
                     POPULATION_V2, FEATURE_FAMILIES_V2)
LEXICONS = {FAMILIES_1.version: FAMILIES_1, FAMILIES_2.version: FAMILIES_2}
