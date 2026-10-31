// Package reading serves the editorial reading library. It parses the
// authoring content set (data/content/reading) and applies the runtime
// editorial gate on every read: an item exists only when a named human
// editor approved its exact text, the authoring-time verifier said
// "supported", and the review is marked publishable.
package reading

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"time"

	readingdata "github.com/YanIanZ/sela/data/content/reading"
)

// Localized is one bilingual string from the content set.
type Localized struct {
	ID string `json:"id"`
	EN string `json:"en"`
}

// LocalizedList is one bilingual list (steps).
type LocalizedList struct {
	ID []string `json:"id"`
	EN []string `json:"en"`
}

// Minutes is a bilingual reading-time estimate.
type Minutes struct {
	ID int `json:"id"`
	EN int `json:"en"`
}

// SourceRef is the public attribution of one source: no user data.
type SourceRef struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Publisher string `json:"publisher"`
	URL       string `json:"url"`
	Licence   string `json:"licence"`
}

// Section is one guide section with plain paragraphs.
type Section struct {
	Heading    Localized   `json:"heading"`
	Paragraphs []Localized `json:"paragraphs"`
}

// Licence is the item's public licence information.
type Licence struct {
	ItemTextLicence string `json:"itemTextLicence"`
	SourceLicence   string `json:"sourceLicence"`
	CommercialUse   string `json:"commercialUse"`
	Note            string `json:"note,omitempty"`
}

// Review is the public review record. The reviewer's name stays internal;
// only the approval date and verifier verdict are served.
type Review struct {
	Status     string `json:"status"`
	ReviewedOn string `json:"reviewedOn"`
	Verifier   string `json:"verifier"`
}

// ListItem is one approved item in GET /api/v3/reading.
type ListItem struct {
	ID                 string      `json:"id"`
	Version            int         `json:"version"`
	Kind               string      `json:"kind"`
	Topics             []string    `json:"topics"`
	Audience           string      `json:"audience"`
	Title              Localized   `json:"title"`
	Summary            Localized   `json:"summary"`
	ReadingTimeMinutes Minutes     `json:"readingTimeMinutes"`
	Sources            []SourceRef `json:"sources"`
	ReviewedOn         string      `json:"reviewedOn"`
}

// SourceInfo describes how the item relates to its sources.
type SourceInfo struct {
	Relation string      `json:"relation"`
	Locator  string      `json:"locator"`
	Sources  []SourceRef `json:"sources"`
}

// Detail is GET /api/v3/reading/{slug}: the full approved item.
type Detail struct {
	ID                 string         `json:"id"`
	Version            int            `json:"version"`
	Kind               string         `json:"kind"`
	Topics             []string       `json:"topics"`
	Audience           string         `json:"audience"`
	Title              Localized      `json:"title"`
	Summary            Localized      `json:"summary"`
	Steps              *LocalizedList `json:"steps,omitempty"`
	Why                *Localized     `json:"why,omitempty"`
	SeekHelpWhen       *Localized     `json:"seekHelpWhen,omitempty"`
	Cautions           *Localized     `json:"cautions,omitempty"`
	Sections           []Section      `json:"sections,omitempty"`
	ReadingTimeMinutes Minutes        `json:"readingTimeMinutes"`
	Source             SourceInfo     `json:"source"`
	Licence            Licence        `json:"licence"`
	Attribution        Localized      `json:"attribution"`
	AdaptationNotice   *Localized     `json:"adaptationNotice,omitempty"`
	AdaptationNote     string         `json:"adaptationNote,omitempty"`
	EvidenceNote       string         `json:"evidenceNote,omitempty"`
	Review             Review         `json:"review"`
}

// EmbeddedReview is the authoring-time review record for one item, shown to
// operators only. The reviewer name is editorial metadata, never user data.
type EmbeddedReview struct {
	Status             string `json:"status"`
	HumanEditor        string `json:"humanEditor,omitempty"`
	ReviewedOn         string `json:"reviewedOn,omitempty"`
	ApprovedTextSHA256 string `json:"approvedTextSha256,omitempty"`
	TeenReview         string `json:"teenReview,omitempty"`
	VerifierStatus     string `json:"verifierStatus"`
	VerifierNote       string `json:"verifierNote,omitempty"`
	Publishable        bool   `json:"publishable"`
}

// ReviewItem is one item as the operator preview needs it: the full detail
// (whether or not it is published) plus the hash an approval must bind to.
type ReviewItem struct {
	Slug           string         `json:"slug"`
	Detail         Detail         `json:"detail"`
	ContentHash    string         `json:"contentHash"`
	EmbeddedReview EmbeddedReview `json:"embeddedReview"`
}

// AdminItem is one row of GET /api/admin/v1/reading. Approval is the current
// or most recent database row; HashMatches says whether that row still binds
// the embedded text; Published is the public gate's answer.
type AdminItem struct {
	ReviewItem
	Approval    *Approval `json:"approval,omitempty"`
	HashMatches bool      `json:"hashMatches"`
	Published   bool      `json:"published"`
}

// AdminItems returns every embedded item for human review, in content-set
// order, including unpublished and merely drafted items.
func (l *Library) AdminItems(approvals map[string]Approval) []AdminItem {
	out := make([]AdminItem, 0, len(l.items))
	for _, it := range l.items {
		rv := Review{
			Status:     it.Review.Status,
			ReviewedOn: deref(it.Review.ReviewedOn),
			Verifier:   it.Review.Verifier.Status,
		}
		item := AdminItem{
			ReviewItem: ReviewItem{
				Slug:        it.ID,
				Detail:      l.detail(it, rv),
				ContentHash: it.contentHash,
				EmbeddedReview: EmbeddedReview{
					Status:             it.Review.Status,
					HumanEditor:        deref(it.Review.HumanEditor),
					ReviewedOn:         deref(it.Review.ReviewedOn),
					ApprovedTextSHA256: it.Review.ApprovedTextSHA256,
					TeenReview:         it.Review.TeenReview,
					VerifierStatus:     it.Review.Verifier.Status,
					VerifierNote:       it.Review.Verifier.Note,
					Publishable:        it.Review.Publishable,
				},
			},
		}
		if ap, ok := approvals[it.ID]; ok {
			a := ap
			item.Approval = &a
			item.HashMatches = ap.ContentHash == it.contentHash
		}
		_, item.Published = l.review(it, approvals)
		out = append(out, item)
	}
	return out
}

type setDoc struct {
	Version string `json:"version"`
	Files   []struct {
		Path string `json:"path"`
	} `json:"files"`
}

type fileDoc struct {
	Items []json.RawMessage `json:"items"`
}

type item struct {
	ID       string   `json:"id"`
	Version  int      `json:"version"`
	Kind     string   `json:"kind"`
	Topics   []string `json:"topics"`
	Audience string   `json:"audience"`
	// contentHash is the SHA-256 of the exact raw item object from the
	// embedded file. It is not part of the public JSON.
	contentHash string
	Content     struct {
		Title        Localized      `json:"title"`
		Summary      Localized      `json:"summary"`
		Steps        *LocalizedList `json:"steps"`
		Why          *Localized     `json:"why"`
		SeekHelpWhen *Localized     `json:"seek_help_when"`
		Cautions     *Localized     `json:"cautions"`
		Sections     []struct {
			Heading    Localized `json:"heading"`
			Paragraphs []struct {
				ID string `json:"id"`
				EN string `json:"en"`
			} `json:"paragraphs"`
		} `json:"sections"`
	} `json:"content"`
	ReadingTimeMinutes Minutes `json:"reading_time_minutes"`
	Source             struct {
		SourceIDs []string `json:"source_ids"`
		Relation  string   `json:"relation"`
		Locator   string   `json:"locator"`
	} `json:"source"`
	Licence struct {
		SourceLicence   string `json:"source_licence"`
		ItemTextLicence string `json:"item_text_licence"`
		CommercialUse   string `json:"commercial_use"`
		Note            string `json:"note"`
	} `json:"licence"`
	Attribution      Localized  `json:"attribution"`
	AdaptationNotice *Localized `json:"adaptation_notice"`
	AdaptationNote   string     `json:"adaptation_note"`
	EvidenceNote     string     `json:"evidence_note"`
	Review           struct {
		Status             string  `json:"status"`
		HumanEditor        *string `json:"human_editor"`
		ReviewedOn         *string `json:"reviewed_on"`
		ApprovedTextSHA256 string  `json:"approved_text_sha256"`
		TeenReview         string  `json:"teen_review"`
		Verifier           struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		} `json:"verifier"`
		Publishable bool `json:"publishable"`
	} `json:"review"`
}

type sourceEntry struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Publisher     string `json:"publisher"`
	URL           string `json:"url"`
	PersistentURL string `json:"persistent_url"`
	Licence       string `json:"licence"`
}

// Library is the parsed content set and its source index.
type Library struct {
	items   []item
	bySlug  map[string]int
	sources map[string]sourceEntry
}

// LoadFS parses a content set rooted at fsys. It is exported so tests can
// exercise the gate with fixtures; production uses Embedded.
func LoadFS(fsys fs.FS) (*Library, error) {
	var set setDoc
	if err := readJSON(fsys, "set.json", &set); err != nil {
		return nil, err
	}
	if len(set.Files) == 0 {
		return nil, fmt.Errorf("reading: set.json lists no content files")
	}
	var sources struct {
		Sources []sourceEntry `json:"sources"`
	}
	if err := readJSON(fsys, "sources.json", &sources); err != nil {
		return nil, err
	}
	lib := &Library{sources: make(map[string]sourceEntry, len(sources.Sources)), bySlug: make(map[string]int)}
	for _, s := range sources.Sources {
		if s.ID != "" {
			lib.sources[s.ID] = s
		}
	}
	seen := make(map[string]bool)
	for _, f := range set.Files {
		var doc fileDoc
		if err := readJSON(fsys, f.Path, &doc); err != nil {
			return nil, err
		}
		for _, raw := range doc.Items {
			var it item
			if err := json.Unmarshal(raw, &it); err != nil {
				return nil, fmt.Errorf("reading: %s has an unreadable item: %w", f.Path, err)
			}
			if it.ID == "" {
				return nil, fmt.Errorf("reading: %s has an item with an empty id", f.Path)
			}
			if seen[it.ID] {
				return nil, fmt.Errorf("reading: duplicate item id %q", it.ID)
			}
			seen[it.ID] = true
			// Bind approvals to the exact bytes of the item object. A text
			// or formatting change therefore changes the hash.
			sum := sha256.Sum256(raw)
			it.contentHash = hex.EncodeToString(sum[:])
			lib.bySlug[it.ID] = len(lib.items)
			lib.items = append(lib.items, it)
		}
	}
	return lib, nil
}

func readJSON(fsys fs.FS, name string, dst any) error {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("reading: %s: %w", name, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("reading: %s: %w", name, err)
	}
	return nil
}

// List returns only published items, in content-set order. The result is an
// empty slice, never nil, so the API always answers {"items":[]}.
func (l *Library) List() []ListItem { return l.ListWith(nil) }

// ListWith applies the editorial gate with the operator approvals loaded
// from PostgreSQL. An item is published when its embedded review is already
// human-approved, or when a current operator approval binds the exact
// content hash and the verifier verdict is supported. A revoked approval is
// a veto, even for an item whose embedded file still claims approval.
func (l *Library) ListWith(approvals map[string]Approval) []ListItem {
	out := make([]ListItem, 0, len(l.items))
	for _, it := range l.items {
		rv, ok := l.review(it, approvals)
		if !ok {
			continue
		}
		out = append(out, l.listItem(it, rv))
	}
	return out
}

// Get returns one published item. Anything else, including an unpublished or
// forged entry, is absent.
func (l *Library) Get(slug string) (Detail, bool) { return l.GetWith(slug, nil) }

// GetWith is Get with the current operator approvals.
func (l *Library) GetWith(slug string, approvals map[string]Approval) (Detail, bool) {
	idx, ok := l.bySlug[slug]
	if !ok {
		return Detail{}, false
	}
	it := l.items[idx]
	rv, ok := l.review(it, approvals)
	if !ok {
		return Detail{}, false
	}
	return l.detail(it, rv), true
}

// ContentHash returns the exact-content hash operators must approve.
func (l *Library) ContentHash(slug string) (string, bool) {
	idx, ok := l.bySlug[slug]
	if !ok {
		return "", false
	}
	return l.items[idx].contentHash, true
}

// review decides publication for one item and returns the public review
// record to serve.
func (l *Library) review(it item, approvals map[string]Approval) (Review, bool) {
	ap, hasApproval := approvals[it.ID]
	if hasApproval && ap.RevokedAt != nil {
		return Review{}, false
	}
	if approved(it) {
		return Review{
			Status:     it.Review.Status,
			ReviewedOn: deref(it.Review.ReviewedOn),
			Verifier:   it.Review.Verifier.Status,
		}, true
	}
	if !hasApproval || !ap.publishes(it.contentHash) {
		return Review{}, false
	}
	return Review{Status: StatusHumanApproved, ReviewedOn: ap.ReviewedOn, Verifier: ap.VerifierStatus}, true
}

// StatusHumanApproved is the review status an operator approval publishes.
const StatusHumanApproved = "human_approved"

// approved is the runtime editorial gate for the embedded file alone. It is
// deliberately independent of the authoring validator: publishable data must
// also carry a named human editor, a real review date and the authoring-time
// verifier's "supported" verdict.
func approved(it item) bool {
	r := it.Review
	if it.ID == "" || r.Status != StatusHumanApproved || !r.Publishable {
		return false
	}
	if r.HumanEditor == nil || strings.TrimSpace(*r.HumanEditor) == "" {
		return false
	}
	if r.ReviewedOn == nil || !validReviewDate(*r.ReviewedOn) {
		return false
	}
	return r.Verifier.Status == "supported"
}

func validReviewDate(value string) bool {
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (l *Library) listItem(it item, rv Review) ListItem {
	return ListItem{
		ID:                 it.ID,
		Version:            it.Version,
		Kind:               it.Kind,
		Topics:             it.Topics,
		Audience:           it.Audience,
		Title:              it.Content.Title,
		Summary:            it.Content.Summary,
		ReadingTimeMinutes: it.ReadingTimeMinutes,
		Sources:            l.refs(it.Source.SourceIDs),
		ReviewedOn:         rv.ReviewedOn,
	}
}

func (l *Library) detail(it item, rv Review) Detail {
	d := Detail{
		ID:                 it.ID,
		Version:            it.Version,
		Kind:               it.Kind,
		Topics:             it.Topics,
		Audience:           it.Audience,
		Title:              it.Content.Title,
		Summary:            it.Content.Summary,
		Steps:              it.Content.Steps,
		Why:                it.Content.Why,
		SeekHelpWhen:       it.Content.SeekHelpWhen,
		Cautions:           it.Content.Cautions,
		ReadingTimeMinutes: it.ReadingTimeMinutes,
		Source: SourceInfo{
			Relation: it.Source.Relation,
			Locator:  it.Source.Locator,
			Sources:  l.refs(it.Source.SourceIDs),
		},
		Licence: Licence{
			ItemTextLicence: it.Licence.ItemTextLicence,
			SourceLicence:   it.Licence.SourceLicence,
			CommercialUse:   it.Licence.CommercialUse,
			Note:            it.Licence.Note,
		},
		Attribution:      it.Attribution,
		AdaptationNotice: it.AdaptationNotice,
		AdaptationNote:   it.AdaptationNote,
		EvidenceNote:     it.EvidenceNote,
		Review:           rv,
	}
	for _, s := range it.Content.Sections {
		sec := Section{Heading: s.Heading, Paragraphs: make([]Localized, 0, len(s.Paragraphs))}
		for _, p := range s.Paragraphs {
			sec.Paragraphs = append(sec.Paragraphs, Localized{ID: p.ID, EN: p.EN})
		}
		d.Sections = append(d.Sections, sec)
	}
	return d
}

func (l *Library) refs(ids []string) []SourceRef {
	out := make([]SourceRef, 0, len(ids))
	for _, id := range ids {
		s, ok := l.sources[id]
		if !ok {
			continue
		}
		url := s.URL
		if s.PersistentURL != "" {
			url = s.PersistentURL
		}
		out = append(out, SourceRef{ID: s.ID, Title: s.Title, Publisher: s.Publisher, URL: url, Licence: s.Licence})
	}
	return out
}

var embedded = sync.OnceValue(func() *Library {
	lib, err := LoadFS(readingdata.FS)
	if err != nil {
		panic("reading: embedded content set: " + err.Error())
	}
	return lib
})

// Embedded returns the content set compiled into the binary.
func Embedded() *Library { return embedded() }
