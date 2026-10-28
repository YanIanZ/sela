package account

import (
	"errors"
	"time"
)

// Error is a stable, content-free error code with an optional field and
// a retry hint. The HTTP layer maps codes to statuses and localized
// messages.
type Error struct {
	Code       string
	Field      string
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.Field != "" {
		return "account: " + e.Code + " (" + e.Field + ")"
	}
	return "account: " + e.Code
}

// Is matches by code, so errors.Is(err, ErrPasswordWeak) holds whatever
// the field.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// Error codes (API_SPEC_V3 §2).
var (
	ErrInvalidInput             = &Error{Code: "invalid_input"}
	ErrNotFound                 = &Error{Code: "not_found"}
	ErrNotSignedIn              = &Error{Code: "not_signed_in"}
	ErrInvalidCredentials       = &Error{Code: "invalid_credentials"}
	ErrRecoveryFailed           = &Error{Code: "recovery_failed"}
	ErrCurrentPasswordIncorrect = &Error{Code: "current_password_incorrect"}
	ErrReauthRequired           = &Error{Code: "reauth_required"}
	ErrAccountRequired          = &Error{Code: "account_required"}
	ErrPasswordLoginDisabled    = &Error{Code: "password_login_disabled"}
	ErrAgeNotSupported          = &Error{Code: "age_not_supported", Field: "ageBand"}
	ErrAgeBandNotEnabled        = &Error{Code: "age_band_not_enabled", Field: "ageBand"}
	ErrNotGuest                 = &Error{Code: "not_guest"}
	ErrRequiredConsent          = &Error{Code: "required_consent"}
	// ErrConsentRequired gates a feature whose own consent is off (here:
	// personal memory). It reuses the existing consent_required code and
	// message; it is deliberately not listed in Codes(), whose v3 entries
	// are checked against the API spec table by TestV3ErrorMessagesComplete.
	ErrConsentRequired   = &Error{Code: "consent_required", Field: "personalMemory"}
	ErrUsernameInvalid   = &Error{Code: "username_invalid", Field: "username"}
	ErrUsernameTaken     = &Error{Code: "username_taken", Field: "username"}
	ErrPasswordTooShort  = &Error{Code: "password_too_short"}
	ErrPasswordTooLong   = &Error{Code: "password_too_long"}
	ErrPasswordWeak      = &Error{Code: "password_weak"}
	ErrServerBusy        = &Error{Code: "server_busy", RetryAfter: 5 * time.Second}
	ErrPolicyOutdated    = &Error{Code: "policy_outdated"}
	ErrStaleVersion      = &Error{Code: "stale_version"}
	ErrInvalidTransition = &Error{Code: "invalid_transition"}
	ErrRateLimited       = &Error{Code: "rate_limited"}
)

func invalid(field string) *Error { return &Error{Code: ErrInvalidInput.Code, Field: field} }

func withField(e *Error, field string) *Error {
	c := *e
	c.Field = field
	return &c
}

func throttled(d time.Duration) *Error {
	if d < time.Second {
		d = time.Second
	}
	return &Error{Code: ErrRateLimited.Code, RetryAfter: d}
}

// CodeOf returns the stable code of an account error, "" otherwise.
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// Codes lists every code this package returns (for message tables).
func Codes() []string {
	all := []*Error{ErrInvalidInput, ErrNotFound, ErrNotSignedIn, ErrInvalidCredentials, ErrRecoveryFailed,
		ErrCurrentPasswordIncorrect, ErrReauthRequired, ErrAccountRequired, ErrPasswordLoginDisabled, ErrAgeNotSupported,
		ErrAgeBandNotEnabled, ErrNotGuest, ErrRequiredConsent, ErrUsernameInvalid, ErrUsernameTaken, ErrPasswordTooShort,
		ErrPasswordTooLong, ErrPasswordWeak, ErrServerBusy, ErrPolicyOutdated, ErrStaleVersion, ErrInvalidTransition, ErrRateLimited}
	out := make([]string, len(all))
	for i, e := range all {
		out[i] = e.Code
	}
	return out
}
