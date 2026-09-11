---
phase: 10-authentik
plan: 09
subsystem: i18n
tags: [i18n, catalogue, sso, two-factor, authentik, glossary, counting-gate]

requires:
  - phase: 10-authentik
    provides: "plan 10-06's two German literals inside {{t}} on account.html and user_list.html — the only user-visible sentences the whole phase minted"
  - phase: 10-authentik
    provides: "plan 10-08's DEPLOY.md, which had to land before the catalogues were counted, or the count would be taken on a moving tree"
  - phase: 11-galerie
    provides: "plan 11-07's method for this job, and its two recorded divergences — the value-equals-key measurement and the byte-identical de-CH rebuild"
provides:
  - "The two SSO sentences translated into en, es, fr and it; `go run ./tools/i18n` reports 0 offen, 0 verwaist on all four written catalogues"
  - "de-CH.json rebuilt with -schweiz and byte-identical: neither sentence carries a sharp s or German quotation marks"
  - "The value-equals-key measurement reported as a number before and after (unchanged: en 28, es 9, fr 16, it 15) — and the finding that the plan's own len(k)>40 filter reads 0 on a tree whose longest such entry is 16 characters"
  - "The honest form of the count gate: the full source key set diffed against the phase-10 baseline commit, ten keys added, eight attributed to Phase 11 by commit and two to plan 10-06"
  - "Two more gates that measure something other than their name — sixth and seventh instance in this phase — with the corrected commands"
  - "The measured register of each catalogue: es and it address the reader informally, fr formally, where the German uses du throughout"
affects: [10-10-browser-pass, 12-umbenennung, verify-work, ship]

actuals:
  tokens: 2250
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "A source-count gate stated as a delta against a named commit, and proved by diffing the tool's own collected key set rather than by comparing two totals — a total is moved by any phase sharing the tree, a key set names who moved it"
    - "The tool's own collector used as the measuring instrument: the key set is read by running `tools/i18n -write` against an archived tree with an emptied catalogue, so no hand-written regex re-implements the collection and no grep shape can slip past it"

key-files:
  created:
    - .planning/phases/10-authentik/10-09-SUMMARY.md
  modified:
    - internal/i18n/locales/en.json
    - internal/i18n/locales/es.json
    - internal/i18n/locales/fr.json
    - internal/i18n/locales/it.json
    - .planning/WINDOWS.md

key-decisions:
  - "The register of each translation was measured at the catalogue, not assumed from the German: es and it keep the informal address the German du has, fr switches to vous, because forty existing du-sentences already do exactly that. Copying the German's du into French would have been a defect no gate in this repository can see."
  - "`Anmeldung` is rendered as the organisation's *sign-in* in en and as *authentification* / *autenticazione* / *inicio de sesión* elsewhere, following the catalogue's own `Zur Anmeldung` and `Anmeldung` entries rather than minting a second vocabulary for the identity provider."
  - "Neither translation says switched off, disabled, désactivée, desactivada or disattivata. The German says the second factor is *enforced elsewhere*, and that is a security statement; the four values were checked against that word list mechanically as well as by eye."
  - "The plan's count gate — 'the count rose by exactly 2 from the phase baseline' — was NOT run as written, because the phase baseline it names was overtaken by Phase 11 landing in the same tree between waves. Replaced with a key-set diff that names all ten added keys and attributes each to its commit. The property the gate protects held; the number it prints did not."
  - "The divergent counting rows were recorded, not adjusted. Seven of the eighteen rows print a number other than the one they name, and in all seven the underlying property holds."

patterns-established:
  - "Measure the register before translating: a catalogue that already answers the tu/vous question in forty places is evidence, and guessing at it is the one translation error that survives every green gate"
  - "Prove a hand-edited catalogue against the tool's own writer: after filling values by hand, re-run `-write` and compare hashes. Identical hashes mean the hand edit is byte-for-byte what the tool would have produced, and no reformatting diff is waiting for the next run"

requirements-completed: [QUAL-01]

coverage:
  - id: D1
    description: "The two sentences plan 10-06 added ship in en, es, fr and it; the tool reports 0 offen, 0 verwaist on all four written catalogues"
    requirement: QUAL-01
    verification:
      - kind: other
        ref: "go run ./tools/i18n | grep -c '0 offen, 0 verwaist' -> 4"
        status: pass
      - kind: other
        ref: "python3: len(en)==len(es)==len(fr)==len(it)==1321, source 1321 Zeichenketten"
        status: pass
    human_judgment: false
  - id: D2
    description: "None of the eight new values is the German sentence copied across, and none flattens 'enforced elsewhere' into 'switched off'"
    requirement: QUAL-01
    verification:
      - kind: other
        ref: "python3: v!=k for all 8; all 8 values distinct; word list (ausgeschaltet, switched off, disabled, desactivada, désactivée, disattivata, deactivated) matches none"
        status: pass
      - kind: other
        ref: "value-equals-key count, unfiltered: en 28 / es 9 / fr 16 / it 15 before AND after"
        status: pass
    human_judgment: true
    rationale: "The mechanical half is measured above and passes. Whether an operator reading 'this installation does not ask for it a second time' then goes and checks their Authentik rather than concluding the second factor is off is the transfer T-10-50 records, and no test in this repository can judge it. The eight sentences are quoted verbatim below so plan 10-10's browser pass can compare a screen against them without opening a catalogue."
  - id: D3
    description: "de-CH.json rebuilt by -schweiz; fr-CH.json and it-CH.json only read"
    requirement: QUAL-01
    verification:
      - kind: other
        ref: "go run ./tools/i18n -schweiz -> 'de-CH.json 70 nach Regel, 3 von Hand'; sha256 of de-CH.json unchanged (5cf09fe0…); git status --porcelain on fr-CH.json and it-CH.json -> 0 lines"
        status: pass
    human_judgment: false
  - id: D4
    description: "This plan touched no Go and no HTML"
    verification:
      - kind: other
        ref: "git diff --name-only 913ef6e..HEAD -- '*.go' '*.html' | wc -l -> 0; gofmt -l . -> 0; go vet ./... exit 0; go build ./... exit 0; go test ./... 44 ok, 0 FAIL"
        status: pass
    human_judgment: false
  - id: D5
    description: "The whole phase's arithmetic measured once against the finished tree, with every divergence named and attributed"
    verification:
      - kind: other
        ref: "the counting table below — 18 rows, 11 matching, 7 divergences named with cause, attribution and (where the gate is misshapen) a corrected command"
        status: pass
    human_judgment: false
  - id: D6
    description: "Nothing untranslatable was minted anywhere else in Phase 10"
    requirement: QUAL-01
    verification:
      - kind: other
        ref: "source key set at cdcfbab (1311) diffed against HEAD (1321): 10 added, 0 removed; 8 attributed to Phase 11 commits by git log -S, 2 to 8d6df09 (10-06)"
        status: pass
    human_judgment: false

duration: 15 min
completed: 2026-09-08
status: complete
---

# Phase 10 Plan 09: The mechanical half of the standing gate — Summary

**Two sentences into four languages, and the arithmetic of a whole phase measured once against the finished tree — where seven of eighteen counting rows printed a number other than the one they name, in every case while the property they protect held.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-09-08T06:15:00Z
- **Completed:** 2026-09-08T06:30:00Z
- **Tasks:** 2 of 2
- **Files modified:** 5 (4 catalogues, the ledger)

## Accomplishments

- `go run ./tools/i18n` reports **0 offen, 0 verwaist** on en, es, fr and it — 1321 keys each, source 1321.
- The eight new values are quoted below in full, so plan 10-10's browser pass has something to hold a screen against.
- `de-CH.json` rebuilt with `-schweiz` and **byte-identical**; `fr-CH.json` and `it-CH.json` never touched.
- The value-equals-key count reported as a number, before and after: **unchanged** in all four languages.
- The count gate re-stated as a **key-set diff**, which names all ten keys added since the phase-10 baseline and attributes eight of them to Phase 11.
- Two more gates found that measure something other than their name — the **sixth and seventh** instance in this phase — both recorded in `.planning/WINDOWS.md` with the corrected command.
- `WINDOWS.md` entry 10 (`unmet-truth`: the two untranslated strings 10-06 left open) closed.

---

## Task 1 — the catalogues, in the tool's own steps

### Step 1: what stood open, and the proof that it was exactly 10-06's two

```
1321 Zeichenketten im Quelltext
de-CH.json   73 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1319 übersetzt, 2 offen, 0 verwaist
es.json      1319 übersetzt, 2 offen, 0 verwaist
fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1319 übersetzt, 2 offen, 0 verwaist
it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1319 übersetzt, 2 offen, 0 verwaist
```

The plan asks that an open key not on `10-06-SUMMARY.md`'s list be found *before* anything is translated. The two open keys were not present in the catalogues at all — they were **missing**, not empty — so listing "entries whose value is blank" printed nothing in all four languages. The check that actually holds the property is set arithmetic, not a search:

- `0 verwaist` on all four means every catalogue key is a source key, so catalogue ⊆ source.
- 1321 − 1319 = 2, so exactly two source keys are absent from each catalogue.
- Both of 10-06's sentences are in the source (`account.html:38`, `user_list.html:10`) and absent from all four catalogues (`has s1: False  has s2: False`).

Therefore the two open keys *are* those two, with nothing left to search for. **Said here because the obvious check — grep the catalogues for an empty value — reads zero and would have been mistaken for "nothing open".**

### Step 2: `-write`

Four catalogues gained the two keys with **empty** values, verified by reading them back (`inserted empty: True` in all four). This is why the copy trap does not arise by accident: the tool never writes the German into a translation slot. It arises the moment somebody fills a value by copying the key, which is a human act and needs a human check.

### Step 3: the eight values

Both sentences say the same thing from two angles: the second factor is not switched off here, it is **enforced where the person signed in**. Both are quoted in full so a reader can judge the translation without opening a catalogue.

#### Sentence 1 — the account screen, second person, shown only when `.ViaSSO`

| | |
|---|---|
| **de** | Du bist über die Anmeldung deiner Organisation hier hereingekommen. Die Bestätigung in zwei Schritten verlangt dort, wer dich angemeldet hat – diese Anlage verlangt sie dann nicht noch einmal. |
| **en** | You came in here through your organisation's sign-in. Two-step verification is required there by whoever signed you in – this installation does not ask for it a second time. |
| **es** | Has entrado aquí a través del inicio de sesión de tu organización. La verificación en dos pasos la exige allí quien te ha autenticado – esta instalación no vuelve a pedirla. |
| **fr** | Vous êtes entré ici par l'authentification de votre organisation. La vérification en deux étapes y est exigée par qui vous a authentifié – cette installation ne la redemande pas. |
| **it** | Sei entrato qui tramite l'autenticazione della tua organizzazione. La verifica in due passaggi la richiede là chi ti ha autenticato – questa installazione non la chiede una seconda volta. |

#### Sentence 2 — the user list, third person, shown only when `.SSOEnabled`

| | |
|---|---|
| **de** | Wer sich über die Anmeldung der Organisation anmeldet, bringt seine Bestätigung in zwei Schritten von dort mit. Ob sie verlangt wird, entscheidet die Anmeldung der Organisation und nicht diese Anlage. |
| **en** | Whoever signs in through the organisation's sign-in brings their two-step verification with them from there. Whether it is required is decided by the organisation's sign-in and not by this installation. |
| **es** | Quien entra por el inicio de sesión de la organización trae consigo desde allí su verificación en dos pasos. Si se exige o no lo decide el inicio de sesión de la organización, no esta instalación. |
| **fr** | Qui se connecte par l'authentification de l'organisation apporte de là sa vérification en deux étapes. C'est l'authentification de l'organisation qui décide si elle est exigée, et non cette installation. |
| **it** | Chi accede tramite l'autenticazione dell'organizzazione porta con sé da lì la sua verifica in due passaggi. Se venga richiesta lo decide l'autenticazione dell'organizzazione e non questa installazione. |

### The vocabulary, measured at the catalogue rather than chosen

Every term either already had a translation or was read off a neighbouring entry. Nothing was invented.

| German | en | es | fr | it | where it came from |
|---|---|---|---|---|---|
| Bestätigung in zwei Schritten | Two-step verification | Verificación en dos pasos | Vérification en deux étapes | Verifica in due passaggi | existing key, four screens use it |
| Anlage | installation | instalación | installation | installazione | `Gilt für Redakteure. Ein Administrator führt die ganze Anlage …` |
| Anmeldung (the act / the provider) | sign-in | inicio de sesión | authentification | autenticazione | `Anmeldung` → `Authentication`, `Zur Anmeldung` → `To the sign-in page` / `Al inicio de sesión` / `Vers la connexion` / `All'accesso` |
| Organisation | Organisation | Organización | Organisation | Organizzazione | `Organisation oder Verein` |

`GLOSSARY.md` binds none of the words in these two sentences beyond `Konto` → `account` (not used here) and the `Label` / `Beschriftung` trap (not reachable here).

### The register, which is the one thing a gate cannot see

Measured before translating, over the **forty** existing catalogue keys that address the reader as *du*:

| | address | evidence |
|---|---|---|
| **es** | informal *tú* | `Como administrador no puedes desactivarla`, `un código de tu aplicación`, `El botón que pulsaste` |
| **fr** | formal *vous* | `En tant qu'administrateur vous ne pouvez pas la désactiver`, `un code de votre application` |
| **it** | informal *tu* | `Come amministratore non puoi disattivarla`, `la tua app`, `il pulsante che hai premuto` |

So French switches register where the German keeps *du*, and forty entries already do. Sentence 1 follows each catalogue's own habit — *tu* in es and it, *vous* in fr. Had I copied the German's second person into French, every gate in this repository would still have read green.

### Step 4: `-schweiz`, and the two files that are only read

```
de-CH.json  70 nach Regel, 3 von Hand
de-CH.json   73 Abweichungen, 0 ohne Gegenstück
```

The rebuild ran and produced a **byte-identical** file — `sha256 5cf09fe0…8805b` before and after, and `de-CH.json` never appears in `git status`. That is the correct result and not a skipped step: `swissSpelling` replaces the sharp s and the German quotation marks, and **neither new sentence contains either**. Same outcome, same cause, as 11-07 recorded for its forty-one strings.

```
git status --porcelain internal/i18n/locales/fr-CH.json internal/i18n/locales/it-CH.json | wc -l
0
```

### The trap the tool cannot see — measured, not asserted

The tool counts presence. A value equal to its German key reports as translated. So the count of such entries was taken before and after, in **two** shapes:

| gate | en | es | fr | it | |
|---|---|---|---|---|---|
| the plan's: `v == k and len(k) > 40` — **before** | 0 | 0 | 0 | 0 | |
| the plan's: `v == k and len(k) > 40` — **after** | 0 | 0 | 0 | 0 | unchanged |
| unfiltered `v == k` (11-07's shape) — **before** | 28 | 9 | 16 | 15 | |
| unfiltered `v == k` (11-07's shape) — **after** | **28** | **9** | **16** | **15** | unchanged |

**The number is unchanged in every language and under both gates, and no new entry was added by this plan.** 11-07 measured 27 → 28 in en and named `Album %s – %s` as the one new entry; this plan starts from that 28 and does not move it, because none of the eight new values coincides with its German key.

**But the plan's own gate is the weaker of the two, and worth naming.** `len(k) > 40` reads **0 on every language**, and the longest value-equals-key entry anywhere in the tree is **16 characters** (`<code>---</code>`). The threshold therefore sits far above every existing entry: the gate has no signal at all on the current tree, and a *short* copied value — the shape 11-07 actually found — would pass it silently. It does hold for this plan's own purpose, because both new sentences are 192 and 200 characters and a verbatim copy would trip it. **The check that measures the general property is the unfiltered count, and that is the one reported as the number above.** Sixth in the family this phase has been collecting.

Three further checks the tool cannot make, run mechanically:

- `v != k` for all eight new values: **pass**
- all eight values distinct from each other: **pass** (a copy-paste between languages would collide)
- none contains `ausgeschaltet`, `switched off`, `disabled`, `desactivada`, `désactivée`, `disattivata` or `deactivated`: **pass** — the direction T-10-50 says would be wrong

### One more thing proved, cheaply

The four values were filled by hand into the tool's flush-left JSON. Re-running `go run ./tools/i18n -write` afterwards left all four files **byte-identical** (sha256 compared before and after). So the hand edit is exactly what the tool's own writer would have produced, and no reformatting diff is lying in wait for the next `-write`.

### The final report

```
1321 Zeichenketten im Quelltext
de-CH.json   73 Abweichungen, 0 ohne Gegenstück — wird von -schweiz erzeugt
en.json      1321 übersetzt, 0 offen, 0 verwaist
es.json      1321 übersetzt, 0 offen, 0 verwaist
fr-CH.json   4 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
fr.json      1321 übersetzt, 0 offen, 0 verwaist
it-CH.json   9 Abweichungen, 0 ohne Gegenstück — nur gelesen, von Hand gepflegt
it.json      1321 übersetzt, 0 offen, 0 verwaist
```

**Commit:** `7bc846f` — `i18n(10-09): die zwei Saetze ueber den zweiten Faktor, in vier Sprachen`

---

## Task 2 — the counting gates, and the baseline the next phase inherits

### The count gate, restated so it measures what its name claims

The plan asserts the source count rises **by exactly the number of sentences 10-06 added and by no more** — the gate proving nothing untranslatable was minted anywhere else in the phase. Read as an absolute against the phase-10 baseline it fails, and the plan's own German note explains why in advance: another phase moved it.

The phase-10 baseline commit is **`cdcfbab`** (`docs(state): zwei Tore standen als offen, die seit Tagen zu sind`, 2026-09-08T01:20:45+02:00) — the commit immediately before `63db4cf`, the first commit of plan 10-01. Running `tools/i18n -root` against a tree archived at that commit:

```
1311 Zeichenketten im Quelltext
en.json  1277 übersetzt, 34 offen, 0 verwaist    (es, fr, it identical)
```

So the count went **1311 → 1321, a rise of ten**, not two, and 34 keys stood open at the phase-10 baseline that are closed today. **Both are Phase 11's doing.**

Rather than adjust the number, the gate was re-stated in the form that carries the evidence: diff the **key sets**, not the totals. The key set was read with the tool's own collector — archive each tree, replace `en.json` with `{}`, run `-write`, read back the keys — so no hand-written regex re-implements the collection and no template shape can slip past it.

```
base keys: 1311   now keys: 1321
added: 10         removed: 0
```

| key (abridged) | first source commit | phase |
|---|---|---|
| `Ein Album mit diesem Namen gibt es schon` | `ad99438` feat(11-03) | 11 |
| `Ein anderes Album hat schon die Adresse, die aus diesem Namen entsteht` | `ad99438` feat(11-03) | 11 |
| `Bitte einen Namen für das Album angeben` | `ad99438` feat(11-03) | 11 |
| `Bitte ein Bild auswählen` | `ad99438` feat(11-03) | 11 |
| `Dieses Album ist voll. Leg für weitere Bilder ein zweites an.` | `ad99438` feat(11-03) | 11 |
| `Dieses Bild gehört nicht zur Mediathek dieser Website` | `ad99438` feat(11-03) | 11 |
| `Das Album oder das Bild gibt es nicht mehr` | `ad99438` feat(11-03) | 11 |
| `%s – dieses Album gibt es nicht mehr` | `fb32335` fix(11) CR-04 | 11 |
| `Du bist über die Anmeldung deiner Organisation …` | `8d6df09` feat(10-06) | **10** |
| `Wer sich über die Anmeldung der Organisation …` | `8d6df09` feat(10-06) | **10** |

**Ten added, eight Phase 11's, two Phase 10's, none removed.** The property the gate protects — *Phase 10 minted exactly two user-visible sentences and nothing else* — holds exactly, and now it holds with names attached rather than with a total that any concurrent phase can move.

**One subtlety the attribution surfaced.** Seven of the eight Phase 11 keys were written on 2026-09-07 at 19:03 (`ad99438`), *before* the phase-10 baseline commit — yet they appear as *added* since it. They became collectable only at `6efb3ba` (`fix(11): WR-01 the six album sentences become visible to tools/i18n`, 2026-09-08T02:01:51, forty minutes after the baseline). **The source count counts collectable strings, not German literals**: a commit that writes no sentence can raise it, and a commit that writes six can leave it flat. Worth carrying forward — a count gate stated in sentences is not the same gate as one stated in the tool's number.

### The whole-phase arithmetic, measured against the finished tree

`base` = `cdcfbab`, the phase-10 baseline. `now` = `HEAD` after this plan. Rows measured on the baseline with `git grep` / `git ls-tree` against the ref, so no checkout was needed and no working tree was disturbed.

| What | Plan: base | Plan: adds | Plan: now | **Measured base** | **Measured now** | |
|---|---|---|---|---|---|---|
| strings in source | vorher | 2 | vorher + 2 | 1311 | **1321** | **divergence — +10, of which 8 are Phase 11's** |
| open keys, en/es/fr/it | 0 | 0 | 0 | 34 | **0** | base divergence: 34 open at the phase baseline, all Phase 11's, closed by 11-07 |
| orphaned keys, en/es/fr/it | 0 | 0 | 0 | 0 | **0** | ✓ |
| catalogues changed | — | 5 | 5 | — | **4** | **divergence — de-CH.json byte-identical** |
| admin templates | vorher | 0 | vorher | 68 | **68** | ✓ |
| `layoutPageNames` entries | 50 | 0 | 50 | 50 | **50** | ✓ (set-identical, no name added or removed) |
| migrations | vorher | 0 | vorher | 50 | **51** | **divergence — Phase 11's `00051`; Phase 10 added none** |
| packages under `internal/` | vorher | 0 | vorher | 41 | **41** | ✓ |
| files in `internal/web` | 13 | 1 | 14 | 13 | **14** | ✓ |
| files in `internal/admin` | vorher | 1 | vorher + 1 | 51 | **52** | ✓ |
| `adminProtectedMux.Handle` | vorher | 0 | vorher | 159 | **159** | ✓ |
| `adminOnly` rows | 19 | 0 | 19 | 19 | **19** | ✓ (`adminOnlyRoutes` also unmoved: 65 → 65) |
| files using `RemoteAddr` | 3 | 0 | 3 | 3 | **3** | ✓ |
| `IsTrustedPeer` mentions | 3 | 1 | 4 | 3 | **4** | ✓ |
| `completeLogin` call sites | 3 | 1 | 4 | 3 | **5** | **divergence — the gate counts a comment; call sites are 4** |
| `MustHaveSecondFactor` non-test | 6 | 0 | 6 | 7 | **8** | **divergence — the gate counts two comments and a declaration; call sites are 5 → 5** |
| `SetRights` call sites | 2 | 2 | 4 | 2 | **4** | ✓ |
| files using `BeginTx` | 14 | 0 | 14 | 15 | **15** | **divergence — base was already 15; file set identical, delta 0** |

**Eleven rows match. Seven diverge. In all seven the property the row protects holds** — two because the gate counts prose, four because another phase moved an absolute the gate names, and the first because both.

### The divergences, one by one

#### 1. `completeLogin` prints 5, wants 4 — the sixth instance

The gate is `grep -rn completeLogin internal/ --include='*.go' | grep -v _test | grep -vc 'func (h \*Handler) completeLogin\|// completeLogin'`. The `-v` exclusions were written for the declaration and for a comment that *begins* with the symbol. What they miss:

```
internal/admin/forwardauth.go:241:	// Before the rotation and before the funnel, because completeLogin is
```

A comment that mentions the symbol **mid-sentence**. Five lines print; four of them are calls.

**Corrected command,** which counts call sites and cannot be tripped by prose:

```
grep -rn 'h\.completeLogin(' internal/ --include='*.go' | grep -v _test | wc -l
```

reads **3** at the baseline and **4** now — exactly the plan's `3 | +1 | 4`. The one new call site is `internal/admin/forwardauth.go:277`, plan 10-03's. The plan's prediction was right about the code and wrong about the number.

#### 2. `MustHaveSecondFactor` prints 8, wants 6 — the seventh instance

`grep -rn … | wc -l` counts every line mentioning the symbol. Three of the eight are not calls:

```
internal/auth/twofactor.go:38:// MustHaveSecondFactor decides who is required to set one up.
internal/auth/twofactor.go:60:func MustHaveSecondFactor(role string, viaSSO bool) bool {
internal/admin/twofactor.go:383:	// this person — the same fact MustHaveSecondFactor decides, read from the
```

The first two were there at the baseline (which is why the baseline measures **7**, not the plan's 6). The third is new — plan 10-06 wrote a prose comment, and the gate read it as growth. Actual **call sites are 5 at the baseline and 5 now**: four in `internal/admin/twofactor.go` and one inside `RequireSecondFactor`. The plan's *Phase adds 0* is true of the property and false of the number. Same family as `WINDOWS.md` entry 9 (the `HasGroup` gate that counted its own explanatory comment).

#### 3. `BeginTx` files: 15, not 14 — a stale absolute, not a drift

The gate's `14` was written on 2026-09-07. The measured file set is **identical at the baseline and now** — same fifteen files, delta 0 — so the property (*nothing in this phase opens a transaction*) holds exactly. The fifteenth is `internal/album/store.go`, Phase 11's, and 11-07 already recorded 15 as the measured value on 2026-09-08. Phase 10 inherited 15 and left it at 15.

#### 4. Migrations: 51, not 49 — two stale, both Phase 11's

Attributed with `git log --diff-filter=A`:

- `00050_albums.sql` — `a3e5780` `feat(11-02)`
- `00051_album_updated_at.sql` — `2e02be3` `fix(11): CR-01`

`git log --diff-filter=A cdcfbab..HEAD -- internal/db/migrations/` lists **only** `2e02be3`. **Phase 10 added zero migrations**, which is the phase-long claim, and it is now measured rather than promised. The plan's `49` predates `00050`; the plan's own baseline note, written a day later, already said 50.

#### 5. `catalogues changed`: 4, not 5

`de-CH.json` was regenerated by `-schweiz` and came out byte-identical, so it does not appear in the diff. The plan counted the rebuild as a change; the rebuild is a change only when the German gains a sharp s or German quotation marks, and neither sentence does.

#### 6. Open keys at the phase baseline: 34, not 0

The plan's table reads `open keys | 0 | 0 | 0`, i.e. the phase began on a closed catalogue. It did not: 34 keys stood open at `cdcfbab`, all of them Phase 11's albums, closed at 06:33 by `322f853` (`i18n(11-07)`) — four hours *into* Phase 10's execution. The row is right about now and wrong about then, and the cause is the same one the plan's note names.

### The correction phase 9 left behind, resolved

The plan asks whether phase 9's `09-06` predictions for `layoutPageNames` (48) and admin templates (65) were gap-closure landing after the summary, or predictions never reconciled. The git history says **gap-closure**:

- `layoutPageNames` measures **50** at the phase-10 baseline and 50 now — set-identical, nothing added or removed by Phase 10. The two beyond 48 are `album_list` and `album_edit`, Phase 11's, and 11-07 measured and recorded 50 (2 of them `album_`) on the finished Phase 11 tree.
- Admin templates measure **68** at the baseline and 68 now, and 11-07 measured 68 as well. The three beyond 65 arrived with the album screens.

So both are Phase 11 additions counted against a Phase 9 prediction, not a Phase 9 miscount. The phase-10 baselines used in these plans are the measured ones, and the next phase inherits **41 packages / 51 migrations / 68 admin templates / 1321 strings / 159 admin routes / 50 `layoutPageNames` / 15 `BeginTx` files / 19 `adminOnly` rows**, all measured on 2026-09-08 against `HEAD`.

### The plan's Task 2 gates, run verbatim

| gate | wants | prints | |
|---|---|---|---|
| `go run ./tools/i18n \| grep -c '0 offen, 0 verwaist'` | 4 | **4** | ✓ QUAL-01's mechanical half |
| `grep -rn RemoteAddr internal/ cmd/ … \| wc -l` | 3 | **3** | ✓ no fourth read appeared in eight plans |
| `grep -rn completeLogin … \| grep -vc …` | 4 | **5** | ✗ — comment; call sites are 4 |
| `grep -rln BeginTx … \| wc -l` | 14 | **15** | ✗ — stale absolute; file set unmoved |
| `ls internal/db/migrations/*.sql \| wc -l` | 49 | **51** | ✗ — stale absolute; Phase 10 added none |
| `go build ./... && go test ./...` | clean | **clean** | ✓ 44 `ok`, 0 `FAIL` |

---

## Verification

| Check | Result |
|---|---|
| `go run ./tools/i18n` — en, es, fr, it | **0 offen, 0 verwaist**, 1321 each |
| `go run ./tools/i18n` — de-CH, fr-CH, it-CH | **0 ohne Gegenstück** (73, 4, 9 deviations) |
| `de-CH.json` rebuilt with `-schweiz` | ran (`70 nach Regel, 3 von Hand`), byte-identical |
| `git status --porcelain` on `fr-CH.json`, `it-CH.json` | **0 lines** |
| `git diff --name-only 913ef6e..HEAD -- '*.go' '*.html'` | **0** |
| `gofmt -l .` | **0** |
| `go vet ./...` | exit 0, no output |
| `go build ./...` | exit 0 |
| `go test ./...` | 44 packages `ok`, **0 `FAIL`** (incl. `cmd/holzcloud`, `tools/i18n`, `internal/i18n`) |
| value-equals-key, unfiltered | en 28 / es 9 / fr 16 / it 15 — **before and after** |
| eight new values `!= ` their German key | pass |
| eight new values pairwise distinct | pass |
| none says switched off / disabled / désactivée / desactivada / disattivata | pass |
| catalogues re-`-write`n after the hand edit | byte-identical — hand fill == tool's writer |

Movement in the Go gates would have been a finding. There was none: this plan changed four JSON files and nothing else.

## Deviations from Plan

**1. [Rule 3 — blocking] The count gate could not be run as written**

- **Found during:** Task 2
- **Issue:** The plan's gate reads *"the count did not rise by exactly 2 from the value measured immediately before this plan"* in Task 1 and *"Phase baseline | 2 | vorher + 2"* in Task 2. Against the value immediately before this plan the rise is **0** (this plan mints nothing); against the phase-10 baseline it is **+10**. Neither is 2, and the plan's own German note predicts exactly this: Phase 11 landed in the shared tree between waves.
- **Fix:** Replaced the total with a **key-set diff** against the phase-10 baseline commit, read through the tool's own collector, naming all ten added keys and attributing each to its first source commit. Eight Phase 11, two Phase 10. The property held; the number never could.
- **Files modified:** none — a measurement, not a change.

**2. [Rule 2 — missing critical check] Two more gates measure something other than their name**

- **Found during:** Task 2
- **Issue:** `completeLogin` prints 5 (a mid-sentence comment slips past an exclusion written for comments that begin with the symbol); `MustHaveSecondFactor` prints 8 (a doc comment, a declaration, and one new prose comment from 10-06). Sixth and seventh instance in this phase, after the wrapped `slog.` line, the `-run` filter that matched nothing, the constant-count gate that counted its own doc comment, the gate a comment tripped, and the `^\./\.planning/` exclusion that never fired.
- **Fix:** Corrected commands recorded above and in `.planning/WINDOWS.md` (entry 12). Both properties verified with the corrected form: `completeLogin` 3 → 4, `MustHaveSecondFactor` 5 → 5.
- **Files modified:** `.planning/WINDOWS.md`

**3. [documented, not fixed] Four of the plan's absolute counting numbers are stale**

- **Found during:** Task 2
- **Issue:** migrations `49` (measured 51), `BeginTx` files `14` (measured 15), phase-baseline open keys `0` (measured 34), catalogues changed `5` (measured 4).
- **Action:** Recorded with cause and attribution above. **Not adjusted** — the plan says a divergence is a finding to write down, and three of the four are another phase's arithmetic showing through, which is the exact thing the plan's note asks the reader to expect.

**4. `WINDOWS.md` entry 10 closed**

- 10-06 entered `unmet-truth`: *"Neue Zeichenkette noch nicht uebersetzt: en/es/fr/it je 2 offen … Plan 10-09 schliesst sie mit tools/i18n -write."* This plan closed them; the entry is now `fixed`.

## Known Stubs

None. This plan added no code and no placeholder.

## Threat Flags

None. No network surface, no auth path, no file access and no schema changed — four JSON catalogues and one planning ledger.

The two threats the plan registers are both mitigated and measured:

- **T-10-50** (a security statement readable in only one language) — both sentences ship in five languages, gated on the tool's own `0 offen, 0 verwaist` rather than on a claim.
- **T-10-51** (a copied German value reported as translated) — measured before and after under two gates, plus the pairwise-distinctness and word-list checks; and the weakness of the plan's own `len(k) > 40` filter is named above rather than relied upon.
- **T-10-52** (a hand-edited regional catalogue) — `de-CH.json` regenerated, `fr-CH.json` and `it-CH.json` gated as untouched at 0 lines of `git status`.

## Notes for Plan 10-10 (the browser pass)

- The account-screen sentence renders only when `.ViaSSO` is true; the user-list sentence only when `.SSOEnabled` is true. Both are inside `<p class="callout">`.
- The eight translated values are quoted verbatim above. Comparing a screen against them needs no catalogue.
- **The catalogue proves presence, not rendering.** `WINDOWS.md` entry 8 is a live example from Phase 11: a string marked, collected, translated four times, `0 offen 0 verwaist` — and still German in the browser, because it was resolved at save time rather than at request time. Both of these sentences go through `{{t}}` at request time in an admin template, which is the path with no such trap, but the browser pass is the only thing that can say so.

## Self-Check: PASSED

- `.planning/phases/10-authentik/10-09-SUMMARY.md` — FOUND
- `internal/i18n/locales/{en,es,fr,it}.json` — FOUND, all four modified in `7bc846f`
- Commit `7bc846f` — FOUND in `git log`
- `internal/i18n/locales/de-CH.json` — FOUND, unchanged (correct)
- `.planning/WINDOWS.md` — FOUND, entry 10 `fixed`, entry 12 appended
