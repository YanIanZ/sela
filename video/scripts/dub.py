#!/usr/bin/env python3
"""
Indonesian dubbing for the 3-minute product demo.

Places one voice clip per narration cue at its exact time, ducks the music
under the voice, normalises the mix and muxes it onto the rendered demo
(the video stream is copied, never re-encoded). English subtitles are already
burned into the video; subtitles/demo.en.srt is added as an optional track.

Voice sources (pick one):
  --lines DIR          recorded takes: DIR/01.wav, DIR/02.wav, ... one per cue in
                       subtitles/demo.id.srt order (any format FFmpeg reads)
  --tts edge           Microsoft Edge neural TTS through the `edge-tts` CLI
                       (pip install edge-tts; needs internet). Default voice
                       id-ID-GadisNeural; try --voice id-ID-ArdiNeural.
  --tts espeak         espeak-ng -v id (offline, robotic; for timing checks only)
  --test-tones         a soft tone per cue, to check timing and ducking

Requires: python3, numpy, scipy, ffmpeg/ffprobe. Run scripts/demo-plan.mjs first.

  python3 scripts/dub.py --tts edge
  python3 scripts/dub.py --lines ~/rekaman/sela-demo
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys

import numpy as np

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SR = 48000
ap = argparse.ArgumentParser()
g = ap.add_mutually_exclusive_group(required=True)
g.add_argument("--lines")
g.add_argument("--tts", choices=["edge", "espeak"])
g.add_argument("--test-tones", action="store_true")
ap.add_argument("--voice", default="id-ID-GadisNeural")
ap.add_argument("--rate", default="-4%", help="edge-tts speaking rate, e.g. -4%% (calmer) or +0%%")
ap.add_argument("--video", default=os.path.join(ROOT, "out/SELA-Product-Demo-3min-1080p-EN-subs.mp4"))
ap.add_argument("--music", default=os.path.join(ROOT, "public/audio/score-demo.wav"))
ap.add_argument("--out", default=os.path.join(ROOT, "out/SELA-Product-Demo-3min-1080p-ID-dub-EN-subs.mp4"))
ap.add_argument("--duck-db", type=float, default=9.0)
args = ap.parse_args()

plan = json.load(open(os.path.join(ROOT, ".cache/demo-plan.json")))
FPS = plan["fps"]
cues = plan["cues"]
work = os.path.join(ROOT, ".cache/dub")
os.makedirs(work, exist_ok=True)


def decode(path):
    raw = subprocess.run(["ffmpeg", "-v", "error", "-i", path, "-ac", "1", "-ar", str(SR), "-f", "f32le", "-"], capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.float32).copy()


def decode_stereo(path):
    raw = subprocess.run(["ffmpeg", "-v", "error", "-i", path, "-ac", "2", "-ar", str(SR), "-f", "f32le", "-"], capture_output=True, check=True).stdout
    return np.frombuffer(raw, dtype=np.float32).reshape(-1, 2).copy()


def trim_silence(x, thr=0.01):
    idx = np.where(np.abs(x) > thr)[0]
    return x[max(0, idx[0] - 480): idx[-1] + 2400] if len(idx) else x


def fit(x, slot):
    """Speed up (max 1.18x, pitch kept) when a take is longer than its slot."""
    dur = len(x) / SR
    if dur <= slot:
        return x, 1.0
    tempo = min(1.18, dur / slot)
    src, dst = os.path.join(work, "fit_in.wav"), os.path.join(work, "fit_out.wav")
    write_wav(src, x)
    subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", src, "-af", f"atempo={tempo:.4f}", dst], check=True)
    return decode(dst), tempo


def write_wav(path, x):
    from scipy.io import wavfile
    wavfile.write(path, SR, np.clip(x, -1, 1).astype(np.float32))


def take(i, c):
    n = f"{i + 1:02d}"
    if args.lines:
        cands = [f for f in os.listdir(args.lines) if f.split(".")[0] == n]
        if not cands:
            sys.exit(f"missing take {n}.* in {args.lines} for: {c['id']}")
        return decode(os.path.join(args.lines, cands[0]))
    if args.test_tones:
        d = (c["end"] - c["start"]) / FPS * 0.85
        t = np.arange(int(d * SR)) / SR
        return (0.25 * np.sin(2 * np.pi * 220 * t) * np.minimum(1, t / 0.05) * np.minimum(1, (d - t) / 0.05)).astype(np.float32)
    mp = os.path.join(work, f"{n}.mp3")
    if args.tts == "edge":
        if not shutil.which("edge-tts"):
            sys.exit("edge-tts not found: pip install edge-tts")
        subprocess.run(["edge-tts", "--voice", args.voice, f"--rate={args.rate}", "--text", c["id"], "--write-media", mp], check=True)
    else:
        exe = shutil.which("espeak-ng") or shutil.which("espeak")
        if not exe:
            sys.exit("espeak-ng not found")
        mp = os.path.join(work, f"{n}.wav")
        subprocess.run([exe, "-v", "id", "-s", "150", "-w", mp, c["id"]], check=True)
    return decode(mp)


music = decode_stereo(args.music)
N = len(music)
vo = np.zeros(N, dtype=np.float32)
active = np.zeros(N, dtype=np.float32)
report = []
for i, c in enumerate(cues):
    x = trim_silence(take(i, c))
    nxt = cues[i + 1]["start"] if i + 1 < len(cues) else int(N / SR * FPS)
    slot = (min(nxt, c["end"] + int(0.6 * FPS)) - c["start"]) / FPS - 0.1
    x, tempo = fit(x, slot)
    a = int(c["start"] / FPS * SR)
    n = min(len(x), N - a)
    vo[a:a + n] += x[:n]
    active[a:a + n] = 1.0
    over = len(x) / SR - slot
    report.append(f"{i + 1:02d} {c['start'] / FPS:6.2f}s slot {slot:4.1f}s take {len(x) / SR:4.1f}s" + (f" tempo x{tempo:.2f}" if tempo > 1 else "") + (f"  !! {over:.1f}s too long" if over > 0.05 else ""))

# duck the music under the voice: 120 ms attack, 350 ms release
att, rel = int(0.12 * SR), int(0.35 * SR)
env = np.convolve(active, np.ones(att) / att, mode="same")
env = np.clip(env * 3, 0, 1)
kernel = np.exp(-np.arange(rel) / (rel / 4))
kernel /= kernel.sum()
env = np.clip(np.convolve(env, kernel)[:N] * 1.4, 0, 1)
gain = 10 ** (-args.duck_db * env / 20)
mixed = music * gain[:, None] + np.stack([vo, vo], 1) * 0.95
mix_path = os.path.join(work, "mix.wav")
from scipy.io import wavfile

wavfile.write(mix_path, SR, np.clip(mixed, -1, 1).astype(np.float32))

# two-pass loudness normalisation to -16 LUFS, then mux
m = subprocess.run(["ffmpeg", "-hide_banner", "-nostats", "-i", mix_path, "-af", "loudnorm=I=-16:TP=-1.5:LRA=11:print_format=json", "-f", "null", "-"], capture_output=True, text=True).stderr
js = json.loads(re.search(r"\{[^{}]*\"input_i\"[^{}]*\}", m, re.S).group(0))
norm = os.path.join(work, "mix-norm.wav")
subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", mix_path, "-af", f"loudnorm=I=-16:TP=-1.5:LRA=11:measured_I={js['input_i']}:measured_TP={js['input_tp']}:measured_LRA={js['input_lra']}:measured_thresh={js['input_thresh']}:offset={js['target_offset']}:linear=true", "-ar", str(SR), norm], check=True)
srt = os.path.join(ROOT, "subtitles/demo.en.srt")
subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", args.video, "-i", norm, "-i", srt, "-map", "0:v", "-map", "1:a", "-map", "2:s", "-c:v", "copy", "-c:a", "aac", "-b:a", "256k", "-c:s", "mov_text", "-metadata:s:a:0", "language=ind", "-metadata:s:s:0", "language=eng", "-shortest", "-movflags", "+faststart", args.out], check=True)
print("\n".join(report))
print("wrote", args.out)
