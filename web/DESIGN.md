---
name: Sela
description: A calm, honest place to talk, write and check in — not therapy.
colors:
  ground: "#EDF0F8"
  surface: "#FFFFFF"
  surface-quiet: "#F3F5FB"
  sunken: "#E7ECF6"
  ink: "#17233A"
  muted: "#52617B"
  border-subtle: "#D3DBEA"
  border-control: "#72809B"
  cobalt: "#3657C7"
  cobalt-hover: "#2844A5"
  cobalt-soft: "#E7ECFD"
  link: "#2E4DB3"
  help: "#A1351E"
  help-bg: "#FBE9E4"
  danger: "#A3271F"
  success: "#23683C"
  warning: "#875200"
  coral: "#F49B87"
  lilac: "#C3C9EF"
  sage: "#6F8B7A"
typography:
  display:
    fontFamily: "Manrope, system-ui, sans-serif"
    fontSize: "clamp(1.75rem, 1rem + 3vw, 3.625rem)"
    fontWeight: 650
    lineHeight: 1.1
    letterSpacing: "-0.035em"
  headline:
    fontFamily: "Manrope, system-ui, sans-serif"
    fontSize: "clamp(2rem, 1.5rem + 1.5vw, 2.5rem)"
    fontWeight: 650
    lineHeight: 1.13
    letterSpacing: "-0.035em"
  title:
    fontFamily: "Manrope, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 650
    lineHeight: 1.3
    letterSpacing: "-0.025em"
  body:
    fontFamily: "Manrope, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.6
    letterSpacing: "normal"
  label:
    fontFamily: "Manrope, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "normal"
rounded:
  sm: "8px"
  md: "12px"
  lg: "16px"
  pill: "999px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
  xxl: "48px"
components:
  button-primary:
    backgroundColor: "{colors.cobalt}"
    textColor: "{colors.surface}"
    rounded: "{rounded.sm}"
    padding: "0.55rem 0.9rem"
  button-primary-hover:
    backgroundColor: "{colors.cobalt-hover}"
    textColor: "{colors.surface}"
    rounded: "{rounded.sm}"
    padding: "0.55rem 0.9rem"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.cobalt}"
    rounded: "{rounded.sm}"
    padding: "0.55rem 0.9rem"
  button-quiet:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.link}"
    rounded: "{rounded.sm}"
    padding: "0.55rem 0.5rem"
  field:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sm}"
    padding: "0.5rem 0.75rem"
  hero-panel:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.lg}"
    padding: "2.25rem"
  entry-row:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "1.125rem 0.75rem"
---

# Design System: Sela

## Overview

**Creative North Star: "The Open Fold"**

Sela is a working room, not a brochure about feeling better. White surfaces and a cool blue-grey ground keep the interface quiet; a cobalt open S and a sculptural paper-fold illustration carry the identity. The folds do not resolve into a tidy shape, which is the point: there is room for a story that is not finished.

The system is compact and legible. It uses one humanist sans, a small radius scale, hairline divisions and only two elevation levels. Coral, lilac and sage appear in the illustration, not as UI status colours. Nothing assesses the person: no scores, progress rings, streaks or before-and-after language.

**Key Characteristics:**
- Flat white reading surfaces on a cool `#EDF0F8` ground.
- Cobalt is the single action and identity colour.
- Flat surfaces separated by 1px rules; shadows only for sheets and dialogs.
- Four-place navigation on mobile; a fixed workspace rail on desktop.
- Real states, honest blocked states, and the Help / Quick exit pair always present.

## Colors

A cool, high-clarity neutral field with one saturated cobalt voice and warm illustration accents.

### Primary
- **Cobalt Signal** (`#3657C7`): primary actions, current navigation, links, focus rings and the open-S mark. Hover deepens to `#2844A5`; the soft tint `#E7ECFD` marks selected rows and quiet selected states.
- **Coral / Lilac / Sage** (`#F49B87`, `#C3C9EF`, `#6F8B7A`): illustration and identity only. They are never repurposed as success, warning or category colours.

### Neutral
- **Cool Paper** (`#EDF0F8`): page ground behind white content.
- **Clean Sheet** (`#FFFFFF`): welcome panel, forms, menu sheet, entry lists.
- **Quiet Sheet** (`#F3F5FB`) and **Sunken** (`#E7ECF6`): hover and inset states.
- **Ink** (`#17233A`): primary text.
- **Muted Ink** (`#52617B`): leads, metadata, hints and secondary copy.
- **Hairline** (`#D3DBEA`): dividers; **Control Edge** (`#72809B`) for input and secondary-button borders.

### Semantic
- **Help** (`#A1351E` on `#FBE9E4`) for crisis and help affordances.
- **Danger** (`#A3271F`), **Success** (`#23683C`), **Warning** (`#875200`) follow the same light-tint pattern.

**The One Voice Rule.** Cobalt is the only action and identity accent. The accepted dashboard invitation may carry the accent as a surface; surrounding reading and feature areas stay quiet.

### Well-being palettes (2026-10-28)
The person chooses a colour mood under Display & accessibility (`prefs.palette`, stored with the other non-sensitive display choices and applied before first paint by `boot.js` as `data-palette`). `src/v3/palettes.css` remaps only the `--c-*` roles of the light and dark schemes and softens radii to 10 / 14 / 20px; high-contrast schemes keep their own values. Each palette keeps one accent voice, and every text pair passes 4.5:1 and controls/focus 3:1 in both schemes.

| Palette | Mood | Light ground / accent | Dark accent |
| --- | --- | --- | --- |
| **Embun** (default) | Morning dew, restorative | `#EDF3EE` / `#2E6A52` | `#8ECBAD` |
| **Samudra** | Calm sea, slow breath | `#EAF2F3` / `#1C636D` | `#7ECDD6` |
| **Teduh** | Evening shade, rest | `#F0EEF5` / `#594994` (replies take sage `#3D6A5F`) | `#BAADF1` |
| **Pasir** | Warm sand, grounded | `#F4EFE7` / `#2D4B6C` | `#9CBADC` |
| **Lipat** | Original cobalt identity | `#EDF0F8` / `#3657C7` | `#A4B8FF` |

Help, danger, success and warning stay identical across palettes so safety affordances never change meaning. In dark schemes the Home invitation sinks into `--c-accent-soft` instead of a pale accent slab.

## Typography

**Display Font:** Manrope (self-hosted `manrope-latin.ttf`, with system-ui fallback)
**Body Font:** Manrope (same file and fallback)

**Character:** A humanist sans with open counters and a slightly technical edge. Headlines are tight and confident; body copy stays plain and roomy.

### Hierarchy
- **Display** (650, `clamp(1.75rem, 1rem + 3vw, 3.625rem)`, 1.1): Intro thesis and other display moments.
- **Headline** (650, `clamp(2rem, 1.5rem + 1.5vw, 2.5rem)`, 1.13): page titles.
- **Title** (650, 1.5rem, 1.3): section headings and record titles.
- **Body** (400, 1rem, 1.6): reading copy, 65–75ch measure, `text-wrap: pretty`.
- **Label** (600, 0.875rem, normal, sentence case): field labels, menu group labels and metadata. No all-caps kickers above headings.

The scale has five fixed steps (0.875, 1, 1.125, 1.5, 1.75rem) plus two responsive ramps: page headline 2–2.5rem and display 1.75–3.625rem. At 320px with text scale 130–150%, display caps at 1.75rem.

### Named Rules
**The No Kicker Rule.** A heading carries its own weight. Do not place an eyebrow, date, section number or category label above it.

## Layout

The shell is a 90rem maximum canvas with 2.5rem desktop gutters, narrowing to 1.5rem and then `1.125rem` on phones. Reading content uses a 38rem measure; settings use a 12rem section rail plus a flexible column.

The public entry (`/` and `/dashboard`) is an information portal before login, separate from the personal Home page. It uses a compact title/action row, a ruled facts strip, and a `2.3fr / 1fr` information/sidebar grid with 1.5rem gaps. The main area contains a soft-accent introduction, six feature rows in two semantic groups, and three reading topics; the sidebar explains the start flow, data choices and service limits. At48em it becomes one column, and at35em feature groups and reading topics also stack. The public portal loads no personal records, shows no personal usage statistics, and routes its main action into the normal age/guest/account/consent flow. Authenticated, non-focused workspaces use a fixed 15rem left rail at 64em and wider; the content flow offsets by 15rem, with 2rem gutters and a context/help/Quick exit header. The journal, weekly reflection, memory, summaries, reading and settings share this frame. Focused writing and listening retain their existing room frame.

The accepted Home composition opens with a compact white title/greeting panel followed by a `1.6fr / 1fr` split at 54em: a cobalt invitation with talk/note/check-in actions, and a continuation rail containing weekly/summary links plus a ruled feature index. Real continuation and saved-record sections follow; loading, retry, empty, retention and age states remain real. The split becomes one column below 54em. Invitation padding is 2rem, grouping gaps 1.5–2rem; panels use 12–16px radii. Home has no illustration, activity score or mission completion claim. Actions stack full-width on phones.

The four-place bottom navigation stays on mobile, switching to 2×2 at 320px or at 130–150% on narrow phones. Settings keep their section index and place the reading/form column on a white surface. Journal rows sit on one shared white sheet; editors align with the content start. Support keeps the original fold illustration beside its summary invitation, then uses two resource columns and a full-width help section on desktop. Reading separates the title/search introduction from ruled publisher rows.

Continuity keeps the existing source/editor and calendar primitives: summary creation has a `0.85fr / 1.4fr` split; weekly records and reflection use equal columns. At 48em these become one column. The seven-day strip reflows to four columns without horizontal scrolling. Reading uses a 68rem maximum width. Consent panels, editors and reflection forms use 1.5rem padding, reducing to 1.25rem at 48em; the summary reading sheet uses 1.75rem before that reduction.

A separate development-only preview starts on the public information portal with no session profile. Its Dashboard–Home navigation switches views in tab memory; Home then uses clearly labelled synthetic in-memory records. Other product actions are disabled, no account is created, and no API or AI request is made. Only the personal workspace notice offsets with the desktop rail. The preview HTML is excluded from the default production entry.

Spacing follows a 4 / 8 / 16 / 24 / 32 / 48 scale. Tight groups sit close; sections are separated by 2–2.5rem and a hairline.

## Elevation & Depth

The interface is flat at rest. Depth comes from surface change and 1px borders, not from stacked cards. Two shadows exist for overlays only: `0 1px 2px rgb(23 35 58 / 0.08)` for a slight lift and `0 8px 24px rgb(23 35 58 / 0.18)` for the menu sheet and dialogs. High-contrast schemes remove both shadows and thicken borders to 2px.

**The Flat-By-Default Rule.** If a surface is not floating above the page, it gets a rule or a tint, never a shadow.

## Shapes

Corners are restrained: 8px for controls, 12px for entry rows and compact panels, 16px for the welcome panel and menu sheet. Pills are reserved for small status chips and the language switch. Borders are 1px; selected or important controls may use 2px. No cut corners, blobs or decorative geometry.

## Components

### Buttons
- **Shape:** 8px radius, minimum 44px target, `0.55rem 0.9rem` padding.
- **Primary:** cobalt ground, white text; hover deepens the cobalt.
- **Secondary:** white ground, 2px cobalt border, cobalt text; hover uses the soft cobalt tint.
- **Quiet:** transparent, underlined link colour; used for the second action in a row.
- **Help:** the semantic help colour; reserved for “Butuh bantuan sekarang”.

### Chips
- **Style:** 0.875rem, pill or 4px radius, tinted background plus a matching text colour; icons accompany state, never colour alone.

### Cards / Containers
- **Corner Style:** 12–16px only for genuine panels; list rows use 12px with hairline separators.
- **Background:** white on the cool ground; quiet and sunken tints for hover.
- **Shadow Strategy:** see Elevation; no shadow on static cards.
- **Border:** 1px `#D3DBEA`; avoid border plus wide shadow on the same card.
- **Internal Padding:** 1–1.5rem; never nest a card inside an equal card.

### Inputs / Fields
- **Style:** white ground, 1px control edge, 8px radius, 44px minimum height.
- **Focus:** 3px cobalt outline with 2px offset.
- **Error:** danger border plus inset 1px danger ring; the message names the problem and recovery.

### Navigation
- **Desktop workspace (≥64em):** fixed 15rem white rail with the open-S mark and Sela wordmark, four places, daily tools and personal settings; current links use the soft cobalt tint. Context, Help and Quick exit stay in the top header. Public and focused screens keep their existing header.
- **Mobile:** two-row header (brand + Menu; Help + Quick exit) and a fixed four-place bottom bar with icons and labels.
- **Menu sheet:** 30rem wide, scrollable, Help / Quick exit first, then places, settings, language and About.

### Signature: Open Fold
The public information portal uses typography and structured information rather than the earlier large illustration hero. The open-S mark carries the shared identity. Support retains the original paper-fold illustration beside the manual-summary invitation, decorative (`alt=""`) and never implying an AI persona or clinical outcome. Personal Home uses the accepted cobalt invitation and feature rail described in Layout.

### Continuity surfaces
- **Memory:** a white consent panel when off; when on, a ruled state row, white editor and separate ruled entries with edit/delete controls. Personal-memory consent is separate and off by default. Only explicit user writing is saved; no automatic extraction or AI injection. Withdrawal hides normal reads while owner export/deletion remains possible.
- **Weekly:** a local Monday–Sunday strip marks only whether writing exists, with text as well as tint. Records use creation dates, or a check-in's chosen day, rather than edit timestamps. The adjacent optional reflection saves as an ordinary note; there is no score or inferred conclusion.
- **Manual summaries:** choose 1–12 current owned note, check-in or conversation versions. The browser copies user-authored conversation turns only into editable wording. Search and “show more” keep the source list manageable. Goal, questions and things to bring sit in an optional disclosure. The detail view separates wording, source references, exact-version approval, download and follow-up with rules. A source or wording change invalidates approval; changed sources show a warning and rebuild action. Download requires approval and a fresh server check. Follow-ups are ordinary editable/deletable owner records tied to the summary; deleting the summary removes its follow-ups.
- **Support and reading:** three ruled Support paths lead to preparation/follow-up, official reading sources and human-help services. `/support/prepare` and `/support/after` open the real summary list; recipient sharing and `/support/shares` remain explicitly unavailable. Reading is a searchable catalogue of four official WHO/NHS links opening with no referrer. It does not release the editorially gated Sela exercise cards.

**Implementation checkpoint (2026-10-28):** these are local, uncommitted workspace implementations, not a live release, user-research result or clinical approval. The incumbent tokens and chosen visual world remain unchanged.

## Do's and Don'ts

### Do:
- **Do** keep the page ground cool and the welcome surface white.
- **Do** use cobalt for exactly one primary action per view.
- **Do** self-host Manrope and keep text resize, dark mode and high contrast working.
- **Do** state unavailable features and provider limits plainly.
- **Do** keep Help and Quick exit in the same header position on every screen.

### Don't:
- **Don't** reintroduce a serif, paper texture or warm editorial costume.
- **Don't** put a kicker, date or all-caps eyebrow above a heading.
- **Don't** add streaks, points, scores, badges, rating stars or healing promises.
- **Don't** use coral, lilac or sage for UI status; they belong to the illustration.
- **Don't** nest cards, add gradient text/glass, or mix a border with a wide shadow.
- **Don't** present AI as a clinician or imply automatic memory, recipient sharing, appointments or provider readiness that the build does not support.
