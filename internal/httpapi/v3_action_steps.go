package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"github.com/YanIanZ/sela/internal/budget"
	"github.com/YanIanZ/sela/internal/safety"
)

// Personal Action Bridge (BW-3, "Satu langkah, pilihanku"). The owner
// writes one small step in their own words, or asks Meta for one tentative
// proposal grounded in one saved source they selected. A proposal is never
// stored: only the owner's explicit save (POST /action-steps) persists an
// action, and the store checks the server-issued proposal token before it
// records a Meta origin.

// AI states of the suggestion route. The UI shows the manual path for every
// state but released.
const (
	actionSuggestReleased    = "released"
	actionSuggestNoInput     = "no_input"
	actionSuggestCrisis      = "crisis"
	actionSuggestStale       = "stale"
	actionSuggestUnavailable = "unavailable"
	actionSuggestBudget      = "budget_limited"
)

// actionExcerptRunes bounds the minimised excerpt of a note or check-in;
// a conversation sends at most three of the owner's newest turns.
const (
	actionExcerptRunes = 1600
	actionExcerptTurns = 3
)

// actionOverreach is the deterministic post-check of a released proposal:
// it must stay an everyday, self-chosen step. Treatment, medication,
// diagnosis, referral, emergency or hospital wording, and any tracking or
// pressure language (streaks, scores, reminders) withhold the proposal; the
// manual path stays open.
var actionOverreach = regexp.MustCompile(`(?i)(obat|medikasi|dosis|terapi|terapis|psikolog|psikiater|dokter|diagnos|darurat|rumah sakit|\bigd\b|hotline|\b11[029]\b|` +
	`medicat|medicine|dose|therap|psycholog|psychiatr|doctor|emergency|hospital|` +
	`streak|skor|\bpoin|score|points|badge|lencana|pengingat|reminder|setiap hari|every day|jangan lupa|don't forget|wajib|harus|must\b)`)

func (s *Server) v3ActionSteps(w http.ResponseWriter, r *http.Request, p account.Principal) {
	steps, err := s.opt.Accounts.ActionSteps(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"actionSteps": steps})
}

func (s *Server) v3ActionStep(w http.ResponseWriter, r *http.Request, p account.Principal) {
	step, err := s.opt.Accounts.ActionStep(r.Context(), p.UserID, r.PathValue("id"))
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, step)
}

func (s *Server) v3CreateActionStep(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.ActionStepInput
	if !decode(w, r, &in) {
		return
	}
	step, err := s.opt.Accounts.CreateActionStep(r.Context(), p.UserID, in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusCreated, step)
}

func (s *Server) v3UpdateActionStep(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in account.ActionStepPatch
	if !decode(w, r, &in) {
		return
	}
	step, err := s.opt.Accounts.UpdateActionStep(r.Context(), p.UserID, r.PathValue("id"), in)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, step)
}

func (s *Server) v3DeleteActionStep(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		ExpectedVersion int `json:"expectedVersion"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := s.opt.Accounts.DeleteActionStep(r.Context(), p.UserID, r.PathValue("id"), in.ExpectedVersion); err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

// actionSuggestion is a released proposal: never stored, carried back by
// the client with its token only if the owner accepts it.
type actionSuggestion struct {
	Text             string               `json:"text"`
	Token            string               `json:"token"`
	SourceRef        account.SourceRef    `json:"sourceRef"`
	Origin           account.ActionOrigin `json:"origin"`
	Tentative        bool                 `json:"tentative"`
	ExpiresInSeconds int                  `json:"expiresInSeconds"`
}

func suggestState(w http.ResponseWriter, state string) {
	writeData(w, http.StatusOK, map[string]any{"aiState": state, "proposal": nil, "help": state == actionSuggestCrisis})
}

// v3SuggestActionStep runs AT-7 over a minimised excerpt of the one saved
// source the owner selected. Order: the source belongs to the owner and is
// at the expected version; the deterministic crisis screen (crisis text gets
// the crisis response, never a productivity suggestion, and no provider
// call); the ai_processing consent; then the router, whose result is
// discarded if the session, consent or source version changed before
// release. Nothing is persisted.
func (s *Server) v3SuggestActionStep(w http.ResponseWriter, r *http.Request, p account.Principal) {
	var in struct {
		SourceRef *struct {
			RecordID string `json:"recordId"`
			Version  int    `json:"version"`
		} `json:"sourceRef"`
	}
	if !decode(w, r, &in) {
		return
	}
	var src account.PersonalRecord
	if in.SourceRef != nil {
		rec, err := s.opt.Accounts.Record(r.Context(), p.UserID, in.SourceRef.RecordID)
		if err != nil {
			s.v3Error(w, r, err)
			return
		}
		if rec.Kind != "note" && rec.Kind != "checkin" && rec.Kind != "conversation" {
			writeError(w, r, http.StatusNotFound, "not_found", "")
			return
		}
		if rec.Version != in.SourceRef.Version {
			writeError(w, r, http.StatusConflict, "stale_version", "sourceRef")
			return
		}
		// Safety first: a saved crisis pause or crisis wording anywhere in
		// the source gets the help response, before consent or any call.
		if rec.Paused || safety.Assess(actionSourceText(rec)).Level == safety.LevelCrisis {
			suggestState(w, actionSuggestCrisis)
			return
		}
		src = rec
	}
	consents, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	granted := false
	for _, c := range consents {
		if c.Purpose == string(ai.PurposeAIProcessing) && c.Granted && !c.Outdated {
			granted = true
		}
	}
	if !granted {
		writeError(w, r, http.StatusForbidden, "consent_required", "aiProcessing")
		return
	}
	spans := actionSpans(src)
	if in.SourceRef == nil || len(spans) == 0 {
		// No selected source, nothing to send: the model receives no
		// personal excerpt and the owner is offered the manual path.
		suggestState(w, actionSuggestNoInput)
		return
	}
	if s.opt.AIRouter == nil {
		suggestState(w, actionSuggestUnavailable)
		return
	}
	profile, err := s.opt.Accounts.Profile(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	var reservation budget.Reservation
	if s.opt.Budget != nil && s.opt.Budget.Enabled() {
		res, admErr := s.opt.Budget.Admit(r.Context(), string(ai.TaskActionSuggestion), s.v3.cfg.Synthetic())
		if admErr != nil {
			if errors.Is(admErr, budget.ErrExceeded) {
				suggestState(w, actionSuggestBudget)
			} else {
				suggestState(w, actionSuggestUnavailable)
			}
			return
		}
		reservation = res
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	version := src.Version
	output, err := s.opt.AIRouter.Run(ctx, ai.TaskActionSuggestion, ai.Input{
		Subject: ai.Subject(p.UserID.String()), Language: profile.Locale, Spans: spans,
		Synthetic: s.v3.cfg.Synthetic(), PrivateNames: s.privateNamesFor(ctx, p.UserID, string(ai.TaskActionSuggestion)),
		Commit: func(ctx context.Context) error {
			principal, e := s.opt.Accounts.Authenticate(ctx, s.authToken(r))
			if e != nil || principal.UserID != p.UserID {
				return errors.New("session ended")
			}
			live, e := s.opt.Accounts.Record(ctx, p.UserID, src.ID)
			if e != nil || live.Version != version || live.Paused {
				return errors.New("stale source")
			}
			return nil
		},
	})
	s.settleSuggestion(r, reservation, output, err)
	if err != nil {
		// The router's error text is a content-free outcome/reason code.
		s.log.Warn("ai_run_failed", "task", string(ai.TaskActionSuggestion), "error", err.Error())
		var re *ai.RunError
		declinedOnly := errors.As(err, &re) && slices.Equal(re.Reasons, []string{ai.ReasonDeclined})
		switch ai.OutcomeOf(err) {
		case ai.OutcomeSafeStop:
			// The model declining for lack of a safe practical step is the
			// manual path; a crisis risk or a verifier safety signal is help.
			if declinedOnly {
				suggestState(w, actionSuggestUnavailable)
			} else {
				suggestState(w, actionSuggestCrisis)
			}
		case ai.OutcomeCrisisHold:
			suggestState(w, actionSuggestCrisis)
		case ai.OutcomeStale:
			suggestState(w, actionSuggestStale)
		default:
			suggestState(w, actionSuggestUnavailable)
		}
		return
	}
	text := ""
	for _, item := range output.Items {
		if item.Source == ai.SourceAIWording {
			text = strings.TrimSpace(restoreProposalText(output.Restore, item.Text))
			break
		}
	}
	if text == "" || utf8.RuneCountInString(text) > account.ActionTextMax || actionOverreach.MatchString(text) ||
		safety.Assess(text).Level != safety.LevelNone {
		suggestState(w, actionSuggestUnavailable)
		return
	}
	origin := account.ActionOrigin{Kind: account.ActionOriginMeta, Draft: output.Provenance.DraftProvenance}
	if g := output.Provenance.Generator; g != nil {
		origin.Model = g.ModelID
	}
	ref := account.SourceRef{RecordID: src.ID, Version: version}
	token, err := s.opt.Accounts.SignActionProposal(r.Context(), p.UserID, account.ActionProposal{
		Text: text, SourceRef: &ref, Model: origin.Model, Draft: origin.Draft,
	})
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"aiState": actionSuggestReleased, "help": output.SupportInfo, "proposal": actionSuggestion{
		Text: text, Token: token, SourceRef: ref, Origin: origin, Tentative: true,
		ExpiresInSeconds: int(account.ActionProposalTTL / time.Second),
	}})
}

func (s *Server) settleSuggestion(r *http.Request, reservation budget.Reservation, output ai.Output, err error) {
	if !reservation.Admitted() {
		return
	}
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
	defer cancel()
	settlement := budget.Settlement{ActualUSD: reservation.ReservedUSD, Outcome: "failed"}
	if err == nil {
		settlement = budget.Settlement{
			ActualUSD: output.Provenance.EstimatedCostUSD, InputTokens: output.Provenance.InputTokens,
			OutputTokens: output.Provenance.OutputTokens, Outcome: string(output.Provenance.Outcome),
		}
		if output.Provenance.Generator != nil {
			settlement.Provider = string(output.Provenance.Generator.Provider)
		}
	}
	if settleErr := s.opt.Budget.Settle(settleCtx, reservation, settlement); settleErr != nil {
		s.log.Warn("ai_budget_settle_failed", "error", settleErr)
	}
}

// actionSourceText is everything the owner wrote in the source, for the
// crisis screen.
func actionSourceText(r account.PersonalRecord) string {
	if r.Kind == "conversation" {
		var parts []string
		for _, t := range r.Turns {
			if t.Role == "user" {
				parts = append(parts, t.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return r.Title + "\n" + r.Text
}

// actionSpans is the minimised excerpt (DM-1): for a conversation the
// owner's newest turns that fit, for a note or check-in its text cut at a
// word boundary. Sela's own replies are never sent.
func actionSpans(r account.PersonalRecord) []ai.Span {
	if r.ID == "" {
		return nil
	}
	if r.Kind == "conversation" {
		var spans []ai.Span
		for i := len(r.Turns) - 1; i >= 0 && len(spans) < actionExcerptTurns; i-- {
			t := r.Turns[i]
			n := utf8.RuneCountInString(strings.TrimSpace(t.Text))
			if t.Role != "user" || n == 0 || n > actionExcerptRunes {
				continue
			}
			spans = append([]ai.Span{{Key: "turn", Role: ai.RoleUser, Text: t.Text}}, spans...)
		}
		for i := range spans {
			spans[i].Key = "turn-" + string(rune('a'+i))
		}
		return spans
	}
	text := strings.TrimSpace(r.Text)
	if text == "" {
		return nil
	}
	if runes := []rune(text); len(runes) > actionExcerptRunes {
		cut := string(runes[:actionExcerptRunes])
		if i := strings.LastIndexAny(cut, " \n\t"); i > actionExcerptRunes/2 {
			cut = cut[:i]
		}
		text = cut
	}
	return []ai.Span{{Key: "source", Role: ai.RoleUser, Text: text}}
}
