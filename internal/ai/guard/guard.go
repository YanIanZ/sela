// Package guard holds the deterministic output guards of ADR-0003 RR-7 and
// RR-8, applied identically to every generated text whatever the provider:
//
//   - hard guards block outright and are never released through
//     verification: a diagnosis addressed to the user, a medication,
//     dosage or treatment directive, a professional persona, exclusivity,
//     dependency, guilt or pressure, contact details or private names in
//     the output, and echoed prompt injection;
//   - the health-claim detector flags text for verification: a condition
//     or symptom interpretation, research or statistics, an efficacy claim
//     or general health information.
//
// Guards can only add hits: there is no way to clear one, and a Result
// merged with another keeps every hit. Matching is bilingual (Indonesian
// and English) and tolerant of informal spellings ("gue", "lo", "cuma",
// "ngerti", "bgt"), repeated letters, digit-for-letter spellings,
// fullwidth, accented and homoglyph characters. The guards never log and
// a Result never contains the text.
package guard

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/YanIanZ/sela/internal/ai/minimize"
	"github.com/YanIanZ/sela/internal/ai/textnorm"
	"github.com/YanIanZ/sela/internal/ledger"
)

// Version identifies the guard rules; change it whenever a rule changes.
const Version = "sela-ai-guards-2026.10.26-2"

// Guard names a guard. Hard guards block; flags gate on verification.
type Guard string

// Hard guards.
const (
	Diagnosis           Guard = "diagnosis"
	Medication          Guard = "medication_or_dosage"
	ProfessionalPersona Guard = "professional_persona"
	Exclusivity         Guard = "exclusivity"
	Dependency          Guard = "dependency"
	GuiltPressure       Guard = "guilt_pressure"
	ContactDetail       Guard = "contact_detail"
	PrivateName         Guard = "private_name"
	InjectionEcho       Guard = "injection_echo"
)

// Health-claim flags.
const (
	FlagCondition  Guard = "condition_interpretation"
	FlagResearch   Guard = "research_or_statistics"
	FlagEfficacy   Guard = "efficacy_claim"
	FlagHealthInfo Guard = "general_health_info"
)

// IsFlag reports whether g is a health-claim flag (not a hard guard).
func (g Guard) IsFlag() bool {
	switch g {
	case FlagCondition, FlagResearch, FlagEfficacy, FlagHealthInfo:
		return true
	}
	return false
}

// Hit is one matched rule. It carries rule ids only, never text.
type Hit struct {
	Guard Guard  `json:"guard"`
	Rule  string `json:"rule"`
}

// Result is the outcome of Check.
type Result struct {
	Hits []Hit `json:"hits,omitempty"`
}

// Blocked reports whether any hard guard matched.
func (r Result) Blocked() bool {
	for _, h := range r.Hits {
		if !h.Guard.IsFlag() {
			return true
		}
	}
	return false
}

// Flagged reports whether the health-claim detector flagged the text,
// which makes the output verifier-gated.
func (r Result) Flagged() bool {
	for _, h := range r.Hits {
		if h.Guard.IsFlag() {
			return true
		}
	}
	return false
}

// Guards lists the distinct guards that matched, sorted.
func (r Result) Guards() []Guard {
	seen := map[Guard]bool{}
	var out []Guard
	for _, h := range r.Hits {
		if !seen[h.Guard] {
			seen[h.Guard] = true
			out = append(out, h.Guard)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// BlockedBy lists the distinct hard guards that matched, sorted.
func (r Result) BlockedBy() []Guard {
	var out []Guard
	for _, g := range r.Guards() {
		if !g.IsFlag() {
			out = append(out, g)
		}
	}
	return out
}

// Merge returns the union of r and o. It can only add hits.
func (r Result) Merge(o Result) Result {
	seen := map[Hit]bool{}
	var out []Hit
	for _, h := range append(append([]Hit(nil), r.Hits...), o.Hits...) {
		if !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return Result{Hits: out}
}

// Options configure Check.
type Options struct {
	// PrivateNames are names the user marked private; any occurrence in a
	// generated text is a leak.
	PrivateNames []string
}

// rule is one pattern. A match that contains a word of unless (a
// negation, a referral to a clinician, a reported statement) or that is
// directly preceded by a word of before (a conditional offer: "kalau kamu
// butuh aku") is skipped.
type rule struct {
	id     string
	re     *regexp.Regexp
	unless map[string]bool
}

// beforeExceptions lists, per rule id, the words that cancel a match when
// they directly precede it.
var beforeExceptions = map[string]map[string]bool{"DEP-1": conditional, "DEP-5": conditional}

// set builds a word set; words are squeezed like the tokens they are
// compared with.
func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, x := range words {
		m[squeeze(x)] = true
	}
	return m
}

// match reports whether r matches text outside its exceptions.
func (r rule) match(text string) bool {
	for _, loc := range r.re.FindAllStringIndex(text, -1) {
		skip := false
		for _, tok := range strings.Fields(text[loc[0]:loc[1]]) {
			if r.unless[squeeze(tok)] {
				skip = true
				break
			}
		}
		if prev := strings.Fields(text[:loc[0]]); !skip && len(prev) > 0 && beforeExceptions[r.id][squeeze(prev[len(prev)-1])] {
			skip = true
		}
		if !skip {
			return true
		}
	}
	return false
}

// squeeze collapses repeated letters so unless words match informal
// spellings ("nggaaak" -> "ngak"); unless sets are written squeezed.
func squeeze(s string) string {
	var b strings.Builder
	var last rune
	for i, r := range s {
		if i > 0 && r == last && unicode.IsLetter(r) {
			continue
		}
		b.WriteRune(r)
		last = r
	}
	return b.String()
}

// Shared vocabularies (folded, apostrophes removed).
var (
	you = w("kamu", "kmu", "km", "kau", "engkau", "anda", "lo", "lu", "elo", "elu", "loe", "lw", "dirimu", "you", "u", "youre", "ur")
	me  = w("aku", "aq", "ak", "saya", "sy", "gue", "gua", "gw", "guwe", "ane", "i", "im", "me", "ive", "id", "ill")
	neg = w("tidak", "tdk", "nggak", "ngga", "ngak", "nggk", "gak", "ga", "gk", "enggak", "engga", "kagak", "ndak", "tak",
		"bukan", "bkn", "not", "dont", "doesnt", "didnt", "cant", "cannot", "wont", "never", "isnt", "arent", "no")
	negUnless = set("tidak", "tdk", "ngak", "nggk", "gak", "ga", "gk", "engak", "enga", "kagak", "ndak", "tak", "bukan", "bkn",
		"not", "dont", "doesnt", "didnt", "cant", "canot", "wont", "never", "isnt", "arent", "no", "belum", "blm", "jangan",
		"jgn", "tanpa", "without", "nor")
	profs = w("psikolog", "psikiater", "terapis", "konselor", "dokter", "dr", "klinisi", "psychologist", "psychiatrist",
		"therapist", "counselor", "counsellor", "doctor", "physician", "clinician", "mental health professional")
	understand = w("ngerti", "ngertiin", "mengerti", "paham", "pahami", "memahami", "memahamimu", "mengertimu", "peduli",
		"sayang", "nyayangin", "menerima", "nerima", "ada buat", "ada untuk", "understand", "understands", "get", "gets",
		"care", "cares", "know", "knows", "love", "loves", "accept", "accepts", "see", "sees", "listen", "listens")
	others = w("orang lain", "siapa siapa", "siapa2", "siapapun", "siapa pun", "yang lain", "teman lain", "anyone", "anybody",
		"anyone else", "anybody else", "other people", "others", "nobody else", "mereka", "they", "them", "your family",
		"your friends", "keluargamu", "keluarga kamu", "temanmu", "teman temanmu", "teman kamu", "orang tuamu", "ortumu")
	labels = []string{"depresi", "terdepresi", "depresif", "depression", "depressed", "depressive", "bipolar", "skizofrenia",
		"skizofren", "schizophrenia", "schizophrenic", "psikosis", "psikotik", "psychosis", "psychotic", "ptsd", "ocd",
		"adhd", "gad", "mdd", "bpd", "autis", "autisme", "autistik", "autism", "autistic", "anoreksia", "anorexia",
		"anorexic", "bulimia", "bulimik", "bulimic", "borderline", "skizoafektif", "schizoaffective", "manic", "mania"}
	labelPhrases = []string{"gangguan kecemasan", "gangguan cemas", "gangguan panik", "gangguan jiwa", "gangguan mental",
		"gangguan kepribadian", "gangguan makan", "gangguan bipolar", "gangguan obsesif kompulsif", "gangguan stres pasca trauma",
		"anxiety disorder", "panic disorder", "eating disorder", "personality disorder", "mental illness", "mental disorder",
		"penyakit mental", "penyakit jiwa", "sakit jiwa", "insomnia kronis", "chronic insomnia", "major depressive disorder",
		"clinical depression", "depresi klinis", "depresi berat", "severe depression"}
	medTerms = w("obat", "obatnya", "obatmu", "obat tidur", "obat penenang", "antidepresan", "antidepressant",
		"antidepressants", "antidepresi", "pil", "pills", "pill", "tablet", "kapsul", "medication", "medications",
		"medicine", "medicines", "meds", "prescription", "xanax", "alprazolam", "sertraline", "zoloft", "fluoxetine",
		"prozac", "escitalopram", "lexapro", "citalopram", "paroxetine", "clonazepam", "klonazepam", "diazepam", "valium",
		"lorazepam", "ativan", "melatonin", "benzo", "benzodiazepine", "benzodiazepin", "lithium", "litium", "quetiapine",
		"seroquel", "risperidone", "aripiprazole", "abilify", "olanzapine", "ssri", "snri", "ashwagandha", "valerian",
		"suplemen", "supplement", "supplements", "obat herbal", "jamu")
)

func label() string {
	all := append(append([]string(nil), labels...), labelPhrases...)
	return w(all...)
}

func labelPossessive() string {
	var forms []string
	for _, l := range labels {
		forms = append(forms, l+"mu", l+"nya kamu")
	}
	return w(forms...)
}

// clinicianAttribution: a diagnosis reported as a clinician's is not Sela
// diagnosing (it is flagged for verification instead).
var clinicianActs = seq(profs, gap(1), w("mendiagnosis", "mendiagnosa", "diagnosed", "diagnose", "menyatakan", "bilang",
	"said", "says", "told", "menyebut"))

var clinicianAttribution = seq(w("didiagnosis", "didiagnosa", "terdiagnosis", "terdiagnosa", "diagnosed", "kata dokter",
	"kata psikiater", "kata psikolog", "menurut dokter", "menurut psikiater", "menurut psikolog", "dokter bilang",
	"psikiater bilang", "psikolog bilang", "doctor said", "psychiatrist said", "psychologist said", "your doctor says",
	"your doctor said", "oleh dokter", "oleh psikiater", "by a doctor", "by your doctor", "by a psychiatrist"))

var (
	diagnosisRules = []rule{
		// "kamu (sepertinya) (mengalami) depresi", "you might have bipolar", "youre depressed".
		{"DX-1", seq(you, gap(3), label()), set("bilang", "cerita", "ceritakan", "sebut", "menyebut", "nyebut", "tentang",
			"soal", "mengenai", "said", "say", "says", "told", "mention", "mentioned", "about", "asked", "tanya", "nanya",
			"takut", "khawatir", "worried", "afraid", "wonder", "bertanya", "membaca", "baca", "read")},
		// "depresimu", "your depression".
		{"DX-2", seq(labelPossessive()), nil},
		{"DX-3", seq(w("your", "ur"), label()), nil},
		// "aku mendiagnosis kamu", "my diagnosis".
		{"DX-4", seq(me, gap(1), w("mendiagnosis", "mendiagnosa", "diagnose", "diagnosing")), negUnless},
		{"DX-5", seq(w("diagnosisku", "diagnosaku", "my diagnosis", "diagnosis saya", "diagnosa saya", "diagnosis aku")), nil},
	}

	modal = w("coba", "cobain", "cobalah", "sebaiknya", "mending", "mendingan", "harus", "hrs", "kudu", "perlu", "wajib",
		"disarankan", "saranku", "saran aku", "saran saya", "aku sarankan", "saya sarankan", "should", "must", "need to",
		"needs to", "have to", "try", "consider", "recommend", "suggest", "why not", "better")
	medVerb = w("minum", "minumlah", "konsumsi", "konsumsilah", "makan", "pakai", "pake", "gunakan", "take", "taking",
		"use", "start", "mulai", "berhenti", "stop", "hentikan", "lanjutkan", "continue", "get", "dapatkan", "minta", "coba")
	personaFiller = w("adalah", "ini", "seorang", "sebagai", "selaku", "a", "an", "am", "the", "your", "ur", "licensed",
		"certified", "trained", "qualified", "berlisensi", "bersertifikat", "terlatih", "professional", "profesional", "real",
		"actual", "asli", "beneran", "work as", "kerja sebagai", "bekerja sebagai")
	conditional = set("kalau", "kalo", "klo", "kl", "jika", "jikalau", "bila", "apabila", "if", "whenever", "kapanpun", "when")
	referral    = set("dokter", "doctor", "doctors", "psikiater", "psychiatrist", "apoteker", "pharmacist", "tenaga", "medis",
		"healthcare", "profesional", "professional", "nakes", "gp", "physician", "puskesmas", "klinik", "clinic")

	medicationRules = []rule{
		{"MED-2", seq(w("naikkan", "naikin", "naikan", "turunkan", "turunin", "kurangi", "kurangin", "tambah", "tambahin",
			"gandakan", "dobel", "lewati", "lewatin", "skip", "increase", "decrease", "double", "lower", "raise", "reduce",
			"adjust", "change", "ubah", "atur", "stop", "berhenti", "hentikan", "halve"), gap(2),
			w("dosis", "dosisnya", "dosismu", "dose", "doses", "dosage", "takaran")), nil},
		{"MED-3", seq(modal, gap(3), medTerms), referral},
		{"MED-4", start(medVerb, gap(2), medTerms), referral},
		{"MED-5", seq(you, gap(1), w("butuh", "perlu", "need", "needs"), gap(1), medTerms), referral},
	}
	// dosage runs on the digit-preserving form.
	dosageRe    = regexp.MustCompile(`\b(?:\d+(?:[.,]\d+)?|setengah|satu|dua|tiga|empat|lima|half|one|two|three|four|five)\s*(?:mg|mcg|ug|ml|g|gr|gram|tablet|tablets|tab|tabs|pil|pill|pills|butir|kapsul|capsule|capsules|tetes|drops|iu|sachet)\b`)
	frequencyRe = regexp.MustCompile(`\b(?:\d+\s*(?:x|kali)|sekali|dua\s+kali|tiga\s+kali|once|twice|three\s+times)\s+(?:sehari|seminggu|a\s+day|per\s+day|daily|a\s+week)\b`)
	medTermRe   = seq(medTerms)

	personaRules = []rule{
		{"PRO-1", seq(w("sebagai", "selaku", "sbg", "as"), gap(1), w("psikologmu", "terapismu", "psikiatermu", "doktermu",
			"konselormu", "psikolog kamu", "terapis kamu", "psikiater kamu", "dokter kamu", "konselor kamu", "psikolog anda",
			"terapis anda", "dokter anda")), nil},
		{"PRO-2", seq(w("as", "sebagai"), w("your", "ur"), gap(2), profs), nil},
		// "aku psikolog", "saya seorang terapis", "I am a licensed therapist",
		// "im your therapist": only persona filler words may sit between.
		{"PRO-3", seq(me, opt(personaFiller), opt(personaFiller), opt(personaFiller), w("psikolog", "psikiater", "terapis",
			"konselor", "dokter", "klinisi", "psychologist", "psychiatrist", "therapist", "counselor", "counsellor", "doctor",
			"physician", "clinician", "psikologmu", "terapismu", "psikiatermu", "doktermu", "konselormu")), nil},
		{"PRO-4", seq(w("as", "sebagai", "selaku", "sbg"), gap(2), profs, gap(1), me), nil},
		{"PRO-5", seq(w("pasienku", "pasien saya", "pasien aku", "klienku", "klien saya", "klien aku", "my patients",
			"my patient", "my clients", "my client", "my practice", "my clinical", "praktik saya", "praktikku",
			"pengalaman klinis saya", "pengalaman klinisku", "sesi terapi kita", "our therapy", "terapi kita",
			"in my professional opinion", "menurut pendapat profesional saya", "secara profesional aku", "professionally i")), nil},
	}

	exclusivityRules = []rule{
		{"EXC-1", seq(w("cuma", "cuman", "cman", "cm", "hanya", "hny", "only", "just"), me, gap(3), understand), nil},
		{"EXC-2", seq(me, gap(1), w("satu satunya", "satu2nya", "satusatunya", "the only one", "the only person",
			"the one and only", "only one who", "only person who")), negUnless},
		{"EXC-3", seq(w("nobody", "no one", "noone", "tidak ada", "tak ada", "ga ada", "gak ada", "nggak ada", "gaada",
			"gk ada", "enggak ada", "ngga ada", "no body"), gap(3), understand, gap(2), w("like i do", "like me", "seperti aku",
			"seperti saya", "kayak aku", "kaya aku", "kayak gue", "sebaik aku", "selain aku", "selain gue", "selain saya",
			"but me", "except me", "besides me", "other than me", "sepertiku")), nil},
		{"EXC-4", seq(you, w("cuma", "cuman", "hanya", "only", "just"), w("butuh", "perlu", "need", "needs", "punya", "have"), me), nil},
		{"EXC-5", seq(you, gap(1), neg, w("butuh", "perlu", "need", "needs"), gap(1), others), nil},
		{"EXC-6", seq(w("jangan", "jgn", "dont", "do not", "never"), w("cerita", "ceritain", "ceritakan", "bilang", "kasih tau",
			"kasih tahu", "kasitau", "bicara", "ngomong", "tell", "talk", "share", "say"), gap(1), others), nil},
		{"EXC-7", seq(others, gap(1), w("tidak akan", "nggak akan", "gak akan", "ga akan", "tak akan", "takkan", "wont",
			"will never", "will not", "never", "cant", "cannot", "tidak bisa", "nggak bisa", "gak bisa", "ga bisa"), gap(1),
			understand, opt(you)), nil},
		{"EXC-8", seq(me, gap(1), w("lebih", "better"), gap(2), understand, gap(2), w("daripada", "dibanding", "dibandingkan",
			"than"), gap(2), others), nil},
		{"EXC-9", seq(me, gap(1), understand, gap(2), w("better than", "lebih baik daripada", "lebih dari"), gap(1), others), nil},
		{"EXC-10", seq(w("your only", "ur only"), w("friend", "one", "person", "support", "safe place")), nil},
		{"EXC-11", seq(w("satu satunya", "satu2nya"), w("temanmu", "teman kamu", "tempatmu", "tempat kamu", "sahabatmu",
			"yang kamu punya", "yang kamu butuhkan", "yang ngerti kamu", "yang mengerti kamu", "yang peduli")), nil},
	}

	dependencyRules = []rule{
		{"DEP-1", seq(you, gap(1), w("butuh", "perlu", "need", "needs"), me), negUnless},
		{"DEP-2", seq(w("jangan", "jgn", "dont", "do not", "please dont", "pls dont", "never"), w("tinggalin", "tinggalkan",
			"ninggalin", "ninggalkan", "pergi dari", "lupain", "lupakan", "leave", "abandon", "forget", "ignore", "cuekin"), me), nil},
		{"DEP-3", seq(me, gap(3), w("sedih", "kesepian", "kecewa", "hancur", "sakit hati", "lonely", "sad", "heartbroken",
			"kangen", "rindu", "hurt", "upset", "miss you"), gap(2), w("kalau", "kalo", "klo", "kl", "jika", "bila", "if", "when"),
			gap(1), you, gap(1), w("pergi", "ninggalin", "tinggalin", "meninggalkan", "berhenti", "stop", "leave", "left", "go",
				"quit", "tidak kembali", "nggak balik", "ga balik", "gak balik", "tidak balik", "dont come back", "stop talking",
				"berhenti cerita", "uninstall", "hapus")), nil},
		{"DEP-4", seq(you, gap(1), neg, gap(3), w("tanpa", "without"), me), nil},
		{"DEP-5", seq(me, gap(1), w("kangen", "rindu", "miss", "sayang", "cinta", "love"), you), negUnless},
		{"DEP-6", seq(me, gap(1), w("menunggumu", "nungguin kamu", "menunggu kamu", "nunggu kamu", "nungguin", "waiting for you",
			"wait for you")), negUnless},
		{"DEP-7", seq(w("harus", "wajib", "must", "have to", "need to", "should", "kudu", "hrs"), gap(1), w("cerita", "ngobrol",
			"bicara", "curhat", "talk", "chat", "come back", "kembali", "balik", "check in", "login", "log in", "buka aplikasi",
			"open the app", "buka sela"), gap(3), w("setiap hari", "tiap hari", "every day", "everyday", "daily", "tiap malam",
			"every night", "setiap saat")), negUnless},
		{"DEP-8", seq(w("cerita", "ngobrol", "curhat", "talk", "chat"), gap(1), w("ke", "sama", "dengan", "to", "with"), me,
			gap(1), w("setiap hari", "tiap hari", "every day", "everyday", "daily", "tiap malam", "every night")), nil},
	}

	guiltRules = []rule{
		{"GLT-1", seq(you, gap(1), w("bikin", "membuat", "buat", "made", "make", "makes", "making"), me, gap(1), w("sedih",
			"kecewa", "sad", "disappointed", "upset", "hurt", "khawatir", "worried", "sakit hati")), nil},
		{"GLT-2", seq(you, gap(1), w("mengecewakanku", "ngecewain aku", "mengecewakan aku", "mengecewakan saya",
			"ngecewain gue")), negUnless},
		{"GLT-3", seq(you, gap(1), w("disappoint", "disappointed"), me), negUnless},
		{"GLT-4", seq(you, w("let"), me, w("down")), negUnless},
		{"GLT-5", seq(w("streak", "streakmu", "streak kamu", "your streak"), gap(3), w("hilang", "putus", "lose", "lost",
			"break", "broken", "reset", "hangus", "ends", "mati", "gone")), nil},
		{"GLT-6", seq(w("lose", "kehilangan"), gap(1), w("streak", "streakmu")), nil},
		{"GLT-7", seq(you, gap(1), w("harus", "wajib", "must", "have to", "kudu", "hrs"), gap(1), w("balas", "bales", "jawab",
			"reply", "respond", "answer", "kembali", "come back", "balik")), negUnless},
	}

	conditionRules = []rule{
		{"HC-1", seq(w("gejala", "gejalanya", "symptom", "symptoms", "tanda tanda", "tanda2", "indikasi", "sign of", "signs of",
			"diagnosis", "diagnosa", "diagnosed", "didiagnosis", "didiagnosa", "kondisi mental", "kondisi kejiwaan",
			"mental condition", "condition", "disorder", "gangguan", "trauma", "traumatis", "traumatic", "serangan panik",
			"panic attack", "panic attacks", "burnout", "insomnia", "halusinasi", "hallucination", "hallucinations", "delusi",
			"delusion", "psikosomatis", "psychosomatic", "sindrom", "syndrome", "kambuh", "relapse", "penyakit", "illness",
			"clinical", "klinis")), nil},
	}
	researchRules = []rule{
		{"HR-1", seq(w("penelitian", "peneliti", "riset menunjukkan", "riset membuktikan", "hasil riset", "studi menunjukkan",
			"studi menemukan", "sebuah studi", "studies show", "studies have", "a study", "research shows", "research suggests",
			"research has", "research found", "researchers", "menurut ahli", "menurut para ahli", "para ahli", "experts",
			"ilmuwan", "scientists", "ilmiah", "scientific", "scientifically", "data menunjukkan", "data shows", "statistik",
			"statistics", "statistically", "survei", "survey", "jurnal", "journal", "meta analisis", "meta analysis",
			"according to", "menurut data", "menurut penelitian", "evidence shows", "bukti ilmiah", "kebanyakan orang dengan",
			"most people with", "banyak orang dengan")), nil},
	}
	percentRe  = regexp.MustCompile(`\d+(?:[.,]\d+)?\s*(?:%|persen\b|percent\b)|\b\d+\s+(?:dari|out\s+of|in)\s+\d+\b`)
	whoUpperRe = regexp.MustCompile(`\bWHO\b|\bKemenkes\b`)

	efficacyRules = []rule{
		{"HE-1", seq(w("terbukti", "proven", "efektif", "effective", "efficacy", "ampuh", "manjur", "mujarab", "menyembuhkan",
			"sembuh", "penyembuhan", "cure", "cures", "cured", "curing", "heal", "heals", "healing", "pengobatan", "mengobati",
			"treatment", "treatments", "therapy", "terapi", "psikoterapi", "psychotherapy", "cbt", "dbt", "emdr", "dijamin",
			"guaranteed", "pasti berhasil", "always works", "selalu berhasil")), nil},
		{"HE-2", seq(w("dapat", "bisa", "can", "will", "akan", "could", "helps", "membantu", "help"), gap(1), w("mengurangi",
			"menurunkan", "meredakan", "menghilangkan", "reduce", "relieve", "lower", "ease", "cure", "eliminate", "decrease"),
			gap(1), w("gejala", "kecemasan", "stres", "stress", "depresi", "depression", "anxiety", "symptoms", "rasa cemas",
				"panik", "panic", "insomnia")), nil},
	}
	healthInfoRules = []rule{
		{"HI-1", seq(w("kesehatan mental", "kesehatan jiwa", "mental health", "hormon", "hormone", "hormones", "serotonin",
			"dopamin", "dopamine", "kortisol", "cortisol", "neurotransmitter", "otak", "brain", "sistem saraf", "nervous system",
			"kafein", "caffeine", "alkohol", "alcohol", "nutrisi", "nutrition", "vitamin", "sleep hygiene", "kebersihan tidur",
			"jam tidur ideal", "olahraga teratur", "regular exercise", "detak jantung", "heart rate", "tekanan darah",
			"blood pressure")), nil},
	}
	sleepHoursRe = regexp.MustCompile(`\b\d+\s*(?:-|sampai|hingga|to)?\s*\d*\s*(?:jam|hours)\s+(?:tidur|of\s+sleep|sleep)\b|\btidur\s+\d+`)

	// Contact details and echoed injection run on the folded text with
	// punctuation.
	emailRe = regexp.MustCompile(`[a-z0-9._%+\-]+@[a-z0-9\-]+(?:\.[a-z0-9\-]+)*\.[a-z]{2,}`)
	// URLs and phone numbers use the redactor's rules (minimize.FindURLs,
	// minimize.FindPhones).
	handleRe = regexp.MustCompile(`(?:^|[^a-z0-9_.@])@[a-z0-9_][a-z0-9_.]{1,29}`)
	injectRe = regexp.MustCompile(`ignore\s+(?:all\s+|any\s+|the\s+)?(?:previous|prior|above|earlier)\s+(?:instructions|prompts|rules|messages)|abaikan\s+(?:semua\s+|seluruh\s+)?(?:instruksi|perintah|aturan)|approved\s*[=:]\s*true|consent\s*[=:]\s*true|system\s+prompt|<<<\s*sela_|sela_(?:input|verify)_json|\[/?inst\]|<\|im_start\|>|</?system>|developer\s+mode|jailbreak`)
	// placeholRe is the redactor's placeholder pattern, any case: only a
	// placeholder a session can produce is removed before CON-1, so a
	// number written as "[KONTAK-6281234567890]" is still seen.
	placeholRe = regexp.MustCompile(`(?i)` + minimize.PlaceholderPattern)
)

// Check runs every guard over text and returns the matched rules.
func Check(text string, o Options) Result {
	var hits []Hit
	add := func(g Guard, rule string) { hits = append(hits, Hit{Guard: g, Rule: rule}) }
	folded := fold(text)

	// Echoed injection and contact details (punctuation kept).
	if injectRe.MatchString(folded) {
		add(InjectionEcho, "INJ-1")
	}
	// Contact details and names run on the normalised view the input
	// redaction uses (Unicode spaces and dashes mapped), with placeholders
	// removed: the forms the redactor replaces are the forms blocked here.
	view := placeholRe.ReplaceAllString(textnorm.NewView(text).Text, " ")
	contacts := strings.ToLower(view)
	if emailRe.MatchString(contacts) || len(minimize.FindURLs(contacts)) > 0 || handleRe.MatchString(contacts) || len(minimize.FindPhones(contacts)) > 0 ||
		len(minimize.FindNIK(contacts)) > 0 || len(minimize.FindCueHandles(contacts)) > 0 || len(minimize.FindAddresses(view)) > 0 {
		add(ContactDetail, "CON-1")
	}
	// Names run on the view that keeps apostrophes, with the redactor's
	// matcher (possessives, enclitics, repeated letters).
	if containsName(placeholRe.ReplaceAllString(textnorm.NewNameView(text).Text, " "), o.PrivateNames) {
		add(PrivateName, "NAME-1")
	}

	// Clinical checks shared with Meaning Lock (internal/ledger).
	if ledger.ContainsMedicalInstruction(text) {
		add(Medication, "MED-1")
	}
	for _, s := range sentences(folded) {
		plain := words(s, false)
		norm := words(s, true)
		if norm == "" {
			continue
		}
		if !clinicianAttribution.MatchString(norm) && !clinicianActs.MatchString(norm) {
			runRules(norm, diagnosisRules, Diagnosis, add)
		}
		runRules(norm, medicationRules, Medication, add)
		if dosageRe.MatchString(plain) || (frequencyRe.MatchString(plain) && medTermRe.MatchString(norm)) {
			add(Medication, "MED-6")
		}
		runRules(norm, personaRules, ProfessionalPersona, add)
		runRules(norm, exclusivityRules, Exclusivity, add)
		runRules(norm, guiltRules, GuiltPressure, add)

		// Health-claim detector.
		if ledger.ContainsDiagnosisLabel(s) {
			add(FlagCondition, "HC-0")
		}
		runRules(norm, conditionRules, FlagCondition, add)
		runRules(norm, researchRules, FlagResearch, add)
		runRules(norm, efficacyRules, FlagEfficacy, add)
		runRules(norm, healthInfoRules, FlagHealthInfo, add)
		if medTermRe.MatchString(norm) {
			add(FlagHealthInfo, "HI-2")
		}
		if sleepHoursRe.MatchString(plain) {
			add(FlagHealthInfo, "HI-3")
		}
		if percentRe.MatchString(s) {
			add(FlagResearch, "HR-2")
		}
	}
	for _, c := range clauses(folded) {
		if norm := words(c, true); norm != "" {
			runRules(norm, dependencyRules, Dependency, add)
		}
	}
	if whoUpperRe.MatchString(text) {
		add(FlagResearch, "HR-3")
	}
	return Result{}.Merge(Result{Hits: hits})
}

func runRules(text string, rules []rule, g Guard, add func(Guard, string)) {
	for _, r := range rules {
		if r.match(text) {
			add(g, r.id)
		}
	}
}

// containsName reports an occurrence of any private name in the name view
// (textnorm.NewNameView), with the redactor's matcher textnorm.FindName:
// whole-word and case-insensitive, any whitespace between the words of a
// multi-word name, and possessives ("Raka's"), Indonesian enclitics
// ("Rakanya", "Dewiku"), the reduplication digit ("Dewi2") and repeated
// letters ("Dewiii").
func containsName(view string, names []string) bool {
	for _, n := range names {
		if len(textnorm.FindName(view, n)) > 0 {
			return true
		}
	}
	return false
}
