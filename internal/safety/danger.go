package safety

import (
	"regexp"
	"strings"
)

// The danger lexicon (ruleset 2026.11.01-1) closes gaps the v1 catalogue left
// open: a threat to hurt someone else, the speaker being hurt, threatened or
// in danger, prepared means, a written farewell and a few self-harm and
// suicide phrasings. It runs on the normalised text (lower case, leetspeak
// and symbols resolved; see Normalize) after the catalogue and can only raise
// the level: a match is crisis, or elevated when a negator stands inside the
// matched phrase ("aku nggak mau bunuh dia", "aku nggak pernah dipukul") or a
// game context makes a threat to others likely play ("aku mau bunuh dia di
// ranked"). Like the catalogue it decides which product path is shown; it is
// not a risk assessment. Ambiguity takes the safer path.

// CategoryInDanger: the speaker is being hurt, threatened, abused or is
// otherwise not safe.
const CategoryInDanger Category = "in_danger"

const (
	dangerSelfID = `(?:aku|saya|gue|gw|gua|aq|ak|sy|ane)`
	dangerIntent = `(?:mau|mo|mw|pengen|pengin|pingin|pgn|ingin|bakal|bakalan|akan|siap|niat|berniat|rencana|berencana|harus|pasti|beneran|udah\s+siap|sudah\s+siap)`
	// dangerTarget is a person or a place full of people; harming an
	// animal, an insect or time ("bunuh waktu") is not in it.
	dangerTarget = `(?:dia|dy|mereka|orang|org|orang-orang|semua|semuanya|kalian|kamu|km|lo|lu|elo|loe|ayah|ayahku|bapak|bapakku|bapak2|ibu|ibuku|mama|mamaku|papa|papaku|` +
		`suami|suamiku|istri|istriku|pacar|pacarku|mantan|mantanku|guru|bos|atasan|atasanku|teman|temen|temanku|temenku|tetangga|keluarga|adik|adikku|kakak|kakakku|anak|anakku|` +
		`rumah|rumahnya|sekolah|kantor|kampus|gedung|masjid|gereja|mall|pasar|kelas|si\s+\w+)`
	dangerHarmID = `(?:bunuh|bunuhin|membunuh|ngebunuh|habisi|habisin|ngabisin|menghabisi|tusuk|nusuk|menusuk|tusukin|bacok|ngebacok|membacok|` +
		`cekik|nyekik|mencekik|racunin|ngeracunin|meracuni|bakar|ngebakar|membakar|bom|ngebom|mengebom|tembak\s+mati|matiin|` +
		`bantai|membantai|ngebantai|aniaya|menganiaya|siram\s+air\s+keras|nyiram\s+air\s+keras)`
	dangerWeaponID = `(?:pisau|golok|parang|celurit|clurit|pistol|senjata|senapan|bom|air\s+keras|samurai|badik)`
	dangerHurtMeID = `(?:bunuh|bunuhin|ngebunuh|membunuh|bacok|ngebacok|tusuk|nusuk|habisin|habisi|bakar|tembak\s+mati|cekik|nyekik|pukul|mukul|mukulin|hajar|ngehajar|` +
		`perkosa|memperkosa|siksa|nyiksa|menyiksa|aniaya|culik|nyulik)`
	dangerVictimID = `(?:dipukul|dipukulin|dipukuli|dihajar|ditampar|ditendang|dicekik|ditusuk|dibacok|disiksa|diperkosa|dirudapaksa|dilecehkan|dicabuli|` +
		`disekap|dikurung|diculik|dianiaya|ditodong|dijual|diancam\s+(?:dibunuh|dibacok|ditusuk|dipukul|disebar|disebarin)|digrepe|digrepein)`
	dangerHitMeID = `(?:mukul|mukulin|memukul|memukuli|nampar|menampar|nendang|menendang|cekik|nyekik|mencekik|hajar|ngehajar|menghajar|siksa|nyiksa|menyiksa|` +
		`perkosa|memperkosa|lecehin|ngelecehin|melecehkan|nyekap|menyekap|ngurung|mengurung|culik|nyulik|menculik|grepe|grepein)`
	dangerOtherEN     = `(?:him|her|them|everyone|everybody|people|you|someone|somebody|all\s+of\s+them|my\s+(?:dad|father|mom|mother|husband|wife|boyfriend|girlfriend|ex|boss|teacher|brother|sister|family|neighbou?r|classmates?|coworkers?)|that\s+(?:guy|girl|man|woman|person|teacher|boss|bitch|bastard|kid))`
	dangerHarmEN      = `(?:kill|murder|stab|shoot|strangle|choke|poison|burn\s+down|bomb|blow\s+up|slaughter|massacre|hurt|beat\s+up)`
	dangerIntentEN    = `(?:'m\s+going\s+to|\s+am\s+going\s+to|'m\s+gonna|\s+am\s+gonna|\s+gonna|'ll|\s+will|\s+want\s+to|\s+wanna|\s+plan\s+to|\s+need\s+to|\s+have\s+to|\s+am\s+about\s+to|'m\s+about\s+to|\s+should|\s+could)`
	dangerOtherSubjEN = `(?:he|she|they|someone|somebody|this\s+guy|a\s+man|a\s+guy|my\s+(?:dad|father|mom|mother|husband|wife|boyfriend|girlfriend|ex|partner|stepdad|stepfather|uncle|brother|boss|teacher))`
	dangerHitMeEN     = `(?:beats?|beating|hits?|hitting|chokes?|choked|choking|strangled|strangles|raped|rapes|raping|abuses|abused|abusing|assaulted|kicks|kicked|punch(?:es|ed)?|slapped|slaps|` +
		`threaten(?:s|ed|ing)?\s+to\s+(?:kill|hurt|stab|shoot|rape)|(?:is|was)\s+(?:going|trying)\s+to\s+kill|wants\s+to\s+kill|will\s+kill|locked)`
)

type dangerRule struct {
	id       string
	category Category
	re       *regexp.Regexp
	// others marks a threat to other people: a game context downgrades it.
	others bool
	// after, when set, discards a match that this expression finds right
	// after it ("tali sepatu", "dipukul bola", "hits me with deadlines").
	after *regexp.Regexp
}

func dRule(id string, c Category, expr string) dangerRule {
	return dangerRule{id: id, category: c, re: regexp.MustCompile(expr)}
}

var dangerRules = []dangerRule{
	// Threats to hurt someone else.
	func() dangerRule {
		r := dRule("danger-id-harm-others", CategoryViolence,
			`\b`+dangerSelfID+`\b(?:\s+\S+){0,3}?\s+`+dangerIntent+`\b(?:\s+\S+){0,2}?\s+`+dangerHarmID+`\s+(?:para\s+|semua\s+)?`+dangerTarget+`\b`)
		r.others = true
		return r
	}(),
	func() dangerRule {
		r := dRule("danger-id-make-die", CategoryViolence,
			`\b`+dangerSelfID+`\b(?:\s+\S+){0,3}?\s+`+dangerIntent+`\b(?:\s+\S+){0,2}?\s+(?:bikin|buat|membuat)\s+`+dangerTarget+`\s+(?:mati|mampus|modar|koid)\b|\bhabis(?:in|i)\s+nyawa(?:nya)?\b`)
		r.others = true
		return r
	}(),
	dRule("danger-id-weapon", CategoryViolence,
		`\b(?:tembak|nembak|menembak)\s+(?:(?:semua|banyak|para)\s+(?:orang|murid|siswa|guru|karyawan)|orang-orang|mereka\s+semua|orang\s+(?:di|dalam)\s+(?:sekolah|kantor|kampus|mall|masjid|gereja|kelas|pasar))\b|`+
			`\b(?:tembak|nembak|menembak)\s+`+dangerTarget+`\s+(?:\S+\s+){0,2}?(?:pake|pakai|dengan|pakek)\s+(?:pistol|senapan|senjata|bedil)\b|`+
			`\b(?:bawa|bawain|siapin|nyiapin|menyiapkan|beli|udah\s+beli|sudah\s+beli|asah|ngasah)\s+`+dangerWeaponID+`\b(?:\s+\S+){0,3}?\s+(?:buat|untuk|ke)\s+`+dangerTarget+`\b`),
	func() dangerRule {
		r := dRule("danger-en-harm-others", CategoryViolence,
			`\bi`+dangerIntentEN+`(?:\s+(?:just|really|literally|actually|seriously|finally|fucking|go))?\s+`+dangerHarmEN+`\s+`+dangerOtherEN+`\b`)
		r.others = true
		r.after = regexp.MustCompile(`^(?:'s|s'|\s+in\s+(?:the\s+)?(?:game|match))`)
		return r
	}(),
	dRule("danger-en-weapon", CategoryViolence,
		`\bi(?:'m|\s+am|'ll|\s+will)?\s+(?:bringing|bring|taking|take|got|have|bought)\s+(?:a\s+|my\s+)?(?:gun|knife|rifle|bomb|weapon)\s+(?:to|into)\s+(?:school|class|work|the\s+office|church|the\s+mall)\b`),

	// The speaker is threatened, hurt, abused or not safe.
	dRule("danger-id-threat-to-me", CategoryInDanger,
		`\b(?:mau|akan|bakal|bakalan|ngancam|ngancem|mengancam|ancam|pengen|ingin|nyoba|mencoba|berusaha)\s+(?:\S+\s+){0,2}?`+dangerHurtMeID+`\s+(?:aku|saya|gue|gw|aq|ak|sy)\b`),
	func() dangerRule {
		r := dRule("danger-id-victim", CategoryInDanger,
			`\b`+dangerSelfID+`\b(?:\s+\S+){0,3}?\s+`+dangerVictimID+`\b`)
		r.after = dangerFigurative
		return r
	}(),
	dRule("danger-id-hurt-me", CategoryInDanger,
		`\b`+dangerHitMeID+`\s+(?:aku|saya|gue|gw|aq|ak|sy)\b`),
	dRule("danger-id-not-safe", CategoryInDanger,
		`\b`+dangerSelfID+`\b(?:\s+\S+){0,2}?\s+(?:(?:lagi\s+|sedang\s+)?dalam\s+bahaya|(?:nggak|gak|ga|tidak|enggak|ngga|tak)\s+aman\s+(?:di\s+rumah|di\s+sini|disini|(?:sama|dengan|dekat|deket)\s+`+dangerPerson+`))\b|`+
			`\b`+dangerSelfID+`\b(?:\s+\S+){0,2}?\s+korban\s+(?:kdrt|kekerasan|perkosaan|pemerkosaan|pelecehan|penganiayaan|perdagangan\s+orang)\b`),
	func() dangerRule {
		r := dRule("danger-en-hurt-me", CategoryInDanger,
			`\b`+dangerOtherSubjEN+`\s+(?:\S+\s+){0,2}?`+dangerHitMeEN+`\s+me\b`)
		r.after = regexp.MustCompile(`^\s+(?:up|with|in\s+(?:the\s+)?(?:game|match))\b`)
		return r
	}(),
	dRule("danger-en-victim", CategoryInDanger,
		`\bi\s*(?:was|got|'ve\s+been|have\s+been|am\s+being|'m\s+being|am|'m)\s+(?:just\s+|being\s+)?(?:raped|sexually\s+assaulted|assaulted|abused|beaten(?:\s+up)?|attacked|kidnapped|trafficked|choked|stabbed|held\s+captive|being\s+followed|stalked)\b`),
	dRule("danger-en-not-safe", CategoryInDanger,
		`\bi(?:'m|\s+am)\s+(?:\S+\s+)?(?:in\s+danger|not\s+safe(?:\s+at\s+home|\s+here)?|unsafe\s+at\s+home)\b`),

	// Prepared means, written farewells and phrasings the catalogue misses.
	func() dangerRule {
		r := dRule("danger-id-prepared-means", CategorySuicide,
			`\b(?:nyiapin|siapin|menyiapkan|nyiapkan|udah\s+beli|sudah\s+beli|ngumpulin|mengumpulkan|nimbun|menimbun|nyimpen)\s+(?:\S+\s+){0,2}?(?:tali|racun|obat\s+tidur|baygon|pestisida|silet|cutter)\b`)
		r.after = regexp.MustCompile(`^\s+(?:sepatu|jemuran|rafia|tas|pramuka|kado|layangan|rambut)\b`)
		return r
	}(),
	func() dangerRule {
		r := dRule("danger-id-farewell", CategorySuicide,
			`\b(?:ini|inilah)\s+(?:pesan|kata-kata|tulisan)\s+terakhir(?:ku)?\b|\b(?:pesan|surat)\s+(?:terakhirku|perpisahanku)\b|`+
				`\b(?:besok|nanti|minggu\s+depan)\s+(?:\S+\s+){0,2}?`+dangerSelfID+`\s+(?:\S+\s+)?(?:udah|sudah|udh|sdh)\s+(?:nggak|gak|tidak|ga|enggak|ngga)\s+(?:ada|di\s+sini|disini)(?:\s+lagi)?\s*(?:$|[.!?,\n])|`+
				`\bnyusul\s+(?:almarhum|almarhumah|alm|mendiang)\b|`+
				`\b`+dangerSelfID+`\b(?:\s+\S+){0,3}?\s+(?:pergi|pamit)\s+(?:untuk\s+)?selamanya\b`)
		r.after = regexp.MustCompile(`^\s+(?:dari|dr)\s+(?:grup|group|gc|kantor|sekolah|kampus|kota|komunitas|tim|organisasi|medsos|sosmed|ig|twitter|tiktok|chat|wa)\b`)
		return r
	}(),
	dRule("danger-id-vanish", CategorySuicide,
		`\b(?:pengen|pengin|ingin|mau|pingin|pgn)\s+(?:\S+\s+)?(?:ilang|hilang|lenyap)\s+(?:dari|dr)\s+(?:dunia|bumi)\b`),
	dRule("danger-id-self-harm-head", CategorySelfHarm,
		`\b(?:mukul|mukul-mukul|mukulin|benturin|membenturkan|jedotin|jedot-jedotin|ngejedotin)\s+kepala(?:ku)?\s+ke\s+(?:tembok|dinding|lantai|meja|kaca)\b`),
	dRule("danger-en-prepared-means", CategorySuicide,
		`\bi\s+(?:have|'ve\s+got|got)\s+(?:the\s+|my\s+|a\s+)?(?:pills|rope|gun|blades?|razor)\s+ready\b|`+
			`\bi(?:'ve|\s+have)?\s+(?:been\s+)?(?:saving|stockpiling|hoarding|collecting)\s+(?:up\s+)?(?:my\s+)?(?:pills|meds|medication|sleeping\s+pills)\b|\bi\s+bought\s+a\s+(?:rope|gun)\b`),
	dRule("danger-en-farewell", CategorySuicide,
		`\bthis\s+is\s+my\s+last\s+(?:message|post|goodbye|note)\b|\bmy\s+last\s+words\b|\bi\s+won'?t\s+be\s+(?:here|around|alive)\s+(?:tomorrow|anymore)\b`),
}

// dangerPerson is someone the speaker can be unsafe with.
const dangerPerson = `(?:dia|dy|mereka|orang\s+itu|ayah|ayahku|bapak|bapakku|ibu|ibuku|mama|mamaku|papa|papaku|ortu|orang\s*tua(?:ku)?|suami|suamiku|istri|istriku|` +
	`pacar|pacarku|mantan|mantanku|keluarga(?:ku)?|om|omku|paman|pamanku|tante|kakak|kakakku|abang|abangku|adik|adikku|tetangga|bos|atasan(?:ku)?|guru|teman|temen|temanku|temenku)`

var (
	// dangerFigurative is what follows a figurative "hit by": a ball, a
	// fact, a deadline, life ("aku ditampar realita", "dihajar tugas").
	dangerFigurative = regexp.MustCompile(`^\s+(?:oleh\s+|sama\s+|ama\s+)?(?:bola|realita|realitas|kenyataan|keadaan|tugas|deadline|kerjaan|pekerjaan|ujian|pr|nilai|hidup|nasib|takdir|angin|ombak|hujan|rindu|kangen|cinta|rasa|tagihan|cicilan|macet|rutinitas|lagu|film|meme|komentar|pertanyaan)\b`)
	dangerNegator    = regexp.MustCompile(`\b(?:nggak|ngga|gak|ga|tidak|tak|enggak|engga|bukan|jangan|belum|pernah\s+nggak|never|not|don'?t|won'?t|wouldn'?t|didn'?t|isn'?t|wasn'?t|can'?t|cannot)\b`)
	dangerGame       = regexp.MustCompile(`\b(?:game|gaming|ranked|rank|mabar|main\s+(?:game|ml|ff|pubg)|ml|mobile\s+legends?|ff|free\s+fire|pubg|valorant|minecraft|roblox|genshin|boss|level|ronde|round|match|karakter|character|npc|zombie|zombi|mode\s+cerita|fortnite|apex|cod|call\s+of\s+duty|overwatch|dota|csgo|cs2|counter-strike|gta|among\s+us|chess|catur|tekken|pvp|lobby|squad|respawn)\b`)
)

// assessDanger returns the danger rules matched in text, each with the level
// it counts at.
func assessDanger(text string) (ids []string, cats []Category, level Level) {
	norm := strings.ReplaceAll(Normalize(text), "’", "'")
	level = LevelNone
	game := dangerGame.MatchString(norm)
	for i := range dangerRules {
		r := &dangerRules[i]
		hit := LevelNone
		for _, loc := range r.re.FindAllStringIndex(norm, -1) {
			m := norm[loc[0]:loc[1]]
			if r.after != nil && r.after.MatchString(norm[loc[1]:]) {
				continue
			}
			l := LevelCrisis
			if dangerNegator.MatchString(m) && !negatorIsTheHarm(r, m) {
				l = LevelElevated
			}
			if r.others && game {
				l = LevelElevated
			}
			if rank(l) > rank(hit) {
				hit = l
			}
		}
		if hit == LevelNone {
			continue
		}
		ids = append(ids, r.id)
		cats = append(cats, r.category)
		if rank(hit) > rank(level) {
			level = hit
		}
	}
	return ids, cats, level
}

// negatorIsTheHarm keeps a negator that is part of the danger itself: "aku
// nggak aman di rumah" (not safe) and "besok aku udah nggak ada" (gone) are
// the risk, not a denial of it.
func negatorIsTheHarm(r *dangerRule, m string) bool {
	switch r.id {
	case "danger-id-not-safe", "danger-en-not-safe", "danger-en-farewell":
		return true
	case "danger-id-farewell":
		return strings.Contains(m, "ada") || strings.Contains(m, "sini")
	}
	return false
}

func rank(l Level) int {
	switch l {
	case LevelCrisis:
		return 2
	case LevelElevated:
		return 1
	}
	return 0
}

// withDanger folds the danger lexicon into a catalogue assessment. It only
// ever raises the level and adds rule ids and categories.
func withDanger(a Assessment, text string) Assessment {
	ids, cats, level := assessDanger(text)
	if len(ids) == 0 {
		return a
	}
	a.RuleIDs = append(a.RuleIDs, ids...)
	for _, c := range cats {
		seen := false
		for _, have := range a.Categories {
			if have == c {
				seen = true
				break
			}
		}
		if !seen {
			a.Categories = append(a.Categories, c)
		}
	}
	if rank(level) > rank(a.Level) {
		a.Level = level
	}
	if level == LevelCrisis {
		// The speaker's own danger or threat is never only someone else's.
		a.ThirdParty = false
	}
	return a
}
