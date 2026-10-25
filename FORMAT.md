# Specification format

`SPEC.md` is the normative source for Sela v2 requirements. Use concise English, stable IDs and explicit implementation status. Supporting docs explain the spec; they must not silently change it.

Required sections: `§G` goal, `§C` constraints, `§I` interfaces, `§V` numbered invariants, `§T` tasks, `§B` bug backpropagation.

Task table: `id | status | task | cites`. Status: `.` pending, `~` in progress, `x` complete, `!` blocked. A completed task needs recorded evidence. `cites` lists the relevant `C`, `I` and `V` IDs.

Bug table: `id | date | cause | fix`. Keep IDs monotonic. Record observed failures and whether a new invariant would prevent recurrence; do not invent bug history. V1 findings remain in the archive until reviewed for v2 relevance.

State whether a statement is user-confirmed, an engineering default, externally sourced, implemented, or pending verification. An implementation discrepancy is recorded and resolved against the spec, not hidden by changing prose. Update supporting docs and validation references whenever the spec changes. Commands must say whether they work now or are planned.
