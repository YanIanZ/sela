package account_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

func TestUsernameRules(t *testing.T) {
	t.Parallel()
	ok := map[string]string{
		"pelangi.senja": "pelangi.senja", "  Pelangi.Senja  ": "pelangi.senja", "abc": "abc", "a_b-c.d": "a_b-c.d",
		"0rang": "0rang", strings.Repeat("a", 32): strings.Repeat("a", 32), "selain": "selain", "teman2026": "teman2026",
	}
	for in, want := range ok {
		got, err := account.NormalizeUsername(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q %v, want %q", in, got, err, want)
		}
	}
	refused := []string{
		"", "ab", strings.Repeat("a", 33), ".start", "-start", "_start", "has space", "emoji🌧", "pelangí", "ｐｅｌａｎｇｉ",
		"admin", "Admin", "sela", "s.e.l.a", "sela.support", "support", "root", "guest", "official", "help", "security",
		"moderator", "dengarin", "budi@gmail.com", "budi.gmail", "pelangi.com", "user.id", "081234567890", "62812345678",
		"nik3201234567890001", "a1b2c3d4e5f6g7h8i9", "user/name", "user+tag",
	}
	for _, in := range refused {
		if _, err := account.NormalizeUsername(in); !errors.Is(err, account.ErrUsernameInvalid) {
			t.Errorf("%q accepted (%v)", in, err)
		}
	}
	var e *account.Error
	_, err := account.NormalizeUsername("x")
	if !errors.As(err, &e) || e.Field != "username" {
		t.Fatalf("field: %v", err)
	}
}

func TestUsernameUniqueIgnoresCase(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("Pelangi.Senja")
	_, err := f.store.Register(ctx, account.RegisterInput{Username: "PELANGI.SENJA", Password: "beda kata sandi yang cukup panjang", Locale: "id",
		AgeBand: account.AgeBand18Plus, TermsPolicyVersion: termsV})
	if !errors.Is(err, account.ErrUsernameTaken) {
		t.Fatalf("case variant: %v", err)
	}
	var stored string
	_ = f.db.Pool.QueryRow(ctx, `SELECT username FROM credentials`).Scan(&stored)
	if stored != "pelangi.senja" {
		t.Fatalf("stored %q", stored)
	}
	// The failed registration left nothing behind.
	if n := f.count(`SELECT count(*) FROM users`); n != 1 {
		t.Fatalf("%d users after a duplicate registration", n)
	}
}

func TestPasswordPolicy(t *testing.T) {
	t.Parallel()
	accepted := []string{
		"tiga kata yang mudah diingat", "aku 🌧️ suka hujan sore hari", "kopi pagi di teras rumah", "849302746512093",
		"Ünïcödé pässwörd ok", "spasi di depan dan belakang ",
	}
	for _, pw := range accepted {
		if _, err := account.CheckPassword(pw, "pelangi.senja", "password"); err != nil {
			t.Errorf("%q refused: %v", pw, err)
		}
	}
	// Exactly 15 code points passes; 14 does not. Counted after NFKC, in
	// code points, not bytes.
	if _, err := account.CheckPassword("sungai biru deh", "", "password"); err != nil {
		t.Errorf("15 code points refused: %v", err)
	}
	if _, err := account.CheckPassword("sungai biru de", "", "password"); !errors.Is(err, account.ErrPasswordTooShort) {
		t.Errorf("14 code points: %v", err)
	}
	if _, err := account.CheckPassword("🌧🌻🍵🚲🎧📚🌙🧩🪴🐢🦋🍉🎈🌈🌊", "", "password"); err != nil {
		t.Errorf("15 emoji refused: %v", err)
	}
	// NFKC: full-width letters count after normalisation.
	n, err := account.CheckPassword("ｐａｓｓｐｈｒａｓｅ ｗｉｔｈ ｗｉｄｔｈ", "", "password")
	if err != nil || n != "passphrase with width" {
		t.Errorf("NFKC: %q %v", n, err)
	}
	long := strings.Repeat("ab", 128) + "c"
	if _, err := account.CheckPassword(long, "", "password"); !errors.Is(err, account.ErrPasswordTooLong) {
		t.Errorf("257 code points: %v", err)
	}
	if _, err := account.CheckPassword(strings.Repeat("🌧🌻🍵🚲", 65), "", "password"); !errors.Is(err, account.ErrPasswordTooLong) {
		t.Errorf("over 256 code points and 1024 bytes: %v", err)
	}
	var maxLen []rune
	for i := 0; i < 256; i++ {
		maxLen = append(maxLen, rune(0x1F300+(i*7)%200))
	}
	if _, err := account.CheckPassword(string(maxLen), "", "password"); err != nil || len(string(maxLen)) != 1024 {
		t.Errorf("256 code points (1024 bytes) refused: %v", err)
	}
	weak := []string{
		"passwordpassword", "Password Password", "correct horse battery staple", "bismillahirrahmanirrahim",
		"aaaaaaaaaaaaaaaa", "abababababababab", "abcabcabcabcabcabc", "123456789012345", "qwertyuiopasdfgh",
		"abcdefghijklmnopq", "sela sela sela 2026", "dengarin12345678", "password 1234567890",
		"pelangi.senja punya kata sandi", "PelangiSenja dan teman teman", "tiga kata panjang untuk diingat",
	}
	for _, pw := range weak {
		_, err := account.CheckPassword(pw, "pelangi.senja", "newPassword")
		var e *account.Error
		if !errors.Is(err, account.ErrPasswordWeak) || !errors.As(err, &e) || e.Field != "newPassword" {
			t.Errorf("%q: %v", pw, err)
		}
	}
}

func TestPasswordHashParameters(t *testing.T) {
	t.Parallel()
	if account.DefaultHashParams != (account.HashParams{MemoryKiB: 65536, Iterations: 2, Parallelism: 1}) {
		t.Fatalf("defaults: %+v", account.DefaultHashParams)
	}
	for _, p := range []account.HashParams{{19455, 2, 1}, {19456, 1, 1}, {19456, 2, 0}, {19456, 2, 17}} {
		if p.Validate() == nil {
			t.Errorf("%+v accepted below the floor", p)
		}
	}
	h, err := account.NewHasher(account.FloorHashParams, 2, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ph, err := h.Hash(ctx, "kata sandi uji yang panjang")
	if err != nil {
		t.Fatal(err)
	}
	if len(ph.Tag) != 32 || len(ph.Salt) != 16 || ph.Params != account.FloorHashParams {
		t.Fatalf("hash shape: %d %d %+v", len(ph.Tag), len(ph.Salt), ph.Params)
	}
	ph2, _ := h.Hash(ctx, "kata sandi uji yang panjang")
	if bytes.Equal(ph.Salt, ph2.Salt) || bytes.Equal(ph.Tag, ph2.Tag) {
		t.Fatal("salt reused")
	}
	if ok, _ := h.Verify(ctx, "kata sandi uji yang panjang", &ph); !ok {
		t.Fatal("verify failed")
	}
	if ok, _ := h.Verify(ctx, "kata sandi uji yang panjanG", &ph); ok {
		t.Fatal("wrong password verified")
	}
	before := h.Verifications()
	if ok, _ := h.Verify(ctx, "anything at all here", nil); ok {
		t.Fatal("dummy verification succeeded")
	}
	if h.Verifications() != before+1 {
		t.Fatal("dummy verification did not run argon2id")
	}
}

func TestPasswordHashStoredPerRowAtFloor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("baris.parameter")
	var v, mem, iter, par int
	var hashLen, saltLen int
	if err := f.db.Pool.QueryRow(ctx, `SELECT argon2_version, argon2_memory_kib, argon2_iterations, argon2_parallelism,
		octet_length(password_hash), octet_length(password_salt) FROM credentials`).Scan(&v, &mem, &iter, &par, &hashLen, &saltLen); err != nil {
		t.Fatal(err)
	}
	if v != 19 || mem != 19456 || iter != 2 || par != 1 || hashLen != 32 || saltLen != 16 {
		t.Fatalf("stored parameters: v=%d m=%d t=%d p=%d", v, mem, iter, par)
	}
	// The database refuses parameters below the floor.
	for _, sql := range []string{`UPDATE credentials SET argon2_memory_kib = 19455`, `UPDATE credentials SET argon2_iterations = 1`} {
		if _, err := f.db.Pool.Exec(ctx, sql); err == nil {
			t.Errorf("%s accepted", sql)
		}
	}
}

func TestRehashOnLogin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.register("ganti.parameter")
	stronger := account.HashParams{MemoryKiB: 20480, Iterations: 3, Parallelism: 1}
	s2 := f.restart(func(c *account.Config) { c.Hash = stronger })
	var oldHash []byte
	_ = f.db.Pool.QueryRow(ctx, `SELECT password_hash FROM credentials`).Scan(&oldHash)
	if _, err := s2.Login(ctx, account.LoginInput{Username: "ganti.parameter", Password: markerPassword}); err != nil {
		t.Fatal(err)
	}
	var mem, iter int
	var newHash []byte
	_ = f.db.Pool.QueryRow(ctx, `SELECT argon2_memory_kib, argon2_iterations, password_hash FROM credentials`).Scan(&mem, &iter, &newHash)
	if mem != 20480 || iter != 3 || bytes.Equal(oldHash, newHash) {
		t.Fatalf("not re-hashed: m=%d t=%d", mem, iter)
	}
	// The new hash still verifies, and a login at equal parameters keeps it.
	if _, err := s2.Login(ctx, account.LoginInput{Username: "ganti.parameter", Password: markerPassword}); err != nil {
		t.Fatal(err)
	}
	var again []byte
	_ = f.db.Pool.QueryRow(ctx, `SELECT password_hash FROM credentials`).Scan(&again)
	if !bytes.Equal(again, newHash) {
		t.Fatal("re-hashed although parameters were current")
	}
}

func TestHashConcurrencyBusy(t *testing.T) {
	t.Parallel()
	h, err := account.NewHasher(account.FloorHashParams, 1, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	release := h.Occupy()
	start := time.Now()
	_, err = h.Hash(ctx, "kata sandi uji yang panjang")
	var e *account.Error
	if !errors.Is(err, account.ErrServerBusy) || !errors.As(err, &e) || e.RetryAfter != 5*time.Second {
		t.Fatalf("busy: %v", err)
	}
	if el := time.Since(start); el < 40*time.Millisecond || el > 2*time.Second {
		t.Fatalf("waited %s", el)
	}
	if _, err := h.Verify(ctx, "x", nil); !errors.Is(err, account.ErrServerBusy) {
		t.Fatalf("verify busy: %v", err)
	}
	release()
	if _, err := h.Hash(ctx, "kata sandi uji yang panjang"); err != nil {
		t.Fatal(err)
	}
	// Waiting requests are served once a slot frees within the wait.
	var wg sync.WaitGroup
	h2, _ := account.NewHasher(account.FloorHashParams, 1, 30*time.Second)
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h2.Hash(ctx, "kata sandi uji yang panjang")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if h2.Hashes() != 3 {
		t.Fatalf("hashes %d", h2.Hashes())
	}
}

func TestRecoveryCodesFormat(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := account.GenerateRecoveryCode()
		if err != nil || !re.MatchString(c) || seen[c] {
			t.Fatalf("code %q %v", c, err)
		}
		seen[c] = true
	}
	n, ok := account.NormalizeRecoveryCode(" 7k2m qx9d-4htr PZ3B ")
	if !ok || n != "7K2MQX9D4HTRPZ3B" {
		t.Fatalf("tolerant entry: %q %v", n, ok)
	}
	if n, ok := account.NormalizeRecoveryCode("oiL0-0000-0000-0000"); !ok || n != "0110000000000000" {
		t.Fatalf("look-alikes: %q %v", n, ok)
	}
	for _, bad := range []string{"", "7K2M-QX9D-4HTR", "7K2M-QX9D-4HTR-PZ3BX", "7K2M-QX9D-4HTR-PZ3U", "7K2M QX9D 4HTR PZ3!"} {
		if _, ok := account.NormalizeRecoveryCode(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
	a, b := account.NewUserID(), account.NewUserID()
	ha, hb := account.RecoveryCodeHash(a, n), account.RecoveryCodeHash(b, n)
	want := sha256.Sum256(append(append([]byte("sela.recovery.v1"), a[:]...), []byte(n)...))
	if len(ha) != 32 || bytes.Equal(ha, hb) || !bytes.Equal(ha, want[:]) {
		t.Fatal("recovery hash is not the user-bound SHA-256")
	}
}

func TestSessionTokenShape(t *testing.T) {
	t.Parallel()
	tok, hash, err := account.NewSessionToken()
	if err != nil || len(tok) != 43 || len(hash) != 32 {
		t.Fatalf("token %d hash %d %v", len(tok), len(hash), err)
	}
	h2, ok := account.HashSessionToken(tok)
	if !ok || !bytes.Equal(h2, hash) {
		t.Fatal("hash mismatch")
	}
	for _, bad := range []string{"", tok[:42], tok + "A", strings.Repeat("!", 43)} {
		if _, ok := account.HashSessionToken(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDeviceFromUserAgent(t *testing.T) {
	t.Parallel()
	for ua, want := range map[string]account.Device{
		"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0 Mobile Safari/537.36":  {Browser: "samsung", OS: "android"},
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1": {Browser: "safari", OS: "ios"},
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 Edg/130.0":                   {Browser: "edge", OS: "windows"},
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14.6; rv:131.0) Gecko/20100101 Firefox/131.0":                                                     {Browser: "firefox", OS: "macos"},
		"Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36":                              {Browser: "chrome", OS: "chromeos"},
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36 OPR/114.0":                             {Browser: "opera", OS: "linux"},
		"curl/8.5": {Browser: "other", OS: "other"},
		"":         {Browser: "other", OS: "other"},
	} {
		if got := account.DeviceFromUserAgent(ua); got != want {
			t.Errorf("%q: %+v want %+v", ua, got, want)
		}
	}
}

func TestBackoffDelaySchedule(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]time.Duration{0: 0, 4: 0, 5: time.Second, 6: 2 * time.Second, 10: 32 * time.Second,
		14: 512 * time.Second, 15: 15 * time.Minute, 40: 15 * time.Minute, 99: 15 * time.Minute} {
		if got := account.BackoffDelay(n); got != want {
			t.Errorf("BackoffDelay(%d) = %s, want %s", n, got, want)
		}
	}
}
