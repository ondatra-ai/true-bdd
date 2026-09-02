# ClickUp tickets to create

List: `901523097822`   Tag: `other`   Source: a local review

4 ticket(s). One ClickUp task per `## ` heading below.

---

## 1. Guard the nil deref

### Why

CodeRabbit raised this on a local review; triage scored it **9/10**.

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
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `major`, source `thread`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.

---

## 2. Review finding

### Why

CodeRabbit raised this on a local review; triage scored it **6/10**.

> (no reason recorded)

### What to change

`docs/x.md:?`

nit

### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `minor`, source `body-only`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.

---

## 3. Sparse row

### Why

An unrecorded source raised this on a local review; triage scored it **0/10**.

> (no reason recorded)

### What to change



### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `?`, source `?`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.

---

## 4. Long body

### Why

The merge postmortem raised this on a local review; triage scored it **7/10**.

> long

### What to change

`a.go:1`

start héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héllo–wörld✅🟠 héll

### Verification

```bash
go run ./scripts/cmd/gates run
```

### Context

Reviewer severity `postmortem`, source `postmortem`.

`Triage Score`, `Triage Date` and `Triage Commit` say what this was
judged to be worth, when, and against which commit. `clickup triage`
re-reads the oldest of those against HEAD and either refreshes this body
or retires the ticket, so a score here is never older than its stamp.
