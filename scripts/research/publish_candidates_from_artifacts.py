#!/usr/bin/env python3
"""Rebuild the 2026-10-26 Europe PMC metadata-only pool from archived Action artifacts."""
import argparse, collections, datetime, hashlib, json, re
from pathlib import Path

SOURCES = [
    ("recent_2021_2026", "sela_candidates.jsonl", "sela_candidates_manifest.json"),
    ("historical_2022_2025", "sela_pre2026_candidates.jsonl", "sela_pre2026_manifest.json"),
]
ALLOWED = {"title","authors","doi","pmid","first_publication_date","journal","publication_type",
           "themes","europepmc_url","screening_status"}

def doi(s):
    s = (s or "").strip().lower()
    return re.sub(r"^(?:https?://(?:dx\.)?doi\.org/|doi:)", "", s)

def main():
    p=argparse.ArgumentParser()
    p.add_argument("--recent",type=Path,required=True)
    p.add_argument("--historical",type=Path,required=True)
    p.add_argument("--output",type=Path,default=Path("data/research"))
    p.add_argument("--curated",type=Path,default=Path("docs/research/sela-relay/research_100.json"))
    args=p.parse_args()
    batches=[]; source_info=[]
    for folder,(name,file,meta_file) in zip([args.recent,args.historical],SOURCES):
        m=json.loads((folder/meta_file).read_text(encoding="utf-8"))
        if m.get("request_errors"): raise ValueError("Incomplete API batch: "+name)
        rows=[json.loads(line) for line in (folder/file).read_text(encoding="utf-8").splitlines() if line.strip()]
        if len(rows)!=m["candidate_count"]: raise ValueError("Source count mismatch: "+name)
        for r in rows:
            if (not r.keys() <= ALLOWED or not r.get("title") or
                not "2021-10-27"<=r.get("first_publication_date","")<="2026-10-26" or
                r.get("screening_status")!="candidate_metadata_only_unreviewed" or
                not isinstance(r.get("themes"),list)):
                raise ValueError("Unexpected record schema/status/date")
        batches.append((name,rows))
        source_info.append({"name":name,"record_count":len(rows),"source_generated_utc":m.get("generated_utc")})
    merged={}; by_doi={}; by_pmid={}; by_title={}; duplicates=0
    for name,rows in batches:
        for r in rows:
            d=doi(r.get("doi")); pm=str(r.get("pmid") or "")
            title=("".join(c for c in r["title"].casefold() if c.isalnum()),r["first_publication_date"][:4])
            i=(by_doi.get(d) if d else None) or (by_pmid.get(pm) if pm else None) or by_title.get(title)
            if i:
                target=merged[i];duplicates+=1
                target["themes"]=sorted(set(target["themes"]+r["themes"]))
                target["source_batches"]=sorted(set(target["source_batches"]+[name]))
                if not target.get("doi") and d:target["doi"]=d
                if not target.get("pmid") and pm:target["pmid"]=pm
            else:
                i=len(merged)+1; target=dict(r);target["source_batches"]=[name];merged[i]=target
            if d:by_doi[d]=i
            if pm:by_pmid[pm]=i
            by_title[title]=i
    rows=sorted(merged.values(),key=lambda r:(r["first_publication_date"],r["title"]),reverse=True)
    if len(rows)!=7580 or duplicates!=420: raise ValueError("Expected 7,580 records and 420 duplicates")
    curated=json.loads(args.curated.read_text(encoding="utf-8"))
    curated_dois={doi(r.get("doi")) for r in curated["papers"] if r.get("doi")}
    overlap=[r.get("doi") for r in rows if doi(r.get("doi")) in curated_dois]
    if overlap: raise ValueError("Overlaps 100-paper reviewed-title register: "+repr(overlap[:3]))
    args.output.mkdir(parents=True,exist_ok=True)
    path=args.output/"sela_research_candidates_7580.jsonl"
    with path.open("w",encoding="utf-8",newline="\n") as f:
        for r in rows:f.write(json.dumps(r,sort_keys=True,ensure_ascii=False,separators=(",",":"))+"\n")
    manifest={
        "dataset_name":"Sela Europe PMC bibliographic candidates",
        "date_window":["2021-10-27","2026-10-26"],
        "generated_utc":datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "source":"Europe PMC REST API","source_batches":source_info,
        "data_file":path.name,"data_sha256":hashlib.sha256(path.read_bytes()).hexdigest(),
        "record_count":len(rows),"raw_records":8000,"duplicate_records_merged":duplicates,
        "records_with_doi":sum(bool(r.get("doi")) for r in rows),
        "excluded_curated_count":len(curated["papers"]),
        "publication_year_distribution":dict(sorted(collections.Counter(r["first_publication_date"][:4] for r in rows).items())),
        "theme_record_counts":dict(sorted(collections.Counter(t for r in rows for t in r["themes"]).items())),
        "review_status":"candidate_metadata_only_unreviewed","includes_full_text":False,"includes_abstracts":False,
        "limitations":"Unreviewed metadata. Strong recency and selection bias; title/abstract screening and full-text appraisal pending. Not a clinical evidence corpus."
    }
    (args.output/"sela_research_candidates_manifest.json").write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    print(f"Validated {len(rows)} unreviewed candidates; SHA-256 {manifest['data_sha256']}")

if __name__=="__main__":main()
