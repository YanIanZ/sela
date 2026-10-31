package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/YanIanZ/sela/internal/vault"
)

// ExportFormat is the machine-readable export format identifier.
const ExportFormat = "sela-export/1"

// Exporter contributes one section to the owner's export. Tables lists
// the tables with a user_id column whose data the section covers (a test
// fails when such a table is claimed by no exporter). Export reads inside
// the export transaction and adds structured, decrypted data; it must
// never add hashes, keys or tokens.
type Exporter interface {
	Section() string
	Tables() []string
	Export(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) error
}

// SecurityOnlyTables have a user_id but hold only secrets (hashes, keys):
// their existence is exported, their contents never.
var SecurityOnlyTables = []string{"credentials", "recovery_codes", "user_data_keys"}

// ExportBuilder collects sections for one export.
type ExportBuilder struct {
	store    *Store
	current  SessionID
	lang     string
	sections map[string]any
}

// Put sets a section's value.
func (b *ExportBuilder) Put(section string, v any) { b.sections[section] = v }

// Vault lets an exporter decrypt the owner's fields.
func (b *ExportBuilder) Vault() *vault.Vault { return b.store.vault }

// Lang is the export language (id or en).
func (b *ExportBuilder) Lang() string { return b.lang }

// CurrentSession is the session that requested the export.
func (b *ExportBuilder) CurrentSession() SessionID { return b.current }

// RegisterExporter adds an exporter; section names are unique.
func (s *Store) RegisterExporter(e Exporter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.exporters {
		if x.Section() == e.Section() {
			return fmt.Errorf("account: exporter section %q registered twice", e.Section())
		}
	}
	s.exporters = append(s.exporters, e)
	return nil
}

// Exporters lists the registered exporters in registration order.
func (s *Store) Exporters() []Exporter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Exporter(nil), s.exporters...)
}

type exporterFunc struct {
	section string
	tables  []string
	fn      func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error)
}

func (e exporterFunc) Section() string  { return e.section }
func (e exporterFunc) Tables() []string { return e.tables }
func (e exporterFunc) Export(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) error {
	v, err := e.fn(ctx, tx, owner, b)
	if err != nil {
		return err
	}
	b.Put(e.section, v)
	return nil
}

// ExportedSession is a session in the export (never its token hash).
type ExportedSession struct {
	SessionView
	RevokedAt    *Timestamp `json:"revokedAt"`
	RevokeReason *string    `json:"revokeReason"`
}

func foundationExporters() []Exporter {
	return []Exporter{
		exporterFunc{"personalRecords", []string{"personal_records", "personal_record_versions"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			return b.store.exportRecords(ctx, tx, owner)
		}},
		exporterFunc{"profile", []string{"users"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			return b.store.profile(ctx, tx, owner)
		}},
		exporterFunc{"account", []string{"credentials", "recovery_codes"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			u, err := loadUser(ctx, tx, owner)
			if err != nil || u.username == nil {
				return nil, err
			}
			out := map[string]any{"username": *u.username, "passwordChangedAt": tsPtr(u.passwordChangedAt),
				"recoveryCodes": map[string]any{"remaining": u.remainingCodes, "generatedAt": tsPtr(u.recoveryGeneratedAt),
					"acknowledged": u.recoveryAcknowledged != nil}}
			return out, nil
		}},
		exporterFunc{"consents", []string{"consents"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			rows, err := loadConsents(ctx, tx, owner)
			if err != nil {
				return nil, err
			}
			out := []ConsentView{}
			names := make([]string, 0, len(rows))
			for n := range rows {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				c := rows[n]
				p, ok := b.store.cfg.purpose(n)
				if !ok {
					p = Purpose{Name: n}
				}
				out = append(out, b.store.consentView(p, &c))
			}
			return out, nil
		}},
		exporterFunc{"consentHistory", []string{"consent_events"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			rows, err := tx.Query(ctx, `SELECT id, purpose, decision, policy_version, source, decided_at FROM consent_events
				WHERE user_id = $1 ORDER BY decided_at DESC, id DESC`, owner)
			if err != nil {
				return nil, err
			}
			return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ConsentEvent, error) {
				var e ConsentEvent
				var id pgtype.UUID
				var at time.Time
				err := r.Scan(&id, &e.Purpose, &e.Decision, &e.PolicyVersion, &e.Source, &at)
				e.ID, e.DecidedAt = id.String(), ts(at)
				return e, err
			})
		}},
		exporterFunc{"shares", []string{"shares", "share_recipient_sessions", "share_events"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			rows, err := tx.Query(ctx, `SELECT id,summary_id,summary_version,label,purpose,state,version,expires_at,created_at,revoked_at,end_reason
				FROM shares WHERE user_id=$1 ORDER BY created_at DESC, id DESC`, owner)
			if err != nil {
				return nil, err
			}
			type event struct {
				ShareID string    `json:"shareId"`
				Kind    string    `json:"kind"`
				At      time.Time `json:"at"`
			}
			type shareExport struct {
				Share
				Events []event `json:"events"`
			}
			shares := []shareExport{}
			byID := map[string]int{}
			for rows.Next() {
				var e shareExport
				var id, summaryID pgtype.UUID
				var revokedAt *time.Time
				var reason *string
				if err := rows.Scan(&id, &summaryID, &e.SummaryVersion, &e.Label, &e.Purpose, &e.State, &e.Version, &e.ExpiresAt, &e.CreatedAt, &revokedAt, &reason); err != nil {
					rows.Close()
					return nil, err
				}
				e.ID, e.SummaryID = uuidText(id.Bytes), uuidText(summaryID.Bytes)
				byID[e.ID] = len(shares)
				shares = append(shares, e)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
			evRows, err := tx.Query(ctx, `SELECT e.share_id,e.kind,e.at FROM share_events e JOIN shares s ON s.id=e.share_id
				WHERE s.user_id=$1 ORDER BY e.at`, owner)
			if err != nil {
				return nil, err
			}
			for evRows.Next() {
				var id pgtype.UUID
				var ev event
				if err := evRows.Scan(&id, &ev.Kind, &ev.At); err != nil {
					evRows.Close()
					return nil, err
				}
				ev.ShareID = uuidText(id.Bytes)
				if i, ok := byID[ev.ShareID]; ok {
					shares[i].Events = append(shares[i].Events, ev)
				}
			}
			evRows.Close()
			return shares, evRows.Err()
		}},
		exporterFunc{"guardianConsents", []string{"guardian_consents"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			return b.store.guardianRecords(ctx, tx, owner)
		}},
		exporterFunc{"sessions", []string{"sessions"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			rows, err := tx.Query(ctx, `SELECT id, kind, trusted_device, device_browser, device_os, created_at, last_seen_at,
				idle_timeout_seconds, absolute_expires_at, revoked_at, revoke_reason FROM sessions WHERE user_id = $1 ORDER BY created_at DESC, id DESC`, owner)
			if err != nil {
				return nil, err
			}
			return pgx.CollectRows(rows, func(r pgx.CollectableRow) (ExportedSession, error) {
				var id SessionID
				var kind string
				var trusted bool
				var d Device
				var created, seen, abs time.Time
				var idle int
				var revoked *time.Time
				var reason *string
				if err := r.Scan(&id, &kind, &trusted, &d.Browser, &d.OS, &created, &seen, &idle, &abs, &revoked, &reason); err != nil {
					return ExportedSession{}, err
				}
				v := sessionView(id, id == b.current, kind, trusted, d, created, seen, time.Duration(idle)*time.Second, abs)
				return ExportedSession{SessionView: v, RevokedAt: tsPtr(revoked), RevokeReason: reason}, nil
			})
		}},
		exporterFunc{"audit", []string{"audit_events"}, func(ctx context.Context, tx pgx.Tx, owner UserID, b *ExportBuilder) (any, error) {
			list, err := b.store.auditRows(ctx, tx, b.current, `SELECT id, at, type, actor, session_id, detail FROM audit_events
				WHERE user_id = $1 ORDER BY at DESC, id DESC`, owner)
			if err != nil {
				return nil, err
			}
			out := make([]AuditEvent, len(list))
			for i, x := range list {
				out[i] = x.ev
			}
			return out, nil
		}},
	}
}

// ExportDoc is the one export model rendered as JSON, HTML and Markdown.
type ExportDoc struct {
	Format      string              `json:"format"`
	GeneratedAt Timestamp           `json:"generatedAt"`
	Environment string              `json:"environment"`
	Synthetic   bool                `json:"synthetic"`
	User        Profile             `json:"user"`
	Sections    map[string]any      `json:"sections"`
	Notices     map[string][]string `json:"notices"`
	order       []string
}

// ExportNotices are stated in both languages in every export.
var ExportNotices = map[string][]string{
	"id": {
		"Pencabutan akses menghentikan akses berikutnya; salinan yang sudah diunduh atau dilihat orang lain tidak dapat ditarik kembali.",
		"Berkas ini berisi data pribadimu dan tidak terenkripsi. Simpan di tempat yang aman dan jangan bagikan tanpa sengaja.",
		"Kata sandi, kode pemulihan, token sesi, dan kunci enkripsi tidak pernah diekspor.",
	},
	"en": {
		"Revoking access stops future access; copies already downloaded or viewed by others cannot be recalled.",
		"This file contains your personal data and is not encrypted. Keep it somewhere safe and do not share it by accident.",
		"Passwords, recovery codes, session tokens and encryption keys are never exported.",
	},
}

// ExportFile is a rendered export ready to download.
type ExportFile struct {
	Format      string
	ContentType string
	Filename    string
	Body        []byte
}

// Export assembles every registered section in one consistent snapshot
// and renders it as json, html or md. Accounts need a recent password
// proof. Writes export.created.
func (s *Store) Export(ctx context.Context, owner UserID, current SessionID, format, lang string) (ExportFile, error) {
	ext := map[string]string{"json": "application/json; charset=utf-8", "html": "text/html; charset=utf-8", "md": "text/markdown; charset=utf-8"}
	ctype, ok := ext[format]
	if !ok {
		return ExportFile{}, invalid("format")
	}
	if lang != "" && !validLocale(lang) {
		return ExportFile{}, invalid("lang")
	}
	var doc ExportDoc
	err := pgx.BeginTxFunc(ctx, s.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		if err := s.requireRecentProof(ctx, tx, owner, current); err != nil {
			return err
		}
		if err := s.audit(ctx, tx, owner, &current, "export.created", actorUser, map[string]any{"format": format}); err != nil {
			return err
		}
		p, err := s.profile(ctx, tx, owner)
		if err != nil {
			return err
		}
		if lang == "" {
			lang = p.Locale
		}
		b := &ExportBuilder{store: s, current: current, lang: lang, sections: map[string]any{}}
		for _, e := range s.Exporters() {
			if err := e.Export(ctx, tx, owner, b); err != nil {
				return fmt.Errorf("account: exporter %s: %w", e.Section(), err)
			}
			doc.order = append(doc.order, e.Section())
		}
		doc.Format, doc.GeneratedAt, doc.Environment, doc.Synthetic = ExportFormat, ts(s.clock()), s.cfg.Environment, s.cfg.Synthetic()
		doc.User, doc.Sections, doc.Notices = p, b.sections, ExportNotices
		return nil
	})
	if err != nil {
		return ExportFile{}, err
	}
	var body []byte
	switch format {
	case "json":
		body, err = json.MarshalIndent(doc, "", "  ")
	case "html":
		body, err = renderHTML(doc, lang)
	case "md":
		body, err = renderMarkdown(doc, lang)
	}
	if err != nil {
		return ExportFile{}, err
	}
	name := "sela-export-" + time.Time(doc.GeneratedAt).UTC().Format("20060102") + "." + format
	return ExportFile{Format: format, ContentType: ctype, Filename: name, Body: body}, nil
}

var sectionTitles = map[string][2]string{
	"profile":          {"Profil", "Profile"},
	"account":          {"Akun", "Account"},
	"consents":         {"Persetujuan", "Consents"},
	"consentHistory":   {"Riwayat persetujuan", "Consent history"},
	"guardianConsents": {"Persetujuan orang tua atau wali", "Parent or guardian consent"},
	"sessions":         {"Sesi masuk", "Sign-in sessions"},
	"audit":            {"Catatan aktivitas keamanan", "Security activity log"},
}

func title(section, lang string) string {
	t, ok := sectionTitles[section]
	if !ok {
		return section
	}
	if lang == "en" {
		return t[1]
	}
	return t[0]
}

// tree turns a value into maps, slices and scalars for the renderers.
func tree(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out any
	err = dec.Decode(&out)
	return out, err
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func scalarText(v any, lang string) string {
	switch x := v.(type) {
	case nil:
		return "—"
	case bool:
		if x {
			return map[string]string{"id": "ya", "en": "yes"}[lang]
		}
		return map[string]string{"id": "tidak", "en": "no"}[lang]
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func htmlValue(b *strings.Builder, v any, lang string) {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			b.WriteString("<p>—</p>")
			return
		}
		b.WriteString("<dl>")
		for _, k := range sortedKeys(x) {
			b.WriteString("<dt>" + html.EscapeString(k) + "</dt><dd>")
			htmlValue(b, x[k], lang)
			b.WriteString("</dd>")
		}
		b.WriteString("</dl>")
	case []any:
		if len(x) == 0 {
			b.WriteString("<p>—</p>")
			return
		}
		b.WriteString("<ol>")
		for _, e := range x {
			b.WriteString("<li>")
			htmlValue(b, e, lang)
			b.WriteString("</li>")
		}
		b.WriteString("</ol>")
	default:
		b.WriteString(html.EscapeString(scalarText(x, lang)))
	}
}

func renderHTML(doc ExportDoc, lang string) ([]byte, error) {
	l := map[string][2]string{
		"title": {"Ekspor data Sela", "Sela data export"}, "generated": {"Dibuat", "Generated"},
		"env": {"Lingkungan", "Environment"}, "synthetic": {"Data sintetis (bukan data nyata)", "Synthetic data (not real data)"},
		"notices": {"Penting", "Important"},
	}
	pick := func(k string) string {
		if lang == "en" {
			return l[k][1]
		}
		return l[k][0]
	}
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="` + html.EscapeString(lang) + `"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1"><title>` + html.EscapeString(pick("title")) + `</title>`)
	b.WriteString(`<style>body{font:16px/1.5 system-ui,sans-serif;max-width:52rem;margin:2rem auto;padding:0 1rem;color:#1f2328;background:#fff}` +
		`h1{font-size:1.6rem}h2{font-size:1.25rem;margin-top:2rem;border-bottom:1px solid #d0d7de}dl{display:grid;grid-template-columns:max-content 1fr;gap:.25rem 1rem}` +
		`dt{font-weight:600}dd{margin:0}ol{padding-left:1.25rem}li{margin:.5rem 0}.note{background:#fff8c5;padding:.75rem;border-radius:.375rem}</style></head><body><main>`)
	b.WriteString("<h1>" + html.EscapeString(pick("title")) + "</h1>")
	b.WriteString("<p>" + html.EscapeString(pick("generated")) + ": " + html.EscapeString(doc.GeneratedAt.String()) + " · " +
		html.EscapeString(pick("env")) + ": " + html.EscapeString(doc.Environment) + "</p>")
	if doc.Synthetic {
		b.WriteString(`<p class="note">` + html.EscapeString(pick("synthetic")) + "</p>")
	}
	for _, sec := range doc.order {
		t, err := tree(doc.Sections[sec])
		if err != nil {
			return nil, err
		}
		b.WriteString("<section><h2>" + html.EscapeString(title(sec, lang)) + "</h2>")
		htmlValue(&b, t, lang)
		b.WriteString("</section>")
	}
	b.WriteString("<section><h2>" + html.EscapeString(pick("notices")) + "</h2><ul>")
	for _, n := range doc.Notices[lang] {
		b.WriteString("<li>" + html.EscapeString(n) + "</li>")
	}
	b.WriteString("</ul></section></main></body></html>\n")
	return []byte(b.String()), nil
}

// mdEscape escapes Markdown control characters in a value so user text
// cannot add links, images, headings or HTML.
func mdEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '`', '*', '_', '{', '}', '[', ']', '(', ')', '#', '+', '-', '.', '!', '|', '<', '>', '~':
			b.WriteRune('\\')
		case '\n', '\r':
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func mdValue(b *strings.Builder, v any, indent int, lang string) {
	pad := strings.Repeat("  ", indent)
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			b.WriteString(pad + "- —\n")
			return
		}
		for _, k := range sortedKeys(x) {
			switch c := x[k].(type) {
			case map[string]any, []any:
				b.WriteString(pad + "- **" + mdEscape(k) + "**:\n")
				mdValue(b, c, indent+1, lang)
			default:
				b.WriteString(pad + "- **" + mdEscape(k) + "**: " + mdEscape(scalarText(c, lang)) + "\n")
			}
		}
	case []any:
		if len(x) == 0 {
			b.WriteString(pad + "- —\n")
			return
		}
		for i, e := range x {
			b.WriteString(fmt.Sprintf("%s%d\\.\n", pad+"- ", i+1))
			mdValue(b, e, indent+1, lang)
		}
	default:
		b.WriteString(pad + "- " + mdEscape(scalarText(x, lang)) + "\n")
	}
}

func renderMarkdown(doc ExportDoc, lang string) ([]byte, error) {
	var b strings.Builder
	head := map[string][3]string{"id": {"Ekspor data Sela", "Dibuat", "Penting"}, "en": {"Sela data export", "Generated", "Important"}}[lang]
	if head[0] == "" {
		return nil, errors.New("account: unknown export language")
	}
	b.WriteString("# " + head[0] + "\n\n")
	b.WriteString(head[1] + ": " + mdEscape(doc.GeneratedAt.String()) + " · " + mdEscape(doc.Environment) + "\n\n")
	for _, sec := range doc.order {
		t, err := tree(doc.Sections[sec])
		if err != nil {
			return nil, err
		}
		b.WriteString("## " + mdEscape(title(sec, lang)) + "\n\n")
		mdValue(&b, t, 0, lang)
		b.WriteString("\n")
	}
	b.WriteString("## " + head[2] + "\n\n")
	for _, n := range doc.Notices[lang] {
		b.WriteString("- " + mdEscape(n) + "\n")
	}
	return []byte(b.String()), nil
}
