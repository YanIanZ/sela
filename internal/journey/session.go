package journey

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// CreateInput starts a session after the user has read the boundaries.
type CreateInput struct {
	Language      string   `json:"language"`
	Identity      Identity `json:"identity"`
	AdultAttested bool     `json:"adultAttested"`
	// PrivacyPolicy is the privacy/boundaries notice version the user saw.
	PrivacyPolicy string `json:"privacyPolicy"`
}

// CreateSession creates an isolated session and returns its opaque id. The
// id is only ever sent as an HttpOnly cookie.
func (s *Service) CreateSession(in CreateInput) (string, *SessionView, error) {
	if in.Language != "id" && in.Language != "en" {
		return "", nil, errf(CodeInvalid, 400, "language")
	}
	if !in.AdultAttested {
		return "", nil, errf(CodeAdultRequired, 400, "adultAttested")
	}
	if in.PrivacyPolicy != s.policy("privacy") {
		return "", nil, errf(CodePolicyOutdated, 409, "privacyPolicy")
	}
	id, err := validIdentity(in.Identity)
	if err != nil {
		return "", nil, err
	}
	now := s.now()
	sess := &Session{
		ID:            randomToken(32),
		Version:       1,
		CreatedAt:     now,
		LastSeen:      now,
		ExpiresAt:     now.Add(SessionMaxLife),
		Language:      in.Language,
		Identity:      id,
		AdultAttested: true,
		PolicyVersion: in.PrivacyPolicy,
		Consents:      map[ConsentKind]Consent{},
		Retention:     RetentionSession,
		Safety:        SafetyState{Level: "none"},
		Handoffs:      []*Handoff{},
		CheckIns:      []CheckIn{},
		Ops:           []*OpRecord{},
		idem:          map[string]idemRecord{},
	}
	if err := s.store.Create(sess); err != nil {
		return "", nil, errf(CodeCapacity, 503, "")
	}
	s.metrics.add("session_created")
	var view *SessionView
	err = s.view(sess.ID, func(x *Session) error { view = s.buildView(x); return nil })
	return sess.ID, view, err
}

var pseudonymPattern = regexp.MustCompile(`^[\p{L}][\p{L} .'\-]{0,22}[\p{L}.]$|^[\p{L}]$`)

func validIdentity(in Identity) (Identity, error) {
	switch in.Mode {
	case IdentityAnonymous, "":
		return Identity{Mode: IdentityAnonymous}, nil
	case IdentityPseudonym:
		p := strings.Join(strings.Fields(in.Pseudonym), " ")
		if utf8.RuneCountInString(p) < 2 || utf8.RuneCountInString(p) > 24 || !pseudonymPattern.MatchString(p) {
			return Identity{}, errf(CodeInvalid, 400, "identity.pseudonym")
		}
		return Identity{Mode: IdentityPseudonym, Pseudonym: p}, nil
	default:
		return Identity{}, errf(CodeInvalid, 400, "identity.mode")
	}
}

// State returns the user-visible state of the session.
func (s *Service) State(sessionID string) (*SessionView, error) {
	var view *SessionView
	err := s.view(sessionID, func(sess *Session) error {
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// SettingsInput changes language or identity.
type SettingsInput struct {
	ExpectedVersion int       `json:"expectedVersion"`
	Language        *string   `json:"language,omitempty"`
	Identity        *Identity `json:"identity,omitempty"`
}

// UpdateSettings changes language and/or identity. Language only changes the
// interface: quotes stay in the user's words and approval is unaffected.
// Identity changes do not alter existing shares (their projections are
// immutable); new shares use the new identity.
func (s *Service) UpdateSettings(sessionID string, in SettingsInput) (*SessionView, error) {
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if in.Language != nil {
			if *in.Language != "id" && *in.Language != "en" {
				return errf(CodeInvalid, 400, "language")
			}
			sess.Language = *in.Language
		}
		if in.Identity != nil {
			id, err := validIdentity(*in.Identity)
			if err != nil {
				return err
			}
			sess.Identity = id
		}
		bump(sess)
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// ConsentInput records one decision.
type ConsentInput struct {
	ExpectedVersion int    `json:"expectedVersion"`
	Granted         bool   `json:"granted"`
	PolicyVersion   string `json:"policyVersion"`
}

// SetConsent records an AI-processing or storage decision. Each is
// independent (SPEC V4): neither implies approval, sharing or the other.
// Withdrawing AI consent cancels running AI operations and discards their
// results. Granting storage extends retention to 24 hours from now;
// withdrawing it returns to session-only retention immediately.
func (s *Service) SetConsent(sessionID string, kind ConsentKind, in ConsentInput) (*SessionView, error) {
	policyKey := map[ConsentKind]string{ConsentAI: "ai_processing", ConsentStorage: "storage"}[kind]
	if policyKey == "" {
		return nil, errf(CodeNotFound, 404, "kind")
	}
	var view *SessionView
	err := s.update(sessionID, func(sess *Session) error {
		if err := checkVersion(sess, in.ExpectedVersion); err != nil {
			return err
		}
		if in.Granted && in.PolicyVersion != s.policy(policyKey) {
			return errf(CodePolicyOutdated, 409, "policyVersion")
		}
		now := s.now()
		prev := sess.consent(kind)
		sess.Consents[kind] = Consent{Granted: in.Granted, PolicyVersion: in.PolicyVersion, DecidedAt: now, Revision: prev.Revision + 1}
		switch kind {
		case ConsentAI:
			if !in.Granted {
				s.cancelSessionOps(sess, "consent_withdrawn")
			}
		case ConsentStorage:
			if in.Granted {
				sess.Retention = RetentionDay
				sess.ExpiresAt = now.Add(DayMaxLife)
			} else {
				sess.Retention = RetentionSession
				limit := sess.CreatedAt.Add(SessionMaxLife)
				if limit.Before(now.Add(SessionIdle)) {
					limit = now.Add(SessionIdle)
				}
				sess.ExpiresAt = limit
			}
			// Shares never outlive the session.
			for _, h := range sess.Handoffs {
				if h.ExpiresAt.After(sess.ExpiresAt) {
					h.ExpiresAt = sess.ExpiresAt
				}
			}
		}
		bump(sess)
		s.metrics.add("consent_" + string(kind) + map[bool]string{true: "_granted", false: "_declined"}[in.Granted])
		view = s.buildView(sess)
		return nil
	})
	return view, err
}

// DeleteSession purges everything for the session: story, ledger, shares,
// recipient access and operations. Other sessions are unaffected.
func (s *Service) DeleteSession(sessionID string) error {
	err := s.update(sessionID, func(sess *Session) error {
		s.cancelSessionOps(sess, "deleted")
		return nil
	})
	if err != nil {
		return err
	}
	s.store.Delete(sessionID)
	s.metrics.add("session_deleted")
	return nil
}

// Sweep purges expired sessions; run periodically.
func (s *Service) Sweep() int {
	n := s.store.Sweep(s.now())
	s.reapOps(s.now())
	return n
}

// containsControl reports control characters other than newline/tab.
func containsControl(t string) bool {
	for _, r := range t {
		if unicode.IsControl(r) && r != '\n' && r != '\t' && r != '\r' {
			return true
		}
	}
	return false
}

// Contact-detail heuristics for practice text. They are deliberately broad:
// the demo must not become a place to store real contact data.
var (
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phonePattern = regexp.MustCompile(`(?:\+?\d[\d\s\-().]{8,}\d)`)
	nikPattern   = regexp.MustCompile(`\b\d{16}\b`)
	urlPattern   = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.`)
)

func containsContactDetails(t string) bool {
	if emailPattern.MatchString(t) || nikPattern.MatchString(t) || urlPattern.MatchString(t) {
		return true
	}
	// A phone-like run needs at least 9 digits, so dates and times pass.
	for _, m := range phonePattern.FindAllString(t, -1) {
		digits := 0
		for _, r := range m {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits >= 9 {
			return true
		}
	}
	return false
}

func (s *Service) sessionExpiry(sess *Session) time.Time { return sess.ExpiresAt }
