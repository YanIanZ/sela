# Specification format

`SPEC.md` is the normative source for Sela requirements: Sela v3 (owner brief v3, the current product) and the v2 requirements and tasks it keeps as history. Use concise English, stable IDs and explicit implementation status. Supporting docs explain the spec; they must not silently change it. A published ID is never reused: a superseded item keeps its ID and names its replacement, and IDs that collide after a merge are renumbered on the side that was not published yet, with a dated table (as in D-025 and D-053).

Required sections: `§G` goal, `§C` constraints, `§I` interfaces, `§V` numbered invariants, `§T` tasks, `§B` bug backpropagation.

Task table: `id | status | task | cites`. Status: `.` pending, `~` in progress, `x` complete, `!` blocked. A completed task needs recorded evidence. `cites` lists the relevant `C`, `I` and `V` IDs.

Bug table: `id | date | cause | fix`. Keep IDs monotonic. Record observed failures and whether a new invariant would prevent recurrence; do not invent bug history. V1 findings remain in the archive until reviewed for v2 and v3 relevance (v3: the defects DX-01 to DX-16 of the feature matrix, guarded in SPEC §B).

State whether a statement is user-confirmed, a PM decision (v3: taken under owner brief v3, D-027 to D-050 and D-054 onward, revisable by the owner), an engineering default or externally sourced, and give implementation status with the status words of SPEC §G (planned, in progress, implemented, verified locally, verified live, ready for controlled pilot, public release, blocked externally); v2 tasks keep their earlier words ("tested locally" for v2 means verified locally). An implementation discrepancy is recorded and resolved against the spec, not hidden by changing prose. Update supporting docs and validation references whenever the spec changes. Commands must say whether they work now or are planned.
