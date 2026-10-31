package account

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/db"
	"github.com/YanIanZ/sela/internal/safety"
)

// Continuity kinds reuse the personal-record envelope: the same per-owner
// vault, append-only revisions, export and deletion paths apply. They are
// never accepted through the generic note/check-in API.
const (
	KindMemory   = "memory"
	KindSummary  = "summary"
	KindFollowup = "followup"
)

// ContinuityTitleMax and ContinuityTextMax bound every continuity text.
// FollowupSectionMax bounds each optional section of an after-consultation
// note independently.
const (
	ContinuityTitleMax = 6000
	ContinuityTextMax  = 6000
	FollowupSectionMax = 2000
	MaxSourceRefs      = 12
)

// Summary state is server-controlled: only an explicit approval can make
// it approved, and any edit or stale source makes it a draft again.
const (
	SummaryDraft    = "draft"
	SummaryApproved = "approved"
)

// SourceRef pins one explicitly selected source record and the exact
// revision the owner saw. The reference is stored inside the sealed
// content, so it is encrypted and owner-scoped like the text.
type SourceRef struct {
	RecordID string `json:"recordId"`
	Version  int    `json:"version"`
}

// SummaryPrepared is the optional preparation the owner writes for a
// consultation. Every field is the owner's own words.
type SummaryPrepared struct {
	Goal      string `json:"goal"`
	Questions string `json:"questions"`
	Bring     string `json:"bring"`
}

// FollowupSections is the optional structured after-consultation note.
// Every field is the owner's own words and is stored inside the sealed
// record content, so it inherits the same encryption, revisions, export
// and deletion behaviour as the free-text field.
type FollowupSections struct {
	Feeling  string `json:"feeling"`
	Remember string `json:"remember"`
	Agreed   string `json:"agreed"`
	AskNext  string `json:"askNext"`
}

func (s *FollowupSections) empty() bool {
	return s == nil || (strings.TrimSpace(s.Feeling) == "" &&
		strings.TrimSpace(s.Remember) == "" &&
		strings.TrimSpace(s.Agreed) == "" &&
		strings.TrimSpace(s.AskNext) == "")
}

// cloneFollowupSections preserves owner spacing but drops a sections
// object whose every field is blank, so old text-only clients and new
// structured clients store the same shape when nothing was written.
func cloneFollowupSections(in *FollowupSections) *FollowupSections {
	if in.empty() {
		return nil
	}
	out := *in
	return &out
}

func validFollowupSections(in *FollowupSections) bool {
	if in == nil {
		return true
	}
	for _, s := range []string{in.Feeling, in.Remember, in.Agreed, in.AskNext} {
		if !utf8.ValidString(s) || utf8.RuneCountInString(s) > FollowupSectionMax {
			return false
		}
	}
	return true
}

// MemoryView is GET /api/v3/memory. Before consent, Items is empty and
// Enabled is false, whether or not old rows still exist.
type MemoryView struct {
	Enabled bool             `json:"enabled"`
	Items   []PersonalRecord `json:"items"`
}

func cloneSourceRefs(in []SourceRef) []SourceRef {
	if len(in) == 0 {
		return nil
	}
	return append([]SourceRef(nil), in...)
}

func clonePrepared(in *SummaryPrepared) *SummaryPrepared {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

// validateContinuity validates one memory, summary or follow-up input.
// minSources is 0 for memory/follow-up and 1 for a summary.
func validateContinuity(in RecordInput, minSources int) error {
	if !in.StorageGranted {
		return ErrStorageRequired
	}
	switch in.Kind {
	case KindMemory, KindSummary, KindFollowup:
	default:
		return invalid("kind")
	}
	if !utf8.ValidString(in.Title) || utf8.RuneCountInString(in.Title) > ContinuityTitleMax ||
		!utf8.ValidString(in.Text) || utf8.RuneCountInString(in.Text) > ContinuityTextMax {
		return invalid("text")
	}
	// A follow-up may be text-only (old clients), sections-only, or both.
	// Every other continuity kind still needs text.
	if strings.TrimSpace(in.Text) == "" && !(in.Kind == KindFollowup && !in.Sections.empty()) {
		return invalid("text")
	}
	// Continuity records never carry check-in fields.
	if in.Day != "" || in.Mood != "" || in.Energy != "" || len(in.Tags) > 0 {
		return invalid("kind")
	}
	if len(in.SourceRefs) > MaxSourceRefs {
		return invalid("sourceRefs")
	}
	if len(in.SourceRefs) < minSources {
		return invalid("sourceRefs")
	}
	if in.Kind == KindFollowup {
		if !validFollowupSections(in.Sections) {
			return invalid("sections")
		}
		if len(in.SourceRefs) != 0 || in.Prepared != nil {
			return invalid("sourceRefs")
		}
		return nil
	}
	if in.Sections != nil {
		return invalid("sections")
	}
	if in.Kind != KindSummary {
		if in.Prepared != nil {
			return invalid("prepared")
		}
		return nil
	}
	if in.Prepared != nil {
		for _, s := range []string{in.Prepared.Goal, in.Prepared.Questions, in.Prepared.Bring} {
			if !utf8.ValidString(s) || utf8.RuneCountInString(s) > ContinuityTextMax {
				return invalid("prepared")
			}
		}
	}
	return nil
}

func continuityRecord(id string, in RecordInput, version int, created, now time.Time) PersonalRecord {
	assessed := in.Title + "\n" + in.Text
	if in.Prepared != nil {
		assessed += "\n" + in.Prepared.Goal + "\n" + in.Prepared.Questions + "\n" + in.Prepared.Bring
	}
	if in.Sections != nil {
		assessed += "\n" + in.Sections.Feeling + "\n" + in.Sections.Remember +
			"\n" + in.Sections.Agreed + "\n" + in.Sections.AskNext
	}
	a := safety.Assess(assessed)
	provenance := "user_original"
	if version > 1 {
		provenance = "user_correction"
	}
	if in.Kind == KindSummary {
		// A manual summary is the owner's own wording. This increment has no
		// AI drafting path, so it is never labelled as AI output.
		provenance = "manual"
	}
	r := PersonalRecord{
		ID: id, Kind: in.Kind, Title: in.Title, Text: in.Text, Version: version,
		Provenance: provenance, ReviewStatus: "confirmed",
		Safety: string(a.Level), Ruleset: a.Ruleset,
		SourceRefs: cloneSourceRefs(in.SourceRefs), Prepared: clonePrepared(in.Prepared),
		Sections: cloneFollowupSections(in.Sections),
		ParentID: in.ParentID, CreatedAt: created, UpdatedAt: now,
	}
	if in.Kind == KindSummary {
		r.SummaryState = SummaryDraft
	}
	return r
}

func (s *Store) insertContinuityRecord(ctx context.Context, tx pgx.Tx, owner UserID, r PersonalRecord) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	ct, err := s.vault.Seal(ctx, tx, owner, recordRef(r.ID, r.Version), raw)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO personal_records
		(id,user_id,kind,local_day,version,content_ct,created_at,updated_at)
		VALUES($1,$2,$3,NULL,$4,$5,$6,$6)`, r.ID, owner, r.Kind, r.Version, ct, r.CreatedAt); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO personal_record_versions VALUES($1,$2,$3,$4,$5)", r.ID, owner, r.Version, ct, r.CreatedAt)
	return err
}

func (s *Store) updateContinuityRecord(ctx context.Context, tx pgx.Tx, owner UserID, r PersonalRecord) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	ct, err := s.vault.Seal(ctx, tx, owner, recordRef(r.ID, r.Version), raw)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE personal_records SET version=$3,content_ct=$4,updated_at=$5
		WHERE user_id=$1 AND id=$2`, owner, r.ID, r.Version, ct, r.UpdatedAt); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO personal_record_versions VALUES($1,$2,$3,$4,$5)", r.ID, owner, r.Version, ct, r.UpdatedAt)
	return err
}

// rewriteCurrentVersion updates the current version's sealed content in
// place. It is used only for server-controlled state (approval) that is
// not a text edit and therefore does not create a new revision.
func (s *Store) rewriteCurrentVersion(ctx context.Context, tx pgx.Tx, owner UserID, r PersonalRecord) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	ct, err := s.vault.Seal(ctx, tx, owner, recordRef(r.ID, r.Version), raw)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE personal_records SET content_ct=$3,updated_at=$4
		WHERE user_id=$1 AND id=$2`, owner, r.ID, ct, r.UpdatedAt); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE personal_record_versions SET content_ct=$4
		WHERE user_id=$1 AND record_id=$2 AND version=$3`, owner, r.ID, r.Version, ct)
	return err
}

// memoryEnabled reports whether the personal_memory consent is granted on
// the current policy version. A revoked or outdated consent is off.
func (s *Store) memoryEnabled(ctx context.Context, q db.DBTX, owner UserID) (bool, error) {
	current := s.cfg.PolicyVersions[PolicyPersonalMemory]
	if current == "" {
		return false, nil
	}
	var granted bool
	var version string
	err := q.QueryRow(ctx, `SELECT granted, policy_version FROM consents
		WHERE user_id=$1 AND purpose=$2`, owner, PolicyPersonalMemory).Scan(&granted, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return granted && version == current, nil
}

func (s *Store) recordsByKind(ctx context.Context, q db.DBTX, owner UserID, kind string) ([]PersonalRecord, error) {
	rows, err := q.Query(ctx, `SELECT id,version,content_ct FROM personal_records
		WHERE user_id=$1 AND kind=$2 ORDER BY updated_at DESC,id DESC`, owner, kind)
	if err != nil {
		return nil, err
	}
	type sealed struct {
		id string
		v  int
		ct []byte
	}
	var data []sealed
	for rows.Next() {
		var id pgtype.UUID
		var v int
		var ct []byte
		if err = rows.Scan(&id, &v, &ct); err != nil {
			rows.Close()
			return nil, err
		}
		data = append(data, sealed{id.String(), v, ct})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []PersonalRecord{}
	for _, x := range data {
		r, err := s.openRecord(ctx, q, owner, x.id, x.v, x.ct)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// validateSourceRefs refuses a reference that is not the owner's, not the
// current revision, or itself a memory/summary/follow-up (no recursive
// sources). lock takes FOR SHARE so a concurrent source update cannot slip
// between validation and the insert.
func (s *Store) validateSourceRefs(ctx context.Context, q db.DBTX, owner UserID, refs []SourceRef, lock bool) error {
	seen := map[string]bool{}
	for _, ref := range refs {
		id, err := ParseUUID(ref.RecordID)
		if err != nil || ref.Version <= 0 {
			return invalid("sourceRefs")
		}
		key := uuidText(id)
		if seen[key] {
			return invalid("sourceRefs")
		}
		seen[key] = true
		sql := "SELECT kind,version FROM personal_records WHERE user_id=$1 AND id=$2"
		if lock {
			sql += " FOR SHARE"
		}
		var kind string
		var version int
		err = q.QueryRow(ctx, sql, owner, key).Scan(&kind, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return withField(ErrNotFound, "sourceRefs")
		}
		if err != nil {
			return err
		}
		switch kind {
		case KindMemory, KindSummary, KindFollowup:
			return invalid("sourceRefs")
		}
		if version != ref.Version {
			return withField(ErrStaleVersion, "sourceRefs")
		}
	}
	return nil
}

// sourceRefsCurrent reports whether every reference still names an
// allowed, owner-owned record at the pinned revision.
func (s *Store) sourceRefsCurrent(ctx context.Context, q db.DBTX, owner UserID, refs []SourceRef, lock bool) (bool, error) {
	for _, ref := range refs {
		id, err := ParseUUID(ref.RecordID)
		if err != nil || ref.Version <= 0 {
			return false, nil
		}
		sql := "SELECT kind,version FROM personal_records WHERE user_id=$1 AND id=$2"
		if lock {
			sql += " FOR SHARE"
		}
		var kind string
		var version int
		err = q.QueryRow(ctx, sql, owner, uuidText(id)).Scan(&kind, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		switch kind {
		case KindMemory, KindSummary, KindFollowup:
			return false, nil
		}
		if version != ref.Version {
			return false, nil
		}
	}
	return true, nil
}

// enrichSummaryState demotes an approved summary to draft on read when a
// source was edited or deleted. The stored approval is left as history;
// every read, export and future share path sees the effective state.
func (s *Store) enrichSummaryState(ctx context.Context, q db.DBTX, owner UserID, r *PersonalRecord) error {
	if r.Kind != KindSummary || r.SummaryState != SummaryApproved {
		return nil
	}
	current, err := s.sourceRefsCurrent(ctx, q, owner, r.SourceRefs, false)
	if err != nil {
		return err
	}
	if !current {
		r.SummaryState = SummaryDraft
	}
	return nil
}

// Memory returns the explicit memory items when the owner has granted the
// personal_memory consent. Without consent it answers enabled=false and
// no items, so a revoked consent stops reads at once.
func (s *Store) Memory(ctx context.Context, owner UserID) (MemoryView, error) {
	enabled, err := s.memoryEnabled(ctx, s.pool, owner)
	if err != nil {
		return MemoryView{}, err
	}
	if !enabled {
		return MemoryView{Enabled: false, Items: []PersonalRecord{}}, nil
	}
	items, err := s.recordsByKind(ctx, s.pool, owner, KindMemory)
	if err != nil {
		return MemoryView{}, err
	}
	return MemoryView{Enabled: true, Items: items}, nil
}

// MemoryItem returns one explicit memory item under the same consent rules
// as Memory: without the current personal_memory consent, or when another
// owner holds the id, the item is not found.
func (s *Store) MemoryItem(ctx context.Context, owner UserID, id string) (PersonalRecord, error) {
	r, err := s.Record(ctx, owner, id)
	if err != nil {
		return PersonalRecord{}, err
	}
	if r.Kind != KindMemory {
		return PersonalRecord{}, ErrNotFound
	}
	return r, nil
}

// CreateMemory stores one explicit memory item. It requires the
// personal_memory consent and storageGranted.
func (s *Store) CreateMemory(ctx context.Context, owner UserID, in RecordInput) (PersonalRecord, error) {
	in.Kind = KindMemory
	if err := validateContinuity(in, 0); err != nil {
		return PersonalRecord{}, err
	}
	id := newID().String()
	now := s.clock()
	r := continuityRecord(id, in, 1, now, now)
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := userKind(ctx, tx, owner, true); err != nil {
			return err
		}
		enabled, err := s.memoryEnabled(ctx, tx, owner)
		if err != nil {
			return err
		}
		if !enabled {
			return withField(ErrConsentRequired, "personalMemory")
		}
		if err := s.validateSourceRefs(ctx, tx, owner, r.SourceRefs, true); err != nil {
			return err
		}
		return s.insertContinuityRecord(ctx, tx, owner, r)
	})
	return r, err
}

// UpdateMemory replaces title/text/sourceRefs in a new revision. It
// requires consent and the exact expected version.
func (s *Store) UpdateMemory(ctx context.Context, owner UserID, id string, in RecordInput) (PersonalRecord, error) {
	var result PersonalRecord
	err := s.tx(ctx, func(tx pgx.Tx) error {
		before, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if before.Kind != KindMemory {
			return ErrNotFound
		}
		if before.Version != in.ExpectedVersion {
			return ErrStaleVersion
		}
		enabled, err := s.memoryEnabled(ctx, tx, owner)
		if err != nil {
			return err
		}
		if !enabled {
			return withField(ErrConsentRequired, "personalMemory")
		}
		in.Kind = KindMemory
		if in.SourceRefs == nil {
			in.SourceRefs = before.SourceRefs
		}
		if err := validateContinuity(in, 0); err != nil {
			return err
		}
		if err := s.validateSourceRefs(ctx, tx, owner, in.SourceRefs, true); err != nil {
			return err
		}
		result = continuityRecord(id, in, before.Version+1, before.CreatedAt, s.clock())
		return s.updateContinuityRecord(ctx, tx, owner, result)
	})
	return result, err
}

// DeleteMemory removes a memory item and its revisions. Deletion is
// allowed after the consent is withdrawn: turning memory off must never
// trap the owner's data.
func (s *Store) DeleteMemory(ctx context.Context, owner UserID, id string, version int) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		r, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if r.Kind != KindMemory {
			return ErrNotFound
		}
		if r.Version != version {
			return ErrStaleVersion
		}
		_, err = tx.Exec(ctx, "DELETE FROM personal_records WHERE user_id=$1 AND id=$2", owner, id)
		return err
	})
}

// Summaries lists the owner's consultation summaries with the effective
// (read-time) state.
func (s *Store) Summaries(ctx context.Context, owner UserID) ([]PersonalRecord, error) {
	list, err := s.recordsByKind(ctx, s.pool, owner, KindSummary)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if err := s.enrichSummaryState(ctx, s.pool, owner, &list[i]); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// Summary returns one summary, demoted to draft when a source changed.
func (s *Store) Summary(ctx context.Context, owner UserID, id string) (PersonalRecord, error) {
	r, err := s.Record(ctx, owner, id)
	if err != nil {
		return PersonalRecord{}, err
	}
	if r.Kind != KindSummary {
		return PersonalRecord{}, ErrNotFound
	}
	return r, nil
}

// CreateSummary stores a manual consultation summary. The text and
// prepared fields are the owner's own words (provenance manual); sources
// are owner-owned records pinned to the revisions the owner selected.
func (s *Store) CreateSummary(ctx context.Context, owner UserID, in RecordInput) (PersonalRecord, error) {
	in.Kind = KindSummary
	if err := validateContinuity(in, 1); err != nil {
		return PersonalRecord{}, err
	}
	id := newID().String()
	now := s.clock()
	r := continuityRecord(id, in, 1, now, now)
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := userKind(ctx, tx, owner, false); err != nil {
			return err
		}
		if err := s.validateSourceRefs(ctx, tx, owner, r.SourceRefs, true); err != nil {
			return err
		}
		return s.insertContinuityRecord(ctx, tx, owner, r)
	})
	return r, err
}

// UpdateSummary replaces the summary content in a new revision. Any edit
// revokes approval (state draft) and the new revision must be approved
// again.
func (s *Store) UpdateSummary(ctx context.Context, owner UserID, id string, in RecordInput) (PersonalRecord, error) {
	var result PersonalRecord
	err := s.tx(ctx, func(tx pgx.Tx) error {
		before, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if before.Kind != KindSummary {
			return ErrNotFound
		}
		if before.Version != in.ExpectedVersion {
			return ErrStaleVersion
		}
		in.Kind = KindSummary
		if in.SourceRefs == nil {
			in.SourceRefs = before.SourceRefs
		}
		if in.Prepared == nil {
			in.Prepared = before.Prepared
		}
		if err := validateContinuity(in, 1); err != nil {
			return err
		}
		if err := s.validateSourceRefs(ctx, tx, owner, in.SourceRefs, true); err != nil {
			return err
		}
		result = continuityRecord(id, in, before.Version+1, before.CreatedAt, s.clock())
		return s.updateContinuityRecord(ctx, tx, owner, result)
	})
	return result, err
}

// ApproveSummary binds approval to the exact current revision. It is
// refused with stale_version when the expected version differs, when a
// source revision changed, or when a source disappeared; a source
// deletion/foreign source is indistinguishable from a stale one here.
func (s *Store) ApproveSummary(ctx context.Context, owner UserID, id string, expectedVersion int) (PersonalRecord, error) {
	var result PersonalRecord
	err := s.tx(ctx, func(tx pgx.Tx) error {
		r, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if r.Kind != KindSummary {
			return ErrNotFound
		}
		if r.Version != expectedVersion {
			return ErrStaleVersion
		}
		current, err := s.sourceRefsCurrent(ctx, tx, owner, r.SourceRefs, true)
		if err != nil {
			return err
		}
		if !current {
			return withField(ErrStaleVersion, "sourceRefs")
		}
		if r.SummaryState == SummaryApproved && r.ApprovedVersion == r.Version {
			result = r
			return nil
		}
		r.SummaryState = SummaryApproved
		r.ApprovedVersion = r.Version
		r.UpdatedAt = s.clock()
		result = r
		return s.rewriteCurrentVersion(ctx, tx, owner, r)
	})
	return result, err
}

// DeleteSummary removes the summary, its revisions and its follow-up
// notes. A follow-up is a child of exactly one summary.
func (s *Store) DeleteSummary(ctx context.Context, owner UserID, id string, version int) error {
	return s.tx(ctx, func(tx pgx.Tx) error {
		r, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if r.Kind != KindSummary {
			return ErrNotFound
		}
		if r.Version != version {
			return ErrStaleVersion
		}
		if err := s.deleteSummaryChildren(ctx, tx, owner, id); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "DELETE FROM personal_records WHERE user_id=$1 AND id=$2", owner, id)
		return err
	})
}

// deleteSummaryChildren removes the follow-up rows whose sealed parentId
// is parentID. parentId is encrypted, so the list is decrypted inside the
// same transaction; follow-ups remain owner scoped; parent ids never leave the encrypted envelope.
func (s *Store) deleteSummaryChildren(ctx context.Context, tx pgx.Tx, owner UserID, parentID string) error {
	rows, err := tx.Query(ctx, `SELECT id,version,content_ct FROM personal_records
		WHERE user_id=$1 AND kind=$2 ORDER BY id`, owner, KindFollowup)
	if err != nil {
		return err
	}
	type sealed struct {
		id string
		v  int
		ct []byte
	}
	var data []sealed
	for rows.Next() {
		var id pgtype.UUID
		var v int
		var ct []byte
		if err = rows.Scan(&id, &v, &ct); err != nil {
			rows.Close()
			return err
		}
		data = append(data, sealed{id.String(), v, ct})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, x := range data {
		r, err := s.openRecord(ctx, tx, owner, x.id, x.v, x.ct)
		if err != nil {
			return err
		}
		if r.ParentID != parentID {
			continue
		}
		if _, err := tx.Exec(ctx, "DELETE FROM personal_records WHERE user_id=$1 AND id=$2", owner, x.id); err != nil {
			return err
		}
	}
	return nil
}

// CreateFollowup stores a note attached to one of the owner's summaries.
func (s *Store) CreateFollowup(ctx context.Context, owner UserID, parentID string, in RecordInput) (PersonalRecord, error) {
	in.Kind = KindFollowup
	in.ParentID = parentID
	if err := validateContinuity(in, 0); err != nil {
		return PersonalRecord{}, err
	}
	id := newID().String()
	now := s.clock()
	r := continuityRecord(id, in, 1, now, now)
	err := s.tx(ctx, func(tx pgx.Tx) error {
		parent, err := s.record(ctx, tx, owner, parentID, true)
		if err != nil {
			return err
		}
		if parent.Kind != KindSummary {
			return ErrNotFound
		}
		return s.insertContinuityRecord(ctx, tx, owner, r)
	})
	return r, err
}

// Followups lists the notes attached to one owner-owned summary.
func (s *Store) Followups(ctx context.Context, owner UserID, parentID string) ([]PersonalRecord, error) {
	parent, err := s.record(ctx, s.pool, owner, parentID, false)
	if err != nil {
		return nil, err
	}
	if parent.Kind != KindSummary {
		return nil, ErrNotFound
	}
	list, err := s.recordsByKind(ctx, s.pool, owner, KindFollowup)
	if err != nil {
		return nil, err
	}
	out := []PersonalRecord{}
	for _, r := range list {
		if r.ParentID == parentID {
			out = append(out, r)
		}
	}
	return out, nil
}

// UpdateFollowup replaces title/text/sections in a new revision. The
// follow-up must belong to the owner and to parentID, so the summary
// route cannot edit a note attached to another summary. A nil Sections
// keeps the stored sections for old text-only clients; an explicit empty
// sections object clears them.
func (s *Store) UpdateFollowup(ctx context.Context, owner UserID, parentID, id string, in RecordInput) (PersonalRecord, error) {
	if _, err := ParseUUID(parentID); err != nil {
		return PersonalRecord{}, ErrNotFound
	}
	return s.updateFollowup(ctx, owner, parentID, id, in)
}

// updateFollowup keeps its sealed parent link and appends a revision.
func (s *Store) updateFollowup(ctx context.Context, owner UserID, parentID, id string, in RecordInput) (PersonalRecord, error) {
	in.Kind = KindFollowup
	in.ParentID = parentID
	var result PersonalRecord
	err := s.tx(ctx, func(tx pgx.Tx) error {
		if _, err := userKind(ctx, tx, owner, true); err != nil {
			return err
		}
		before, err := s.record(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		if before.Kind != KindFollowup {
			return ErrNotFound
		}
		if parentID != "" {
			if before.ParentID != parentID {
				return ErrNotFound
			}
			parent, err := s.record(ctx, tx, owner, parentID, false)
			if err != nil {
				return err
			}
			if parent.Kind != KindSummary {
				return ErrNotFound
			}
		}
		if before.Version != in.ExpectedVersion {
			return ErrStaleVersion
		}
		if in.Sections == nil {
			in.Sections = before.Sections
		}
		if err := validateContinuity(in, 0); err != nil {
			return err
		}
		in.ParentID = before.ParentID
		result = continuityRecord(id, in, before.Version+1, before.CreatedAt, s.clock())
		return s.updateContinuityRecord(ctx, tx, owner, result)
	})
	return result, err
}
