---
name: Sela
description: A calm, honest place to talk, write and check in — not therapy.
colors:
  paper: "#F3EEE4"
  page: "#FBF8F2"
  page-quiet: "#F7F2E9"
  sunken: "#EBE3D5"
  ink: "#231C17"
  muted-ink: "#5C5047"
  hairline: "#DCD2C2"
  control-edge: "#85776A"
  indigo: "#2F3A63"
  indigo-deep: "#212A4A"
  indigo-soft: "#E8E3E8"
  cover-brick: "#B4472F"
  cover-forest: "#2E5A4C"
  cover-ochre: "#D6A23E"
  cover-indigo: "#2F3A63"
  cover-ink: "#3B322C"
  on-cover: "#FFF8F1"
  on-cover-muted: "#FBEDE6"
  help: "#A1351E"
  help-bg: "#FBE9E4"
  danger: "#A3271F"
  success: "#23683C"
  warning: "#875200"
typography:
  display:
    fontFamily: "Literata, Iowan Old Style, Georgia, serif"
    fontSize: "clamp(2.25rem, 1.2rem + 4vw, 4.75rem)"
    fontWeight: 600
    lineHeight: 0.98
    letterSpacing: "-0.03em"
  headline:
    fontFamily: "Literata, Iowan Old Style, Georgia, serif"
    fontSize: "clamp(2.25rem, 1.4rem + 3vw, 4.25rem)"
    fontWeight: 600
    lineHeight: 1.02
    letterSpacing: "-0.03em"
  title:
    fontFamily: "Literata, Iowan Old Style, Georgia, serif"
    fontSize: "1.625rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  lead:
    fontFamily: "Literata, Iowan Old Style, Georgia, serif"
    fontSize: "clamp(1.125rem, 1rem + 0.6vw, 1.5rem)"
    fontWeight: 400
    lineHeight: 1.4
  record:
    fontFamily: "Literata, Iowan Old Style, Georgia, serif"
    fontSize: "1.0625rem"
    fontWeight: 500
    lineHeight: 1.45
  body:
    fontFamily: "Hanken Grotesk, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "Hanken Grotesk, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 600
    lineHeight: 1.4
rounded:
  hairline: "2px"
  control: "4px"
  panel: "6px"
  card: "8px"
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
    backgroundColor: "{colors.indigo}"
    textColor: "{colors.page}"
    rounded: "{rounded.control}"
    padding: "0.55rem 0.9rem"
  button-primary-hover:
    backgroundColor: "{colors.indigo-deep}"
    textColor: "{colors.page}"
    rounded: "{rounded.control}"
  button-secondary:
    backgroundColor: "{colors.page}"
    textColor: "{colors.indigo}"
    rounded: "{rounded.control}"
    padding: "0.55rem 0.9rem"
  button-on-cover:
    backgroundColor: "{colors.on-cover}"
    textColor: "{colors.cover-brick}"
    rounded: "{rounded.control}"
    padding: "0.55rem 0.9rem"
  field:
    backgroundColor: "{colors.page}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "0.5rem 0.75rem"
  cover-band:
    backgroundColor: "{colors.cover-brick}"
    textColor: "{colors.on-cover}"
    padding: "3rem 0 2.75rem"
  top-bar:
    backgroundColor: "{colors.page}"
    textColor: "{colors.ink}"
    height: "4.75rem"
---

# Design System: Sela

## Overview

**Creative North Star: "Sampul Buku"**

Each place in Sela is a book the person owns. A flat cover colour, printed edge to edge like an Indonesian paperback of the 1970s and 80s, says where they are; underneath, a warm paper page holds their own words in a calm serif, divided by ink rules rather than boxes. The interface is professional the way a well-made book is: quiet, legible, sure of its structure, with colour used to orient rather than decorate.

The system is flat and print-like. One serif (Literata) carries titles, leads and the person's saved writing; one grotesk (Hanken Grotesk) carries every control, label and paragraph of instruction. Actions speak in a single deep indigo. Nothing assesses the person: no scores, progress, streaks or before/after language.

**Key Characteristics:**
- Warm paper ground (`paper`) with a lighter page surface for bars, panels and fields.
- Four cover colours as wayfinding: brick Beranda, forest Cerita, ochre Catatanku, indigo Dukungan.
- Full-bleed cover band at the top of every non-focused page; focused rooms get a 4px spine instead.
- Sections open on a 2px ink rule; rows divide on 1px hairlines.
- Help and Quick exit always in the top-right of the bar, in the same order.

## Colors

Warm paper and ink with four flat cover colours and one indigo action voice.

### Primary
- **Bound Indigo** (`indigo`): primary buttons, links, focus rings, selected tool/chip states (`indigo-soft`). Hover deepens to `indigo-deep`.

### Secondary
- **Cover colours** (`cover-brick`, `cover-forest`, `cover-ochre`, `cover-indigo`, `cover-ink`): place identity only — the cover band, the tab underline of the current place, nav spines, shelf spines, the bottom-bar marks and the four-spine logo. Text on covers is `on-cover` / `on-cover-muted`, except ochre, which takes `ink`. Settings, About and Policies use `cover-ink`; Help uses `help-bg` with help text; sign-up and onboarding use forest.

### Neutral
- **Paper** (`paper`): page ground. **Page** (`page`): top bar, tool strip, fields, panels. **Quiet page** / **Sunken**: hover and pressed.
- **Ink** (`ink`): text and section rules. **Muted ink** (`muted-ink`): metadata, kind labels, hints.
- **Hairline** (`hairline`): row dividers. **Control edge** (`control-edge`): input and secondary borders.

### Semantic
- **Help** (`help` on `help-bg`), **Danger**, **Success**, **Warning** keep the same values in every scheme so safety meaning never moves.

### Schemes
Sampul is the only look; there are no alternative colour moods. Display & accessibility offers light, dark (or follow system) and high contrast. High-contrast schemes keep their own values and turn covers into ruled headings. Dark mode deepens covers (brick `#8C3826`, forest `#25483D`, ochre `#B98A33` with dark text, indigo `#2C355A`) on a `#1A1613` ground.

**The Cover Is Not a Button Rule.** Cover colours never mark actions, status or categories of content; they only name the place. Actions stay indigo (or reverse to `on-cover` when sitting on a cover).

## Typography

**Display Font:** Literata (self-hosted `literata-latin.woff2`, variable 400–700 with optical sizing; Georgia fallback)
**Body Font:** Hanken Grotesk (self-hosted `hanken-grotesk-latin.woff2`, 400–700; system-ui fallback)

**Character:** A bookish, open serif for what the person reads and writes, paired with a plain, slightly warm grotesk for everything they operate.

### Hierarchy
- **Display** (600, `clamp(2.25rem, 1.2rem + 4vw, 4.75rem)`, 0.98): Beranda and the public introduction cover titles.
- **Headline** (600, `clamp(2.25rem, 1.4rem + 3vw, 4.25rem)`, 1.02): every other cover title; focused rooms cap at 3rem.
- **Title** (600, 1.625rem / h3 1.25rem): section headings above ink rules.
- **Lead** (Literata 400, 1.125–1.5rem, 1.4): the sentence under a cover title.
- **Record** (Literata 500, 1.0625rem, two-line clamp): previews of saved notes, check-ins and conversations in lists.
- **Body** (Hanken 400, 1rem, 1.6, 65–75ch) and **Label** (Hanken 600–700, 0.875rem, sentence case).

**The Serif Is Theirs Rule.** Literata is for titles and the person's own words; instructions, buttons and labels are always Hanken Grotesk.

## Layout

A centred 90rem canvas with 2.5rem desktop gutters (1.5rem below 75em, 1.125rem on phones). Every non-focused page opens with a full-bleed cover band (3rem / 2.75rem padding; 1.75rem on phones) that sits flush under the bar, then content follows 2.5rem below.

Desktop (≥64em) shell: a 4.75rem top bar (wordmark, four place tabs, Help, Quick exit, Menu) and, for signed-in workspaces, a 3rem tool strip with context, daily tools and personal settings (context and the "Untuk harimu" label hide below 96em). Below 64em the place tabs move to a fixed four-place bottom bar (2×2 at 320px or at 130–150% text) and the header becomes two rows: brand + Menu, then Help + Quick exit.

Beranda: cover split `1.15fr / 1fr` at ≥54em (title + greeting left; invitation, three actions and storage hint right), then a ruled page `1.45fr / 1fr` (continuation, recent records and memory left; the shelf of other rooms right). Everything stacks below 54em and actions go full width below 48em. Onboarding and sign-in cards carry their cover as the card's top edge. Reading measure stays 38–40rem; settings keep their section index.

Spacing follows 4 / 8 / 16 / 24 / 32 / 48; groups of related sections are 2.5rem apart, with more space above a heading than below it.

## Elevation & Depth

Flat. Depth is the change from cover to page and from paper to page surface, plus ink rules. The only shadows belong to floating layers: `shadow-1` for a slight lift and `shadow-2` (`0 12px 32px rgb(35 28 23 / 0.2)`) for the menu sheet and dialogs; high contrast removes both.

**The Print Rule.** If it is not floating above the page, it gets a rule or a tint, never a shadow.

## Shapes

Small, printed corners: 2px for spines, 4px for controls, 6px for panels and service cards, 8px for onboarding cards and the menu sheet. Cover bands are square and full-bleed. Pills remain only for status chips. Borders are 1px hairlines; sections open on a 2px ink rule; buttons use 1.5px strokes.

## Components

### Buttons
- **Shape:** 4px radius, 44px minimum target, 1.5px border, Hanken 700.
- **Primary:** indigo with page-coloured text; hover deepens.
- **Secondary:** page ground, indigo stroke and text.
- **On a cover:** primary reverses to `on-cover` fill with the cover colour as text; secondaries become transparent with an `on-cover` stroke; focus rings take the cover's ink.
- **Help (top bar):** help-bg fill, help stroke and text; hover fills with help. **Quick exit:** ink stroke; hover inverts.

### Navigation
- **Top bar:** four tabs, each with a 0.5rem × 1.35rem spine in its cover colour; the current place is bold with a 4px underline in its own colour.
- **Tool strip:** text links with 4px-radius hover; current link takes `indigo-soft`.
- **Bottom bar (phones):** icon + label; a 4px cover-colour mark on top, faint at rest and full width on the current place.
- **Subnav tabs:** muted text; current is ink, bold, with a 3px inset ink underline.

### Lists
- **Records and timeline:** 2px ink rule on top, hairline rows; date and kind label in muted Hanken, preview in the Literata record style; hover tints the row.
- **Shelf rows:** a 0.5rem cover-colour spine at the left of each room link, title + one-line description, arrow at the end.

### Inputs / Fields
- Page ground, 1px control edge, 4px radius, 44px minimum; 3px indigo focus ring with 2px offset; errors keep the danger border plus message.

### Signature: Cover Band
Full-bleed place colour behind the page header (`.s-page-header`, Beranda `.s-cover`, the portal heading, the public introduction and the Help emergency block). Implemented with `background-color` plus a `border-image` fill outset so it bleeds to the viewport without horizontal overflow. In high contrast and forced colours it becomes a ruled heading.

### Mark
Four book spines of different heights in the four cover colours, the last leaning; it replaces the open S and doubles as the favicon.

## Do's and Don'ts

### Do:
- **Do** open each non-focused page on its place's cover band, title in Literata.
- **Do** keep one indigo primary action per view (reversed on covers).
- **Do** divide sections with a 2px ink rule and rows with 1px hairlines.
- **Do** keep Help and Quick exit top-right on every screen, and keep text resize, dark mode and high contrast working.
- **Do** label synthetic data and unavailable features plainly.

### Don't:
- **Don't** use cover colours for actions, statuses or content categories.
- **Don't** put a kicker, date or all-caps eyebrow above a heading.
- **Don't** add streaks, points, scores, badges or progress visuals.
- **Don't** float static panels with shadows or nest cards.
- **Don't** reintroduce the retired cobalt open-S identity or alternative colour moods.
