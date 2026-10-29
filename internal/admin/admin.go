// Package admin is Sela's separate operator identity and audit domain.
//
// It is not an extension of the anonymous user account system: staff are
// provisioned out of band, authenticate with a password and TOTP, and can
// never read a user's journal or conversation. Every mutation is written to
// PostgreSQL admin_audit; Redis is never the authority for a permission.
package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Role is one least-privilege operator role.
type Role string

const (
	RoleOwner             Role = "owner"
	RoleSecurityAdmin     Role = "security_admin"
	RoleDirectoryEditor   Role = "directory_editor"
	RoleDirectoryReviewer Role = "directory_reviewer"
	RoleResearchEditor    Role = "research_editor"
	RoleSafetyReviewer    Role = "safety_reviewer"
	RoleOps               Role = "ops"
)

// Permission strings are deliberately narrow and checked server-side.
const (
	PermOverviewRead    = "ops/read"
	PermAuditRead       = "security/audit/read"
	PermCacheWrite      = "ops/cache/write"
	PermDirectoryWrite  = "directory/write"
	PermDirectoryVerify = "directory/verify"
	PermResearchReview  = "research/review"
	PermSafetyReview    = "safety-content/review"
	PermStaffManage     = "staff/manage"
)

var rolePermissions = map[Role][]string{
	RoleOwner:             {PermOverviewRead, PermAuditRead, PermCacheWrite, PermDirectoryWrite, PermDirectoryVerify, PermResearchReview, PermSafetyReview, PermStaffManage},
	RoleSecurityAdmin:     {PermOverviewRead, PermAuditRead, PermStaffManage},
	RoleDirectoryEditor:   {PermOverviewRead, PermDirectoryWrite},
	RoleDirectoryReviewer: {PermOverviewRead, PermDirectoryVerify},
	RoleResearchEditor:    {PermOverviewRead, PermResearchReview},
	RoleSafetyReviewer:    {PermOverviewRead, PermSafetyReview},
	RoleOps:               {PermOverviewRead, PermCacheWrite},
}

// ValidRole reports whether r is one of the closed set.
func ValidRole(r Role) bool { _, ok := rolePermissions[r]; return ok }

// Permissions returns a copy of the role's server-enforced permissions.
func Permissions(r Role) []string {
	p := rolePermissions[r]
	return append([]string(nil), p...)
}

// Principal is an authenticated staff session. It deliberately has no user
// id or anonymous-session field.
type Principal struct {
	AdminID           AdminID
	SessionID         SessionID
	Username          string
	Role              Role
	OrgID             string
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
}

// Can reports whether the principal holds permission p.
func (p Principal) Can(permission string) bool {
	for _, got := range Permissions(p.Role) {
		if got == permission {
			return true
		}
	}
	return false
}

// AdminID is a uuid v4 staff identity.
type AdminID [16]byte

// SessionID is a uuid v4 admin session row.
type SessionID [16]byte

func newUUID() [16]byte {
	var u [16]byte
	if _, err := rand.Read(u[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return u
}

func newAdminID() AdminID     { return AdminID(newUUID()) }
func newSessionID() SessionID { return SessionID(newUUID()) }

func (id AdminID) String() string   { return pgtype.UUID{Bytes: id, Valid: true}.String() }
func (id SessionID) String() string { return pgtype.UUID{Bytes: id, Valid: true}.String() }

// UUIDValue lets pgx encode the id as uuid.
func (id AdminID) UUIDValue() (pgtype.UUID, error) { return pgtype.UUID{Bytes: id, Valid: true}, nil }

// ScanUUID lets pgx decode a uuid column.
func (id *AdminID) ScanUUID(v pgtype.UUID) error {
	if !v.Valid {
		return fmt.Errorf("admin: NULL admin id")
	}
	*id = v.Bytes
	return nil
}

// UUIDValue lets pgx encode the session id as uuid.
func (id SessionID) UUIDValue() (pgtype.UUID, error) {
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

// ScanUUID lets pgx decode a uuid column.
func (id *SessionID) ScanUUID(v pgtype.UUID) error {
	if !v.Valid {
		return fmt.Errorf("admin: NULL session id")
	}
	*id = v.Bytes
	return nil
}

// Errors are deliberately generic; callers must not reveal whether a staff
// username exists.
var (
	ErrInvalidCredentials = errors.New("admin: invalid credentials")
	ErrNotSignedIn        = errors.New("admin: not signed in")
	ErrForbidden          = errors.New("admin: forbidden")
	ErrUnavailable        = errors.New("admin: unavailable")
	ErrThrottled          = errors.New("admin: throttled")
	ErrInvalid            = errors.New("admin: invalid input")
	ErrLastOwner          = errors.New("admin: the last active owner cannot be revoked")
)

// StaffView is one operator identity without any secret material.
type StaffView struct {
	Username     string     `json:"username"`
	Role         Role       `json:"role"`
	OrgID        string     `json:"orgId,omitempty"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	LiveSessions int64      `json:"liveSessions"`
}

// Overview is a privacy-safe operator aggregate. It contains no user text,
// user id, session id, IP or content-derived metric.
type Overview struct {
	ActiveStaff   int64     `json:"activeStaff"`
	LiveSessions  int64     `json:"liveSessions"`
	AuditLast24h  int64     `json:"auditLast24h"`
	GeneratedAt   time.Time `json:"generatedAt"`
	CacheEnabled  bool      `json:"cacheEnabled"`
	DatabaseReady bool      `json:"databaseReady"`
	Version       string    `json:"version"`
}

// AuditEvent is one immutable operator action. Reason/old/new are free of
// personal content by construction.
type AuditEvent struct {
	ID         string    `json:"id"`
	Actor      string    `json:"actor,omitempty"`
	Action     string    `json:"action"`
	ObjectKind string    `json:"objectKind,omitempty"`
	ObjectID   string    `json:"objectId,omitempty"`
	OldStatus  string    `json:"oldStatus,omitempty"`
	NewStatus  string    `json:"newStatus,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// HashUsername returns a fixed-size key for unknown-staff timing work.
func HashUsername(username string) [32]byte {
	return sha256.Sum256([]byte("sela.admin.username.v1:" + username))
}

// ValidateUsername mirrors the account username shape.
func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("%w: username length", ErrInvalid)
	}
	for i, r := range username {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			if i == 0 && !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
				return fmt.Errorf("%w: username start", ErrInvalid)
			}
			continue
		}
		return fmt.Errorf("%w: username character", ErrInvalid)
	}
	return nil
}
