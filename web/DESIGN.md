---
name: Sela
description: A calm, honest place to talk, write and check in — not therapy.
colors:
  page: "#FBFBF7"
  surface: "#FFFFFF"
  surface-quiet: "#F4F6EF"
  sunken: "#EAEFE2"
  ink: "#1C1B18"
  ink-hover: "#3A3D35"
  muted-ink: "#55584F"
  hairline: "#E3E7DA"
  control-edge: "#7B8472"
  sage-wash: "#E1EDD3"
  leaf: "#2F6A43"
  leaf-soft: "#8FC17A"
  nav-text: "#D3DAC9"
  place-home: "#D4E5C3"
  place-talk: "#CDE8DC"
  place-notes: "#F6E7B4"
  place-support: "#F4DDD3"
  place-quiet: "#E6EADF"
  on-place-muted: "#3D4338"
  user: "#9A4A35"
  user-bg: "#F6E1D8"
  user-text: "#7A3B2A"
  ai-bg: "#E6F0DC"
  ai-text: "#24543A"
  help: "#a1351e"
  help-hover: "#852a16"
  help-bg: "#fbe9e4"
  help-text: "#8c2c17"
  danger: "#a3271f"
  danger-bg: "#fce9e7"
  success: "#23683c"
  success-bg: "#e6f2e9"
  warning: "#875200"
  warning-bg: "#fcf1d9"
  warning-border: "#a86e00"
  info: "#1f5788"
  info-bg: "#e7f0f8"
  demo: "#5e4a99"
  demo-bg: "#eee9f8"
  test: "#7a5600"
  test-bg: "#fff1c7"
  mood-very-heavy: "#F0786D"
  mood-heavy: "#F49C6E"
  mood-okay: "#F6C46D"
  mood-good: "#B8DA6C"
  mood-very-good: "#88D4B9"
  mood-mixed: "#CBBBEA"
  face-ink: "#2A1F1A"
typography:
  display:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "clamp(2.125rem, 1.25rem + 3vw, 4rem)"
    fontWeight: 600
    lineHeight: 1.04
    letterSpacing: "-0.035em"
  headline:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "clamp(2rem, 1.35rem + 2.4vw, 3.5rem)"
    fontWeight: 600
    lineHeight: 1.06
    letterSpacing: "-0.035em"
  headline-compact:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "clamp(1.75rem, 1.35rem + 1.4vw, 2.5rem)"
    fontWeight: 600
    lineHeight: 1.06
    letterSpacing: "-0.035em"
  wordmark:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "1.625rem"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "-0.04em"
  title-large:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.025em"
  title:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "1.375rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.025em"
  statement:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.02em"
  title-small:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.015em"
  lead:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "clamp(1.0625rem, 0.98rem + 0.45vw, 1.3125rem)"
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: "-0.01em"
  action:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 500
    lineHeight: 1.4
    letterSpacing: "-0.01em"
  reading:
    fontFamily: "Hanken Grotesk, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "1.1875rem"
    fontWeight: 400
    lineHeight: 1.65
  body:
    fontFamily: "Hanken Grotesk, system-ui, -apple-system, Segoe UI, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.6
  nav:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 500
    lineHeight: 1.4
    letterSpacing: "-0.01em"
  label:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 500
    lineHeight: 1.4
    letterSpacing: "-0.005em"
  caption:
    fontFamily: "Lexend, Hanken Grotesk, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 500
    lineHeight: 1.4
rounded:
  tail: "6px"
  radius-1: "8px"
  radius-2: "14px"
  radius-3: "20px"
  radius-4: "28px"
  sheet: "32px"
  pill: "999px"
spacing:
  space-1: "0.25rem"
  space-2: "0.5rem"
  space-3: "0.75rem"
  space-4: "1rem"
  space-5: "1.5rem"
  space-6: "2rem"
  space-7: "3rem"
  space-8: "4rem"
  gutter: "1.125rem"
  stack-gap: "1.25rem"
components:
  button-primary:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.page}"
    typography: "{typography.action}"
    rounded: "{rounded.pill}"
    padding: "0.55rem 1.15rem"
    height: "2.75rem"
  button-primary-hover:
    backgroundColor: "{colors.ink-hover}"
    textColor: "{colors.page}"
    rounded: "{rounded.pill}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    padding: "0.55rem 1.15rem"
    height: "2.75rem"
  button-secondary-hover:
    backgroundColor: "{colors.sage-wash}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
  button-go:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.page}"
    typography: "{typography.action}"
    rounded: "{rounded.pill}"
    padding: "0.35rem 1.35rem 0.35rem 0.4rem"
    height: "3.5rem"
  button-help:
    backgroundColor: "{colors.help-bg}"
    textColor: "{colors.help-text}"
    rounded: "{rounded.pill}"
    height: "2.75rem"
  button-help-hover:
    backgroundColor: "{colors.help}"
    textColor: "{colors.surface}"
    rounded: "{rounded.pill}"
  button-quick-exit:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    height: "2.75rem"
  field:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.radius-2}"
    padding: "0.65rem 0.95rem"
    height: "2.75rem"
  place-nav:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.nav-text}"
    typography: "{typography.nav}"
    rounded: "{rounded.pill}"
    padding: "0.3125rem"
  place-nav-active:
    backgroundColor: "{colors.place-home}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
    height: "2.625rem"
  place-sheet:
    backgroundColor: "{colors.place-home}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sheet}"
    padding: "clamp(1.75rem, 1rem + 2.6vw, 3.25rem) clamp(1.5rem, 0.9rem + 2.6vw, 3.25rem)"
  activity-card:
    backgroundColor: "{colors.place-support}"
    textColor: "{colors.ink}"
    typography: "{typography.title-small}"
    rounded: "{rounded.radius-3}"
    padding: "1.1rem 1.15rem"
    height: "10.5rem"
  record-list:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.radius-3}"
    padding: "0.25rem 0.5rem"
  chip:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "0 1rem"
  chip-selected:
    backgroundColor: "{colors.sage-wash}"
    textColor: "{colors.ink}"
    rounded: "{rounded.pill}"
---

# Design System: Sela

## Overview

**Creative North Star: "Taman Napas"**

Sela is a quiet garden you step into. Every place opens on a soft tinted sheet with generously rounded corners that holds the greeting and the way in; the work below sits on a clean near-white page. The tint tells the person where they are (sage for Beranda, mint for Cerita, butter for Catatanku, blush for Dukungan) and nothing else. Actions stay in one ink voice: a black pill.

The world is flat, round and calm. Lexend, open and rounded, carries headings, buttons and navigation; Hanken Grotesk carries body copy and the person's saved words. Ornament is limited to drawn geometry: a two-leaf sprout mark, four-point sparkle stars on the sheets, and six drawn mood faces that always sit beside their words. There are no raster images; every piece of art is inline SVG and every font is self-hosted under the OFL. Nothing assesses the person: no scores, streaks, progress or before/after language.

It replaces the earlier "Sampul Buku" paperback world (cream paper, Literata serif, square cover bands, indigo actions). That world is retired.

**Key Characteristics:**
- Near-white page (`page`) with white cards (`surface`) and a pale sage quiet well (`surface-quiet`).
- Four place tints name the place on the sheet, the phone header and small tinted icon circles; they never mark actions or status.
- One ink pill (`ink`) for the primary action; deep leaf green (`leaf`) for links, focus, caret, selection and icon accents.
- Places travel in a dark ink pill navigation with a sage active segment: centred in the desktop top bar, floating at the bottom on phones.
- Help (blush pill) and Quick exit (ink-outline pill) always sit top-right, in that order.
- High contrast drops every tint, sparkle and soft shadow and turns sheets into ruled headings.

## Colors

A near-white page and ink, warmed by four pastel place tints, with one deep leaf green for wayfinding and focus.

### Primary
- **Garden Ink** (`ink`): text, the primary pill button, the navigation pill, selected tabs and settings index, checked boxes. Hover lifts to **Ink Hover** (`ink-hover`). Text on ink is `page`.
- **Deep Leaf** (`leaf`): links, focus rings, caret, the switch "on" track, info icons, timeline and fact icon glyphs. **Fresh Leaf** (`leaf-soft`) is the sprout's second leaf and the selection tint (45% mix).

### Secondary
- **Place tints** (`place-home` sage, `place-talk` mint, `place-notes` butter, `place-support` blush, `place-quiet` for Settings, About and Policies): the place sheet, the phone header above it, activity cards, continuity tiles, tinted icon circles in "more spaces", the voice stage. Text on a tint is `ink`, muted text `on-place-muted`. Help pages use `help-bg` with `help-text` as their sheet.
- **Sage Wash** (`sage-wash`): selected chips, filters, choices and language buttons; secondary-button hover; timeline and fact icon circles; the onboarding step pill.
- **Navigation** (`ink` ground, `nav-text` at rest, `place-home` active with `ink` text): the dark place pill in both its desktop and phone forms.

### Tertiary
- **Mood faces** (`mood-very-heavy` coral, `mood-heavy` apricot, `mood-okay` marigold, `mood-good` lime, `mood-very-good` seafoam, `mood-mixed` lilac) drawn with `face-ink` features. These fill the drawn faces only; each face is always beside its text label.
- **Conversation** (`user` / `user-bg` / `user-text` blush for the person's turns; `ai-bg` / `ai-text` green for Sela's).

### Neutral
- **Page** (`page`): body ground and top bar. **Surface** (`surface`): cards, lists, fields, white circular icon buttons. **Quiet** (`surface-quiet`) and **Sunken** (`sunken`): hover, empty states, footer panel, subnav track.
- **Muted Ink** (`muted-ink`): hints, metadata, kind labels. **Hairline** (`hairline`): card borders and list rows. **Control Edge** (`control-edge`): field borders.

### Semantic
- **Help** (`help`, `help-hover`, `help-bg`, `help-text`), **Danger**, **Success**, **Warning** (with `warning-border`), **Info**, plus provenance markers **Demo** and **Test**. Their meaning and placement never change between schemes; each scheme tunes the values to keep 4.5:1 text and 3:1 control contrast.

### Schemes
Light is the token set above. Dark (explicit or following the system) moves to a deep green-black ground (`#121713`, surface `#1A211B`), pale text (`#EEF2E8`), a pale sage primary pill (`#DCEBCB`) with dark text, a lighter leaf (`#A9D5A0`) and deep place tints (sage `#253425`, mint `#1E3430`, butter `#37321F`, blush `#3A2A25`). Contrast-light and contrast-dark keep their own values (pure white or black grounds, navy/periwinkle accents, yellow focus in contrast-dark), set every place tint to the page ground, turn the navigation to ground-and-text, hide sparkles and shadows, square the sheet (`--radius-sheet: 0px`) and use 2px borders. Every scheme's full role list lives in the sidecar.

**The Tint Names the Place Rule.** Place tints only say where you are. Actions keep the ink pill, status keeps its semantic colour, and nothing depends on a tint alone.

**The One Ink Voice Rule.** The primary action is always the ink pill; there is one per view, and on a tinted sheet it stays ink.

## Typography

**Display Font:** Lexend (self-hosted `lexend-latin.woff2`, variable 100–900, OFL; Hanken Grotesk and system-ui fallback)
**Body Font:** Hanken Grotesk (self-hosted `hanken-grotesk-latin.woff2`, 400–700, OFL; system-ui fallback)

**Character:** Lexend's wide, rounded letterforms give greetings and buttons a soft, friendly voice; Hanken Grotesk keeps paragraphs and saved writing plain and very readable.

### Hierarchy
- **Display** (Lexend 600, `display`, 1.04, −0.035em, max 13ch): the Beranda greeting on the sage sheet.
- **Headline** (Lexend 600, `headline`, 1.06, −0.035em, max 18ch): every other sheet title. **Headline compact** for focused rooms and sign-in/onboarding cards.
- **Wordmark** (Lexend 600, `wordmark`; steps down to `title-large` on phones): "Sela" beside the sprout.
- **Title large** (Lexend 600, `title-large`): room toolbar titles, menu sheet heading, the phone wordmark, and the Beranda greeting at 130–150% text on small phones.
- **Title** (Lexend 600, `title`): section headings. **Statement** (Lexend 600): footer mark, voice status and voice transcript heading. **Title small** (Lexend 600/500): h3, activity card titles, editor aside headings.
- **Lead** (Lexend 400, `lead`, 1.45, max 42ch): the sentence under a sheet title, in `on-place-muted`.
- **Action** (Lexend 500): the big "way in" button, notice titles. Standard buttons use Lexend 500 at body size.
- **Reading** (Hanken 400, 1.65, max 62ch): a saved note read back. **Body** (Hanken 400, 1.6; relaxed spacing raises it to 1.8): everything else. List previews use Hanken 500 at body size with a two-line clamp.
- **Nav** (Lexend 500): desktop place pill and prompts. **Label** (Lexend 500): field labels, list dates and kinds, chips, badges. **Caption** (Lexend 500): phone navigation labels, demo stamp, voice provider flag. Sentence case throughout.

**The Rounded Voice Rule.** Everything the person operates or is greeted with (headings, buttons, navigation, labels) speaks Lexend; paragraphs and their own words stay in Hanken Grotesk.

## Layout

One content column, at most 80rem (`--content-max`) wide and centred: the header, the daily-tool row, notices, the demo note, the page and the footer all share its edges. Side padding (`--page-pad`) is 2.5rem from 75em, 1.5rem from 48em and the 1.125rem `gutter` on phones. Narrow pages (check-in, notes, settings) centre in that column. Reading measure is 38rem.

Top bar (≥64em): sprout wordmark left, the dark place pill centred, Help + Quick exit + round Menu right; signed-in workspaces add one row of daily-tool pills beneath it that never wraps: it scrolls sideways and fades at the right edge when space runs out. Below 64em the places move into a floating ink pill at the bottom (min(28rem, 100% − 1.5rem) wide), which becomes a rounded block at ≤24em or at 130–150% text.

Each non-focused page opens on its place sheet, 2.5rem above the content. When the sheet is wider than 52rem its lead and actions sit beside the title (`1.15fr / 1fr`). On phones (<48em) the header shares the sheet's tint and the sheet runs edge to edge with only its lower corners rounded. Focused writing and listening rooms keep a small tinted back pill instead of a sheet. On phones the conversation room scrolls as a page and the composer stays pinned to the bottom of the screen (static at 130–150% text).

Beranda (≥60em): sheet split `1fr / 1.05fr` (greeting and way in left; two activity cards right), then the six-face mood row (`0.8fr / 1.6fr`), then the page `1.5fr / 1fr` (continue + recent records left, more spaces right). Mood faces drop to three per row below 48em; activity cards stack at ≤24em and reflow at 130–150% text. Nothing scrolls horizontally at 320px.

Spacing follows the `space-1`…`space-8` steps; section groups sit 2.5rem apart and a heading sits closer to what it introduces (0.6rem) than to what came before.

## Elevation & Depth

Mostly flat. Depth comes from the tint of the sheet against the near-white page and from white cards on the page. Soft shadows are reserved for things that float or invite a press: `shadow-1` on the navigation pill, white circular icon buttons, back pill, composer and menu button; `shadow-2` on the floating phone navigation, menus, dialogs and activity-card hover; a small ink lift under primary buttons. High contrast removes all of them.

**The Float Rule.** A shadow means "this floats or responds to your touch". Static cards and lists get a hairline border, never a shadow.

## Shapes

Round everywhere. Sheets use the largest radius (`sheet`); large cards, editors, dialogs and the footer use `radius-4`; lists, activity cards, notices and tiles `radius-3`; fields, choice rows and prompts `radius-2`; checkboxes and the person's speech-bubble tail `tail`. Every button, chip, tab, filter, badge and navigation segment is a full pill; icon buttons, mood faces and icon wells are circles. Borders are 1px hairlines, 1.5px for outlined pills, 2px for selected choices and in high contrast. The only decorative silhouettes are the sprout, the four-point sparkle and the drawn faces.

## Components

### Buttons
- **Shape:** full pill, 2.75rem minimum target, Lexend 500.
- **Primary:** ink fill with page-coloured text and a soft ink lift; hover deepens and rises 1px.
- **Secondary:** white with a 1.5px ink outline; hover fills with sage wash. On a sheet it stays ink-outlined over a 70% white wash.
- **Way in (go):** a taller ink pill with a sage circle holding the icon on the left and three chevrons on the right that slide 4px on hover.
- **Quiet:** leaf-green underlined text; hover gets a sage wash.
- **Help (top bar):** blush pill with help text and a 1.5px help-tinted edge; hover and current fill with `help`. **Quick exit:** white pill with an ink outline; hover inverts. **Menu:** white circle with a hairline and `shadow-1`.
- **Focus:** 3px focus-colour outline with 3px offset, following the pill's radius.

### Chips
- **Style:** white pills with a hairline; Lexend label size.
- **State:** selected chips, filters and choices take a 2px ink edge on sage wash; tag chips show a check icon instead of a radio circle.

### Cards / Containers
- **Corner Style:** `radius-3` for lists and tiles, `radius-4` for editors, records, check-in sheets and sign-in cards.
- **Background:** white on the page; tinted cards only for place-led tiles (activity cards, continuity tiles, support paths, the editor's butter prompt aside).
- **Shadow Strategy:** none at rest (see Elevation & Depth); activity cards lift on hover.
- **Border:** 1px hairline.
- **Internal Padding:** 1.25–1.75rem (clamped).

### Inputs / Fields
- **Style:** white wells, 1px control edge, `radius-2`, leaf caret, muted placeholder.
- **Focus:** border turns leaf with a 3px soft focus halo plus a 1px focus ring.
- **Error / Disabled:** errors keep the danger border plus a message; disabled buttons go dashed and muted.

### Navigation
- **Place pill:** ink ground, pill segments in Lexend nav size, `nav-text` at rest, sage active segment with ink text; icons hide between 64em and 80em. On phones it floats at the bottom with icon above a caption label.
- **Subnav tabs:** pill track in `surface-quiet`; current tab is ink with page text.
- **Daily-tool row (desktop):** white hairline pills; current takes the sage place tint.

### Place Sheet (signature)
The place tint with `sheet` corners, a cluster of white sparkle stars top-right (two twinkle gently when motion is allowed), the title, lead and actions. Back is a white pill at its top. Sign-in and onboarding cards wear the sheet as their rounded top. In high contrast and forced colours it becomes a ruled heading; in print the tint is dropped.

### Activity Card (signature)
A tinted tile (blush for "write a note", mint for "check how you feel") with the title top-left, a white circular arrow button top-right that turns 45° on hover, and a drawn object or overlapping mood faces in the lower right. Lifts 3px with `shadow-2` on hover.

### Mood Faces (signature)
Six drawn faces in pastel circles with `face-ink` features, each inside a link or choice that always shows its word. They sit in a quiet circular well, rise on hover, and in high contrast become outlined faces on the ground colour.

### Lists
Records and timelines are rounded white cards with hairline rows: date and kind in muted Lexend label, preview in Hanken 500, a quiet circular arrow at the end; hover tints the row.

### Mark
A two-leaf sprout (`leaf` and `leaf-soft`) on a sage circle (white in the top bar). It is the wordmark's companion and the empty-state mark.

## Do's and Don'ts

### Do:
- **Do** open each non-focused page on its place sheet, title in Lexend, with Help and Quick exit top-right.
- **Do** keep one ink pill primary action per view, also on tinted sheets.
- **Do** show every mood face beside its text label, and keep colour from carrying meaning alone.
- **Do** keep help, danger, success and warning in the same roles in every scheme, at 4.5:1 text and 3:1 controls and focus.
- **Do** let high contrast drop tints, sparkles and shadows and rule the sheet instead.
- **Do** draw ornament as inline SVG geometry (sprout, sparkle, faces) and self-host OFL fonts.
- **Do** label synthetic data and unavailable features plainly.

### Don't:
- **Don't** use place tints for actions, statuses or content categories.
- **Don't** put a kicker, date or all-caps eyebrow above a heading.
- **Don't** add streaks, points, scores, badges or progress visuals.
- **Don't** put shadows on static cards or nest cards inside cards.
- **Don't** add raster illustrations, mascots or icon fonts.
- **Don't** reintroduce the retired Sampul Buku serif-and-cover look or the cobalt open-S identity.
