import asyncio, os, re, ssl, sys, json
import edge_tts, edge_tts.communicate as C
# Behind a TLS-inspecting proxy, point SSL_CERT_FILE at its CA bundle.
if os.environ.get("SSL_CERT_FILE"):
    C._SSL_CTX = ssl.create_default_context(cafile=os.environ["SSL_CERT_FILE"])
md, outdir, voice = sys.argv[1], sys.argv[2], sys.argv[3]
rate = sys.argv[4] if len(sys.argv) > 4 else "+0%"
rows = []
for line in open(md, encoding="utf-8"):
    m = re.match(r"\|\s*(\d\d)\s*\|\s*(\d+):(\d+(?:\.\d+)?)\s*\|\s*([\d.]+)\s*s\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*$", line)
    if m:
        n, mi, se, mx, idt, en = m.groups()
        rows.append(dict(n=n, start=int(mi)*60+float(se), max=float(mx), id=idt, en=en))
json.dump(rows, open(os.path.join(outdir, "plan.json"), "w"), ensure_ascii=False, indent=1)
only = set(sys.argv[5].split(",")) if len(sys.argv) > 5 else None
async def main():
    for r in rows:
        if only and r["n"] not in only: continue
        text = r["id"]
        c = edge_tts.Communicate(text, voice, rate=rate, proxy=os.environ.get("HTTPS_PROXY"))
        await c.save(os.path.join(outdir, f"{r['n']}.mp3"))
        print(r["n"], flush=True)
asyncio.run(main())
print(len(rows), "rows")
