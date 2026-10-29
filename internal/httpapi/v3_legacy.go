package httpapi

import (
	"net/http"

	"github.com/YanIanZ/sela/internal/fixtures"
)

// v3HelpContent serves the reviewed legacy content that survived the v1
// runtime: the self-help card library, the source-cited help-destination
// checklists and the support-options directory. It is public content, not
// user data, and the model may only choose card ids from it.
func (s *Server) v3HelpContent(w http.ResponseWriter, r *http.Request) {
	c, err := fixtures.LoadLegacyContent()
	if err != nil {
		s.log.Error("help_content_unavailable", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	writeData(w, http.StatusOK, c)
}
