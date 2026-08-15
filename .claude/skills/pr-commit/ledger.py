#!/usr/bin/env python3
"""Append one review round to docs/ci/review-rounds.json.

The question the ledger answers: is reviewing BEFORE the push actually
worth its minutes? That is measurable — count what the local round found,
then count what the GitHub bot still found after the push. The second
number is the local round's misses, and if it stays at zero the local
round is carrying the review; if it stays high, it is theatre.

Usage:
  ledger.py --source coderabbit-cli --findings tmp/review-findings.jsonl \\
            --verdicts fix=6,reject=1 [--pr 60]

Verdicts come from triage, so they are passed in rather than guessed:
this script records, it does not judge.
"""
import argparse
import json
import os
import subprocess
from datetime import datetime, timezone

LEDGER = "docs/ci/review-rounds.json"
SCHEMA = 1


def git(*args: str) -> str:
    return subprocess.run(["git", *args], capture_output=True, text=True, check=False).stdout.strip()


def count_findings(path: str) -> tuple[int, dict[str, int]]:
    if not path or not os.path.exists(path):
        return 0, {}

    total, by_severity = 0, {}
    with open(path, errors="replace") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if event.get("type") == "finding":
                total += 1
                severity = event.get("severity", "unknown")
                by_severity[severity] = by_severity.get(severity, 0) + 1

    return total, by_severity


VERDICTS = ("fix", "defer", "reject")


def parse_verdicts(raw: str) -> dict[str, int]:
    """Only the three verdicts triage can reach, and only counts.

    A typo used to be recorded verbatim, which left `precision` computed
    over a denominator that silently omitted those findings — a wrong
    number is worse here than a crash, because nobody re-derives it."""
    verdicts = {}
    for pair in filter(None, (raw or "").split(",")):
        key, _, value = pair.partition("=")
        key = key.strip()
        if key not in VERDICTS:
            raise SystemExit(f"unknown verdict {key!r}: expected one of {', '.join(VERDICTS)}")
        count = int(value or 0)
        if count < 0:
            raise SystemExit(f"verdict {key!r} cannot be negative")
        verdicts[key] = count

    return verdicts


def load() -> dict:
    if os.path.exists(LEDGER):
        with open(LEDGER) as handle:
            return json.load(handle)

    return {"schema": SCHEMA, "rounds": [], "totals": {}}


def totals(rounds: list[dict]) -> dict:
    """Per-source rollup. Precision is what survives triage: a finder that
    reports ten issues of which one is real costs more than it saves."""
    out: dict[str, dict] = {}
    for round_ in rounds:
        source = round_["source"]
        bucket = out.setdefault(source, {"rounds": 0, "found": 0, "fixed": 0,
                                         "deferred": 0, "rejected": 0})
        bucket["rounds"] += 1
        bucket["found"] += round_.get("found", 0)
        for verdict in ("fix", "defer", "reject"):
            bucket[{"fix": "fixed", "defer": "deferred", "reject": "rejected"}[verdict]] += \
                round_.get("verdicts", {}).get(verdict, 0)

    for bucket in out.values():
        judged = bucket["fixed"] + bucket["deferred"] + bucket["rejected"]
        bucket["precision"] = round((bucket["fixed"] + bucket["deferred"]) / judged, 3) if judged else None

    return out


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True,
                        help="coderabbit-cli (pre-push) | coderabbit-github (post-push)")
    parser.add_argument("--findings", default="")
    parser.add_argument("--verdicts", default="", help="fix=N,defer=N,reject=N")
    parser.add_argument("--pr", type=int)
    parser.add_argument("--count", type=int, help="finding count when there is no findings file")
    parser.add_argument("--pre-commit", action="store_true",
                        help="the round reviewed work that is not committed yet")
    args = parser.parse_args()

    found, by_severity = count_findings(args.findings)
    if args.count is not None:
        found = args.count

    ledger = load()
    ledger["rounds"].append({
        "at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        # A pre-commit round reviews work that has no commit yet, so
        # HEAD is its PARENT. Recording HEAD there would file the round
        # against the previous change and quietly corrupt the comparison
        # the ledger exists for.
        "sha": ("uncommitted-on-" + git("rev-parse", "--short=7", "HEAD")
                if args.pre_commit else git("rev-parse", "--short=7", "HEAD")),
        "branch": git("rev-parse", "--abbrev-ref", "HEAD"),
        "pr": args.pr,
        "source": args.source,
        "found": found,
        "by_severity": by_severity,
        "verdicts": parse_verdicts(args.verdicts),
    })
    ledger["totals"] = totals(ledger["rounds"])

    os.makedirs(os.path.dirname(LEDGER), exist_ok=True)
    with open(LEDGER, "w") as handle:
        json.dump(ledger, handle, indent=2)
        handle.write("\n")

    print(f"{LEDGER}: recorded {found} finding(s) from {args.source}")


if __name__ == "__main__":
    main()
