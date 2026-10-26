#!/usr/bin/env python3
"""Compute WCAG 2.2 contrast ratios for the Sela v3 colour roles.

Reads the colour blocks of docs/design/tokens.css (between "@scheme <name>"
and "@end" comments), computes every foreground/background pair that the
design uses, and prints Markdown tables or fails on any pair below its
minimum.

Usage (works now, standard library only):
    python3 docs/design/tools/contrast.py            # summary, exit 1 on any failure
    python3 docs/design/tools/contrast.py --md       # contrast tables (Markdown)
    python3 docs/design/tools/contrast.py --roles    # colour role table (Markdown)

Formula: WCAG 2.2 relative luminance and contrast ratio
(https://www.w3.org/TR/WCAG22/#dfn-relative-luminance), sRGB channel
threshold 0.04045. Minimums: 4.5:1 for text (SC 1.4.3; Sela also holds
large text to 4.5:1), 3:1 for user-interface components, focus indicators
and meaningful graphics (SC 1.4.11).
"""
import pathlib
import re
import sys

TOKENS = pathlib.Path(__file__).resolve().parent.parent / "tokens.css"
SCHEMES = ["light", "dark", "contrast-light", "contrast-dark"]

# (foreground role, background role, minimum ratio, where it is used)
TEXT = [
    ("text", "bg", 4.5, "Body text on the page"),
    ("text", "surface", 4.5, "Body text on cards, dialogs and inputs"),
    ("text", "sunken", 4.5, "Text in the composer area and inset panels"),
    ("text", "accent-soft", 4.5, "Label of a selected choice chip"),
    ("text", "user-bg", 4.5, "Text inside a 'Kata-katamu' (your words) block"),
    ("text", "ai-bg", 4.5, "Text inside an AI summary or 'Sela menangkap…' block"),
    ("text", "help-bg", 4.5, "Text in the crisis support panel"),
    ("text", "danger-bg", 4.5, "Text in an error notice"),
    ("text", "success-bg", 4.5, "Text in a success notice"),
    ("text", "warning-bg", 4.5, "Text in a warning notice"),
    ("text", "info-bg", 4.5, "Text in an info notice and the guest banner"),
    ("text", "demo-bg", 4.5, "Text in the demo banner"),
    ("text", "test-bg", 4.5, "Text in the test-double banner"),
    ("muted", "bg", 4.5, "Secondary text and timestamps on the page"),
    ("muted", "surface", 4.5, "Hints, meta lines and placeholder text on cards and inputs"),
    ("muted", "sunken", 4.5, "Hints in the composer area"),
    ("muted", "user-bg", 4.5, "Meta line inside a 'your words' block"),
    ("muted", "ai-bg", 4.5, "Meta line inside an AI block"),
    ("link", "bg", 4.5, "Links (always underlined) on the page"),
    ("link", "surface", 4.5, "Links on cards"),
    ("link", "sunken", 4.5, "Links in the composer area"),
    ("on-accent", "accent", 4.5, "Primary button label"),
    ("on-accent", "accent-hover", 4.5, "Primary button label, hover and pressed"),
    ("accent", "surface", 4.5, "Secondary button label (outlined)"),
    ("accent", "bg", 4.5, "Secondary button label on the page"),
    ("on-help", "help", 4.5, "'Butuh bantuan sekarang' and 'Lihat semua bantuan' link label (Help variant)"),
    ("on-help", "help-hover", 4.5, "Help-variant label, hover and pressed"),
    ("surface", "text", 4.5, "Label of the ink-filled destructive confirm button (the confirm step of delete, forget, revoke)"),
    ("surface", "muted", 4.5, "Same label, pressed state (fill --c-muted)"),
    ("user-text", "user-bg", 4.5, "'Kata-katamu' provenance label on its tint"),
    ("user-text", "surface", 4.5, "'Kata-katamu' provenance label on a card"),
    ("ai-text", "ai-bg", 4.5, "'Tulisan AI' / 'Tafsiran Sela' label on its tint"),
    ("ai-text", "surface", 4.5, "AI provenance label on a card"),
    ("help-text", "help-bg", 4.5, "Crisis panel heading"),
    ("help-text", "surface", 4.5, "Help link text on cards"),
    ("help-text", "bg", 4.5, "Help link text on the page"),
    ("danger", "surface", 4.5, "Destructive button label (outlined) and field error text on cards"),
    ("danger", "bg", 4.5, "Field error text on the page"),
    ("danger", "danger-bg", 4.5, "Error notice heading"),
    ("success", "surface", 4.5, "Success status text on a card"),
    ("success", "success-bg", 4.5, "Success notice heading"),
    ("warning", "surface", 4.5, "Warning status text on a card"),
    ("warning", "warning-bg", 4.5, "Warning notice heading"),
    ("info", "surface", 4.5, "Info status text on a card"),
    ("info", "info-bg", 4.5, "Info notice heading"),
    ("demo", "demo-bg", 4.5, "Demo banner label"),
    ("test", "test-bg", 4.5, "Test-double banner label"),
    ("muted", "help-bg", 4.5, "Service-card source and verification line inside the crisis panel"),
    ("help-text", "help-bg", 4.5, "Help-quiet label: 'Butuh bantuan sekarang' in the top bar, the help strip and full-height sheets"),
]

# Controls and links also sit on the tints: reflection-card buttons on ai-bg,
# crisis-panel channel links and buttons on help-bg, quote source links on
# user-bg, and links or buttons inside notices and banners. Every tint is
# checked for link text, secondary-button labels, the focus ring and control
# borders, so a colour change that breaks one of them fails CI.
TINTS = ["ai-bg", "help-bg", "user-bg", "danger-bg", "success-bg", "warning-bg", "info-bg", "demo-bg", "test-bg"]
TEXT += [("link", t, 4.5, f"Link text inside a block tinted --c-{t}") for t in TINTS]
TEXT += [("accent", t, 4.5, f"Secondary button label inside a block tinted --c-{t}") for t in TINTS]
UI = [
    ("border-control", "bg", 3, "Input, chip, checkbox and radio boundary on the page"),
    ("border-control", "surface", 3, "Input, chip, checkbox and radio boundary on a card"),
    ("border-control", "sunken", 3, "Composer text area boundary"),
    ("accent", "bg", 3, "Primary button shape, checked checkbox and radio fill, selected-chip border"),
    ("accent", "surface", 3, "Same, on cards"),
    ("accent", "accent-soft", 3, "Selected-chip border against its own fill"),
    ("focus", "bg", 3, "Focus ring against the page (2.4.7; 2.4.13 change of contrast)"),
    ("focus", "surface", 3, "Focus ring against cards and dialogs"),
    ("focus", "sunken", 3, "Focus ring in the composer area"),
    ("user", "bg", 3, "'Your words' rule (4px) on the page"),
    ("user", "surface", 3, "'Your words' rule on a card"),
    ("user", "user-bg", 3, "'Your words' rule against its tint"),
    ("ai", "bg", 3, "AI dashed or dotted outline on the page"),
    ("ai", "surface", 3, "AI outline on a card"),
    ("ai", "ai-bg", 3, "AI outline against its tint"),
    ("help", "bg", 3, "Help button shape on the page"),
    ("help", "surface", 3, "Help button shape on a card"),
    ("danger", "bg", 3, "Destructive button border and invalid-field border on the page"),
    ("danger", "surface", 3, "Invalid-field border on a card"),
    ("warning-border", "bg", 3, "Warning notice and status chip border"),
    ("warning-border", "surface", 3, "Warning chip border on a card"),
    ("success", "bg", 3, "Success chip border"),
    ("info", "bg", 3, "Info chip border"),
    ("demo", "bg", 3, "Demo banner border"),
    ("test", "bg", 3, "Test-double banner border"),
    ("text", "bg", 3, "Icons that carry meaning (drawn in the text colour)"),
    ("muted", "bg", 3, "Secondary icons that carry meaning (decorative ones are exempt)"),
    ("help-hover", "bg", 3, "Help-variant shape, hover and pressed, on the page"),
    ("help-hover", "surface", 3, "Help-variant shape, hover and pressed, on a card"),
    ("help", "help-bg", 3, "Help-variant link shape inside the crisis panel"),
    ("text", "surface", 3, "Ink-filled destructive confirm button shape on a card"),
    ("warning-border", "warning-bg", 3, "Warning notice border against its own tint"),
    ("help-text", "help-bg", 3, "Help-quiet life-buoy icon on its own fill"),
    ("help-hover", "help-bg", 3, "Help-quiet border, hover and pressed (3px), against its own fill"),
]
UI += [("focus", t, 3, f"Focus ring next to a control inside a block tinted --c-{t}") for t in TINTS]
UI += [("accent", t, 3, f"Secondary button border inside a block tinted --c-{t}") for t in TINTS]
UI += [("border-control", t, 3, f"Input or checkbox boundary inside a block tinted --c-{t}") for t in TINTS]


ROLES = [
    ("bg", "Page background (kertas, paper)"),
    ("surface", "Cards, dialogs, inputs, sheets"),
    ("sunken", "Composer area, inset panels, hover fill"),
    ("text", "Body text, headings, meaningful icons (tinta, ink)"),
    ("muted", "Secondary text: hints, timestamps, meta lines"),
    ("border-subtle", "Decorative separators only (no contrast requirement, never a control edge)"),
    ("border-control", "Edges of inputs, chips, checkboxes, radios"),
    ("accent", "Primary action fill, checked controls, selected chips, current navigation (laut, deep sea)"),
    ("accent-hover", "Primary action hover and pressed"),
    ("on-accent", "Text and icons on accent"),
    ("accent-soft", "Selected chip fill"),
    ("link", "Inline links (always underlined)"),
    ("focus", "Focus ring"),
    ("user", "Rule and icon marking the user's own words (kunyit, turmeric)"),
    ("user-bg", "Tint behind quoted user words in notes and summaries"),
    ("user-text", "'Kata-katamu' / 'Koreksimu' label text"),
    ("ai", "Outline and icon marking AI-written text (nila, indigo)"),
    ("ai-bg", "Tint behind AI wording and 'Sela menangkap…'"),
    ("ai-text", "'Tulisan AI' / 'Tafsiran Sela' label text"),
    ("help", "Help-variant fill for 'Lihat semua bantuan' in the crisis panel, the pause line and /help (mengkudu, noni red); the 2px border of Help-quiet. Red fill means help only"),
    ("help-hover", "Help-variant hover and pressed fill; Help-quiet hover border"),
    ("on-help", "Text on help"),
    ("help-bg", "Crisis support panel tint; Help-quiet fill ('Butuh bantuan sekarang' in the top bar)"),
    ("help-text", "Help headings, help links and the Help-quiet label"),
    ("danger", "Destructive actions (outlined only, never filled), field errors"),
    ("danger-bg", "Error notice tint"),
    ("success", "Success text and chip border"),
    ("success-bg", "Success notice tint"),
    ("warning", "Warning text"),
    ("warning-bg", "Warning notice tint"),
    ("warning-border", "Warning notice and chip border"),
    ("info", "Info text and chip border"),
    ("info-bg", "Info notice and guest banner tint"),
    ("demo", "Demo banner label and border"),
    ("demo-bg", "Demo banner tint"),
    ("test", "Test-double banner label and border"),
    ("test-bg", "Test-double banner tint"),
]


def parse(css: str) -> dict:
    blocks = {}
    for name, body in re.findall(r"/\*\s*@scheme\s+([\w-]+)\s*\*/(.*?)/\*\s*@end\s*\*/", css, re.S):
        blocks[name] = dict(re.findall(r"--c-([\w-]+):\s*(#[0-9A-Fa-f]{6})\s*;", body))
    return blocks


def luminance(hex_colour: str) -> float:
    h = hex_colour.lstrip("#")
    channels = [int(h[i:i + 2], 16) / 255 for i in (0, 2, 4)]
    lin = [c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4 for c in channels]
    return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2]


def ratio(a: str, b: str) -> float:
    la, lb = luminance(a), luminance(b)
    return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)


def main() -> int:
    blocks = parse(TOKENS.read_text(encoding="utf-8"))
    problems = []
    for s in SCHEMES:
        if s not in blocks:
            problems.append(f"missing scheme block {s}")
    if blocks.get("dark-fallback") != blocks.get("dark"):
        problems.append("dark-fallback block differs from dark block")
    md = []
    for s in SCHEMES:
        t = blocks.get(s, {})
        for title, rows in (("text", TEXT), ("UI components and graphics", UI)):
            md.append(f"\n#### {s}: {title}\n")
            md.append("| Foreground | Background | Ratio | Minimum | Result | Used for |")
            md.append("|---|---|---|---|---|---|")
            for fg, bg, minimum, use in rows:
                if fg not in t or bg not in t:
                    problems.append(f"{s}: role missing ({fg} or {bg})")
                    continue
                r = ratio(t[fg], t[bg])
                ok = r >= minimum
                if not ok:
                    problems.append(f"{s}: --c-{fg} on --c-{bg} is {r:.2f}:1, needs {minimum}:1")
                md.append(f"| `--c-{fg}` {t[fg]} | `--c-{bg}` {t[bg]} | {r:.2f}:1 | {minimum}:1 | {'pass' if ok else 'FAIL'} | {use} |")
    if "--roles" in sys.argv:
        print("| Role | Purpose | light | dark | contrast-light | contrast-dark |")
        print("|---|---|---|---|---|---|")
        for role, purpose in ROLES:
            vals = [blocks.get(s, {}).get(role, "missing") for s in SCHEMES]
            if "missing" in vals:
                problems.append(f"role {role} missing in a scheme")
            print(f"| `--c-{role}` | {purpose} | " + " | ".join(vals) + " |")
    elif "--md" in sys.argv:
        print("\n".join(md).lstrip("\n"))
    else:
        for s in SCHEMES:
            t = blocks.get(s, {})
            if not t:
                continue
            tmin = min(ratio(t[a], t[b]) for a, b, _, _ in TEXT if a in t and b in t)
            umin = min(ratio(t[a], t[b]) for a, b, _, _ in UI if a in t and b in t)
            print(f"{s}: {len(TEXT)} text pairs, lowest {tmin:.2f}:1; {len(UI)} UI pairs, lowest {umin:.2f}:1")
    for p in problems:
        print("PROBLEM:", p, file=sys.stderr)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
