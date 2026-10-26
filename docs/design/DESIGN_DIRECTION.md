# Sela v3 design direction

Status: **design decision** by the design lead, 2026-10-26, revised the same day after review rounds 1 and 2 ([§11](#11-revision-log)). **Not implemented.** This folder (`docs/design/**`) is owned by the design lead; code, API and other docs are owned by other roles and are only referenced here.

Labels used in all design documents:

- **user-confirmed**: stated in owner brief v3.
- **PM decision**: D-026 to D-050 in the PM decision record (decided; this design does not reopen them).
- **design decision**: taken here; the design lead may revise it with a recorded reason.
- **externally sourced**: from a cited external document (W3C and others).
- **planned**: no code yet.
- **pending verification**: needs a check by someone else (legal, owner, engineering, real users).

Companion documents:

- [INFORMATION_ARCHITECTURE.md](INFORMATION_ARCHITECTURE.md): places, navigation, routes, access, help placement, how the v2 flow moves.
- [SCREENS.md](SCREENS.md): one section per screen.
- [CONTENT_STYLE.md](CONTENT_STYLE.md): voice, plain language, AI reply style, microcopy.
- [ACCESSIBILITY_CRITERIA.md](ACCESSIBILITY_CRITERIA.md): all 55 WCAG 2.2 A/AA criteria as acceptance criteria.
- [tokens.css](tokens.css): the reference token sheet.
- [tools/contrast.py](tools/contrast.py) and [tools/sync_docs.py](tools/sync_docs.py): compute the contrast tables and refresh the generated sections of this file. Both run now with the Python standard library.

Inputs read: owner brief v3 §4–§7 (user-confirmed); PM decisions D-026–D-050; the discovery reports (v2 gap analysis, v1 in-repo inventory, compliance and accessibility facts, research audit); [API_SPEC_V3](../API_SPEC_V3.md); [ADR-0002](../adr/ADR-0002-persistence-accounts-and-encryption.md); [ADR-0003](../adr/ADR-0003-ai-routing-verifier-fallback.md) (AI routing; it appeared during this pass and was read to align provider labels, consent timing and the minors rules); the v2 frontend (`web/src`: pages, components, `styles.css`, i18n); the v1 UI (`apps/web/src`: pages, Tailwind tokens in `app/globals.css`, `components/ui/AccessibilityMenu.tsx`, `components/ui/QuickExitButton.tsx`, `components/Navbar.tsx`).

---

## 1. What the design has to hold

### 1.1 Identity

Sela: "A little space to feel heard" / "Ruang sejenak untuk didengar" (user-confirmed). In Indonesian, *sela* is the gap between things: *di sela-sela kesibukan* means "in the gaps of a busy day". The product is that gap. It is a short, private pause in which a person is listened to. They can look back at what they said, and carry what they choose to someone who can help.

Listening is the core function. Preparing for a professional is offered, never required (brief §1 and §5, user-confirmed; D-035).

### 1.2 Situations, not age personas

The brief says not to assume that older users share the same limits or preferences (§6). So the design works from situations. Each one can happen to anyone aged 15 to 50+.

| Situation | What the person needs from the interface | Design response |
|---|---|---|
| A heavy evening; they want to talk now | Start fast. No profile, no long form | **Returning:** "Mulai bercerita" on Home opens the room with the person's own defaults and asks nothing. **First visit:** three short screens (age, where data is kept with the default for conversations, AI replies) before the room, so at most four screens from "Mulai" to the composer (measured in journey J1). The intent picker is optional |
| A shared phone or an unsafe home | Leave in one action; nothing on screen gives them away | "Keluar cepat" on every screen and as the second item of every full-height sheet, Esc twice (with an armed hint; can be turned off), Quick exit that can also sign out, optional "hide previews" (which also hides recipient labels and the crisis follow-up banner; offered to guests during onboarding, because a guest cannot sign out), optional neutral tab titles, no notifications |
| Low vision, presbyopia, tired eyes | Text that grows without breaking; meaning that is not in small icons or small print | rem-only type scale, text size up to 150% on top of browser zoom, icons always labelled, 18px reading text, and every label that carries meaning (provenance badges, status chips, meta-line actions, navigation) at 16px even for people who never change a setting; 14px only for timestamps and footnotes |
| A screen-reader user | Calm, complete announcements; never token-by-token chatter; never panels read out in full | The transcript is a silent `role="log"`. Each released reply is announced once as "Sela: …" through its own live region, and every other change gets one short fixed line (§6.5) |
| A cheap Android phone, prepaid data, slow 3G | Fast first screen; nothing heavy to download; drafts never lost; the conversation stays readable while the keyboard is open | System fonts, no raster images, a JavaScript budget, help that renders without the network, drafts kept in memory on errors, and a typing layout that keeps only the help control fixed and caps the composer, so Sela's reply stays readable above the keyboard (§6.5) |
| Visiting a Puskesmas next week | Gather what they said, check it, bring it in the form that works there | Summary built from chosen notes, show-on-screen and print views, sourced destination checklists |
| After a consultation | Note what happened in their own words and carry on | "Setelah konsultasi" follow-up, linked to the same summary |
| 15 to 17 years old | Respect; clear limits of confidentiality; a guardian who consents without getting access | Teen copy and confidentiality screens; memory opt-in only (D-031) |
| Unsure what the AI is | Honest labels and plain explanations | Provenance badges, reply details, consent cards that name provider, place and purpose |
| Tremor or limited dexterity | Large targets; no drag, no hold, no time pressure | 44px targets, no gesture-only actions, no timed steps |
| Code-switching between Indonesian and English | Replies in their language; switching the interface anywhere | Language on the Introduction and in the menu; `lang` set on each part that is mostly in the other language, from a `lang` field on the record that the server fills, including the spoken reply announcement (§6.5) |

### 1.3 Constraints

- **Frontend stack (PM decision D-032):** React 19, TypeScript and Vite with plain CSS and custom properties. I add no UI framework (§9).
- **Content Security Policy:** `default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'` (observed in the `Content-Security-Policy` header set in `internal/httpapi/server.go`). This rules out font CDNs and inline scripts. React `style` props are CSSOM writes, which the policy allows.
- **Accessibility target:** WCAG 2.2 AA (user-confirmed, brief §6). See [ACCESSIBILITY_CRITERIA.md](ACCESSIBILITY_CRITERIA.md).
- **Product rules (PM decisions):**
  - no streaks, scores or guilt prompts (D-037);
  - honest terms (D-039);
  - six provenance classes with time, version and review status (D-040);
  - handoff statuses (D-041);
  - recipients are invited by the user and their identity is not verified (D-042);
  - crisis handling that never erases the user's words (D-036);
  - minors implemented but disabled in production (D-031).

---

## 2. Directions explored

I sketched four directions. Each covers information architecture, interaction model and visual language together, because the brief rejects a cosmetic redesign (§4).

I walked each one through the same seven journeys:

1. First visit to first reply.
2. A heavy evening that triggers crisis support.
3. Correcting what Sela understood.
4. Building and sharing a summary.
5. Revoking access.
6. Writing after a consultation.
7. Deleting everything.

### Direction A: "Ruang teduh" (a calm hub)

- **IA:** Hub and spoke. Home is a grid of eight equal tiles: Ngobrol, Jurnal, Check-in, Linimasa, Memori, Ringkasan, Bacaan and Pengaturan. Each feature is its own page.
- **Interaction:** Every task starts from Home. The consultation summary is a separate wizard. Storage and consent controls live only in Settings.
- **Visual:** Pastel sage, lilac and peach; a rounded web display font; soft illustrations of plants and sky; large rounded cards.
- **Strengths:**
  - every feature is visible;
  - simple to build.
- **Weaknesses:**
  - Eight equal choices land on a tired person.
  - The layout is organised by feature. The main journey (talk → keep → summarise → share) jumps between unrelated pages.
  - Pastels and illustrations strain contrast and data budgets, and a web font adds bytes.
  - The v1 navigation already shows this failure mode. Its header budgets every pixel across four width tiers (1024, 1280, 1536 and 1680px) and hides or shortens labels at each one to fit more than a dozen destinations (`apps/web/src/components/Navbar.tsx`).

### Direction B: "Teman ngobrol" (chat first)

- **IA:** One thread. Home is a list of conversations. Check-in, reflection, summary and sharing appear as cards inside the conversation. Settings sit behind a menu.
- **Interaction:** Everything starts by typing, and Sela proposes cards.
- **Visual:** Messenger bubbles, an avatar for Sela with presence ("online", typing dots), a green close to WhatsApp.
- **Strengths:**
  - Almost nothing to learn: WhatsApp is used across every age group in Indonesia.
  - Listening is literally at the centre.
- **Weaknesses:**
  - The messenger frame says "a person is here". That works against D-035 (no exclusive relationship, no dependency cues) and brief §5.
  - Kept notes, memory and shares have no place of their own, so privacy controls are buried.
  - A long consultation review inside a chat is hard to do and harder to check.
  - Bubbles shorten lines to about 15 characters at 320px with large text, and they break under 1.4.12 text spacing.
  - A `role="log"` full of interactive cards is heavy for screen readers.

### Direction C: "Ruang & Catatan" (a room and a notebook). Chosen.

- **IA:** Four places, each with one job:
  - **Beranda** (Home): a calm start, not a dashboard.
  - **Cerita** (Talk): the listening room.
  - **Catatanku** (My notes): everything kept, on one timeline, plus what Sela remembers.
  - **Dukungan** (Support): preparing a consultation, reading and small exercises, and support services.

  Every screen also has two safety controls ("Butuh bantuan sekarang" and "Keluar cepat") and one personal menu (privacy and data, display and accessibility, account). Details are in [INFORMATION_ARCHITECTURE.md](INFORMATION_ARCHITECTURE.md).
- **Interaction:**
  - The room is a focused mode: navigation steps back and the composer sits at the bottom of the screen.
  - The notebook is calm, structured documents. Every kept item shows its provenance, with its controls next to it.
  - Support offers; it never pushes.
- **Visual: "kertas, tinta, dan celah cahaya"** (paper, ink and a gap of light).
  - Warm off-white paper, near-black ink, and one deep-sea teal for actions.
  - Three meaning colours taken from Indonesian natural dyes:
    - *kunyit* (turmeric) marks the user's own words;
    - *nila* (indigo) marks anything an AI wrote;
    - *mengkudu* (noni-root red) marks help entry points.
  - Typography carries the hierarchy. There are no illustrations of people, no avatars and no bubbles.
- **Strengths:**
  - The user's words are visually first-class.
  - Provenance becomes a visual language, not an afterthought.
  - Privacy controls sit where the data is.
  - Starting in the room is as simple as a chat, while the notebook gives summaries and sharing a proper home.
- **Weaknesses (accepted, with mitigation):**
  - Two visual registers (room and notebook) must stay consistent. Mitigation: one component set (§6).
  - People have to learn four places and a menu. Mitigation: plain labels, and Home offers the three most common starts.
  - "Dukungan" groups three different things. Mitigation: a hub page with three clearly headed sections.

### Direction D: "Pantau dirimu" (a wellbeing tracker). Rejected at sketch stage.

Mood charts, weekly scores and a streak calendar on Home. I rejected it for three reasons:

- It rewards frequency (D-037).
- It turns feelings into a score that reads like an assessment. D-038 dropped severity scoring because it acts as triage, which brief §11 forbids.
- Every chart would need a full text alternative.

A calm, text-first pattern view survives as an opt-in inside Catatanku ([SCREENS.md](SCREENS.md#s12-timeline)).

### Comparison against the PM criteria (brief §4)

Scores run from 1 (poor) to 5 (strong). They come from the walkthroughs above. **This is a design review, not user research:** no participants were involved. Owner testing (P8) and later real-user sessions must check it (pending verification).

| Criterion | A: calm hub | B: chat first | C: room and notebook | Main reason for C's score |
|---|---|---|---|---|
| Clarity | 3 | 3 | 4 | Four named places with one job each. Loses a point because people must learn the places |
| Consistency | 4 | 4 | 4 | One component set across both registers; help and exit are in the same position everywhere |
| Legibility | 3 | 3 | 5 | Full-width text blocks instead of bubbles, an 18px reading size, system fonts, a palette that passes 4.5:1 for every text pair (§5.3) |
| Ease of completing tasks | 3 | 2 | 4 | One tap to talk from Home, and at most four screens from "Mulai" to the composer on a first visit. The summary is built from the timeline. Sharing has its own page with status and revoke. Loses a point because the summary flow has four steps |
| Privacy control | 3 | 2 | 5 | Storage choice in the room; delete, forget and revoke next to each item; consent cards that name provider, place and purpose |
| Accessibility | 3 | 2 | 5 | A screen-reader-safe reply pattern (silent log, one announcement per reply), no gesture-only actions, provenance that never relies on colour, bars that wrap at 320px |
| Implementation quality | 4 | 3 | 4 | Plain React and CSS on top of v2's i18n and op runtime; the router needs parameters and scroll restoration (§8). The room's release-and-announce pattern is the hardest part |
| **Total (of 35)** | **23** | **19** | **31** | |

**Decision (design decision): Direction C.** It is the only direction in which three things all sit on one visible structure:

1. the core function (listening) stays as easy as a chat;
2. the brief's honesty duties (provenance, consent by content, recipient and purpose, revocation) have a natural place;
3. the accessibility target can be met.

The chat-first direction fails on the anti-dependency rules and on privacy control. The hub fails on clarity at the moment of need.

**What C borrows:**

- **From A:** a Home that offers a few clear starts.
- **From B:**
  - the single-column transcript with the composer at the bottom;
  - reflection cards inside the conversation. Only "Sela menangkap…" appears there; nothing else becomes a card.
- **From v1 Dengar.in:**
  - Quick Exit;
  - the display menu (text size, contrast, spacing, motion);
  - the sourced destination checklists;
  - hiding the bottom bar while typing.
- **From v2:**
  - the separation of quote and wording;
  - inline confirmation for destructive actions;
  - the operation and server-sent-events runtime;
  - the warm neutral lineage.

---

## 3. Design principles

Each principle is a design decision. It is also a review question: a screen that fails one of them is not done.

1. **Listening first, structure later.** Starting to talk takes one action. Structure (reflections, summaries) appears only when the person asks, or when they chose an intent that asks for it. *Review question: can someone type their first sentence within one tap of Home?*
2. **The person's words are the source of truth.** The user's original words are shown first, marked by the *kunyit* rule and the label "Kata-katamu". AI text is always labelled, shown below or beside the user's words, and never replaces them silently. *Review question: on this screen, can I tell what I wrote from what an AI wrote, without colour?*
3. **Everything kept says where it came from.** Every stored item shows its provenance class, time, version and review status (D-040). There are six classes, each with a distinct icon, label and border style (§6.7).
4. **Control sits next to the content.** Storage, deletion, forgetting, sharing and revocation are offered where the item is shown, not only in Settings. Settings repeats them as an overview.
5. **Nothing pushes.** No streaks, points, badges, counters of days, guilt prompts, push notifications or countdowns (D-037). The only countdown in the product is the expiry a person chose for a share. Each suggestion is offered once, in words like "kalau kamu mau" ("if you want"), and can be ignored without consequence.
6. **Help is one step away, everywhere, in the same place.** "Butuh bantuan sekarang" and "Keluar cepat" keep the same position and order on every screen and as the first two items of every full-height sheet (WCAG 3.2.6), with full labels that wrap instead of shortening. The help control is calm in the top bar (the Help-quiet form) and loud only where help is the subject (§6.2). Crisis support never waits for a model (D-036).
7. **Calm, not clinical, not cute.**
   - No medical imagery, brains, hearts, robots, sparkles or stock photos of sad people.
   - No cartoon faces or mascots.
   - No "assessment results" or "patient" vocabulary.
8. **One product for every age; preferences, not assumptions.** Nothing changes by adult age. People choose text size, contrast, spacing and motion directly. Teen differences are about consent and confidentiality, not a childish look.
9. **Works on a cheap phone on a slow connection.** System fonts, no raster images, a script budget, help that renders without the network, and drafts that survive errors (§7).
10. **Honest by construction.** Demo, test-double and guest states are always labelled. Every AI-written item names its generator and its checker, **taken from the item's stored provenance, never from configuration**; a test program is never named as Meta, Claude or DeepSeek (§6.7). No live AI provider is called without the person's consent for it. Sela never claims a partnership, booking, verified professional or medical record (D-039, D-042). Nothing is shown as recorded unless it was recorded. Revocation copy always says that copies already seen cannot be recalled.
11. **Nothing reaches the screen before it is checked.** An AI reply is shown only after the server's guards (and the verifier, when the reply is flagged or the backup model DeepSeek wrote it) have released it (ADR-0003 §5, RR-8, §17 O-6). The person never sees text that is later taken back.

---

## 4. Visual language

### 4.1 Colour concept

| Name | Role | Why |
|---|---|---|
| *Kertas* (paper), warm off-white `#FAF7F2` | Page background | Calm, not clinical white, and easy on the eyes in the evening (dark scheme available) |
| *Tinta* (ink), warm near-black `#1E1C19` | Text | 15.9:1 on paper. Body text is far above the minimum because many people will read with tired eyes |
| *Laut* (deep sea), teal `#1D5C57` | Actions, current navigation, selection | Steady and not alarming. Distinct from the three meaning colours |
| *Kunyit* (turmeric), ochre `#9A5A00` | The user's own words (rule and icon) | Warm, "yours". Turmeric is a natural dye in Indonesian textiles |
| *Nila* (indigo), `#5048A8` | Anything an AI wrote (outline and icon) | Cool and distinct from kunyit. Indigo (*nila*, *tarum*) is the classic natural batik dye |
| *Mengkudu* (noni-root red), `#A1351E` | Help entry points only. In the top bar, the help strip and sheets it is the **Help-quiet** form: a pale mengkudu tint, a 2px mengkudu border and a dark mengkudu label, so the first thing on every screen is findable without reading as an alarm. The **solid red fill** is kept where help is the subject: "Lihat semua bantuan" in the crisis panel, the pause line and `/help`. **A red fill always means help**: destructive actions are outlined, never filled red (§6.2). Both forms are compared in owner testing before the pilot (pending verification, §10) | Visible without being an alarm siren. Mengkudu root gives the red of traditional batik |

The dye names are a reference only. The design uses no batik motifs as decoration, to avoid cliché and appropriation of regional patterns.

### 4.2 Typography

- **Fonts:** the system UI stack only: `system-ui, -apple-system, "Segoe UI", Roboto, "Noto Sans", …`. That means Roboto or Noto on Android, SF on iOS and Segoe UI on Windows. All of them cover Indonesian and English and download nothing (CSP `font-src 'self'`, slow connections). The v1 display face (Schibsted Grotesk) is not used: one more file for headings is not worth the bytes.
- **Hierarchy:** size and weight only. Regular 400 for body text, 600 for labels and emphasis, 700 for headings. Italics are not used for meaning.
- **Reading text** (transcript, journal, reading library): 18px (`--font-size-4`), line-height 1.6, measure 38rem (about 65 characters).
- **Times and codes:** `font-variant-numeric: tabular-nums` for meta lines. Access and recovery codes use the mono stack, grouped in blocks of four.
- **Long words and URLs:** `overflow-wrap: anywhere` on user-generated text and URLs. No automatic hyphenation, because browser support for Indonesian hyphenation is uneven and broken words are harder to read.

### 4.3 Shape and space

Shape tells the kind of element:

- **Rectangle with 8px radius:** an action (button) or a field.
- **Pill:** a choice (chip).
- **Small 4px tag:** a status or provenance label, never interactive. Its text is `--font-size-2` (16px), the same as buttons.
- **12px card:** a container.

Spacing follows a 4px grid in rem. Pages have one column on phones. Wide layouts add a sidebar, and the room adds a details panel; content never sits in more than two columns.

### 4.4 Iconography

- **One outline set:** a 24px grid, 1.75px stroke, round joins, drawn in `currentColor`. Sourced from Lucide (ISC licence, already used by v1). The roughly 30 icons Sela needs are copied as inline SVG components into `web/src/components/icons.tsx`, with the ISC notice kept. No icon package is added as a dependency.
- **Always with a visible text label** for navigation, actions, statuses and provenance.
- **Icon-only buttons** are allowed only for dialog Close. That button has an accessible name, "Tutup" / "Close".
- **Decorative icons** carry `aria-hidden="true"`. Meaningful icons sit next to text that already says the same thing, so they also carry `aria-hidden` and are not announced twice.
- **Icon contrast:** at least 3:1 (they are drawn in `--c-text` or a role colour; §5.3).
- **Icons used (indicative):**

  | Meaning | Icon |
  |---|---|
  | Home | house |
  | Talk | message-square |
  | My notes | notebook |
  | Support | hand-heart |
  | Help | life-buoy |
  | Quick exit | log-out |
  | Menu | menu |
  | Send | send |
  | Stop | square |
  | Microphone | mic |
  | Your words | quote |
  | Your correction | pencil |
  | AI wording | sparkle is **not** used; a square "AI" monogram instead |
  | Sela's interpretation | circle-help |
  | Replies paused | pause |
  | Name kept private | eye-off |
  | You marked it (self-reported step) | pencil-line |
  | Upload (reserved) | paperclip |
  | Verified note (reserved) | badge-check |
  | Saved | bookmark |
  | Not saved | bookmark-x |
  | Shared | share |
  | Opened | eye |
  | Acknowledged | check |
  | In consultation | calendar-check |
  | Revoked | ban |
  | Expired | clock |
  | Ended | square-check |
  | Delete | trash |
  | Download | download |
  | Copy | copy |
  | Back | arrow-left |
  | Close | x |
  | Settings | sliders |

### 4.5 Imagery and the mark

- **No photographs and no illustrations of people.** Reasons:
  - avoid stereotyping age, gender, religion or region;
  - avoid showing "sad person" imagery to someone who is struggling;
  - save data.
- **No decorative raster images.** The only images are the mark and the icons, both inline SVG.
- **The mark** is two curved strokes that almost meet, like a pair of parentheses with room between them: a space held open, *sela*. It is drawn in `currentColor`, so it follows every scheme and forced colours. It appears in the top bar, the intro and empty states.
- **Empty states** use text and, at most, the mark. They never use a mascot.

### 4.6 Motion

Motion only explains a change of place or state: a sheet arriving, a disclosure opening, a press. Nothing moves on its own for more than five seconds. There is no parallax, confetti or celebration. The full rules are in §5.6.

---

## 5. Token system

### 5.1 Files and theming

- **Source of truth:** [tokens.css](tokens.css). The frontend copies it to `web/src/styles/tokens.css` (planned). Components use the roles, never raw hex values.
- **Schemes:** four, set by the boot script on `<html data-scheme="…">`:
  - `light` (default);
  - `dark`;
  - `contrast-light`;
  - `contrast-dark`.

  The effective scheme comes from the user's display settings ("Tema: Ikuti sistem / Terang / Gelap" and "Kontras tinggi: Ikuti sistem / Ya / Tidak", both radio groups) combined with `prefers-color-scheme` and `prefers-contrast: more`, and it updates when they change.
- **Boot script:** `web/public/boot.js` (planned) is a same-origin file loaded in `<head>` before the stylesheet. It cannot be inline because the CSP is `script-src 'self'`. It reads display preferences from `localStorage` inside `try/catch` and writes `data-scheme`, `data-text-scale`, `data-spacing` and `data-motion` before first paint. If it does not run, the light scheme applies, or dark through the CSS fallback block.
- **Forced colours** (Windows contrast themes): see §5.10.

### 5.2 Colour roles

Generated from `tokens.css` by `python3 docs/design/tools/contrast.py --roles`.

<!-- BEGIN GENERATED: roles -->
| Role | Purpose | light | dark | contrast-light | contrast-dark |
|---|---|---|---|---|---|
| `--c-bg` | Page background (kertas, paper) | #FAF7F2 | #141311 | #FFFFFF | #000000 |
| `--c-surface` | Cards, dialogs, inputs, sheets | #FFFFFF | #1D1B18 | #FFFFFF | #000000 |
| `--c-sunken` | Composer area, inset panels, hover fill | #F1ECE4 | #25221E | #FFFFFF | #000000 |
| `--c-text` | Body text, headings, meaningful icons (tinta, ink) | #1E1C19 | #F2EEE7 | #000000 | #FFFFFF |
| `--c-muted` | Secondary text: hints, timestamps, meta lines | #575249 | #BCB4A7 | #1F1F1F | #E8E8E8 |
| `--c-border-subtle` | Decorative separators only (no contrast requirement, never a control edge) | #E3DCD0 | #35312B | #000000 | #FFFFFF |
| `--c-border-control` | Edges of inputs, chips, checkboxes, radios | #7B7368 | #8E8679 | #000000 | #FFFFFF |
| `--c-accent` | Primary action fill, checked controls, selected chips, current navigation (laut, deep sea) | #1D5C57 | #7FC6BC | #00403B | #A6F2E8 |
| `--c-accent-hover` | Primary action hover and pressed | #154743 | #9BD5CD | #002B28 | #C8F8F1 |
| `--c-on-accent` | Text and icons on accent | #FFFFFF | #0C1D1B | #FFFFFF | #000000 |
| `--c-accent-soft` | Selected chip fill | #DDEBE8 | #1F3532 | #FFFFFF | #000000 |
| `--c-link` | Inline links (always underlined) | #1B5A55 | #8ED2C8 | #003A99 | #A6F2E8 |
| `--c-focus` | Focus ring | #2747C9 | #94B2FF | #0033CC | #FFE14D |
| `--c-user` | Rule and icon marking the user's own words (kunyit, turmeric) | #9A5A00 | #E2A24C | #6B3500 | #FFB85C |
| `--c-user-bg` | Tint behind quoted user words in notes and summaries | #FBF0DF | #2C2317 | #FFFFFF | #000000 |
| `--c-user-text` | 'Kata-katamu' / 'Koreksimu' label text | #6E4100 | #F0C27E | #4D2600 | #FFC97F |
| `--c-ai` | Outline and icon marking AI-written text (nila, indigo) | #5048A8 | #ABA6F2 | #2A2283 | #CFCBFF |
| `--c-ai-bg` | Tint behind AI wording and 'Sela menangkap…' | #EFEEF9 | #232139 | #FFFFFF | #000000 |
| `--c-ai-text` | 'Tulisan AI' / 'Tafsiran Sela' label text | #3E378F | #C4C0FA | #2A2283 | #CFCBFF |
| `--c-help` | Help-variant fill for 'Lihat semua bantuan' in the crisis panel, the pause line and /help (mengkudu, noni red); the 2px border of Help-quiet. Red fill means help only | #A1351E | #F2917A | #7E1A07 | #FFA28D |
| `--c-help-hover` | Help-variant hover and pressed fill; Help-quiet hover border | #852A16 | #F7AE9C | #5E1205 | #FFC2B3 |
| `--c-on-help` | Text on help | #FFFFFF | #2B0E07 | #FFFFFF | #000000 |
| `--c-help-bg` | Crisis support panel tint; Help-quiet fill ('Butuh bantuan sekarang' in the top bar) | #FBE9E4 | #36211B | #FFFFFF | #000000 |
| `--c-help-text` | Help headings, help links and the Help-quiet label | #8C2C17 | #F6AE9C | #7E1A07 | #FFB4A3 |
| `--c-danger` | Destructive actions (outlined only, never filled), field errors | #A3271F | #F38E85 | #8B0F08 | #FF9C94 |
| `--c-danger-bg` | Error notice tint | #FCE9E7 | #3A1C19 | #FFFFFF | #000000 |
| `--c-success` | Success text and chip border | #23683C | #83CC9B | #0B4A22 | #8FE3AA |
| `--c-success-bg` | Success notice tint | #E6F2E9 | #18301F | #FFFFFF | #000000 |
| `--c-warning` | Warning text | #875200 | #F1C46F | #5C3700 | #FFD27A |
| `--c-warning-bg` | Warning notice tint | #FCF1D9 | #352A12 | #FFFFFF | #000000 |
| `--c-warning-border` | Warning notice and chip border | #A86E00 | #D9A440 | #5C3700 | #FFD27A |
| `--c-info` | Info text and chip border | #1F5788 | #90C3F0 | #0B3A66 | #A3D2FF |
| `--c-info-bg` | Info notice and guest banner tint | #E7F0F8 | #172A3B | #FFFFFF | #000000 |
| `--c-demo` | Demo banner label and border | #5E4A99 | #C6B6F2 | #3B2475 | #D9CCFF |
| `--c-demo-bg` | Demo banner tint | #EEE9F8 | #2A2340 | #FFFFFF | #000000 |
| `--c-test` | Test-double banner label and border | #7A5600 | #F2C94C | #4F3800 | #FFDA66 |
| `--c-test-bg` | Test-double banner tint | #FFF1C7 | #352B0E | #FFFFFF | #000000 |
<!-- END GENERATED: roles -->

`--c-overlay` (the dialog backdrop) and the shadows are not contrast-bearing. Nothing must be read against them.

### 5.3 Contrast results

The checker computes every text/background and UI-component pair the components use (`python3 docs/design/tools/contrast.py`, works now). It uses the WCAG 2.2 relative-luminance formula (externally sourced: https://www.w3.org/TR/WCAG22/#dfn-relative-luminance).

| Scheme | Text pairs | Lowest text ratio (needs 4.5:1) | UI pairs | Lowest UI ratio (needs 3:1) |
|---|---|---|---|---|
| light | 67 | 5.78:1 (warning heading on warning tint) | 61 | 3.82:1 (warning border on its own tint) |
| dark | 67 | 6.63:1 | 61 | 3.88:1 (control border on the test-double tint) |
| contrast-light | 67 | 9.68:1 | 61 | 8.95:1 |
| contrast-dark | 67 | 10.44:1 | 61 | 10.44:1 |

That is 512 computed pairs. Round 1 added every pair in which a control, link or focus ring sits on a tint: reflection-card buttons on `--c-ai-bg`, crisis-panel links and buttons on `--c-help-bg`, quote source links on `--c-user-bg`, and links and buttons inside every notice and banner. All of them already passed (the lowest in light are the control border on the demo tint at 3.93:1, on the help tint at 3.98:1, and the focus ring on the demo tint at 6.23:1), so no colour changed, but CI now catches a regression. Round 2 added the three Help-quiet pairs (its label at 7.19:1 and its icon on `--c-help-bg`, and its hover border `--c-help-hover` against its own fill at 7.64:1 in light); its resting border is the existing `--c-help` on `--c-help-bg` and `--c-bg` pairs. No colour changed.

Design rules behind the numbers (design decisions):

- **All text must reach 4.5:1, including large text.** WCAG allows 3:1 for large text; Sela does not use that exception.
- **Red fill means help.** The Help variant is the only red-filled control; the Help-quiet form in the top bar is a tint with a red border. Destructive actions are outlined in `--c-danger` on a `--c-surface` fill with the trash or ban icon, never on a tint, so the two outlined forms differ in fill, icon and label as well as place; the confirm step of a destructive action uses an ink fill (`--c-text` with a `--c-surface` label, `--c-muted` when pressed) plus the trash or ban icon and a verb. A filled danger button would be indistinguishable from help (help against danger is 1.06:1 in light and 1.01:1 in dark), and white on `--c-danger` fails in dark (2.33:1).
- **No low-contrast disabled look.** Unavailable actions use `aria-disabled="true"` with a dashed border, keep text at 4.5:1 or better, and give the reason in text. WCAG exempts inactive controls, but people with low vision still need to read why something is unavailable.
- **Focus ring adjacency.** The ring sits 2px outside the component (`outline-offset: 2px`), so it is adjacent to whatever surrounds the component: the page, a card, the composer area, or a tinted block (AI wording, the crisis panel, a quote, a notice or a banner). The checker covers every one of these surfaces, not button fills. Scroll containers keep at least 6px of inner padding so the offset ring is never clipped. The fixed help strip (§5.8) is such a surface too: it has its own `--c-bg` background and `--strip-pad` (6px) around the control, so the ring is drawn on the strip. **The ring is never drawn inset on a red fill** (it would be 1.08:1 against `--c-help` in light).
- **Placeholders are not labels.** Placeholder text, where used at all, is `--c-muted` and passes 4.5:1.
- **Shadows and backdrops carry no text.**

The full tables are in [Appendix A](#appendix-a-contrast-tables).

### 5.4 Type scale, zoom and text spacing

| Token | Size at 100% | At Sela "Paling besar" (150%) | Used for |
|---|---|---|---|
| `--font-size-1` | 14px | 21px | Timestamps and footnotes only. Nothing is smaller |
| `--font-size-2` | 16px | 24px | Buttons, chips, labels, provenance badges, status chips, meta-line actions ("Tentang balasan ini", "Kurang tepat?", "Bacakan"), navigation labels |
| `--font-size-3` | 17px | 25.5px | Body text |
| `--font-size-4` | 18px | 27px | Transcript, journal, reading |
| `--font-size-5` | 21px | 31.5px | h3, card titles |
| `--font-size-6` | 24px | 36px | h2 |
| `--font-size-7` | 28px | 42px | h1 (compact layouts) |
| `--font-size-8` | 34px | 51px | h1 (wide layouts), intro title |

Rules that make the scale survive zoom, text resizing and text spacing (design decisions, tested by ACCESSIBILITY_CRITERIA 1.4.4, 1.4.10 and 1.4.12):

1. **rem only for type.** Never `vw` or `clamp()` with `vw` in font sizes: a viewport-based term does not grow with text-only zoom, so 200% text zoom would yield less than 200%.
2. **No fixed heights on anything that contains text.**
   - Use `min-height` and padding.
   - Never put `overflow: hidden` on text containers.
   - Chips, buttons and status chips wrap their label onto two lines rather than truncating.
   - Ellipsis truncation is allowed only for list previews that link to the full text, and never under "Jarak teks lebih lega" (relaxed spacing).
3. **Unitless line heights** (1.6 body, 1.3 headings, 1.4 UI). Then the 1.4.12 user values (line-height 1.5, paragraph spacing 2em, letter spacing 0.12em, word spacing 0.16em) never clip text.
4. **Layout breakpoints are container queries in em on the app shell** (§5.8). Because the shell's font size follows the user's text-size setting, the layout steps down (sidebar to bottom bar, two columns to one) as text grows. Browser zoom does the same through the viewport.
5. **Wrap by available space, never by text-size setting.** Every row of controls (top bar, bottom bar, room toolbar, composer row, the Home action pair, dialog buttons) is `flex-wrap: wrap` with flex bases in em, so it wraps wherever the space runs out: at 320px, at 150% text, or under a user's 1.4.12 spacing stylesheet. No layout rule reads `data-text-scale`. Labels never truncate; multi-word labels wrap inside their control, and single-word labels are kept short enough to fit their slot (estimates in §6.1).
6. **Supported combinations:**
   - browser zoom to 400% at 1280px, which equals 320 CSS px;
   - Sela text size 150% combined with browser zoom 200%;
   - the 1.4.12 spacing stylesheet at every text size.

   None of these may cause horizontal scrolling, overlap, clipping, or a control that cannot be reached.

   This covers v1 Dengar.in's largest text setting (140%) with room to spare, and the 1.4.12 spacing values on top of it.
7. **Headings:** `text-wrap: balance` where supported. Paragraphs: `text-wrap: pretty`. Both degrade safely.

### 5.5 Space, shape, size and elevation

The values are in [tokens.css](tokens.css). The rules:

- **Space:** `--space-1` to `--space-8` on a 4px grid. The side gutter is `--gutter` (16px) at every width below 40em. The vertical rhythm between blocks is `--stack-gap`.
- **Targets:** every pointer target is at least `--target-min` (44px) high and wide, with at least `--target-gap` (8px) between neighbours. Inline links in running text are exempt (WCAG 2.5.8 exception) but get extra line-height. This is stricter than the 24px minimum in 2.5.8, because of tremor, older thumbs and small phones. Bottom-bar slots may touch each other, because each slot is at least about 80×56px. The target of a native checkbox or radio is its whole label row, and the target of a chip is the chip.
- **Borders:**
  - `--border-width` for control edges (2px in the high-contrast schemes);
  - `--border-width-strong` for selected and invalid states;
  - `--rule-width` (4px) for the "your words" rule.
- **Elevation:**
  - `--shadow-1` for the sticky bars;
  - `--shadow-2` for dialogs and sheets.

  High-contrast schemes have no shadows: borders do the job.

### 5.6 Motion and reduced motion

| Token | Default | Reduced |
|---|---|---|
| `--duration-quick` | 120ms (hover, press, colour) | 0.01ms |
| `--duration-gentle` | 200ms (sheet and dialog entry, disclosure) | 0.01ms |
| `--reveal-step` | 180ms between sentences when a released reply is revealed | 0ms (no reveal) |
| `--ease-standard` | `cubic-bezier(0.2, 0, 0, 1)` | — |

Reduced durations are 0.01ms, not 0ms: a transition whose combined duration is zero never starts and fires no `transitionend`. Components still **never wait for `transitionend` or `animationend`** to finish a state change (removing a sheet, finishing a disclosure, revealing a reply). They set the end state directly and let CSS animate towards it, so nothing can hang for reduced-motion users.

Behaviour (design decisions):

- **Who decides.** "Kurangi gerakan" (Reduce motion) has three options:
  - Ikuti sistem (follow the system), the default;
  - Ya (yes): always reduced;
  - Tidak (no): full motion even if the system asks for less.

  The token block implements all three.
- **Under reduced motion:**
  - Sheets and dialogs appear without sliding.
  - Disclosure opens without animation.
  - Smooth scrolling is off.
  - A released reply appears whole, with no sentence reveal (§6.5).
- **Always, whatever the setting:**
  - Nothing flashes more than three times per second (2.3.1).
  - Nothing animates on its own for more than five seconds (2.2.2). There are **no animated indicators at all**: "Sela sedang menulis…", the cold-start line and every loading line are static text that changes at most once per second. The longest motion is the reply reveal, capped at about 1.5 seconds.
  - No autoplaying audio or video.
  - No looping illustrations.
  - No celebrations on saving or checking in.

### 5.7 Focus indicator

```css
:where(a, button, input, select, textarea, summary, [tabindex]):focus-visible {
  outline: var(--focus-width) solid var(--c-focus);
  outline-offset: var(--focus-offset);
}
/* Text fields show the ring on any focus. The edge looks 2px thick through an
   inset shadow, so the border width (and the layout) never changes. */
:where(input, select, textarea):focus {
  outline: var(--focus-width) solid var(--c-focus);
  outline-offset: var(--focus-offset);
  box-shadow: inset 0 0 0 1px var(--c-border-control);
}
/* Visually hidden native inputs behind chips: the label carries the ring. */
.chip-input:focus-visible + .chip { outline: var(--focus-width) solid var(--c-focus); outline-offset: var(--focus-offset); }
/* Only <main>, the route-change focus target, never shows a ring: a ring
   around the whole page is noise. <main> is named by the page h1
   (aria-labelledby), so moving focus to it announces the page name.
   Every other target that receives focus by script uses :focus, not
   :focus-visible: browsers do not match :focus-visible when a script moves
   focus after a pointer or touch activation, so a magnifier or touch user
   who taps "Ke balasan terakhir", a new-content button or "Cabut akses"
   would otherwise see no indicator. Articles also get a 4px leading bar. */
main:focus { outline: none; }
[tabindex="-1"]:not(main):focus {   /* articles, headings, status lines, error summaries, confirm sentences */
  outline: var(--focus-width) solid var(--c-focus);
  outline-offset: var(--focus-offset);
}
article[tabindex="-1"]:focus { box-shadow: inset var(--rule-width) 0 0 var(--c-focus); }
/* The fixed help strip (§5.8): the ring is drawn on the strip's own --c-bg,
   never inset on the control's fill. */
.help-strip { background: var(--c-bg); padding: var(--strip-pad) max(var(--gutter), env(safe-area-inset-left)); }
/* Sticky layers. --bottom-bar-h is 0px whenever the bar is hidden (the room,
   text entry, the Menu-only tier); --composer-h is 0px outside the room. The
   two are never sticky together, and the sum stays safe if that changes. */
html {
  scroll-padding-top: calc(var(--top-bar-h, 0px) + var(--space-3));
  scroll-padding-bottom: calc(var(--bottom-bar-h, 0px) + var(--composer-h, 0px) + var(--space-3));
}
/* Wide room: the transcript is its own scroll container beside the composer. */
.transcript--own-scroll { scroll-padding-block: var(--space-3); }
```

- **2.4.7 Focus Visible:** every focusable element has the ring, in all four schemes and in forced colours (where `outline-color` is replaced by a system colour).
- **2.4.13 Focus Appearance** (AAA, used as guidance; externally sourced: https://www.w3.org/TR/WCAG22/#focus-appearance):
  - The ring is a 3px solid outline around the whole perimeter. That is more area than a 2px perimeter of the unfocused component.
  - It changes the pixels it covers by at least 3:1, because the ring is 6.23:1 or more against every surface it can sit on, tints included (§5.3).
- **2.4.11 Focus Not Obscured (Minimum):**
  - The sticky top bar (or, in short viewports and while typing in the room on a phone, the fixed help strip), the bottom bar and the composer report `--top-bar-h`, `--bottom-bar-h` and `--composer-h` through a `ResizeObserver` (planned), always from their **measured** height, because every bar grows with text and wraps (§6.1). A hidden bar reports `0px` explicitly.
  - `scroll-padding` keeps the focused element out from under them. In the room on compact layouts the composer is the bottom layer (its label, a text area of up to 40% of the visual viewport, or 25% while typing, and its button row; capped at 50% of the visual viewport while typing). The pause line, the room's end bar and per-message notices are in the page flow, never in the sticky layer, so the sticky layer can never grow taller than the screen. Tabbing forward to a meta-line action, a reflection action or a crisis-panel link scrolls it above the composer, not under it.
  - While typing on a phone, only the help control stays fixed at the top, and if the help strip and the sticky composer would leave less than three transcript lines, the composer joins the page flow until the field loses focus (§6.5, "Typing on a phone"). The layers are capped by the **visual** viewport (`--vvh`), not `vh`, because iOS Safari does not shrink the layout viewport for the keyboard.
  - On wide layouts the transcript is its own scroll container with its own `scroll-padding-block`.
  - Banners are in the page flow, never fixed. Dialogs are modal, so nothing behind them takes focus.
  - Safari's use of `scroll-padding` during focus navigation must be checked (S-5 runs in Chrome and Safari). If it is not honoured, the room calls `scrollIntoView({block: "nearest"})` with a margin on `focusin` inside the transcript.
- **Programmatic focus targets:** `main` (after a route change) shows no ring and is named by the page h1 through `aria-labelledby`, so NVDA, JAWS and VoiceOver announce the page name rather than only "main landmark". Headings, articles, status lines and error summaries that receive focus by script show the ring on any focus (`:focus`), whether the move followed a key press, a click or a tap. Screen readers announce all of them.

### 5.8 Layout, breakpoints and short viewports

- **Mobile first.** The default layout is the compact one, so browsers without container queries get a working phone layout.
- **Container queries** on `.app-shell` (`container-type: inline-size`), in em, so they follow both zoom and Sela's text size:

  | Size | Width of the shell | Navigation | Content |
  |---|---|---|---|
  | compact | below 40em (640px at 100%) | Top bar plus bottom bar (in one of three tiers, §6.1); menu sheet | One column, 16px gutters |
  | medium | 40em to 63.99em | Top bar plus bottom bar; menu sheet | One column, max `--measure-wide` |
  | wide | 64em and over | Left sidebar (16rem) plus a top strip | Main column; the room adds a details panel at 75em and over |

- **Bars wrap by available space** (§5.4 rule 5): the top bar, the bottom bar and the room toolbar grow to more rows, or move to a lighter tier, instead of clipping. The rules and the 320px estimates are in §6.1.
- **Short viewports.** A viewport less than **30 root em** tall (480px at 100% text, 720px at 150%). This happens on landscape phones, at 200% to 400% zoom on a laptop (1280×720 at 400% is 320×180 CSS px; 1280×1024 at 400% is 320×256), and on small screens with large text.
  - **How it is decided:** a script (part of the shell, planned) divides the layout-viewport height by the root font size in px and sets `data-viewport="short"` on `<html>`. Without the script, `@media (max-height: 30em)` applies, which follows browser zoom but not Sela's text size.
  - **The soft keyboard never switches the mode.** With `interactive-widget=resizes-content`, opening the keyboard shrinks the layout viewport below 30em on most phones. If that switched modes on every keystroke-driven resize, layers would jump while the person types (SD-7). So the script measures only while no text-entry field has focus, keeps the last value while one does, and re-measures on `orientationchange` and on resizes that happen without a focused text field. What the room does while the keyboard is open is decided separately, once per focus, by the **typing layout** (§6.5): it is triggered by focus in the room, not by the viewport height, and it follows `visualViewport` (`--vvh`), so Kirim stays above the keyboard on Android and iOS alike.
  - **What changes:** the bottom bar and the composer become part of the page flow (not sticky). The header's help control moves into the **help strip**, which is `position: fixed` at the top.
  - **The help strip** (also used while typing in the room, §6.5): a full-width band with its own `--c-bg` background. The control (the Help-quiet form, §6.2) sits inside it, inset by the gutter on both sides and by `--strip-pad` (6px) above and below, so its offset focus ring is drawn on the strip: never clipped by the viewport edge and never inset on the control's fill. The control is `min-height: 44px` and **grows with its text** (two lines when "Butuh bantuan sekarang" wraps). "Keluar cepat" and Menu join it in the strip only when all three fit in one row (measured); otherwise they stay in the header in the page flow, directly below the strip, where they are also the next items in the DOM. Quick exit also always works with Esc twice.
  - This is the one deliberate fixed element. D-036 requires an always-visible help control. `--top-bar-h` comes from the strip's measured height, and `scroll-padding-top` keeps focus visible below it (2.4.11).
  - **Estimated strip heights:** at 100% text, one line about 56px (44 + 2 × 6); with the label on two lines about 73px. At 150% text the label wraps on every phone width, about 95px.

    | Viewport | 100% text | 150% text |
    |---|---|---|
    | 320×256 (1280×1024 at 400%) | 56px, 22% · about 200px left | 95px, 37% · in the page flow (below) |
    | 320×180 (1280×720 at 400%) | 56px, 31% · about 124px left | 95px, 53% · in the page flow (below) |

  - **When the strip would take more than one third of the viewport height** (design decision): it is not fixed. The header stays in the page flow, with "Butuh bantuan sekarang" as the first focusable element after the skip link, at the top of every page, and the help link inside every crisis panel, pause line and dialog that could be reached in distress is unchanged. A fixed band covering half the screen would leave too little room to read anything, the crisis panel included. **Exception:** while a text field in the room has focus on a phone, the strip stays fixed whatever its share (§6.5), because that is where a person in distress is writing. This reading of D-036's "always visible" at extreme zoom is pending PM confirmation ([§10](#10-open-points-for-the-pm-and-other-owners)). S-1 and S-5 check 320×256 and 320×180 at 100% and 150% text.
- **Safe areas:** `padding-inline: max(var(--gutter), env(safe-area-inset-left))` and the same for the right side; the bottom bar adds `env(safe-area-inset-bottom)`.
- **Soft keyboard:** `<meta name="viewport" content="width=device-width, initial-scale=1, interactive-widget=resizes-content">`. Zoom is never disabled. When a **text-entry** field has focus on compact and medium layouts, the bottom bar hides, as in v1, so the composer is not squeezed between the keyboard and the bar; in the room the same selector also switches the header to the help strip (§6.5). The selector is limited to fields that open the keyboard, so focusing a chip, a check-in tile or a radio never hides the navigation:

  ```css
  body:has(:is(textarea, [contenteditable="true"],
      input:not([type="checkbox"], [type="radio"], [type="button"], [type="submit"],
                 [type="reset"], [type="range"], [type="color"], [type="file"], [type="image"])):focus)
    .bottom-bar { display: none; }
  /* The room's typing layout (§6.5) starts on the same condition (shell below
     64em). The shell script then sets data-typing="sticky" or data-typing="flow"
     on .room after measuring the visual viewport, and removes it on blur. */
  ```

### 5.9 Display preferences

Set in "Tampilan & aksesibilitas" (Display & accessibility; [SCREENS.md](SCREENS.md#s23-display-and-accessibility-settings)). Stored on this device in `localStorage`, behind `try/catch`. If storage is blocked, the choice lasts until the tab closes. Synchronising them to the account is planned and needs an API.

Control types follow §6.3: a three-way choice is a radio group; a two-state preference that applies at once and sends nothing anywhere is a switch; anything that sends data somewhere (dictation) is turned on with an explicit button after its disclosure.

| Preference | Values | Control | Mechanism |
|---|---|---|---|
| Ukuran teks (text size) | 100 / 115 / 130 / 150% | Radio group with an "Aa" sample | `data-text-scale` changes the root font size |
| Tema (theme) | Ikuti sistem / Terang / Gelap | Radio group | `data-scheme` |
| Kontras tinggi (high contrast) | Ikuti sistem / Ya / Tidak | Radio group | `data-scheme` set to `contrast-light` or `contrast-dark` |
| Jarak teks lebih lega (relaxed spacing) | Aktif / Mati | Switch | `data-spacing="relaxed"` |
| Kurangi gerakan (reduce motion) | Ikuti sistem / Ya / Tidak | Radio group | `data-motion` |
| Tampilkan balasan sekaligus (show replies at once) | Aktif / Mati (default: follows reduced motion) | Switch | No sentence reveal (§6.5) |
| Enter untuk mengirim (Enter sends) | Aktif / Mati (default Mati: Enter adds a new line) | Switch | Composer behaviour |
| Umumkan balasan ke pembaca layar (announce replies to screen readers) | Aktif (default) / Mati | Switch, in the **Pembaca layar** group of S23, apart from Suara. Hint: "Untuk pembaca layar. Ini tidak memutar suara; untuk suara, pakai Bacakan dengan suara." | Whether a released reply is sent to the reply live region (§6.5). The transcript itself is always `aria-live="off"`. Earlier drafts called this "Bacakan balasan otomatis"; that name is dropped because "Bacakan" means speech everywhere else |
| Bacakan dengan suara (read aloud) | Off by default | Switch in the **Suara** group (local voices only, nothing leaves the device) | Browser speech synthesis (§6.5) |
| Dikte suara (voice dictation) | Off by default | **Button** "Nyalakan dikte" / "Turn on dictation" after the disclosure; "Matikan dikte" to turn it off | Browser speech recognition (§6.5) |
| Pintasan keluar cepat (Quick exit shortcut, Esc twice) | Aktif (default) / Mati | Switch | Global key handler (§6.12) |
| Keluar cepat juga mengeluarkan akun (Quick exit also signs out; accounts only) | Ikuti "Ingat perangkat ini" (default: signs out sessions that were not remembered) / Selalu / Tidak pernah | Radio group | `POST /api/v3/auth/logout` with `keepalive` before leaving (§6.12) |
| Tujuan keluar cepat (Quick exit destination) | A short list (planned) | Radio group | §6.12 |
| Sembunyikan pratinjau isi (hide content previews) | Aktif / Mati (default Mati) | Switch. Guests are also offered it during onboarding, as an optional equal-weight choice (S02 step 2) | Lists show only type and time; summary titles, recipient labels and the crisis follow-up banner are hidden too |
| Judul tab netral (neutral tab titles) | Aktif / Mati (default Mati); planned for P7 | Switch | `document.title` uses neutral names (see the 2.4.2 note in ACCESSIBILITY_CRITERIA) |

Memory and storage defaults for new conversations are privacy choices, so they live in Privasi & data (S22), not here.

### 5.10 Forced colours

Rules under `@media (forced-colors: active)`:

- Every control keeps a real border (transparent borders become visible).
- Icons use `currentColor`.
- Selected chips and the current navigation item add `forced-color-adjust: none` only on the check icon and use `Highlight`/`HighlightText`. The text label still says "dipilih" ("selected") to assistive technology.
- Provenance badges rely on their icon, label and border style (solid, dashed or dotted), which survive forced colours.
- The focus outline maps to the system colour automatically.

---

## 6. Component inventory

Format for each component: **Use · Anatomy · Variants · States · Semantics and keyboard · Rules**. All copy is ID/EN through i18n keys. Examples here are Indonesian with English after a slash.

### 6.1 App shell

Width estimates in this section use Liberation Sans Bold at 16px as a proxy for the system semibold, 20px icons, 12px inline padding per control, 8px gaps and 288px of content width at 320 CSS px (16px gutters). They are estimates; checks S-1, S-3 and S-5 confirm them in real browsers.

| Label (16px, bold proxy) | 100% | 150% text | 100% with 1.4.12 spacing |
|---|---|---|---|
| Butuh bantuan sekarang | 188px | 281px | 235px |
| Keluar cepat | 96px | 144px | 122px |
| Menu | 42px | 63px | 49px |
| Catatanku / Dukungan (widest place labels) | 77 / 78px | 116 / 117px | 95 / 94px |
| Akhiri percakapan | 139px | 208px | 174px |
| Ke balasan terakhir | 148px | 222px | 187px |

**Top bar**
- *Use:* the same on every owner and public screen.
- *Anatomy, in DOM order:*
  1. skip links;
  2. the mark with "Sela", a link to Home (or to the Introduction when signed out). It is shown from the medium layout up; on compact layouts Home is in the bottom bar (or the Menu), and the Introduction shows the mark above its h1;
  3. "Butuh bantuan sekarang" / "Need help now": a **link** to `/help` styled with the **Help-quiet** variant (§6.2);
  4. "Keluar cepat" / "Quick exit": a button;
  5. the Menu button (compact and medium only).
- *Labels are never shortened.* "Butuh bantuan sekarang" and "Keluar cepat" keep their full text at every width and wrap inside their control when space runs out. The accessible name is the visible label (2.5.3). "Keluar" alone is never used for Quick exit, because "Keluar dari akun" / "Sign out" is a different action (3.2.4).
- *Wrapping (by available space; §5.4 rule 5):* the bar is `display: flex; flex-wrap: wrap`, every item `flex: 1 1 auto`, rows grow with their content (`min-height: var(--bar-min-height)`, never a fixed height). DOM order is the visual order, row by row.
  - One row needs about 498px of content (240 + 148 + 94 plus gaps), so it appears from about 530px of viewport at 100% text, for example on tablets and landscape phones.
  - **At 320–480px (all phones in portrait): two rows.** Row 1 holds "Butuh bantuan sekarang" across the full width (about 240px of 288). Row 2 holds "Keluar cepat" and Menu (about 250px). Height about 104px.
  - At 150% text on a 320px screen: three rows (help, with its label on two lines; Quick exit; Menu).
  - With the 1.4.12 spacing stylesheet at 100%: still two rows (help about 287px; Quick exit and Menu about 283px).
- *Wide layouts:* the header is a **full-width strip across the window**, above both the sidebar and the main column: the mark, "Butuh bantuan sekarang" and "Keluar cepat" start at the left edge in that order. The sidebar and the main column sit below it, and the notices region is at the top of the main column. Visual order therefore equals DOM order (header, navigation, notices, main), so focus never jumps from the top right to the left sidebar and back (2.4.3, 3.2.6).
- *Sticky:* only in tall viewports. In short viewports, and in the room while a text field has focus on compact and medium layouts, the help control alone is fixed, in the help strip (§5.8, §6.5). Outside the room's typing layout, a strip that would take more than a third of the viewport height stays in the page flow instead (§5.8).
- *Semantics:* `<header>` (banner). The relative order of help, Quick exit and Menu never changes (3.2.3, 3.2.6).

**Navigation (bottom bar on compact and medium, sidebar on wide)**
- *Use:* the four places.
- *Anatomy:* `<nav aria-label="Navigasi utama">` holding a `<ul>` of four links. Labels are `--font-size-2` (16px).
- *Current place:* `aria-current="page"`, a 3px accent bar on the top edge (bottom bar) or the leading edge (sidebar), a 600-weight label and the filled icon variant. That is three cues besides colour.
- *Bottom bar tiers.* The four labels are single words, which cannot wrap, so the bar changes tier by available space instead:

  | Tier | Layout | When (container query on the nav, in em of its 16px labels) | Estimated result |
  |---|---|---|---|
  | A | Four across, icon above label, one row | Nav at least 21.5em wide (each slot at least the widest label plus 8px) | 360px phones and wider at 100% text |
  | B | 2×2 grid, icon beside label, two rows | 15.5em to 21.5em | 320px phones at 100–115% text; 360px phones at 115–130% |
  | C | No bottom bar. The four places are the first group in the Menu sheet, in the same order | Below 15.5em | 150% text on any phone; 130% text on a 320px phone |

  A script also checks the bar after layout. If any label is wider than its slot (as happens under a user's 1.4.12 spacing stylesheet, which no query can see), the bar drops one tier. Without the script, labels get `overflow-wrap: anywhere` as a last resort, so nothing ever overflows. In tier C, and whenever the bar is hidden, `--bottom-bar-h` is `0px`.
- *Sidebar extras:* below the four places, a "Pengaturan" (Settings) group: Privasi & data, Tampilan & aksesibilitas, Akun. Below that, the language switch and links to "Tentang Sela" and "Kebijakan". The mark is not repeated in the sidebar; it is in the header strip.
- *DOM order:* the nav comes after the header and before `<main>` on every layout, so screen-reader order is the same on every device. Skip links bypass it.

**Menu sheet**
- *Use:* compact and medium layouts.
- *Anatomy:* a modal `<dialog>` (full height on compact layouts) labelled by its heading "Menu". `showModal()` makes the header behind it inert, so **the sheet repeats the two safety controls as its first items**, with the same labels, look and order as the header (3.2.4, 3.2.6):
  1. "Butuh bantuan sekarang" (Help-quiet link to `/help`);
  2. "Keluar cepat" (works with one activation, exactly as in the header);
  3. "Tutup" / "Close", in the place the Menu button has in the header;
  4. the heading "Menu" (focus moves here when the sheet opens; Shift+Tab reaches help and Quick exit at once);
  5. the four places;
  6. Pengaturan: Privasi & data, Tampilan & aksesibilitas, Akun;
  7. Bahasa (language switch);
  8. Tentang Sela, Kebijakan;
  9. for accounts: "Keluar dari akun" / "Sign out" (never shortened).

  Every full-height sheet (the Menu and the room's Detail sheet) follows this pattern. Small dialogs keep a help link in their text instead (§6.12).
- *Keyboard:* Esc closes the sheet and focus returns to the Menu button. That Esc press never counts towards Quick exit (§6.12); "Keluar cepat" inside the sheet is the one-action exit.

**Skip links**
- "Lewati ke konten" / "Skip to content" is always the first focusable element.
- In the room, a second link follows it: "Lewati ke kotak tulis" / "Skip to the writing box".

**Announcement regions**
- Three visually hidden live regions belong to the shell. They are rendered **empty with the first paint**, as the **last children of `<body>`** (after the footer, outside `<main>`, so they never sit in the reading order between the header and the content), and are never removed or re-created on route changes (a region created together with its text is often not announced):
  - `#sr-status`: `role="status"` (polite, atomic). Short, fixed status lines.
  - `#sr-reply`: `aria-live="polite" aria-atomic="true"`. Only the text of a released Sela reply, as "Sela: …", and only when "Umumkan balasan ke pembaca layar" is on (§6.5). The reply part carries its own `lang` (below).
  - `#sr-alert`: `role="alert"`. Only the crisis line and errors that block the task.
- **Setting text.** `announce(region, segments)` takes one or more segments, each with an optional `lang`. It clears the region, then sets the new content on the next animation frame (or after about 100 ms when frames are throttled), so a repeated sentence is a real DOM change and is announced again. A segment whose `lang` differs from the interface is rendered as `<span lang="…">`: "Sela: <span lang="en">⟨reply⟩</span>", with the "Sela:" prefix and any suffix ("Bantuan tersedia di bawah balasan ini.") in the interface language.
- **Clearing.** All three regions are cleared on every route change and on the person's next input after an announcement (the next `input`, `pointerdown` or non-modifier `keydown`, but not sooner than 1 s after the text was set), so no stale reply or status text stays readable in the page. On a shared screen with "Sembunyikan pratinjau isi" on, that matters: a later reader in browse mode never finds the last reply at the end of the page. Clearing never interrupts speech already in progress.
- One element never tries to be both status and alert.

**Notices region**
- *Anatomy:* `<section aria-label="Pemberitahuan">` between the navigation and `<main>` in the DOM; visually above the page heading (at the top of the main column on wide layouts). It holds banners (§6.11), including the crisis follow-up banner on Home and in the room, so banners have the same place relative to the content on every page.
- *Position:* in the page flow, never fixed.

**Page header**
- *Anatomy:*
  - an optional back link ("← Catatanku");
  - `<h1 id="page-title">` (one per view). `<main tabindex="-1" aria-labelledby="page-title">` takes its name from it;
  - an optional lead paragraph (`--c-muted`, at most two sentences).

### 6.2 Buttons

| Variant | Look | Use | Never |
|---|---|---|---|
| Primary | Filled `--c-accent`, label `--c-on-accent`, 8px radius; hover and pressed `--c-accent-hover` | One per view or per card: the main next step ("Mulai bercerita", "Kirim", "Simpan") | Two primaries next to each other; a consent choice |
| Secondary | 2px `--c-accent` border, `--c-surface` fill, accent label; hover thickens the underline of the label | Real alternatives ("Tulis catatan"); **both** buttons of a consent choice, so neither looks preferred (§6.9) | — |
| Quiet | Text with underline, no border, 44px hit area | Low-emphasis actions ("Lewati", "Ubah", "Ke balasan terakhir") | Destructive actions |
| Danger | 2px `--c-danger` border, danger label, trash or ban icon. **Never filled** | The first step of delete, forget, revoke, sign out everywhere | First step of an irreversible action without a confirm |
| Confirm-destructive | Filled `--c-text` (ink), `--c-surface` label, trash or ban icon; pressed `--c-muted` | Only the action button(s) inside the confirm step of a destructive action. When a confirm offers **two** destructive choices (deleting a source "with everything made from it" or "only the source", §6.12), **both** use this variant at the same size, and neither is visually recommended; Batal comes last | Anywhere else |
| Help | Filled `--c-help`, `--c-on-help` label, life-buoy icon; hover and pressed `--c-help-hover` (darker in light schemes, lighter in dark ones, so the label contrast rises) | Only "Lihat semua bantuan", where help is the subject: the crisis panel, its condensed form, the pause line, the support line and `/help`. A **link** (`<a href="/help">`), because it navigates | Any other action. Red fill means help and nothing else |
| Help-quiet | `--c-help-bg` fill, 2px `--c-help` border, `--c-help-text` label and life-buoy icon; hover and pressed: a 3px `--c-help-hover` border drawn inset (an inset shadow, so the size never changes) and an underlined label | "Butuh bantuan sekarang" in the top bar, the help strip and the first item of every full-height sheet. A **link** to `/help`. It stays the first control on every screen, with its full label, but it no longer opens every screen with a full-width red block | Any other action. **Pending verification:** owner testing compares Help-quiet with the solid Help fill in the top bar (findability in a hurry, and whether the solid form reads as alarm); the decision is recorded in §11 |

- *Anatomy:* an optional leading icon plus a label (verb first: "Simpan catatan", not "Catatan"). The minimum size is 44×44px, and labels may wrap.
- *States:*
  - hover: as in the table;
  - focus: the ring;
  - busy: `aria-busy="true"`, a leading **static** icon, the label changes to the progressive form ("Menyimpan…"), and the width is kept;
  - unavailable: `aria-disabled="true"`, a dashed border and a reason in an adjacent hint linked by `aria-describedby`.
- *Rules:*
  - Use native `<button type="button">` or `type="submit"` for actions and `<a href>` for navigation. A link styled as a button (Help) keeps link semantics; a button never navigates.
  - Do not use the `disabled` attribute where focus may need to return to the button. Use `aria-disabled` instead (the v2 ConfirmButton rule).
  - When a control disappears while it has focus (Stop when the reply arrives, a row's button after the row is removed), focus moves to a defined target (§6.5, §6.12), never to `<body>`.
  - The same action has the same name everywhere (3.2.4). The glossary is in [CONTENT_STYLE.md](CONTENT_STYLE.md#4-product-glossary).

### 6.3 Inputs

**Text field, password field and text area**
- *Anatomy:*
  1. visible `<label>` above the field (never placeholder-only);
  2. optional hint below the label and above the field (`aria-describedby`);
  3. the field (`--c-surface` fill, `--c-border-control` 1px edge, 8px radius, 44px minimum height);
  4. optional counter;
  5. error text below the field.
- *Password field:* a "Tampilkan" / "Show" toggle (`aria-pressed`) and the right `autocomplete` value. Paste and password managers are always allowed (3.3.8).
- *Password prompts for a known person* (reauthentication before export or deletion, "Buat kode baru", session renewal before the absolute expiry): the dialog or form holds a **read-only username field** (`autocomplete="username"`, shown as "Masuk sebagai pelangi.senja" / "Signed in as pelangi.senja") before the password field (`autocomplete="current-password"`), so password managers can fill a password-only step reliably (1.3.5, 3.3.8).
- *Text area:* grows with its content up to 12 lines (in the room: 40% of the visual viewport, or 25% while typing on a phone, §6.5), then scrolls. `resize: vertical`.
- *Counter:*
  - visible only from 80% of the limit ("3.250 dari 4.000 karakter");
  - announced through `#sr-status` at 80%, at 95% and at the limit only, never on every keystroke.
- *Field error:*
  - the field gets `aria-invalid="true"` and a 2px `--c-danger` edge (drawn as an inset shadow, so the layout does not move);
  - the error text starts with an alert-circle icon and the word "Perlu diperbaiki:" / "Needs fixing:", then says what to do (3.3.1, 3.3.3);
  - the error id is added to `aria-describedby`.
- *Group error* (a required radio group, such as the room's storage choice or the age band): the error belongs to the **group**, not to each radio. The message is placed inside the `<legend>` after the question, in a `<span>` with the alert icon and "Perlu diperbaiki:", so it is read with the group's name. The `<fieldset>` gets `role="radiogroup"` and `aria-invalid="true"` (WAI-ARIA 1.2 supports `aria-invalid` on `radiogroup` but not on `radio`, and deprecates it as a global state). It has no `aria-describedby`, because the legend already carries the message and it would be read twice. The radios carry no `aria-invalid`. Focus moves to the first radio.
- *Form-level errors:*
  - Longer forms show an error summary at the top (`role="alert"`; focus moves to its heading) with links to each field.
  - **Uniform credential errors** (sign in, recovery, the reauthentication and session-renewal dialogs), where the server cannot say which field is wrong: a form-level error region **above the fields**, present empty from page load with `role="alert"`, receives the alert icon, "Perlu diperbaiki:" and the text. The editable fields get `aria-invalid="true"` and list the region in their `aria-describedby`. **Typed values are kept** (the password stays masked, with "Tampilkan" to check it), because a long password typed by hand is costly to retype and the mistake may be the username. Focus moves to the first editable field (the username in sign-in; the code in recovery; the password, with its text selected, in a dialog whose username is read-only). A screen-reader user therefore hears the alert and then that field with the same message.

**Select:** native `<select>` with the same frame. Used only for long lists (no custom listbox). A select never applies a change that moves content or navigates by itself: when the choice moves something (S15 "Ganti jenis"), a separate "Pindahkan" button applies it, because some browsers fire `change` on every arrow key in a closed select (3.2.2).

**Checkbox and radio:**
- native inputs at 24px visual size inside a label row of at least 44px (the label row is the target);
- grouped in `<fieldset>` with a `<legend>`;
- checked state uses the `--c-accent` fill and a check or dot glyph (3:1 against the background).

**Switch:**
- used **only for two-state display preferences** that apply immediately, send nothing anywhere and are trivially reversible ("Jarak teks lebih lega");
- built as `<button role="switch" aria-checked aria-labelledby="…">` whose accessible name comes **from its visible label only** ("Jarak teks lebih lega"). The visible "Aktif" / "Mati" text next to the track is `aria-hidden="true"` (or outside the button), so the state is not read twice ("… Aktif, switch, on"); `aria-checked` carries it;
- **never used** for consent, storage, a three-way choice (theme, contrast and motion are radio groups) or anything that sends data elsewhere (dictation is turned on with a button after its disclosure). Those use explicit buttons or radios with a stated outcome (§6.9).

### 6.4 Choice chips

- *Use:*
  - the intent picker;
  - check-in tags;
  - timeline filters;
  - reading topics;
  - share purpose (single choice).
- *Anatomy:* a pill, 44px minimum height, an optional leading icon and a label. Long labels wrap rather than truncate.
- *Never colour alone:*
  - Unselected: 1px `--c-border-control`, surface fill, regular label.
  - Selected:
    - a 2px `--c-accent` border;
    - an `--c-accent-soft` fill;
    - a **check icon added before the label**;
    - a 600-weight label.

  Three cues besides colour (1.4.1). In forced colours, the check icon and the border remain.
- *Semantics:*
  - Single required choice: native radios inside `<fieldset>`/`<legend>`, visually hidden, with the chip as the `<label>`.
  - Multi choice: native checkboxes, the same way.
  - Optional single choice that can be cleared (the intent picker): `<button aria-pressed>` items inside `role="group"` with `aria-labelledby`. At most one is pressed, and pressing the pressed one clears it.
  - The group label states that the choice is optional ("boleh dilewati" / "optional").
- *Keyboard:*
  - radios: arrows move and select, as native;
  - checkboxes and toggle buttons: Tab moves, Space toggles.
- *Rules:* a chip never navigates, and status chips (§6.8) are not chips.

### 6.5 Conversation: transcript, messages, composer

This is the most important component group. Two sources govern it:

- the streaming guidance in the facts report §1.4 (externally sourced from WAI-ARIA 1.2: `role="log"` has an implicit `aria-live="polite"`, and `aria-busy` is only a "MAY" for assistive technology, so manual tests are mandatory);
- ADR-0003 §5: an AI reply is **released** only after the hard guards and the risk flag have run, after at most one same-provider regeneration (RR-5), and after the verifier when the health-claim detector flags it or DeepSeek wrote it as the fallback (RR-8, §17 O-6). Crisis-flagged output is never shown (RR-7).

**Room layout, compact:**
1. top bar;
2. room toolbar;
3. conversation strip (until the first message; then one line);
4. transcript (the page scrolls);
5. the **room end bar**, in the page flow directly after the transcript: the pause line (when replies are paused, §6.15) and the quiet button "Ke balasan terakhir" (once there is a Sela reply);
6. the **new-content button**, only while the reader is not at the bottom (below);
7. composer, sticky at the bottom.

The bottom navigation is hidden in the room, which is a focused mode (`--bottom-bar-h: 0px`). **Nothing that can grow is inside the sticky layer:** the pause line, the room end bar and per-message notices (including the contact-detail notice) are in the page flow, so the sticky composer holds only its label, the text area and its action row. **Wide layout:** the transcript sits in a column of `--measure` width, with the details panel on the right at 75em and over.

**Typing on a phone** (design decision; compact and medium layouts, while a text-entry field in the room has focus, the condition that already hides the bottom bar, §5.8). The aim is that the person can read Sela's last reply while answering it, on a 360×640-class phone with the keyboard open (a visual viewport of about 230–300px).
1. **Header:** only "Butuh bantuan sekarang" stays fixed, in the help strip (§5.8; one line when it fits). "Keluar cepat" and Menu return to the page flow at the top of the page; they stay the next items in the DOM, and Esc twice still works. The room toolbar and the strip line scroll with the page as always.
2. **Composer mode is unchanged:** it stays sticky.
3. **Caps from the visual viewport** (`--vvh`, written by the shell script from `visualViewport.height`; never `vh`, which iOS Safari does not shrink for the keyboard): the focused text area grows to at most 25% of `--vvh`, never below two lines, then scrolls inside; the sticky composer is at most 50% of `--vvh`.
4. **Compact action row:** the text area and the action group ([Mic] [Hentikan balasan] [Kirim]) share one line whenever the group fits beside at least 10em of text area; otherwise the group moves below the text area as one unit, with Kirim last. This applies in every layout, so focusing the field does not rearrange the composer.
5. **Reading budget:** after the keyboard has opened (the first `visualViewport` resize after focus, debounced), the script compares the room left for the transcript, `--vvh` minus the help strip and the composer, with `--room-reading-min` (three transcript lines). If less would remain, the composer **joins the page flow** for as long as the field keeps focus (`data-typing="flow"`): transcript and composer become one scrolling document under the fixed help strip, so the person scrolls up to read with the whole screen, the field keeps its focus and text, and typing brings the caret back into view. Kirim sits beside or directly below the text area, so it stays in view with the caret, and Tab reaches it uncovered. The decision is made once per focus and kept until blur or an orientation change, so nothing jumps while the person types.
6. **Leaving typing:** on blur the header returns to its normal sticky two rows, and the composer to its normal caps.

Estimated results at 100% text (Liberation Sans proxy as in §6.1; the scripted checks confirm them, ST-11 and S-14):

| Visual viewport | Help strip | Composer while typing (2-line field, Kirim beside it) | Transcript left | Mode |
|---|---|---|---|---|
| 360×300 (Android, layout viewport resized) | 56px | about 112px | about 132px (4.5 lines) | sticky |
| 320×280 | 56px | about 112px | about 112px (3.9 lines) | sticky |
| 360×230 (640px-tall phone, keyboard open) | 56px | about 112px | about 62px | flow: about 174px when scrolled up, about 82px above the field with the caret in view |
| While a reply is pending (Hentikan and Kirim below the field) | — | about 164px | about 2.5 lines less | flow at 320×280 and below |

At 150% text the help label wraps (strip about 95px) and a two-line field is about 102px, so on these viewports the room always uses the flow mode while typing; scrolled up, about 135–205px of transcript (three to four lines at 150%) is visible below the strip. A phone screen with the keyboard open cannot hold four large lines, a two-line field, Kirim and the help control at once; reading by scrolling, with nothing covered, is the honest result (recorded in §10).

**Room toolbar** (part of the page header inside `<main>`; it scrolls with the page):
- *DOM and visual order:*
  1. "← Cerita" (back link);
  2. **"Akhiri percakapan" / "End conversation"** (a secondary button). The visible label is the accessible name, and the crisis panel uses the same label for the same action (2.5.3, 3.2.4);
  3. the `<h1>`: "Percakapan baru" / "New conversation", or the conversation's date and its optional name;
  4. the storage status chip ("Disimpan" with a bookmark icon, or "Tidak disimpan" with bookmark-x);
  5. "Detail" (a button that opens the details sheet on compact or toggles the panel on wide).
- *Wrapping:* by available space, in DOM order. Estimated at 320px and 100% text: row 1 holds the back link and "Akhiri percakapan" (about 268px of 288); row 2 the h1; row 3 the chip and Detail (about 262px). At 150% text each item may take its own row. Nothing truncates.

**Conversation strip:**
- *Where:* above the composer until the first message is sent, then collapsed into one line that names only what applies: "Didengarkan saja · Disimpan · Memori dipakai · Ubah" / "Just listen · Saved · Using memory · Change". The same controls are in Detail at any time.
- *Contents:*
  - the intent picker (§6.4; optional);
  - **storage:** the default chosen in onboarding (S02 step 2) or in Privasi & data (S22), stated as a sentence with "Ubah": "Percakapan ini disimpan di Catatanku · Ubah" (§6.9). The room never asks again unless the person never chose a default;
  - **memory:** "Pakai yang Sela ingat di percakapan ini" / "Use what Sela remembers in this conversation", a Ya / Tidak radio pair with the scope hint "Hanya untuk percakapan ini." / "For this conversation only." (§6.9). Shown only when the `personal_memory` consent is granted and memory is not paused. The default comes from S22 ("Pakai yang Sela ingat di percakapan baru"). With "Tidak", no memory item is sent with this conversation's replies (ADR-0003 §3.1, AT-1).

**Transcript:**
- *Markup:* `<section aria-labelledby>` with a visually hidden `<h2>` "Isi percakapan" / "Conversation". Inside it, `<div role="log" aria-live="off">`.
- **The log is always silent.** `aria-live="off"` is set permanently, whatever "Umumkan balasan ke pembaca layar" is set to. The log gives structure (screen readers expose it as a log, and its articles can be navigated), while every announcement goes through the shell's announcement regions (§6.1) with fixed, short text. So inserting an interactive panel into the log never reads the panel out. This is the weakness that ruled out Direction B, removed by construction.
- *Children,* in DOM order (1.3.2). The log holds only:
  1. message articles (the person's and Sela's);
  2. the **pending block** while a reply is being prepared (one at a time);
  3. per-message notices: "Belum terkirim", the contact-detail notice on a sent message, the no-reply notices, and the support line beside an `elevated` reply;
  4. the crisis panel, right after the message that triggered it (§6.15);
  5. the "Sela menangkap…" card, right after the reply it belongs to (§6.6);
  6. the closing panel, at the end.
- *Keyboard scrolling:* on wide layouts the transcript is its own scroll region with `tabindex="0"`, the name "Isi percakapan" and its own `scroll-padding-block` (2.1.1, 2.4.11). On compact layouts the page scrolls.

**Announcements: what is heard for each event** (design decision; the scripted checks assert this text, §6.3 of ACCESSIBILITY_CRITERIA, and ST-1 to ST-22 test it by hand):

| Event | What appears | What is announced, and where | Focus |
|---|---|---|---|
| The person sends a message | Their article; the composer clears | **Nothing.** The person just wrote it. The next status confirms the send. If no reply will follow (replies paused or AI off): "Terkirim." (`#sr-status`) | Stays in the composer |
| The sent message contained contact details | The message is sent at once. On its article, a non-blocking notice: "Nomor telepon disamarkan untuk AI." with [Samarkan juga di Catatanku] (or, for an ID or card number, "… dan di Catatanku" with [Tampilkan seperti yang kutulis]) | "Nomor telepon disamarkan untuk AI." (`#sr-status`), once, before any reply status | Stays |
| A reply is being prepared | The pending block "Sela sedang menulis…" after the person's message; "Hentikan balasan" appears | "Sela sedang menulis…" (`#sr-status`), once | Stays |
| Cold start (server estimate above 5 s) | The pending block reads "Sela sedang bersiap, bisa sampai sekitar 1 menit." | That sentence (`#sr-status`), once, instead of the line above | Stays |
| Still waiting after 20 s | The pending block adds "Masih menulis. Kadang ini butuh lebih lama." | That sentence (`#sr-status`), once | Stays |
| The reply was flagged and goes to the verifier | The pending block reads "Sela sedang memeriksa balasan ini…" | That sentence (`#sr-status`), once | Stays |
| A guard rejected the reply and the same provider regenerates once (RR-5) | Nothing changes: the pending block still reads "Sela sedang menulis…" | Nothing | Stays |
| **Released** | The pending block is replaced by the Sela article in one DOM insertion | "Umumkan balasan ke pembaca layar" on: **"Sela: ⟨reply text⟩"** (`#sr-reply`), with the reply text in a `<span lang>` when its language differs from the interface, and nothing after it for this reply. Off: "Balasan selesai." (`#sr-status`). The author line, provider, time and meta-line buttons are never part of the announcement | Stays |
| Released with risk flag `elevated` (RR-7) | The article, then a static support line (§6.15) | Announce on: "Sela: ⟨reply text⟩ Bantuan tersedia di bawah balasan ini." as **one** announcement (`#sr-reply`), the reply part in its own `lang`, the suffix in the interface language. Off: "Balasan selesai. Bantuan tersedia di bawah balasan ini." (`#sr-status`) | Stays |
| Withheld (verifier verdict, verifier unavailable, checker not allowed, or discarded at commit) | The pending block is replaced by the no-reply notice, which names the reason. When the person has **never answered** the checker question, the notice holds the in-context checker consent card (§6.9) | "Sela tidak menampilkan balasan ini." (`#sr-status`); the reason is in the visible notice | Stays |
| A guard rejected the regeneration too | The no-reply notice | "Sela tidak menampilkan balasan untuk pesan ini." (`#sr-status`) | Stays |
| Crisis: a lexicon match, generator risk `crisis`, or a verifier `safety_signal` | The crisis panel after the person's message (full or condensed; §6.15). No reply. **At the same moment** the pause line appears in the room end bar, directly above the composer, with [Ke bantuan] | "Bantuan ditampilkan di bawah pesanmu." (`#sr-alert`), and **nothing from the panel itself** | Stays |
| "Sela menangkap…" card inserted | The card after its reply | "Sela menangkap 2 hal di bawah balasan." (`#sr-status`). With a memory proposal: "Sela menangkap 2 hal di bawah balasan, dan 1 usulan untuk diingat." | Stays |
| Stop pressed before release | The pending block is replaced by a meta line "Balasan dihentikan sebelum ditampilkan" | That sentence (`#sr-status`) | Moves to the text area (the Stop button is gone) |
| AI unavailable and no fallback (AT-1 failure path) | The no-AI notice with its actions | "Sela belum bisa membalas sekarang." (`#sr-status`) | Stays |
| Send failed or offline | The article's meta line "Belum terkirim" with "Kirim lagi" | "Belum terkirim. Tulisanmu masih di sini." (`#sr-status`) | Stays |
| The person closes the crisis panel | The panel goes; the pause line stays, and its [Ke bantuan] becomes [Lihat semua bantuan] | "Bantuan ditutup. Balasan Sela tetap dijeda." (`#sr-status`) | Moves to the text area |
| **Anything is inserted while the reader is not at the bottom** (a reply, the crisis panel, a no-reply notice, a reflection card) | The **new-content button** directly above the composer, naming what arrived: "Pesan baru dari Sela", "Bantuan ditampilkan di bawah pesanmu", "Sela tidak menampilkan balasan", "Sela menangkap 2 hal" (each with a ↓ arrow that is `aria-hidden`). When several arrive, it names the most important: help first | Nothing extra: the event's own line was already announced | Stays. **Pressing the button** scrolls to the item and moves focus to its heading or article (with the ring, §5.7) |
| "Akhiri percakapan" | The closing panel | Nothing extra | Moves to the closing panel heading (the person asked for it) |

Ordering rule: after a reply announcement, nothing else about that reply is posted to a live region, because on VoiceOver iOS and TalkBack a later polite update can cut off or drop a long one. "Balasan selesai" is therefore used only when "Umumkan balasan ke pembaca layar" is off. A reflection card that arrives while a reply is probably still being read is the known exception; ST-14 measures it, and the card stays reachable by heading navigation and in Detail if its line is dropped.

**Message block:**
- *Markup:* an `<article>` with a visible author line, the body (`--font-size-4`, full column width, no bubble) and a meta line. Meta-line actions are `--font-size-2` with 44px targets, and the line wraps.
- *The person's message:*
  - author line "Kamu" / "You";
  - a 4px *kunyit* left rule and the quote icon (not colour alone);
  - meta line: time (`<time datetime>`); "Koreksi untuk Sela" when it is a correction; "tanpa balasan (dijeda)" when sent while replies are paused; "tanpa balasan (balasan AI mati)" when AI is off.
- *Sela's reply:*
  - author line "Sela · balasan AI (Meta)" / "Sela · AI reply (Meta)", with the AI monogram. The generator is always named on the visible line, and the verifier when one checked it: "· diperiksa oleh Claude" / "· checked by Claude", or "· diperiksa oleh pemeriksa alternatif (DeepSeek)". A fallback reply is always checked by Claude (ADR-0003 §17 O-6), so it reads "Sela · balasan AI (DeepSeek, cadangan) · diperiksa oleh Claude". The test double reads "Sela · balasan uji (bukan AI)" / "Sela · test reply (not AI)", and a check by the test-double verifier reads "· diperiksa oleh program uji (bukan AI)" / "· checked by a test program (not AI)". **Every part comes from the reply's stored provenance** (generator provider and verification provider, ADR-0003 §10), never from the environment's configuration. This follows ADR-0003 §10 and §10.1, D-046 and D-050;
  - no rule and no fill (calm reading);
  - **no provenance badge and no review-status chip in the transcript** (the exception in §6.7): the author line already says who wrote it, and a "Belum ditinjau" chip on every reply would turn listening into a review queue. The class and status are stored. The only status shown is "Dikoreksi" / "Corrected", on the meta line, after the person sent a correction for that reply. The full badge and chip appear in "Tentang balasan ini" and in exports;
  - meta line: time; "Dikoreksi" when it applies; "Tentang balasan ini" (a disclosure: the badge "Tulisan AI" with its review status, provider, model and served version, route, checks, **whether memory items were used** ("Memakai 2 hal yang Sela ingat · Lihat" or "Tidak memakai yang Sela ingat"), and time; D-030 and D-050); "Kurang tepat?" / "Not quite right?"; "Bacakan" when read-aloud is on.
- *Language of parts (3.1.2):* each message record carries a nullable BCP 47 `lang` field (planned in the P3 contract), **set by the server**, never by a client detector (which would not fit the JavaScript budget, §7). For a reply it is the language Sela was asked to reply in. For the person's text it is the result of the same server-side detection that chooses the reply language ([CONTENT_STYLE.md §6.1](CONTENT_STYLE.md#61-principles-pm-decision-d-035-made-concrete) rule 8), stored only when the detection is confident that most of the text is in one language; mixed text ("Honestly aku udah gak tau…") stays untagged under the 3.1.2 exception for words of indeterminate language. The article gets `lang` when it differs from the interface, and so does the **spoken announcement** of a reply (`#sr-reply`, §6.1). The same nullable `lang` is carried by reflection items, memory items and proposals, Sela-suggested opening lines and summary items, and travels with them into the card, Yang Sela ingat, Persiapan, summaries, the recipient view, the export and print.
- *Sending states:* "Mengirim…" (sending); "Belum terkirim" (not sent) with "Kirim lagi" and "Hapus dari layar". The text is never lost.

**How a reply reaches the screen (post-release):**
1. The server screens the person's text (RR-1). A crisis match goes to the crisis path, with no model call.
2. Otherwise the server generates the **whole** reply, runs the hard guards and the risk flag, regenerates once on the same provider if a guard rejects it (RR-5), and sends a flagged reply, and every reply DeepSeek wrote as the fallback, to the verifier (RR-8, §17 O-6). The client receives only progress events on the operation's event stream: `writing`, `preparing` (cold start), `checking`, then exactly one of `released` (with the final text), `withheld` (with a reason code), `crisis`, `no_ai` or `stopped`. **Unreleased text never reaches the browser**, so it can never be shown, announced or stored by the client (planned in the P3/P5 operation contract).
3. On `released`, the pending block is replaced by the final article in one DOM insertion, with its complete text.
4. **Reveal:** under full motion, the article's sentences fade in one after another (`opacity` only, `--reveal-step` apart, the whole reveal capped at about 1.5 s). The DOM holds the complete text from the first frame, so assistive technology gets it at once. Under reduced motion, or with "Tampilkan balasan sekaligus", there is no reveal.
5. **Focus never moves.** It stays in the composer while Sela writes and after release.
6. **Auto-scroll** happens only while the reader is at the bottom. If they have scrolled up (or a magnifier shows another part of the page), the new-content button appears directly above the composer and names what arrived ("Pesan baru dari Sela" / "New message from Sela"). The same rule covers every insertion: the crisis panel, a no-reply notice and a reflection card. Sighted keyboard, magnifier and touch users therefore always get a visible cue, not only screen-reader users.

Why not stream tokens or sentences (design decision, recorded): token or sentence streaming would put unguarded text on screen, and possibly into the stored transcript, before the hard guards run; VC-8 withholds the **whole** reply when any sentence is unsupported, so text already read would be taken back; an RR-5 regeneration would visibly replace it; and RR-7 forbids showing crisis-flagged output. Releasing sentence by sentence after per-sentence guards was considered and rejected for the second and third reasons. The cost is latency: the first words appear when the whole reply is ready (typically about 12 s warm, with a p95 target of 25 s in ADR-0003 §12) rather than at the first token. Mitigations: the status appears at once, Stop is always there, and the person can keep typing. P5 measures it.

**Stop:**
- While a reply is pending (before release), "Hentikan balasan" / "Stop reply" (square icon, secondary button) is in the composer row, before Send. Send is `aria-disabled` with the hint "Tunggu balasan Sela, atau hentikan dulu." / "Wait for Sela's reply, or stop it first."
- Pressing Stop cancels the operation (`DELETE`, idempotent). Nothing of the reply is shown or stored. The pending block becomes the meta line "Balasan dihentikan sebelum ditampilkan" / "Reply stopped before it was shown". There is no partial reply, because nothing was released.
- If the server had already released the reply when Stop arrived, the reply appears normally (it passed every guard) and the status says "Balasan sudah selesai sebelum dihentikan."
- When Stop disappears while it has focus (pressed, or the reply was released), focus moves to the text area.

**"Kurang tepat?" (correction):**
- Opens an inline field under the reply: "Apa yang kurang tepat? Tulis dengan kata-katamu." / "What wasn't right? Say it in your own words." Focus goes to the field.
- Sending it adds a message marked "Koreksi untuk Sela" (class `user_correction`; §6.7.1). Sela's next reply takes it into account and does not argue. The corrected reply's review status becomes "Dikoreksi".

**"Ke balasan terakhir" / "Go to the last reply":**
- A quiet button in the **room end bar**, in the page flow directly before the composer (so Shift+Tab from the text area reaches it first). It is outside the sticky layer, which keeps the composer small while typing.
- It moves focus to the last Sela article (`tabindex="-1"`), which then shows the ring and the 4px leading bar on any focus (§5.7), so screen-reader users with announcements off, and magnifier and touch users, can find it without hunting.

**New-content button:**
- A quiet button with a top border, attached to the top edge of the composer (sticky with it; on wide layouts at the bottom of the transcript column). It exists only while the reader is not at the bottom and something new was inserted.
- Its label names what arrived (table above). Pressing it scrolls to the item and moves focus to the item's heading (the crisis panel's h3, the card's h3) or article; it then disappears.
- It is never announced by itself; the item's own line already was.

**Composer:**
- *Anatomy:*
  1. visible label "Tulis untuk Sela" / "Write to Sela";
  2. a hint, visible only with a fine pointer, always in `aria-describedby`: "Enter untuk baris baru, Ctrl+Enter untuk mengirim." / "Enter for a new line, Ctrl+Enter to send.";
  3. the text area (`--c-sunken` area, `--c-surface` field);
  4. the action group: [Mic, only when dictation is on] [Hentikan balasan, while a reply is pending] [**Kirim** (primary, send icon plus label)].

  The pause line and "Ke balasan terakhir" are in the room end bar before the composer, and the contact-detail notice is on the sent message, so none of them can make the sticky layer taller (§6.5 room layout).
- *Text area and actions share a line when they fit* (`flex-wrap: wrap`; the text area `flex: 1 1 10em`, the group `flex: 0 0 auto`), whatever the text size; Kirim is always last. Estimated at 320px and 100% text: idle, the field and Kirim on one line (Kirim about 94px); while a reply is pending, "Hentikan balasan" and Kirim (about 285px) move below the field.
- *Keyboard:*
  - Ctrl or Cmd+Enter sends.
  - Enter adds a new line, unless "Enter untuk mengirim" is on.
  - Esc inside the composer does nothing on its own. A first Esc arms Quick exit (§6.12).
- *Contact details* (design decision, round 2): **sending is never gated.** The server always replaces contact details with placeholders before any model call (phone and WhatsApp numbers, email, NIK, card numbers, URLs, social handles; the v1 `piiRedactor` port, ADR-0003 DM-2), so AI never receives them whatever the person chooses; the only real decision is how the person's **stored** copy looks. So the message is sent at once, and the server's redaction result (not a client copy of the patterns) adds a non-blocking notice to the sent message's article:
  - phone, WhatsApp, email, URL, social handle: "Nomor telepon disamarkan untuk AI." / "Phone number hidden from AI." with [Samarkan juga di Catatanku] / [Hide it in My notes too]. The stored copy stays as written until pressed (when the conversation is saved);
  - NIK and payment-card numbers (high-risk kinds): "Nomor KTP disamarkan untuk AI dan di Catatanku." / "ID number hidden from AI and in My notes." with [Tampilkan seperti yang kutulis] / [Show it as I wrote it]. The stored copy is masked by default, because a stored ID or card number can do harm if the device or an export leaks; the person can undo it for that message;
  - the notice names the kind, never repeats the detail, and stays until the person acts or leaves the conversation; `#sr-status` says the first sentence once;
  - unsaved conversations show only the first sentence (nothing is stored);
  - **pattern quality** (acceptance for P3, owned by the AI engineer): the shared pattern set is run against an Indonesian false-positive corpus of times, money and dates ("jam 20.30", "pukul 08.00", "Rp50.000", "Rp 1.250.000", "12/10/2026", "tgl 3-4"), office or place handles in prose ("@kantor", "di @rumah"), and English equivalents; none of them may trigger the notice or a placeholder.

  Names on the person's "Nama yang disamarkan" list (§6.9) are masked before every model call without asking.
- *Limits:* 4,000 characters per message (planned; the counter appears at 80%).
- *Drafts:* kept in memory only, never in `localStorage` (shared devices). They survive network errors and session expiry within the tab. Quick exit discards them.
- *Offline:* Send stays available. Pressing it shows "Belum terkirim — tidak ada koneksi" with "Kirim lagi", and the draft stays. Sela never resends a message by itself.

**Voice (all opt-in; D-038; brief §6):**
- *Dictation:*
  - The mic button appears only after dictation is turned on in Display settings with the button "Nyalakan dikte" / "Turn on dictation", shown under this disclosure: "Dikte memakai layanan suara dari browsermu. Di beberapa browser (misalnya Chrome), suaramu dikirim ke server penyedia browser untuk diubah menjadi teks." / "Dictation uses your browser's speech service. In some browsers (for example Chrome), your voice is sent to the browser vendor's servers to be turned into text."
  - Microphone permission is requested only when the person presses the mic.
  - Recording state is shown in text ("Mendengarkan… tekan lagi untuk berhenti") and announced through `#sr-status`.
  - It is a toggle button (`aria-pressed`), never press-and-hold (2.5.2).
  - The transcript goes **into the editable composer**. It is never sent automatically.
- *Read aloud:*
  - "Bacakan dengan suara" lists only voices with `localService === true`, so text does not leave the device through a remote voice. If there is no local voice for the language, it says so and stays off.
  - It never autoplays. Each Sela reply gets a "Bacakan" / "Read aloud" button, with Pause and Stop. A reply with `lang` uses a voice for that language.

### 6.6 Reflection card: "Sela menangkap…"

- *Use:*
  - inside the transcript, after the reply in which Sela offers a reflection;
  - in the conversation's details;
  - in the closing panel (S09).
- *When it appears (design decision, interaction tied to intent):*
  - "Didengarkan saja" (just listen): only when the person presses "Apa yang Sela tangkap?" in Detail or the closing panel.
  - "Merapikan pikiran", "Memahami pola" and "Menyiapkan konsultasi": Sela may offer one after enough has been said, at most once every few turns. Turn counts and thresholds are set by the AI engineer and the PM in P3/P5.
  - Like a reply, a card is shown only after release (AT-2: guards, and the verifier when flagged). A card that is not released is simply not shown; nothing replaces it (ADR-0003 §3).
- *Anatomy:*
  - `<section>` with an `<h3>`: "Sela menangkap…" / "Sela understood…";
  - the generator line (§6.7): "Ditulis dengan bantuan AI (Meta)", plus "· diperiksa oleh Claude" when it was flagged and checked (or the test-program form, from the stored provenance);
  - a one-line explanation, phrased as an offer: "Ini tafsiran Sela, belum tentu benar. Kalau mau, periksa satu per satu." / "This is Sela's reading and may be wrong. Check each one if you like.";
  - an `<ol>` of one to three statements, each `<li>` with `lang` when the item's `lang` differs from the interface;
  - "Ada yang terlewat? Tambahkan dengan kata-katamu" / "Missed something? Add it in your own words".
- *Each statement:*
  - the text, with the badge "Tafsiran Sela" (class `unconfirmed_interpretation`) and, next to it, the review status chip, starting at "Belum ditinjau" (§6.7);
  - actions:
    - "Benar" / "That's right": status becomes **Dikonfirmasi**. The badge stays "Tafsiran Sela", because the class says who wrote it;
    - "Ubah" / "Change": inline field; the new version has class `user_correction` ("Koreksimu") and status **Dikoreksi**. The original stays visible in the history;
    - "Bukan, hapus" / "No, remove": status becomes **Ditolak**. It collapses to "Kamu menghapus satu tafsiran · Urungkan" / "You removed one reading · Undo", and the Undo stays until the card is left (no timer).
- *Insertion:* the card is inserted into the silent log and announced with one short line, "Sela menangkap 2 hal di bawah balasan." / "Sela understood 2 things, below the reply." (§6.5). Focus does not move. If the reader is not at the bottom, the new-content button names it.
- *Card states:*
  - loading: "Sela sedang menyusun…", with Stop;
  - verifier gate: memory proposals from this card always go through the verifier (D-030, AT-3). While checking, the card shows "Sedang diperiksa…". If no permitted verifier is available, no proposal is offered and the card says why, in neutral words: "Sela tidak mengusulkan untuk mengingat apa pun sekarang, karena pemeriksa (Claude) belum diizinkan." or "… karena pemeriksanya sedang tidak tersedia." ([CONTENT_STYLE.md](CONTENT_STYLE.md#79-ai-provenance-and-checks));
  - **storage off:** the card works but nothing is kept, and it says so: "Tidak disimpan, karena percakapan ini tidak disimpan." **A conversation that is not saved never produces memory proposals** (ADR-0003 AT-3 and DM-4), and the card says why: "Sela tidak mengusulkan untuk mengingat apa pun dari percakapan yang tidak disimpan." / "Sela doesn't suggest remembering anything from a conversation that isn't saved."
- *Memory consent asked in context:* the first time a proposal could be offered without the `personal_memory` consent, the card shows the consent card (§6.9) with three buttons of equal weight: [Izinkan Sela mengingat] [Tidak sekarang] [Jangan tanya lagi]. After "Tidak sekarang", Sela does not ask again in this conversation. "Jangan tanya lagi" stops the question everywhere until the person changes it in S22. **Never for 16–17 or `unknown`:** for them memory is opt-in in Yang Sela ingat only (D-031). Until they turn it on there, the card offers no proposal and asks no consent question; afterwards proposals appear as for adults.
- *Keyboard:* Tab moves through each statement's actions in reading order. After an action, focus stays on the same statement's next sensible control, or on Undo.
- *Never:*
  - more than three statements;
  - labels or diagnoses ("kamu depresi");
  - statements about other people's minds stated as facts.

### 6.7 Provenance badges and review status

Six provenance classes (PM decision D-040). Each has a **distinct icon, label and border style**, so none relies on colour. The badge is a 4px-radius tag at `--font-size-2` (16px), semibold.

**The class and the review status are two separate things, shown side by side and never merged** (D-040). The class says *who wrote this content* and never changes when the person reviews it. The status says *what the person did with it*. A content edit creates a new version, which may have a different class (§6.7.1).

**One exception: Sela's replies in the transcript** (design decision, round 2). A reply carries only its author line ("Sela · balasan AI (Meta)", which already names the class in words) and, after a correction, "Dikoreksi" on its meta line. Its badge and its review status are stored and shown in "Tentang balasan ini", in exports and anywhere the reply is listed outside the room, but not on every reply in the conversation: "Tulisan AI · Belum ditinjau" under each reply would turn listening into a review queue, against brief §1 and §5. Everything else that is kept (reflection items, memory items, summary items, opening lines) shows badge and status side by side.

| Class | Badge ID / EN | Icon | Block style where the content is shown | Colour role |
|---|---|---|---|---|
| `user_original` | "Kata-katamu" / "Your words" | quote | 4px solid left rule; `--c-user-bg` tint in notes and summaries; no tint in the transcript | user |
| `ai_summary` | "Tulisan AI" / "AI wording" | "AI" monogram | 1px **dashed** outline, `--c-ai-bg` tint | ai |
| `user_correction` | "Koreksimu" / "Your correction" | pencil | 4px solid left rule plus "dikoreksi dari: …" disclosure of the earlier text | user |
| `unconfirmed_interpretation` | "Tafsiran Sela" / "Sela's interpretation" | circle-help | 2px **dotted** outline, `--c-ai-bg` tint | ai |
| `uploaded_document` (reserved, D-043) | "Dokumen unggahan" / "Uploaded document" | paperclip | 1px solid outline | text |
| `verified_professional_note` (reserved, D-042) | "Catatan profesional terverifikasi" / "Verified professional note" | badge-check | 2px solid outline | text |

- **Reserved classes are never rendered in the pilot.** The component refuses `verified_professional_note` unless the server marks the record as verified and the environment enables it. In development this throws, and in production it falls back to showing nothing plus a logged, content-free counter (planned). No verified professionals exist (D-042).
- **Review status chip** (next to the badge; `--font-size-2`): "Belum ditinjau" / "Not reviewed" (circle), "Dikonfirmasi" / "Confirmed" (check), "Dikoreksi" / "Corrected" (pencil), "Ditolak" / "Rejected" (ban). The person's own words are `confirmed` by definition; their chip is recorded but not shown.
- **Generator line** on AI-written content (D-050; ADR-0003 §10). Every AI-written item, or the heading of a group of AI-written items, carries one line that names who wrote it and who checked it ([CONTENT_STYLE.md §7.9](CONTENT_STYLE.md#79-ai-provenance-and-checks)):
  - "Ditulis dengan bantuan AI (Meta)" / "Written with AI help (Meta)", or "(DeepSeek, cadangan)" / "(DeepSeek, backup)";
  - then "· diperiksa oleh Claude" / "· checked by Claude", or "· diperiksa oleh pemeriksa alternatif (DeepSeek)" / "· checked by the alternative checker (DeepSeek)", when a verifier checked it;
  - the test-double generator: "Ditulis oleh program uji (bukan AI)" / "Written by a test program (not AI)";
  - the test-double verifier (ADR-0003 §10.1; development, test and the demo): "· diperiksa oleh program uji (bukan AI)" / "· checked by a test program (not AI)", whether it stood in for Claude or for the alternative checker. The words "Claude" and "DeepSeek" never appear in the provenance of an item a test program checked;
  - template text (generator `manual`) whose quoted parts are all the person's own words: "Dari templat Sela, tanpa AI" / "From a Sela template, no AI";
  - template text that quotes AI wording the person confirmed: "Dari templat Sela, memakai kalimat AI yang sudah kamu konfirmasi (Meta · diperiksa oleh Claude)" / "From a Sela template, using AI wording you confirmed (Meta · checked by Claude)", naming the original generator and checker; when it quotes a correction the person made (and no AI wording): "Dari templat Sela, memakai kalimat yang kamu koreksi" / "From a Sela template, using wording you corrected". "Tanpa AI" is used only when every quoted part is `user_original`.

  **Source of every part:** the item's **stored** generation and verification provenance (ADR-0003 §10: generator provider, verification provider and status), never the environment's configuration. A description of what *will* happen (the summary builder's step 2, a consent card) may come from the live AI status, and in a test-double environment it says so ("program uji"); a label of what *did* happen always comes from the record.

  It appears in the room, the reflection card, memory items, the summary builder and view, the share preview, **the recipient page and the print view**. In a reply it is the author line (§6.5).
- **Meta line** (full form): `[badge] [status] · 12 Okt 2026, 20.14 · versi 2`. In the compact list form, only the badge and the date show; the rest is in the item detail.
- **Accessibility:** the badge and the chip are plain text with an `aria-hidden` icon, so screen readers read "Tafsiran Sela, Dikonfirmasi". Each item's container has `aria-describedby` pointing at its meta line, so the provenance is read with the content.

#### 6.7.1 Record types: class, statuses and transitions

One mapping for every record type, using only the six D-040 classes and the four review statuses, aligned with ADR-0003 §3 and §10. ADR-0003 §10 allows only `ai_summary` and `unconfirmed_interpretation` for AI output; `ai_summary` is therefore the class of all AI *wording* (replies, summary wording, suggested opening lines), and its badge reads "Tulisan AI", not "Ringkasan AI", so it is honest outside summaries too. **That replies are `ai_summary` is the design's reading, pending confirmation by the AI engineer and the PM** ([§10](#10-open-points-for-the-pm-and-other-owners)); if they choose otherwise, only the reply row changes. Generator values are ADR-0003's (`meta_modal`, `deepseek`, `test_double`, `none`; draft value `manual` for templates). Every row that holds text also carries the nullable `lang` of §6.5.

**Only the person's own words are quoted.** A quote block ("Kata-katamu", the kunyit rule) is only ever `user_original` (or `user_correction`, labelled "Koreksimu"). Anything Sela wrote, even after the person confirmed it, is shown as an item with its own class and generator line, never inside a quote block, and the recipient sees it as Sela's interpretation, not the sender's words (brief §7, D-040).

| Record (screen) | Generator | Class at creation | Statuses | Transitions (who) |
|---|---|---|---|---|
| The person's message (S09) | — | `user_original` | `confirmed` (fixed; chip not shown) | None. Deleted with the conversation |
| Correction message, "Kurang tepat?" (S09) | — | `user_correction` | `confirmed` (fixed) | None |
| Sela's reply (S09) | Meta, or DeepSeek as fallback; test double in tests | `ai_summary` | `unreviewed`, `corrected` | → `corrected` when the person sends a correction for it. The reply text never changes; the correction is its own message, linked by `replyTo` |
| Reflection item, "Sela menangkap…" (S09, S10) | Meta or DeepSeek; verified when flagged | `unconfirmed_interpretation` | `unreviewed`, `confirmed`, `corrected`, `rejected` | Benar → `confirmed` (class unchanged). Ubah → new version, class `user_correction`, status `corrected` (the earlier version is kept in history). Bukan, hapus → `rejected`. Urungkan → back to the previous status |
| Memory proposal (S09, S13) | Meta or DeepSeek; **always verified** (AT-3) | `unconfirmed_interpretation` | `unreviewed` | Ingat → becomes a memory item (next row). Ubah before accepting → memory item of class `user_correction`, status `corrected`. Jangan → `rejected`; the proposal is discarded and only a content-free counter remains. Never created from an unsaved conversation |
| Memory item accepted from a proposal (S13) | as above | `unconfirmed_interpretation` | `confirmed`, `corrected` | Ubah → new version, `user_correction`, `corrected`. Lupakan deletes it (not a status); when a summary includes it, Lupakan uses the derived-items confirm (§6.12) |
| Memory item added by hand (S13) | — | `user_original` | `confirmed` | Ubah → new version, still `user_original`. Lupakan deletes it, with the derived-items confirm when a summary includes it |
| Summary item: the quote (S15) | — | `user_original`, copied from the source entry at its version, with private names masked | `confirmed` (fixed) | Removed with its item |
| Summary item from a memory item that Sela proposed (S15) | The memory item's original generator and checker, shown in its generator line | `unconfirmed_interpretation`, copied at the memory item's version. **No quote block**: it has no words of the person to quote, and it is never sent to AI drafting as the person's words. It shows "Tafsiran Sela" with the line "Dari Yang Sela ingat · asalnya percakapan 24 Okt" (a link to the source, when it still exists) | Starts `unreviewed` in the summary (the person checks it again for this new context, whatever its status in memory) → `confirmed`, `corrected`, `rejected` | Benar → `confirmed`; shown as "Tafsiran Sela · Dikonfirmasi"; the recipient sees "Tafsiran Sela · dikonfirmasi pengirim". Ubah → new version, `user_correction`, `corrected` ("Koreksimu"). Hapus → `rejected`. There is no "Pakai kata-kataku saja", because there is no quote; "Lihat asalnya" opens the source so the person can add their own words as a quote |
| Summary item from a memory item the person wrote or corrected (S15) | — | `user_original` ("Kata-katamu · dari Yang Sela ingat, kamu tambahkan 24 Okt") or `user_correction` ("Koreksimu"), copied at its version; shown as the quote, with the quote as the wording | `confirmed` | As a manual-path item |
| Summary item: the wording, AI path (S15) | Meta or DeepSeek; **always verified** (AT-4) | `ai_summary` | `unreviewed`, `confirmed`, `corrected`, `rejected` | Benar → `confirmed`. Pakai kata-kataku saja → the wording becomes the quote: class `user_original`, `confirmed`. Ubah → `user_correction`, `corrected`. Hapus → `rejected` (the item leaves the summary and stays in its version history) |
| Summary item: the wording, manual path (S15) | `manual` | `user_original` | `confirmed` | Ubah → new version, still `user_original` |
| Opening line from a template (S16) | `manual`, plus the original generator and checker of any AI wording it quotes | The class of the content it quotes: `user_original` for a quote ("Dari templat Sela, tanpa AI"), `ai_summary` for confirmed AI wording ("Dari templat Sela, memakai kalimat AI yang sudah kamu konfirmasi (Meta · diperiksa oleh Claude)"), `user_correction` for corrected wording | `confirmed` | Ubah → `user_correction`, `corrected` |
| Opening line suggested by Sela (S16) | Meta or DeepSeek; verified when flagged (AT-5) | `ai_summary` | `unreviewed`, `confirmed`, `corrected`, `rejected` | Pakai kalimat ini → `confirmed`. Ubah → `user_correction`, `corrected`. Hapus → `rejected` |
| Own opening line (S16) | — | `user_original` | `confirmed` | Ubah → new version |
| Note, follow-up note, check-in (S10, S11, S19) | — | `user_original` | `confirmed` | Ubah → new version, still `user_original` |
| Shared projection item (S17, S25) | Frozen from the approved summary version | Inherited | Inherited, frozen | None. Revoking ends access |
| Uploaded document; verified professional note | — | Reserved | — | Never created in the pilot |

### 6.8 Handoff status chips and the share's event list

PM decision D-041:

- **flow:** prepared → shared → opened → acknowledged → consultation_in_progress;
- **end states:** revoked, expired, ended.

| Status | Label ID / EN | Icon | Who records it |
|---|---|---|---|
| `prepared` | "Disiapkan" / "Prepared" | circle | System, when a summary version is approved and can be shared |
| `shared` | "Dibagikan" / "Shared" | share | System, when the link and code are created |
| `opened` | "Dibuka penerima" / "Opened by recipient" | eye | System, on the recipient's first successful sign-in |
| `acknowledged` | "Diakui penerima" / "Acknowledged by recipient" | check | The recipient's action |
| `consultation_in_progress` | "Kamu menandai: konsultasi berlangsung" / "You marked: consultation in progress" | pencil-line | **The person**, with "Aku sudah mulai konsultasi". It is self-reported, so the label says who set it. Who else may set it is pending P4 |
| `revoked` | "Akses dicabut" / "Access revoked" | ban | The person |
| `expired` | "Kedaluwarsa" / "Expired" | clock | System |
| `ended` | "Selesai dibagikan" / "Sharing ended" | square-check | The person, with "Tandai selesai dan hentikan akses", or the recipient's "Tidak dapat menerima" (pending P4) |

- **End states always say who ended them and why.** The D-041 value stays; the chip is always followed by a reason line, wherever it appears (S14, S15, S18, the timeline):
  - "Selesai dibagikan · kamu menandai konsultasi selesai" / "Sharing ended · you marked the consultation finished";
  - "Selesai dibagikan · penerima tidak dapat menerima" / "Sharing ended · the recipient couldn't accept it";
  - "Akses dicabut · olehmu" / "Access revoked · by you" (or "· karena sumbernya dihapus" / "· because its source was deleted");
  - "Kedaluwarsa · 2 Nov 2026, 21.00" / "Expired · 2 Nov 2026, 21:00".

  A recipient's decline therefore never reads as if the share had served its purpose.
- **Where each label appears.** All five flow labels appear in one consistent place: the share status page (S18). `prepared` also appears in S14 and S15 as "Disiapkan · versi 3" for an approved version that has not been shared; once a version is shared, S14 shows the share's latest recorded status instead.
- **Status chip:** a 4px-radius tag at `--font-size-2` with icon and label; neutral border by default; `--c-warning-border` for expired. Revoked is a person's own choice, not an error, so it uses neutral styling, not danger. The chip shows the **latest recorded** event.
- **Event list (replaces a progress stepper).** An `<ol>` of the five flow steps in D-041 order, built **only from recorded events**:
  - each step shows its own recorded time: "Dibuka penerima · 26 Okt 2026, 21.03";
  - a step with no record shows "belum" / "not yet" when no later step is recorded, and "tidak tercatat" / "not recorded" when a later step is recorded but this one is not. For example, the summary was shown on screen in person, then the person marked the consultation: "Dibuka penerima · tidak tercatat";
  - **a step is never inferred from a later one.** No check mark appears without a recorded time;
  - the latest recorded step carries `aria-current="step"` and a 600-weight label;
  - in an end state, the list stays as history, headed by the end-state chip with its time.
- **Fixed sentence under every status:** "Dibagikan atau dibuka tidak berarti kamu sudah mendapat perawatan." / "Shared or opened does not mean you have received care." (D-041).

### 6.9 Consent, storage, memory and private-name controls

**Consent card** (each AI provider and each other purpose):
- *Anatomy:*
  - `<h3>` with purpose and provider ("Balasan AI · Meta, diproses di Modal (Amerika Serikat)");
  - a definition list:
    - Untuk apa (what for);
    - Apa yang dikirim (what is sent);
    - Di mana diproses (where it is processed);
    - Berapa lama penyedia menyimpannya (how long the provider keeps it; pending verification per provider);
    - Kalau tidak diizinkan (if not allowed);
  - a status line with icon and text ("Diizinkan sejak 26 Okt 2026 · versi kebijakan …" or "Belum diizinkan");
  - the actions (below);
  - a link to the history.
- *Actions:*
  - **Asked in context** (onboarding, or the first time a feature needs it): two buttons of **equal visual weight**, both Secondary: "Izinkan" / "Allow" and "Tidak sekarang" / "Not now". Asks that could recur (memory, the verifier) add a third, equal button: "Jangan tanya lagi" / "Don't ask again". After "Tidak sekarang", Sela does not ask again in the same conversation or summary. "Jangan tanya lagi" stops the in-context question until the person turns it back on in S22 ("Tanyakan lagi saat diperlukan").
  - **Several cards on one screen** (onboarding step 3): the allow button names its purpose ("Izinkan balasan AI", "Izinkan pemeriksa", "Izinkan model cadangan"), and each "Tidak sekarang" gets a visually hidden suffix with the purpose, so a button list or voice control never meets two identical names (2.4.6, 2.5.3). Pressing a button records that card's answer at once (`PUT /api/v3/me/consents/{purpose}`) and turns the card's status line into "Diizinkan" or "Tidak diizinkan untuk sekarang" with [Ubah].
  - **The checker (Claude) asked in context** only when the person never answered it (a profile from before the onboarding card existed, or someone who left onboarding through "Lihat beranda dulu"): the first reply withheld for lack of a checker shows this card inside its no-reply notice, with [Izinkan pemeriksa] [Tidak sekarang] [Jangan tanya lagi]. After "Izinkan pemeriksa", a quiet "Minta Sela membalas lagi" / "Ask Sela to reply again" starts a new reply for the same message; the withheld one is never shown. Whether the person answered is a profile flag (planned P3, with the "ask again" flags), because a consent record has only granted and withdrawn.
  - **In Privasi & data (S22):** one button that matches the state: "Izinkan" / "Allow", or "Cabut izin" / "Withdraw".
- *Semantics:* a change shows a status next to the button and in `#sr-status`: "Izin dicabut. Sela berhenti memakai Meta mulai sekarang." / "Permission withdrawn. Sela stops using Meta from now on."
- *Rules:*
  - No switches.
  - No pre-checked boxes, and no visual preference for "Izinkan".
  - One purpose per card (UU PDP Pasal 22 requires distinguishable purposes; externally sourced in the facts report).
  - Withdrawing never needs reauthentication; granting needs the current policy version.
  - Never asked during an outage (ADR-0003 §8).

**Storage choice:**
- *Look:* a radio group shown as two large option rows:
  - "Simpan percakapan ini di Catatanku" / "Save this conversation to My notes", with the hint "Kamu bisa menghapusnya kapan saja." / "You can delete it at any time.";
  - "Jangan simpan" / "Don't save", with the hint "Sela memproses tulisanmu untuk membalas dan tidak menyimpannya. Percakapan dibuang saat kamu menutup tab ini, memulai percakapan baru, atau memakai Keluar cepat. Penyedia AI yang membalas bisa menyimpannya sementara sesuai kebijakannya; lihat Privasi & data." / "Sela processes your writing to reply and doesn't keep it. The conversation is discarded when you close this tab, start a new one or use Quick exit. The AI provider that replies may keep it for a while under its own policy; see Privacy & data." The provider clause is finalised with the P3 and P5 contracts.
- *Where it is chosen:* once in onboarding as the default for new conversations (S02 step 2), changeable in S22, and per conversation in the strip and Detail.
- *Error:* if the room ever has no default (an account created before this choice existed), Send with nothing selected uses the group-error pattern (§6.3): "Perlu diperbaiki: pilih dulu, simpan atau jangan simpan." in the legend, and focus moves to the first option.
- *Changing mid-conversation:*
  - From saved to not saved, a dialog asks:
    - "Hapus juga yang sudah tersimpan dari percakapan ini" / "Also delete what's already saved"; or
    - "Simpan yang sudah ada, berhenti menyimpan mulai sekarang" / "Keep what's saved, stop saving from now".
  - From not saved to saved, it asks:
    - "Simpan juga pesan sebelumnya" / "Also save earlier messages"; or
    - "Mulai sekarang saja" / "From now on only".
- *Exact behaviour of "don't save"* (in-memory hold time, reconnection) is set by the P3 conversation contract (planned). The copy must match it. Two rules are fixed here (2.2.1, 3.3.4):
  - **The hold is kept across in-app navigation.** An unsaved conversation stays held in the tab and on the server while the person opens Help, Privasi & data or any other place, and returns with "Percakapan yang sedang berjalan (tidak disimpan)" (S07, S08). It is discarded only by Quick exit, closing the tab or leaving the site (`pagehide`, `sendBeacon`), starting a new conversation (after an inline confirm that says so), "Buang sekarang", or the hold timeout below.
  - **The hold timeout warns first.** At least 2 minutes (never less than 20 seconds) before an inactive hold runs out, a dialog says: "Percakapan yang tidak disimpan ini akan dibuang dalam sekitar 2 menit karena tidak ada aktivitas." / "This unsaved conversation will be discarded in about 2 minutes because of inactivity." [Tetap buka / Keep it open] [Simpan percakapan ini / Save this conversation] [Buang sekarang / Discard now], plus the help link. "Tetap buka" restarts the hold with one action, any number of times (at least ten). If the hold still runs out, the tab keeps the transcript visible, read only, with "Percakapan ini sudah dibuang dari server Sela. Isinya masih terlihat di tab ini sampai kamu meninggalkannya." and [Mulai percakapan baru]; the unsent draft stays in the composer.

**Memory controls and their scopes** (design decision, round 2). Four controls touch memory. Each one shows a one-line scope next to it, so they cannot be confused:

| Control | Where | Scope line (ID / EN) |
|---|---|---|
| `personal_memory` consent | S13 (asked there or in context), S22 Memori | "Izin supaya Sela boleh menyimpan dan memakai yang kamu setujui. Mencabutnya melupakan semuanya." / "Permission for Sela to keep and use what you agree to. Withdrawing it forgets everything." |
| "Jeda memori" / "Pause memory" | S13, S22 Memori | "Berlaku untuk semua percakapan mulai sekarang, termasuk yang sedang terbuka. Yang Sela ingat tetap tersimpan." / "Applies to every conversation from now, including open ones. What Sela remembers stays saved." |
| "Pakai yang Sela ingat di percakapan baru" (Ya / Tidak) | S22 Memori | "Berlaku untuk percakapan yang kamu mulai setelah ini. Kamu tetap bisa mengubahnya di setiap percakapan." / "Applies to conversations you start after this. You can still change it in each conversation." |
| "Pakai yang Sela ingat di percakapan ini" (Ya / Tidak) | The room's strip and Detail | "Hanya untuk percakapan ini." / "For this conversation only." |

- The per-conversation choice is shown only when memory is on (consent granted, not paused).
- "Tentang balasan ini" says whether memory items were used for that reply.
- An unsaved conversation can *use* memory (nothing is stored), but never *produces* memory proposals (AT-3, DM-4).
- **Withdrawing the consent forgets everything by default:** "Cabut izin mengingat? Sela akan melupakan semua yang Sela ingat (12 hal). Percakapan dan catatanmu tetap ada." [Cabut izin dan lupakan semuanya] [Batal], with the derived-items extension of §6.12 when summaries include memory items. Keeping the items paused after withdrawal is **not offered** unless the legal review (B-10) confirms a basis for storing them without the consent ([§10](#10-open-points-for-the-pm-and-other-owners)).

**Names kept private ("Nama yang disamarkan" / "Names kept private")** (D-030 data minimisation; ADR-0003 DM-2; v1 F23 redaction preview):
- *Where:* a section of Privasi & data (S22), and inline where a suggestion appears.
- *Anatomy:* a list of names or words the person added ("Rina", "Pak Budi", "SMA 3"), each with [Hapus dari daftar]; a field "Tambahkan nama atau kata" / "Add a name or word" with [Tambahkan]; the explanation "Sela mengganti nama ini dengan [nama disamarkan] sebelum mengirim apa pun ke AI, dan di ringkasan yang kamu bagikan, kecuali kamu memilih menampilkannya." / "Sela replaces these names with [name hidden] before sending anything to AI, and in summaries you share, unless you choose to show them."
- *Effect:*
  - before every model call, listed names and all contact details become placeholders (`[NAMA-1]`, `[KONTAK-1]`) and are mapped back only in the person's own view (DM-2);
  - in the summary builder (S15), a quote shows a masked name as "Rina" with a dotted underline, the eye-off icon and the visually hidden text "(disamarkan)", so the person sees which name is hidden;
  - in the share preview (S17) and on the recipient page, it is "[nama disamarkan]" / "[name hidden]", unless the person chose to show that name in that share (S17 step 1).
- *Inline suggestion (deferred, see below):* "Samarkan 'Pak Budi'? Sela akan menggantinya dengan [nama disamarkan] sebelum dikirim ke AI dan di ringkasan." [Samarkan nama ini] [Biarkan], offered at most once per name.
- *Decision (design decision, recorded):* the list, the masking before every model call, the masking in summaries and shares with a per-name choice per share, and the contact-detail notice in the composer (§6.5) are in scope for P3 and P4. **Automatic detection of names is deferred** until a deterministic, bilingual heuristic (for example honorific patterns such as "Pak", "Bu", "Mas", "Mbak", "Kak" followed by a capitalised word) has a measured false-positive rate on Indonesian and English text. Until then, names are masked only when the person lists them. Detection is never done by a model.

**Retention line:** a short sentence wherever stored data is listed. For guests: "Mode tamu · dihapus otomatis jika 7 hari tidak aktif, paling lambat 25 Nov 2026" (dates from `user.guest`, ADR-0002 §7).

### 6.10 State patterns: empty, loading, error, success, offline

| State | Pattern | Semantics |
|---|---|---|
| Empty | A heading-level sentence, one explanation sentence, and at most two actions. The mark at most; no illustration. Example: "Belum ada yang tersimpan. Yang kamu simpan akan muncul di sini, dari yang terbaru." [Mulai bercerita] [Tulis catatan] | Plain content |
| Loading | Static text "Memuat yang tersimpan…" (or the specific thing) after a 400ms delay (no flash on fast connections). Space is reserved so nothing jumps. **No shimmer skeletons and no spinners that turn** (motion, and noise for screen readers). After 8s: "Masih memuat… koneksimu mungkin lambat." plus [Coba lagi] | `#sr-status` |
| Error | Notice with `--c-danger-bg` tint, alert icon, heading "Belum bisa memuat yang tersimpan" / "Couldn't load what you've saved", then what is safe ("Tidak ada yang hilang."), then actions [Coba lagi] [Kembali ke beranda] | `#sr-alert` only when it blocks the task; otherwise `#sr-status` |
| Success | Inline status next to the action, kept until the next change: "Tersimpan · 20.14" / "Saved · 20:14". **No toasts that disappear** (2.2.1 and 4.1.3) | `#sr-status` |
| Partial or offline | Banner "Kamu sedang offline…" plus a label on cached views: "Ditampilkan dari saat terakhir dimuat, 20.14" | `#sr-status` once |
| Unavailable feature | Explained, never hidden: "Balasan AI sedang mati di server ini. Kamu tetap bisa menulis." | Plain content |

### 6.11 Banners

Order inside the notices region (highest first). At most three are expanded; more collapse into "2 pemberitahuan lain" / "2 more notices", which expands on demand.

| Banner | Look | Text (ID / EN) | Where and how long |
|---|---|---|---|
| Crisis follow-up (after crisis support was shown in this browser session) | `--c-help-bg` tint, life-buoy icon | "Bantuan tetap ada di sini kalau kamu butuh." [Lihat semua bantuan] / "Help is still here if you need it." [See all help] | **Only on Home and in the room**, not on every screen. Hidden when "Sembunyikan pratinjau isi" is on, because on a shared screen it tells others that something happened. Dismissible ("Tutup") until the tab closes |
| Offline | `--c-warning-bg`, wifi-off icon | "Kamu sedang offline. Tulisanmu tetap di layar." / "You're offline. Your writing stays on screen." | Every screen; goes away on reconnect |
| Policy updated (`actionsRequired: accept_policy:*`) | `--c-info-bg`, info icon | "Kebijakan privasi diperbarui. Lihat yang berubah." / "The privacy policy changed. See what's new." | Every screen until answered. Never blocks privacy actions |
| Guest expiry (from 7 days before the maximum age) | `--c-info-bg`, clock icon | "Mode tamu: datamu dihapus otomatis pada 25 Nov 2026. [Simpan ke akun]" | Dismissible; returns after 24h |
| Recovery codes not confirmed (`acknowledge_recovery_codes`) | `--c-warning-bg`, key icon | "Kode pemulihanmu belum dikonfirmasi tersimpan. [Lihat kode baru]" | Until confirmed |
| Demo (`environment: demo` / `synthetic: true`) | `--c-demo-bg`, 2px `--c-demo` border, "Demo" status chip | "Demo: semua data di sini sintetis. Fitur berjalan sungguhan." / "Demo: all data here is synthetic. Features really run." | Always |
| Test double (the generator **or** the verifier is a test double: AI mode `test_double`, or a configured test-double verifier, ADR-0003 §10.1; and on any page showing an item whose stored provenance names a test double) | `--c-test-bg`, 2px `--c-test` border, "Mode uji" chip | "Mode uji: balasan atau pemeriksaan di sini bisa dibuat oleh program uji, bukan model AI. Setiap hasilnya diberi label." / "Test mode: replies or checks here may come from a test program, not an AI model. Each one is labelled." | Always, while either condition holds |

- **Crisis support itself is not a banner.** It is the inline panel in the room (§6.15) and the help page.
- **Session warnings** are dialogs (§6.12), not banners, because they need an answer.

### 6.12 Dialogs, sheets and inline confirmation

- **Native `<dialog>` with `showModal()`.** This gives an inert background, Esc to close and focus kept inside. Focus goes to the dialog heading, or to the first safe action for destructive dialogs. On close, focus returns to the trigger.
- **Use a dialog only when the decision reaches beyond the current context:**
  - delete account or guest data;
  - sign out everywhere;
  - changing storage mid-conversation;
  - the two session dialogs below, and the unsaved-conversation hold warning (§6.9);
  - the menu sheet, and the room's Detail sheet on compact layouts.
- **Full-height sheets** (Menu, Detail) are inert-background dialogs too, so they repeat "Butuh bantuan sekarang" and "Keluar cepat" as their first two items, with the header's labels, look and order, followed by "Tutup" in the Menu button's place (§6.1). Leaving from an open sheet therefore takes one action, as everywhere else. **Small dialogs** keep a help link in their text, and Esc closes them; a distressed person is one Esc and one Quick exit away.
- **Single destructive actions in context** (delete one note, forget one memory, revoke one share) use the **inline confirm** pattern from v2 `ConfirmButton`: the Danger button expands into a group holding a sentence, [the action, Confirm-destructive variant] and [Batal]. Batal returns focus to the trigger.
- **Deleting or forgetting something that other records were built from** (a conversation, a note, a check-in, or a memory item that a summary includes) extends the inline confirm (brief §7: "Penghapusan data serta turunannya"). For a memory item the verbs are "Lupakan" ("Hal ini ada di 1 ringkasan (sedang dibagikan ke 'Psikolog Puskesmas')." [Lupakan dan keluarkan dari ringkasan] [Lupakan saja] [Batal]), and the same applies to "Lupakan semuanya" and to withdrawing the memory consent:
  1. Pressing "Hapus percakapan" first asks the server which derived items exist (planned P3/P4 endpoint; [§10](#10-open-points-for-the-pm-and-other-owners)). The button shows "Memeriksa…" meanwhile.
  2. **Nothing derived:** the plain confirm, "Hapus percakapan ini dan semua yang Sela tangkap darinya?" [Hapus percakapan] [Batal].
  3. **Derived items exist:** the confirm lists them in one sentence: "Kutipan dari percakapan ini ada di 2 ringkasan (1 sedang dibagikan ke 'Psikolog Puskesmas') dan 1 hal yang Sela ingat." Then three actions:
     - [Hapus percakapan dan semua turunannya] / [Delete the conversation and everything made from it]: the conversation goes; each affected summary gets a **new version without these quotes** (items whose only source was this conversation are removed, and the approval is cleared); each active share of an affected version is **revoked**, with the fixed revocation line ("Yang sudah dilihat … tidak bisa ditarik kembali"); memory items from this conversation are forgotten;
     - [Hapus percakapannya saja] / [Delete only the conversation]: the conversation goes; the quotes stay in the summaries and shares as copies; memory items stay, with the source "dari percakapan yang sudah dihapus";
     - [Batal].

     **Both destructive choices use the Confirm-destructive variant at the same size**, side by side when they fit and stacked full width otherwise, in this order, with Batal last. Neither is filled differently, preselected or focused first: focus moves to the confirm's sentence (`tabindex="-1"`), so the person reads the consequences before reaching a button. Each label carries its consequence (3.3.4).
  4. When previews are hidden, the recipient label in the sentence is replaced by "1 sedang dibagikan".
  5. Afterwards the status says what happened: "Percakapan dihapus. 2 ringkasan diperbarui, 1 berbagi dicabut, 1 hal dilupakan."
- **Focus after a row is removed** (a deleted note in a list, a forgotten memory item, a revoked session, a removed private name): focus moves to the first control of the next row, or of the previous row if it was the last, or to the list heading if the list is now empty. `#sr-status` says what happened ("Dilupakan.").
- **Button order:** in dialogs and inline confirms, the action named by the verb comes first and Batal / Cancel last, on every screen (3.2.4). The buttons wrap; on compact layouts they stack full width.
- **Idle warning** (accounts; ADR-0002 §8 idle time of 2 hours, or 7 days on a remembered device):
  - shown 2 minutes before idle expiry;
  - text: "Sesimu akan berakhir dalam sekitar 2 menit karena tidak ada aktivitas." / "Your session will end in about 2 minutes because of inactivity.";
  - actions: [Tetap masuk / Stay signed in] [Keluar dari akun / Sign out];
  - unsent drafts stay in the tab if the session ends;
  - meets 2.2.1 (a warning at least 20 seconds before, extendable with one action, any number of times).
- **Absolute expiry warning** (accounts; ADR-0002 §8: 12 hours after signing in, or 30 days on a remembered device; "Tetap masuk" cannot extend it):
  - shown 10 minutes before the absolute expiry, as a dialog over the current page;
  - text: "Sesimu berakhir pukul 21.40, 12 jam setelah kamu masuk. Untuk tetap masuk, masukkan kata sandimu. Halaman ini dan tulisanmu tetap di sini." / "Your session ends at 21:40, 12 hours after you signed in. To stay signed in, enter your password. This page and your writing stay here.";
  - a read-only username field ("Masuk sebagai pelangi.senja", `autocomplete="username"`) and a password field (`autocomplete="current-password"`, paste and password managers allowed, "Tampilkan" toggle, the uniform credential-error pattern of §6.3); [Masuk lagi / Sign in again] [Nanti saja / Not now];
  - success replaces the session without reloading the page; drafts and the open view stay. If the person waits, the session ends at the stated time and S27 "Sesimu sudah berakhir" keeps the drafts in the tab;
  - API: signing in again while still signed in (`POST /api/v3/auth/login`, or a dedicated renewal route) is a question for the API team (§10);
  - *rationale:* the absolute lifetime is a security control. Extending it without the password would defeat it, so the 12-hour limit is treated under the 2.2.1 essential exception, with the warning and loss-free re-authentication as mitigation (2.2.5, AAA, used as guidance). The 30-day remembered-device limit is beyond the 20-hour exception. Both are pending the accessibility auditor's review.
- **Quick exit:**
  - The button replaces the page with a neutral site (`location.replace`). Default `https://www.google.com/`, overridable by the operator; the user may choose from a short list in Display settings (planned).
  - Before leaving, it clears in-memory drafts and any unsaved conversation held in the tab.
  - **Signing out on Quick exit** (accounts): controlled by "Keluar cepat juga mengeluarkan akun" in Display settings. The default signs out sessions that were not remembered ("Ingat perangkat ini" off), which is the shared-device case; "Selalu" and "Tidak pernah" override it. When it applies, Quick exit sends `POST /api/v3/auth/logout` with `keepalive: true` first. **Guests are never signed out by Quick exit**, because the cookie is their only key (ADR-0002 §7).
  - **Shortcut: Esc twice** (on by default; can be turned off in S23). The rules prevent accidental exits:
    - only two Esc presses **in a row** count, with no other key in between, within 1.5 seconds;
    - `event.repeat` is ignored, so a held key or key repeat never fires it;
    - presses are ignored when the event is already handled (`defaultPrevented`), during IME composition (`isComposing`), inside an open dialog or menu (that Esc closes it), and **for 1.5 seconds after a dialog, menu, disclosure or autocomplete list closes**, so pressing Esc three times to close something never exits;
    - the first counted press **arms** Quick exit and shows a hint next to the Quick exit button, also sent to `#sr-status`: "Tekan Esc sekali lagi untuk keluar cepat." / "Press Esc once more for Quick exit." The hint and the armed state clear after 1.5 seconds or on any other key;
    - screen readers in browse or focus mode may take the first Esc themselves; the page then sees the next press as the first one and arms, which the hint makes audible.
  - Copy is honest about limits: "Ini tidak menghapus riwayat browser." / "This doesn't clear your browser history."

### 6.13 Sub-navigation, tabs and pagination

- **Route-level sections** ("Linimasa" | "Yang Sela ingat" inside Catatanku; the Support sections) are **links** in `<nav aria-label="Bagian Catatanku">`, styled as a segmented row with `aria-current="page"`. They are not ARIA tabs, because each one is its own route.
- **ARIA tabs** (`role="tablist"`) only for panels inside one view, such as the export format preview. Arrow keys move and Home and End jump.
- **Pagination:** a "Muat lebih banyak" / "Load more" button at the end of lists. No infinite scroll (slow connections, keyboard reach, footer reach). After loading, focus moves to the first new item, and the status says "20 entri lagi dimuat" / "20 more entries loaded". The loaded pages and the scroll position are kept when the person opens an entry and comes Back (§8).

### 6.14 Lists: timeline and selectable lists

- **Timeline:**
  - `<h2>` per day ("Hari ini", "Kemarin", "Sabtu, 24 Oktober");
  - under each, an `<ol>` of entry rows;
  - each row is **one link** with a type icon and type label ("Percakapan", "Catatan", "Cek perasaan", "Ringkasan", "Berbagi"), the time, a one-line preview and the provenance badge where relevant. With "Sembunyikan pratinjau isi" on, a row shows only its type and time: no preview, no conversation or summary title, and no recipient label;
  - no nested interactive elements;
  - each row opens its entry's own detail: a conversation in the room (S09), a note in S10, a check-in in the check-in detail (S11), a summary in S15, a share in S18.
- **Selectable list** (choosing what goes into a summary): `<fieldset>` holding checkbox rows, each with date, type, preview and badge. The whole row is the checkbox's label, so the target is the row (at least 44px). A sticky-in-flow summary line says "3 dipilih" / "3 selected" plus [Lanjut]. Select all is not offered: choosing is deliberate (brief §7).

### 6.15 Help: crisis panel, pause line, support line and service card

**Crisis panel (in the room, journal and check-in):**
- *Content:* **static text from the bundle and verified directory data, never model output** (D-036). It never quotes the words that matched.
- *Triggers in the room:* a lexicon match on the person's text (RR-1, before any model call); the generator's risk flag `crisis` (RR-7; that output is **not shown**); a verifier `safety_signal` (VC-8; the output is withheld). All three show the same panel and pause replies the same way.
- *Anatomy:*
  - `<section>` with `--c-help-bg` tint, a 2px `--c-help` top rule and an `<h3>`: "Kamu tidak sendirian" / "You're not alone";
  - two short sentences;
  - up to three service cards (below), then "Lihat semua bantuan" (a Help-variant link to `/help`);
  - the actions:
    - "Lanjutkan bercerita dengan Sela" / "Keep talking with Sela". Resumes AI replies in this conversation (D-036);
    - "Akhiri percakapan" / "End conversation". The same label and function as the room toolbar;
    - "Tutup bantuan ini" / "Close this help", for when it does not fit. No reason is asked.
- *Insertion and announcement:* the panel is inserted into the silent log right after the message (1.3.2). Only `#sr-alert` speaks, with one line: "Bantuan ditampilkan di bawah pesanmu." / "Help is shown below your message." **Nothing from the panel is announced**, and focus does not move. The panel is reachable by heading navigation and by Tab. **Visible cue:** if the reader is not at the bottom (scrolled up, or a magnifier shows another part of the page), the new-content button above the composer says "Bantuan ditampilkan di bawah pesanmu ↓"; pressing it scrolls to the panel and moves focus to its h3 (with the ring).
- *The pause line appears at once.* As soon as replies pause, the **pause line** appears in the room end bar, in the page flow directly above the composer (never inside the sticky layer), so whoever is at the composer sees that help was shown and that replies are paused, without reading the transcript. It is persistent (it cannot be dismissed) until replies resume or the conversation ends:
  - "Balasan Sela dijeda di percakapan ini. Kamu tetap bisa menulis." / "Sela's replies are paused in this conversation. You can still write."
  - while the panel is in the transcript: [Ke bantuan] / [Go to help] (a quiet button that scrolls to the panel and moves focus to its h3) and [Lanjutkan bercerita dengan Sela];
  - after the panel was closed: [Lanjutkan bercerita dengan Sela] [Lihat semua bantuan].

  From the text area, Shift+Tab therefore reaches the pause line first, not the panel's last button.
- *While replies are paused:* the person can keep writing. Each message is still screened, kept according to the storage choice, and carries the meta line "tanpa balasan (dijeda)" / "no reply (paused)". On send, `#sr-status` says "Terkirim." The pause belongs to the conversation: it is stored with a saved conversation and held in the tab for an unsaved one, and reopening a paused conversation shows the pause line.
- *After "Tutup bantuan ini":* the panel is removed and the pause line stays, with [Ke bantuan] replaced by [Lihat semua bantuan]. Focus moves to the text area and `#sr-status` says "Bantuan ditutup. Balasan Sela tetap dijeda."
- *A later match* (while paused, or after resuming): the panel appears again after that message, and replies pause again. If a full panel was already shown in this conversation within the person's last 5 messages, the **condensed form** appears instead:
  - heading "Bantuan tetap ada" / "Help is still here"; one line, "Layanan yang tadi ditampilkan bisa kamu hubungi kapan saja." / "You can contact the services shown earlier at any time.";
  - [Lihat semua bantuan] [Tampilkan layanan lagi] (expands the full panel in place) [Lanjutkan bercerita dengan Sela] [Tutup bantuan ini];
  - the same single `#sr-alert` line.

**Support line beside an `elevated` reply** (RR-7): the reply is released, and a static line follows it in the transcript: "Kalau terasa berat, bantuan bisa kamu hubungi kapan saja." [Lihat semua bantuan]. It is not a panel, does not pause anything and is announced only as part of the reply announcement (§6.5).

**Service card:**
- name (with `lang` if in the other language);
- what it is, in one line;
- channels as real links: `tel:`, `https://wa.me/…` and web, each labelled with the service name and channel ("Telepon Healing119");
- hours and cost, **only if verified**, otherwise "belum diverifikasi" / "not verified";
- source and verification: "Sumber: situs resmi · diverifikasi 15 Okt 2026 · periksa ulang sebelum 14 Jan 2027".

A stale entry (past its re-verify-by date) shows a warning chip: "Lewat tanggal periksa ulang — cek situs resminya" / "Past its re-check date — check the official site" (D-044).

**When no verified entry fits** (for example, teen help is requested but no teen entry is verified): the panel and S27 show the static S21 lead, "Kalau kamu dalam bahaya sekarang, hubungi layanan darurat setempat atau orang terdekat yang bisa menemanimu.", followed by the verified general entries. A list is never empty.

### 6.16 Secrets: recovery codes, share link and access code

- **Recovery codes:**
  - an `<ol>` of 10 codes in the mono stack, grouped `XXXX-XXXX-XXXX-XXXX`;
  - actions [Salin semua] [Unduh .txt] [Cetak];
  - required checkbox "Aku sudah menyimpan kode ini di tempat aman" / "I have saved these codes somewhere safe" before [Lanjut].
  - Shown once (API_SPEC_V3 §5).
- **Share link and access code:**
  - two copy fields (the v2 `CopyField` pattern: read-only input, Copy button, "Disalin" status);
  - the access code keeps the v2 format and tolerant entry (`internal/journey/service.go`, `randomCode` and `normalizeCode`): 8 characters shown as `XXXX-XXXX`, from an alphabet without look-alike characters (`23456789ABCDEFGHJKMNPQRSTVWXYZ`: no 0, O, 1, I, L or U), and entry that ignores case, spaces and hyphens. The P4 contract keeps both;
  - **the recipient's field checks the format before anything is sent** (S25): after the same normalisation, a code that is not 8 characters long, or that contains 0, 1, O, I, L or U, gets a field error and **uses up no attempt** ("Perlu diperbaiki: kode terdiri dari 8 huruf dan angka. Angka 0 dan 1, dan huruf O, I, L, dan U tidak dipakai." / "Needs fixing: the code has 8 letters and digits. It never uses 0, 1, O, I, L or U."). Only a well-formed code goes to the server, where 5 wrong codes lock the share (v2);
  - the advice puts a second **text** channel first, so the recipient can paste the code (3.3.8): "Kirim tautan dan kode lewat cara yang berbeda, misalnya tautan lewat WhatsApp dan kode lewat SMS atau aplikasi pesan lain. Kalau tidak ada cara lain, kode juga bisa dibacakan langsung." / "Send the link and the code in different ways, for example the link by WhatsApp and the code by SMS or another messaging app. If there's no other way, the code can also be read out in person.";
  - an acknowledgement "Aku sudah menyalin keduanya" / "I've copied both".
  - Whether they can be shown again depends on the P4 decision about stored secrets (the v2 gap analysis §5 recommends showing once). If shown once, losing them leads to "Buat tautan baru", which revokes the old one.

### 6.17 Check-in scale

- **Feeling:** a native radio group with legend "Bagaimana perasaanmu sekarang?" / "How are you feeling right now?". Five ordered options plus one outside the scale:
  - "Berat" / "Heavy";
  - "Kurang baik" / "Not great";
  - "Biasa saja" / "Okay";
  - "Cukup baik" / "Fairly good";
  - "Baik" / "Good";
  - "Campur aduk / tidak yakin" / "Mixed / not sure".
- **Look:** tiles of at least 44px with a label. Order is shown by a small row of one to five filled dots (not colour, not faces).
- **Energy:** optional, three options.
- **Tags:** optional checkbox chips.
- **Note:** optional text area.
- **No slider** (2.5.7), no emoji faces, no score.

### 6.18 Reading card

- title (link);
- a one-line description;
- topic chips (display only);
- reading time ("3 menit baca");
- source line ("Diadaptasi dari WHO · CC BY-NC-SA 3.0 IGO");
- a bookmark button "Simpan" (`aria-pressed`) for signed-in people.
- Items written in only one language carry `lang`.

### 6.19 Language switch

- A group of two `<button aria-pressed>`: "Bahasa Indonesia" (`lang="id"`) and "English" (`lang="en"`). Each label is in its own language, so someone who cannot read the current language can still find theirs.
- Placed on the Introduction (S01), in the menu, the sidebar and the recipient header.
- A change applies at once. Signed in, it is saved to the profile (`PATCH /api/v3/me`, `locale`); signed out, it is kept in memory and sent with guest start or registration. The status says "Bahasa diganti" / "Language changed".

---

## 7. Performance budget for small devices and slow connections

Design targets (pending verification by the frontend engineer with Lighthouse and throttled e2e runs in P7):

| Item | Budget |
|---|---|
| Initial JavaScript (gzip), app shell plus Home | ≤ 120 KB, including React 19 |
| Route chunks (summary builder, settings, recipient) | Loaded on demand (`React.lazy`), ≤ 40 KB each |
| CSS (gzip) | ≤ 25 KB |
| Fonts | 0 bytes (system fonts) |
| Images | None besides inline SVG |
| Immediate help | Rendered from bundled data with no extra request; the API refresh is a progressive enhancement (as v2 `help.fallback.json`) |
| Time to a usable composer on throttled "Slow 4G" (about 1.6 Mbps, 150ms RTT) | ≤ 3s on a mid-range Android |
| Same on "3G" (about 400 kbps, 400ms RTT) | ≤ 6s |
| Network failures | Every request has a timeout. Reads retry with backoff. Writes are retried only by the person, except idempotent ones. A dropped operation event stream resumes with `Last-Event-ID` (v2 SSE runtime); the reply itself arrives whole, so nothing is half-shown |
| Cold start of the AI (Modal scale to zero) | Shown as static text, "Sela sedang bersiap, bisa sampai sekitar 1 menit", using the server's estimate (`coldStartEstimateMs`), with Stop. No progress bar or looping animation (2.2.2) |
| Reply release latency | A reply appears when the whole reply has passed the guards (§6.5). Target: typical about 12 s warm, p95 at most 25 s (ADR-0003 §12 `latency.listening_reply_p95_warm_ms`). Measured in P5 on live Meta |

---

## 8. Implementation notes for the frontend (planned; owned by the frontend engineer)

- **Styles:**
  - `web/src/styles/tokens.css` (from this folder);
  - `base.css` (element defaults, focus, forced colours);
  - `layout.css` (shell, container queries, bar tiers);
  - one CSS file per component family.
  - Plain CSS, class names in BEM-like `block__part--state`. No CSS-in-JS. The current `web/src/styles.css` is replaced, not extended.
- **Components:** `web/src/components/ui/*` (Button, Field, ChoiceChips, Notice, Banner, Dialog, ConfirmInline, ConfirmDerived, ProvenanceBadge, ReviewStatus, GeneratorLine, StatusChip, ShareEvents, ConsentCard, StorageChoice, PrivateNames, CopyField, Icon) and `web/src/components/room/*` (Transcript, Message, PendingReply, Composer, ContactNotice, ReflectionCard, CrisisPanel, PauseLine, ClosingPanel). Reuse v2's i18n (`t`, `tx`, `dateTime`), the API client and `useOperation` (SSE). Under the post-release model (§6.5) `useOperation` carries progress events and one final event; it never renders partial text.
- **Shell scripts (small, dependency-free, planned):**
  - `Announcer`: renders `#sr-status`, `#sr-reply` and `#sr-alert` once at app start as the last children of `<body>`, and exposes `announce(region, segments)`, where each segment is `{text, lang?}`. It clears the region, then sets the content on the next animation frame (or after about 100 ms), rendering a segment whose `lang` differs from the interface as `<span lang>`. It also clears all three regions on every route change and on the person's next input (not sooner than 1 s after setting) (§6.1);
  - bar measurement: a `ResizeObserver` per sticky layer writes `--top-bar-h`, `--bottom-bar-h` and `--composer-h` (`0px` when hidden), and a `visualViewport` listener writes `--vvh`;
  - the bottom-bar tier check (§6.1) and the short-viewport decision that ignores the soft keyboard (§5.8), including the one-third rule for the help strip;
  - the room's typing layout (§6.5): on focus of a text-entry field in the room (shell below 64em), switch the header to the help strip, cap the text area and the composer from `--vvh`, and after the keyboard has opened set `data-typing="sticky"` or `"flow"` on `.room` from the reading budget; remove it on blur;
  - the Quick exit key handler with the arming rules (§6.12);
  - the unsaved-conversation hold timer with its warning dialog (§6.9), and the recipient session's idle warning (S25).
- **Router.** The v2 router (`web/src/lib/router.tsx`) matches whole paths only and scrolls to the top on every navigation. v3 needs three changes:
  1. **Route patterns with parameters**, matched in order:

     | Pattern | Screen | Parameters |
     |---|---|---|
     | `/`, `/about` | S01 | — |
     | `/start/:step` | S02, S04, S26 | `step` ∈ age, teen, guardian, storage, account, codes, ai |
     | `/signin`, `/recover` | S03, S05 | — |
     | `/home` | S07 | — |
     | `/talk`, `/talk/new`, `/talk/:id` | S08, S09 | `id` |
     | `/notes`, `/notes/patterns`, `/notes/memory`, `/notes/write`, `/notes/check-in`, `/notes/:id` | S12, S13, S10, S11; `/notes/:id` renders S10 for a note and the S11 check-in detail for a check-in (the entry id carries its type; an unknown or deleted id gives S27 Not found) | `id`; query `types`, `cursor` |
     | `/support`, `/support/summaries/new`, `/support/summaries/:id`, `/support/summaries/:id/edit/:step`, `/support/summaries/:id/prepare`, `/support/summaries/:id/share/:step`, `/support/summaries/:id/after`, `/support/shares/:id` | S14–S19 | `id`, `step` |
     | `/reading`, `/reading/:slug` | S20 | `slug`; query `topic` |
     | `/help` | S21 | fragment `#layanan` |
     | `/settings/privacy`, `/settings/display`, `/settings/account`, `/settings/account/upgrade` | S22–S24, S06 | fragment for the section (`#ai`, `#nama`) |
     | `/policies`, `/policies/:purpose` | S28 Policy pages | `purpose`; query `version` |
     | `/r` | S25 | fragment `#t=…` (scrubbed on load) |
     | `/research` | v2 reviewer page | — |
     | anything else | S27 Not found | — |
  2. **Scroll restoration.** `history.scrollRestoration = "manual"`. A push navigation scrolls to the top and focuses `<main>` (as now). On `popstate`, the router restores the scroll position saved in that entry's `history.state` after the view has rendered, and focuses `<main>` without scrolling (`focus({preventScroll: true})`).
  3. **Loaded pages are kept for Back.** Lists that load more (timeline, conversations, audit) keep their loaded pages and cursor in an in-memory cache keyed by the URL (path plus query) for the life of the tab. Filters live in the query string. On Back, the list renders the cached pages first, then the scroll position is restored, then it refreshes quietly. The cache is cleared by Quick exit, sign-out and deletion.
- **Boot script:** `web/public/boot.js` (§5.1). It must be tiny and dependency-free.
- **Titles:** each route sets `document.title` to "{Judul layar} – Sela" in the active language (2.4.2, a gap in v2). The Introduction is "Ruang sejenak untuk didengar – Sela" / "A little space to feel heard – Sela", so every title follows the same pattern. `<main>` is named by the page h1 (`aria-labelledby="page-title"`).
- **Tests to add** (frontend and e2e owners):
  - a unit test that parses `tokens.css` and runs the same pairs as `contrast.py` (or calls it);
  - axe with `wcag22aa`, failing on every impact, in all four schemes and in forced colours;
  - a 320px reflow check, including 320×256;
  - the 1.4.12 spacing stylesheet check, including the three bars;
  - the typing-layout checks at 360×300, 320×280 and with a 230px visual viewport (ST-11, S-14);
  - a component test that an item whose stored verification provider is a test double never renders "Claude" or "DeepSeek" anywhere in its provenance, and that one whose generator is a test double never renders "Meta".

  Details are in [ACCESSIBILITY_CRITERIA.md](ACCESSIBILITY_CRITERIA.md#6-automation-plan).

## 9. Why no UI framework

- The needed components are few and specific: the room, provenance, consent. Native HTML covers the hard parts: `<dialog>`, radios and checkboxes, `<details>`, `<select>`.
- A component library would add bytes (§7) and bring its own focus and live-region behaviour, which would have to be audited against the streaming pattern.
- Container queries, `:has()` and custom properties now do what a CSS framework would.
- v2 already shows the accessible baseline works without one.
- Tailwind (v1) is not carried over. Utility classes made v1's navigation hard to reason about, and the token roles here need semantic names.

## 10. Open points for the PM and other owners

Each item names the owner who must decide or build it. The design works without guessing in every case: where a contract is missing, the screen shows the honest fallback named here.

1. **"Don't save" conversations (P3 contract):** how long the server holds an unsaved conversation in memory after the last activity; that it keeps it across in-app navigation (Help, Privasi & data and back) and while the closing panel is open, and discards it only on Quick exit, tab close or leaving the site (a `sendBeacon` discard on `pagehide`), a new conversation, "Buang sekarang", or the hold timeout; that the timeout can be **extended by the person** ("Tetap buka" restarts it, any number of times) after a warning at least 20 seconds (proposed: 2 minutes) before it runs out (WCAG 2.2.1; §6.9); and what "discard" covers at each provider. The copy in §6.9 and S09 waits for it.
2. **Post-release replies (P3/P5 operation contract):** the operation event stream carries progress events and one final event (`released` with text, `withheld` with a reason code `no_consent | unavailable | not_supported | stale`, `crisis`, `no_ai`, `stopped`), and never unreleased text (§6.5). The AI engineer confirms the reason codes and measures release latency.
3. **Derived items before deletion (P3/P4):** an endpoint that returns, for a conversation, note or check-in, the summaries (with version and share state) and memory items built from it, and a delete that takes `derived: "delete" | "keep"` (§6.12). Proposal: `GET /api/v3/derived?source=<type>:<id>` and `DELETE …?derived=delete|keep`.
4. **Memory per conversation (P3):** `useMemory` on `POST` and `PATCH /api/v3/conversations`, a default in the profile, and a per-reply record of how many memory items were used (for "Tentang balasan ini").
5. **Names kept private (P3):** storage for the list (encrypted per person), its use in DM-2 redaction, and the per-share choice of names to show (P4). Automatic name detection is deferred (§6.9).
6. **Language of parts (P3/P4/P5):** a nullable BCP 47 `lang` on message, note, reflection-item, memory-item and proposal, opening-line and summary-item records, **set by the server** (for the person's text, from the same detection that picks the reply language; for AI output, the language it was asked to write in), carried into shares, the recipient projection, export and print, and delivered with the `released` event so the reply announcement can carry it (§6.1, §6.5). Owners: the AI engineer (detection) and the API team (fields).
7. **Conversations end after inactivity (P3):** proposal: a conversation with no activity for 12 hours ends automatically, so Home stops offering "Lanjutkan" and the list never shows "unfinished" (S07, S08).
8. **Share expiry options, secret display and the recipient session (P4):** propose 24 hours, 3 days, 7 days and 14 days, and **never an option under 20 hours** (v2's 1-hour option is dropped), so the share's own expiry falls under the 2.2.1 20-hour exception; show the link and code once. **The recipient session** (v2: `RecipientTTL` 2 hours absolute, `internal/journey/service.go`) is not carried over as is: proposal, an idle limit (2 hours) with a warning dialog at least 2 minutes before ("Tetap buka", any number of times) and no absolute limit shorter than the share itself; when it ends while the share may still be active, the page offers to re-enter the code with the link token still held in the tab (S25). This needs the P4 contract. The access code keeps the v2 alphabet and normalisation (§6.16).
9. **Who sets `consultation_in_progress` and `ended` (P4):** the proposal is the person for both, and the recipient's "Tidak dapat menerima" ends the share after an inline confirm. The recipient may also ask for clarification from the fixed reasons in S25 (no free text, so nothing looks like a professional note).
10. **Blocking 15-year-olds (D-031, decided).** The UI now asks "15 tahun" and "16–17 tahun" separately and blocks 15 in every environment, creating nothing (S02, S26). The open dependency is only the API: `ageBand` has `15_17` alone, so the server cannot enforce the block yet. Proposal: a hidden sub-band (`15` / `16_17`) or a separate `ageBand` value (API team).
11. **Guardian consent before anything is created (API team, legal B-10):** proposal: `POST /api/v3/auth/guest` and `/auth/register` accept the guardian attestation in the same request, so a declined consent creates nothing. Until then, teens may start **only as a guest**, the guardian step runs right after the guest is created, and a decline calls `DELETE /api/v3/me` (no reauthentication for guests) and says "Data yang baru dibuat sudah dihapus." (PP 17/2025 Pasal 9(4)–(5)).
12. **Signing in again before the absolute expiry (API team):** whether `POST /api/v3/auth/login` may be called with a live session to replace it without a reload, or a renewal route is added (§6.12).
13. **Display preference sync to the account:** needs a `/api/v3/me` preference field (planned).
14. **Quick-exit destinations list and the operator default:** owner decision.
15. **Neutral tab titles (P7):** an opt-in; the 2.4.2 note is in ACCESSIBILITY_CRITERIA.
16. The **document inventory** (`docs/DOCUMENT_INVENTORY.md`, owned by the documentation owner) lists the five design documents. `tokens.css` and `tools/` may also need entries.
17. **The class of Sela's replies (AI engineer and PM):** this design treats replies as `ai_summary` ("Tulisan AI"), the only AI class ADR-0003 §10 allows besides `unconfirmed_interpretation` (§6.7.1). It needs their confirmation; if replies get another treatment, only the reply row of §6.7.1 and "Tentang balasan ini" change, because replies show no badge in the transcript (§6.7).
18. **Recording answers to in-context questions (P3/P5, API team):** a consent record has only `granted` and `withdrawn`, so whether the person answered the checker card ("Tidak sekarang" in onboarding) or chose "Jangan tanya lagi" is a profile flag, planned with the "ask again" flags of S22. The withheld-reply reason codes must let the UI tell "never answered" from "not allowed" (§6.9, S09).
19. **The `ai_processing` purpose before P5:** no live provider is called without it (RR-2). Until the purpose is active, the room runs in the "AI off" state outside development and test, where the labelled test double may reply. The AI engineer and the API team confirm that the server enforces this (S02 step 3).
20. **The `unknown` age band (legal review B-10, PM):** this design gives `unknown` every minor default (teen copy, teen help first, no in-context memory ask, no DeepSeek, the confidentiality step) except the guardian step, because the API records guardian consent for `15_17` only (S26). Open: whether `unknown` may skip guardian consent at all, and whether `unknown` must be disabled wherever `15_17` is enabled for real users (it is enabled with it in development, test, demo and staging by default, ADR-0002 §3), since otherwise it is a way around both the guardian step and the block on 15-year-olds.
21. **Keeping memory paused after the consent is withdrawn (legal review B-10):** not offered unless the review confirms a basis for storing memory items without the `personal_memory` consent (§6.9). Until then withdrawal forgets everything.
22. **The help strip at extreme zoom (PM):** when the fixed strip would take more than a third of the viewport height (about 400% zoom with large text), it returns to the page flow as the first control on every page, except while typing in the room (§5.8). This is the design's reading of D-036's "always-visible" control at that size; the PM confirms or asks for the strip to stay fixed regardless.
23. **Help-quiet or solid Help in the top bar (owner testing, P8):** the top bar now uses Help-quiet (§6.2). Owner-test sessions compare both forms with participants of different ages, for findability under stress and for whether the solid form reads as alarm; the result is recorded in §11.
24. **Contact-detail patterns (P3, AI engineer):** the false-positive corpus of Indonesian and English times, money, dates and handles in prose (§6.5) is an acceptance test for the shared pattern set, and the server returns the detected kinds with the sent message so the notice can name them.
25. **Typing at 150% text on phones:** with the keyboard open, four lines of large transcript, a two-line field, Kirim and the help control do not fit on a 230–300px visual viewport; the room switches to the flow mode and the person reads by scrolling (§6.5). Owner testing with people who use large text checks whether this is acceptable or whether a different control (for example a one-line field that expands on demand) works better.

---

## 11. Revision log

**Round 1 (2026-10-26), after review.** What changed, in short:

- **Replies are released, not streamed** (§6.5): the browser receives a reply only after the server's guards, the one allowed regeneration and, when flagged, the verifier. Stop before release shows nothing of the reply. New room states for risk `crisis` and `elevated`, guard rejection with regeneration, and withheld replies that name the reason.
- **One announcement rule** (§6.1, §6.5): the transcript log is always `aria-live="off"`; three shell regions exist from the first paint; each event has one fixed line; the person's own message is not read back; the crisis panel and reflection card are announced with one line each.
- **Crisis pause made visible** (§6.15): a persistent pause line after "Tutup bantuan ini", the meta line on messages sent while paused, a condensed panel on repeat matches, and one label, "Lanjutkan bercerita dengan Sela", for resuming.
- **Labels:** "Akhiri percakapan" is visible in the toolbar and the crisis panel (2.5.3); "Keluar cepat" and "Butuh bantuan sekarang" are never shortened; "Keluar dari akun" is the only sign-out label (3.2.4).
- **Shell widths** (§6.1): the top bar wraps to two rows on phones, the bottom bar has three tiers, the room toolbar and the composer row wrap by available space, the short-viewport strip grows with text and ignores the soft keyboard (§5.8).
- **Provenance** (§6.7): class and review status separated; "Tafsiran Sela" and "Tulisan AI" labels; one record-type mapping (§6.7.1); a generator line on every AI-written item, including the recipient page and print.
- **Handoff** (§6.8): an event list built only from recorded events; the self-set step says "Kamu menandai".
- **Privacy controls added:** memory per conversation, names kept private, the contact-detail notice, and deletion that lists and optionally removes derived items (§6.9, §6.12).
- **Tokens and checks:** `--c-help-hover`; the Confirm-destructive (ink) variant so a red fill means help only; badges, status chips, meta-line actions and navigation labels at 16px; reduced-motion durations of 0.01ms; 500 contrast pairs including every tint.
- **Focus** (§5.7): only `main` hides its ring; text fields no longer change border width; `scroll-padding-bottom` includes the composer.

Also renamed for fit and accuracy: "Baca balasan terakhir" is now "Ke balasan terakhir" / "Go to the last reply", because the button moves focus rather than reading aloud, and the shorter label keeps the idle composer row on one line at 320px.

**Round 2 (2026-10-26), after review.** What changed, in short:

- **Provenance honesty:** summary items built from memory keep their class; a confirmed Sela interpretation is never a quote and reaches the recipient as "Tafsiran Sela · dikonfirmasi pengirim" (§6.7.1). Forgetting a memory item that a summary includes uses the derived-items confirm (§6.12). A test-double checker is "diperiksa oleh program uji (bukan AI)", derived from stored provenance, and the "Mode uji" banner shows whenever the generator or the verifier is a test double (§6.7, §6.11). Template opening lines that quote AI wording say so (§6.7).
- **Consent:** the Claude checker card is in onboarding step 3 below the Meta card; an unanswered checker question is asked in context at the first withheld reply; all "not allowed" copy is neutral ("Pemeriksa (Claude) belum diizinkan"). No live provider is called without `ai_processing` (S02, §10 item 19).
- **Typing on a phone** (§6.5): only the help control stays fixed while typing; the pause line, "Ke balasan terakhir" and notices left the sticky layer; text area and composer are capped from the visual viewport; a reading budget switches the composer into the page flow when it would leave less than three lines.
- **Visible cues for every insertion:** the new-content button names what arrived (help first), and the pause line appears as soon as replies pause, with "Ke bantuan" (§6.5, §6.15).
- **Contact details no longer block sending:** a non-blocking notice on the sent message; high-risk kinds are masked in storage by default (§6.5).
- **Shell:** Help-quiet in the top bar (§6.2); full-height sheets start with help and Quick exit (§6.1); on wide layouts the header spans the window so visual and DOM order match; the help strip has its own background and padding for the focus ring (§5.8); live regions sit at the end of `<body>`, carry `lang`, and are cleared on route change and next input (§6.1).
- **Focus and forms:** script-moved focus shows the ring on any focus; `<main>` is named by its h1 (§5.7); group errors live on the radiogroup, not on each radio; switches take their name from the label only; typed credentials are kept after a uniform error; every password prompt has a read-only username (§6.3).
- **Smaller fixes:** end-state chips say who ended the share (§6.8); both destructive choices of a derived-items confirm are equal (§6.12); memory controls state their scope and consent withdrawal forgets by default (§6.9); "Umumkan balasan ke pembaca layar" replaces "Bacakan balasan otomatis" (§5.9); the unsaved-conversation hold survives in-app navigation and warns before its timeout (§6.9).

**Considered and not changed in round 2:**

- *"Diperiksa oleh program uji (bukan Claude)"* (the review's example) became "(bukan AI)": a label that names Claude in order to deny it still puts "Claude" next to a check, and the component test can then assert that "Claude" never appears in a test-double report's provenance.
- *Merging "Jeda memori" into the default for new conversations* was not done: pausing stops memory in conversations that are already open, which the default cannot do. Each control now states its scope instead (§6.9).
- *Keeping a blocking contact-detail notice for NIK and card numbers* was not chosen: those are masked in the stored copy by default with an undo, so nothing in the listening flow waits for a decision (§6.5).

**Considered and not changed in round 1:**

- *Sentence-by-sentence release* (offered by the review as an option) was not chosen: VC-8 can withhold the whole reply after some sentences were shown, and an RR-5 regeneration would replace them (§6.5).
- *A per-item choice to show a private name in a share* became a **per-name** choice per share (S17): showing a name in one item and hiding it in another would still reveal it, and one choice per name is fewer decisions.
- *A filled danger button with a `--c-on-danger` label* was not added: destructive buttons are now never filled (§6.2), so the role would be unused. The ink-filled confirm replaces it.
- *"Lihat apa yang Sela tangkap" for unsaved conversations* was kept, not removed: while the closing panel is open the conversation still exists in memory, so the card can be made and is labelled "tidak disimpan" (S09).

---

## Appendix A: Contrast tables

Generated by `python3 docs/design/tools/contrast.py --md` from [tokens.css](tokens.css). Refresh with `python3 docs/design/tools/sync_docs.py` after any colour change. A failing pair makes both commands exit with status 1.

<!-- BEGIN GENERATED: tables -->
#### light: text

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-text` #1E1C19 | `--c-bg` #FAF7F2 | 15.91:1 | 4.5:1 | pass | Body text on the page |
| `--c-text` #1E1C19 | `--c-surface` #FFFFFF | 17.00:1 | 4.5:1 | pass | Body text on cards, dialogs and inputs |
| `--c-text` #1E1C19 | `--c-sunken` #F1ECE4 | 14.46:1 | 4.5:1 | pass | Text in the composer area and inset panels |
| `--c-text` #1E1C19 | `--c-accent-soft` #DDEBE8 | 13.86:1 | 4.5:1 | pass | Label of a selected choice chip |
| `--c-text` #1E1C19 | `--c-user-bg` #FBF0DF | 15.08:1 | 4.5:1 | pass | Text inside a 'Kata-katamu' (your words) block |
| `--c-text` #1E1C19 | `--c-ai-bg` #EFEEF9 | 14.79:1 | 4.5:1 | pass | Text inside an AI summary or 'Sela menangkap…' block |
| `--c-text` #1E1C19 | `--c-help-bg` #FBE9E4 | 14.47:1 | 4.5:1 | pass | Text in the crisis support panel |
| `--c-text` #1E1C19 | `--c-danger-bg` #FCE9E7 | 14.53:1 | 4.5:1 | pass | Text in an error notice |
| `--c-text` #1E1C19 | `--c-success-bg` #E6F2E9 | 14.77:1 | 4.5:1 | pass | Text in a success notice |
| `--c-text` #1E1C19 | `--c-warning-bg` #FCF1D9 | 15.16:1 | 4.5:1 | pass | Text in a warning notice |
| `--c-text` #1E1C19 | `--c-info-bg` #E7F0F8 | 14.75:1 | 4.5:1 | pass | Text in an info notice and the guest banner |
| `--c-text` #1E1C19 | `--c-demo-bg` #EEE9F8 | 14.28:1 | 4.5:1 | pass | Text in the demo banner |
| `--c-text` #1E1C19 | `--c-test-bg` #FFF1C7 | 15.10:1 | 4.5:1 | pass | Text in the test-double banner |
| `--c-muted` #575249 | `--c-bg` #FAF7F2 | 7.26:1 | 4.5:1 | pass | Secondary text and timestamps on the page |
| `--c-muted` #575249 | `--c-surface` #FFFFFF | 7.75:1 | 4.5:1 | pass | Hints, meta lines and placeholder text on cards and inputs |
| `--c-muted` #575249 | `--c-sunken` #F1ECE4 | 6.59:1 | 4.5:1 | pass | Hints in the composer area |
| `--c-muted` #575249 | `--c-user-bg` #FBF0DF | 6.88:1 | 4.5:1 | pass | Meta line inside a 'your words' block |
| `--c-muted` #575249 | `--c-ai-bg` #EFEEF9 | 6.74:1 | 4.5:1 | pass | Meta line inside an AI block |
| `--c-link` #1B5A55 | `--c-bg` #FAF7F2 | 7.44:1 | 4.5:1 | pass | Links (always underlined) on the page |
| `--c-link` #1B5A55 | `--c-surface` #FFFFFF | 7.95:1 | 4.5:1 | pass | Links on cards |
| `--c-link` #1B5A55 | `--c-sunken` #F1ECE4 | 6.76:1 | 4.5:1 | pass | Links in the composer area |
| `--c-on-accent` #FFFFFF | `--c-accent` #1D5C57 | 7.72:1 | 4.5:1 | pass | Primary button label |
| `--c-on-accent` #FFFFFF | `--c-accent-hover` #154743 | 10.43:1 | 4.5:1 | pass | Primary button label, hover and pressed |
| `--c-accent` #1D5C57 | `--c-surface` #FFFFFF | 7.72:1 | 4.5:1 | pass | Secondary button label (outlined) |
| `--c-accent` #1D5C57 | `--c-bg` #FAF7F2 | 7.22:1 | 4.5:1 | pass | Secondary button label on the page |
| `--c-on-help` #FFFFFF | `--c-help` #A1351E | 6.90:1 | 4.5:1 | pass | 'Butuh bantuan sekarang' and 'Lihat semua bantuan' link label (Help variant) |
| `--c-on-help` #FFFFFF | `--c-help-hover` #852A16 | 8.97:1 | 4.5:1 | pass | Help-variant label, hover and pressed |
| `--c-surface` #FFFFFF | `--c-text` #1E1C19 | 17.00:1 | 4.5:1 | pass | Label of the ink-filled destructive confirm button (the confirm step of delete, forget, revoke) |
| `--c-surface` #FFFFFF | `--c-muted` #575249 | 7.75:1 | 4.5:1 | pass | Same label, pressed state (fill --c-muted) |
| `--c-user-text` #6E4100 | `--c-user-bg` #FBF0DF | 7.70:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on its tint |
| `--c-user-text` #6E4100 | `--c-surface` #FFFFFF | 8.68:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on a card |
| `--c-ai-text` #3E378F | `--c-ai-bg` #EFEEF9 | 8.50:1 | 4.5:1 | pass | 'Tulisan AI' / 'Tafsiran Sela' label on its tint |
| `--c-ai-text` #3E378F | `--c-surface` #FFFFFF | 9.78:1 | 4.5:1 | pass | AI provenance label on a card |
| `--c-help-text` #8C2C17 | `--c-help-bg` #FBE9E4 | 7.19:1 | 4.5:1 | pass | Crisis panel heading |
| `--c-help-text` #8C2C17 | `--c-surface` #FFFFFF | 8.44:1 | 4.5:1 | pass | Help link text on cards |
| `--c-help-text` #8C2C17 | `--c-bg` #FAF7F2 | 7.90:1 | 4.5:1 | pass | Help link text on the page |
| `--c-danger` #A3271F | `--c-surface` #FFFFFF | 7.32:1 | 4.5:1 | pass | Destructive button label (outlined) and field error text on cards |
| `--c-danger` #A3271F | `--c-bg` #FAF7F2 | 6.85:1 | 4.5:1 | pass | Field error text on the page |
| `--c-danger` #A3271F | `--c-danger-bg` #FCE9E7 | 6.26:1 | 4.5:1 | pass | Error notice heading |
| `--c-success` #23683C | `--c-surface` #FFFFFF | 6.74:1 | 4.5:1 | pass | Success status text on a card |
| `--c-success` #23683C | `--c-success-bg` #E6F2E9 | 5.85:1 | 4.5:1 | pass | Success notice heading |
| `--c-warning` #875200 | `--c-surface` #FFFFFF | 6.49:1 | 4.5:1 | pass | Warning status text on a card |
| `--c-warning` #875200 | `--c-warning-bg` #FCF1D9 | 5.78:1 | 4.5:1 | pass | Warning notice heading |
| `--c-info` #1F5788 | `--c-surface` #FFFFFF | 7.56:1 | 4.5:1 | pass | Info status text on a card |
| `--c-info` #1F5788 | `--c-info-bg` #E7F0F8 | 6.56:1 | 4.5:1 | pass | Info notice heading |
| `--c-demo` #5E4A99 | `--c-demo-bg` #EEE9F8 | 6.05:1 | 4.5:1 | pass | Demo banner label |
| `--c-test` #7A5600 | `--c-test-bg` #FFF1C7 | 5.91:1 | 4.5:1 | pass | Test-double banner label |
| `--c-muted` #575249 | `--c-help-bg` #FBE9E4 | 6.60:1 | 4.5:1 | pass | Service-card source and verification line inside the crisis panel |
| `--c-help-text` #8C2C17 | `--c-help-bg` #FBE9E4 | 7.19:1 | 4.5:1 | pass | Help-quiet label: 'Butuh bantuan sekarang' in the top bar, the help strip and full-height sheets |
| `--c-link` #1B5A55 | `--c-ai-bg` #EFEEF9 | 6.92:1 | 4.5:1 | pass | Link text inside a block tinted --c-ai-bg |
| `--c-link` #1B5A55 | `--c-help-bg` #FBE9E4 | 6.77:1 | 4.5:1 | pass | Link text inside a block tinted --c-help-bg |
| `--c-link` #1B5A55 | `--c-user-bg` #FBF0DF | 7.06:1 | 4.5:1 | pass | Link text inside a block tinted --c-user-bg |
| `--c-link` #1B5A55 | `--c-danger-bg` #FCE9E7 | 6.80:1 | 4.5:1 | pass | Link text inside a block tinted --c-danger-bg |
| `--c-link` #1B5A55 | `--c-success-bg` #E6F2E9 | 6.91:1 | 4.5:1 | pass | Link text inside a block tinted --c-success-bg |
| `--c-link` #1B5A55 | `--c-warning-bg` #FCF1D9 | 7.09:1 | 4.5:1 | pass | Link text inside a block tinted --c-warning-bg |
| `--c-link` #1B5A55 | `--c-info-bg` #E7F0F8 | 6.90:1 | 4.5:1 | pass | Link text inside a block tinted --c-info-bg |
| `--c-link` #1B5A55 | `--c-demo-bg` #EEE9F8 | 6.68:1 | 4.5:1 | pass | Link text inside a block tinted --c-demo-bg |
| `--c-link` #1B5A55 | `--c-test-bg` #FFF1C7 | 7.07:1 | 4.5:1 | pass | Link text inside a block tinted --c-test-bg |
| `--c-accent` #1D5C57 | `--c-ai-bg` #EFEEF9 | 6.71:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-ai-bg |
| `--c-accent` #1D5C57 | `--c-help-bg` #FBE9E4 | 6.57:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-help-bg |
| `--c-accent` #1D5C57 | `--c-user-bg` #FBF0DF | 6.85:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-user-bg |
| `--c-accent` #1D5C57 | `--c-danger-bg` #FCE9E7 | 6.60:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-danger-bg |
| `--c-accent` #1D5C57 | `--c-success-bg` #E6F2E9 | 6.70:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-success-bg |
| `--c-accent` #1D5C57 | `--c-warning-bg` #FCF1D9 | 6.88:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-warning-bg |
| `--c-accent` #1D5C57 | `--c-info-bg` #E7F0F8 | 6.70:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-info-bg |
| `--c-accent` #1D5C57 | `--c-demo-bg` #EEE9F8 | 6.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-demo-bg |
| `--c-accent` #1D5C57 | `--c-test-bg` #FFF1C7 | 6.86:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-test-bg |

#### light: UI components and graphics

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-border-control` #7B7368 | `--c-bg` #FAF7F2 | 4.37:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on the page |
| `--c-border-control` #7B7368 | `--c-surface` #FFFFFF | 4.67:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on a card |
| `--c-border-control` #7B7368 | `--c-sunken` #F1ECE4 | 3.97:1 | 3:1 | pass | Composer text area boundary |
| `--c-accent` #1D5C57 | `--c-bg` #FAF7F2 | 7.22:1 | 3:1 | pass | Primary button shape, checked checkbox and radio fill, selected-chip border |
| `--c-accent` #1D5C57 | `--c-surface` #FFFFFF | 7.72:1 | 3:1 | pass | Same, on cards |
| `--c-accent` #1D5C57 | `--c-accent-soft` #DDEBE8 | 6.29:1 | 3:1 | pass | Selected-chip border against its own fill |
| `--c-focus` #2747C9 | `--c-bg` #FAF7F2 | 6.94:1 | 3:1 | pass | Focus ring against the page (2.4.7; 2.4.13 change of contrast) |
| `--c-focus` #2747C9 | `--c-surface` #FFFFFF | 7.42:1 | 3:1 | pass | Focus ring against cards and dialogs |
| `--c-focus` #2747C9 | `--c-sunken` #F1ECE4 | 6.31:1 | 3:1 | pass | Focus ring in the composer area |
| `--c-user` #9A5A00 | `--c-bg` #FAF7F2 | 5.12:1 | 3:1 | pass | 'Your words' rule (4px) on the page |
| `--c-user` #9A5A00 | `--c-surface` #FFFFFF | 5.47:1 | 3:1 | pass | 'Your words' rule on a card |
| `--c-user` #9A5A00 | `--c-user-bg` #FBF0DF | 4.86:1 | 3:1 | pass | 'Your words' rule against its tint |
| `--c-ai` #5048A8 | `--c-bg` #FAF7F2 | 6.94:1 | 3:1 | pass | AI dashed or dotted outline on the page |
| `--c-ai` #5048A8 | `--c-surface` #FFFFFF | 7.41:1 | 3:1 | pass | AI outline on a card |
| `--c-ai` #5048A8 | `--c-ai-bg` #EFEEF9 | 6.45:1 | 3:1 | pass | AI outline against its tint |
| `--c-help` #A1351E | `--c-bg` #FAF7F2 | 6.46:1 | 3:1 | pass | Help button shape on the page |
| `--c-help` #A1351E | `--c-surface` #FFFFFF | 6.90:1 | 3:1 | pass | Help button shape on a card |
| `--c-danger` #A3271F | `--c-bg` #FAF7F2 | 6.85:1 | 3:1 | pass | Destructive button border and invalid-field border on the page |
| `--c-danger` #A3271F | `--c-surface` #FFFFFF | 7.32:1 | 3:1 | pass | Invalid-field border on a card |
| `--c-warning-border` #A86E00 | `--c-bg` #FAF7F2 | 4.01:1 | 3:1 | pass | Warning notice and status chip border |
| `--c-warning-border` #A86E00 | `--c-surface` #FFFFFF | 4.29:1 | 3:1 | pass | Warning chip border on a card |
| `--c-success` #23683C | `--c-bg` #FAF7F2 | 6.30:1 | 3:1 | pass | Success chip border |
| `--c-info` #1F5788 | `--c-bg` #FAF7F2 | 7.08:1 | 3:1 | pass | Info chip border |
| `--c-demo` #5E4A99 | `--c-bg` #FAF7F2 | 6.74:1 | 3:1 | pass | Demo banner border |
| `--c-test` #7A5600 | `--c-bg` #FAF7F2 | 6.22:1 | 3:1 | pass | Test-double banner border |
| `--c-text` #1E1C19 | `--c-bg` #FAF7F2 | 15.91:1 | 3:1 | pass | Icons that carry meaning (drawn in the text colour) |
| `--c-muted` #575249 | `--c-bg` #FAF7F2 | 7.26:1 | 3:1 | pass | Secondary icons that carry meaning (decorative ones are exempt) |
| `--c-help-hover` #852A16 | `--c-bg` #FAF7F2 | 8.40:1 | 3:1 | pass | Help-variant shape, hover and pressed, on the page |
| `--c-help-hover` #852A16 | `--c-surface` #FFFFFF | 8.97:1 | 3:1 | pass | Help-variant shape, hover and pressed, on a card |
| `--c-help` #A1351E | `--c-help-bg` #FBE9E4 | 5.87:1 | 3:1 | pass | Help-variant link shape inside the crisis panel |
| `--c-text` #1E1C19 | `--c-surface` #FFFFFF | 17.00:1 | 3:1 | pass | Ink-filled destructive confirm button shape on a card |
| `--c-warning-border` #A86E00 | `--c-warning-bg` #FCF1D9 | 3.82:1 | 3:1 | pass | Warning notice border against its own tint |
| `--c-help-text` #8C2C17 | `--c-help-bg` #FBE9E4 | 7.19:1 | 3:1 | pass | Help-quiet life-buoy icon on its own fill |
| `--c-help-hover` #852A16 | `--c-help-bg` #FBE9E4 | 7.64:1 | 3:1 | pass | Help-quiet border, hover and pressed (3px), against its own fill |
| `--c-focus` #2747C9 | `--c-ai-bg` #EFEEF9 | 6.45:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-ai-bg |
| `--c-focus` #2747C9 | `--c-help-bg` #FBE9E4 | 6.32:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-help-bg |
| `--c-focus` #2747C9 | `--c-user-bg` #FBF0DF | 6.58:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-user-bg |
| `--c-focus` #2747C9 | `--c-danger-bg` #FCE9E7 | 6.34:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-danger-bg |
| `--c-focus` #2747C9 | `--c-success-bg` #E6F2E9 | 6.44:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-success-bg |
| `--c-focus` #2747C9 | `--c-warning-bg` #FCF1D9 | 6.61:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-warning-bg |
| `--c-focus` #2747C9 | `--c-info-bg` #E7F0F8 | 6.43:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-info-bg |
| `--c-focus` #2747C9 | `--c-demo-bg` #EEE9F8 | 6.23:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-demo-bg |
| `--c-focus` #2747C9 | `--c-test-bg` #FFF1C7 | 6.59:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-test-bg |
| `--c-accent` #1D5C57 | `--c-ai-bg` #EFEEF9 | 6.71:1 | 3:1 | pass | Secondary button border inside a block tinted --c-ai-bg |
| `--c-accent` #1D5C57 | `--c-help-bg` #FBE9E4 | 6.57:1 | 3:1 | pass | Secondary button border inside a block tinted --c-help-bg |
| `--c-accent` #1D5C57 | `--c-user-bg` #FBF0DF | 6.85:1 | 3:1 | pass | Secondary button border inside a block tinted --c-user-bg |
| `--c-accent` #1D5C57 | `--c-danger-bg` #FCE9E7 | 6.60:1 | 3:1 | pass | Secondary button border inside a block tinted --c-danger-bg |
| `--c-accent` #1D5C57 | `--c-success-bg` #E6F2E9 | 6.70:1 | 3:1 | pass | Secondary button border inside a block tinted --c-success-bg |
| `--c-accent` #1D5C57 | `--c-warning-bg` #FCF1D9 | 6.88:1 | 3:1 | pass | Secondary button border inside a block tinted --c-warning-bg |
| `--c-accent` #1D5C57 | `--c-info-bg` #E7F0F8 | 6.70:1 | 3:1 | pass | Secondary button border inside a block tinted --c-info-bg |
| `--c-accent` #1D5C57 | `--c-demo-bg` #EEE9F8 | 6.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-demo-bg |
| `--c-accent` #1D5C57 | `--c-test-bg` #FFF1C7 | 6.86:1 | 3:1 | pass | Secondary button border inside a block tinted --c-test-bg |
| `--c-border-control` #7B7368 | `--c-ai-bg` #EFEEF9 | 4.06:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-ai-bg |
| `--c-border-control` #7B7368 | `--c-help-bg` #FBE9E4 | 3.98:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-help-bg |
| `--c-border-control` #7B7368 | `--c-user-bg` #FBF0DF | 4.15:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-user-bg |
| `--c-border-control` #7B7368 | `--c-danger-bg` #FCE9E7 | 3.99:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-danger-bg |
| `--c-border-control` #7B7368 | `--c-success-bg` #E6F2E9 | 4.06:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-success-bg |
| `--c-border-control` #7B7368 | `--c-warning-bg` #FCF1D9 | 4.17:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-warning-bg |
| `--c-border-control` #7B7368 | `--c-info-bg` #E7F0F8 | 4.05:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-info-bg |
| `--c-border-control` #7B7368 | `--c-demo-bg` #EEE9F8 | 3.93:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-demo-bg |
| `--c-border-control` #7B7368 | `--c-test-bg` #FFF1C7 | 4.15:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-test-bg |

#### dark: text

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-text` #F2EEE7 | `--c-bg` #141311 | 16.06:1 | 4.5:1 | pass | Body text on the page |
| `--c-text` #F2EEE7 | `--c-surface` #1D1B18 | 14.86:1 | 4.5:1 | pass | Body text on cards, dialogs and inputs |
| `--c-text` #F2EEE7 | `--c-sunken` #25221E | 13.69:1 | 4.5:1 | pass | Text in the composer area and inset panels |
| `--c-text` #F2EEE7 | `--c-accent-soft` #1F3532 | 11.25:1 | 4.5:1 | pass | Label of a selected choice chip |
| `--c-text` #F2EEE7 | `--c-user-bg` #2C2317 | 13.35:1 | 4.5:1 | pass | Text inside a 'Kata-katamu' (your words) block |
| `--c-text` #F2EEE7 | `--c-ai-bg` #232139 | 13.47:1 | 4.5:1 | pass | Text inside an AI summary or 'Sela menangkap…' block |
| `--c-text` #F2EEE7 | `--c-help-bg` #36211B | 13.06:1 | 4.5:1 | pass | Text in the crisis support panel |
| `--c-text` #F2EEE7 | `--c-danger-bg` #3A1C19 | 13.35:1 | 4.5:1 | pass | Text in an error notice |
| `--c-text` #F2EEE7 | `--c-success-bg` #18301F | 12.26:1 | 4.5:1 | pass | Text in a success notice |
| `--c-text` #F2EEE7 | `--c-warning-bg` #352A12 | 12.18:1 | 4.5:1 | pass | Text in a warning notice |
| `--c-text` #F2EEE7 | `--c-info-bg` #172A3B | 12.69:1 | 4.5:1 | pass | Text in an info notice and the guest banner |
| `--c-text` #F2EEE7 | `--c-demo-bg` #2A2340 | 12.85:1 | 4.5:1 | pass | Text in the demo banner |
| `--c-text` #F2EEE7 | `--c-test-bg` #352B0E | 12.08:1 | 4.5:1 | pass | Text in the test-double banner |
| `--c-muted` #BCB4A7 | `--c-bg` #141311 | 9.04:1 | 4.5:1 | pass | Secondary text and timestamps on the page |
| `--c-muted` #BCB4A7 | `--c-surface` #1D1B18 | 8.37:1 | 4.5:1 | pass | Hints, meta lines and placeholder text on cards and inputs |
| `--c-muted` #BCB4A7 | `--c-sunken` #25221E | 7.71:1 | 4.5:1 | pass | Hints in the composer area |
| `--c-muted` #BCB4A7 | `--c-user-bg` #2C2317 | 7.52:1 | 4.5:1 | pass | Meta line inside a 'your words' block |
| `--c-muted` #BCB4A7 | `--c-ai-bg` #232139 | 7.58:1 | 4.5:1 | pass | Meta line inside an AI block |
| `--c-link` #8ED2C8 | `--c-bg` #141311 | 10.79:1 | 4.5:1 | pass | Links (always underlined) on the page |
| `--c-link` #8ED2C8 | `--c-surface` #1D1B18 | 9.98:1 | 4.5:1 | pass | Links on cards |
| `--c-link` #8ED2C8 | `--c-sunken` #25221E | 9.20:1 | 4.5:1 | pass | Links in the composer area |
| `--c-on-accent` #0C1D1B | `--c-accent` #7FC6BC | 8.87:1 | 4.5:1 | pass | Primary button label |
| `--c-on-accent` #0C1D1B | `--c-accent-hover` #9BD5CD | 10.60:1 | 4.5:1 | pass | Primary button label, hover and pressed |
| `--c-accent` #7FC6BC | `--c-surface` #1D1B18 | 8.76:1 | 4.5:1 | pass | Secondary button label (outlined) |
| `--c-accent` #7FC6BC | `--c-bg` #141311 | 9.47:1 | 4.5:1 | pass | Secondary button label on the page |
| `--c-on-help` #2B0E07 | `--c-help` #F2917A | 7.79:1 | 4.5:1 | pass | 'Butuh bantuan sekarang' and 'Lihat semua bantuan' link label (Help variant) |
| `--c-on-help` #2B0E07 | `--c-help-hover` #F7AE9C | 9.83:1 | 4.5:1 | pass | Help-variant label, hover and pressed |
| `--c-surface` #1D1B18 | `--c-text` #F2EEE7 | 14.86:1 | 4.5:1 | pass | Label of the ink-filled destructive confirm button (the confirm step of delete, forget, revoke) |
| `--c-surface` #1D1B18 | `--c-muted` #BCB4A7 | 8.37:1 | 4.5:1 | pass | Same label, pressed state (fill --c-muted) |
| `--c-user-text` #F0C27E | `--c-user-bg` #2C2317 | 9.36:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on its tint |
| `--c-user-text` #F0C27E | `--c-surface` #1D1B18 | 10.41:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on a card |
| `--c-ai-text` #C4C0FA | `--c-ai-bg` #232139 | 9.10:1 | 4.5:1 | pass | 'Tulisan AI' / 'Tafsiran Sela' label on its tint |
| `--c-ai-text` #C4C0FA | `--c-surface` #1D1B18 | 10.04:1 | 4.5:1 | pass | AI provenance label on a card |
| `--c-help-text` #F6AE9C | `--c-help-bg` #36211B | 8.24:1 | 4.5:1 | pass | Crisis panel heading |
| `--c-help-text` #F6AE9C | `--c-surface` #1D1B18 | 9.37:1 | 4.5:1 | pass | Help link text on cards |
| `--c-help-text` #F6AE9C | `--c-bg` #141311 | 10.13:1 | 4.5:1 | pass | Help link text on the page |
| `--c-danger` #F38E85 | `--c-surface` #1D1B18 | 7.38:1 | 4.5:1 | pass | Destructive button label (outlined) and field error text on cards |
| `--c-danger` #F38E85 | `--c-bg` #141311 | 7.97:1 | 4.5:1 | pass | Field error text on the page |
| `--c-danger` #F38E85 | `--c-danger-bg` #3A1C19 | 6.63:1 | 4.5:1 | pass | Error notice heading |
| `--c-success` #83CC9B | `--c-surface` #1D1B18 | 9.06:1 | 4.5:1 | pass | Success status text on a card |
| `--c-success` #83CC9B | `--c-success-bg` #18301F | 7.48:1 | 4.5:1 | pass | Success notice heading |
| `--c-warning` #F1C46F | `--c-surface` #1D1B18 | 10.53:1 | 4.5:1 | pass | Warning status text on a card |
| `--c-warning` #F1C46F | `--c-warning-bg` #352A12 | 8.63:1 | 4.5:1 | pass | Warning notice heading |
| `--c-info` #90C3F0 | `--c-surface` #1D1B18 | 9.20:1 | 4.5:1 | pass | Info status text on a card |
| `--c-info` #90C3F0 | `--c-info-bg` #172A3B | 7.86:1 | 4.5:1 | pass | Info notice heading |
| `--c-demo` #C6B6F2 | `--c-demo-bg` #2A2340 | 8.05:1 | 4.5:1 | pass | Demo banner label |
| `--c-test` #F2C94C | `--c-test-bg` #352B0E | 8.80:1 | 4.5:1 | pass | Test-double banner label |
| `--c-muted` #BCB4A7 | `--c-help-bg` #36211B | 7.35:1 | 4.5:1 | pass | Service-card source and verification line inside the crisis panel |
| `--c-help-text` #F6AE9C | `--c-help-bg` #36211B | 8.24:1 | 4.5:1 | pass | Help-quiet label: 'Butuh bantuan sekarang' in the top bar, the help strip and full-height sheets |
| `--c-link` #8ED2C8 | `--c-ai-bg` #232139 | 9.05:1 | 4.5:1 | pass | Link text inside a block tinted --c-ai-bg |
| `--c-link` #8ED2C8 | `--c-help-bg` #36211B | 8.78:1 | 4.5:1 | pass | Link text inside a block tinted --c-help-bg |
| `--c-link` #8ED2C8 | `--c-user-bg` #2C2317 | 8.97:1 | 4.5:1 | pass | Link text inside a block tinted --c-user-bg |
| `--c-link` #8ED2C8 | `--c-danger-bg` #3A1C19 | 8.97:1 | 4.5:1 | pass | Link text inside a block tinted --c-danger-bg |
| `--c-link` #8ED2C8 | `--c-success-bg` #18301F | 8.24:1 | 4.5:1 | pass | Link text inside a block tinted --c-success-bg |
| `--c-link` #8ED2C8 | `--c-warning-bg` #352A12 | 8.18:1 | 4.5:1 | pass | Link text inside a block tinted --c-warning-bg |
| `--c-link` #8ED2C8 | `--c-info-bg` #172A3B | 8.53:1 | 4.5:1 | pass | Link text inside a block tinted --c-info-bg |
| `--c-link` #8ED2C8 | `--c-demo-bg` #2A2340 | 8.64:1 | 4.5:1 | pass | Link text inside a block tinted --c-demo-bg |
| `--c-link` #8ED2C8 | `--c-test-bg` #352B0E | 8.12:1 | 4.5:1 | pass | Link text inside a block tinted --c-test-bg |
| `--c-accent` #7FC6BC | `--c-ai-bg` #232139 | 7.94:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-ai-bg |
| `--c-accent` #7FC6BC | `--c-help-bg` #36211B | 7.70:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-help-bg |
| `--c-accent` #7FC6BC | `--c-user-bg` #2C2317 | 7.87:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-user-bg |
| `--c-accent` #7FC6BC | `--c-danger-bg` #3A1C19 | 7.87:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-danger-bg |
| `--c-accent` #7FC6BC | `--c-success-bg` #18301F | 7.23:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-success-bg |
| `--c-accent` #7FC6BC | `--c-warning-bg` #352A12 | 7.18:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-warning-bg |
| `--c-accent` #7FC6BC | `--c-info-bg` #172A3B | 7.48:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-info-bg |
| `--c-accent` #7FC6BC | `--c-demo-bg` #2A2340 | 7.58:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-demo-bg |
| `--c-accent` #7FC6BC | `--c-test-bg` #352B0E | 7.12:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-test-bg |

#### dark: UI components and graphics

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-border-control` #8E8679 | `--c-bg` #141311 | 5.16:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on the page |
| `--c-border-control` #8E8679 | `--c-surface` #1D1B18 | 4.78:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on a card |
| `--c-border-control` #8E8679 | `--c-sunken` #25221E | 4.40:1 | 3:1 | pass | Composer text area boundary |
| `--c-accent` #7FC6BC | `--c-bg` #141311 | 9.47:1 | 3:1 | pass | Primary button shape, checked checkbox and radio fill, selected-chip border |
| `--c-accent` #7FC6BC | `--c-surface` #1D1B18 | 8.76:1 | 3:1 | pass | Same, on cards |
| `--c-accent` #7FC6BC | `--c-accent-soft` #1F3532 | 6.64:1 | 3:1 | pass | Selected-chip border against its own fill |
| `--c-focus` #94B2FF | `--c-bg` #141311 | 8.90:1 | 3:1 | pass | Focus ring against the page (2.4.7; 2.4.13 change of contrast) |
| `--c-focus` #94B2FF | `--c-surface` #1D1B18 | 8.24:1 | 3:1 | pass | Focus ring against cards and dialogs |
| `--c-focus` #94B2FF | `--c-sunken` #25221E | 7.59:1 | 3:1 | pass | Focus ring in the composer area |
| `--c-user` #E2A24C | `--c-bg` #141311 | 8.41:1 | 3:1 | pass | 'Your words' rule (4px) on the page |
| `--c-user` #E2A24C | `--c-surface` #1D1B18 | 7.78:1 | 3:1 | pass | 'Your words' rule on a card |
| `--c-user` #E2A24C | `--c-user-bg` #2C2317 | 6.99:1 | 3:1 | pass | 'Your words' rule against its tint |
| `--c-ai` #ABA6F2 | `--c-bg` #141311 | 8.37:1 | 3:1 | pass | AI dashed or dotted outline on the page |
| `--c-ai` #ABA6F2 | `--c-surface` #1D1B18 | 7.75:1 | 3:1 | pass | AI outline on a card |
| `--c-ai` #ABA6F2 | `--c-ai-bg` #232139 | 7.02:1 | 3:1 | pass | AI outline against its tint |
| `--c-help` #F2917A | `--c-bg` #141311 | 8.05:1 | 3:1 | pass | Help button shape on the page |
| `--c-help` #F2917A | `--c-surface` #1D1B18 | 7.45:1 | 3:1 | pass | Help button shape on a card |
| `--c-danger` #F38E85 | `--c-bg` #141311 | 7.97:1 | 3:1 | pass | Destructive button border and invalid-field border on the page |
| `--c-danger` #F38E85 | `--c-surface` #1D1B18 | 7.38:1 | 3:1 | pass | Invalid-field border on a card |
| `--c-warning-border` #D9A440 | `--c-bg` #141311 | 8.25:1 | 3:1 | pass | Warning notice and status chip border |
| `--c-warning-border` #D9A440 | `--c-surface` #1D1B18 | 7.64:1 | 3:1 | pass | Warning chip border on a card |
| `--c-success` #83CC9B | `--c-bg` #141311 | 9.79:1 | 3:1 | pass | Success chip border |
| `--c-info` #90C3F0 | `--c-bg` #141311 | 9.95:1 | 3:1 | pass | Info chip border |
| `--c-demo` #C6B6F2 | `--c-bg` #141311 | 10.06:1 | 3:1 | pass | Demo banner border |
| `--c-test` #F2C94C | `--c-bg` #141311 | 11.70:1 | 3:1 | pass | Test-double banner border |
| `--c-text` #F2EEE7 | `--c-bg` #141311 | 16.06:1 | 3:1 | pass | Icons that carry meaning (drawn in the text colour) |
| `--c-muted` #BCB4A7 | `--c-bg` #141311 | 9.04:1 | 3:1 | pass | Secondary icons that carry meaning (decorative ones are exempt) |
| `--c-help-hover` #F7AE9C | `--c-bg` #141311 | 10.16:1 | 3:1 | pass | Help-variant shape, hover and pressed, on the page |
| `--c-help-hover` #F7AE9C | `--c-surface` #1D1B18 | 9.40:1 | 3:1 | pass | Help-variant shape, hover and pressed, on a card |
| `--c-help` #F2917A | `--c-help-bg` #36211B | 6.55:1 | 3:1 | pass | Help-variant link shape inside the crisis panel |
| `--c-text` #F2EEE7 | `--c-surface` #1D1B18 | 14.86:1 | 3:1 | pass | Ink-filled destructive confirm button shape on a card |
| `--c-warning-border` #D9A440 | `--c-warning-bg` #352A12 | 6.26:1 | 3:1 | pass | Warning notice border against its own tint |
| `--c-help-text` #F6AE9C | `--c-help-bg` #36211B | 8.24:1 | 3:1 | pass | Help-quiet life-buoy icon on its own fill |
| `--c-help-hover` #F7AE9C | `--c-help-bg` #36211B | 8.26:1 | 3:1 | pass | Help-quiet border, hover and pressed (3px), against its own fill |
| `--c-focus` #94B2FF | `--c-ai-bg` #232139 | 7.47:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-ai-bg |
| `--c-focus` #94B2FF | `--c-help-bg` #36211B | 7.24:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-help-bg |
| `--c-focus` #94B2FF | `--c-user-bg` #2C2317 | 7.41:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-user-bg |
| `--c-focus` #94B2FF | `--c-danger-bg` #3A1C19 | 7.41:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-danger-bg |
| `--c-focus` #94B2FF | `--c-success-bg` #18301F | 6.80:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-success-bg |
| `--c-focus` #94B2FF | `--c-warning-bg` #352A12 | 6.75:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-warning-bg |
| `--c-focus` #94B2FF | `--c-info-bg` #172A3B | 7.04:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-info-bg |
| `--c-focus` #94B2FF | `--c-demo-bg` #2A2340 | 7.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-demo-bg |
| `--c-focus` #94B2FF | `--c-test-bg` #352B0E | 6.70:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-test-bg |
| `--c-accent` #7FC6BC | `--c-ai-bg` #232139 | 7.94:1 | 3:1 | pass | Secondary button border inside a block tinted --c-ai-bg |
| `--c-accent` #7FC6BC | `--c-help-bg` #36211B | 7.70:1 | 3:1 | pass | Secondary button border inside a block tinted --c-help-bg |
| `--c-accent` #7FC6BC | `--c-user-bg` #2C2317 | 7.87:1 | 3:1 | pass | Secondary button border inside a block tinted --c-user-bg |
| `--c-accent` #7FC6BC | `--c-danger-bg` #3A1C19 | 7.87:1 | 3:1 | pass | Secondary button border inside a block tinted --c-danger-bg |
| `--c-accent` #7FC6BC | `--c-success-bg` #18301F | 7.23:1 | 3:1 | pass | Secondary button border inside a block tinted --c-success-bg |
| `--c-accent` #7FC6BC | `--c-warning-bg` #352A12 | 7.18:1 | 3:1 | pass | Secondary button border inside a block tinted --c-warning-bg |
| `--c-accent` #7FC6BC | `--c-info-bg` #172A3B | 7.48:1 | 3:1 | pass | Secondary button border inside a block tinted --c-info-bg |
| `--c-accent` #7FC6BC | `--c-demo-bg` #2A2340 | 7.58:1 | 3:1 | pass | Secondary button border inside a block tinted --c-demo-bg |
| `--c-accent` #7FC6BC | `--c-test-bg` #352B0E | 7.12:1 | 3:1 | pass | Secondary button border inside a block tinted --c-test-bg |
| `--c-border-control` #8E8679 | `--c-ai-bg` #232139 | 4.33:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-ai-bg |
| `--c-border-control` #8E8679 | `--c-help-bg` #36211B | 4.20:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-help-bg |
| `--c-border-control` #8E8679 | `--c-user-bg` #2C2317 | 4.29:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-user-bg |
| `--c-border-control` #8E8679 | `--c-danger-bg` #3A1C19 | 4.29:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-danger-bg |
| `--c-border-control` #8E8679 | `--c-success-bg` #18301F | 3.94:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-success-bg |
| `--c-border-control` #8E8679 | `--c-warning-bg` #352A12 | 3.91:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-warning-bg |
| `--c-border-control` #8E8679 | `--c-info-bg` #172A3B | 4.08:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-info-bg |
| `--c-border-control` #8E8679 | `--c-demo-bg` #2A2340 | 4.13:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-demo-bg |
| `--c-border-control` #8E8679 | `--c-test-bg` #352B0E | 3.88:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-test-bg |

#### contrast-light: text

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-text` #000000 | `--c-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Body text on the page |
| `--c-text` #000000 | `--c-surface` #FFFFFF | 21.00:1 | 4.5:1 | pass | Body text on cards, dialogs and inputs |
| `--c-text` #000000 | `--c-sunken` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in the composer area and inset panels |
| `--c-text` #000000 | `--c-accent-soft` #FFFFFF | 21.00:1 | 4.5:1 | pass | Label of a selected choice chip |
| `--c-text` #000000 | `--c-user-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text inside a 'Kata-katamu' (your words) block |
| `--c-text` #000000 | `--c-ai-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text inside an AI summary or 'Sela menangkap…' block |
| `--c-text` #000000 | `--c-help-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in the crisis support panel |
| `--c-text` #000000 | `--c-danger-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in an error notice |
| `--c-text` #000000 | `--c-success-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in a success notice |
| `--c-text` #000000 | `--c-warning-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in a warning notice |
| `--c-text` #000000 | `--c-info-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in an info notice and the guest banner |
| `--c-text` #000000 | `--c-demo-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in the demo banner |
| `--c-text` #000000 | `--c-test-bg` #FFFFFF | 21.00:1 | 4.5:1 | pass | Text in the test-double banner |
| `--c-muted` #1F1F1F | `--c-bg` #FFFFFF | 16.48:1 | 4.5:1 | pass | Secondary text and timestamps on the page |
| `--c-muted` #1F1F1F | `--c-surface` #FFFFFF | 16.48:1 | 4.5:1 | pass | Hints, meta lines and placeholder text on cards and inputs |
| `--c-muted` #1F1F1F | `--c-sunken` #FFFFFF | 16.48:1 | 4.5:1 | pass | Hints in the composer area |
| `--c-muted` #1F1F1F | `--c-user-bg` #FFFFFF | 16.48:1 | 4.5:1 | pass | Meta line inside a 'your words' block |
| `--c-muted` #1F1F1F | `--c-ai-bg` #FFFFFF | 16.48:1 | 4.5:1 | pass | Meta line inside an AI block |
| `--c-link` #003A99 | `--c-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Links (always underlined) on the page |
| `--c-link` #003A99 | `--c-surface` #FFFFFF | 10.17:1 | 4.5:1 | pass | Links on cards |
| `--c-link` #003A99 | `--c-sunken` #FFFFFF | 10.17:1 | 4.5:1 | pass | Links in the composer area |
| `--c-on-accent` #FFFFFF | `--c-accent` #00403B | 11.69:1 | 4.5:1 | pass | Primary button label |
| `--c-on-accent` #FFFFFF | `--c-accent-hover` #002B28 | 15.26:1 | 4.5:1 | pass | Primary button label, hover and pressed |
| `--c-accent` #00403B | `--c-surface` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label (outlined) |
| `--c-accent` #00403B | `--c-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label on the page |
| `--c-on-help` #FFFFFF | `--c-help` #7E1A07 | 10.30:1 | 4.5:1 | pass | 'Butuh bantuan sekarang' and 'Lihat semua bantuan' link label (Help variant) |
| `--c-on-help` #FFFFFF | `--c-help-hover` #5E1205 | 13.42:1 | 4.5:1 | pass | Help-variant label, hover and pressed |
| `--c-surface` #FFFFFF | `--c-text` #000000 | 21.00:1 | 4.5:1 | pass | Label of the ink-filled destructive confirm button (the confirm step of delete, forget, revoke) |
| `--c-surface` #FFFFFF | `--c-muted` #1F1F1F | 16.48:1 | 4.5:1 | pass | Same label, pressed state (fill --c-muted) |
| `--c-user-text` #4D2600 | `--c-user-bg` #FFFFFF | 13.18:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on its tint |
| `--c-user-text` #4D2600 | `--c-surface` #FFFFFF | 13.18:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on a card |
| `--c-ai-text` #2A2283 | `--c-ai-bg` #FFFFFF | 12.69:1 | 4.5:1 | pass | 'Tulisan AI' / 'Tafsiran Sela' label on its tint |
| `--c-ai-text` #2A2283 | `--c-surface` #FFFFFF | 12.69:1 | 4.5:1 | pass | AI provenance label on a card |
| `--c-help-text` #7E1A07 | `--c-help-bg` #FFFFFF | 10.30:1 | 4.5:1 | pass | Crisis panel heading |
| `--c-help-text` #7E1A07 | `--c-surface` #FFFFFF | 10.30:1 | 4.5:1 | pass | Help link text on cards |
| `--c-help-text` #7E1A07 | `--c-bg` #FFFFFF | 10.30:1 | 4.5:1 | pass | Help link text on the page |
| `--c-danger` #8B0F08 | `--c-surface` #FFFFFF | 9.68:1 | 4.5:1 | pass | Destructive button label (outlined) and field error text on cards |
| `--c-danger` #8B0F08 | `--c-bg` #FFFFFF | 9.68:1 | 4.5:1 | pass | Field error text on the page |
| `--c-danger` #8B0F08 | `--c-danger-bg` #FFFFFF | 9.68:1 | 4.5:1 | pass | Error notice heading |
| `--c-success` #0B4A22 | `--c-surface` #FFFFFF | 10.41:1 | 4.5:1 | pass | Success status text on a card |
| `--c-success` #0B4A22 | `--c-success-bg` #FFFFFF | 10.41:1 | 4.5:1 | pass | Success notice heading |
| `--c-warning` #5C3700 | `--c-surface` #FFFFFF | 10.49:1 | 4.5:1 | pass | Warning status text on a card |
| `--c-warning` #5C3700 | `--c-warning-bg` #FFFFFF | 10.49:1 | 4.5:1 | pass | Warning notice heading |
| `--c-info` #0B3A66 | `--c-surface` #FFFFFF | 11.59:1 | 4.5:1 | pass | Info status text on a card |
| `--c-info` #0B3A66 | `--c-info-bg` #FFFFFF | 11.59:1 | 4.5:1 | pass | Info notice heading |
| `--c-demo` #3B2475 | `--c-demo-bg` #FFFFFF | 12.39:1 | 4.5:1 | pass | Demo banner label |
| `--c-test` #4F3800 | `--c-test-bg` #FFFFFF | 11.06:1 | 4.5:1 | pass | Test-double banner label |
| `--c-muted` #1F1F1F | `--c-help-bg` #FFFFFF | 16.48:1 | 4.5:1 | pass | Service-card source and verification line inside the crisis panel |
| `--c-help-text` #7E1A07 | `--c-help-bg` #FFFFFF | 10.30:1 | 4.5:1 | pass | Help-quiet label: 'Butuh bantuan sekarang' in the top bar, the help strip and full-height sheets |
| `--c-link` #003A99 | `--c-ai-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-ai-bg |
| `--c-link` #003A99 | `--c-help-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-help-bg |
| `--c-link` #003A99 | `--c-user-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-user-bg |
| `--c-link` #003A99 | `--c-danger-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-danger-bg |
| `--c-link` #003A99 | `--c-success-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-success-bg |
| `--c-link` #003A99 | `--c-warning-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-warning-bg |
| `--c-link` #003A99 | `--c-info-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-info-bg |
| `--c-link` #003A99 | `--c-demo-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-demo-bg |
| `--c-link` #003A99 | `--c-test-bg` #FFFFFF | 10.17:1 | 4.5:1 | pass | Link text inside a block tinted --c-test-bg |
| `--c-accent` #00403B | `--c-ai-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-ai-bg |
| `--c-accent` #00403B | `--c-help-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-help-bg |
| `--c-accent` #00403B | `--c-user-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-user-bg |
| `--c-accent` #00403B | `--c-danger-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-danger-bg |
| `--c-accent` #00403B | `--c-success-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-success-bg |
| `--c-accent` #00403B | `--c-warning-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-warning-bg |
| `--c-accent` #00403B | `--c-info-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-info-bg |
| `--c-accent` #00403B | `--c-demo-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-demo-bg |
| `--c-accent` #00403B | `--c-test-bg` #FFFFFF | 11.69:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-test-bg |

#### contrast-light: UI components and graphics

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-border-control` #000000 | `--c-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on the page |
| `--c-border-control` #000000 | `--c-surface` #FFFFFF | 21.00:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on a card |
| `--c-border-control` #000000 | `--c-sunken` #FFFFFF | 21.00:1 | 3:1 | pass | Composer text area boundary |
| `--c-accent` #00403B | `--c-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Primary button shape, checked checkbox and radio fill, selected-chip border |
| `--c-accent` #00403B | `--c-surface` #FFFFFF | 11.69:1 | 3:1 | pass | Same, on cards |
| `--c-accent` #00403B | `--c-accent-soft` #FFFFFF | 11.69:1 | 3:1 | pass | Selected-chip border against its own fill |
| `--c-focus` #0033CC | `--c-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring against the page (2.4.7; 2.4.13 change of contrast) |
| `--c-focus` #0033CC | `--c-surface` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring against cards and dialogs |
| `--c-focus` #0033CC | `--c-sunken` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring in the composer area |
| `--c-user` #6B3500 | `--c-bg` #FFFFFF | 9.84:1 | 3:1 | pass | 'Your words' rule (4px) on the page |
| `--c-user` #6B3500 | `--c-surface` #FFFFFF | 9.84:1 | 3:1 | pass | 'Your words' rule on a card |
| `--c-user` #6B3500 | `--c-user-bg` #FFFFFF | 9.84:1 | 3:1 | pass | 'Your words' rule against its tint |
| `--c-ai` #2A2283 | `--c-bg` #FFFFFF | 12.69:1 | 3:1 | pass | AI dashed or dotted outline on the page |
| `--c-ai` #2A2283 | `--c-surface` #FFFFFF | 12.69:1 | 3:1 | pass | AI outline on a card |
| `--c-ai` #2A2283 | `--c-ai-bg` #FFFFFF | 12.69:1 | 3:1 | pass | AI outline against its tint |
| `--c-help` #7E1A07 | `--c-bg` #FFFFFF | 10.30:1 | 3:1 | pass | Help button shape on the page |
| `--c-help` #7E1A07 | `--c-surface` #FFFFFF | 10.30:1 | 3:1 | pass | Help button shape on a card |
| `--c-danger` #8B0F08 | `--c-bg` #FFFFFF | 9.68:1 | 3:1 | pass | Destructive button border and invalid-field border on the page |
| `--c-danger` #8B0F08 | `--c-surface` #FFFFFF | 9.68:1 | 3:1 | pass | Invalid-field border on a card |
| `--c-warning-border` #5C3700 | `--c-bg` #FFFFFF | 10.49:1 | 3:1 | pass | Warning notice and status chip border |
| `--c-warning-border` #5C3700 | `--c-surface` #FFFFFF | 10.49:1 | 3:1 | pass | Warning chip border on a card |
| `--c-success` #0B4A22 | `--c-bg` #FFFFFF | 10.41:1 | 3:1 | pass | Success chip border |
| `--c-info` #0B3A66 | `--c-bg` #FFFFFF | 11.59:1 | 3:1 | pass | Info chip border |
| `--c-demo` #3B2475 | `--c-bg` #FFFFFF | 12.39:1 | 3:1 | pass | Demo banner border |
| `--c-test` #4F3800 | `--c-bg` #FFFFFF | 11.06:1 | 3:1 | pass | Test-double banner border |
| `--c-text` #000000 | `--c-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Icons that carry meaning (drawn in the text colour) |
| `--c-muted` #1F1F1F | `--c-bg` #FFFFFF | 16.48:1 | 3:1 | pass | Secondary icons that carry meaning (decorative ones are exempt) |
| `--c-help-hover` #5E1205 | `--c-bg` #FFFFFF | 13.42:1 | 3:1 | pass | Help-variant shape, hover and pressed, on the page |
| `--c-help-hover` #5E1205 | `--c-surface` #FFFFFF | 13.42:1 | 3:1 | pass | Help-variant shape, hover and pressed, on a card |
| `--c-help` #7E1A07 | `--c-help-bg` #FFFFFF | 10.30:1 | 3:1 | pass | Help-variant link shape inside the crisis panel |
| `--c-text` #000000 | `--c-surface` #FFFFFF | 21.00:1 | 3:1 | pass | Ink-filled destructive confirm button shape on a card |
| `--c-warning-border` #5C3700 | `--c-warning-bg` #FFFFFF | 10.49:1 | 3:1 | pass | Warning notice border against its own tint |
| `--c-help-text` #7E1A07 | `--c-help-bg` #FFFFFF | 10.30:1 | 3:1 | pass | Help-quiet life-buoy icon on its own fill |
| `--c-help-hover` #5E1205 | `--c-help-bg` #FFFFFF | 13.42:1 | 3:1 | pass | Help-quiet border, hover and pressed (3px), against its own fill |
| `--c-focus` #0033CC | `--c-ai-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-ai-bg |
| `--c-focus` #0033CC | `--c-help-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-help-bg |
| `--c-focus` #0033CC | `--c-user-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-user-bg |
| `--c-focus` #0033CC | `--c-danger-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-danger-bg |
| `--c-focus` #0033CC | `--c-success-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-success-bg |
| `--c-focus` #0033CC | `--c-warning-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-warning-bg |
| `--c-focus` #0033CC | `--c-info-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-info-bg |
| `--c-focus` #0033CC | `--c-demo-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-demo-bg |
| `--c-focus` #0033CC | `--c-test-bg` #FFFFFF | 8.95:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-test-bg |
| `--c-accent` #00403B | `--c-ai-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-ai-bg |
| `--c-accent` #00403B | `--c-help-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-help-bg |
| `--c-accent` #00403B | `--c-user-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-user-bg |
| `--c-accent` #00403B | `--c-danger-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-danger-bg |
| `--c-accent` #00403B | `--c-success-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-success-bg |
| `--c-accent` #00403B | `--c-warning-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-warning-bg |
| `--c-accent` #00403B | `--c-info-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-info-bg |
| `--c-accent` #00403B | `--c-demo-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-demo-bg |
| `--c-accent` #00403B | `--c-test-bg` #FFFFFF | 11.69:1 | 3:1 | pass | Secondary button border inside a block tinted --c-test-bg |
| `--c-border-control` #000000 | `--c-ai-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-ai-bg |
| `--c-border-control` #000000 | `--c-help-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-help-bg |
| `--c-border-control` #000000 | `--c-user-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-user-bg |
| `--c-border-control` #000000 | `--c-danger-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-danger-bg |
| `--c-border-control` #000000 | `--c-success-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-success-bg |
| `--c-border-control` #000000 | `--c-warning-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-warning-bg |
| `--c-border-control` #000000 | `--c-info-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-info-bg |
| `--c-border-control` #000000 | `--c-demo-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-demo-bg |
| `--c-border-control` #000000 | `--c-test-bg` #FFFFFF | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-test-bg |

#### contrast-dark: text

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-text` #FFFFFF | `--c-bg` #000000 | 21.00:1 | 4.5:1 | pass | Body text on the page |
| `--c-text` #FFFFFF | `--c-surface` #000000 | 21.00:1 | 4.5:1 | pass | Body text on cards, dialogs and inputs |
| `--c-text` #FFFFFF | `--c-sunken` #000000 | 21.00:1 | 4.5:1 | pass | Text in the composer area and inset panels |
| `--c-text` #FFFFFF | `--c-accent-soft` #000000 | 21.00:1 | 4.5:1 | pass | Label of a selected choice chip |
| `--c-text` #FFFFFF | `--c-user-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text inside a 'Kata-katamu' (your words) block |
| `--c-text` #FFFFFF | `--c-ai-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text inside an AI summary or 'Sela menangkap…' block |
| `--c-text` #FFFFFF | `--c-help-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text in the crisis support panel |
| `--c-text` #FFFFFF | `--c-danger-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text in an error notice |
| `--c-text` #FFFFFF | `--c-success-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text in a success notice |
| `--c-text` #FFFFFF | `--c-warning-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text in a warning notice |
| `--c-text` #FFFFFF | `--c-info-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text in an info notice and the guest banner |
| `--c-text` #FFFFFF | `--c-demo-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text in the demo banner |
| `--c-text` #FFFFFF | `--c-test-bg` #000000 | 21.00:1 | 4.5:1 | pass | Text in the test-double banner |
| `--c-muted` #E8E8E8 | `--c-bg` #000000 | 17.14:1 | 4.5:1 | pass | Secondary text and timestamps on the page |
| `--c-muted` #E8E8E8 | `--c-surface` #000000 | 17.14:1 | 4.5:1 | pass | Hints, meta lines and placeholder text on cards and inputs |
| `--c-muted` #E8E8E8 | `--c-sunken` #000000 | 17.14:1 | 4.5:1 | pass | Hints in the composer area |
| `--c-muted` #E8E8E8 | `--c-user-bg` #000000 | 17.14:1 | 4.5:1 | pass | Meta line inside a 'your words' block |
| `--c-muted` #E8E8E8 | `--c-ai-bg` #000000 | 17.14:1 | 4.5:1 | pass | Meta line inside an AI block |
| `--c-link` #A6F2E8 | `--c-bg` #000000 | 16.49:1 | 4.5:1 | pass | Links (always underlined) on the page |
| `--c-link` #A6F2E8 | `--c-surface` #000000 | 16.49:1 | 4.5:1 | pass | Links on cards |
| `--c-link` #A6F2E8 | `--c-sunken` #000000 | 16.49:1 | 4.5:1 | pass | Links in the composer area |
| `--c-on-accent` #000000 | `--c-accent` #A6F2E8 | 16.49:1 | 4.5:1 | pass | Primary button label |
| `--c-on-accent` #000000 | `--c-accent-hover` #C8F8F1 | 18.15:1 | 4.5:1 | pass | Primary button label, hover and pressed |
| `--c-accent` #A6F2E8 | `--c-surface` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label (outlined) |
| `--c-accent` #A6F2E8 | `--c-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label on the page |
| `--c-on-help` #000000 | `--c-help` #FFA28D | 10.80:1 | 4.5:1 | pass | 'Butuh bantuan sekarang' and 'Lihat semua bantuan' link label (Help variant) |
| `--c-on-help` #000000 | `--c-help-hover` #FFC2B3 | 13.62:1 | 4.5:1 | pass | Help-variant label, hover and pressed |
| `--c-surface` #000000 | `--c-text` #FFFFFF | 21.00:1 | 4.5:1 | pass | Label of the ink-filled destructive confirm button (the confirm step of delete, forget, revoke) |
| `--c-surface` #000000 | `--c-muted` #E8E8E8 | 17.14:1 | 4.5:1 | pass | Same label, pressed state (fill --c-muted) |
| `--c-user-text` #FFC97F | `--c-user-bg` #000000 | 13.91:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on its tint |
| `--c-user-text` #FFC97F | `--c-surface` #000000 | 13.91:1 | 4.5:1 | pass | 'Kata-katamu' provenance label on a card |
| `--c-ai-text` #CFCBFF | `--c-ai-bg` #000000 | 13.64:1 | 4.5:1 | pass | 'Tulisan AI' / 'Tafsiran Sela' label on its tint |
| `--c-ai-text` #CFCBFF | `--c-surface` #000000 | 13.64:1 | 4.5:1 | pass | AI provenance label on a card |
| `--c-help-text` #FFB4A3 | `--c-help-bg` #000000 | 12.31:1 | 4.5:1 | pass | Crisis panel heading |
| `--c-help-text` #FFB4A3 | `--c-surface` #000000 | 12.31:1 | 4.5:1 | pass | Help link text on cards |
| `--c-help-text` #FFB4A3 | `--c-bg` #000000 | 12.31:1 | 4.5:1 | pass | Help link text on the page |
| `--c-danger` #FF9C94 | `--c-surface` #000000 | 10.44:1 | 4.5:1 | pass | Destructive button label (outlined) and field error text on cards |
| `--c-danger` #FF9C94 | `--c-bg` #000000 | 10.44:1 | 4.5:1 | pass | Field error text on the page |
| `--c-danger` #FF9C94 | `--c-danger-bg` #000000 | 10.44:1 | 4.5:1 | pass | Error notice heading |
| `--c-success` #8FE3AA | `--c-surface` #000000 | 13.74:1 | 4.5:1 | pass | Success status text on a card |
| `--c-success` #8FE3AA | `--c-success-bg` #000000 | 13.74:1 | 4.5:1 | pass | Success notice heading |
| `--c-warning` #FFD27A | `--c-surface` #000000 | 14.75:1 | 4.5:1 | pass | Warning status text on a card |
| `--c-warning` #FFD27A | `--c-warning-bg` #000000 | 14.75:1 | 4.5:1 | pass | Warning notice heading |
| `--c-info` #A3D2FF | `--c-surface` #000000 | 13.22:1 | 4.5:1 | pass | Info status text on a card |
| `--c-info` #A3D2FF | `--c-info-bg` #000000 | 13.22:1 | 4.5:1 | pass | Info notice heading |
| `--c-demo` #D9CCFF | `--c-demo-bg` #000000 | 14.03:1 | 4.5:1 | pass | Demo banner label |
| `--c-test` #FFDA66 | `--c-test-bg` #000000 | 15.47:1 | 4.5:1 | pass | Test-double banner label |
| `--c-muted` #E8E8E8 | `--c-help-bg` #000000 | 17.14:1 | 4.5:1 | pass | Service-card source and verification line inside the crisis panel |
| `--c-help-text` #FFB4A3 | `--c-help-bg` #000000 | 12.31:1 | 4.5:1 | pass | Help-quiet label: 'Butuh bantuan sekarang' in the top bar, the help strip and full-height sheets |
| `--c-link` #A6F2E8 | `--c-ai-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-ai-bg |
| `--c-link` #A6F2E8 | `--c-help-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-help-bg |
| `--c-link` #A6F2E8 | `--c-user-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-user-bg |
| `--c-link` #A6F2E8 | `--c-danger-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-danger-bg |
| `--c-link` #A6F2E8 | `--c-success-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-success-bg |
| `--c-link` #A6F2E8 | `--c-warning-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-warning-bg |
| `--c-link` #A6F2E8 | `--c-info-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-info-bg |
| `--c-link` #A6F2E8 | `--c-demo-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-demo-bg |
| `--c-link` #A6F2E8 | `--c-test-bg` #000000 | 16.49:1 | 4.5:1 | pass | Link text inside a block tinted --c-test-bg |
| `--c-accent` #A6F2E8 | `--c-ai-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-ai-bg |
| `--c-accent` #A6F2E8 | `--c-help-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-help-bg |
| `--c-accent` #A6F2E8 | `--c-user-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-user-bg |
| `--c-accent` #A6F2E8 | `--c-danger-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-danger-bg |
| `--c-accent` #A6F2E8 | `--c-success-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-success-bg |
| `--c-accent` #A6F2E8 | `--c-warning-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-warning-bg |
| `--c-accent` #A6F2E8 | `--c-info-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-info-bg |
| `--c-accent` #A6F2E8 | `--c-demo-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-demo-bg |
| `--c-accent` #A6F2E8 | `--c-test-bg` #000000 | 16.49:1 | 4.5:1 | pass | Secondary button label inside a block tinted --c-test-bg |

#### contrast-dark: UI components and graphics

| Foreground | Background | Ratio | Minimum | Result | Used for |
|---|---|---|---|---|---|
| `--c-border-control` #FFFFFF | `--c-bg` #000000 | 21.00:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on the page |
| `--c-border-control` #FFFFFF | `--c-surface` #000000 | 21.00:1 | 3:1 | pass | Input, chip, checkbox and radio boundary on a card |
| `--c-border-control` #FFFFFF | `--c-sunken` #000000 | 21.00:1 | 3:1 | pass | Composer text area boundary |
| `--c-accent` #A6F2E8 | `--c-bg` #000000 | 16.49:1 | 3:1 | pass | Primary button shape, checked checkbox and radio fill, selected-chip border |
| `--c-accent` #A6F2E8 | `--c-surface` #000000 | 16.49:1 | 3:1 | pass | Same, on cards |
| `--c-accent` #A6F2E8 | `--c-accent-soft` #000000 | 16.49:1 | 3:1 | pass | Selected-chip border against its own fill |
| `--c-focus` #FFE14D | `--c-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring against the page (2.4.7; 2.4.13 change of contrast) |
| `--c-focus` #FFE14D | `--c-surface` #000000 | 16.13:1 | 3:1 | pass | Focus ring against cards and dialogs |
| `--c-focus` #FFE14D | `--c-sunken` #000000 | 16.13:1 | 3:1 | pass | Focus ring in the composer area |
| `--c-user` #FFB85C | `--c-bg` #000000 | 12.26:1 | 3:1 | pass | 'Your words' rule (4px) on the page |
| `--c-user` #FFB85C | `--c-surface` #000000 | 12.26:1 | 3:1 | pass | 'Your words' rule on a card |
| `--c-user` #FFB85C | `--c-user-bg` #000000 | 12.26:1 | 3:1 | pass | 'Your words' rule against its tint |
| `--c-ai` #CFCBFF | `--c-bg` #000000 | 13.64:1 | 3:1 | pass | AI dashed or dotted outline on the page |
| `--c-ai` #CFCBFF | `--c-surface` #000000 | 13.64:1 | 3:1 | pass | AI outline on a card |
| `--c-ai` #CFCBFF | `--c-ai-bg` #000000 | 13.64:1 | 3:1 | pass | AI outline against its tint |
| `--c-help` #FFA28D | `--c-bg` #000000 | 10.80:1 | 3:1 | pass | Help button shape on the page |
| `--c-help` #FFA28D | `--c-surface` #000000 | 10.80:1 | 3:1 | pass | Help button shape on a card |
| `--c-danger` #FF9C94 | `--c-bg` #000000 | 10.44:1 | 3:1 | pass | Destructive button border and invalid-field border on the page |
| `--c-danger` #FF9C94 | `--c-surface` #000000 | 10.44:1 | 3:1 | pass | Invalid-field border on a card |
| `--c-warning-border` #FFD27A | `--c-bg` #000000 | 14.75:1 | 3:1 | pass | Warning notice and status chip border |
| `--c-warning-border` #FFD27A | `--c-surface` #000000 | 14.75:1 | 3:1 | pass | Warning chip border on a card |
| `--c-success` #8FE3AA | `--c-bg` #000000 | 13.74:1 | 3:1 | pass | Success chip border |
| `--c-info` #A3D2FF | `--c-bg` #000000 | 13.22:1 | 3:1 | pass | Info chip border |
| `--c-demo` #D9CCFF | `--c-bg` #000000 | 14.03:1 | 3:1 | pass | Demo banner border |
| `--c-test` #FFDA66 | `--c-bg` #000000 | 15.47:1 | 3:1 | pass | Test-double banner border |
| `--c-text` #FFFFFF | `--c-bg` #000000 | 21.00:1 | 3:1 | pass | Icons that carry meaning (drawn in the text colour) |
| `--c-muted` #E8E8E8 | `--c-bg` #000000 | 17.14:1 | 3:1 | pass | Secondary icons that carry meaning (decorative ones are exempt) |
| `--c-help-hover` #FFC2B3 | `--c-bg` #000000 | 13.62:1 | 3:1 | pass | Help-variant shape, hover and pressed, on the page |
| `--c-help-hover` #FFC2B3 | `--c-surface` #000000 | 13.62:1 | 3:1 | pass | Help-variant shape, hover and pressed, on a card |
| `--c-help` #FFA28D | `--c-help-bg` #000000 | 10.80:1 | 3:1 | pass | Help-variant link shape inside the crisis panel |
| `--c-text` #FFFFFF | `--c-surface` #000000 | 21.00:1 | 3:1 | pass | Ink-filled destructive confirm button shape on a card |
| `--c-warning-border` #FFD27A | `--c-warning-bg` #000000 | 14.75:1 | 3:1 | pass | Warning notice border against its own tint |
| `--c-help-text` #FFB4A3 | `--c-help-bg` #000000 | 12.31:1 | 3:1 | pass | Help-quiet life-buoy icon on its own fill |
| `--c-help-hover` #FFC2B3 | `--c-help-bg` #000000 | 13.62:1 | 3:1 | pass | Help-quiet border, hover and pressed (3px), against its own fill |
| `--c-focus` #FFE14D | `--c-ai-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-ai-bg |
| `--c-focus` #FFE14D | `--c-help-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-help-bg |
| `--c-focus` #FFE14D | `--c-user-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-user-bg |
| `--c-focus` #FFE14D | `--c-danger-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-danger-bg |
| `--c-focus` #FFE14D | `--c-success-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-success-bg |
| `--c-focus` #FFE14D | `--c-warning-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-warning-bg |
| `--c-focus` #FFE14D | `--c-info-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-info-bg |
| `--c-focus` #FFE14D | `--c-demo-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-demo-bg |
| `--c-focus` #FFE14D | `--c-test-bg` #000000 | 16.13:1 | 3:1 | pass | Focus ring next to a control inside a block tinted --c-test-bg |
| `--c-accent` #A6F2E8 | `--c-ai-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-ai-bg |
| `--c-accent` #A6F2E8 | `--c-help-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-help-bg |
| `--c-accent` #A6F2E8 | `--c-user-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-user-bg |
| `--c-accent` #A6F2E8 | `--c-danger-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-danger-bg |
| `--c-accent` #A6F2E8 | `--c-success-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-success-bg |
| `--c-accent` #A6F2E8 | `--c-warning-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-warning-bg |
| `--c-accent` #A6F2E8 | `--c-info-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-info-bg |
| `--c-accent` #A6F2E8 | `--c-demo-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-demo-bg |
| `--c-accent` #A6F2E8 | `--c-test-bg` #000000 | 16.49:1 | 3:1 | pass | Secondary button border inside a block tinted --c-test-bg |
| `--c-border-control` #FFFFFF | `--c-ai-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-ai-bg |
| `--c-border-control` #FFFFFF | `--c-help-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-help-bg |
| `--c-border-control` #FFFFFF | `--c-user-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-user-bg |
| `--c-border-control` #FFFFFF | `--c-danger-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-danger-bg |
| `--c-border-control` #FFFFFF | `--c-success-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-success-bg |
| `--c-border-control` #FFFFFF | `--c-warning-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-warning-bg |
| `--c-border-control` #FFFFFF | `--c-info-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-info-bg |
| `--c-border-control` #FFFFFF | `--c-demo-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-demo-bg |
| `--c-border-control` #FFFFFF | `--c-test-bg` #000000 | 21.00:1 | 3:1 | pass | Input or checkbox boundary inside a block tinted --c-test-bg |
<!-- END GENERATED: tables -->

## Appendix B: Token sheet

A verbatim copy of [tokens.css](tokens.css), generated by `sync_docs.py`. If the two ever differ, `tokens.css` wins.

<!-- BEGIN GENERATED: tokens -->
```css
/*
 * Sela v3 design tokens (reference sheet).
 *
 * Status: design decision by the design lead, 2026-10-26. Not implemented:
 * the frontend copies this file to web/src/styles/tokens.css when it builds
 * the v3 design system. docs/design/tools/contrast.py reads the colour
 * blocks below (between "@scheme <name>" and "@end" markers) and computes
 * every WCAG contrast pair listed in DESIGN_DIRECTION.md.
 *
 * Rules
 * - Sizes are rem or em only (no vw/vh in type), so browser zoom, text-only
 *   zoom and Sela's own text-size setting all scale every text token.
 * - Colour is set on :root by scheme. The boot script (web/public/boot.js,
 *   same-origin, no inline script because the CSP is script-src 'self')
 *   writes data-scheme, data-text-scale, data-spacing and data-motion on
 *   <html> before the stylesheet is applied. Without the script, the light
 *   scheme applies, or the dark one through the prefers-color-scheme
 *   fallback at the end of this file.
 * - Components never use raw hex values; they use these roles.
 */

:root {
  /* ---------- Typography ---------- */
  --font-sans: system-ui, -apple-system, "Segoe UI", Roboto, "Noto Sans", "Helvetica Neue", Arial, sans-serif;
  --font-mono: ui-monospace, "SFMono-Regular", "Roboto Mono", "Noto Sans Mono", Menlo, Consolas, monospace;

  --font-size-1: 0.875rem;   /* 14px: timestamps and footnotes only. Never smaller, and never for
                                labels that carry meaning (provenance badges, status chips, meta-line
                                actions and navigation labels use --font-size-2). */
  --font-size-2: 1rem;       /* 16px: buttons, chips, labels, badges, status chips, navigation labels */
  --font-size-3: 1.0625rem;  /* 17px: body text */
  --font-size-4: 1.125rem;   /* 18px: transcript, journal and reading text */
  --font-size-5: 1.3125rem;  /* 21px: h3, card titles */
  --font-size-6: 1.5rem;     /* 24px: h2 */
  --font-size-7: 1.75rem;    /* 28px: h1 in compact layouts */
  --font-size-8: 2.125rem;   /* 34px: h1 in wide layouts, intro title */

  --line-height-body: 1.6;
  --line-height-heading: 1.3;
  --line-height-ui: 1.4;
  --letter-spacing-body: normal;
  --word-spacing-body: normal;
  --paragraph-gap: 0.75em;

  --font-weight-regular: 400;
  --font-weight-medium: 600;
  --font-weight-bold: 700;

  --measure: 38rem;          /* about 65 characters at --font-size-4 */
  --measure-wide: 52rem;     /* forms with two columns, settings */
  --shell-max: 80rem;

  /* ---------- Space (4px grid) ---------- */
  --space-1: 0.25rem;
  --space-2: 0.5rem;
  --space-3: 0.75rem;
  --space-4: 1rem;
  --space-5: 1.5rem;
  --space-6: 2rem;
  --space-7: 3rem;
  --space-8: 4rem;
  --gutter: 1rem;            /* side gutter at every width below 40em */
  --stack-gap: 1.25rem;      /* default vertical rhythm between blocks */

  /* ---------- Shape ---------- */
  --radius-1: 4px;           /* provenance badges, status chips, code */
  --radius-2: 8px;           /* buttons, inputs, notices */
  --radius-3: 12px;          /* cards, panels, transcript blocks */
  --radius-4: 20px;          /* sheet top corners, dialogs */
  --radius-pill: 999px;      /* choice chips only (shape means "a choice") */

  --border-width: 1px;
  --border-width-strong: 2px;
  --rule-width: 4px;         /* "your words" rule */

  /* ---------- Size ---------- */
  --target-min: 2.75rem;     /* 44px minimum for every pointer target except inline links */
  --target-gap: 0.5rem;      /* minimum gap between adjacent targets */
  --icon-size: 1.25rem;      /* 20px; icons scale with text */
  --icon-size-large: 1.5rem;
  --bar-min-height: 3.5rem;  /* minimum height of one row of the top bar or bottom bar; bars grow
                                with their content and wrap to more rows (never a fixed height) */
  --strip-pad: 0.375rem;     /* 6px of --c-bg around the fixed help control, so its offset focus ring
                                (2px offset + 3px width) is drawn on the strip, never clipped by the
                                viewport edge and never on the control's own fill */
  --room-reading-min: 5.4rem;    /* three transcript lines (3 x 1.6 x --font-size-4; 86px at 100% text).
                                    While typing in the room on a phone, if the help strip and the sticky
                                    composer would leave less than this, the composer joins the page flow */
  --room-reading-target: 7.2rem; /* four transcript lines (115px at 100% text): the target the scripted
                                    typing checks assert while the composer is sticky */
  /* --vvh (set by the shell script from visualViewport.height, in px) caps the room's layers:
     the focused text area at 25% of --vvh while typing, the sticky composer at 50% of --vvh. */

  /* ---------- Focus ---------- */
  --focus-width: 3px;
  --focus-offset: 2px;

  /* ---------- Elevation ---------- */
  --shadow-1: 0 1px 2px rgb(30 28 25 / 0.10);
  --shadow-2: 0 8px 24px rgb(30 28 25 / 0.18);

  /* ---------- Motion ---------- */
  --duration-quick: 120ms;   /* hover, press, colour changes */
  --duration-gentle: 200ms;  /* sheet and dialog entry, disclosure */
  --reveal-step: 180ms;      /* delay between sentences when a released reply is revealed (full motion only) */
  --ease-standard: cubic-bezier(0.2, 0, 0, 1);

  /* ---------- Layers ---------- */
  --z-sticky: 10;            /* top bar, bottom navigation, composer */
  --z-sheet: 30;
  --z-dialog: 40;
  --z-skip: 50;

  font-family: var(--font-sans);
  font-size: 100%;
}

/* ---------- Colour schemes ---------- */

/* @scheme light */
:root,
:root[data-scheme="light"] {
  color-scheme: light;
  --c-bg: #FAF7F2;             /* kertas (paper) */
  --c-surface: #FFFFFF;
  --c-sunken: #F1ECE4;
  --c-text: #1E1C19;           /* tinta (ink) */
  --c-muted: #575249;
  --c-border-subtle: #E3DCD0;  /* decorative separators only */
  --c-border-control: #7B7368; /* inputs, chips, checkboxes */
  --c-accent: #1D5C57;         /* laut (deep sea teal) */
  --c-accent-hover: #154743;
  --c-on-accent: #FFFFFF;
  --c-accent-soft: #DDEBE8;
  --c-link: #1B5A55;
  --c-focus: #2747C9;
  --c-user: #9A5A00;           /* kunyit (turmeric): the user's own words */
  --c-user-bg: #FBF0DF;
  --c-user-text: #6E4100;
  --c-ai: #5048A8;             /* nila (indigo): anything an AI wrote */
  --c-ai-bg: #EFEEF9;
  --c-ai-text: #3E378F;
  --c-help: #A1351E;           /* mengkudu (noni red): help entry points only */
  --c-help-hover: #852A16;      /* help hover and pressed: darker in light schemes, lighter in dark ones */
  --c-on-help: #FFFFFF;
  --c-help-bg: #FBE9E4;
  --c-help-text: #8C2C17;
  --c-danger: #A3271F;
  --c-danger-bg: #FCE9E7;
  --c-success: #23683C;
  --c-success-bg: #E6F2E9;
  --c-warning: #875200;
  --c-warning-bg: #FCF1D9;
  --c-warning-border: #A86E00;
  --c-info: #1F5788;
  --c-info-bg: #E7F0F8;
  --c-demo: #5E4A99;
  --c-demo-bg: #EEE9F8;
  --c-test: #7A5600;
  --c-test-bg: #FFF1C7;
  --c-overlay: rgb(20 19 17 / 0.5);
}
/* @end */

/* @scheme dark */
:root[data-scheme="dark"] {
  color-scheme: dark;
  --c-bg: #141311;
  --c-surface: #1D1B18;
  --c-sunken: #25221E;
  --c-text: #F2EEE7;
  --c-muted: #BCB4A7;
  --c-border-subtle: #35312B;
  --c-border-control: #8E8679;
  --c-accent: #7FC6BC;
  --c-accent-hover: #9BD5CD;
  --c-on-accent: #0C1D1B;
  --c-accent-soft: #1F3532;
  --c-link: #8ED2C8;
  --c-focus: #94B2FF;
  --c-user: #E2A24C;
  --c-user-bg: #2C2317;
  --c-user-text: #F0C27E;
  --c-ai: #ABA6F2;
  --c-ai-bg: #232139;
  --c-ai-text: #C4C0FA;
  --c-help: #F2917A;
  --c-help-hover: #F7AE9C;
  --c-on-help: #2B0E07;
  --c-help-bg: #36211B;
  --c-help-text: #F6AE9C;
  --c-danger: #F38E85;
  --c-danger-bg: #3A1C19;
  --c-success: #83CC9B;
  --c-success-bg: #18301F;
  --c-warning: #F1C46F;
  --c-warning-bg: #352A12;
  --c-warning-border: #D9A440;
  --c-info: #90C3F0;
  --c-info-bg: #172A3B;
  --c-demo: #C6B6F2;
  --c-demo-bg: #2A2340;
  --c-test: #F2C94C;
  --c-test-bg: #352B0E;
  --c-overlay: rgb(0 0 0 / 0.6);
  --shadow-1: 0 1px 2px rgb(0 0 0 / 0.5);
  --shadow-2: 0 8px 24px rgb(0 0 0 / 0.6);
}
/* @end */

/* @scheme contrast-light */
:root[data-scheme="contrast-light"] {
  color-scheme: light;
  --c-bg: #FFFFFF;
  --c-surface: #FFFFFF;
  --c-sunken: #FFFFFF;
  --c-text: #000000;
  --c-muted: #1F1F1F;
  --c-border-subtle: #000000;
  --c-border-control: #000000;
  --c-accent: #00403B;
  --c-accent-hover: #002B28;
  --c-on-accent: #FFFFFF;
  --c-accent-soft: #FFFFFF;
  --c-link: #003A99;
  --c-focus: #0033CC;
  --c-user: #6B3500;
  --c-user-bg: #FFFFFF;
  --c-user-text: #4D2600;
  --c-ai: #2A2283;
  --c-ai-bg: #FFFFFF;
  --c-ai-text: #2A2283;
  --c-help: #7E1A07;
  --c-help-hover: #5E1205;
  --c-on-help: #FFFFFF;
  --c-help-bg: #FFFFFF;
  --c-help-text: #7E1A07;
  --c-danger: #8B0F08;
  --c-danger-bg: #FFFFFF;
  --c-success: #0B4A22;
  --c-success-bg: #FFFFFF;
  --c-warning: #5C3700;
  --c-warning-bg: #FFFFFF;
  --c-warning-border: #5C3700;
  --c-info: #0B3A66;
  --c-info-bg: #FFFFFF;
  --c-demo: #3B2475;
  --c-demo-bg: #FFFFFF;
  --c-test: #4F3800;
  --c-test-bg: #FFFFFF;
  --c-overlay: rgb(0 0 0 / 0.7);
  --shadow-1: none;
  --shadow-2: none;
  --border-width: 2px;
}
/* @end */

/* @scheme contrast-dark */
:root[data-scheme="contrast-dark"] {
  color-scheme: dark;
  --c-bg: #000000;
  --c-surface: #000000;
  --c-sunken: #000000;
  --c-text: #FFFFFF;
  --c-muted: #E8E8E8;
  --c-border-subtle: #FFFFFF;
  --c-border-control: #FFFFFF;
  --c-accent: #A6F2E8;
  --c-accent-hover: #C8F8F1;
  --c-on-accent: #000000;
  --c-accent-soft: #000000;
  --c-link: #A6F2E8;
  --c-focus: #FFE14D;
  --c-user: #FFB85C;
  --c-user-bg: #000000;
  --c-user-text: #FFC97F;
  --c-ai: #CFCBFF;
  --c-ai-bg: #000000;
  --c-ai-text: #CFCBFF;
  --c-help: #FFA28D;
  --c-help-hover: #FFC2B3;
  --c-on-help: #000000;
  --c-help-bg: #000000;
  --c-help-text: #FFB4A3;
  --c-danger: #FF9C94;
  --c-danger-bg: #000000;
  --c-success: #8FE3AA;
  --c-success-bg: #000000;
  --c-warning: #FFD27A;
  --c-warning-bg: #000000;
  --c-warning-border: #FFD27A;
  --c-info: #A3D2FF;
  --c-info-bg: #000000;
  --c-demo: #D9CCFF;
  --c-demo-bg: #000000;
  --c-test: #FFDA66;
  --c-test-bg: #000000;
  --c-overlay: rgb(0 0 0 / 0.8);
  --shadow-1: none;
  --shadow-2: none;
  --border-width: 2px;
}
/* @end */

/* CSS-only fallback when the boot script did not run: follow the system
   dark preference. Values must equal the "dark" block (contrast.py checks). */
/* @scheme dark-fallback */
@media (prefers-color-scheme: dark) {
  :root:not([data-scheme]) {
    color-scheme: dark;
    --c-bg: #141311;
    --c-surface: #1D1B18;
    --c-sunken: #25221E;
    --c-text: #F2EEE7;
    --c-muted: #BCB4A7;
    --c-border-subtle: #35312B;
    --c-border-control: #8E8679;
    --c-accent: #7FC6BC;
    --c-accent-hover: #9BD5CD;
    --c-on-accent: #0C1D1B;
    --c-accent-soft: #1F3532;
    --c-link: #8ED2C8;
    --c-focus: #94B2FF;
    --c-user: #E2A24C;
    --c-user-bg: #2C2317;
    --c-user-text: #F0C27E;
    --c-ai: #ABA6F2;
    --c-ai-bg: #232139;
    --c-ai-text: #C4C0FA;
    --c-help: #F2917A;
    --c-help-hover: #F7AE9C;
    --c-on-help: #2B0E07;
    --c-help-bg: #36211B;
    --c-help-text: #F6AE9C;
    --c-danger: #F38E85;
    --c-danger-bg: #3A1C19;
    --c-success: #83CC9B;
    --c-success-bg: #18301F;
    --c-warning: #F1C46F;
    --c-warning-bg: #352A12;
    --c-warning-border: #D9A440;
    --c-info: #90C3F0;
    --c-info-bg: #172A3B;
    --c-demo: #C6B6F2;
    --c-demo-bg: #2A2340;
    --c-test: #F2C94C;
    --c-test-bg: #352B0E;
    --c-overlay: rgb(0 0 0 / 0.6);
    --shadow-1: 0 1px 2px rgb(0 0 0 / 0.5);
    --shadow-2: 0 8px 24px rgb(0 0 0 / 0.6);
  }
}
/* @end */

/* ---------- Display preferences (set by the user in Tampilan & aksesibilitas) ---------- */

:root[data-text-scale="115"] { font-size: 115%; }
:root[data-text-scale="130"] { font-size: 130%; }
:root[data-text-scale="150"] { font-size: 150%; }

/* "Jarak teks lebih lega" (relaxed spacing). Stays within WCAG 1.4.12 values,
   which the layout must also survive when a user stylesheet applies them. */
:root[data-spacing="relaxed"] {
  --line-height-body: 1.8;
  --line-height-ui: 1.5;
  --letter-spacing-body: 0.04em;
  --word-spacing-body: 0.08em;
  --paragraph-gap: 1.25em;
}

/* Reduced motion: the system setting applies unless the user chose "full";
   the Sela setting "reduced" applies regardless of the system.
   Durations become 0.01ms, not 0ms: a transition whose combined duration is
   zero never starts and fires no transitionend. Components must still never
   wait for transitionend or animationend to finish a state change (remove a
   sheet, finish a disclosure, reveal a reply); they set the end state directly. */
@media (prefers-reduced-motion: reduce) {
  :root:not([data-motion="full"]) {
    --duration-quick: 0.01ms;
    --duration-gentle: 0.01ms;
    --reveal-step: 0ms;
  }
}
:root[data-motion="reduced"] {
  --duration-quick: 0.01ms;
  --duration-gentle: 0.01ms;
  --reveal-step: 0ms;
}
```
<!-- END GENERATED: tokens -->
