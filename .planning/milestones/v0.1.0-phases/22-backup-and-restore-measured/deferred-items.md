# Deferred items, phase 22

- **STATE.md position (found in 22-01, out of scope):** `state.advance-plan` refuses with `ambiguous_position_phase`, because STATE.md carries three `Phase:` entries in its Current Position sections (22, "none open ... v2.1 is closed", 12) and the frontmatter progress counters read `total_phases: 0`, `completed_plans: 0`. The plan counter was therefore not advanced and `state.update-progress` was skipped; `record-session`, `add-decision` and `roadmap.update-plan-progress` worked. The orchestrator or the owner has to collapse the historical sections before the counters can move.
- **Requirements BAK-01 and BAK-02** are declared by more than one plan of this phase, so `requirements.ready-ids` reports 0 of 2 ready; they are marked complete when the last declaring plan has its SUMMARY.
- **README claim without a check (found in 22-02, out of scope):** `deploy/test/README.md` says a `DEVIATION[...]` name outside the documented set fails the run; `deploy/test/run.sh` has no such check (the only readers are `deviation_kinds` and the chain's own distinct-kinds loop). Either add the check or reword the README sentence in a later plan.
  status: acknowledged
