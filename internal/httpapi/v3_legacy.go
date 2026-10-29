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
	key := ""
	if s.opt.Cache != nil {
		key = s.opt.Cache.Key("help-content", fixtures.Version, "v1")
		var cached fixtures.LegacyContent
		if ok, err := s.opt.Cache.GetJSON(r.Context(), key, &cached); ok {
			writeData(w, http.StatusOK, &cached)
			return
		} else if err != nil {
			s.log.Warn("help_content_cache_invalid", "error", err)
		}
	}
	c, err := fixtures.LoadLegacyContent()
	if err != nil {
		s.log.Error("help_content_unavailable", "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal", "")
		return
	}
	if key != "" {
		if err := s.opt.Cache.SetJSON(r.Context(), key, c, 0); err != nil {
			s.log.Warn("help_content_cache_write", "error", err)
		}
	}
	writeData(w, http.StatusOK, c)
}
