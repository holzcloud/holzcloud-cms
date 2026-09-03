---
schema_version: 1
open_count: 0
waived_count: 1
fixed_count: 0
total_count: 1
last_updated: 2026-09-03T06:49:41.839Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | quick-260903-bsk | deviation | internal/i18n/locales/en.json |  | tools/i18n writeCatalog emits flush-left JSON while the four full catalogues carried a two-space indent; -write reformatted ~2250 lines each. Tool format kept as canonical. | waived | Accepted during execution, not an open defect: the tool's flush-left format is canonical (de-CH, fr-CH and it-CH were already flush-left); the two-space indent in the four full catalogues was drift from a hand-translation pass. | 2026-09-03T06:49:26.839Z | 2026-09-03T06:49:41.839Z |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "quick-260903-bsk",
    "file": "internal/i18n/locales/en.json",
    "line": null,
    "description": "tools/i18n writeCatalog emits flush-left JSON while the four full catalogues carried a two-space indent; -write reformatted ~2250 lines each. Tool format kept as canonical.",
    "status": "waived",
    "reason": "Accepted during execution, not an open defect: the tool's flush-left format is canonical (de-CH, fr-CH and it-CH were already flush-left); the two-space indent in the four full catalogues was drift from a hand-translation pass.",
    "recorded_at": "2026-09-03T06:49:26.839Z",
    "resolved_at": "2026-09-03T06:49:41.839Z"
  }
]
````
