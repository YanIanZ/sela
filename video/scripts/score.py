#!/usr/bin/env python3
"""
Original score and sound design for SELA — Beyond Being Heard.

Everything is synthesised here from sine/saw oscillators and filtered noise:
no samples, no third-party audio, no MSI material. The result is original work
for this project (see docs/SOUND_DESIGN.md for licensing).

Every event is placed from src/timeline.json, so the sound follows the
typography exactly. Outputs (48 kHz, 24-bit stereo WAV):
  public/audio/score-master.wav   180 s
  public/audio/score-teaser.wav    30 s (cut from the master on the teaser edit)

usage: python3 scripts/score.py
"""
import json
import os
import subprocess
import re

import numpy as np
from scipy import signal
from scipy.io import wavfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TL = json.load(open(os.path.join(ROOT, "src/timeline.json")))
SR = 48000
FPS = TL["fps"]
DUR = TL["durationInFrames"] / FPS
N = int(DUR * SR)
rng = np.random.default_rng(20261102)

L = np.zeros(N)
R = np.zeros(N)
REV_SEND = np.zeros(N)  # mono send into the shared reverb


def chap(cid):
    return next(c for c in TL["chapters"] if c["id"] == cid)


def at(cid, cue):
    """global seconds of a cue"""
    return (chap(cid)["from"] + TL["cues"][cid][cue]) / FPS


def sec(cid, local):
    return (chap(cid)["from"] + local) / FPS


def note(name):
    names = {"C": 0, "C#": 1, "D": 2, "D#": 3, "E": 4, "F": 5, "F#": 6, "G": 7, "G#": 8, "A": 9, "A#": 10, "B": 11}
    p, o = name[:-1], int(name[-1])
    return 440.0 * 2 ** ((names[p] + 12 * (o + 1) - 69) / 12)


def place(sig, t0, gain=1.0, pan=0.0, rev=0.2):
    i0 = int(t0 * SR)
    if i0 >= N:
        return
    if i0 < 0:
        sig = sig[-i0:]
        i0 = 0
    n = min(len(sig), N - i0)
    s = sig[:n] * gain
    lg = np.cos((pan + 1) * np.pi / 4)
    rg = np.sin((pan + 1) * np.pi / 4)
    L[i0:i0 + n] += s * lg
    R[i0:i0 + n] += s * rg
    REV_SEND[i0:i0 + n] += s * rev


def env_adsr(n, a, r, curve=3.0):
    t = np.arange(n) / SR
    e = np.minimum(1.0, t / max(a, 1e-4))
    tail = np.clip((n / SR - t) / max(r, 1e-4), 0, 1) ** curve
    return e * np.minimum(1.0, tail * 1.0 + (t < n / SR - r))


# ---------------------------------------------------------------- pads
def pad(t0, t1, notes, cutoff=1400.0, gain=0.06, fade_in=2.5, fade_out=2.5, bright_to=None, pan_spread=0.5):
    """Additive band-limited detuned-saw pad (no aliasing, smooth)."""
    n = int((t1 - t0) * SR)
    t = np.arange(n) / SR
    out_l = np.zeros(n)
    out_r = np.zeros(n)
    cut = np.full(n, cutoff) if bright_to is None else np.linspace(cutoff, bright_to, n)
    for vi, nm in enumerate(notes):
        f0 = note(nm) if isinstance(nm, str) else nm
        for d, pn in ((-7, -pan_spread), (0, 0.0), (7, pan_spread)):
            f = f0 * 2 ** (d / 1200)
            ph = rng.uniform(0, 2 * np.pi)
            v = np.zeros(n)
            k = 1
            while k * f < 9000 and k < 40:
                resp = 1.0 / np.sqrt(1 + (k * f / cut) ** 4)  # 2-pole style roll-off
                v += np.sin(2 * np.pi * k * f * t + ph * k) * resp / k
                k += 1
            lfo = 1 + 0.08 * np.sin(2 * np.pi * (0.07 + 0.03 * vi) * t + vi)
            v *= lfo
            out_l += v * np.cos((pn + 1) * np.pi / 4)
            out_r += v * np.sin((pn + 1) * np.pi / 4)
    e = np.minimum(1, t / fade_in) * np.minimum(1, (t[-1] - t) / fade_out + 1e-9)
    e = np.clip(e, 0, 1) ** 1.5
    i0 = int(t0 * SR)
    m = min(n, N - i0)
    norm = gain * 1.6 / max(1, len(notes))
    L[i0:i0 + m] += out_l[:m] * e[:m] * norm
    R[i0:i0 + m] += out_r[:m] * e[:m] * norm
    REV_SEND[i0:i0 + m] += (out_l[:m] + out_r[:m]) * 0.5 * e[:m] * norm * 0.6


# ---------------------------------------------------------------- one-shots
def boom(strength=1.0, f_hi=90.0, f_lo=36.0, decay=1.4):
    n = int(decay * 2.2 * SR)
    t = np.arange(n) / SR
    f = f_lo + (f_hi - f_lo) * np.exp(-t * 9)
    ph = 2 * np.pi * np.cumsum(f) / SR
    body = np.sin(ph) * np.exp(-t / decay * 2.2)
    click = rng.standard_normal(n) * np.exp(-t * 220) * 0.35
    b, a = signal.butter(2, 2500 / (SR / 2))
    click = signal.lfilter(b, a, click)
    # a little upper body so it reads on laptop speakers
    body2 = np.sin(2 * np.pi * f * 2 * t) * np.exp(-t * 7) * 0.25
    return (body + click + body2) * strength


def hit(t0, strength=1.0, pan=0.0, bright=0.5):
    place(boom(strength), t0, 0.40, pan, rev=0.25)
    # airy noise layer
    n = int(1.6 * SR)
    t = np.arange(n) / SR
    nz = rng.standard_normal(n) * np.exp(-t * 4.5)
    b, a = signal.butter(2, [800 / (SR / 2), (3000 + 6000 * bright) / (SR / 2)], btype="band")
    place(signal.lfilter(b, a, nz) * 0.08 * strength, t0, 1.0, pan, rev=0.6)


def heartbeat(t0, strength=1.0):
    for k, dt in enumerate((0.0, 0.22)):
        n = int(0.7 * SR)
        t = np.arange(n) / SR
        s = np.sin(2 * np.pi * (52 - 8 * t) * t) * np.exp(-t * 11) * (1.0 if k == 0 else 0.6)
        place(s * strength * 0.5, t0 + dt, 1.0, 0, rev=0.3)


def tick(t0, freq=2400.0, gain=0.07, pan=0.0):
    n = int(0.12 * SR)
    t = np.arange(n) / SR
    s = (np.sin(2 * np.pi * freq * t) + 0.3 * np.sin(2 * np.pi * freq * 2.01 * t)) * np.exp(-t * 60)
    place(s, t0, gain, pan, rev=0.35)


def pluck(t0, nm, gain=0.08, pan=0.0, decay=0.9):
    gain *= 1.6
    f = note(nm)
    n = int(decay * 2 * SR)
    t = np.arange(n) / SR
    s = sum(np.sin(2 * np.pi * f * k * t) * np.exp(-t * (3 + 2.5 * k) / decay) / k ** 1.3 for k in range(1, 7))
    place(s * np.minimum(1, t / 0.003), t0, gain, pan, rev=0.35)


def bell(t0, nm, gain=0.09, pan=0.0, decay=3.5):
    f = note(nm)
    n = int(decay * 1.6 * SR)
    t = np.arange(n) / SR
    partials = ((1.0, 1.0), (2.0, 0.45), (2.76, 0.32), (4.07, 0.16), (5.4, 0.1))
    s = sum(a * np.sin(2 * np.pi * f * p * t) * np.exp(-t * (1.2 + p * 0.6) / decay * 2) for p, a in partials)
    place(s * np.minimum(1, t / 0.002), t0, gain, pan, rev=0.55)


def whoosh(t0, dur=0.9, gain=0.08, up=True, pan=0.0):
    n = int(dur * SR)
    t = np.arange(n) / SR
    nz = rng.standard_normal(n)
    out = np.zeros(n)
    steps = 24
    for i in range(steps):
        a, b_ = i * n // steps, (i + 1) * n // steps
        x = i / (steps - 1)
        fc = 300 + (5200 if up else 5200 * (1 - x) + 300) * (x if up else 1)
        bb, aa = signal.butter(2, min(fc, 18000) / (SR / 2))
        out[a:b_] = signal.lfilter(bb, aa, nz[a:b_])
    shape = np.sin(np.pi * np.clip(t / dur, 0, 1)) ** 2
    place(out * shape, t0, gain, pan, rev=0.5)


def riser(t_end, dur=2.2, gain=0.09):
    n = int(dur * SR)
    t = np.arange(n) / SR
    x = t / dur
    nz = rng.standard_normal(n)
    out = np.zeros(n)
    steps = 40
    for i in range(steps):
        a, b_ = i * n // steps, (i + 1) * n // steps
        fc = 400 + 9000 * (i / steps) ** 2
        bb, aa = signal.butter(2, fc / (SR / 2))
        out[a:b_] = signal.lfilter(bb, aa, nz[a:b_])
    tone = np.sin(2 * np.pi * np.cumsum(220 + 440 * x ** 2) / SR) * 0.25
    place((out + tone) * x ** 2.2, t_end - dur, gain, 0, rev=0.4)


# ================================================================ arrangement
# CH01 — darkness, pulse, the question
pad(0.0, sec("ch01", 255), ["A1", "E2", "B2", "C3"], cutoff=500, gain=0.07, fade_in=4.0, fade_out=1.0)
heartbeat(at("ch01", "pulse1"), 0.8)
heartbeat(at("ch01", "pulse2"), 1.0)
hit(at("ch01", "every"), 1.1)
hit(at("ch01", "matters"), 0.9, bright=0.7)
# intentional silence ~ silenceFrom..silenceTo, then the question
pad(sec("ch01", 292), sec("ch02", 12), ["A1", "E2", "C3", "G3"], cutoff=650, gain=0.07, fade_in=1.5, fade_out=1.4)
hit(at("ch01", "butWho"), 0.5)
riser(at("ch01", "understands"), 1.4, 0.07)
hit(at("ch01", "understands"), 1.2, bright=0.9)
whoosh(sec("ch01", 535), 1.6, 0.06, up=False)

# CH02 — beyond listening
pad(sec("ch02", 0), sec("ch03", 6), ["F2", "C3", "E3", "A3"], cutoff=900, gain=0.07, fade_in=1.2, fade_out=1.0, bright_to=1500)
hit(at("ch02", "listen"), 0.5, bright=0.3)
hit(at("ch02", "understand"), 1.0, bright=0.8)
whoosh(at("ch02", "ui") - 0.5, 1.0, 0.07)
tick(at("ch02", "reply"), 2000)
tick(at("ch02", "reply") + 1.45, 2600, 0.05)
whoosh(at("ch02", "reflection") - 0.4, 0.8, 0.05)
tick(at("ch02", "reflection") + 1.0, 2200, 0.05)

# CH03 — Meaning Integrity Engine
pad(sec("ch03", 0), sec("ch03", 292), ["D2", "A2", "F3", "E4"], cutoff=800, gain=0.07, fade_in=0.8, fade_out=0.6)
hit(at("ch03", "oneWord"), 0.8)
whoosh(at("ch03", "swap") - 0.2, 0.7, 0.07, up=False)
tick(at("ch03", "swap") + 0.55, 1500, 0.08)
hit(at("ch03", "different"), 0.9, bright=0.6)
whoosh(at("ch03", "diff") - 0.4, 0.9, 0.06)
tick(at("ch03", "flag"), 1800, 0.09)
tick(at("ch03", "flag") + 0.12, 1350, 0.08)
# silence, then the correction
hit(at("ch03", "oneCorrection"), 1.2, bright=0.8)
pad(sec("ch03", 320), sec("ch03", 545), ["A2", "E3", "G3", "C4"], cutoff=900, gain=0.06, fade_in=1.2, fade_out=0.6)
whoosh(at("ch03", "impact") - 0.4, 0.8, 0.05)
tick(at("ch03", "impactHit"), 2100, 0.07)
hit(at("ch03", "every"), 0.9)
riser(at("ch03", "graph"), 1.8, 0.08)
hit(at("ch03", "graph"), 1.0, bright=0.5)
# driving pulse under the graph: 16ths at 112 bpm on Am
pad(sec("ch03", 546), sec("ch03", 960), ["A1", "E2", "C3", "E3"], cutoff=600, gain=0.07, fade_in=0.4, fade_out=1.0, bright_to=2400)
step = 60 / 112 / 4
seq = ["A3", "E4", "C4", "E4", "A3", "E4", "B3", "E4", "A3", "E4", "C4", "G4", "A3", "E4", "B3", "D4"]
t = at("ch03", "edges")
i = 0
while t < sec("ch03", 870):
    g = 0.045 + 0.03 * (t > at("ch03", "correct"))
    pluck(t, seq[i % 16], g, pan=(-0.35 if i % 2 else 0.35), decay=0.5)
    if i % 4 == 0:
        heartbeat(t, 0.35)
    t += step
    i += 1
for k in range(4):
    tick(at("ch03", "edges") + k * 0.4, 2600 + 200 * k, 0.04)
hit(at("ch03", "correct"), 1.3, bright=0.9)
bell(at("ch03", "correct"), "E5", 0.07)
whoosh(at("ch03", "wave") - 0.1, 2.4, 0.05)
tick(at("ch03", "stale"), 1700, 0.09, -0.2)
tick(at("ch03", "revoked"), 1300, 0.09, 0.3)
tick(at("ch03", "cancelled"), 1000, 0.09, 0.3)
whoosh(at("ch03", "proof") - 0.4, 0.9, 0.05)
pad(sec("ch03", 940), sec("ch04", 10), ["F2", "C3", "E3", "B3"], cutoff=1100, gain=0.07, fade_in=1.0, fade_out=0.8)
hit(at("ch03", "title"), 1.0, bright=0.7)

# CH04 — context over time (calm)
pad(sec("ch04", 0), sec("ch05", 8), ["C2", "G2", "B2", "D3", "E3"], cutoff=900, gain=0.08, fade_in=1.0, fade_out=1.0, bright_to=1300)
hit(at("ch04", "moment"), 0.45, bright=0.3)
hit(at("ch04", "context"), 0.85, bright=0.6)
whoosh(at("ch04", "axis") - 0.3, 1.6, 0.05)
for c, nm in (("m1", "E4"), ("m2", "G4"), ("m3", "B4")):
    pluck(at("ch04", c), nm, 0.09, decay=1.6)
hit(at("ch04", "noScores"), 0.4, bright=0.2)
whoosh(at("ch04", "ui") - 0.4, 0.9, 0.05)

# CH05 — your next step
pad(sec("ch05", 0), sec("ch06", 8), ["G2", "D3", "E3", "B3"], cutoff=1000, gain=0.08, fade_in=0.8, fade_out=1.0)
hit(at("ch05", "beginning"), 0.8)
hit(at("ch05", "next"), 1.0, bright=0.8)
whoosh(at("ch05", "write") - 0.4, 0.9, 0.05)
tick(at("ch05", "saved"), 2300, 0.06)
pluck(at("ch05", "optional"), "D5", 0.07, decay=1.4)

# CH06 — your words, your choice, your approval
pad(sec("ch06", 0), sec("ch06", 340), ["F2", "C3", "G3", "A3"], cutoff=1000, gain=0.08, fade_in=0.6, fade_out=1.0)
hit(at("ch06", "words"), 0.7, bright=0.4)
hit(at("ch06", "choice"), 0.8, bright=0.6)
hit(at("ch06", "approval"), 0.95, bright=0.8)
whoosh(at("ch06", "cards") - 0.4, 0.9, 0.05)
tick(at("ch06", "pick1"), 2400, 0.07)
tick(at("ch06", "pick2"), 2700, 0.07)
whoosh(at("ch06", "assemble") - 0.1, 1.3, 0.05)
pad(sec("ch06", 330), sec("ch07", 8), ["C2", "G2", "E3", "D4"], cutoff=1200, gain=0.08, fade_in=1.0, fade_out=1.0)
bell(at("ch06", "approve"), "G5", 0.06)
hit(at("ch06", "approve"), 0.6, bright=0.9)
whoosh(at("ch06", "ui") - 0.4, 0.9, 0.05)

# CH07 — support, on your terms
pad(sec("ch07", 0), sec("ch07", 380), ["A1", "E2", "G2", "C3", "E3"], cutoff=900, gain=0.08, fade_in=0.6, fade_out=1.0)
hit(at("ch07", "ready"), 0.45, bright=0.3)
hit(at("ch07", "terms"), 1.0, bright=0.8)
whoosh(at("ch07", "connect") - 0.3, 1.2, 0.05)
tick(at("ch07", "send"), 2200, 0.06, -0.4)
tick(at("ch07", "propose"), 1800, 0.06, 0.4)
bell(at("ch07", "confirm"), "C5", 0.05)
place(boom(0.7, 70, 32, 0.9), at("ch07", "revoke"), 0.5, 0, rev=0.3)
pad(sec("ch07", 380), sec("ch08", 10), ["F2", "C3", "A3", "E4"], cutoff=1000, gain=0.07, fade_in=1.0, fade_out=1.0)
whoosh(at("ch07", "ui") - 0.4, 0.9, 0.05)

# CH08 — final reveal, sonic signature
pad(sec("ch08", 0), sec("ch08", 90), ["A1", "E2", "C3"], cutoff=500, gain=0.06, fade_in=0.4, fade_out=1.2)
riser(at("ch08", "logo"), 2.4, 0.08)
place(boom(0.9, 60, 30, 2.4), at("ch08", "logo"), 0.5, 0, rev=0.4)
bell(at("ch08", "logo"), "C5", 0.09, decay=4.5)
pad(sec("ch08", 90), DUR, ["C2", "G2", "E3", "D4", "G4"], cutoff=1300, gain=0.09, fade_in=1.5, fade_out=3.5)
for c, nm in (("story", "E4"), ("meaning", "G4"), ("next", "C5")):
    hit(at("ch08", c), 0.45, bright=0.5)
    pluck(at("ch08", c), nm, 0.08, decay=1.5)
hit(at("ch08", "wordmark"), 0.9, bright=0.6)
bell(at("ch08", "wordmark"), "G4", 0.07, decay=4.0)
# signature: two-note sprout motif, restrained
bell(at("ch08", "built"), "C5", 0.06, decay=5)
bell(at("ch08", "built") + 0.38, "G5", 0.05, decay=5)

# ================================================================ mix
def reverb_ir(seconds=2.8, seed=7):
    r = np.random.default_rng(seed)
    n = int(seconds * SR)
    t = np.arange(n) / SR
    ir = r.standard_normal(n) * np.exp(-t * 3.2)
    b, a = signal.butter(1, 5500 / (SR / 2))
    ir = signal.lfilter(b, a, ir)
    ir[: int(0.012 * SR)] *= np.linspace(0, 1, int(0.012 * SR))
    return ir / np.sqrt(np.sum(ir ** 2))


wet_l = signal.fftconvolve(REV_SEND, reverb_ir(seed=7))[:N]
wet_r = signal.fftconvolve(REV_SEND, reverb_ir(seed=11))[:N]
mix_l = L + wet_l * 0.55
mix_r = R + wet_r * 0.55

# gentle high-pass to remove DC/rumble, then soft-knee limiting
b, a = signal.butter(2, 28 / (SR / 2), btype="high")
mix_l = signal.lfilter(b, a, mix_l)
mix_r = signal.lfilter(b, a, mix_r)
peak = max(np.max(np.abs(mix_l)), np.max(np.abs(mix_r)))
target = 0.89
mix_l, mix_r = mix_l / peak * 1.25, mix_r / peak * 1.25
mix_l = np.tanh(mix_l) * target / np.tanh(1.25)
mix_r = np.tanh(mix_r) * target / np.tanh(1.25)
# fade the very end
fo = int(1.2 * SR)
mix_l[-fo:] *= np.linspace(1, 0, fo) ** 2
mix_r[-fo:] *= np.linspace(1, 0, fo) ** 2

out_dir = os.path.join(ROOT, "public/audio")
os.makedirs(out_dir, exist_ok=True)


def write(path, l, r):
    st = np.stack([l, r], axis=1)
    pcm = np.clip(st, -1, 1)
    pcm32 = (pcm * (2 ** 31 - 1)).astype(np.int32)
    wavfile.write(path, SR, pcm32)


write(os.path.join(out_dir, "score-master.wav"), mix_l, mix_r)

# teaser: the master score cut on the teaser edit, equal-power crossfades
xf = int(0.08 * SR)
tl, tr = [], []
for k, s in enumerate(TL["teaser"]):
    a0, a1 = int(s["from"] / FPS * SR), int(s["to"] / FPS * SR)
    sl, sr_ = mix_l[a0:a1 + xf].copy(), mix_r[a0:a1 + xf].copy()
    if k == len(TL["teaser"]) - 1:
        sl, sr_ = sl[: a1 - a0], sr_[: a1 - a0]
    if tl:
        w = np.sin(np.linspace(0, np.pi / 2, xf))
        tl[-1][-xf:] = tl[-1][-xf:] * w[::-1] + sl[:xf] * w
        tr[-1][-xf:] = tr[-1][-xf:] * w[::-1] + sr_[:xf] * w
        sl, sr_ = sl[xf:], sr_[xf:]
    tl.append(sl)
    tr.append(sr_)
TLs, TRs = np.concatenate(tl), np.concatenate(tr)
total = int(sum(s["to"] - s["from"] for s in TL["teaser"]) / FPS * SR)
TLs, TRs = TLs[:total], TRs[:total]
fo = int(1.0 * SR)
TLs[-fo:] *= np.linspace(1, 0, fo) ** 2
TRs[-fo:] *= np.linspace(1, 0, fo) ** 2
write(os.path.join(out_dir, "score-teaser.wav"), TLs, TRs)
print("wrote", out_dir, f"master {N / SR:.1f}s teaser {len(TLs) / SR:.1f}s")


def loudnorm(path, target=-16.0, tp=-1.5):
    """Two-pass linear EBU R128 normalisation with FFmpeg (keeps dynamics)."""
    m = subprocess.run(["ffmpeg", "-hide_banner", "-nostats", "-i", path, "-af", f"loudnorm=I={target}:TP={tp}:LRA=11:print_format=json", "-f", "null", "-"], capture_output=True, text=True).stderr
    js = json.loads(re.search(r"\{[^{}]*\"input_i\"[^{}]*\}", m, re.S).group(0))
    tmp = path.replace(".wav", ".norm.wav")
    af = f"loudnorm=I={target}:TP={tp}:LRA=11:measured_I={js['input_i']}:measured_TP={js['input_tp']}:measured_LRA={js['input_lra']}:measured_thresh={js['input_thresh']}:offset={js['target_offset']}:linear=true"
    subprocess.run(["ffmpeg", "-y", "-loglevel", "error", "-i", path, "-af", af, "-ar", str(SR), "-c:a", "pcm_s24le", tmp], check=True)
    os.replace(tmp, path)


for f in ("score-master.wav", "score-teaser.wav"):
    loudnorm(os.path.join(out_dir, f))
print("normalised to -16 LUFS")
