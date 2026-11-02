#!/usr/bin/env python3
"""
Original music bed and UI sound design for the 3-minute product demo.

Same synthesis approach as scripts/score.py (oscillators + filtered noise, no
samples), but arranged as a calm bed under narration: chapter chords, a soft
tick on every recorded click, hits on chapter titles, the MIE pulse under the
graph insert, and the end-card signature. Event times come from
.cache/demo-plan.json (run scripts/demo-plan.mjs first).

Output: public/audio/score-demo.wav (48 kHz, 24-bit), normalised to -18 LUFS
so a dub can sit on top (scripts/dub.py mixes the final at about -16 LUFS).
"""
import json
import os
import re
import subprocess

import numpy as np
from scipy import signal
from scipy.io import wavfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PLAN = json.load(open(os.path.join(ROOT, ".cache/demo-plan.json")))
CH3 = json.load(open(os.path.join(ROOT, "src/timeline.json")))["cues"]["ch03"]
CH8 = json.load(open(os.path.join(ROOT, "src/timeline.json")))["cues"]["ch08"]
SR, FPS = 48000, PLAN["fps"]
N = int(5400 / FPS * SR)
rng = np.random.default_rng(20261103)
L, R, SEND = np.zeros(N), np.zeros(N), np.zeros(N)
seg = {s["id"]: s for s in PLAN["segments"]}
OUTRO_OFFSET = 130  # keep in sync with src/demo/DemoFilm.tsx
GRAPH_OFFSET = 640


def sec(f):
    return f / FPS


def note(n):
    names = {"C": 0, "C#": 1, "D": 2, "D#": 3, "E": 4, "F": 5, "F#": 6, "G": 7, "G#": 8, "A": 9, "A#": 10, "B": 11}
    return 440.0 * 2 ** ((names[n[:-1]] + 12 * (int(n[-1]) + 1) - 69) / 12)


def place(sig, t0, gain=1.0, pan=0.0, rev=0.25):
    i0 = int(t0 * SR)
    if i0 < 0:
        sig, i0 = sig[-i0:], 0
    n = min(len(sig), N - i0)
    if n <= 0:
        return
    s = sig[:n] * gain
    L[i0:i0 + n] += s * np.cos((pan + 1) * np.pi / 4)
    R[i0:i0 + n] += s * np.sin((pan + 1) * np.pi / 4)
    SEND[i0:i0 + n] += s * rev


def pad(t0, t1, notes, cutoff=900.0, gain=0.07, fi=1.5, fo=1.5):
    n = int((t1 - t0) * SR)
    if n <= 0:
        return
    t = np.arange(n) / SR
    ol, orr = np.zeros(n), np.zeros(n)
    for vi, nm in enumerate(notes):
        f0 = note(nm)
        for d, pn in ((-7, -0.5), (0, 0.0), (7, 0.5)):
            f = f0 * 2 ** (d / 1200)
            ph = rng.uniform(0, 2 * np.pi)
            v = np.zeros(n)
            k = 1
            while k * f < 7000 and k < 30:
                v += np.sin(2 * np.pi * k * f * t + ph * k) / (k * np.sqrt(1 + (k * f / cutoff) ** 4))
                k += 1
            v *= 1 + 0.08 * np.sin(2 * np.pi * (0.06 + 0.03 * vi) * t + vi)
            ol += v * np.cos((pn + 1) * np.pi / 4)
            orr += v * np.sin((pn + 1) * np.pi / 4)
    e = np.clip(np.minimum(t / fi, (t[-1] - t) / fo), 0, 1) ** 1.5
    i0 = int(t0 * SR)
    m = min(n, N - i0)
    g = gain * 1.6 / len(notes)
    L[i0:i0 + m] += ol[:m] * e[:m] * g
    R[i0:i0 + m] += orr[:m] * e[:m] * g
    SEND[i0:i0 + m] += (ol[:m] + orr[:m]) * 0.3 * e[:m] * g


def boom(strength=1.0, f_hi=90.0, f_lo=36.0, decay=1.3):
    n = int(decay * 2.2 * SR)
    t = np.arange(n) / SR
    f = f_lo + (f_hi - f_lo) * np.exp(-t * 9)
    body = np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t / decay * 2.2)
    b, a = signal.butter(2, 2500 / (SR / 2))
    click = signal.lfilter(b, a, rng.standard_normal(n) * np.exp(-t * 220)) * 0.35
    return (body + click + np.sin(2 * np.pi * f * 2 * t) * np.exp(-t * 7) * 0.25) * strength


def hit(t0, strength=1.0, bright=0.5):
    place(boom(strength), t0, 0.40, rev=0.25)
    n = int(1.5 * SR)
    t = np.arange(n) / SR
    b, a = signal.butter(2, [800 / (SR / 2), (3000 + 6000 * bright) / (SR / 2)], btype="band")
    place(signal.lfilter(b, a, rng.standard_normal(n) * np.exp(-t * 4.5)) * 0.08 * strength, t0, rev=0.6)


def tick(t0, freq=2300.0, gain=0.05, pan=0.0):
    n = int(0.1 * SR)
    t = np.arange(n) / SR
    place((np.sin(2 * np.pi * freq * t) + 0.3 * np.sin(2 * np.pi * freq * 2.01 * t)) * np.exp(-t * 70), t0, gain, pan, rev=0.3)


def pluck(t0, nm, gain=0.1, pan=0.0, decay=0.8):
    f = note(nm)
    n = int(decay * 2 * SR)
    t = np.arange(n) / SR
    s = sum(np.sin(2 * np.pi * f * k * t) * np.exp(-t * (3 + 2.5 * k) / decay) / k ** 1.3 for k in range(1, 7))
    place(s * np.minimum(1, t / 0.003), t0, gain, pan, rev=0.35)


def bell(t0, nm, gain=0.08, decay=3.5):
    f = note(nm)
    n = int(decay * 1.6 * SR)
    t = np.arange(n) / SR
    s = sum(a * np.sin(2 * np.pi * f * p * t) * np.exp(-t * (1.2 + p * 0.6) / decay * 2) for p, a in ((1, 1), (2, 0.45), (2.76, 0.32), (4.07, 0.16), (5.4, 0.1)))
    place(s * np.minimum(1, t / 0.002), t0, gain, rev=0.55)


def whoosh(t0, dur=0.8, gain=0.05):
    n = int(dur * SR)
    nz = rng.standard_normal(n)
    out = np.zeros(n)
    for i in range(20):
        a_, b_ = i * n // 20, (i + 1) * n // 20
        bb, aa = signal.butter(2, (300 + 5000 * i / 19) / (SR / 2))
        out[a_:b_] = signal.lfilter(bb, aa, nz[a_:b_])
    place(out * np.sin(np.pi * np.arange(n) / n) ** 2, t0, gain, rev=0.5)


def heartbeat(t0, s=1.0):
    for k, dt in enumerate((0.0, 0.22)):
        n = int(0.6 * SR)
        t = np.arange(n) / SR
        place(np.sin(2 * np.pi * (52 - 8 * t) * t) * np.exp(-t * 11) * (1 if k == 0 else 0.6) * s * 0.5, t0 + dt, rev=0.3)


# ---------------------------------------------------------------- arrangement
CHORDS = {
    "talk": ["F2", "C3", "E3", "A3"],
    "memory": ["D2", "A2", "F3", "E4"],
    "reach-summary": ["A1", "E2", "C3", "E3"],
    "reach-share": ["A1", "E2", "C3", "E3"],
    "reach-care": ["A1", "E2", "C3", "E3"],
    "timeline": ["C2", "G2", "B2", "D3", "E3"],
    "steps": ["G2", "D3", "E3", "B3"],
    "passport": ["F2", "C3", "G3", "A3"],
    "care": ["A1", "E2", "G2", "C3", "E3"],
    "revoke": ["F2", "C3", "A3", "E4"],
}

# intro
i0 = sec(seg["intro"]["from"])
pad(i0, sec(seg["intro"]["dur"]) + 0.6, ["A1", "E2", "B2", "C3"], cutoff=550, gain=0.08, fi=1.0, fo=0.8)
heartbeat(i0 + sec(2))
hit(i0 + sec(8), 1.0)
hit(i0 + sec(28), 0.8, 0.7)
hit(i0 + sec(112), 0.45)
hit(i0 + sec(130), 1.0, 0.9)
bell(i0 + sec(194), "C5", 0.06)

for s in PLAN["segments"]:
    if s["id"] in CHORDS:
        a, b = sec(s["from"]), sec(s["from"] + s["dur"])
        pad(a - 0.4, b + 0.6, CHORDS[s["id"]], cutoff=850, gain=0.075, fi=0.8, fo=1.0)
    if s.get("title"):
        whoosh(sec(s["from"]) - 0.4, 0.7, 0.04)
        hit(sec(s["from"] + 4), 0.7, 0.6)

for c in PLAN["clicks"]:
    tick(sec(c) + 0.02, 2200 + rng.uniform(-150, 150), 0.045)

# graph insert: pulse, correction, propagation
g = seg["graph"]
g0 = sec(g["from"])
pad(g0 - 0.3, sec(g["from"] + g["dur"]) + 0.5, ["A1", "E2", "C3", "E3"], cutoff=700, gain=0.08, fi=0.4, fo=0.8)
step = 60 / 112 / 4
seq = ["A3", "E4", "C4", "E4", "A3", "E4", "B3", "E4"]
t, i = g0, 0
while t < sec(g["from"] + g["dur"]) - 0.4:
    pluck(t, seq[i % 8], 0.06, pan=(-0.3 if i % 2 else 0.3), decay=0.5)
    if i % 4 == 0:
        heartbeat(t, 0.3)
    t += step
    i += 1
gc = lambda cue: g0 + sec(CH3[cue] - GRAPH_OFFSET)
hit(gc("correct"), 1.0, 0.9)
bell(gc("correct"), "E5", 0.06)
tick(gc("stale"), 1700, 0.08)
tick(gc("revoked"), 1300, 0.08)
tick(gc("cancelled"), 1000, 0.08)

# outro: end card from chapter 08
o = seg["outro"]
o0 = sec(o["from"])
oc = lambda cue: o0 + sec(CH8[cue] - OUTRO_OFFSET)
pad(o0 - 0.5, sec(5400), ["C2", "G2", "E3", "D4", "G4"], cutoff=1200, gain=0.09, fi=1.0, fo=2.5)
place(boom(0.7, 60, 30, 2.2), o0, 0.45, rev=0.4)
for cue, nm in (("story", "E4"), ("meaning", "G4"), ("next", "C5")):
    pluck(oc(cue), nm, 0.09, decay=1.4)
hit(oc("wordmark"), 0.8, 0.6)
bell(oc("wordmark"), "G4", 0.06, decay=4)
bell(oc("built"), "C5", 0.05, decay=5)
bell(oc("built") + 0.38, "G5", 0.045, decay=5)

# ---------------------------------------------------------------- mix
def ir(seed):
    r = np.random.default_rng(seed)
    n = int(2.6 * SR)
    t = np.arange(n) / SR
    x = r.standard_normal(n) * np.exp(-t * 3.4)
    b, a = signal.butter(1, 5500 / (SR / 2))
    x = signal.lfilter(b, a, x)
    return x / np.sqrt(np.sum(x ** 2))


ml = L + signal.fftconvolve(SEND, ir(7))[:N] * 0.5
mr = R + signal.fftconvolve(SEND, ir(11))[:N] * 0.5
b, a = signal.butter(2, 28 / (SR / 2), btype="high")
ml, mr = signal.lfilter(b, a, ml), signal.lfilter(b, a, mr)
pk = max(np.abs(ml).max(), np.abs(mr).max())
ml, mr = np.tanh(ml / pk * 1.2) * 0.89 / np.tanh(1.2), np.tanh(mr / pk * 1.2) * 0.89 / np.tanh(1.2)
fo = int(0.8 * SR)
ml[-fo:] *= np.linspace(1, 0, fo) ** 2
mr[-fo:] *= np.linspace(1, 0, fo) ** 2
out = os.path.join(ROOT, "public/audio/score-demo.wav")
os.makedirs(os.path.dirname(out), exist_ok=True)
wavfile.write(out, SR, (np.clip(np.stack([ml, mr], 1), -1, 1) * (2 ** 31 - 1)).astype(np.int32))

m = subprocess.run(["ffmpeg", "-hide_banner", "-nostats", "-i", out, "-af", "loudnorm=I=-18:TP=-2:LRA=11:print_format=json", "-f", "null", "-"], capture_output=True, text=True).stderr
js = json.loads(re.search(r"\{[^{}]*\"input_i\"[^{}]*\}", m, re.S).group(0))
tmp = out.replace(".wav", ".norm.wav")
subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", out, "-af", f"loudnorm=I=-18:TP=-2:LRA=11:measured_I={js['input_i']}:measured_TP={js['input_tp']}:measured_LRA={js['input_lra']}:measured_thresh={js['input_thresh']}:offset={js['target_offset']}:linear=true", "-ar", str(SR), "-c:a", "pcm_s24le", tmp], check=True)
os.replace(tmp, out)
print("wrote", out)
