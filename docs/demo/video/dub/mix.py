import json, subprocess
plan = json.load(open("best/sched.json"))
import os
V = os.environ["SRC_VIDEO"]
TOTAL = 183.0
args = ["ffmpeg","-v","error","-y"]
for p in plan: args += ["-i", f"best/{p['n']}.wav"]
fc = []
for i, p in enumerate(plan):
    d = p["dur"]/p["f"]
    fc.append(f"[{i}]highpass=f=70,atempo={p['f']},afade=t=in:d=0.015,afade=t=out:st={max(0,d-0.04):.3f}:d=0.04,adelay={int(p['start']*1000)}:all=1[v{i}]")
fc.append("".join(f"[v{i}]" for i in range(len(plan))) + f"amix=inputs={len(plan)}:normalize=0,apad=whole_dur={TOTAL},atrim=0:{TOTAL}[vo]")
args += ["-filter_complex", ";".join(fc), "-map", "[vo]", "-ac", "1", "-ar", "48000", "vo.wav"]
subprocess.run(args, check=True)
# music ducking envelope (-9 dB under voice, 0.25 s ramp in, 0.45 s ramp out)
terms = []
for p in plan:
    s, e = p["start"]-0.25, p["end"]+0.45
    terms.append(f"min(clip((t-{s:.3f})/0.25,0,1),clip(({e:.3f}-t)/0.45,0,1))")
g = terms[0]
for t in terms[1:]: g = f"max({g},{t})"
open("work/duck.txt","w").write(f"volume='1-(1-0.355)*{g}':eval=frame")
