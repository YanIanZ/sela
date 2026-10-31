package account

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/vault"
)

// Private-name list limits (the "names kept private" privacy control).
const (
	// MaxPrivateNames bounds one owner's list.
	MaxPrivateNames = 20
	// MaxPrivateNameRunes bounds one name (1..120 runes after trimming).
	MaxPrivateNameRunes = 120
)

// PrivateName is one name the owner marked private. It is decrypted only
// for the owner's own settings screen and export; every AI request gets the
// plaintext only long enough to redact it before a provider payload exists.
type PrivateName struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// ErrNameTaken refuses a name already on the owner's list; the comparison
// is case-insensitive because the ciphertext cannot carry a unique index.
var ErrNameTaken = &Error{Code: "name_taken", Field: "name"}

// ErrNameLimitReached refuses the 21st name.
var ErrNameLimitReached = &Error{Code: "name_limit_reached", Field: "name"}

func privateNameRef(id string) vault.Ref {
	return vault.Ref{Kind: "private_name", RecordID: id, Field: "name"}
}

// normalizePrivateName trims the edges, then enforces 1..120 runes of valid
// UTF-8 without control characters. Nothing is stored before this passes.
func normalizePrivateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if !utf8.ValidString(name) || name == "" || utf8.RuneCountInString(name) > MaxPrivateNameRunes {
		return "", invalid("name")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", invalid("name")
		}
	}
	return name, nil
}

// privateNames reads and decrypts the owner's list in creation order.
func (s *Store) privateNames(ctx context.Context, q db.DBTX, owner UserID) ([]PrivateName, error) {
	rows, err := q.Query(ctx, `SELECT id, name_ct, created_at FROM private_names WHERE user_id = $1 ORDER BY created_at, id`, owner)
	if err != nil {
		return nil, err
	}
	type sealed struct {
		id pgtype.UUID
		ct []byte
		at time.Time
	}
	var list []sealed
	for rows.Next() {
		var x sealed
		if err := rows.Scan(&x.id, &x.ct, &x.at); err != nil {
			rows.Close()
			return nil, err
		}
		list = append(list, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []PrivateName{}
	for _, x := range list {
		id := x.id.String()
		raw, err := s.vault.Open(ctx, q, owner, privateNameRef(id), x.ct)
		if err != nil {
			return nil, err
		}
		out = append(out, PrivateName{ID: id, Name: string(raw), CreatedAt: x.at})
	}
	return out, nil
}

// PrivateNames lists the owner's names, oldest first.
func (s *Store) PrivateNames(ctx context.Context, owner UserID) ([]PrivateName, error) {
	if _, err := userKind(ctx, s.pool, owner, false); err != nil {
		return nil, err
	}
	return s.privateNames(ctx, s.pool, owner)
}

// CreatePrivateName seals and stores one name. The owner's user row is
// locked for the duplicate check and the 20-name limit, so two concurrent
// creates cannot both pass.
func (s *Store) CreatePrivateName(ctx context.Context, owner UserID, raw string) (PrivateName, error) {
	name, err := normalizePrivateName(raw)
	if err != nil {
		return PrivateName{}, err
	}
	id := newID().String()
	now := s.clock()
	out := PrivateName{ID: id, Name: name, CreatedAt: now}
	err = s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := userKind(ctx, tx, owner, true); err != nil {
			return err
		}
		existing, err := s.privateNames(ctx, tx, owner)
		if err != nil {
			return err
		}
		if len(existing) >= MaxPrivateNames {
			return ErrNameLimitReached
		}
		for _, n := range existing {
			if strings.EqualFold(n.Name, name) {
				return ErrNameTaken
			}
		}
		ct, err := s.vault.Seal(ctx, tx, owner, privateNameRef(id), []byte(name))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO private_names (id, user_id, name_ct, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $4)`, id, owner, ct, now)
		if err != nil {
			return err
		}
		return nil
	})
	return out, err
}

// DeletePrivateName removes one owned name. Another owner's id, an unknown
// id and a malformed id all answer ErrNotFound.
func (s *Store) DeletePrivateName(ctx context.Context, owner UserID, id string) error {
	if _, err := ParseUUID(id); err != nil {
		return ErrNotFound
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM private_names WHERE user_id = $1 AND id = $2`, owner, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
