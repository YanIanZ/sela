#!/usr/bin/env python3
"""Refresh the generated sections of docs/design/DESIGN_DIRECTION.md.

Usage (works now): python3 docs/design/tools/sync_docs.py
Replaces the text between "<!-- BEGIN GENERATED: <name> -->" and
"<!-- END GENERATED: <name> -->" with the output of contrast.py
(roles, tables) or the content of tokens.css (tokens). Exit 1 if any
contrast pair fails or a marker is missing.
"""
import pathlib
import re
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
DOC = HERE.parent / "DESIGN_DIRECTION.md"


def run(flag: str) -> str:
    out = subprocess.run([sys.executable, "-I", str(HERE / "contrast.py"), flag], capture_output=True, text=True)
    if out.returncode != 0:
        sys.stderr.write(out.stderr)
        sys.exit(1)
    return out.stdout.strip()


def main() -> int:
    parts = {
        "roles": run("--roles"),
        "tables": run("--md"),
        "tokens": "```css\n" + (HERE.parent / "tokens.css").read_text(encoding="utf-8").strip() + "\n```",
    }
    text = DOC.read_text(encoding="utf-8")
    for name, body in parts.items():
        pattern = re.compile(rf"(<!-- BEGIN GENERATED: {name} -->\n)(?:.*?\n)?(<!-- END GENERATED: {name} -->)", re.S)
        if not pattern.search(text):
            print(f"marker for {name} not found", file=sys.stderr)
            return 1
        text = pattern.sub(lambda m: m.group(1) + body + "\n" + m.group(2), text)
    DOC.write_text(text, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
