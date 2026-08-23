# ClickUp tickets to create

List: `901523097822`   Tag: `other`   Source: a local review

4 ticket(s). One ClickUp task per `## ` heading below.

---

## 1. Guard the nil deref

### Why

CodeRabbit raised this on a local review; triage scored it **9/10** — real, but not
worth blocking that merge on.

> A real crash path.

### What to change

`services/bdd-cli/main.go:42`

## What

`x` may be nil.

| a | b |
|---|---|
| 1 | 2 |

### Verification

```bash
./.claude/skills/pr-commit/gates.sh
```

### Context

Deferred rather than fixed inline: the merge loop caps fixing at two
review rounds, because CodeRabbit's free tier allows ~4 PR reviews an
hour and PR #76 exhausted that by fixing everything inline over four
rounds. Reviewer severity `major`, source `thread`.

---

## 2. Review finding

### Why

CodeRabbit raised this on a local review; triage scored it **6/10** — real, but not
worth blocking that merge on.

> (no reason recorded)

### What to change

`docs/x.md:?`

nit

### Verification

```bash
./.claude/skills/pr-commit/gates.sh
```

### Context

Deferred rather than fixed inline: the merge loop caps fixing at two
review rounds, because CodeRabbit's free tier allows ~4 PR reviews an
hour and PR #76 exhausted that by fixing everything inline over four
rounds. Reviewer severity `minor`, source `body-only`.

---

## 3. Sparse row

### Why

CodeRabbit raised this on a local review; triage scored it **0/10** — real, but not
worth blocking that merge on.

> (no reason recorded)

### What to change

`?:?`



### Verification

```bash
./.claude/skills/pr-commit/gates.sh
```

### Context

Deferred rather than fixed inline: the merge loop caps fixing at two
review rounds, because CodeRabbit's free tier allows ~4 PR reviews an
hour and PR #76 exhausted that by fixing everything inline over four
rounds. Reviewer severity `?`, source `?`.

---

## 4. Long body

### Why

CodeRabbit raised this on a local review; triage scored it **7/10** — real, but not
worth blocking that merge on.

> long

### What to change

`a.go:1`

start héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héll

### Verification

```bash
./.claude/skills/pr-commit/gates.sh
```

### Context

Deferred rather than fixed inline: the merge loop caps fixing at two
review rounds, because CodeRabbit's free tier allows ~4 PR reviews an
hour and PR #76 exhausted that by fixing everything inline over four
rounds. Reviewer severity `postmortem`, source `postmortem`.
