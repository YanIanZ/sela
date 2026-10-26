package account

import (
	"bufio"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math/bits"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

// Username rules (ADR-0002 §6).
const (
	UsernameMinLength = 3
	UsernameMaxLength = 32
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// reservedUsernames may not be a username (after removing separators).
var reservedUsernames = map[string]bool{
	"admin": true, "administrator": true, "sela": true, "selaapp": true, "dengarin": true, "support": true, "root": true,
	"system": true, "guest": true, "tamu": true, "moderator": true, "mod": true, "official": true, "resmi": true, "help": true,
	"bantuan": true, "security": true, "keamanan": true, "staff": true, "team": true, "tim": true, "operator": true,
	"owner": true, "api": true, "www": true, "mail": true, "email": true, "null": true, "undefined": true, "anonymous": true,
	"anonim": true, "privacy": true, "privasi": true, "legal": true, "abuse": true, "info": true, "contact": true,
	"kontak": true, "psikolog": true, "psychologist": true, "doctor": true, "dokter": true, "counselor": true, "konselor": true,
}

// reservedParts may not appear even as one dot, underscore or hyphen
// separated part, because they would impersonate Sela or its staff
// ("sela.support", "admin_budi").
var reservedParts = map[string]bool{
	"sela": true, "selaapp": true, "dengarin": true, "admin": true, "administrator": true, "official": true, "resmi": true,
	"support": true, "moderator": true, "security": true, "staff": true, "operator": true, "root": true, "system": true,
}

// emailLike catches usernames shaped like an address or a domain at a
// mail provider, even without "@" (which the character set refuses).
var emailLike = regexp.MustCompile(`(gmail|googlemail|yahoo|ymail|hotmail|outlook|icloud|live|proton|protonmail)|\.(com|net|org|id|co|io|edu|gov|ac|sch)$`)

// NormalizeUsername trims, refuses non-ASCII (instead of folding look-alike
// characters), lower-cases and validates a username. The normalised form is
// what is stored, so uniqueness is case-insensitive.
func NormalizeUsername(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return "", ErrUsernameInvalid
		}
	}
	s = strings.ToLower(s)
	if !usernamePattern.MatchString(s) {
		return "", ErrUsernameInvalid
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '_' || r == '-' })
	if reservedUsernames[strings.Join(parts, "")] {
		return "", ErrUsernameInvalid
	}
	for _, p := range parts {
		if reservedParts[p] {
			return "", ErrUsernameInvalid
		}
	}
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	// Nine or more digits look like a phone or an ID number (NIK has 16).
	if digits >= 9 || emailLike.MatchString(s) || containsContactDetails(s) {
		return "", ErrUsernameInvalid
	}
	return s, nil
}

// ThrottleKeyPurpose is the HKDF info of the username-key secret
// (vault.DeriveKey). Rotating the KEK changes the secret, which only
// resets the login throttle.
const ThrottleKeyPurpose = "sela.login-throttle.v1"

// usernameKey is the rate-limit and backoff key of a typed username:
// HMAC-SHA256 under a server-held secret (never stored in the database)
// of "u:" and its normalised form, or, when the input is not a valid
// username, of "r:" and the trimmed lower-cased input (applied whether or
// not an account exists). Without the secret a dictionary cannot reverse
// a key. Only keys of valid usernames are ever persisted (login_throttle);
// the others stay in the in-memory buckets. It also returns the
// normalised username and whether the input is a valid one.
func usernameKey(secret []byte, raw string) (key [32]byte, username string, valid bool) {
	m := hmac.New(sha256.New, secret)
	if n, err := NormalizeUsername(raw); err == nil {
		m.Write([]byte("u:" + n))
		username, valid = n, true
	} else {
		m.Write([]byte("r:" + strings.ToLower(strings.TrimSpace(raw))))
	}
	copy(key[:], m.Sum(nil))
	return key, username, valid
}

// Contact-detail heuristics, the same as the v2 practice-text detector
// (internal/journey), used for usernames and guardian names.
var (
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern = regexp.MustCompile(`(?:\+?\d[\d\s\-().]{8,}\d)`)
	nikPattern   = regexp.MustCompile(`\b\d{16}\b`)
	urlPattern   = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.`)
)

func containsContactDetails(t string) bool {
	if emailPattern.MatchString(t) || nikPattern.MatchString(t) || urlPattern.MatchString(t) || strings.Contains(t, "@") {
		return true
	}
	for _, m := range phonePattern.FindAllString(t, -1) {
		digits := 0
		for _, r := range m {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits >= 9 {
			return true
		}
	}
	return false
}

// Password rules (ADR-0002 §6, NIST SP 800-63B-4 §3.1.1.2).
const (
	PasswordMinLength = 15
	PasswordMaxLength = 256
	PasswordMaxBytes  = 1024
)

//go:embed password_blocklist.txt
var blocklistFile string

var blocklist = func() map[string]bool {
	m := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(blocklistFile))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m[skeleton(line)] = true
	}
	return m
}()

// contextWords are service and context words a password must not consist
// of (with digits and separators).
var contextWords = []string{"selaapp", "sela", "dengarin", "dengar", "password", "passw0rd", "katasandi", "sandi", "rahasia", "secret"}

// skeleton lower-cases and keeps only letters and digits, so spacing and
// punctuation variants of a listed password match it.
func skeleton(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(norm.NFKC.String(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var sequences = []string{
	strings.Repeat("0123456789", 30), strings.Repeat("9876543210", 30),
	strings.Repeat("abcdefghijklmnopqrstuvwxyz", 12), strings.Repeat("zyxwvutsrqponmlkjihgfedcba", 12),
	strings.Repeat("qwertyuiopasdfghjklzxcvbnm", 12), strings.Repeat("qwertzuiopasdfghjklyxcvbnm", 12),
	strings.Repeat("azertyuiopqsdfghjklmwxcvbn", 12), strings.Repeat("1qaz2wsx3edc4rfv5tgb6yhn7ujm8ik9ol0p", 8),
}

// periodic reports a rune sequence that repeats a unit of at most 4 runes.
func periodic(r []rune) bool {
	for p := 1; p <= 4 && p < len(r); p++ {
		ok := true
		for i := p; i < len(r); i++ {
			if r[i] != r[i-p] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func weakPassword(normalized, username string) bool {
	lower := strings.ToLower(normalized)
	sk := skeleton(normalized)
	if username != "" {
		if strings.Contains(lower, username) {
			return true
		}
		if us := skeleton(username); len(us) >= UsernameMinLength && strings.Contains(sk, us) {
			return true
		}
	}
	if blocklist[sk] || blocklist[strings.TrimRightFunc(sk, unicode.IsDigit)] {
		return true
	}
	runes := []rune(strings.Join(strings.Fields(lower), ""))
	distinct := map[rune]bool{}
	for _, r := range runes {
		distinct[r] = true
	}
	if len(distinct) <= 3 || periodic(runes) {
		return true
	}
	for _, seq := range sequences {
		if len(sk) >= 8 && strings.Contains(seq, sk) {
			return true
		}
	}
	// Context words padded with digits ("sela2026sela2026") are weak.
	rest := sk
	for _, w := range contextWords {
		rest = strings.ReplaceAll(rest, w, "")
	}
	if rest == sk {
		return false
	}
	restRunes := map[rune]bool{}
	for _, r := range rest {
		if !unicode.IsDigit(r) {
			restRunes[r] = true
		}
	}
	return len(restRunes) <= 3
}

// CheckPassword NFKC-normalises a password and applies the length (15-256
// code points, at most 1024 bytes), blocklist and username rules. Any
// character is allowed, spaces and emoji included; there are no
// composition rules. field names the request field for errors.
func CheckPassword(password, username, field string) (string, error) {
	n := norm.NFKC.String(password)
	if len(n) > PasswordMaxBytes || utf8.RuneCountInString(n) > PasswordMaxLength {
		return "", withField(ErrPasswordTooLong, field)
	}
	if utf8.RuneCountInString(n) < PasswordMinLength {
		return "", withField(ErrPasswordTooShort, field)
	}
	if weakPassword(n, username) {
		return "", withField(ErrPasswordWeak, field)
	}
	return n, nil
}

// HashParams are argon2id parameters, stored per credential row.
type HashParams struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
}

// Argon2id parameters: the default and the floor (OWASP minimum, also a
// database CHECK).
var (
	DefaultHashParams = HashParams{MemoryKiB: 65536, Iterations: 2, Parallelism: 1}
	FloorHashParams   = HashParams{MemoryKiB: 19456, Iterations: 2, Parallelism: 1}
)

// Validate enforces the floor.
func (p HashParams) Validate() error {
	if p.MemoryKiB < FloorHashParams.MemoryKiB || p.Iterations < FloorHashParams.Iterations || p.Parallelism < 1 || p.Parallelism > 16 {
		return errors.New("account: argon2id parameters below the floor (m>=19456 KiB, t>=2, 1<=p<=16)")
	}
	return nil
}

const argon2Version = 19 // argon2.Version (0x13)

// PasswordHash is a stored argon2id hash.
type PasswordHash struct {
	Tag    []byte
	Salt   []byte
	Params HashParams
}

// ParamCount is how many stored credentials use one parameter tuple.
type ParamCount struct {
	Params HashParams
	Count  int64
}

// dummyTuple is one dummy hash of the unknown-username distribution; upto
// is the cumulative weight up to and including it.
type dummyTuple struct {
	hash PasswordHash
	upto uint64
}

// Hasher computes argon2id behind a concurrency semaphore. A request that
// waits longer than the configured time gets ErrServerBusy.
//
// Unknown usernames are verified against dummy hashes whose parameters
// follow the distribution of the stored credentials (SetDistribution), so
// a failed proof costs the same whether or not the account exists, also
// after the operator changes the parameters: an existing account costs
// what its own row's parameters cost until its next successful login
// re-hashes it. Each unknown username is mapped to one tuple by its keyed
// username key, in proportion to the stored counts, so repeating a
// username repeats its cost as an existing account does. The mapping is by
// fraction (the key's first 64 bits read as a number in [0, 1) and scaled
// to the total), not by remainder: when the counts change (a
// registration, an upgrade, a deletion, a re-hash) only the usernames
// between the old and the new boundary of two tuples move, as many as the
// shift of that boundary, instead of about half of all unknown usernames
// at every refresh (review round 3, R3-1).
type Hasher struct {
	params HashParams
	sem    chan struct{}
	wait   time.Duration

	mu      sync.RWMutex
	dummies []dummyTuple
	total   uint64

	hashes        atomic.Uint64
	verifications atomic.Uint64
	byParams      sync.Map // HashParams -> *atomic.Uint64
}

// NewHasher validates the parameters and prepares the dummy hash used for
// unknown usernames (the configured parameters until SetDistribution).
func NewHasher(p HashParams, concurrency int, wait time.Duration) (*Hasher, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if concurrency < 1 {
		concurrency = 2
	}
	if wait <= 0 {
		wait = 5 * time.Second
	}
	h := &Hasher{params: p, sem: make(chan struct{}, concurrency), wait: wait}
	h.SetDistribution(nil)
	return h, nil
}

// newDummy returns a dummy hash with the given parameters. Its tag is
// random: the verification result is discarded, only its cost matters.
func newDummy(p HashParams) PasswordHash {
	tag := make([]byte, 32)
	_, _ = rand.Read(tag)
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	return PasswordHash{Tag: tag, Salt: salt, Params: p}
}

// SetDistribution replaces the unknown-username dummies with one per
// parameter tuple in dist, weighted by its count (tuples below the floor
// or without rows are ignored). An empty distribution (no credentials
// yet) uses the configured parameters.
func (h *Hasher) SetDistribution(dist []ParamCount) {
	dist = slices.Clone(dist)
	slices.SortFunc(dist, func(a, b ParamCount) int {
		return cmp.Or(cmp.Compare(a.Params.MemoryKiB, b.Params.MemoryKiB), cmp.Compare(a.Params.Iterations, b.Params.Iterations),
			cmp.Compare(a.Params.Parallelism, b.Params.Parallelism))
	})
	var dummies []dummyTuple
	var total uint64
	for _, d := range dist {
		if d.Count <= 0 || d.Params.Validate() != nil {
			continue
		}
		total += uint64(d.Count)
		dummies = append(dummies, dummyTuple{hash: newDummy(d.Params), upto: total})
	}
	if total == 0 {
		dummies, total = []dummyTuple{{hash: newDummy(h.params), upto: 1}}, 1
	}
	h.mu.Lock()
	h.dummies, h.total = dummies, total
	h.mu.Unlock()
}

// Distribution reports the parameter tuples unknown usernames are
// verified with and their weights.
func (h *Hasher) Distribution() []ParamCount {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]ParamCount, len(h.dummies))
	var prev uint64
	for i, d := range h.dummies {
		out[i] = ParamCount{Params: d.hash.Params, Count: int64(d.upto - prev)}
		prev = d.upto
	}
	return out
}

// dummyFor picks the dummy hash of an unknown username from its key: the
// high word of key[:8] × total, that is floor(fraction × total), so a
// key keeps its tuple while the boundaries around its fraction stay on
// the same side of it (see Hasher).
func (h *Hasher) dummyFor(seed [32]byte) *PasswordHash {
	h.mu.RLock()
	defer h.mu.RUnlock()
	v, _ := bits.Mul64(binary.BigEndian.Uint64(seed[:8]), h.total)
	for i := range h.dummies {
		if v < h.dummies[i].upto {
			return &h.dummies[i].hash
		}
	}
	return &h.dummies[len(h.dummies)-1].hash
}

// UnknownParams reports the parameters a failed proof of an unknown
// username with this key is verified with (tests and diagnostics; it runs
// no hash).
func (h *Hasher) UnknownParams(key [32]byte) HashParams { return h.dummyFor(key).Params }

// Params returns the configured parameters.
func (h *Hasher) Params() HashParams { return h.params }

// Verifications counts argon2id verifications (tests check uniform work).
func (h *Hasher) Verifications() uint64 { return h.verifications.Load() }

// VerificationsAt counts argon2id verifications run with parameters p,
// real and dummy (tests check that unknown usernames follow the stored
// distribution).
func (h *Hasher) VerificationsAt(p HashParams) uint64 {
	if c, ok := h.byParams.Load(p); ok {
		return c.(*atomic.Uint64).Load()
	}
	return 0
}

// Hashes counts argon2id hash computations.
func (h *Hasher) Hashes() uint64 { return h.hashes.Load() }

func (h *Hasher) acquire(ctx context.Context) (func(), error) {
	t := time.NewTimer(h.wait)
	defer t.Stop()
	select {
	case h.sem <- struct{}{}:
		return func() { <-h.sem }, nil
	case <-t.C:
		return nil, ErrServerBusy
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Occupy takes every hashing slot until release is called (tests).
func (h *Hasher) Occupy() (release func()) {
	for i := 0; i < cap(h.sem); i++ {
		h.sem <- struct{}{}
	}
	return func() {
		for i := 0; i < cap(h.sem); i++ {
			<-h.sem
		}
	}
}

// Hash computes a new argon2id hash with a random 16-byte salt.
func (h *Hasher) Hash(ctx context.Context, password string) (PasswordHash, error) {
	release, err := h.acquire(ctx)
	if err != nil {
		return PasswordHash{}, err
	}
	defer release()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return PasswordHash{}, err
	}
	h.hashes.Add(1)
	p := h.params
	return PasswordHash{Tag: argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, 32), Salt: salt, Params: p}, nil
}

// Verify checks a password against a stored hash in constant time. A nil
// stored hash is VerifyUnknown with a zero key.
func (h *Hasher) Verify(ctx context.Context, password string, stored *PasswordHash) (bool, error) {
	if stored == nil {
		return h.VerifyUnknown(ctx, password, [32]byte{})
	}
	return h.verify(ctx, password, stored)
}

// VerifyUnknown runs the verification of a username without an account
// against the dummy hash its key selects and returns false, so unknown
// usernames cost what existing accounts with the same distribution of
// stored parameters cost.
func (h *Hasher) VerifyUnknown(ctx context.Context, password string, key [32]byte) (bool, error) {
	_, err := h.verify(ctx, password, h.dummyFor(key))
	return false, err
}

func (h *Hasher) verify(ctx context.Context, password string, ph *PasswordHash) (bool, error) {
	release, err := h.acquire(ctx)
	if err != nil {
		return false, err
	}
	defer release()
	h.verifications.Add(1)
	c, _ := h.byParams.LoadOrStore(ph.Params, new(atomic.Uint64))
	c.(*atomic.Uint64).Add(1)
	p := ph.Params
	got := argon2.IDKey([]byte(password), ph.Salt, p.Iterations, p.MemoryKiB, p.Parallelism, uint32(len(ph.Tag)))
	return subtle.ConstantTimeCompare(got, ph.Tag) == 1, nil
}

// Recovery codes (ADR-0002 §6): 10 codes of 16 Crockford base32
// characters (80 bits), shown as XXXX-XXXX-XXXX-XXXX once.
const (
	RecoveryCodeCount  = 10
	RecoveryCodeLength = 16
	recoveryDomain     = "sela.recovery.v1"
)

const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var crockford = base32.NewEncoding(crockfordAlphabet).WithPadding(base32.NoPadding)

// GenerateRecoveryCode returns one formatted code.
func GenerateRecoveryCode() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s := crockford.EncodeToString(b)
	return s[0:4] + "-" + s[4:8] + "-" + s[8:12] + "-" + s[12:16], nil
}

// NormalizeRecoveryCode accepts any case, spaces and hyphens, and maps the
// Crockford look-alikes O→0 and I/L→1. It reports false for anything that
// is not 16 alphabet characters.
func NormalizeRecoveryCode(in string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(in) {
		switch {
		case r == ' ' || r == '-' || r == '\t':
			continue
		case r == 'O':
			r = '0'
		case r == 'I' || r == 'L':
			r = '1'
		}
		if !strings.ContainsRune(crockfordAlphabet, r) {
			return "", false
		}
		b.WriteRune(r)
	}
	s := b.String()
	return s, len(s) == RecoveryCodeLength
}

// RecoveryCodeHash is SHA-256("sela.recovery.v1" ‖ user id ‖ normalised
// code). Binding the user id prevents cross-account lookups; 80 bits of
// entropy make a slow hash unnecessary.
func RecoveryCodeHash(user UserID, normalized string) []byte {
	h := sha256.New()
	h.Write([]byte(recoveryDomain))
	h.Write(user[:])
	h.Write([]byte(normalized))
	return h.Sum(nil)
}

// Session tokens: 32 random bytes, base64url without padding (43
// characters). Only SHA-256(token) is stored.
const SessionTokenLength = 43

// NewSessionToken returns a token and the hash stored for it.
func NewSessionToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	tok := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(tok))
	return tok, h[:], nil
}

// HashSessionToken validates a cookie value and returns its lookup hash.
func HashSessionToken(tok string) ([]byte, bool) {
	if len(tok) != SessionTokenLength {
		return nil, false
	}
	if b, err := base64.RawURLEncoding.DecodeString(tok); err != nil || len(b) != 32 {
		return nil, false
	}
	h := sha256.Sum256([]byte(tok))
	return h[:], true
}

// Device is the closed-enum device label derived from the User-Agent at
// session creation; the raw User-Agent and the IP are never stored.
type Device struct {
	Browser string `json:"browser"`
	OS      string `json:"os"`
}

// DeviceFromUserAgent maps a User-Agent to the closed enums.
func DeviceFromUserAgent(ua string) Device {
	d := Device{Browser: "other", OS: "other"}
	switch {
	case strings.Contains(ua, "SamsungBrowser/"):
		d.Browser = "samsung"
	case strings.Contains(ua, "Edg/") || strings.Contains(ua, "EdgA/") || strings.Contains(ua, "EdgiOS/"):
		d.Browser = "edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		d.Browser = "opera"
	case strings.Contains(ua, "Firefox/") || strings.Contains(ua, "FxiOS/"):
		d.Browser = "firefox"
	case strings.Contains(ua, "CriOS/") || strings.Contains(ua, "Chrome/"):
		d.Browser = "chrome"
	case strings.Contains(ua, "Safari/") && strings.Contains(ua, "Version/"):
		d.Browser = "safari"
	}
	switch {
	case strings.Contains(ua, "Android"):
		d.OS = "android"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad") || strings.Contains(ua, "iPod"):
		d.OS = "ios"
	case strings.Contains(ua, "CrOS"):
		d.OS = "chromeos"
	case strings.Contains(ua, "Windows"):
		d.OS = "windows"
	case strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X"):
		d.OS = "macos"
	case strings.Contains(ua, "Linux"):
		d.OS = "linux"
	}
	return d
}
