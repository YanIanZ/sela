package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/db"
)

// v3Messages are the localized messages of the codes /api/v3 adds
// (API_SPEC_V3 §2). Shared codes keep their v2 messages.
var v3Messages = map[string]message{
	"storage_required":           {"Pilih simpan sebelum menyimpan catatan ini.", "Choose to save before storing this entry."},
	"not_signed_in":              {"Kamu belum masuk atau sesimu sudah berakhir. Silakan masuk lagi.", "You are not signed in, or your session has ended. Please sign in again."},
	"invalid_credentials":        {"Nama pengguna atau kata sandi tidak cocok.", "The username or password does not match."},
	"recovery_failed":            {"Nama pengguna atau kode pemulihan tidak cocok, atau kode itu sudah pernah dipakai.", "The username or recovery code does not match, or that code was already used."},
	"current_password_incorrect": {"Kata sandi saat ini tidak cocok.", "The current password does not match."},
	"reauth_required":            {"Untuk melindungi datamu, masukkan kata sandimu sekali lagi.", "To protect your data, enter your password once more."},
	"account_required":           {"Langkah ini memerlukan akun. Simpan data tamumu ke akun terlebih dahulu.", "This step needs an account. Save your guest data to an account first."},
	"password_login_disabled":    {"Masuk dengan kata sandi dinonaktifkan sementara karena terlalu banyak percobaan. Pakai kode pemulihan untuk membuat kata sandi baru.", "Password sign-in is disabled after too many attempts. Use a recovery code to set a new password."},
	"age_not_supported":          {"Sela saat ini dibuat untuk usia 16 tahun ke atas. Jika kamu butuh bantuan sekarang, lihat informasi bantuan.", "Sela is currently made for people aged 16 and over. If you need help now, see the help information."},
	"age_band_not_enabled":       {"Sela belum tersedia untuk kelompok usia ini.", "Sela is not yet available for this age group."},
	"not_guest":                  {"Akun ini sudah terdaftar.", "This account is already registered."},
	"required_consent":           {"Persetujuan ini diperlukan untuk memakai Sela. Jika tidak lagi setuju, kamu bisa mengekspor lalu menghapus akunmu.", "Sela cannot be used without this consent. If you no longer agree, you can export and then delete your account."},
	"username_invalid":           {"Nama pengguna harus 3–32 karakter: huruf a–z, angka, titik, garis bawah, atau tanda hubung. Jangan memakai nama asli, email, atau nomor telepon.", "Usernames are 3–32 characters: letters a–z, digits, dot, underscore or hyphen. Do not use your real name, email or phone number."},
	"username_taken":             {"Nama pengguna ini sudah dipakai. Pilih yang lain.", "This username is taken. Choose another."},
	"password_too_short":         {"Kata sandi minimal 15 karakter. Coba gabungan beberapa kata yang mudah kamu ingat.", "Passwords need at least 15 characters. Try a few words you can remember."},
	"password_too_long":          {"Kata sandi maksimal 256 karakter.", "Passwords can have at most 256 characters."},
	"password_weak":              {"Kata sandi ini terlalu umum atau mudah ditebak. Pilih yang lain.", "This password is too common or easy to guess. Choose another."},
	"store_unavailable":          {"Penyimpanan sedang tidak tersedia. Coba lagi sebentar lagi.", "Storage is unavailable right now. Try again shortly."},
	"server_busy":                {"Server sedang sibuk. Coba lagi sebentar lagi.", "The server is busy. Try again shortly."},
	"invalid_share":              {"Tautan atau kode tidak cocok, atau sudah tidak berlaku.", "The link or code doesn't match, or it is no longer valid."},
	"care_org_unavailable":       {"Layanan ini sedang tidak menerima permintaan. Pilih layanan lain.", "This service isn't taking requests right now. Choose another one."},
	"meaning_graph_limit":        {"Terlalu banyak hal bergantung pada catatan ini untuk diubah sekaligus. Hapus beberapa ringkasan atau tautan dulu.", "Too much rests on this entry to change it at once. Remove some summaries or links first."},
	"quote_not_in_source":        {"Kutipan ini tidak ada di versi sumber yang dipilih.", "This quote is not in the selected version of its source."},
	"claim_not_usable":           {"Pemahaman yang dirujuk belum kamu konfirmasi, sudah berubah, atau perlu ditinjau ulang.", "The meaning this cites isn't confirmed by you, has changed, or needs review again."},
	"name_taken":                 {"Nama ini sudah ada di daftarmu.", "This name is already on your list."},
	"name_limit_reached":         {"Daftar nama berisi paling banyak 20 nama. Hapus satu dulu untuk menambah yang lain.", "The name list holds at most 20 names. Remove one before adding another."},
}

// v3Status is the documented HTTP status of every code /api/v3 returns.
var v3Status = map[string]int{
	"storage_required": http.StatusForbidden,
	"consent_required": http.StatusForbidden,
	"invalid_input":    http.StatusBadRequest, "not_found": http.StatusNotFound, "method_not_allowed": http.StatusMethodNotAllowed,
	"stale_version": http.StatusConflict, "invalid_transition": http.StatusConflict, "policy_outdated": http.StatusConflict,
	"too_large": http.StatusRequestEntityTooLarge, "unsupported_media": http.StatusUnsupportedMediaType, "cross_origin": http.StatusForbidden,
	"rate_limited": http.StatusTooManyRequests, "internal": http.StatusInternalServerError,

	"not_signed_in": http.StatusUnauthorized, "invalid_credentials": http.StatusUnauthorized, "recovery_failed": http.StatusUnauthorized,
	"current_password_incorrect": http.StatusForbidden, "reauth_required": http.StatusForbidden, "account_required": http.StatusForbidden,
	"password_login_disabled": http.StatusForbidden, "age_not_supported": http.StatusForbidden, "age_band_not_enabled": http.StatusForbidden,
	"not_guest": http.StatusConflict, "required_consent": http.StatusConflict, "username_invalid": http.StatusBadRequest,
	"username_taken": http.StatusConflict, "password_too_short": http.StatusBadRequest, "password_too_long": http.StatusBadRequest,
	"password_weak": http.StatusBadRequest, "invalid_share": http.StatusUnauthorized,
	"name_taken": http.StatusConflict, "name_limit_reached": http.StatusConflict,
	"care_org_unavailable": http.StatusConflict,
	"meaning_graph_limit":  http.StatusConflict, "quote_not_in_source": http.StatusBadRequest, "claim_not_usable": http.StatusConflict,
	"store_unavailable": http.StatusServiceUnavailable, "server_busy": http.StatusServiceUnavailable,
}

func init() {
	for code, m := range v3Messages {
		messages[code] = m
	}
}

func retryAfter(w http.ResponseWriter, d time.Duration) {
	secs := int((d + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
}

// storeRetryAfter is the Retry-After of 503 store_unavailable when the
// database cannot be reached at run time.
const storeRetryAfter = 5 * time.Second

// v3Error writes an account error with its documented status; a
// not_signed_in answer also clears the cookie. A database that cannot be
// reached (refused or lost connection, timeout, closed pool, server
// shutting down: db.Unavailable) is 503 store_unavailable with
// Retry-After, so clients back off and retry (API_SPEC_V3 §2). Anything
// else is a 500. Both log only a value-free error class.
func (s *Server) v3Error(w http.ResponseWriter, r *http.Request, err error) {
	var e *account.Error
	if errors.As(err, &e) {
		status, ok := v3Status[e.Code]
		if !ok {
			status = http.StatusInternalServerError
		}
		if e.Code == "not_signed_in" {
			s.clearAuthCookie(w)
		}
		if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
			d := e.RetryAfter
			if d <= 0 {
				d = 5 * time.Second
			}
			retryAfter(w, d)
		}
		writeError(w, r, status, e.Code, e.Field)
		return
	}
	if db.Unavailable(err) {
		s.log.Warn("v3.store_unavailable", "request_id", w.Header().Get("X-Request-Id"), "route", r.Pattern, "class", db.Classify(err))
		retryAfter(w, storeRetryAfter)
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return
	}
	s.log.Error("v3.error", "request_id", w.Header().Get("X-Request-Id"), "route", r.Pattern, "class", db.Classify(err))
	writeError(w, r, http.StatusInternalServerError, "internal", "")
}
