package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/YanIanZ/sela/internal/admin"
	"github.com/YanIanZ/sela/internal/reading"
)

func (s *Server) adminReadingRoutes() {
	m := s.mux
	m.HandleFunc("GET /api/admin/v1/reading", s.read(s.withAdmin(s.adminReadingList)))
	m.HandleFunc("GET /api/admin/v1/reading/audit", s.read(s.withAdmin(s.adminReadingAudit)))
	m.HandleFunc("POST /api/admin/v1/reading/{slug}/approve", s.mutation("mutation", smallBody, s.withAdmin(s.adminReadingApprove)))
	m.HandleFunc("POST /api/admin/v1/reading/{slug}/revoke", s.mutation("mutation", smallBody, s.withAdmin(s.adminReadingRevoke)))
}

// canReadingReview accepts either editorial reviewer permission. The reading
// library is support content, so a safety reviewer and a research editor may
// both inspect it; every mutation is checked again server-side.
func canReadingReview(p admin.Principal) bool {
	return p.Can(admin.PermResearchReview) || p.Can(admin.PermSafetyReview)
}

func (s *Server) readingReviewReady(w http.ResponseWriter, r *http.Request) bool {
	if s.opt.ReadingReviews == nil {
		writeError(w, r, http.StatusServiceUnavailable, "store_unavailable", "")
		return false
	}
	return true
}

func (s *Server) adminReadingList(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !canReadingReview(p) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.readingReviewReady(w, r) {
		return
	}
	rows, err := s.opt.ReadingReviews.List(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	bySlug := make(map[string]reading.Approval, len(rows))
	for _, row := range rows {
		bySlug[row.Slug] = row
	}
	writeData(w, http.StatusOK, map[string]any{"items": s.v3.readingLib.AdminItems(bySlug)})
}

func (s *Server) adminReadingApprove(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !canReadingReview(p) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.readingReviewReady(w, r) {
		return
	}
	var req struct {
		ReviewerName   string `json:"reviewerName"`
		ReviewedOn     string `json:"reviewedOn"`
		Note           string `json:"note"`
		VerifierStatus string `json:"verifierStatus"`
		ContentHash    string `json:"contentHash"`
		Confirm        bool   `json:"confirm"`
	}
	if !decode(w, r, &req) {
		return
	}
	// The confirmation is the operator's assertion that they read the exact
	// current text. It is never inferred from a role or a previous approval.
	if !req.Confirm {
		writeError(w, r, http.StatusBadRequest, "confirmation_required", "confirm")
		return
	}
	if req.VerifierStatus != "supported" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "verifierStatus")
		return
	}
	slug := r.PathValue("slug")
	currentHash, ok := s.v3.readingLib.ContentHash(slug)
	if !ok {
		writeError(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	// When the client sends the hash it previewed, a changed item is refused
	// rather than silently approving different text. An empty hash binds the
	// approval to the hash read in this request.
	if req.ContentHash != "" && req.ContentHash != currentHash {
		writeError(w, r, http.StatusConflict, "reading_content_changed", "")
		return
	}
	reviewedOn, err := time.Parse(time.DateOnly, req.ReviewedOn)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "reviewedOn")
		return
	}
	approval, err := s.opt.ReadingReviews.Approve(r.Context(), p.AdminID, slug, reading.ApproveInput{
		ReviewerName:   req.ReviewerName,
		ReviewedOn:     reviewedOn,
		Note:           req.Note,
		VerifierStatus: req.VerifierStatus,
		ContentHash:    currentHash,
	})
	if err != nil {
		if errors.Is(err, reading.ErrReviewInvalid) {
			writeError(w, r, http.StatusBadRequest, "invalid_input", "")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"slug": slug, "approval": approval})
}

func (s *Server) adminReadingRevoke(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !canReadingReview(p) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.readingReviewReady(w, r) {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decodeOptional(w, r, &req) {
		return
	}
	slug := r.PathValue("slug")
	if _, ok := s.v3.readingLib.ContentHash(slug); !ok {
		writeError(w, r, http.StatusNotFound, "not_found", "")
		return
	}
	approval, err := s.opt.ReadingReviews.Revoke(r.Context(), p.AdminID, slug, req.Reason)
	if err != nil {
		if errors.Is(err, reading.ErrReviewNotFound) {
			writeError(w, r, http.StatusConflict, "reading_not_approved", "")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"slug": slug, "approval": approval})
}

func (s *Server) adminReadingAudit(w http.ResponseWriter, r *http.Request, p admin.Principal) {
	if !canReadingReview(p) {
		writeError(w, r, http.StatusForbidden, "admin_forbidden", "")
		return
	}
	if !s.readingReviewReady(w, r) {
		return
	}
	events, err := s.opt.ReadingReviews.Audit(r.Context(), 50)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, map[string]any{"events": events})
}

// readingApprovals loads the current and revoked rows for the public gate.
// On a database error the embedded-only gate stays in force: a DB-approved
// item is not served on trust and a revocation is never guessed.
func (s *Server) readingApprovals(ctx context.Context) map[string]reading.Approval {
	if s.opt.ReadingReviews == nil {
		return nil
	}
	rows, err := s.opt.ReadingReviews.List(ctx)
	if err != nil {
		s.log.Warn("reading.approvals_unavailable")
		return nil
	}
	bySlug := make(map[string]reading.Approval, len(rows))
	for _, row := range rows {
		bySlug[row.Slug] = row
	}
	return bySlug
}
