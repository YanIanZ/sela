package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/YanIanZ/sela/internal/journey"
)

// message is a localized error message.
type message struct{ id, en string }

// messages localizes stable error codes. The frontend has its own copy of
// these strings; the server message is a fallback for API clients.
var messages = map[string]message{
	journey.CodeInvalid:           {"Permintaan tidak valid. Periksa isian lalu coba lagi.", "The request is invalid. Check the input and try again."},
	journey.CodeSessionMissing:    {"Sesi demo tidak ditemukan atau sudah berakhir. Data sesi lama sudah dihapus.", "The demo session was not found or has ended. Old session data has been deleted."},
	journey.CodeStale:             {"Data sudah berubah di tab atau langkah lain. Muat ulang untuk melihat versi terbaru.", "The data changed in another tab or step. Reload to see the latest version."},
	journey.CodeConsentRequired:   {"Langkah ini memerlukan izinmu terlebih dahulu.", "This step needs your permission first."},
	journey.CodeCrisis:            {"Alur biasa dihentikan. Silakan lihat informasi bantuan segera.", "The ordinary journey has stopped. Please see the immediate-help information."},
	journey.CodeApprovalRequired:  {"Setujui versi catatan terbaru terlebih dahulu.", "Approve the latest version of your note first."},
	journey.CodeReviewIncomplete:  {"Tinjau setiap butir catatan sebelum menyetujui.", "Review every note item before approving."},
	journey.CodeNotFound:          {"Data tidak ditemukan.", "Not found."},
	journey.CodeForbidden:         {"Akses ditolak.", "Access denied."},
	journey.CodeRateLimited:       {"Terlalu banyak permintaan. Tunggu sebentar lalu coba lagi.", "Too many requests. Wait a moment and try again."},
	journey.CodeAIDisabled:        {"Fitur AI tidak aktif di server ini. Kamu tetap bisa memakai jalur manual.", "AI is not enabled on this server. You can still use the manual path."},
	journey.CodeAIUnavailable:     {"Model AI belum tersedia saat ini. Kamu bisa mencoba lagi atau memakai jalur manual.", "The AI model is unavailable right now. You can retry or use the manual path."},
	journey.CodeOpInProgress:      {"Proses AI sebelumnya masih berjalan.", "A previous AI step is still running."},
	journey.CodeInvalidTransition: {"Langkah ini tidak bisa dilakukan pada status sekarang.", "This step is not possible in the current status."},
	journey.CodeIdempotency:       {"Permintaan dengan kunci yang sama tetapi isi berbeda ditolak.", "A request with the same key but different content was rejected."},
	journey.CodeIdempotencyKey:    {"Permintaan ini membutuhkan kunci idempotensi.", "This request needs an idempotency key."},
	journey.CodeCapacity:          {"Demo sedang penuh. Coba lagi beberapa menit lagi.", "The demo is full. Try again in a few minutes."},
	journey.CodeContactDetails:    {"Teks tampak memuat kontak, alamat web, atau nomor identitas. Hapus data asli sebelum melanjutkan.", "The text appears to contain contact details, a web address or an ID number. Remove real data before continuing."},
	journey.CodePolicyOutdated:    {"Ketentuan sudah diperbarui. Baca versi terbaru sebelum menyetujui.", "The terms changed. Read the latest version before agreeing."},
	journey.CodeAccessEnded:       {"Akses ke catatan ini tidak tersedia: tautan atau kode salah, atau akses sudah berakhir atau dicabut.", "Access to this note is not available: the link or code is wrong, or access has ended or been revoked."},
	journey.CodeHandoffExists:     {"Sudah ada permintaan aktif ke penerima ini.", "There is already an active request to this recipient."},
	journey.CodeSlotUnavailable:   {"Jadwal fiktif ini tidak tersedia. Pilih jadwal lain.", "This fictional slot is not available. Choose another one."},
	journey.CodeNoStory:           {"Ceritakan atau pilih cerita contoh terlebih dahulu.", "Write or choose a sample story first."},
	journey.CodeAdultRequired:     {"Demo ini hanya untuk orang dewasa (18+).", "This demo is for adults (18+) only."},
	"too_large":                   {"Isian terlalu besar.", "The input is too large."},
	"cross_origin":                {"Permintaan lintas situs ditolak.", "Cross-site requests are rejected."},
	"unsupported_media":           {"Gunakan JSON.", "Use JSON."},
	"internal":                    {"Terjadi kesalahan di server. Coba lagi.", "Something went wrong on the server. Try again."},
	"research_unavailable":        {"Korpus riset belum dibangun di server ini.", "The research corpus has not been built on this server."},
	"frontend_not_built":          {"Frontend belum dibangun.", "The frontend has not been built."},
}

// lang picks the message language: explicit header, then Accept-Language.
func lang(r *http.Request) string {
	if l := r.Header.Get("X-Sela-Language"); l == "en" || l == "id" {
		return l
	}
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		return "en"
	}
	return "id"
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Field   string `json:"field,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, field string) {
	var b errorBody
	b.Error.Code, b.Error.Field = code, field
	m, ok := messages[code]
	if !ok {
		m = messages["internal"]
	}
	if lang(r) == "en" {
		b.Error.Message = m.en
	} else {
		b.Error.Message = m.id
	}
	writeJSON(w, status, b)
}

// writeAppError maps journey errors; anything unknown is a 500 without detail.
func writeAppError(w http.ResponseWriter, r *http.Request, err error) {
	var e *journey.Error
	if errors.As(err, &e) {
		status := e.Status
		if status == 0 {
			status = http.StatusBadRequest
		}
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "60")
		}
		writeError(w, r, status, e.Code, e.Field)
		return
	}
	writeError(w, r, http.StatusInternalServerError, "internal", "")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeData(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, map[string]any{"data": v})
}
