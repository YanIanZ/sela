package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/YanIanZ/sela/internal/account"
)

// Partner invitations (D-079). An operator invites a partner account from
// the console; only the SHA-256 of a one-time token is stored. The invitee
// opens the activation link, gets a fresh TOTP secret and sets their own
// password, so the inviting operator never learns either. Internal staff
// accounts stay out of band (D-061).

// InviteTTL is how long an activation link stays valid.
const InviteTTL = 72 * time.Hour

// maxActivationAttempts bounds wrong passwords or codes per invitation.
const maxActivationAttempts = 10

// Invitation errors.
var (
	ErrInviteInvalid = errors.New("admin: invitation invalid, used, revoked or expired")
	ErrUsernameTaken = errors.New("admin: username taken")
	ErrBadCode       = errors.New("admin: wrong authenticator code")
)

// InviteView is an invitation without its token or secret.
type InviteView struct {
	ID               string     `json:"id"`
	Username         string     `json:"username"`
	Role             Role       `json:"role"`
	CareOrgID        string     `json:"careOrgId"`
	DisplayName      string     `json:"displayName,omitempty"`
	ProfessionalType string     `json:"professionalType,omitempty"`
	LicenseNo        string     `json:"licenseNo,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	AcceptedAt       *time.Time `json:"acceptedAt,omitempty"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
	State            string     `json:"state"` // open | accepted | revoked | expired
}

// Activation is what the invitee sees while activating: who they will be
// and the TOTP secret to enrol, shown only to the holder of the link.
type Activation struct {
	Username    string `json:"username"`
	Role        Role   `json:"role"`
	OrgName     string `json:"orgName"`
	DisplayName string `json:"displayName,omitempty"`
	Secret      string `json:"secret"`
	OTPAuthURL  string `json:"otpauthUrl"`
}

func inviteTokenHash(token string) []byte {
	sum := sha256.Sum256([]byte("sela.admin-invite.v1:" + token))
	return sum[:]
}

func newInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func otpauthURL(username, secret string) string {
	return "otpauth://totp/Sela:" + username + "?secret=" + secret + "&issuer=Sela&algorithm=SHA1&digits=6&period=30"
}

func validPartner(in PartnerInput) (PartnerInput, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.LicenseNo = strings.TrimSpace(in.LicenseNo)
	if err := ValidateUsername(in.Username); err != nil {
		return in, err
	}
	if !IsPartnerRole(in.Role) || in.CareOrgID == "" {
		return in, ErrInvalid
	}
	if n := len([]rune(in.DisplayName)); in.DisplayName != "" && (n < 2 || n > 120) {
		return in, ErrInvalid
	}
	if in.Role == RolePractitioner {
		if in.DisplayName == "" || !ProfessionalTypes[in.ProfessionalType] {
			return in, ErrInvalid
		}
	} else {
		in.ProfessionalType = ""
	}
	if n := len([]rune(in.LicenseNo)); in.LicenseNo != "" && (n < 3 || n > 64) {
		return in, ErrInvalid
	}
	return in, nil
}

// CreateInvite records an invitation and returns the one-time token. The
// organisation must exist and not be suspended; the username must be free.
func (s *Store) CreateInvite(ctx context.Context, actor AdminID, in PartnerInput) (InviteView, string, error) {
	in, err := validPartner(in)
	if err != nil {
		return InviteView{}, "", err
	}
	token, err := newInviteToken()
	if err != nil {
		return InviteView{}, "", err
	}
	now := s.now()
	v := InviteView{ID: newAdminID().String(), Username: in.Username, Role: in.Role, CareOrgID: in.CareOrgID, DisplayName: in.DisplayName,
		ProfessionalType: in.ProfessionalType, LicenseNo: in.LicenseNo, CreatedAt: now, ExpiresAt: now.Add(InviteTTL), State: "open"}
	err = s.tx(ctx, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM care_orgs WHERE id::text=$1`, in.CareOrgID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalid
			}
			return err
		}
		if status == "suspended" {
			return ErrInvalid
		}
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM admin_users WHERE username=$1)
			OR EXISTS (SELECT 1 FROM admin_invites WHERE username=$1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > $2)`,
			in.Username, now).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return ErrUsernameTaken
		}
		// An expired open invitation for the same name no longer blocks it.
		if _, err := tx.Exec(ctx, `UPDATE admin_invites SET revoked_at=$2 WHERE username=$1 AND accepted_at IS NULL AND revoked_at IS NULL`, in.Username, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO admin_invites (id, admin_id, token_hash, username, role, care_org_id, display_name, professional_type,
			license_no, created_by, created_at, expires_at) VALUES ($1,$2,$3,$4,$5,$6::uuid,NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),$10,$11,$12)`,
			v.ID, newAdminID(), inviteTokenHash(token), in.Username, in.Role, in.CareOrgID, in.DisplayName, in.ProfessionalType, in.LicenseNo,
			actor, now, v.ExpiresAt); err != nil {
			return err
		}
		return s.audit(ctx, tx, &actor, "network.invite_create", "admin_invite", in.Username, "", "open", string(in.Role))
	})
	if err != nil {
		return InviteView{}, "", err
	}
	return v, token, nil
}

// ListInvites returns an organisation's invitations, newest first.
func (s *Store) ListInvites(ctx context.Context, orgID string) ([]InviteView, error) {
	now := s.now()
	rows, err := s.pool.Query(ctx, `SELECT id::text, username, role, care_org_id::text, COALESCE(display_name,''), COALESCE(professional_type,''),
		COALESCE(license_no,''), created_at, expires_at, accepted_at, revoked_at FROM admin_invites
		WHERE care_org_id::text=$1 ORDER BY created_at DESC LIMIT 100`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []InviteView{}
	for rows.Next() {
		var v InviteView
		if err := rows.Scan(&v.ID, &v.Username, &v.Role, &v.CareOrgID, &v.DisplayName, &v.ProfessionalType, &v.LicenseNo,
			&v.CreatedAt, &v.ExpiresAt, &v.AcceptedAt, &v.RevokedAt); err != nil {
			return nil, err
		}
		switch {
		case v.AcceptedAt != nil:
			v.State = "accepted"
		case v.RevokedAt != nil:
			v.State = "revoked"
		case !now.Before(v.ExpiresAt):
			v.State = "expired"
		default:
			v.State = "open"
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// RevokeInvite ends an open invitation.
func (s *Store) RevokeInvite(ctx context.Context, actor AdminID, id string) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		var username string
		err := tx.QueryRow(ctx, `UPDATE admin_invites SET revoked_at=$2 WHERE id::text=$1 AND accepted_at IS NULL AND revoked_at IS NULL
			RETURNING username`, id, s.now()).Scan(&username)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInviteInvalid
		}
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, &actor, "network.invite_revoke", "admin_invite", username, "open", "revoked", "")
	})
}

type openInvite struct {
	id, username, orgID, orgName, displayName, ptype, license string
	adminID                                                   AdminID
	role                                                      Role
	sealed                                                    []byte
	attempts                                                  int
}

func (s *Store) openInvite(ctx context.Context, tx pgx.Tx, token string) (openInvite, error) {
	var v openInvite
	err := tx.QueryRow(ctx, `SELECT i.id::text, i.admin_id, i.username, i.role, i.care_org_id::text, o.name, COALESCE(i.display_name,''),
		COALESCE(i.professional_type,''), COALESCE(i.license_no,''), i.mfa_secret_ct, i.attempts
		FROM admin_invites i JOIN care_orgs o ON o.id = i.care_org_id
		WHERE i.token_hash=$1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > $2 AND o.status <> 'suspended'
		FOR UPDATE OF i`, inviteTokenHash(token), s.now()).Scan(&v.id, &v.adminID, &v.username, &v.role, &v.orgID, &v.orgName,
		&v.displayName, &v.ptype, &v.license, &v.sealed, &v.attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrInviteInvalid
	}
	if err != nil {
		return v, err
	}
	if v.attempts >= maxActivationAttempts {
		return v, ErrInviteInvalid
	}
	return v, nil
}

var totpText = base32.StdEncoding.WithPadding(base32.NoPadding)

// StartActivation returns the invitation and its TOTP secret, creating the
// secret on first use and returning the same one on a reload.
func (s *Store) StartActivation(ctx context.Context, token string) (Activation, error) {
	var out Activation
	err := s.tx(ctx, func(tx pgx.Tx) error {
		v, err := s.openInvite(ctx, tx, token)
		if err != nil {
			return err
		}
		var secret []byte
		if len(v.sealed) > 0 {
			if secret, err = openMFA(s.mfaKey, v.adminID, v.sealed); err != nil {
				return err
			}
		} else {
			text, err := NewTOTPSecret()
			if err != nil {
				return err
			}
			if secret, err = DecodeTOTPSecret(text); err != nil {
				return err
			}
			sealed, err := sealMFA(s.mfaKey, v.adminID, secret)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE admin_invites SET mfa_secret_ct=$2 WHERE id::text=$1`, v.id, sealed); err != nil {
				return err
			}
		}
		text := totpText.EncodeToString(secret)
		out = Activation{Username: v.username, Role: v.role, OrgName: v.orgName, DisplayName: v.displayName, Secret: text,
			OTPAuthURL: otpauthURL(v.username, text)}
		return nil
	})
	return out, err
}

// CompleteActivation checks the password policy and the first
// authenticator code, creates the partner account with the enrolled
// secret, marks the invitation accepted and opens the first session, all
// in one transaction.
func (s *Store) CompleteActivation(ctx context.Context, token, password, code string) (Session, error) {
	var sess Session
	var refusal error
	err := s.tx(ctx, func(tx pgx.Tx) error {
		v, err := s.openInvite(ctx, tx, token)
		if err != nil {
			return err
		}
		if len(v.sealed) == 0 {
			return ErrInviteInvalid
		}
		count := func() error {
			_, err := tx.Exec(ctx, `UPDATE admin_invites SET attempts=attempts+1 WHERE id::text=$1`, v.id)
			return err
		}
		normalized, err := account.CheckPassword(password, v.username, "password")
		if err != nil {
			refusal = ErrInvalid
			return count()
		}
		secret, err := openMFA(s.mfaKey, v.adminID, v.sealed)
		if err != nil {
			return err
		}
		step, ok := VerifyTOTP(secret, code, s.now(), 0)
		if !ok {
			refusal = ErrBadCode
			return count()
		}
		hash, err := s.hasher.Hash(ctx, normalized)
		if err != nil {
			return err
		}
		now := s.now()
		if _, err := tx.Exec(ctx, `INSERT INTO admin_users
			(id, username, password_hash, password_salt, argon2_version, argon2_memory_kib, argon2_iterations, argon2_parallelism,
			 mfa_secret_ct, last_totp_step, role, care_org_id, display_name, professional_type, license_no, status, created_at)
			VALUES ($1,$2,$3,$4,19,$5,$6,$7,$8,$9,$10,$11::uuid,NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),'active',$15)`,
			v.adminID, v.username, hash.Tag, hash.Salt, hash.Params.MemoryKiB, hash.Params.Iterations, hash.Params.Parallelism,
			v.sealed, step, v.role, v.orgID, v.displayName, v.ptype, v.license, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_invites SET accepted_at=$2 WHERE id::text=$1`, v.id, now); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, &v.adminID, "admin.create", "admin_user", v.username, "", "active", "partner invitation accepted"); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, &v.adminID, "network.invite_accept", "admin_invite", v.username, "open", "accepted", ""); err != nil {
			return err
		}
		sessionToken, tokenHash, err := account.NewSessionToken()
		if err != nil {
			return err
		}
		sessionID := newSessionID()
		absolute := now.Add(s.absolute)
		if _, err := tx.Exec(ctx, `UPDATE admin_users SET last_login_at=$2 WHERE id=$1`, v.adminID, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO admin_sessions
			(id, admin_id, token_hash, created_at, last_seen_at, idle_timeout_seconds, absolute_expires_at)
			VALUES ($1,$2,$3,$4,$4,$5,$6)`, sessionID, v.adminID, tokenHash, now, int(s.idle/time.Second), absolute); err != nil {
			return err
		}
		sess = Session{Token: sessionToken, Principal: Principal{AdminID: v.adminID, SessionID: sessionID, Username: v.username, Role: v.role,
			CareOrgID: v.orgID, DisplayName: v.displayName, IdleExpiresAt: now.Add(s.idle), AbsoluteExpiresAt: absolute}}
		return s.audit(ctx, tx, &v.adminID, "admin.login", "admin_session", sessionID.String(), "", "active", "")
	})
	if err != nil {
		return Session{}, err
	}
	if refusal != nil {
		return Session{}, refusal
	}
	return sess, nil
}
