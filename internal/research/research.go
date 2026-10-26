// Package research serves the reviewer-facing research corpus built by
// scripts/research/corpus_pipeline.py (data/research/corpus/). It never
// reads session data. Corpus strings are data: no model sees them and the
// frontend renders them as text, so instructions inside a record cannot act.
//
// Search is deterministic BM25 over title, authors, journal, theme and the
// register's own annotations (not abstracts or full text, which are not
// committed).
package research

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Record is one knowledge-base row. Unknown values are null in the file and
// zero/nil here; the JSON keeps them null for the UI.
type Record struct {
	ID                   string           `json:"id"`
	Title                string           `json:"title"`
	Authors              []string         `json:"authors"`
	Year                 *int             `json:"year"`
	PublicationDate      *string          `json:"publication_date"`
	Journal              *string          `json:"journal"`
	DOI                  *string          `json:"doi"`
	URL                  *string          `json:"url"`
	Theme                *string          `json:"theme"`
	NoteID               *string          `json:"note_id"`
	TransferLimitationID *string          `json:"transfer_limitation_id"`
	StudyType            *string          `json:"study_type"`
	Population           []string         `json:"population"`
	CountryOrLanguage    []string         `json:"country_or_language"`
	SampleSize           *int             `json:"sample_size"`
	License              *string          `json:"license"`
	AccessStatus         *string          `json:"access_status"`
	ScreeningStatus      *string          `json:"screening_status"`
	ReviewerStatus       *string          `json:"reviewer_status"`
	WithinCutoff         *bool            `json:"within_cutoff"`
	Flags                []string         `json:"flags"`
	Evidence             []map[string]any `json:"evidence"`
	IndexVersion         *string          `json:"index_version"`
	// Extra keeps any additional fields the pipeline exports.
	Extra map[string]json.RawMessage `json:"-"`
}

// Hit is a search result.
type Hit struct {
	Record *Record `json:"record"`
	Score  float64 `json:"score"`
}

// SearchResult is a page of hits plus facet values.
type SearchResult struct {
	Total        int      `json:"total"`
	Results      []Hit    `json:"results"`
	IndexVersion string   `json:"indexVersion"`
	Themes       []string `json:"themes"`
	Statuses     []string `json:"statuses"`
}

// Index is a loaded, immutable corpus.
type Index struct {
	records  []*Record
	byID     map[string]*Record
	terms    []map[string]int
	lengths  []int
	avgLen   float64
	df       map[string]int
	manifest map[string]any
	version  string
	themes   []string
	statuses []string
}

const maxRecords = 10000

// Load reads knowledge_base.jsonl and the sibling manifest.json.
func Load(path string) (*Index, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("research: %w", err)
	}
	x := &Index{byID: map[string]*Record{}, df: map[string]int{}}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		b := bytes.TrimSpace(sc.Bytes())
		if len(b) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("research: line %d: %w", line, err)
		}
		if r.ID == "" || r.Title == "" {
			return nil, fmt.Errorf("research: line %d: id and title are required", line)
		}
		if x.byID[r.ID] != nil {
			return nil, fmt.Errorf("research: duplicate id %q", r.ID)
		}
		if len(x.records) >= maxRecords {
			return nil, errors.New("research: too many records")
		}
		rec := r
		x.records = append(x.records, &rec)
		x.byID[rec.ID] = &rec
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(x.records) == 0 {
		return nil, errors.New("research: empty knowledge base")
	}
	if m, err := os.ReadFile(filepath.Join(filepath.Dir(path), "manifest.json")); err == nil {
		_ = json.Unmarshal(m, &x.manifest)
	}
	if v, ok := x.manifest["index_version"].(string); ok {
		x.version = v
	} else if x.records[0].IndexVersion != nil {
		x.version = *x.records[0].IndexVersion
	}
	themeSet, statusSet := map[string]bool{}, map[string]bool{}
	total := 0
	for _, r := range x.records {
		tf := map[string]int{}
		n := 0
		for _, tok := range tokenize(searchText(r)) {
			tf[tok]++
			n++
		}
		x.terms = append(x.terms, tf)
		x.lengths = append(x.lengths, n)
		total += n
		for tok := range tf {
			x.df[tok]++
		}
		if r.Theme != nil {
			themeSet[*r.Theme] = true
		}
		if r.ScreeningStatus != nil {
			statusSet[*r.ScreeningStatus] = true
		}
	}
	x.avgLen = float64(total) / float64(len(x.records))
	x.themes, x.statuses = sortedKeys(themeSet), sortedKeys(statusSet)
	return x, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func searchText(r *Record) string {
	parts := []string{r.ID, r.Title, strings.Join(r.Authors, " "), deref(r.Journal), deref(r.Theme), deref(r.NoteID), deref(r.TransferLimitationID), deref(r.StudyType), strings.Join(r.Population, " "), strings.Join(r.CountryOrLanguage, " ")}
	return strings.Join(parts, " ")
}

// tokenize lowercases and splits on non-letters/digits; tokens of one rune
// are dropped. Indonesian and English share one tokenizer.
func tokenize(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := f[:0]
	for _, t := range f {
		if len([]rune(t)) > 1 {
			out = append(out, t)
		}
	}
	return out
}

// Manifest returns the corpus manifest (as exported by the pipeline).
func (x *Index) Manifest() any {
	if x.manifest == nil {
		return map[string]any{"index_version": x.version, "records": len(x.records)}
	}
	return x.manifest
}

// Record returns one record by id.
func (x *Index) Record(id string) (any, bool) {
	r, ok := x.byID[id]
	return r, ok
}

// Search ranks records by BM25 for q (empty q lists all in id order),
// filtered by exact theme and screening status. limit is clamped to 1..100.
func (x *Index) Search(q, theme, status string, limit int) any {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	terms := tokenize(q)
	var hits []Hit
	const k1, b = 1.2, 0.75
	n := float64(len(x.records))
	for i, r := range x.records {
		if theme != "" && deref(r.Theme) != theme {
			continue
		}
		if status != "" && deref(r.ScreeningStatus) != status {
			continue
		}
		score := 0.0
		for _, t := range terms {
			tf := float64(x.terms[i][t])
			if tf == 0 {
				continue
			}
			df := float64(x.df[t])
			idf := math.Log(1 + (n-df+0.5)/(df+0.5))
			score += idf * (tf * (k1 + 1)) / (tf + k1*(1-b+b*float64(x.lengths[i])/x.avgLen))
		}
		if len(terms) > 0 && score == 0 {
			continue
		}
		hits = append(hits, Hit{Record: r, Score: math.Round(score*1000) / 1000})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Record.ID < hits[j].Record.ID
	})
	total := len(hits)
	if len(hits) > limit {
		hits = hits[:limit]
	}
	if hits == nil {
		hits = []Hit{}
	}
	return SearchResult{Total: total, Results: hits, IndexVersion: x.version, Themes: x.themes, Statuses: x.statuses}
}

// IDs returns all record ids in file order (for validation tools).
func (x *Index) IDs() []string {
	out := make([]string, 0, len(x.records))
	for _, r := range x.records {
		out = append(out, r.ID)
	}
	return out
}
