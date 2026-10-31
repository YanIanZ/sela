package httpapi

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/safety"
)

// The memory proposal route (AT-3) only reads saved conversation records,
// never a live unsaved hold (DM-4, V27). These are the AI states the UI
// distinguishes: a released run may legitimately propose nothing.
const (
	memoryProposalReleased    = "released"
	memoryProposalNoInput     = "no_input"
	memoryProposalUnavailable = "unavailable"
)

// v3CreateMemoryProposals runs AT-3 over the recent user turns of one saved
// conversation. It requires both the personal_memory and ai_processing
// consents, the same owner, and a current conversation version at commit.
// Only AI wording the router released is stored: withheld, dropped and
// quote-fallback items never become proposals.
func (s *Server) v3CreateMemoryProposals(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ConversationID  string `json:"conversationId"`
		ExpectedVersion int    `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	if strings.TrimSpace(in.ConversationID) == "" {
		writeError(w, r, 400, "invalid_input", "conversationId")
		return
	}
	conv, err := s.opt.Accounts.Record(r.Context(), p.UserID, in.ConversationID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	if conv.Kind != "conversation" {
		writeError(w, r, 404, "not_found", "")
		return
	}
	if in.ExpectedVersion > 0 && conv.Version != in.ExpectedVersion {
		writeError(w, r, 409, "stale_version", "")
		return
	}
	if conv.Paused {
		// A saved crisis pause stays in force until the owner explicitly
		// continues; no proposal is generated from it.
		writeError(w, r, 409, "invalid_transition", "")
		return
	}
	consents, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	granted := map[string]bool{}
	for _, c := range consents {
		if c.Granted && !c.Outdated {
			granted[c.Purpose] = true
		}
	}
	if !granted[account.PolicyPersonalMemory] {
		writeError(w, r, 403, "consent_required", "personalMemory")
		return
	}
	if !granted[string(ai.PurposeAIProcessing)] {
		writeError(w, r, 403, "consent_required", "aiProcessing")
		return
	}
	if s.opt.AIRouter == nil {
		writeError(w, r, 503, "store_unavailable", "")
		return
	}
	// Confirmed memory is context for de-duplication only (AT-3 §3.1); a
	// failed read never adds proposals, so it is not fatal.
	var memories []account.PersonalRecord
	if view, memErr := s.opt.Accounts.Memory(r.Context(), p.UserID); memErr != nil {
		s.log.Warn("ai_memory_context_failed", "task", "memory_proposal", "error", memErr.Error())
	} else if view.Enabled {
		memories = view.Items
	}
	spans, quotes := memoryProposalSpans(conv.Turns, memories)
	if quotes == nil {
		writeData(w, http.StatusOK, map[string]any{"proposals": []account.PersonalRecord{}, "aiState": memoryProposalNoInput})
		return
	}
	profile, err := s.opt.Accounts.Profile(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	version := conv.Version
	output, err := s.opt.AIRouter.Run(r.Context(), ai.TaskMemoryProposal, ai.Input{
		Subject: ai.Subject(p.UserID.String()), Language: profile.Locale, Spans: spans,
		Synthetic: s.v3.cfg.Synthetic(),
		Commit: func(ctx context.Context) error {
			principal, e := s.opt.Accounts.Authenticate(ctx, s.authToken(r))
			if e != nil || principal.UserID != p.UserID {
				return errors.New("session ended")
			}
			live, e := s.opt.Accounts.Record(ctx, p.UserID, conv.ID)
			if e != nil || live.Kind != "conversation" || live.Version != version {
				return errors.New("stale conversation")
			}
			return nil
		},
	})
	if err != nil {
		// The router's error text is a content-free outcome/reason code.
		s.log.Warn("ai_run_failed", "task", "memory_proposal", "error", err.Error())
		writeData(w, http.StatusOK, map[string]any{"proposals": []account.PersonalRecord{}, "aiState": memoryProposalUnavailable})
		return
	}
	proposals := make([]account.MemoryProposalInput, 0, len(output.Items))
	for _, item := range output.Items {
		if item.Source != ai.SourceAIWording || !item.Verified {
			// A quote fallback is the user's own words, not an
			// interpretation; it is never relabelled as a proposal. An
			// unchecked item cannot exist for AT-3, but is never stored.
			continue
		}
		text := strings.TrimSpace(restoreProposalText(output.Restore, item.Text))
		if text == "" {
			continue
		}
		quote, turnKey := "", ""
		for _, key := range item.SpanKeys {
			if q, ok := quotes[key]; ok {
				quote, turnKey = q, key
				break
			}
		}
		if quote == "" {
			continue
		}
		proposals = append(proposals, account.MemoryProposalInput{
			Text: text,
			SourceRefs: []account.SourceRef{
				{RecordID: conv.ID, Version: version, Quote: quote, TurnKey: turnKey},
			},
		})
	}
	if len(proposals) == 0 {
		writeData(w, http.StatusOK, map[string]any{"proposals": []account.PersonalRecord{}, "aiState": memoryProposalReleased})
		return
	}
	records, err := s.opt.Accounts.CreateMemoryProposals(r.Context(), p.UserID, proposals)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"proposals": records, "aiState": memoryProposalReleased})
}

// v3ReviewMemory applies confirm, correct or reject to one proposal. The
// store enforces ownership, the expected version and the memory consent.
func (s *Server) v3ReviewMemory(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int    `json:"expectedVersion"`
		Action          string `json:"action"`
		Text            string `json:"text"`
	}
	if !decode(w, r, &in) {
		return
	}
	record, err := s.opt.Accounts.ReviewMemory(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion, in.Action, in.Text)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, record)
}

// memoryProposalSpans builds the AT-3 payload from the newest user turns
// that fit the task limits, then confirmed/corrected memory for
// de-duplication. A turn above the span cap is skipped, never cut. The map
// returned holds the caller keys of the selected user turns and their
// original (unredacted) text, so a released item can carry the owner's
// exact quote. A nil map means no user turn can be sent.
func memoryProposalSpans(turns []account.ConversationTurn, items []account.PersonalRecord) ([]ai.Span, map[string]string) {
	pol, ok := ai.PolicyFor(ai.TaskMemoryProposal)
	if !ok {
		return nil, nil
	}
	type picked struct {
		key, text string
		runes     int
	}
	var newest []picked
	total := 0
	for i := len(turns) - 1; i >= 0 && len(newest) < 12; i-- {
		t := turns[i]
		if t.Role != "user" || safety.Assess(t.Text).Level == safety.LevelCrisis {
			continue
		}
		n := utf8.RuneCountInString(t.Text)
		if n == 0 || (pol.MaxSpanRunes > 0 && n > pol.MaxSpanRunes) {
			continue
		}
		newest = append(newest, picked{key: "turn-" + strconv.Itoa(i), text: t.Text, runes: n})
		total += n
	}
	for len(newest) > 0 && pol.MaxTotalRunes > 0 && total > pol.MaxTotalRunes {
		total -= newest[len(newest)-1].runes
		newest = newest[:len(newest)-1]
	}
	if len(newest) == 0 {
		return nil, nil
	}
	slices.Reverse(newest)
	quotes := make(map[string]string, len(newest))
	turnSpans := make([]ai.Span, 0, len(newest))
	for _, t := range newest {
		quotes[t.key] = t.text
		turnSpans = append(turnSpans, ai.Span{Key: t.key, Role: ai.RoleUser, Text: t.text})
	}
	maxMemory := 10
	if lim, ok := pol.RoleLimits[ai.RoleMemory]; ok && lim.MaxSpans > 0 {
		maxMemory = lim.MaxSpans
	}
	memorySpans := []ai.Span{}
	for _, r := range items {
		if len(memorySpans) >= maxMemory || len(memorySpans)+len(turnSpans) >= pol.MaxSpans {
			break
		}
		if r.ReviewStatus != account.ReviewConfirmed && r.ReviewStatus != account.ReviewCorrected {
			continue
		}
		text := strings.TrimSpace(r.Text)
		if title := strings.TrimSpace(r.Title); title != "" {
			text = title + "\n" + text
		}
		n := utf8.RuneCountInString(text)
		if n == 0 || (pol.MaxSpanRunes > 0 && n > pol.MaxSpanRunes) ||
			(pol.MaxTotalRunes > 0 && total+n > pol.MaxTotalRunes) {
			continue
		}
		memorySpans = append(memorySpans, ai.Span{Key: "memory-" + strconv.Itoa(len(memorySpans)), Role: ai.RoleMemory, Text: text})
		total += n
	}
	return append(memorySpans, turnSpans...), quotes
}

// restoreProposalText maps the owner's redacted names and contacts back
// into released AI wording before it is stored in the owner's sealed
// record. The mapping never leaves Sela.
func restoreProposalText(restore map[string]string, text string) string {
	if len(restore) == 0 {
		return text
	}
	pairs := make([]string, 0, len(restore)*2)
	for placeholder, value := range restore {
		pairs = append(pairs, placeholder, value)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}
