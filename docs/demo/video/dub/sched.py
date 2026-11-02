import json, subprocess, sys
D = sys.argv[1]; CAP = float(sys.argv[2]); END = float(sys.argv[3])
rows = json.load(open("lines/plan.json"))
def dur(n): return float(subprocess.check_output(["ffprobe","-v","error","-show_entries","format=duration","-of","csv=p=0",f"{D}/{n}.wav"]))
GAP, EARLY, SPILL = 0.15, 0.45, 0.35
prev_end = 0.0; plan = []
for i, r in enumerate(rows):
    d = dur(r["n"])
    start = max(prev_end + GAP, r["start"] - EARLY)
    nxt = rows[i+1]["start"] + SPILL if i+1 < len(rows) else END - 0.25
    f = max(1.0, d / (nxt - start))
    f2 = min(f, CAP)
    end = start + d / f2
    plan.append(dict(n=r["n"], start=round(start,3), dur=d, f=round(f2,3), need=round(f,3), end=round(end,3), drift=round(start-r["start"],2)))
    prev_end = end
for p in plan: print(p)
json.dump(plan, open(f"{D}/sched.json","w"), indent=1)
print("last end", round(prev_end,2), "overflow lines", [p["n"] for p in plan if p["need"]>CAP])
