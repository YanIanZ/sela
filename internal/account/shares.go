package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/vault"
)

// Share states (V30/V31).
const (
	ShareActive  = "active"
	ShareRevoked = "revoked"
	ShareExpired = "expired"
)

// Share purposes are a fixed list (V30).
var sharePurposes = map[string]bool{"consultation": true, "second_opinion": true, "family": true, "other": true}

// Share is the owner-facing metadata of one share. It never carries the
// link token, access code or projection content. The access counters are
// derived from share_events: they count links opened and read receipts,
// not people and not care.
type Share struct {
	ID                string     `json:"id"`
	SummaryID         string     `json:"summaryId"`
	SummaryVersion    int        `json:"summaryVersion"`
	Label             string     `json:"label"`
	Purpose           string     `json:"purpose"`
	State             string     `json:"state"`
	Version           int        `json:"version"`
	ExpiresAt         time.Time  `json:"expiresAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	Acknowledged      bool       `json:"acknowledged"`
	OpenedCount       int        `json:"openedCount"`
	AcknowledgedCount int        `json:"acknowledgedCount"`
	LastOpenedAt      *time.Time `json:"lastOpenedAt"`
}

// ShareEvent is one access-history entry the owner may see. It carries no
// recipient identity, IP address, code, token or projection content.
type ShareEvent struct {
	Kind string    `json:"kind"`
	At   time.Time `json:"at"`
}

// SharedSummaryItem is one frozen statement in a shared summary. It
// carries only what a recipient needs: the owner's wording, the exact
// source quote it rests on and the label of the pinned source. Internal
// ids, review state and provenance do not leave the owner's surface.
type SharedSummaryItem struct {
	Text        string `json:"text"`
	Quote       string `json:"quote,omitempty"`
	SourceLabel string `json:"sourceLabel"`
	Position    int    `json:"position"`
}

// SharedProjection is exactly what a recipient may see: an immutable
// snapshot of the approved summary, with no owner, account or session id.
type SharedProjection struct {
	ShareID        string              `json:"shareId"`
	Label          string              `json:"label"`
	Purpose        string              `json:"purpose"`
	SummaryVersion int                 `json:"summaryVersion"`
	Title          string              `json:"title"`
	Text           string              `json:"text"`
	Prepared       *SummaryPrepared    `json:"prepared,omitempty"`
	Items          []SharedSummaryItem `json:"items,omitempty"`
	Sources        []string            `json:"sources"`
	CreatedAt      time.Time           `json:"createdAt"`
	ExpiresAt      time.Time           `json:"expiresAt"`
	Acknowledged   bool                `json:"acknowledged"`
}

// ErrShareInvalid covers an unknown link, a wrong code and an ended share;
// the three are deliberately indistinguishable to a recipient.
var ErrShareInvalid = &Error{Code: "invalid_share"}

func shareToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// shareCode returns 8 characters from an unambiguous alphabet.
func shareCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func codeHash(token, code string) []byte {
	sum := sha256.Sum256([]byte(token + ":" + code))
	return sum[:]
}

func newShareID() string { return newID().String() }

// CreateShare freezes an approved summary into a share and returns the link
// token and access code once, to be shown to the owner.
func (s *Store) CreateShare(ctx context.Context, owner UserID, summaryID, label, purpose string, hours int) (Share, string, string, error) {
	label = strings.TrimSpace(label)
	if label == "" || len([]rune(label)) > 80 {
		return Share{}, "", "", invalid("label")
	}
	if !sharePurposes[purpose] {
		return Share{}, "", "", invalid("purpose")
	}
	if hours < 1 || hours > 168 {
		return Share{}, "", "", invalid("durationHours")
	}
	token, err := shareToken()
	if err != nil {
		return Share{}, "", "", err
	}
	code, err := shareCode()
	if err != nil {
		return Share{}, "", "", err
	}
	now := s.clock()
	share := Share{ID: newShareID(), SummaryID: summaryID, Label: label, Purpose: purpose, State: ShareActive,
		Version: 1, CreatedAt: now, ExpiresAt: now.Add(time.Duration(hours) * time.Hour)}
	err = s.tx(ctx, func(tx pgx.Tx) error {
		return s.createShareTx(ctx, tx, owner, &share, token, code)
	})
	if err != nil {
		return Share{}, "", "", err
	}
	return share, token, code, nil
}

// createShareTx freezes the approved summary share.SummaryID into share
// inside tx. share carries the id, label, purpose and times; it gains the
// summary version. Care requests (D-076) use it with a token and code that
// are never shown, so their share can only be read through the request.
func (s *Store) createShareTx(ctx context.Context, tx pgx.Tx, owner UserID, share *Share, token, code string) error {
	summaryID, label, purpose, now := share.SummaryID, share.Label, share.Purpose, share.CreatedAt
	{
		summary, err := s.record(ctx, tx, owner, summaryID, true)
		if err != nil {
			return err
		}
		if summary.Kind != KindSummary {
			return ErrNotFound
		}
		if err := s.enrichSummaryState(ctx, tx, owner, &summary); err != nil {
			return err
		}
		if summary.SummaryState != SummaryApproved || summary.ApprovedVersion != summary.Version {
			return ErrInvalidTransition
		}
		share.SummaryVersion = summary.Version
		share.Acknowledged = false
		sources := make([]string, 0, len(summary.SourceRefs))
		sourceLabels := make(map[string]string, len(summary.SourceRefs))
		for _, ref := range summary.SourceRefs {
			label := ref.RecordID
			if src, err := s.record(ctx, tx, owner, ref.RecordID, false); err == nil {
				if src.Title != "" {
					label = src.Title
				} else if src.Kind != "" {
					label = src.Kind
				}
			}
			display := label + " · v" + strconv.Itoa(ref.Version)
			sources = append(sources, display)
			sourceLabels[summarySourceKey(ref)] = display
		}
		items := make([]SharedSummaryItem, 0, len(summary.Items))
		for _, item := range summary.Items {
			itemLabel := sourceLabels[summaryItemSourceKey(item.SourceRef)]
			if itemLabel == "" {
				itemLabel = item.SourceRef.RecordID + " · v" + strconv.Itoa(item.SourceRef.Version)
			}
			items = append(items, SharedSummaryItem{
				Text: item.Text, Quote: item.Quote, Position: item.Position,
				SourceLabel: itemLabel,
			})
		}
		projection := SharedProjection{
			ShareID: share.ID, Label: label, Purpose: purpose, SummaryVersion: summary.Version,
			Title: summary.Title, Text: summary.Text, Prepared: clonePrepared(summary.Prepared),
			Items: items, Sources: sources,
			CreatedAt: now, ExpiresAt: share.ExpiresAt,
		}
		// Names the owner marked private are masked in the frozen projection
		// too: sharing must not undo the privacy control the AI path honours.
		privateNames, err := s.privateNames(ctx, tx, owner)
		if err != nil {
			return err
		}
		projection = maskProjection(projection, privateNames)
		raw, err := json.Marshal(projection)
		if err != nil {
			return err
		}
		ct, err := s.vault.Seal(ctx, tx, owner, vault.Ref{Kind: "share", RecordID: share.ID, Field: "projection"}, raw)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO shares
			(id,user_id,summary_id,summary_version,label,purpose,projection_ct,token_hash,code_hash,state,version,expires_at,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'active',1,$10,$11,$11)`,
			share.ID, owner, summaryID, share.SummaryVersion, label, purpose, ct, tokenHash(token), codeHash(token, code),
			share.ExpiresAt, now); err != nil {
			return err
		}
		return s.shareEvent(ctx, tx, share.ID, "created", now)
	}
}

// Shares lists the owner's shares, newest first, without secrets. The
// per-share counters come from one grouped join over share_events, so the
// list stays a single query.
func (s *Store) Shares(ctx context.Context, owner UserID) ([]Share, error) {
	rows, err := s.pool.Query(ctx, `SELECT s.id,s.summary_id,s.summary_version,s.label,s.purpose,s.state,s.version,s.expires_at,s.created_at,
			count(ev.id) FILTER (WHERE ev.kind='opened'),
			count(ev.id) FILTER (WHERE ev.kind='acknowledged'),
			max(ev.at) FILTER (WHERE ev.kind='opened')
		FROM shares s LEFT JOIN share_events ev ON ev.share_id=s.id
		WHERE s.user_id=$1
		GROUP BY s.id
		ORDER BY s.updated_at DESC, s.id DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Share{}
	for rows.Next() {
		var sh Share
		var id, summaryID pgtype.UUID
		var lastOpened *time.Time
		if err := rows.Scan(&id, &summaryID, &sh.SummaryVersion, &sh.Label, &sh.Purpose, &sh.State, &sh.Version,
			&sh.ExpiresAt, &sh.CreatedAt, &sh.OpenedCount, &sh.AcknowledgedCount, &lastOpened); err != nil {
			return nil, err
		}
		sh.ID, sh.SummaryID = uuidText(id.Bytes), uuidText(summaryID.Bytes)
		sh.LastOpenedAt = lastOpened
		sh.Acknowledged = sh.AcknowledgedCount > 0
		if sh.State == ShareActive && s.clock().After(sh.ExpiresAt) {
			sh.State = ShareExpired
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// ShareEvents returns one share's access history, newest first, scoped to
// its owner. Unknown and foreign ids are the same ErrNotFound. Revoked and
// expired shares keep their history readable by the owner.
func (s *Store) ShareEvents(ctx context.Context, owner UserID, id string) ([]ShareEvent, error) {
	if _, err := ParseUUID(id); err != nil {
		return nil, ErrNotFound
	}
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT true FROM shares WHERE user_id=$1 AND id=$2", owner, id).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT ev.kind,ev.at FROM share_events ev
		WHERE ev.share_id=$1 ORDER BY ev.at DESC, ev.id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShareEvent{}
	for rows.Next() {
		var ev ShareEvent
		if err := rows.Scan(&ev.Kind, &ev.At); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// RevokeShare ends every future recipient access at once.
func (s *Store) RevokeShare(ctx context.Context, owner UserID, id string, expectedVersion int) (Share, error) {
	var sh Share
	err := s.tx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT summary_version,label,purpose,state,version,expires_at,created_at
			FROM shares WHERE user_id=$1 AND id=$2 FOR UPDATE`, owner, id).
			Scan(&sh.SummaryVersion, &sh.Label, &sh.Purpose, &sh.State, &sh.Version, &sh.ExpiresAt, &sh.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if sh.Version != expectedVersion {
			return ErrStaleVersion
		}
		if sh.State == ShareRevoked {
			return ErrInvalidTransition
		}
		now := s.clock()
		sh.State, sh.Version = ShareRevoked, sh.Version+1
		if _, err := tx.Exec(ctx, `UPDATE shares SET state='revoked',version=$3,updated_at=$4,revoked_at=$4,end_reason='owner_revoked'
			WHERE user_id=$1 AND id=$2`, owner, id, sh.Version, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "DELETE FROM share_recipient_sessions WHERE share_id=$1", id); err != nil {
			return err
		}
		return s.shareEvent(ctx, tx, id, "revoked", now)
	})
	sh.ID = id
	return sh, err
}

// OpenShare validates a link and code, creates a recipient session and
// returns the frozen projection. A wrong code counts against the share.
func (s *Store) OpenShare(ctx context.Context, token, code string) (SharedProjection, string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	now := s.clock()
	var (
		projection   SharedProjection
		owner        pgtype.UUID
		shareID      string
		state        string
		expires      time.Time
		attempts     int
		codeDigest   []byte
		userID       pgtype.UUID
		sessionToken string
	)
	err := s.tx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT id,user_id,state,expires_at,attempts,code_hash
			FROM shares WHERE token_hash=$1 FOR UPDATE`, tokenHash(token)).
			Scan(&userID, &owner, &state, &expires, &attempts, &codeDigest)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrShareInvalid
		}
		if err != nil {
			return err
		}
		shareID = uuidText(userID.Bytes)
		if state != ShareActive || now.After(expires) || attempts >= 5 {
			return ErrShareInvalid
		}
		if subtle.ConstantTimeCompare(codeDigest, codeHash(token, code)) != 1 {
			_, _ = tx.Exec(ctx, "UPDATE shares SET attempts=attempts+1,updated_at=$2 WHERE id=$1", shareID, now)
			return ErrShareInvalid
		}
		var ct []byte
		if err := tx.QueryRow(ctx, "SELECT projection_ct FROM shares WHERE id=$1", shareID).Scan(&ct); err != nil {
			return err
		}
		raw, err := s.vault.Open(ctx, tx, owner.Bytes, vault.Ref{Kind: "share", RecordID: shareID, Field: "projection"}, ct)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &projection); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE shares SET attempts=0,updated_at=$2 WHERE id=$1", shareID, now); err != nil {
			return err
		}
		session, err := shareToken()
		if err != nil {
			return err
		}
		sessionExpiry := now.Add(24 * time.Hour)
		if sessionExpiry.After(expires) {
			sessionExpiry = expires
		}
		if _, err := tx.Exec(ctx, `INSERT INTO share_recipient_sessions(id,share_id,token_hash,created_at,expires_at)
			VALUES($1,$2,$3,$4,$5)`, newID().String(), shareID, tokenHash(session), now, sessionExpiry); err != nil {
			return err
		}
		if err := s.shareEvent(ctx, tx, shareID, "opened", now); err != nil {
			return err
		}
		projection.ShareID = shareID
		projection.Acknowledged = false
		sessionToken = session
		return nil
	})
	return projection, sessionToken, err
}

// RecipientShare returns the projection for a live recipient session.
func (s *Store) RecipientShare(ctx context.Context, session string) (SharedProjection, error) {
	var (
		projection SharedProjection
		owner      pgtype.UUID
		shareID    string
		expires    time.Time
		ack        *time.Time
		state      string
		shareEnd   time.Time
		ct         []byte
	)
	err := s.pool.QueryRow(ctx, `SELECT s.user_id,s.id,s.state,s.expires_at,s.projection_ct,rs.expires_at,rs.acknowledged_at
		FROM share_recipient_sessions rs JOIN shares s ON s.id=rs.share_id
		WHERE rs.token_hash=$1`, tokenHash(session)).
		Scan(&owner, &shareID, &state, &shareEnd, &ct, &expires, &ack)
	if errors.Is(err, pgx.ErrNoRows) {
		return SharedProjection{}, ErrShareInvalid
	}
	if err != nil {
		return SharedProjection{}, err
	}
	now := s.clock()
	if state != ShareActive || now.After(shareEnd) || now.After(expires) {
		return SharedProjection{}, ErrShareInvalid
	}
	raw, err := s.vault.Open(ctx, s.pool, owner.Bytes, vault.Ref{Kind: "share", RecordID: shareID, Field: "projection"}, ct)
	if err != nil {
		return SharedProjection{}, err
	}
	if err := json.Unmarshal(raw, &projection); err != nil {
		return SharedProjection{}, err
	}
	projection.Acknowledged = ack != nil
	return projection, nil
}

// AcknowledgeShare records that the recipient confirmed they read the
// projection. It is idempotent: a repeat call writes no second event, so
// the owner-facing count stays a count of acknowledgements, not clicks.
func (s *Store) AcknowledgeShare(ctx context.Context, session string) error {
	now := s.clock()
	err := s.tx(ctx, func(tx pgx.Tx) error {
		var id pgtype.UUID
		err := tx.QueryRow(ctx, `UPDATE share_recipient_sessions SET acknowledged_at=$2
			WHERE token_hash=$1 AND acknowledged_at IS NULL RETURNING share_id`, tokenHash(session), now).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			err := tx.QueryRow(ctx, "SELECT true FROM share_recipient_sessions WHERE token_hash=$1", tokenHash(session)).Scan(&exists)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrShareInvalid
			}
			return err
		}
		if err != nil {
			return err
		}
		return s.shareEvent(ctx, tx, uuidText(id.Bytes), "acknowledged", now)
	})
	return err
}

func (s *Store) shareEvent(ctx context.Context, q pgx.Tx, shareID, kind string, at time.Time) error {
	_, err := q.Exec(ctx, "INSERT INTO share_events(id,share_id,kind,at) VALUES($1,$2,$3,$4)", newID().String(), shareID, kind, at)
	return err
}

// maskProjection replaces the owner's private names, case-insensitively,
// with stable [NAMA-n] placeholders in everything a recipient can read.
// The owner's own summary is unchanged; only the frozen share is masked.
func maskProjection(p SharedProjection, names []PrivateName) SharedProjection {
	if len(names) == 0 {
		return p
	}
	type term struct {
		text string
		idx  int
	}
	var terms []term
	for i, n := range names {
		name := strings.TrimSpace(n.Name)
		if name == "" {
			continue
		}
		terms = append(terms, term{text: name, idx: i + 1})
		for _, part := range strings.Fields(name) {
			if utf8.RuneCountInString(part) >= 3 {
				terms = append(terms, term{text: part, idx: i + 1})
			}
		}
	}
	sort.SliceStable(terms, func(i, j int) bool { return len(terms[i].text) > len(terms[j].text) })
	mask := func(s string) string {
		for _, t := range terms {
			re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(t.text))
			s = re.ReplaceAllString(s, "[NAMA-"+strconv.Itoa(t.idx)+"]")
		}
		return s
	}
	p.Title = mask(p.Title)
	p.Text = mask(p.Text)
	if p.Prepared != nil {
		prepared := *p.Prepared
		prepared.Goal = mask(prepared.Goal)
		prepared.Questions = mask(prepared.Questions)
		prepared.Bring = mask(prepared.Bring)
		p.Prepared = &prepared
	}
	for i := range p.Items {
		p.Items[i].Text = mask(p.Items[i].Text)
		p.Items[i].Quote = mask(p.Items[i].Quote)
		p.Items[i].SourceLabel = mask(p.Items[i].SourceLabel)
	}
	for i := range p.Sources {
		p.Sources[i] = mask(p.Sources[i])
	}
	return p
}
