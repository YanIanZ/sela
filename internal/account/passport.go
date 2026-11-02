package account

import (
	"context"
	"errors"
	"time"
)

// Care Passport (BW-005) is a presentation of an existing consultation
// summary, not a new record: no table, no token and no second projection.
// The preview below is computed on read from the summary, its pinned
// sources and projectSummary, the same function share and care-request
// creation freeze.

// Passport states shown to the owner. needs_review covers a stale (MIE)
// version and a pinned source that changed or disappeared; such a version
// can be neither approved nor exported or shared as approved.
const (
	PassportDraft       = "draft"
	PassportApproved    = "approved"
	PassportNeedsReview = "needs_review"
)

// PassportSource is one owner-selected source as the owner sees it. Label
// is the owner's own (unmasked) source title; Current says whether the
// pinned version is still the source's current version.
type PassportSource struct {
	RecordID string `json:"recordId"`
	Version  int    `json:"version"`
	Kind     string `json:"kind,omitempty"`
	Label    string `json:"label"`
	Current  bool   `json:"current"`
}

// RecipientContent is the content part of SharedProjection: exactly the
// fields a recipient of this version would receive, without share metadata
// (no share id, label, purpose or times exist before a share does).
type RecipientContent struct {
	SummaryVersion int                 `json:"summaryVersion"`
	Title          string              `json:"title"`
	Text           string              `json:"text"`
	Prepared       *SummaryPrepared    `json:"prepared,omitempty"`
	Items          []SharedSummaryItem `json:"items"`
	Sources        []string            `json:"sources"`
}

// RecipientContentOf copies the content fields of a projection; it never
// recomputes them.
func RecipientContentOf(p SharedProjection) RecipientContent {
	items := p.Items
	if items == nil {
		items = []SharedSummaryItem{}
	}
	sources := p.Sources
	if sources == nil {
		sources = []string{}
	}
	return RecipientContent{SummaryVersion: p.SummaryVersion, Title: p.Title, Text: p.Text,
		Prepared: clonePrepared(p.Prepared), Items: items, Sources: sources}
}

// PassportPreview is the owner-only Care Passport view of one summary
// version. Every field is server-authoritative (CP-10).
type PassportPreview struct {
	SummaryID       string           `json:"summaryId"`
	SummaryVersion  int              `json:"summaryVersion"`
	State           string           `json:"state"`
	ApprovedVersion int              `json:"approvedVersion,omitempty"`
	Shareable       bool             `json:"shareable"`
	Stale           *StaleInfo       `json:"stale,omitempty"`
	Sources         []PassportSource `json:"sources"`
	Recipient       RecipientContent `json:"recipient"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// PassportPreview returns the owner's preview of one summary. An unknown
// or foreign id is ErrNotFound, like every other summary route. It writes
// nothing.
func (s *Store) PassportPreview(ctx context.Context, owner UserID, id string) (PassportPreview, error) {
	r, err := s.Summary(ctx, owner, id)
	if err != nil {
		return PassportPreview{}, err
	}
	sources := make([]PassportSource, 0, len(r.SourceRefs))
	allCurrent := true
	for _, ref := range r.SourceRefs {
		ps := PassportSource{RecordID: ref.RecordID, Version: ref.Version}
		if src, err := s.record(ctx, s.pool, owner, ref.RecordID, false); err == nil {
			ps.Kind, ps.Label = src.Kind, src.Title
			switch src.Kind {
			case KindMemory, KindSummary, KindFollowup:
			default:
				ps.Current = src.Version == ref.Version
			}
		} else if !errors.Is(err, ErrNotFound) {
			return PassportPreview{}, err
		}
		allCurrent = allCurrent && ps.Current
		sources = append(sources, ps)
	}
	projection, err := s.projectSummary(ctx, s.pool, owner, r)
	if err != nil {
		return PassportPreview{}, err
	}
	out := PassportPreview{SummaryID: r.ID, SummaryVersion: r.Version, ApprovedVersion: r.ApprovedVersion,
		Stale: r.Stale, Sources: sources, Recipient: RecipientContentOf(projection), UpdatedAt: r.UpdatedAt}
	switch {
	case r.Stale != nil || !allCurrent:
		out.State = PassportNeedsReview
	case r.SummaryState == SummaryApproved && r.ApprovedVersion == r.Version:
		out.State = PassportApproved
	default:
		out.State = PassportDraft
	}
	out.Shareable = out.State == PassportApproved
	return out, nil
}
