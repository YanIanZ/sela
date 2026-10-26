package ledger

import (
	"regexp"
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

// Clinical condition labels (v1 DIAGNOSTIC_LABEL plus English and a few
// common labels). Go's RE2 has no lookbehind; v1's patterns need none, and
// matching runs on safetyText so punctuation and hyphens are already spaces.
var diagnosisLabelRe = regexp.MustCompile(`\b(?:depresi|depression|bipolar|skizofrenia|schizophrenia|skizoafektif|schizoaffective|psikosis|psychosis|ptsd|ocd|adhd|gad|mdd|anxiety\s+disorder|gangguan\s+(?:kecemasan|cemas|panik|jiwa|mental|kepribadian|makan|bipolar|obsesif|stres\s+pasca\s*trauma)|panic\s+disorder|eating\s+disorder|personality\s+disorder|major\s+depressive|obsessive\s+compulsive|post\s+traumatic\s+stress|anoreksia|anorexia|bulimia|borderline|bpd|insomnia\s+kronis)(?:ku|mu|nya)?\b`)

// clinicianAttributionRe recognises a label reported as a clinician's
// finding ("didiagnosis ... oleh psikiater", "kata dokter", "my doctor said").
var clinicianAttributionRe = regexp.MustCompile(`\b(?:didiagnosis|didiagnosa|terdiagnosis|terdiagnosa|diagnosis\s+dari|diagnosa\s+dari|(?:kata|menurut|oleh|dari)\s+(?:dokter|psikiater|psikolog)(?:ku|nya)?|(?:dokter|psikiater|psikolog)(?:ku|nya)?\s+(?:bilang|mengatakan|mendiagnosis|mendiagnosa|menyatakan)|diagnosed|(?:my|the|a)\s+(?:doctor|psychiatrist|psychologist|therapist|gp)\s+(?:said|says|told\s+me|diagnosed|thinks)|according\s+to\s+(?:my|the)\s+(?:doctor|psychiatrist|psychologist|therapist))\b`)

const (
	condPattern       = `(?:bipolar|depresi(?:\s+mayor)?|depression|skizofrenia|schizophrenia|psikosis|psychosis|gangguan\s+kecemasan|gangguan\s+panik|anxiety\s+disorder|panic\s+disorder|adhd|gad|ocd|ptsd|bpd|borderline)`
	medicationPattern = `(?:obat(?:ku|mu|nya)?|medikasi|medication|medications|meds|dosis(?:ku|mu|nya)?|dose|doses|antidepresan|antidepressants?|antipsikotik|antipsychotics?|xanax|sertraline|fluoxetine|alprazolam|zoloft|prozac|pil\s+tidur|sleeping\s+pills?|pills?|tablets?)`
	clinicianPattern  = `(?:dokter(?:ku|mu|nya)?|doctors?|psikiater(?:ku|mu|nya)?|apoteker(?:ku|mu|nya)?|tenaga\s+kesehatan|clinician)`
	overridePattern   = `(?:abaikan|ignore|jangan\s+(?:ikuti|dengarkan|dengerin)|tak\s+perlu\s+ikuti|lawan)`
)

var (
	definitiveDiagnosisRe  = regexp.MustCompile(`\b(?:kamu|anda|lu|loe|you)\s+(?:(?:pasti|jelas|memang|definitely|certainly|clearly)\s+)?(?:mengalami|menderita|mengidap|terkena|terdiagnosis|didiagnosis|memiliki|punya|have|has|are|suffer\s+from)\s+` + condPattern + `\b|\b(?:kamu|anda|lu|loe|you)\s+(?:(?:pasti|jelas|definitely|certainly)\s+|(?:adalah|itu)\s+)?` + condPattern + `\b`)
	medicationDirectiveRe  = regexp.MustCompile(`\b(?:berhenti(?:lah)?|hentikan|stop|jangan\s+(?:lagi\s+)?(?:minum|konsumsi|consume|take)|mulai(?:lah)?|start|naikkan|tingkatkan|increase|turunkan|kurangi|reduce|double|ganti(?:kan)?|switch|ubah|gabungkan|kombinasikan|combine|mix|skip|lewati|minumlah|minum|konsumsi|consume|take|ambil(?:lah)?|gunakan|pakai)\b(?:\s+\p{L}+){0,4}\s+` + medicationPattern + `\b`)
	professionalOverrideRe = regexp.MustCompile(`\b` + overridePattern + `\b.{0,48}\b` + clinicianPattern + `\b|\b` + clinicianPattern + `\b.{0,48}\b` + overridePattern + `\b`)
	treatmentDirectiveRe   = regexp.MustCompile(`\b(?:(?:kamu|anda|lu|loe|you)?\s*(?:harus|perlu|wajib)\s+(?:(?:menjalani|mulai|minum)\s+)?|(?:lakukan|jalani|mulai|cobalah)\s+)(?:pengobatan|treatment|terapi\s+(?:medis|cbt|paparan|elektrokonvulsif)|rawat\s+inap|ect|terapi\s+kejut)\b|\brawat\s+inap\s+(?:sekarang|hari\s+ini|minggu\s+ini)\b|\b(?:must|should|need\s+to|have\s+to)\s+(?:(?:get|start|begin|undergo|receive)\s+)?(?:treatment|medical\s+treatment|hospitali[sz]ation|ect|electroconvulsive\s+therapy)\b`)
	reportedByClinicianRe  = regexp.MustCompile(`\b(?:oleh|dari)\s+(?:dokter|psikiater|psikolog|tenaga\s+kesehatan)\b`)
	compactDirectiveRe     = regexp.MustCompile(`(?:stop|minum|naikkan|turunkan|ganti|double)(?:minum|take)?(?:obat|medikasi|medication|dosis|dose)`)
)

// ContainsDiagnosisLabel reports a clinical condition label ("depresi",
// "bipolar", "anxiety disorder", ...), with or without a "-ku/-mu/-nya"
// clitic. It matches across case, width, accents and punctuation.
func ContainsDiagnosisLabel(s string) bool { return diagnosisLabelRe.MatchString(safetyText(s)) }

// clinicianAttributed reports whether s attributes a finding to a clinician.
func clinicianAttributed(s string) bool { return clinicianAttributionRe.MatchString(safetyText(s)) }

// ContainsMedicalInstruction reports a medication or treatment directive, an
// instruction to ignore a clinician, or a definitive diagnosis addressed to
// the reader ("kamu pasti depresi"). It ports v1 containsUnsafeMedicalAdvice:
// a definitive diagnosis reported as a clinician's ("oleh dokter") does not
// count, and spacing/punctuation evasions ("stop.minum.obat") are caught.
func ContainsMedicalInstruction(s string) bool {
	text := safetyText(s)
	compact := strings.ReplaceAll(text, " ", "")
	return (definitiveDiagnosisRe.MatchString(text) && !reportedByClinicianRe.MatchString(text)) ||
		medicationDirectiveRe.MatchString(text) ||
		professionalOverrideRe.MatchString(text) ||
		treatmentDirectiveRe.MatchString(text) ||
		compactDirectiveRe.MatchString(compact)
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
