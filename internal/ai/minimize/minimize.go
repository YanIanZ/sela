// Package minimize builds provider payloads under ADR-0003 §9:
//
//   - DM-1: payloads are built only from spans the caller selected
//     explicitly; a span's caller key (a database id) is never serialized,
//     and the payload types have no field for any account, user, session,
//     conversation or database identifier;
//   - DM-2: contact details (e-mail addresses, phone numbers, NIK and card
//     numbers, URLs, social handles, postal addresses) become [KONTAK-n],
//     and names the user marked private or introduced themselves with
//     ("namaku Raka", "panggil aku Raka") become [NAMA-n], before any
//     call; the mapping back is kept in Go for the owner's own view only;
//   - DM-3: spans get opaque per-request ids ("s1", "s2", ...);
//   - a size cap bounds every span and the whole payload.
//
// The patterns are a port of the v1 services/validator/src/piiRedactor.ts
// (email, URL, NIK, phone, card, handle, address_hint, person_name_hint),
// extended with grouped NIK forms ("3273 2504 9001 0003",
// "32.73.25.040990.0003"), handles named by a platform cue ("ig aku
// raka.putra"), any host with a path whatever its top-level domain
// ("saweria.co/raka", "discord.gg/x"), street addresses in any case when a
// house number follows ("jalan melati no 12"), and the informal name cues
// "dipanggil" and "manggil". Matching runs on the normalised views of
// internal/ai/textnorm, the same normalisation the output guard uses, so
// fullwidth, accented, zero-width and homoglyph spellings, no-break spaces
// and Unicode dashes are recognised; the original bytes are replaced.
//
// Names are matched whole-word with textnorm.FindName on a view that keeps
// apostrophes, so possessives, Indonesian enclitics, the reduplication
// digit and repeated letters ("Raka's", "Rakanya", "Dewiku", "Dewi2",
// "Dewiii") are redacted too. A name the user introduced themselves with
// ("namaku Raka Putra") is learned for the whole request: Build reads every
// span for such cues first, so every later mention of the name, and of its
// given name ("Raka"), in any span gets the same placeholder; Names lists
// them for the output guard.
//
// Placeholders already in a text (an earlier reply that was redacted
// before) are kept, and the session numbers new values after the highest
// kept number, so two people never share one placeholder.
//
// Redaction is deterministic and conservative; it reduces accidental
// disclosure but does not guarantee anonymity. Nothing here logs.
package minimize

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/ai/textnorm"
)

// Kind is a redacted kind.
type Kind string

// Redacted kinds.
const (
	KindEmail    Kind = "email"
	KindURL      Kind = "url"
	KindNIK      Kind = "nik"
	KindCard     Kind = "card"
	KindPhone    Kind = "phone"
	KindHandle   Kind = "handle"
	KindAddress  Kind = "address"
	KindName     Kind = "name"
	KindNameHint Kind = "name_hint"
)

// nameRe is v1's NAME: one to three capitalised words.
const nameRe = `[A-Z][\p{L}\-]+(?:[ \t]+[A-Z][\p{L}\-]+){0,2}`

// urlTLDs are the top-level domains a bare host without a path is
// recognised by: the v1 list plus the hosts of common contact and support
// links (saweria.co, linktr.ee, discord.gg, twitch.tv, youtu.be, beacons.ai,
// goo.gl, forms.gle, carrd.co ...). A host followed by a path is a URL
// whatever its top-level domain (see urlRes).
const urlTLDs = `com|net|org|co\.id|ac\.id|go\.id|or\.id|my\.id|web\.id|sch\.id|biz\.id|id|me|io|ly|link|app|dev|xyz|info|biz|` +
	`co|ee|gg|tv|to|cc|site|online|store|page|be|ai|gl|gle|bio|so|sh|lol|live|social|space|art|blog|fm|im`

// urlPath is the path part of a URL (trailing punctuation is trimmed by
// the caller).
const urlPath = `[^\s<>"'` + "`" + `]`

var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,}`)
	// urlRes: a scheme or "www."; any host.tld followed by a path, where
	// the top-level domain has at least two characters and starts with a
	// letter (so "7.5/10" or "kg.m/s" is not a URL); a bare host with a
	// known top-level domain.
	urlRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:https?://|www\.)` + urlPath + `+`),
		regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9\-]*(?:\.[a-z0-9\-]+)*\.[a-z][a-z0-9\-]*[a-z0-9]/` + urlPath + `+`),
		regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9\-]*(?:\.[a-z0-9\-]+)*\.(?:` + urlTLDs + `)\b(?:/` + urlPath + `*)?`),
	}
	// NikRes match a NIK (16 digits) written contiguously or grouped with
	// spaces, dots or hyphens: 4-4-4-4, 6-6-4 and the KTP form 2-2-2-6-4.
	// Digit boundaries are checked by the caller.
	NikRes = []*regexp.Regexp{
		regexp.MustCompile(`\d{4}[ .\-]?\d{4}[ .\-]?\d{4}[ .\-]?\d{4}`),
		regexp.MustCompile(`\d{6}[ .\-]?\d{6}[ .\-]?\d{4}`),
		regexp.MustCompile(`\d{2}[ .\-]?\d{2}[ .\-]?\d{2}[ .\-]?\d{6}[ .\-]?\d{4}`),
	}
	cardRe = regexp.MustCompile(`\d(?:[ \-]?\d){12,18}`)
	// phoneRunRe is a phone-number run: a prefix ("+", or "0" or "62" at
	// the start of a word) and then digits with any whitespace, dots
	// (a View maps the middle dots "·", "•", "∙", "・" to '.'), hyphens,
	// slashes, underscores and parentheses between them ("0812 - 3456 -
	// 7890", "0812  3456  7890", "+62 (812) 3456-7890", "0 8 1 2 3 4 5 6",
	// "0812·3456·7890", "0812/3456/7890", "0812_3456_7890"). See
	// FindPhones for the digit count and the date rule.
	phoneRunRe = regexp.MustCompile(`(?:\+|\b0|\b62)[\d\s().\-/_]*\d`)
	handleRe   = regexp.MustCompile(`(?:^|[^\w.@])(@[A-Za-z0-9_](?:[A-Za-z0-9_.]{0,28}[A-Za-z0-9_])?)`)
	// CueHandleRe matches a handle named by a platform cue without "@":
	// "ig aku raka.putra", "line: raka99", "tiktoknya @raka". Group 1 is
	// the separator, group 2 an "@", group 3 the identifier; the caller
	// accepts it when the separator is ":" or "=", there is an "@", or the
	// identifier has a dot, an underscore or a digit (so "line ini" or
	// "3 x sehari" is not a handle).
	CueHandleRe = regexp.MustCompile(`(?i)\b(?:ig|insta|instagram|tiktok|twitter|x|telegram|tele|tg|line|discord|fb|facebook|threads|snapchat|snap|youtube|twitch)(?:nya|ku|mu)?\b(?:[ \t]+(?:aku|ku|gue|gua|gw|saya|ane|dia|kamu|lo|lu|aja|ya|tuh|is|me|my|account|akun|username|id))*([ \t]*[:=][ \t]*|[ \t]+)(@?)([A-Za-z0-9_][A-Za-z0-9_.]{1,28}[A-Za-z0-9_](?:#\d{4})?)`)
	// AddressRes are v1's address_hint patterns, with the street cue in any
	// case: "Jl. Melati No. 12" and "jalan melati no 12" (a street cue and
	// a house number, any case), "Jl. Melati Raya" and "Gg. Mawar 3" (an
	// abbreviated cue and capitalised words), "Jalan Merdeka No. 5" (the
	// full cue and capitalised words: a lowercase "jalan" alone is the
	// ordinary word "walk" or "road"), "RT 03/RW 05", "Kel. Cempaka Putih",
	// "Kec./Kab./Desa X".
	AddressRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:jl|jln|jalan|gg|gang)\.?\s+[^\n,;]{1,50}?\s*\bno\.?\s*\d+[a-z]?\b`),
		regexp.MustCompile(`\b(?i:jl|jln|gg)\.?\s+[A-Z0-9][\p{L}\d.\-]*(?:[ \t]+[A-Z0-9][\p{L}\d.\-]*){0,3}`),
		regexp.MustCompile(`\b(?i:jalan)\s+[A-Z][\p{L}.\-]*(?:[ \t]+[A-Z0-9][\p{L}\d.\-]*){0,3}(?:\s*\b(?i:no)\.?\s*\d+[a-z]?\b)?`),
		regexp.MustCompile(`(?i)\bRT\s*\.?\s*/?\s*RW\s*\.?\s*\d{1,3}\s*/\s*\d{1,3}\b|\bRT\s*\.?\s*\d{1,3}\s*/\s*RW\s*\.?\s*\d{1,3}\b`),
		regexp.MustCompile(`\b(?i:kel(?:urahan)?|kec(?:amatan)?|kab(?:upaten)?|desa)\.?\s+` + nameRe),
	}
	// nameHintRes are v1's person_name_hint patterns (group 1 is the
	// name), only after an explicit self-introduction cue, plus their
	// English forms and the informal "dipanggil Raka", "manggil aku Raka".
	nameHintRes = []*regexp.Regexp{
		regexp.MustCompile(`\b(?:[Nn]ama(?:ku|[ \t]+(?:aku|saya|gue|gw))(?:[ \t]+adalah)?|[PpMm]anggil[ \t]+(?:aku|saya|gue|gw)|[Dd]ipanggil)[ \t]+(` + nameRe + `)`),
		regexp.MustCompile(`\b(?:[Aa]ku|[Ss]aya)[ \t]+(` + nameRe + `)[ \t]+dari\b`),
		regexp.MustCompile(`\b(?:[Mm]y[ \t]+name[ \t]+is|[Cc]all[ \t]+me)[ \t]+(` + nameRe + `)`),
	}
	// placeholderRe matches placeholders this package produced, so text
	// that already went through Redact is left alone; group 1 is the kind
	// and group 2 the number.
	placeholderRe = regexp.MustCompile(PlaceholderPattern)
)

// PlaceholderPattern matches a placeholder this package can produce:
// "[NAMA-n]" or "[KONTAK-n]" with n from 1 to 9999 (no request redacts
// more values). A placeholder-shaped token with a longer number
// ("[KONTAK-6281234567890]") is not a placeholder: it is neither kept
// verbatim here nor removed by the output guard, so the contact matchers
// see its digits. The output guard compiles the same pattern
// case-insensitively.
const PlaceholderPattern = `\[(KONTAK|NAMA)-([1-9]\d{0,3})\]`

// Phone digit counts (v1: 8 to 15 digits).
const (
	phoneMinDigits = 8
	phoneMaxDigits = 15
)

// FindPhones returns the byte ranges of phone numbers in text: runs of
// phoneRunRe with 8 to 15 digits. Leading digit groups shaped like a date
// (dateAt: "08/10/2026", "05.11.2026", "05-11-2026", "05/11",
// "01/02/2026 10/11/2026") are not part of a number: they are skipped and
// the rest of the run is read as numbers again, so "05/11 08123456789" and
// "05.11.2026 0812 3456 789" keep the date and match the whole number. A run that opens with "+" or "62" is an
// international number and never holds a date ("+62 812/3456/7890").
// Slashes otherwise join a number ("021/7654321", "0812/3456/7890").
// Whitespace may surround a separator
// ("0812 - 3456 - 7890", "WA 0812 . 3456 . 7890"). A run with more digits
// (two numbers side by side) is cut at the gaps between digit groups into
// consecutive numbers of at most 15 digits (a rest too short to be a
// number goes with the number before it), and one-digit groups that
// follow a number ("0812 3456 7890 2 kali") are left out while 8 digits
// remain; a number spelled digit by digit keeps every digit. The output
// guard (CON-1) uses this matcher on the same normalised view, so a form
// one of them recognises the other recognises too.
func FindPhones(text string) [][2]int {
	var out [][2]int
	for _, loc := range phoneRunRe.FindAllStringIndex(text, -1) {
		var groups [][2]int
		for i := loc[0]; i < loc[1]; {
			if !isDigitAt(text, i) {
				i++
				continue
			}
			j := i
			for j < loc[1] && isDigitAt(text, j) {
				j++
			}
			groups = append(groups, [2]int{i, j})
			i = j
		}
		width := func(g [2]int) int { return g[1] - g[0] }
		intl := text[loc[0]] == '+' || strings.HasPrefix(text[loc[0]:], "62")
		prev, prevEnd := -1, -1 // the last number of this run (index in out) and the group after it
		for i := 0; i < len(groups); {
			if !intl {
				if k := dateAt(text, groups, i); k > 0 {
					i += k // a date, never part of a number
					continue
				}
			}
			j, n := i, 0
			for j < len(groups) && n+width(groups[j]) <= phoneMaxDigits {
				n += width(groups[j])
				j++
			}
			if j == i {
				i++ // one group of more than 15 digits: not a phone number
				continue
			}
			// Leave out one-digit groups after the last longer group
			// while the number keeps 8 digits.
			last := -1
			for k := i; k < j; k++ {
				if width(groups[k]) > 1 {
					last = k
				}
			}
			if last >= 0 && last < j-1 {
				kept := 0
				for k := i; k <= last; k++ {
					kept += width(groups[k])
				}
				if kept >= phoneMinDigits {
					j, n = last+1, kept
				}
			}
			if n >= phoneMinDigits {
				start := groups[i][0]
				if i == 0 {
					start = loc[0] // keep the "+" of the prefix
				}
				out = append(out, [2]int{start, groups[j-1][1]})
				prev, prevEnd = len(out)-1, j
			} else if prev >= 0 && prevEnd == i {
				// The rest of a run cut at 15 digits right after a number
				// is too short to be a number: it goes with that number
				// (one-digit groups excepted), so no part of it is left in
				// clear.
				for k := j - 1; k >= i; k-- {
					if width(groups[k]) > 1 {
						out[prev][1] = groups[k][1]
						break
					}
				}
			}
			i = j
		}
	}
	return out
}

// dateAt returns how many digit groups from groups[i] form a date, or 0:
// a day and a month (one or two digits each, 1 to 31, one of them at most
// 12) and an optional year of two digits or of four starting 19 or 20,
// joined by one separator used throughout the date, "/", "." or "-"
// (whitespace may surround it), with no such separator directly before or
// after the date; or a day, a month and a four-digit year starting 19 or
// 20 separated by whitespace alone ("05 11 2026"). So "05/11",
// "08/10/2026", "1/2/26", "05.11.2026" and "05-11-2026" are dates, while
// "08/12/3456/7890", "08.12.3456.7890", "02/1234567" and "021/7654321" are
// not.
func dateAt(text string, groups [][2]int, i int) int {
	sep := func(k int) string { // the separator before groups[k]
		if k <= 0 || k >= len(groups) {
			return ""
		}
		return text[groups[k-1][1]:groups[k][0]]
	}
	digits := func(k int) string { return text[groups[k][0]:groups[k][1]] }
	dayOrMonth := func(k int) int {
		d := digits(k)
		if len(d) > 2 {
			return 0
		}
		v, _ := strconv.Atoi(d)
		if v > 31 {
			return 0
		}
		return v
	}
	longYear := func(y string) bool { return len(y) == 4 && (strings.HasPrefix(y, "19") || strings.HasPrefix(y, "20")) }
	if i+1 >= len(groups) {
		return 0
	}
	a, b := dayOrMonth(i), dayOrMonth(i+1)
	if a == 0 || b == 0 || (a > 12 && b > 12) {
		return 0
	}
	c := strings.TrimSpace(sep(i + 1))
	if c == "" {
		// "05 11 2026": whitespace alone, so the four-digit year is needed.
		if i+2 < len(groups) && strings.TrimSpace(sep(i+2)) == "" && longYear(digits(i+2)) &&
			strings.TrimSpace(sep(i)) == "" && strings.TrimSpace(sep(i+3)) == "" {
			return 3
		}
		return 0
	}
	if c != "/" && c != "." && c != "-" || strings.Contains(sep(i), c) {
		return 0
	}
	n := 2
	if i+2 < len(groups) && strings.TrimSpace(sep(i+2)) == c {
		if y := digits(i + 2); len(y) == 2 || longYear(y) {
			n = 3
		}
	}
	if strings.Contains(sep(i+n), c) {
		return 0 // more groups with the separator follow: a number, not a date
	}
	return n
}

// honorifics are address terms that may open a self-introduced name
// ("panggil aku Kak Raka"); they are never learned as the given name.
var honorifics = map[string]bool{"kak": true, "ka": true, "kakak": true, "mas": true, "mbak": true, "mba": true, "bu": true,
	"ibu": true, "pak": true, "bapak": true, "bang": true, "abang": true, "dik": true, "adik": true, "dek": true, "om": true,
	"tante": true, "teh": true, "aa": true, "cik": true, "koh": true, "ci": true, "bro": true, "sis": true, "mr": true,
	"mrs": true, "ms": true, "miss": true, "dr": true, "prof": true, "sir": true, "nona": true, "tuan": true, "nyonya": true}

// FindURLs returns the byte ranges of URLs in text (see urlRes), without
// trailing punctuation. The output guard (CON-1) uses the same rules.
func FindURLs(text string) [][2]int {
	var out [][2]int
	for _, re := range urlRes {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			end := loc[1]
			for end > loc[0] && strings.ContainsRune(".,;:!?)]}", rune(text[end-1])) {
				end--
			}
			if end > loc[0] {
				out = append(out, [2]int{loc[0], end})
			}
		}
	}
	return out
}

// priority: on overlap the earlier kind wins. Name kinds share a rank, so
// the longer of two overlapping names wins ("Raka Putra" over "Raka").
var priority = map[Kind]int{KindEmail: 0, KindURL: 1, KindNIK: 2, KindCard: 3, KindPhone: 4, KindHandle: 5, KindAddress: 6,
	KindName: 7, KindNameHint: 7}

// kindPlaceholder marks a placeholder already in the text.
const kindPlaceholder Kind = "placeholder"

// finding is one match. In findContacts, start and end are offsets into
// the view; Redact maps them to the original text. norm is the normalised
// match and key, when set, the placeholder key of a listed name.
type finding struct {
	kind       Kind
	start, end int
	norm, key  string
	// value, when set, is the restore value (a learned name's full name).
	value string
}

func digitsOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func luhn(d string) bool {
	sum, double := 0, false
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}

func isDigitAt(s string, i int) bool { return i >= 0 && i < len(s) && s[i] >= '0' && s[i] <= '9' }

// FindNIK returns the byte ranges of NIK-shaped numbers in text (see
// NikRes) that have no digit directly before or after them.
func FindNIK(text string) [][2]int {
	var out [][2]int
	for _, re := range NikRes {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			if !isDigitAt(text, loc[0]-1) && !isDigitAt(text, loc[1]) && len(digitsOf(text[loc[0]:loc[1]])) == 16 {
				out = append(out, [2]int{loc[0], loc[1]})
			}
		}
	}
	return out
}

// FindCueHandles returns the byte ranges of the identifiers that a
// platform cue names (see CueHandleRe).
func FindCueHandles(text string) [][2]int {
	var out [][2]int
	for _, m := range CueHandleRe.FindAllStringSubmatchIndex(text, -1) {
		sep, at, id := text[m[2]:m[3]], m[5] > m[4], text[m[6]:m[7]]
		if strings.ContainsAny(sep, ":=") || at || strings.ContainsAny(id, "._#0123456789") {
			start := m[6]
			if at {
				start = m[4]
			}
			out = append(out, [2]int{start, m[7]})
		}
	}
	return out
}

// FindAddresses returns the byte ranges of address hints in text (see
// AddressRes).
func FindAddresses(text string) [][2]int {
	var out [][2]int
	for _, re := range AddressRes {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			out = append(out, [2]int{loc[0], loc[1]})
		}
	}
	return out
}

// findContacts lists contact-detail candidates in the normalised text.
func findContacts(text string) []finding {
	var out []finding
	for _, loc := range emailRe.FindAllStringIndex(text, -1) {
		out = append(out, finding{kind: KindEmail, start: loc[0], end: loc[1]})
	}
	for _, loc := range FindURLs(text) {
		out = append(out, finding{kind: KindURL, start: loc[0], end: loc[1]})
	}
	for _, loc := range FindNIK(text) {
		out = append(out, finding{kind: KindNIK, start: loc[0], end: loc[1]})
	}
	for _, loc := range cardRe.FindAllStringIndex(text, -1) {
		d := digitsOf(text[loc[0]:loc[1]])
		if len(d) >= 13 && len(d) <= 19 && luhn(d) && !isDigitAt(text, loc[0]-1) && !isDigitAt(text, loc[1]) {
			out = append(out, finding{kind: KindCard, start: loc[0], end: loc[1]})
		}
	}
	for _, loc := range FindPhones(text) {
		out = append(out, finding{kind: KindPhone, start: loc[0], end: loc[1]})
	}
	for _, m := range handleRe.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, finding{kind: KindHandle, start: m[2], end: m[3]})
	}
	for _, loc := range FindCueHandles(text) {
		out = append(out, finding{kind: KindHandle, start: loc[0], end: loc[1]})
	}
	for _, loc := range FindAddresses(text) {
		out = append(out, finding{kind: KindAddress, start: loc[0], end: loc[1]})
	}
	for _, re := range nameHintRes {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			if !onlyHonorifics(text[m[2]:m[3]]) {
				out = append(out, finding{kind: KindNameHint, start: m[2], end: m[3]})
			}
		}
	}
	return out
}

// nameEntry is one name the session redacts: a name the user marked
// private, or one learned from a self-introduction (and its given name).
type nameEntry struct {
	name string
	// key is the placeholder key: a learned given name shares the key of
	// the full name, so both get one placeholder.
	key  string
	kind Kind
	// value, for a learned name, is the full name as introduced (the
	// original text), which the owner's view restores the placeholder to.
	value string
	// common marks a part of a private name that is also an everyday word
	// ("Kasih" of "Kasih Ramadhani"): it is matched by FindCommonPart.
	common bool
}

// nameKey is the placeholder key of a name: "Ｄｅｗｉ" and "Dewi" are one
// person.
func nameKey(name string) string {
	return "name\x00" + textnorm.Fold(strings.Join(strings.Fields(name), " "))
}

// Session redacts the texts of one request with stable placeholders: the
// same value gets the same placeholder in every span of the request.
// It is not safe for concurrent use.
type Session struct {
	names []nameEntry
	// listed maps every listed name, folded, to its entry, so a
	// self-introduction cue that repeats a listed name ("manggil aku
	// Raka" after "namaku Raka Putra") gets the listed name's placeholder.
	listed   map[string]nameEntry
	byValue  map[string]string
	restore  map[string]string
	contacts int
	people   int
	counts   map[Kind]int
}

// NewSession returns a session for one request. privateNames are the
// names the user marked private; longer names are matched first. Each word
// of a private name of two or more words is listed too ("Budi Santoso":
// "Budi" and "Santoso" alone), with the full name's placeholder and
// restore value, so a part that appears alone is redacted and blocked by
// the output guard like the full name. Address terms ("Pak", "Kak") and
// name particles ("bin", "van") are not parts, and a part that two private
// names share gets a placeholder of its own (it names neither for sure).
// A part that is also an everyday word ("Kasih", "Cinta", "Indah", see
// commonParts) is matched alone only where FindCommonPart finds it, so
// "terima kasih" is neither redacted nor blocked while "Kasih bilang" is;
// the full name is matched in any case.
func NewSession(privateNames []string) *Session {
	s := &Session{listed: map[string]nameEntry{}, byValue: map[string]string{}, restore: map[string]string{}, counts: map[Kind]int{}}
	for _, n := range privateNames {
		s.addName(nameEntry{name: n, key: nameKey(n), kind: KindName})
	}
	owners := map[string]map[string]bool{} // folded part -> keys of the names it is part of
	for _, n := range privateNames {
		for _, part := range nameParts(n) {
			f := textnorm.Fold(part)
			if owners[f] == nil {
				owners[f] = map[string]bool{}
			}
			owners[f][nameKey(n)] = true
		}
	}
	for _, n := range privateNames {
		for _, part := range nameParts(n) {
			e := nameEntry{name: part, key: nameKey(n), kind: KindName, value: strings.Join(strings.Fields(n), " "),
				common: isCommonPart(part)}
			if len(owners[textnorm.Fold(part)]) > 1 {
				e.key, e.value = nameKey(part), ""
			}
			s.addName(e) // a part that is itself a listed name keeps that entry
		}
	}
	return s
}

// commonParts are words of Indonesian given names that are also everyday
// words (and so appear in ordinary text and in replies: "terima kasih",
// "aku cinta kamu", "pemandangannya indah"). As a standalone part of a
// private name they are matched by FindCommonPart only. The value is true
// where the everyday meaning dominates ("kasih", "sayang", "indah",
// "bagus", "sabar"): such a part names the person only capitalised. The
// others are rarely an everyday word in chat ("bayu", "agung", "intan",
// "lestari", "fajar"): they are matched in any case outside a fixed phrase,
// so "agung blm bales" is the name.
var commonParts = map[string]bool{
	// The everyday meaning dominates: capitalised only.
	"kasih": true, "cinta": true, "sayang": true, "indah": true, "bunga": true, "bintang": true, "cahaya": true,
	"senja": true, "embun": true, "mentari": true, "pelangi": true, "suci": true, "murni": true, "setia": true,
	"sabar": true, "damai": true, "bahagia": true, "harapan": true, "berkah": true, "mulia": true, "bagus": true,
	"langit": true, "manis": true, "cantik": true, "rezeki": true, "hidayah": true,
	// Rarely an everyday word: any case.
	"putri": false, "putra": false, "ayu": false, "fajar": false, "intan": false, "mutiara": false, "permata": false,
	"melati": false, "mawar": false, "rahmat": false, "teguh": false, "jaya": false, "agung": false, "bayu": false,
	"surya": false, "ratu": false, "raja": false, "lestari": false}

// isCommonPart reports whether a word of a private name is also an
// everyday word (commonParts).
func isCommonPart(part string) bool {
	_, ok := commonParts[textnorm.Fold(part)]
	return ok
}

// fixedPhrases are everyday phrases that hold a common part as an ordinary
// word ("Terima kasih", "Kasih sayang itu penting"); see FindCommonPart for
// where a common part inside one is still the name.
var fixedPhrases = []string{"terima kasih", "trima kasih", "kasih sayang", "kasih tahu", "kasih tau", "maha pengasih",
	"jatuh cinta", "cinta kasih", "rasa cinta", "sayang sekali", "sayang banget", "indah sekali", "indah banget",
	"bunga tidur", "bintang jatuh", "putri saya", "putra saya", "putri kami", "putra kami", "tuan putri", "cahaya matahari",
	"fajar menyingsing", "harapan baru", "rahmat tuhan", "rahmat allah", "bagus sekali", "bagus banget", "bagus kok",
	"bagus ya", "maha agung", "masjid agung", "mahkamah agung", "jaksa agung", "tetap sabar", "sabar ya", "sabar dulu",
	"rasa damai", "rasa bahagia", "sehat selalu", "tetap teguh", "teguh pendirian", "tenaga surya", "panel surya",
	"tata surya", "bunga mawar", "bunga melati", "permata hati"}

// FindCommonPart returns the spans of part, a word of a private name that
// is also an everyday word (commonParts), in text (a name view, as for
// textnorm.FindName) where it names the person. A lowercase occurrence
// counts only for a part whose everyday meaning is rare ("bayu", "agung")
// and outside a fixed phrase ("masjid agung"). A capitalised occurrence
// ("Kasih bilang", "KASIH") counts unless it is inside a fixed phrase and
// is the everyday word there: it opens a sentence ("Indah banget!",
// "Kasih sayang itu penting") or follows a capitalised word of the phrase
// ("Terima Kasih"). After a lowercase word in the middle of a sentence it
// is the name ("aku kangen Indah banget", "cerita ke Bagus ya"). The
// redactor and the output guard (NAME-1) both use it, so a form one of
// them matches the other matches too.
func FindCommonPart(text, part string) []textnorm.Span {
	f := textnorm.Fold(part)
	capOnly := commonParts[f]
	var phrases []textnorm.Span
	for _, ph := range fixedPhrases {
		for _, w := range strings.Fields(ph) {
			if w == f {
				phrases = append(phrases, textnorm.FindName(text, ph)...)
				break
			}
		}
	}
	var out []textnorm.Span
	for _, sp := range textnorm.FindName(text, part) {
		upper := startsUpper(text[sp.Start:])
		if capOnly && !upper {
			continue
		}
		everyday := false
		for _, p := range phrases {
			if p.Start <= sp.Start && sp.End <= p.End {
				everyday = !upper || opensSentence(text, sp.Start) || (sp.Start > p.Start && startsUpper(text[prevWord(text, sp.Start):]))
				if everyday {
					break
				}
			}
		}
		if !everyday {
			out = append(out, sp)
		}
	}
	return out
}

func startsUpper(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsUpper(r)
}

// opensSentence reports whether the word at text[at:] opens a sentence:
// only spaces and opening quotes or brackets stand between it and the
// start of text, a line break or ".", "!", "?" or "…".
func opensSentence(text string, at int) bool {
	for at > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:at])
		switch {
		case r == '\n' || r == '\r' || r == '.' || r == '!' || r == '?' || r == '…':
			return true
		case unicode.IsSpace(r) || unicode.Is(unicode.Pi, r) || unicode.Is(unicode.Ps, r) || r == '"' || r == '\'' || r == '*' || r == '_':
			at -= size
		default:
			return false
		}
	}
	return true
}

// prevWord returns the start of the word before the one at text[at:]
// (at itself when there is none).
func prevWord(text string, at int) int {
	i := at
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:i])
		if !unicode.IsSpace(r) {
			break
		}
		i -= size
	}
	if i == at {
		return at
	}
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:i])
		if !textnorm.WordRune(r) && r != '\'' {
			break
		}
		i -= size
	}
	return i
}

// nameParticles are words inside a name that are not a name on their own.
var nameParticles = map[string]bool{"bin": true, "binti": true, "bt": true, "ibn": true, "ibnu": true, "al": true, "el": true,
	"van": true, "von": true, "der": true, "den": true, "de": true, "del": true, "della": true, "di": true, "da": true,
	"dos": true, "du": true, "la": true, "le": true, "y": true}

// nameParts returns the words of a name of two or more words that may
// name the person alone: not an address term or a particle, at least two
// letters (addName's minimum).
func nameParts(name string) []string {
	toks := strings.Fields(name)
	if len(toks) < 2 {
		return nil
	}
	var out []string
	for _, tok := range toks {
		tok = strings.Trim(tok, ".,-")
		if honorific(tok) || nameParticles[textnorm.Fold(tok)] || utf8.RuneCountInString(tok) < 2 {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// addName lists a name (once; a name shorter than two letters is never
// matched) and keeps the list longest first.
func (s *Session) addName(e nameEntry) {
	f := textnorm.Fold(strings.Join(strings.Fields(e.name), " "))
	if _, ok := s.listed[f]; ok || utf8.RuneCountInString(strings.ReplaceAll(f, " ", "")) < 2 {
		return
	}
	s.listed[f] = e
	s.names = append(s.names, e)
	sort.SliceStable(s.names, func(i, j int) bool { return len(s.names[i].name) > len(s.names[j].name) })
}

// givenName returns the first word of a self-introduced name of two or
// more words that is not an address term ("Raka Putra" -> "Raka",
// "Kak Raka" -> "Raka"), or "".
func givenName(name string) string {
	toks := strings.Fields(name)
	if len(toks) < 2 {
		return ""
	}
	for _, tok := range toks {
		if !honorific(tok) {
			return tok
		}
	}
	return ""
}

func honorific(tok string) bool { return honorifics[textnorm.Fold(strings.Trim(tok, ".-"))] }

// onlyHonorifics reports whether every word of name is an address term.
func onlyHonorifics(name string) bool {
	for _, tok := range strings.Fields(name) {
		if !honorific(tok) {
			return false
		}
	}
	return true
}

// learn prepares the session for text before anything is redacted: a
// placeholder already in it moves the session's counter past its number
// (so a new value never reuses it), and a name it introduces ("namaku
// Raka Putra", "dipanggil Raka", "my name is Dewi Lestari") is listed,
// with its given name, so every mention in this and every later text of
// the request is redacted with one placeholder.
func (s *Session) learn(text string) {
	v := textnorm.NewView(text)
	for _, m := range placeholderRe.FindAllStringSubmatch(v.Text, -1) {
		n, err := strconv.Atoi(m[2])
		if err != nil || n > 1<<20 {
			continue
		}
		if m[1] == "NAMA" {
			s.people = max(s.people, n)
		} else {
			s.contacts = max(s.contacts, n)
		}
	}
	for _, re := range nameHintRes {
		for _, m := range re.FindAllStringSubmatchIndex(v.Text, -1) {
			name := v.Text[m[2]:m[3]]
			if onlyHonorifics(name) {
				continue // "panggil aku Kak aja": an address term, not a name
			}
			start, end := v.Orig(m[2], m[3])
			e := nameEntry{name: name, key: nameKey(name), kind: KindNameHint, value: text[start:end]}
			s.uncommon(name)
			s.addName(e)
			if g := givenName(name); g != "" {
				e.name = g
				s.uncommon(g)
				s.addName(e)
			}
		}
	}
}

// uncommon makes a listed common part that the user introduced
// themselves with ("namaku Kasih" when "Kasih Ramadhani" is private) a
// name matched in any case, as it is for a session without private names;
// the entry keeps its key and value (the private name's placeholder).
func (s *Session) uncommon(name string) {
	f := textnorm.Fold(strings.Join(strings.Fields(name), " "))
	e, ok := s.listed[f]
	if !ok || !e.common {
		return
	}
	e.common = false
	s.listed[f] = e
	for i := range s.names {
		if textnorm.Fold(strings.Join(strings.Fields(s.names[i].name), " ")) == f {
			s.names[i].common = false
		}
	}
}

// Names returns every name the session redacts wherever it appears: the
// private names, their parts and the self-introduced names (and given
// names) learned so far, without the common-word parts (CommonNameParts).
// The output guard blocks them (NAME-1).
func (s *Session) Names() []string {
	out := make([]string, 0, len(s.names))
	for _, e := range s.names {
		if !e.common {
			out = append(out, e.name)
		}
	}
	return out
}

// CommonNameParts returns the parts of private names that are also
// everyday words; the session redacts them, and the output guard blocks
// them (NAME-1), only where FindCommonPart finds them.
func (s *Session) CommonNameParts() []string {
	var out []string
	for _, e := range s.names {
		if e.common {
			out = append(out, e.name)
		}
	}
	return out
}

// Redact replaces contact details and names in text. Contacts are matched
// on the normalised view, names on the view that keeps apostrophes; the
// original bytes of every match are replaced.
func (s *Session) Redact(text string) string {
	if text == "" {
		return text
	}
	s.learn(text)
	v, nv := textnorm.NewView(text), textnorm.NewNameView(text)
	var found []finding
	add := func(view textnorm.View, f finding) {
		if f.end <= f.start {
			return
		}
		f.norm = view.Text[f.start:f.end]
		f.start, f.end = view.Orig(f.start, f.end)
		if f.end > f.start {
			found = append(found, f)
		}
	}
	for _, f := range findContacts(v.Text) {
		add(v, f)
	}
	for _, e := range s.names {
		find := textnorm.FindName
		if e.common {
			find = FindCommonPart
		}
		for _, sp := range find(nv.Text, e.name) {
			add(nv, finding{kind: e.kind, start: sp.Start, end: sp.End, key: e.key, value: e.value})
		}
	}
	for _, loc := range placeholderRe.FindAllStringIndex(v.Text, -1) {
		add(v, finding{kind: kindPlaceholder, start: loc[0], end: loc[1]})
	}
	sort.SliceStable(found, func(i, j int) bool {
		pi, pj := rank(found[i].kind), rank(found[j].kind)
		if pi != pj {
			return pi < pj
		}
		if found[i].start != found[j].start {
			return found[i].start < found[j].start
		}
		return found[i].end > found[j].end
	})
	accepted := resolveOverlaps(text, found)
	var b strings.Builder
	last := 0
	for _, f := range accepted {
		if f.start < last {
			continue // cannot happen: accepted ranges do not overlap
		}
		b.WriteString(text[last:f.start])
		if f.kind == kindPlaceholder {
			b.WriteString(text[f.start:f.end])
		} else {
			b.WriteString(s.placeholder(f, text[f.start:f.end]))
		}
		last = f.end
	}
	b.WriteString(text[last:])
	return b.String()
}

func rank(k Kind) int {
	if p, ok := priority[k]; ok {
		return p
	}
	return -1 // existing placeholders win
}

// resolveOverlaps returns disjoint findings, sorted by start, that cover
// every byte of every finding in found (sorted by rank, then start, longer
// first). A finding inside the ones already accepted adds nothing. A
// finding that overlaps accepted ones and reaches past them is never
// dropped, because its uncovered part would be sent in clear (a cue handle
// or an @mention that covers only "Dewi" of the private name "Dewi
// Lestari"): the overlapping findings are merged into one range. When the
// new finding covers all of them it keeps its own kind and key (a private
// name around a handle stays that name's placeholder); otherwise the
// merged range takes the highest-priority kind among them with a fresh key,
// so the owner's view restores the merged text exactly.
func resolveOverlaps(text string, found []finding) []finding {
	var accepted []finding
	for _, f := range found {
		if f.end <= f.start {
			continue
		}
		var over, rest []finding
		for _, a := range accepted {
			if f.start < a.end && a.start < f.end {
				over = append(over, a)
			} else {
				rest = append(rest, a)
			}
		}
		if len(over) == 0 {
			accepted = append(accepted, f)
			continue
		}
		sort.Slice(over, func(i, j int) bool { return over[i].start < over[j].start })
		if inside(f, over) {
			continue
		}
		lo, hi := f.start, f.end
		for _, a := range over {
			lo, hi = min(lo, a.start), max(hi, a.end)
		}
		m := f
		if lo != f.start || hi != f.end {
			m = finding{kind: f.kind, start: lo, end: hi, norm: textnorm.NewView(text[lo:hi]).Text}
			for _, a := range over {
				if a.kind != kindPlaceholder && rank(a.kind) < rank(m.kind) {
					m.kind = a.kind
				}
			}
		}
		accepted = append(rest, m)
	}
	sort.Slice(accepted, func(i, j int) bool { return accepted[i].start < accepted[j].start })
	return accepted
}

// inside reports whether f lies within the union of spans (disjoint,
// sorted by start).
func inside(f finding, spans []finding) bool {
	pos := f.start
	for _, a := range spans {
		if a.start > pos {
			return false
		}
		pos = max(pos, a.end)
		if pos >= f.end {
			return true
		}
	}
	return pos >= f.end
}

// placeholder returns the placeholder of a match. Its key is the listed
// name's key, else the normalised match (the stable key: "Ｄｅｗｉ" and
// "Dewi" are one person); value is the original text (kept for the
// owner's view).
func (s *Session) placeholder(f finding, value string) string {
	k := f.kind
	name := k == KindName || k == KindNameHint
	key := f.key
	switch {
	case key != "":
	case name:
		key = nameKey(f.norm)
		if e, ok := s.listed[strings.TrimPrefix(key, "name\x00")]; ok {
			key = e.key
			if f.value == "" {
				f.value = e.value
			}
		}
	default:
		key = "contact\x00" + strings.ToLower(strings.Join(strings.Fields(f.norm), " "))
	}
	if p, ok := s.byValue[key]; ok {
		return p
	}
	var p string
	if name {
		s.people++
		p = "[NAMA-" + strconv.Itoa(s.people) + "]"
	} else {
		s.contacts++
		p = "[KONTAK-" + strconv.Itoa(s.contacts) + "]"
	}
	if f.value != "" {
		value = f.value // a learned name restores to the full name as introduced
	}
	s.byValue[key] = p
	s.restore[p] = value
	s.counts[k]++
	return p
}

// Restore returns placeholder -> original value, for the owner's own view
// only. It must never be sent to a provider or another recipient, logged
// or put in provenance.
func (s *Session) Restore() map[string]string {
	out := make(map[string]string, len(s.restore))
	for k, v := range s.restore {
		out[k] = v
	}
	return out
}

// Counts returns how many distinct values of each kind were redacted
// (metadata only).
func (s *Session) Counts() map[Kind]int {
	out := make(map[Kind]int, len(s.counts))
	for k, v := range s.counts {
		out[k] = v
	}
	return out
}

// Role says what a span is. Roles are sent; they carry no identity.
type Role string

// Span roles (ADR-0003 §3.1).
const (
	RoleUser         Role = "user"          // the user's own words
	RoleEarlierReply Role = "earlier_reply" // an earlier Sela reply: unverified, data
	RoleMemory       Role = "memory"        // a confirmed memory item
	RoleItem         Role = "item"          // a selected summary item's quote
	RoleSource       Role = "source"        // a licensed source passage (AT-6)
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleUser, RoleEarlierReply, RoleMemory, RoleItem, RoleSource:
		return true
	}
	return false
}

// UserAuthored reports whether a span holds the user's own words (these are
// screened for crisis language before any call).
func (r Role) UserAuthored() bool { return r == RoleUser || r == RoleMemory || r == RoleItem }

// Selection is one span the caller selected explicitly. Key is the
// caller's own reference (for example a database id); it stays in Go.
type Selection struct {
	Key  string
	Role Role
	Text string
}

// Span is one span as sent: a per-request id, a role and redacted text.
// There is no field for the caller's key.
type Span struct {
	ID   string `json:"id"`
	Role Role   `json:"role"`
	Text string `json:"text"`
}

// Limits bound one payload.
type Limits struct {
	// MaxSpans is the largest number of spans sent.
	MaxSpans int
	// MaxSpanRunes bounds one span after redaction.
	MaxSpanRunes int
	// MaxTotalRunes bounds all spans together.
	MaxTotalRunes int
	// DropOldest: when the spans exceed MaxSpans or MaxTotalRunes, drop
	// the oldest (first) spans instead of failing, keeping at least the
	// last one (AT-1: "the oldest are dropped first").
	DropOldest bool
	// PerRole bounds the spans of one role, over the whole selection and
	// after redaction (ADR-0003 §3.1: AT-1 at most 5 memory items of 600
	// characters together, AT-3 at most 10 memory items). Exceeding it
	// fails with ErrRoleLimit; nothing is dropped silently.
	PerRole map[Role]RoleLimit
}

// RoleLimit bounds the spans of one role in a payload (zero: no bound).
type RoleLimit struct {
	MaxSpans int
	MaxRunes int
}

// Errors.
var (
	ErrNoSpans   = errors.New("minimize: no spans selected")
	ErrTooLarge  = errors.New("minimize: payload exceeds the size cap")
	ErrInvalid   = errors.New("minimize: invalid span")
	ErrRoleLimit = errors.New("minimize: more spans or characters of one role than the task allows")
)

// Payload is the minimised span list of one request plus the id mapping
// that stays in Go.
type Payload struct {
	Spans []Span
	// keys maps a per-request id to the caller's key.
	keys map[string]string
	// texts maps a per-request id to the redacted text (for quote
	// fallbacks).
	texts map[string]string
	// Dropped counts spans dropped under Limits.DropOldest.
	Dropped int
}

// Key returns the caller's key for a per-request span id.
func (p Payload) Key(id string) (string, bool) {
	k, ok := p.keys[id]
	return k, ok
}

// Text returns the redacted text sent for a span id.
func (p Payload) Text(id string) (string, bool) {
	t, ok := p.texts[id]
	return t, ok
}

// Has reports whether id is one of the payload's span ids.
func (p Payload) Has(id string) bool {
	_, ok := p.keys[id]
	return ok
}

// Build redacts the selected spans, assigns per-request ids and enforces
// the limits. Span text above MaxSpanRunes is never cut (cutting changes
// meaning): it fails with ErrTooLarge. Empty spans are invalid. Every span
// is read for self-introduced names and kept placeholders before any is
// redacted (see Session), so a name introduced in a later span is
// redacted in the earlier ones too.
func Build(sel []Selection, lim Limits, s *Session) (Payload, error) {
	if len(sel) == 0 {
		return Payload{}, ErrNoSpans
	}
	type item struct {
		role Role
		key  string
		text string
		n    int
	}
	texts := make([]string, len(sel))
	for i, x := range sel {
		if !x.Role.Valid() {
			return Payload{}, fmt.Errorf("%w: span %d has an unknown role", ErrInvalid, i)
		}
		texts[i] = strings.TrimSpace(x.Text)
		if texts[i] == "" || !utf8.ValidString(texts[i]) {
			return Payload{}, fmt.Errorf("%w: span %d is empty or not UTF-8", ErrInvalid, i)
		}
	}
	for _, t := range texts {
		s.learn(t)
	}
	items := make([]item, 0, len(sel))
	perRole := map[Role]RoleLimit{}
	for i, x := range sel {
		red := s.Redact(texts[i])
		n := utf8.RuneCountInString(red)
		if lim.MaxSpanRunes > 0 && n > lim.MaxSpanRunes {
			return Payload{}, fmt.Errorf("%w: span %d is above %d characters", ErrTooLarge, i, lim.MaxSpanRunes)
		}
		items = append(items, item{role: x.Role, key: x.Key, text: red, n: n})
		got := perRole[x.Role]
		got.MaxSpans++
		got.MaxRunes += n
		perRole[x.Role] = got
	}
	for role, l := range lim.PerRole {
		got := perRole[role]
		if (l.MaxSpans > 0 && got.MaxSpans > l.MaxSpans) || (l.MaxRunes > 0 && got.MaxRunes > l.MaxRunes) {
			return Payload{}, fmt.Errorf("%w: %d %s spans, %d characters", ErrRoleLimit, got.MaxSpans, role, got.MaxRunes)
		}
	}
	dropped := 0
	total := func() int {
		t := 0
		for _, it := range items {
			t += it.n
		}
		return t
	}
	for (lim.MaxSpans > 0 && len(items) > lim.MaxSpans) || (lim.MaxTotalRunes > 0 && total() > lim.MaxTotalRunes) {
		if !lim.DropOldest || len(items) == 1 {
			return Payload{}, fmt.Errorf("%w: %d spans, %d characters", ErrTooLarge, len(items), total())
		}
		items = items[1:]
		dropped++
	}
	p := Payload{Spans: make([]Span, 0, len(items)), keys: map[string]string{}, texts: map[string]string{}, Dropped: dropped}
	for i, it := range items {
		id := "s" + strconv.Itoa(i+1)
		p.Spans = append(p.Spans, Span{ID: id, Role: it.role, Text: it.text})
		p.keys[id] = it.key
		p.texts[id] = it.text
	}
	return p, nil
}
