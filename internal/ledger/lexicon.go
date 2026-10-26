package ledger

import "strings"

// Lexicons are folded (lowercase, accent-free) tokens as produced by
// tokenize. Multi-word phrases are space-separated tokens. They target
// Indonesian, English and code-switched text, including common informal
// spellings. They catch known patterns only; the user's review remains the
// final check.

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// negators are single-token negation words.
var negators = set(
	// Indonesian, formal and informal.
	"tidak", "tak", "nggak", "ngga", "ngak", "gak", "ga", "enggak", "engga", "ndak", "nda", "kagak", "kaga",
	"tdk", "gk", "bukan", "belum", "blm", "tanpa", "jangan",
	// English ("n't" contractions are matched by suffix).
	"not", "no", "never", "without", "nobody", "nothing", "neither", "nor", "none", "nowhere", "cannot",
)

// isNegator reports whether a token negates.
func isNegator(t string) bool { return negators[t] || strings.HasSuffix(t, "n't") }

// contrastive conjunctions open a new clause and region.
var contrastive = set(
	"tapi", "tetapi", "namun", "padahal", "sedangkan", "melainkan", "meskipun", "walaupun", "walau",
	"but", "however", "whereas", "although", "though",
)

// thirdPartyPronouns refer to someone other than the user.
var thirdPartyPronouns = set("dia", "mereka", "beliau", "doi", "ia", "he", "she", "they")

// thirdPartyNouns name people other than the user. Indonesian entries are
// stems: "-ku", "-mu" and "-nya" are stripped before lookup ("temanku").
var thirdPartyNouns = set(
	"teman", "temen", "kawan", "sahabat", "ibu", "ayah", "bapak", "mama", "papa", "mamah", "papah", "mami", "papi",
	"bunda", "emak", "adik", "adek", "kakak", "abang", "pacar", "suami", "istri", "anak", "dosen", "guru", "bos",
	"atasan", "rekan", "keluarga", "ortu", "orangtua", "nenek", "kakek", "om", "tante", "sepupu", "tetangga",
	"dokter", "psikolog", "psikiater", "konselor", "terapis", "pasangan", "mantan",
	"friend", "friends", "mother", "mom", "mum", "father", "dad", "sister", "brother", "partner", "boyfriend",
	"girlfriend", "husband", "wife", "boss", "lecturer", "teacher", "professor", "parents", "parent", "roommate",
	"family", "doctor", "therapist", "counselor", "counsellor", "psychiatrist", "psychologist", "colleague",
	"coworker", "classmate", "cousin", "aunt", "uncle", "grandma", "grandmother", "grandpa", "grandfather",
)

// frameSpeakers are additional speakers recognised only in reported-speech
// frames ("kata orang", "people say").
var frameSpeakers = set("orang", "people", "everyone", "everybody")

// firstPerson are tokens that make the user the subject.
var firstPerson = set(
	"aku", "saya", "gue", "gua", "gw", "ane", "kami", "kita",
	"kurasa", "kupikir", "kurasakan", "kubilang", "kulihat", "kutahu", "kuingat", "kucoba", "kuanggap",
	"i", "me", "my", "myself", "mine", "we", "us", "our", "i'm", "i've", "i'd", "i'll", "im", "ive",
)

// kuWords end in "ku" without being a first-person possessive.
var kuWords = set("aku", "buku", "kuku", "laku", "saku", "suku", "paku", "baku", "liku", "siku", "tiku", "kaku", "ragu")

// possessives that, before a third-party noun, belong to that noun.
var possessiveDeterminers = set("my", "our", "his", "her", "their", "your")

// speechVerbs introduce reported speech or opinion after a speaker.
var speechVerbs = set(
	"bilang", "berkata", "mengatakan", "ngomong", "menyebut", "nyebut", "menganggap", "menilai", "bilangnya",
	"said", "says", "say", "told", "tells", "tell", "thinks", "think", "thought", "believes", "believe",
	"claims", "claimed", "insists", "insisted",
)

const (
	tagPast    = string(TimePast)
	tagFuture  = string(TimeFuture)
	tagCurrent = string(TimeCurrent)
	tagHypo    = string(CertaintyHypothetical)
	tagUnsure  = string(CertaintyUncertain)
)

// timeLexicon tags time-frame markers.
var timeLexicon = newLexicon(map[string][]string{
	tagPast: {
		"dulu", "dahulu", "dulunya", "waktu kecil", "masa kecil", "waktu sd", "waktu smp", "waktu sma",
		"tahun lalu", "bulan lalu", "minggu lalu", "semester lalu", "tahun kemarin", "bulan kemarin",
		"minggu kemarin", "kemarin", "tadi", "sempat", "pernah",
		"used to", "last year", "last month", "last week", "last semester", "last night", "last time",
		"when i was", "as a kid", "as a child", "in the past", "back then", "ago", "previously", "formerly",
		"yesterday",
	},
	tagFuture: {
		"nanti", "besok", "akan", "lusa", "minggu depan", "bulan depan", "tahun depan", "semester depan",
		"suatu hari", "will", "going to", "gonna", "tomorrow", "next week", "next month", "next year",
		"next semester", "i'll", "we'll", "it'll", "you'll", "won't", "in the future", "someday",
	},
	tagCurrent: {
		"sekarang", "kini", "saat ini", "akhir-akhir ini", "akhir2 ini", "belakangan", "belakangan ini",
		"sedang", "masih", "hari ini", "minggu ini", "bulan ini", "semester ini", "tahun ini",
		// "It has been long since I could ..." describes an ongoing state.
		"sudah lama tidak", "sudah lama nggak", "sudah lama gak", "sudah lama ga", "sudah lama enggak",
		"udah lama tidak", "udah lama nggak", "udah lama gak", "udah lama ga", "udah lama enggak",
		"sudah lama", "udah lama", "sejak dulu", "dari dulu",
		"lately", "these days", "now", "right now", "currently", "nowadays", "at the moment", "still",
		"recently", "today", "this week", "this month", "this semester", "this year",
	},
})

// certaintyLexicon tags hedges and hypotheticals.
var certaintyLexicon = newLexicon(map[string][]string{
	tagHypo: {
		"kalau nanti", "kalo nanti", "jika nanti", "seandainya", "andaikan", "andai", "takut kalau", "takut kalo",
		"takut jika", "takut nanti", "khawatir kalau", "khawatir kalo", "khawatir jika", "khawatir nanti",
		"jangan-jangan", "bagaimana kalau", "gimana kalau", "gimana kalo", "kalau sampai", "kalo sampai",
		"what if", "if i", "afraid that", "worried that", "scared that", "afraid i", "worried i", "scared i",
		"afraid i'll", "worried i'll", "scared i'll", "in case", "suppose", "imagine if",
	},
	tagUnsure: {
		"mungkin", "kayaknya", "kayanya", "sepertinya", "keknya", "entah", "entahlah", "barangkali", "agaknya",
		"kira-kira", "belum yakin", "kurang yakin", "tidak yakin", "nggak yakin", "gak yakin", "ga yakin",
		"ngga yakin", "enggak yakin", "tak yakin", "kurang tahu",
		"maybe", "might", "not sure", "unsure", "i guess", "probably", "perhaps", "possibly",
	},
})

// numberWords are quantities a model could invent (v1 NUMBER_WORDS plus
// English and "se-" forms).
var numberWords = set(
	"nol", "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh", "sebelas",
	"belas", "puluh", "ratus", "ribu", "juta", "setengah", "beberapa", "banyak", "sebagian", "semua", "seluruh",
	"sehari", "seminggu", "sebulan", "setahun", "sejam", "persen",
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve",
	"thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen", "twenty", "thirty",
	"forty", "fifty", "sixty", "seventy", "eighty", "ninety", "hundred", "thousand", "million", "half", "once",
	"twice", "dozen", "several", "many", "percent", "percentage",
)

// claimMarkers are time, frequency and severity claims an opening line may
// only repeat, never add (v1 CLAIM_MARKERS plus English).
var claimMarkers = set(
	"hari", "minggu", "bulan", "tahun", "sehari", "seminggu", "sebulan", "setahun", "berhari", "berminggu",
	"berbulan", "bertahun", "sejak", "selama", "belakangan", "akhir", "kemarin", "terakhir", "tempo", "sering",
	"selalu", "setiap", "tiap", "jarang", "kadang", "terus", "menerus", "rutin", "berulang", "lama", "semalam",
	"parah", "berat", "ringan", "sangat", "banget", "sekali", "ekstrem", "semakin", "makin", "memburuk",
	"buruk", "hebat", "kronis", "akut", "gawat", "darurat", "pernah", "kembali",
	"day", "days", "week", "weeks", "month", "months", "year", "years", "daily", "weekly", "since", "lately",
	"recently", "always", "often", "constantly", "every", "sometimes", "rarely", "frequently", "severe",
	"severely", "extremely", "very", "worse", "worst", "terrible", "chronic", "acute", "urgent", "emergency",
	"anymore", "again",
)

// stopwords carry no fact of their own (v1 STOPWORDS plus English and the
// word parts left by English contractions).
var stopwords = set(
	"saya", "aku", "kami", "kita", "dan", "atau", "yang", "ini", "itu", "di", "ke", "dari", "untuk", "dengan", "pada",
	"juga", "jadi", "karena", "kalau", "jika", "tapi", "tetapi", "namun", "agar", "supaya", "sudah", "belum", "akan",
	"sedang", "lagi", "masih", "bisa", "dapat", "boleh", "mau", "ingin", "perlu", "butuh", "harus", "ada", "tidak",
	"nggak", "gak", "bukan", "ya", "nya", "ku", "mu", "dok", "dokter", "bu", "pak", "mbak", "mas", "kak", "halo",
	"hai", "selamat", "pagi", "siang", "sore", "malam", "permisi", "maaf", "mohon", "tolong", "terima", "kasih",
	"apakah", "bagaimana", "gimana", "apa", "kenapa", "mengapa", "siapa", "mana", "seperti", "begitu", "sih",
	"deh", "dong", "kok", "sebenarnya", "rasanya", "merasa", "terasa", "rasakan", "merasakan", "dirasakan",
	"sendiri", "soal", "tentang", "hal",
	"sesuatu", "sini", "situ", "sana", "lebih", "saat", "ketika", "waktu", "sekarang", "nanti", "bikin", "membuat",
	"buat", "menjadi", "sampai", "hingga", "sehingga", "makanya", "soalnya", "terutama", "misalnya", "cukup",
	"agak", "sedikit", "coba", "mencoba", "tentu", "memang", "justru", "pun", "saja", "aja", "hanya", "cuma",
	"bahwa", "kalo", "lalu", "kemudian", "serta", "oleh", "bagi", "dalam", "luar", "antara", "tanpa",
	"sama", "biar", "semoga", "mungkin", "sebaiknya", "kira", "persis", "secara", "begini", "gitu", "gini",
	"kayak", "kaya", "nih", "tuh", "loh", "kan", "dia", "mereka", "orang", "diri", "punya", "adalah",
	"ialah", "yakni", "yaitu", "tersebut", "tersebutlah", "dulu", "baru", "lama", "pertama", "kali",
	"gue", "gua", "enggak", "ngga", "tak", "udah", "kalian", "anda", "kamu", "engkau",
	"the", "and", "or", "but", "if", "so", "to", "of", "in", "on", "at", "for", "with", "from", "by", "about", "as",
	"into", "than", "then", "that", "this", "these", "those", "there", "here", "is", "am", "are", "was", "were",
	"be", "been", "being", "have", "has", "had", "having", "do", "does", "did", "doing", "done", "will", "would",
	"could", "should", "can", "may", "might", "must", "shall", "not", "no", "yes", "really", "just", "also", "too",
	"some", "any", "more", "most", "much", "what", "which", "who", "whom", "whose", "when", "where", "why", "how",
	"all", "both", "each", "other", "such", "only", "own", "same", "even", "ever", "now", "out", "off", "because",
	"while", "until", "like", "want", "wanted", "wants", "need", "needs", "needed", "hello", "dear", "please",
	"thank", "thanks", "sorry", "okay", "something", "thing", "things", "way", "get", "got", "getting", "feel",
	"feels", "feeling", "felt", "myself", "you", "your", "she", "her", "him", "his", "they", "them", "their", "its",
	"our", "going", "through", "lot", "kind", "maybe", "still", "don", "doesn", "didn", "isn", "aren", "wasn",
	"weren", "won", "couldn", "wouldn", "shouldn", "haven", "hasn", "hadn", "ain", "let", "let's", "it's", "i'm",
	"would've", "able",
)

// intentVocab is the only extra vocabulary an opening line may add: how one
// asks a helper for help (v1 INTENT_VOCAB plus English and template words).
var intentVocab = set(
	"dirujuk", "rujukan", "merujuk", "rujuk", "psikolog", "psikiater", "poli", "jiwa", "konseling", "konselor",
	"konsultasi", "bercerita", "cerita", "menceritakan", "ceritakan", "bantuan", "dibantu", "membantu", "bantu",
	"bicara", "berbicara", "membicarakan", "dibicarakan", "bicarakan", "ngobrol", "layanan", "langkah",
	"selanjutnya", "berikutnya", "lanjut", "lanjutan", "mulai", "memulai", "dimulai", "tahu", "mengetahui",
	"tanya", "bertanya", "menanyakan", "tanyakan", "minta", "meminta", "saran", "arahan", "dukungan",
	"pendampingan", "kampus", "kantor", "puskesmas", "klinik", "faskes", "fktp", "bertemu", "ketemu", "janji",
	"catatan", "membawa", "bawa", "sesi", "kerahasiaan", "rahasia", "dicatat", "bersama", "tempat", "tepat",
	"cocok", "pilihan", "jalur", "proses", "cara", "menjelaskan", "jelaskan", "menyampaikan", "sampaikan",
	"mengatakan", "bilang", "situasi", "keadaan", "kondisi", "menghubungi", "hubungi", "chat", "telepon",
	"whatsapp", "menulis", "tulis", "bahas", "membahas", "pertanyaan", "datang", "mampir", "dengar",
	"mendengar", "didengar", "dengarkan", "siap", "bersedia", "ditangani", "penanganan",
	"profesional", "ahli", "spesialis", "ditemani", "teman", "menemani", "keseharian",
	"pengaruhnya", "pengaruh", "utama", "menyebutnya", "mulainya", "ceritanya",
	"alami", "mengalami", "dialami", "berpengaruh", "harapkan", "berharap", "harapan", "pertemuan", "yakin",
	"talk", "talking", "discuss", "share", "sharing", "tell", "telling", "explain", "help", "helped", "support",
	"advice", "guidance", "referral", "refer", "referred", "psychologist", "psychiatrist", "counselor",
	"counsellor", "counseling", "counselling", "professional", "appointment", "session", "meeting", "meet",
	"start", "started", "starting", "begin", "know", "understand", "ask", "asking", "question", "questions",
	"next", "step", "steps", "option", "options", "service", "services", "clinic", "campus", "note", "notes",
	"brought", "bring", "bringing", "ready", "sure", "hope", "hoping", "situation", "affect", "affected",
	"affects", "affecting", "life", "confidential", "confidentiality", "listen", "heard", "place", "contact",
	"message", "write", "happening", "experiencing", "experience", "everything", "mind", "would", "like",
)

// knowWords follow a negator in hedges that state no fact ("not sure",
// "belum tahu").
var knowWords = set("sure", "yakin", "tahu", "tau", "know", "paham", "understand")

// pressureLexicon is urgency, guarantees, guilt and coercion.
var pressureLexicon = newLexicon(map[string][]string{
	"pressure": {
		"harus segera", "pasti sembuh", "pasti bisa sembuh", "pasti berhasil", "dijamin", "jaminan", "terjamin",
		"jangan sampai terlambat", "sebelum terlambat", "sebelum semuanya terlambat", "sekarang juga",
		"kalau tidak sekarang", "kalau nggak sekarang", "anda harus", "kamu harus", "anda wajib", "kamu wajib",
		"hanya anda yang bisa", "hanya kamu yang bisa", "cuma anda yang bisa", "cuma kamu yang bisa",
		"anda tidak punya pilihan", "kamu tidak punya pilihan", "kalau anda peduli", "kalau kamu peduli",
		"anda egois", "kamu egois", "salahmu", "salah anda", "salah kamu", "anda akan menyesal",
		"kamu akan menyesal", "nanti menyesal", "jangan kecewakan", "mengecewakan saya",
		"you must", "you have to", "you need to", "guaranteed", "guarantee", "guarantees", "right now or",
		"only you can", "you have no choice", "before it's too late", "if you cared", "if you really cared",
		"you owe", "your fault", "you'll regret", "you will regret", "don't let me down", "you're selfish",
		"you are selfish", "act now", "immediately or", "promise me", "definitely get better", "cure me",
	},
})
