package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/safety"
	"github.com/YanIanZ/sela/internal/vault"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrStorageRequired = &Error{Code: "storage_required"}

// Reflection is one "Sela menangkap…" card: a quote from the user's own
// words, the AI interpretation, and the user's review. It is stored inside
// the sealed conversation content, so it inherits the room's encryption,
// revision, export and deletion behaviour.
type Reflection struct {
	ID           string    `json:"id"`
	TurnID       string    `json:"turnId,omitempty"`
	Quote        string    `json:"quote"`
	Text         string    `json:"text"`
	ReviewStatus string    `json:"reviewStatus"`
	Provenance   string    `json:"provenance"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Reflection review states.
const (
	ReflectionUnreviewed = "unreviewed"
	ReflectionConfirmed  = "confirmed"
	ReflectionCorrected  = "corrected"
	ReflectionRejected   = "rejected"
)

type ConversationTurn struct {
	AIProvenance json.RawMessage `json:"aiProvenance,omitempty"`
	ReviewStatus string          `json:"reviewStatus"`
	Role         string          `json:"role"`
	Text         string          `json:"text"`
	Provenance   string          `json:"provenance"`
	Model        string          `json:"model,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type RecordInput struct {
	Paused          bool               `json:"-"`
	Turns           []ConversationTurn `json:"-"`
	Kind            string             `json:"kind"`
	Title           string             `json:"title"`
	Text            string             `json:"text"`
	Mood            string             `json:"mood"`
	Energy          string             `json:"energy"`
	Tags            []string           `json:"tags"`
	Day             string             `json:"day"`
	StorageGranted  bool               `json:"storageGranted"`
	ExpectedVersion int                `json:"expectedVersion"`
	SourceRefs      []SourceRef        `json:"sourceRefs"`
	Prepared        *SummaryPrepared   `json:"prepared"`
	Sections        *FollowupSections  `json:"sections"`
	ParentID        string             `json:"-"`
	Reflections     []Reflection       `json:"-"`
}

type PersonalRecord struct {
	Paused          bool               `json:"paused,omitempty"`
	Turns           []ConversationTurn `json:"turns,omitempty"`
	ID              string             `json:"id"`
	Kind            string             `json:"kind"`
	Title           string             `json:"title"`
	Text            string             `json:"text"`
	Mood            string             `json:"mood,omitempty"`
	Energy          string             `json:"energy,omitempty"`
	Tags            []string           `json:"tags,omitempty"`
	Day             string             `json:"day,omitempty"`
	Version         int                `json:"version"`
	Provenance      string             `json:"provenance"`
	ReviewStatus    string             `json:"reviewStatus"`
	Safety          string             `json:"safety"`
	Ruleset         string             `json:"ruleset"`
	SourceRefs      []SourceRef        `json:"sourceRefs,omitempty"`
	SummaryState    string             `json:"summaryState,omitempty"`
	ApprovedVersion int                `json:"approvedVersion,omitempty"`
	Prepared        *SummaryPrepared   `json:"prepared,omitempty"`
	Sections        *FollowupSections  `json:"sections,omitempty"`
	ParentID        string             `json:"parentId,omitempty"`
	Reflections     []Reflection       `json:"reflections,omitempty"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

func recordRef(id string, version int) vault.Ref {
	return vault.Ref{Kind: "personal_record", RecordID: id, Field: fmt.Sprintf("content_v%d", version)}
}
func validateRecord(in RecordInput) error {
	if !in.StorageGranted {
		return ErrStorageRequired
	}
	if len(in.Turns) > 64 {
		return invalid("turns")
	}
	if len(in.Reflections) > 50 {
		return invalid("reflections")
	}
	for _, r := range in.Reflections {
		if !utf8.ValidString(r.Quote) || !utf8.ValidString(r.Text) || utf8.RuneCountInString(r.Quote) > 1200 ||
			utf8.RuneCountInString(r.Text) > 1200 {
			return invalid("reflections")
		}
		switch r.ReviewStatus {
		case ReflectionUnreviewed, ReflectionConfirmed, ReflectionCorrected, ReflectionRejected:
		default:
			return invalid("reflections")
		}
	}
	if !utf8.ValidString(in.Text) || utf8.RuneCountInString(in.Text) > 6000 || utf8.RuneCountInString(in.Title) > 120 {
		return invalid("text")
	}
	// Structured sections belong to a follow-up only; the generic record
	// surface must not silently drop them.
	if in.Sections != nil {
		return invalid("sections")
	}
	if in.Kind != "note" && in.Kind != "checkin" && in.Kind != "conversation" {
		return invalid("kind")
	}
	if in.Kind == "note" && strings.TrimSpace(in.Text) == "" {
		return invalid("text")
	}
	if in.Kind == "checkin" {
		if _, err := time.Parse("2006-01-02", in.Day); err != nil {
			return invalid("day")
		}
		switch in.Mood {
		case "", "very_heavy", "heavy", "okay", "good", "very_good", "mixed":
		default:
			return invalid("mood")
		}
		switch in.Energy {
		case "", "low", "medium", "high":
		default:
			return invalid("energy")
		}
		if len(in.Tags) > 10 {
			return invalid("tags")
		}
		seen := map[string]bool{}
		for _, tag := range in.Tags {
			switch tag {
			case "sleep", "school", "work", "family", "friends", "partner", "money", "health", "loneliness", "other":
			default:
				return invalid("tags")
			}
			if seen[tag] {
				return invalid("tags")
			}
			seen[tag] = true
		}
	}
	return nil
}
func makeRecord(id string, in RecordInput, version int, created, now time.Time) PersonalRecord {
	a := safety.Assess(in.Title + "\n" + in.Text)
	for _, turn := range in.Turns {
		if turn.Role == "user" {
			assessment := safety.Assess(turn.Text)
			if assessment.Level == safety.LevelCrisis || (assessment.Level == safety.LevelElevated && a.Level != safety.LevelCrisis) {
				a = assessment
			}
		}
	}
	prov := "user_original"
	if version > 1 {
		prov = "user_correction"
	}
	return PersonalRecord{Paused: in.Paused, Turns: in.Turns, Reflections: append([]Reflection(nil), in.Reflections...), ID: id, Kind: in.Kind, Title: in.Title, Text: in.Text, Mood: in.Mood, Energy: in.Energy, Tags: in.Tags, Day: in.Day, Version: version, Provenance: prov, ReviewStatus: "confirmed", Safety: string(a.Level), Ruleset: a.Ruleset, CreatedAt: created, UpdatedAt: now}
}
func (s *Store) CreateRecord(ctx context.Context, owner UserID, in RecordInput) (PersonalRecord, error) {
	if err := validateRecord(in); err != nil {
		return PersonalRecord{}, err
	}
	id := newID().String()
	now := s.clock()
	r := makeRecord(id, in, 1, now, now)
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := userKind(ctx, tx, owner, true); err != nil {
			return err
		}
		if in.Kind == "checkin" {
			var exists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM personal_records WHERE user_id=$1 AND kind='checkin' AND local_day=$2)", owner, in.Day).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return ErrStaleVersion
			}
		}
		raw, err := json.Marshal(r)
		if err != nil {
			return err
		}
		ct, err := s.vault.Seal(ctx, tx, owner, recordRef(id, 1), raw)
		if err != nil {
			return err
		}
		var day any
		if in.Kind == "checkin" {
			day = in.Day
		}
		if _, err = tx.Exec(ctx, "INSERT INTO personal_records(id,user_id,kind,local_day,version,content_ct,created_at,updated_at) VALUES($1,$2,$3,$4,1,$5,$6,$6)", id, owner, in.Kind, day, ct, now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO personal_record_versions VALUES($1,$2,1,$3,$4)", id, owner, ct, now)
		return err
	})
	return r, err
}
func (s *Store) openRecord(ctx context.Context, q db.DBTX, owner UserID, id string, version int, ct []byte) (PersonalRecord, error) {
	var r PersonalRecord
	raw, err := s.vault.Open(ctx, q, owner, recordRef(id, version), ct)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(raw, &r)
	return r, err
}
func (s *Store) record(ctx context.Context, q db.DBTX, owner UserID, id string, lock bool) (PersonalRecord, error) {
	if _, err := ParseUUID(id); err != nil {
		return PersonalRecord{}, ErrNotFound
	}
	sql := "SELECT version,content_ct FROM personal_records WHERE user_id=$1 AND id=$2"
	if lock {
		sql += " FOR UPDATE"
	}
	var v int
	var ct []byte
	err := q.QueryRow(ctx, sql, owner, id).Scan(&v, &ct)
	if errors.Is(err, pgx.ErrNoRows) {
		return PersonalRecord{}, ErrNotFound
	}
	if err != nil {
		return PersonalRecord{}, err
	}
	return s.openRecord(ctx, q, owner, id, v, ct)
}
func (s *Store) Record(ctx context.Context, owner UserID, id string) (PersonalRecord, error) {
	r, err := s.record(ctx, s.pool, owner, id, false)
	if err != nil {
		return PersonalRecord{}, err
	}
	if r.Kind == KindMemory {
		enabled, err := s.memoryEnabled(ctx, s.pool, owner)
		if err != nil {
			return PersonalRecord{}, err
		}
		if !enabled {
			return PersonalRecord{}, ErrNotFound
		}
	}
	if err := s.enrichSummaryState(ctx, s.pool, owner, &r); err != nil {
		return PersonalRecord{}, err
	}
	return r, nil
}
func (s *Store) records(ctx context.Context, q db.DBTX, owner UserID) ([]PersonalRecord, error) {
	rows, err := q.Query(ctx, "SELECT id,version,content_ct FROM personal_records WHERE user_id=$1 ORDER BY updated_at DESC,id DESC", owner)
	if err != nil {
		return nil, err
	}
	type sealed struct {
		id pgtype.UUID
		v  int
		ct []byte
	}
	var data []sealed
	for rows.Next() {
		var x sealed
		if err = rows.Scan(&x.id, &x.v, &x.ct); err != nil {
			rows.Close()
			return nil, err
		}
		data = append(data, x)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []PersonalRecord{}
	for _, x := range data {
		r, err := s.openRecord(ctx, q, owner, x.id.String(), x.v, x.ct)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	for i := range out {
		if err := s.enrichSummaryState(ctx, q, owner, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Store) Records(ctx context.Context, owner UserID) ([]PersonalRecord, error) {
	records, err := s.records(ctx, s.pool, owner)
	if err != nil {
		return nil, err
	}
	enabled, err := s.memoryEnabled(ctx, s.pool, owner)
	if err != nil {
		return nil, err
	}
	out := make([]PersonalRecord, 0, len(records))
	for _, r := range records {
		if r.Kind != KindMemory || enabled {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *Store) UpdateRecord(ctx context.Context, owner UserID, id string, in RecordInput) (PersonalRecord, error) {
	if in.Kind == KindFollowup {
		return s.updateFollowup(ctx, owner, "", id, in)
	}
	if err := validateRecord(in); err != nil {
		return PersonalRecord{}, err
	}
	var result PersonalRecord
	err := s.tx(ctx, func(tx pgx.Tx) error {
		before, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if before.Version != in.ExpectedVersion {
			return ErrStaleVersion
		}
		if before.Kind != in.Kind || before.Day != in.Day {
			return invalid("kind")
		}
		result = makeRecord(id, in, before.Version+1, before.CreatedAt, s.clock())
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		ct, err := s.vault.Seal(ctx, tx, owner, recordRef(id, result.Version), raw)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE personal_records SET version=$3,content_ct=$4,updated_at=$5 WHERE user_id=$1 AND id=$2", owner, id, result.Version, ct, result.UpdatedAt); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO personal_record_versions VALUES($1,$2,$3,$4,$5)", id, owner, result.Version, ct, result.UpdatedAt)
		return err
	})
	return result, err
}

// DeleteRecord removes one owned record and every revision. derived is
// DerivedKeep (the default) or DerivedDelete: keep leaves summaries that
// pin the source in place, where enrichSummaryState demotes their effective
// approval; delete removes those summaries, their follow-ups and (through
// the existing foreign keys) their shares in the same transaction.
func (s *Store) DeleteRecord(ctx context.Context, owner UserID, id string, version int, derived string) error {
	if derived == "" {
		derived = DerivedKeep
	}
	if derived != DerivedKeep && derived != DerivedDelete {
		return invalid("derived")
	}
	return s.tx(ctx, func(tx pgx.Tx) error {
		if derived == DerivedDelete {
			if err := lockOwnerSummaries(ctx, tx, owner); err != nil {
				return err
			}
		}
		r, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if version != r.Version {
			return ErrStaleVersion
		}
		if derived == DerivedDelete {
			if err := s.deleteDerivedSummaries(ctx, tx, owner, r.ID); err != nil {
				return err
			}
		}
		if r.Kind == KindSummary {
			if err := s.deleteSummaryChildren(ctx, tx, owner, id); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, "DELETE FROM personal_records WHERE user_id=$1 AND id=$2", owner, id)
		return err
	})
}
func (s *Store) RecordHistory(ctx context.Context, owner UserID, id string) ([]PersonalRecord, error) {
	if _, err := s.Record(ctx, owner, id); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, "SELECT version,content_ct FROM personal_record_versions WHERE user_id=$1 AND record_id=$2 ORDER BY version", owner, id)
	if err != nil {
		return nil, err
	}
	type revision struct {
		v  int
		ct []byte
	}
	var data []revision
	for rows.Next() {
		var x revision
		if err = rows.Scan(&x.v, &x.ct); err != nil {
			rows.Close()
			return nil, err
		}
		data = append(data, x)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []PersonalRecord{}
	for _, x := range data {
		r, err := s.openRecord(ctx, s.pool, owner, id, x.v, x.ct)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) exportRecords(ctx context.Context, tx pgx.Tx, owner UserID) (any, error) {
	records, err := s.records(ctx, tx, owner)
	if err != nil {
		return nil, err
	}
	type rev struct {
		id pgtype.UUID
		v  int
		ct []byte
	}
	var data []rev
	rows, err := tx.Query(ctx, "SELECT record_id,version,content_ct FROM personal_record_versions WHERE user_id=$1 ORDER BY record_id,version", owner)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r rev
		if err = rows.Scan(&r.id, &r.v, &r.ct); err != nil {
			rows.Close()
			return nil, err
		}
		data = append(data, r)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	versions := []PersonalRecord{}
	for _, r := range data {
		record, err := s.openRecord(ctx, tx, owner, r.id.String(), r.v, r.ct)
		if err != nil {
			return nil, err
		}
		versions = append(versions, record)
	}
	return map[string]any{"records": records, "versions": versions}, nil
}
