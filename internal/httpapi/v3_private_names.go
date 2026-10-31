package httpapi

import (
	"context"
	"net/http"

	"github.com/YanIanZ/sela/internal/account"
)

// v3PrivateNames lists the owner's "names kept private" list. The list is
// owner-scoped and decrypted only for this response.
func (s *Server) v3PrivateNames(w http.ResponseWriter, r *http.Request, p account.Principal) {
	names, err := s.opt.Accounts.PrivateNames(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 200, map[string]any{"names": names})
}

// v3CreatePrivateName adds one name (1..120 runes, at most 20 names).
func (s *Server) v3CreatePrivateName(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	name, err := s.opt.Accounts.CreatePrivateName(r.Context(), p.UserID, in.Name)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, name)
}

// v3DeletePrivateName removes one owned name. Another owner's id and an
// unknown id are both 404 not_found.
func (s *Server) v3DeletePrivateName(w http.ResponseWriter, r *http.Request, p account.Principal) {
	if err := s.opt.Accounts.DeletePrivateName(r.Context(), p.UserID, r.PathValue("id")); err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, 200, map[string]bool{"deleted": true})
}

// privateNamesFor returns the owner's names for redaction before an AI
// call. A failed read never blocks the operation: it logs a content-free
// warning and returns nil, so the request continues with the deterministic
// contact-detail redaction only. No name is ever logged.
func (s *Server) privateNamesFor(ctx context.Context, owner account.UserID, task string) []string {
	if s.opt.Accounts == nil {
		return nil
	}
	names, err := s.opt.Accounts.PrivateNames(ctx, owner)
	if err != nil {
		s.log.Warn("ai_private_names_failed", "task", task, "error", err.Error())
		return nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, n.Name)
	}
	return out
}
