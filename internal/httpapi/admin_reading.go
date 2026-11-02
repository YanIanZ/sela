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
	writeData(w, http.StatusOK, map[string]any{
		"items": s.v3.readingLib.AdminItems(bySlug),
		// The rule set and the checklist the reviewer must confirm. The
		// approve request echoes both versions back.
		"accessibility": map[string]any{
			"checksVersion":   reading.A11yChecksVersion,
			"manualVersion":   reading.A11yManualVersion,
			"manualChecklist": reading.ManualChecklist,
			"overrideReason":  map[string]int{"min": reading.A11yOverrideMin, "max": reading.A11yOverrideMax},
		},
	})
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
		// Accessibility is required: see reading.GateAccessibility.
		Accessibility *reading.A11yRequest `json:"accessibility"`
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
	report, currentHash, ok := s.v3.readingLib.Accessibility(slug)
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
	// Accessibility is checked against the report computed from the same
	// embedded bytes as currentHash, never against anything the client says
	// about the checks.
	a11y, err := reading.GateAccessibility(report, req.Accessibility)
	if err != nil {
		writeA11yGateError(w, r, err)
		return
	}
	approval, err := s.opt.ReadingReviews.Approve(r.Context(), p.AdminID, slug, reading.ApproveInput{
		ReviewerName:   req.ReviewerName,
		ReviewedOn:     reviewedOn,
		Note:           req.Note,
		VerifierStatus: req.VerifierStatus,
		ContentHash:    currentHash,
		Accessibility:  &a11y,
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

// writeA11yGateError maps an accessibility refusal to its stable code.
func writeA11yGateError(w http.ResponseWriter, r *http.Request, err error) {
	field := "accessibility"
	var ge *reading.A11yGateError
	if errors.As(err, &ge) {
		field = ge.Field
	}
	switch {
	case errors.Is(err, reading.ErrA11yVersion):
		writeError(w, r, http.StatusConflict, "accessibility_checks_changed", field)
	case errors.Is(err, reading.ErrA11yBlocked):
		writeError(w, r, http.StatusUnprocessableEntity, "accessibility_checks_failed", field)
	case errors.Is(err, reading.ErrA11yOverrideRequired):
		writeError(w, r, http.StatusUnprocessableEntity, "accessibility_override_required", field)
	case errors.Is(err, reading.ErrA11yChecklist):
		writeError(w, r, http.StatusBadRequest, "accessibility_checklist_incomplete", field)
	default:
		writeError(w, r, http.StatusBadRequest, "invalid_input", field)
	}
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
