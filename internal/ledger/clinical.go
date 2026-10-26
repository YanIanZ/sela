package ledger

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// safetyText mirrors v1 normalizeSafetyText: folded and lowercased, with
// every run of non-letter, non-digit runes collapsed to one space.
func safetyText(s string) string {
	var b strings.Builder
	gap := false
	for _, f := range foldSpans(s) {
		if !isWordRune(f.r) {
			gap = true
			continue
		}
		if gap && b.Len() > 0 {
			b.WriteByte(' ')
		}
		gap = false
		b.WriteRune(f.r)
	}
	return b.String()
}

// foldedText is s folded rune by rune (punctuation kept).
func foldedText(s string) string {
	var b strings.Builder
	for _, f := range foldSpans(s) {
		b.WriteRune(f.r)
	}
	return b.String()
}

// Clinical condition labels (v1 DIAGNOSTIC_LABEL plus English, adjective and
// inflected forms, and a few common labels). Go's RE2 has no lookbehind; v1's
// patterns need none, and matching runs on safetyText so punctuation and
// hyphens are already spaces. "depressed" is a label for the same reason v1
// treats the colloquial "aku depresi" as one.
var diagnosisLabelRe = regexp.MustCompile(`\b(?:` +
	`(?:ter|di)?depresi|depresif|depression|depressive|depressed|bipolar|skizofrenia|skizofren|schizophrenia|` +
	`schizophrenic|skizoafektif|schizoaffective|psikosis|psikotik|psychosis|psychotic|manic|mania|autis|autisme|` +
	`autistik|autism|autistic|ptsd|ocd|adhd|gad|mdd|anxiety\s+disorder|` +
	`gangguan\s+(?:kecemasan|cemas|panik|jiwa|mental|kepribadian|makan|bipolar|obsesif|disosiatif|stres\s+pasca\s*trauma)|` +
	`panic\s+disorder|eating\s+disorder|personality\s+disorder|dissociative\s+identity|major\s+depressive|` +
	`obsessive\s+compulsive|post\s+traumatic\s+stress|social\s+phobia|fobia\s+sosial|agorafobia|agoraphobia|` +
	`anoreksia|anoreksik|anorexia|anorexic|bulimia|bulimik|bulimic|borderline|bpd|insomnia\s+kronis` +
	`)(?:ku|mu|nya)?\b`)

const (
	condPattern = `(?:bipolar|depresi(?:\s+mayor)?|depression|skizofrenia|schizophrenia|psikosis|psychosis|gangguan\s+kecemasan|gangguan\s+panik|anxiety\s+disorder|panic\s+disorder|adhd|gad|ocd|ptsd|bpd|borderline)`
	// medicationPattern names medication, doses and prescriptions.
	medicationPattern = `(?:obat(?:ku|mu|nya)?|obatan|medikasi|medication|medications|medicine|medicines|meds|` +
		`dosis(?:ku|mu|nya)?|dose|doses|dosage|antidepresan|antidepressants?|antipsikotik|antipsychotics?|` +
		`antiansietas|anxiolytics?|sedatif|sedatives?|penenang|tranquili[sz]ers?|xanax|sertraline|fluoxetine|` +
		`alprazolam|zoloft|prozac|escitalopram|lexapro|citalopram|paroxetine|venlafaxine|bupropion|lithium|litium|` +
		`quetiapine|seroquel|risperidone|aripiprazole|olanzapine|clonazepam|lorazepam|diazepam|valium|ativan|` +
		`klonopin|zolpidem|ambien|trazodone|mirtazapine|amitriptyline|amitriptilin|methylphenidate|ritalin|` +
		`adderall|melatonin|ssris?|snris?|benzodiazepines?|benzodiazepin|benzos?|pil\s+tidur|sleeping\s+pills?|` +
		`pills?|tablets?|resep(?:nya)?|prescriptions?|suplemen|supplements?)`
	clinicianPattern = `(?:dokter(?:ku|mu|nya)?|doctors?|psikiater(?:ku|mu|nya)?|apoteker(?:ku|mu|nya)?|tenaga\s+kesehatan|clinician)`
	overridePattern  = `(?:abaikan|ignore|jangan\s+(?:ikuti|dengarkan|dengerin)|tak\s+perlu\s+ikuti|lawan)`
	// secondPerson addresses the reader; adverbPattern may stand between it
	// and a modal ("kamu mungkin butuh", "you really need").
	secondPerson  = `(?:kamu|anda|lu|loe|lo|kau|engkau|you|u)`
	adverbPattern = `(?:(?:mungkin|memang|benar|sangat|pasti|jelas|juga|sebenarnya|really|probably|definitely|clearly|just|also|still|actually|seriously)\s+){0,2}`
)

var (
	definitiveDiagnosisRe = regexp.MustCompile(`\b(?:kamu|anda|lu|loe|you)\s+(?:(?:pasti|jelas|memang|definitely|certainly|clearly)\s+)?(?:mengalami|menderita|mengidap|terkena|terdiagnosis|didiagnosis|memiliki|punya|have|has|are|suffer\s+from)\s+` + condPattern + `\b|\b(?:kamu|anda|lu|loe|you)\s+(?:(?:pasti|jelas|definitely|certainly)\s+|(?:adalah|itu)\s+)?` + condPattern + `\b`)
	medicationDirectiveRe = regexp.MustCompile(`\b(?:berhenti(?:lah)?|hentikan|stop|jangan\s+(?:lagi\s+)?(?:minum|konsumsi|consume|take)|mulai(?:lah)?|start|naikkan|naikin|tingkatkan|increase|turunkan|turunin|kurangi|kurangin|reduce|double|perbanyak|tambah(?:kan|in)?|ganti(?:kan)?|switch|ubah|gabungkan|kombinasikan|combine|mix|skip|lewati|minumlah|minum|konsumsi|consume|take|ambil(?:lah)?|gunakan|pakai)\b(?:\s+\p{L}+){0,4}\s+` + medicationPattern + `\b`)
	// needMedicationRe: the reader is told they need or should get medication
	// ("kamu butuh antidepresan", "you should get a prescription").
	needMedicationRe = regexp.MustCompile(`\b` + secondPerson + `\s+` + adverbPattern + `(?:butuh|perlu|harus|wajib|mesti|sebaiknya|need|needs|should|must|have\s+to|ought\s+to|better)\b(?:\s+\p{L}+){0,4}\s+` + medicationPattern + `\b`)
	// imperativeMedicationRe is matched per sentence: an imperative verb opens
	// the sentence ("coba melatonin", "ask your doctor for xanax").
	imperativeMedicationRe = regexp.MustCompile(`^(?:(?:tolong|please|silakan|just|langsung|sebaiknya|mending|mendingan|better)\s+)?(?:coba(?:lah|in)?|try|ask|minta(?:lah)?|mintakan|tanyakan|get|dapatkan|cari(?:lah)?|beli(?:lah)?|buy|use)\b(?:\s+\p{L}+){0,4}\s+` + medicationPattern + `\b`)
	professionalOverrideRe = regexp.MustCompile(`\b` + overridePattern + `\b.{0,48}\b` + clinicianPattern + `\b|\b` + clinicianPattern + `\b.{0,48}\b` + overridePattern + `\b`)
	treatmentDirectiveRe   = regexp.MustCompile(`\b(?:(?:kamu|anda|lu|loe|you)?\s*(?:harus|perlu|wajib|mesti|sebaiknya)\s+(?:(?:segera|menjalani|mulai|minum)\s+)?|(?:lakukan|jalani|mulai|cobalah)\s+)(?:pengobatan|treatment|terapi\s+(?:medis|cbt|paparan|elektrokonvulsif)|rawat\s+inap|dirawat(?:\s+inap)?|diopname|opname|masuk\s+rumah\s+sakit|ect|terapi\s+kejut)\b|\brawat\s+inap\s+(?:sekarang|hari\s+ini|minggu\s+ini)\b|\b(?:must|should|need\s+to|needs\s+to|have\s+to|has\s+to|ought\s+to)\s+(?:(?:be|get|start|begin|undergo|receive|go|consider)\s+){0,2}(?:treatment|medical\s+treatment|hospitali[sz]ed|hospitali[sz]ation|admitted|ect|electroconvulsive\s+therapy|medicated)\b`)
	reportedByClinicianRe  = regexp.MustCompile(`\b(?:oleh|dari)\s+(?:dokter|psikiater|psikolog|tenaga\s+kesehatan)\b`)
	compactDirectiveRe     = regexp.MustCompile(`(?:stop|minum|naikkan|turunkan|ganti|double)(?:minum|take)?(?:obat|medikasi|medication|dosis|dose)`)
)

// ContainsDiagnosisLabel reports a clinical condition label ("depresi",
// "bipolar", "anxiety disorder", "depresif", "psychotic", ...), with or
// without a "-ku/-mu/-nya" clitic. It matches across case, width, accents
// and punctuation.
func ContainsDiagnosisLabel(s string) bool { return diagnosisLabelRe.MatchString(safetyText(s)) }

// ContainsMedicalInstruction reports a medication or treatment directive
// ("kamu butuh antidepresan", "coba melatonin", "you should get a
// prescription", "kamu harus dirawat inap"), an instruction to ignore a
// clinician, or a definitive diagnosis addressed to the reader ("kamu pasti
// depresi"). It ports v1 containsUnsafeMedicalAdvice: a definitive diagnosis
// reported as a clinician's ("oleh dokter") does not count, and
// spacing/punctuation evasions ("stop.minum.obat") are caught.
func ContainsMedicalInstruction(s string) bool {
	text := safetyText(s)
	compact := strings.ReplaceAll(text, " ", "")
	if (definitiveDiagnosisRe.MatchString(text) && !reportedByClinicianRe.MatchString(text)) ||
		medicationDirectiveRe.MatchString(text) ||
		needMedicationRe.MatchString(text) ||
		professionalOverrideRe.MatchString(text) ||
		treatmentDirectiveRe.MatchString(text) ||
		compactDirectiveRe.MatchString(compact) {
		return true
	}
	for _, sentence := range strings.FieldsFunc(foldedText(s), isHardBoundary) {
		if imperativeMedicationRe.MatchString(safetyText(sentence)) {
			return true
		}
	}
	return false
}

// clinicianNouns can attribute a diagnosis (after stripping "-ku/-mu/-nya").
var clinicianNouns = set("dokter", "dr", "psikiater", "psikolog", "terapis", "doctor", "doctors", "psychiatrist",
	"psychologist", "therapist", "gp", "physician", "clinician")

// passiveDiagnosis are Indonesian passive forms of "to diagnose".
var passiveDiagnosis = set("didiagnosis", "didiagnosa", "terdiagnosis", "terdiagnosa")

// beAuxiliaries before "diagnosed" make it passive ("I was diagnosed").
var beAuxiliaries = set("was", "were", "been", "got", "get", "be", "is", "am", "are", "being")

// beliefWords between an attribution and a label end the attribution's
// scope: the label is then the user's own belief ("kata dokter aku sehat,
// aku yakin aku bipolar").
var beliefWords = set("yakin", "rasa", "kurasa", "pikir", "kupikir", "kira", "sure", "think", "believe", "guess",
	"feel", "merasa", "ngerasa")

// coordinators followed by a new subject also end the scope ("... dan aku
// bipolar"), while coordinated labels stay in it ("depresi dan PTSD").
var coordinators = set("dan", "and", "terus", "lalu", "kemudian", "serta", "plus", "so", "jadi")

// attribution is a clinician attribution over tokens [start, end); frame is
// true when its content may follow in the next clause ("kata dokter, ...").
type attribution struct {
	start, end int
	frame      bool
}

func isClinician(t string) bool { return clinicianNouns[t] || clinicianNouns[stripPossessive(t)] }

func isDeterminer(t string) bool {
	return possessiveDeterminers[t] || t == "the" || t == "a" || t == "an"
}

// attributions finds clinician attributions in toks: "kata/menurut
// <clinician>", "according to <clinician>", "<clinician> bilang/said/
// mendiagnosis", "oleh/dari/by <clinician>", "diagnosis dari <clinician>",
// passive "didiagnosis" and "was/been diagnosed". An active "diagnosed"
// without a clinician ("I diagnosed myself") is not one.
func attributions(toks []token) []attribution {
	var out []attribution
	in := func(i, j int) bool { return j >= 0 && j < len(toks) && toks[j].clause == toks[i].clause }
	// clinicianAt returns the index just past "[determiner] <clinician>" at j.
	clinicianAt := func(i, j int) int {
		if in(i, j) && isDeterminer(toks[j].text) {
			j++
		}
		if in(i, j) && isClinician(toks[j].text) {
			return j + 1
		}
		return -1
	}
	for i, tk := range toks {
		t := tk.text
		switch {
		case t == "kata" || t == "menurut":
			if e := clinicianAt(i, i+1); e > 0 {
				out = append(out, attribution{start: i, end: e, frame: true})
			}
		case t == "according" && in(i, i+1) && toks[i+1].text == "to":
			if e := clinicianAt(i, i+2); e > 0 {
				out = append(out, attribution{start: i, end: e, frame: true})
			}
		case passiveDiagnosis[t]:
			out = append(out, attribution{start: i, end: i + 1, frame: true})
		case t == "diagnosed" && in(i, i-1) && beAuxiliaries[toks[i-1].text]:
			out = append(out, attribution{start: i - 1, end: i + 1, frame: true})
		case (stripPossessive(t) == "diagnosis" || stripPossessive(t) == "diagnosa") && in(i, i+1) &&
			(toks[i+1].text == "dari" || toks[i+1].text == "oleh" || toks[i+1].text == "from" || toks[i+1].text == "by"):
			if e := clinicianAt(i, i+2); e > 0 {
				out = append(out, attribution{start: i, end: e, frame: true})
			}
		case isClinician(t):
			start := i
			if in(i, i-1) && isDeterminer(toks[i-1].text) {
				start = i - 1
			}
			// "<clinician> bilang/said", allowing one adverb ("dokterku juga
			// bilang", "my doctor also said").
			j := i + 1
			if in(i, j) && (toks[j].text == "juga" || toks[j].text == "sudah" || toks[j].text == "pernah" ||
				toks[j].text == "also" || toks[j].text == "once") {
				j++
			}
			if in(i, j) && (speechVerbs[toks[j].text] || toks[j].text == "menyatakan" || toks[j].text == "mendiagnosis" ||
				toks[j].text == "mendiagnosa" || toks[j].text == "diagnosed") {
				out = append(out, attribution{start: start, end: j + 1, frame: true})
				continue
			}
			// "oleh psikiater", "by my GP".
			if in(start, start-1) {
				switch toks[start-1].text {
				case "oleh", "dari", "by":
					out = append(out, attribution{start: start - 1, end: i + 1})
				}
			}
		}
	}
	return out
}

// negatedAt reports a negator within negationWindow tokens before toks[i] in
// its clause ("belum didiagnosis", "nobody diagnosed me").
func negatedAt(toks []token, i int) bool {
	for k := i - 1; k >= 0 && k >= i-negationWindow && toks[k].clause == toks[i].clause; k-- {
		if isNegator(toks[k].text) {
			return true
		}
	}
	return false
}

// broken reports a belief word, or a coordinator followed by a pronoun, among
// toks[from:to].
func broken(toks []token, from, to int) bool {
	for k := from; k < to; k++ {
		t := toks[k].text
		if beliefWords[t] {
			return true
		}
		if coordinators[t] && k+1 < to && (firstPerson[toks[k+1].text] || thirdPartyPronouns[toks[k+1].text]) {
			return true
		}
	}
	return false
}

// covers reports whether attribution a covers the label over tokens [ls, le).
func (a attribution) covers(toks []token, ls, le int) bool {
	first, last := toks[a.start], toks[a.end-1]
	lab, labEnd := toks[ls], toks[le-1]
	if lab.region != first.region || labEnd.clause != lab.clause {
		return false
	}
	switch {
	case lab.clause == last.clause && ls >= a.end:
		// "didiagnosis depresi", "my doctor said I have ADHD".
		return !broken(toks, a.end, ls)
	case lab.clause == first.clause && le <= a.start:
		// "depresi oleh psikiater", "depression, diagnosed by my GP".
		return !broken(toks, le, a.start)
	case a.frame && ls >= a.end && a.end < len(toks) && toks[a.end].clause != last.clause &&
		toks[a.end].clause == lab.clause:
		// The frame ends its clause and the label is in the next one: "kata
		// dokter, aku depresi", "diagnosis dari psikiater: depresi".
		return !broken(toks, a.end, ls)
	case a.frame && le <= a.start && first.clause != lab.clause && toks[a.start-1].clause == lab.clause &&
		(a.end == len(toks) || toks[a.end].region != first.region):
		// "aku depresi, kata dokter." (the frame closes the sentence).
		return !broken(toks, le, a.start)
	}
	return false
}

// labelsAttributed reports whether s names at least one clinical label and
// every label is reported as a clinician's finding in its own clause: after a
// diagnosis verb or reported-speech frame ("didiagnosis depresi", "kata
// dokter, aku depresi", "my doctor said I have ADHD") or followed by one
// ("depresi oleh psikiater", "aku depresi, kata dokter"). A negated
// attribution ("belum didiagnosis", "nobody diagnosed me"), a contrastive
// conjunction ("kata dokter aku sehat, tapi aku bipolar"), a new conjunct or
// the user's own belief between them breaks the attribution.
func labelsAttributed(s string) bool {
	toks := tokenize(s)
	labels := labelSpans(toks)
	if len(labels) == 0 {
		return false
	}
	var atts []attribution
	for _, a := range attributions(toks) {
		if !negatedAt(toks, a.start) {
			atts = append(atts, a)
		}
	}
	for _, l := range labels {
		ok := false
		for _, a := range atts {
			if a.covers(toks, l[0], l[1]) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// labelSpans returns the token spans [start, end) of diagnosis labels in toks.
func labelSpans(toks []token) [][2]int {
	var b strings.Builder
	var partTok, partOff []int
	for i, t := range toks {
		for _, p := range wordParts(t.text) {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			partTok = append(partTok, i)
			partOff = append(partOff, b.Len())
			b.WriteString(p)
		}
	}
	var out [][2]int
	for _, m := range diagnosisLabelRe.FindAllStringIndex(b.String(), -1) {
		i := sort.SearchInts(partOff, m[0])
		j := sort.SearchInts(partOff, m[1]) - 1
		if i < len(partOff) && j >= i {
			out = append(out, [2]int{partTok[i], partTok[j] + 1})
		}
	}
	return out
}

var (
	urlRe         = regexp.MustCompile(`\bhttps?://|\bwww\.|\b[a-z0-9-]+\.(?:com|net|org|id|co|io|me|info|biz|ly|gg|app)\b`)
	placeholderRe = regexp.MustCompile(`\[[A-Z_]+\]`)
	phoneRunRe    = regexp.MustCompile(`\+?\d[\d\s().-]{5,}\d`)
)

// containsIdentifier reports URLs, e-mail addresses, @handles, redaction
// placeholders ("[NAMA]") and phone-like digit runs (eight or more digits).
func containsIdentifier(s string) bool {
	if placeholderRe.MatchString(s) {
		return true
	}
	f := foldedText(s)
	if strings.ContainsRune(f, '@') || urlRe.MatchString(f) {
		return true
	}
	for _, run := range phoneRunRe.FindAllString(f, -1) {
		digits := 0
		for _, r := range run {
			if unicode.IsDigit(r) {
				digits++
			}
		}
		if digits >= 8 {
			return true
		}
	}
	return false
}

var digitRunRe = regexp.MustCompile(`\p{Nd}+(?:[.,]\p{Nd}+)*`)

// quantities lists the digit runs and number words in s, and whether s
// contains a percent sign.
func quantities(s string) (nums map[string]bool, percent bool) {
	nums = map[string]bool{}
	for _, t := range tokenize(s) {
		for _, d := range digitRunRe.FindAllString(t.text, -1) {
			nums[strings.ReplaceAll(d, ",", ".")] = true
		}
		for _, p := range wordParts(t.text) {
			if numberWords[p] {
				nums[p] = true
			}
		}
	}
	return nums, strings.ContainsAny(foldedText(s), "%‰")
}

// inventsNumber reports whether text contains a digit, number word or
// percentage that source does not.
func inventsNumber(text, source string) bool {
	have, srcPct := quantities(source)
	return addsQuantity(text, have, srcPct)
}

// addsQuantity reports whether text has a quantity outside have, or a
// percent sign when havePct is false.
func addsQuantity(text string, have map[string]bool, havePct bool) bool {
	got, pct := quantities(text)
	if pct && !havePct {
		return true
	}
	for n := range got {
		if !have[n] {
			return true
		}
	}
	return false
}
