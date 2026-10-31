package account

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// RequireGuardianConsent is the server-side guardian gate (D-031). Content
// writes call it before they change anything: an owner in the stored 15_17
// band whose guardian consent is missing or withdrawn is refused while
// SELA_GUARDIAN_CONSENT_REQUIRED is on (the default). 18_plus and unknown
// owners are unaffected, and an explicit 15 band (under_16, input only,
// never stored) is refused with age_not_supported.
//
// The check reads the database rather than a profile copy, so a withdrawal
// takes effect on the next write. It is one method used by every gated
// /api/v3 route, never by the UI alone.
func (s *Store) RequireGuardianConsent(ctx context.Context, owner UserID) error {
	var band string
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT u.age_band,
		EXISTS (SELECT 1 FROM guardian_consents g WHERE g.user_id = u.id AND g.withdrawn_at IS NULL)
		FROM users u WHERE u.id = $1`, owner).Scan(&band, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	switch band {
	case AgeBand18Plus, AgeBandUnknown:
		return nil
	case AgeBand15to17:
		if s.cfg.GuardianConsentRequired && !active {
			return ErrGuardianConsentRequired
		}
		return nil
	case AgeBandUnder15, AgeBandUnder16:
		// Accepted as input only and never stored; kept here as defence in
		// depth for a row that predates the input rejection.
		return ErrAgeNotSupported
	default:
		return nil
	}
}
