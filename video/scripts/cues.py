#!/usr/bin/env python3
"""Writes subtitles/master.en.srt and subtitles/master.id.srt from the
narration cue sheet in docs/vo_cues.tsv (start, end, Indonesian, English)."""
import csv, os
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
rows = list(csv.reader(open(os.path.join(ROOT, "docs/vo_cues.tsv"), encoding="utf-8"), delimiter="\t"))[1:]
def ts(s):
    m, sec = s.split(":"); t = int(m) * 60 + float(sec)
    h = int(t // 3600); mm = int(t % 3600 // 60); ss = t % 60
    return f"{h:02d}:{mm:02d}:{int(ss):02d},{int(round((ss - int(ss)) * 1000)):03d}"
for col, lang in ((2, "id"), (3, "en")):
    out = [f"{i}\n{ts(r[0])} --> {ts(r[1])}\n{r[col]}\n" for i, r in enumerate(rows, 1)]
    open(os.path.join(ROOT, f"subtitles/master.{lang}.srt"), "w", encoding="utf-8").write("\n".join(out))
for r in rows:
    m0, s0 = r[0].split(":"); m1, s1 = r[1].split(":")
    d = (int(m1) * 60 + float(s1)) - (int(m0) * 60 + float(s0))
    cps = len(r[3]) / d
    if cps > 17: print(f"fast EN cue {r[0]} {cps:.1f} cps")
print(len(rows), "cues")
