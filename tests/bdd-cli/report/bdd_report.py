#!/usr/bin/env python3
"""Render a self-contained HTML run report for a true-bdd BDD session.

The report's spine is a gap-free accounting of each fixture's wall clock:
every slice is tagged DETERMINISTIC (Go code whose duration is a property
of the machine) or NON-DETERMINISTIC (a model deciding how long to take).
The two must sum to the wall clock `go test` measured — if they don't, the
report is wrong and says so rather than hiding the remainder.

Sources:

  1. tmp/test_run/<session>/<fixture>/tmp/true-bdd.log.json
     The engine's own JSON log. Every AI turn leaves "Dispatching AI turn"
     (turn/label/role/cli/model), "AI turn usage" (claude only: cost +
     tokens) and "AI turn returned"/"AI turn failed" (duration_ms). The
     spans BETWEEN those records are the engine's deterministic work.

  2. The `go test -v` output (--gotest). Two things live only here: the
     per-fixture wall clock / verdict, and the harness judge's own slog
     records — the judge runs in the test process, so its cost never
     reaches the engine log.

  3. Test-runner artifacts inside the fixture tmpdir (e.g. Playwright's
     test-results/.last-run.json). Their mtimes pin when the engine's
     test-discovery run finished, which the engine itself does not log.

Produce the input it wants like this:

  go test -tags bdd ./tests/bdd-cli/... -v -timeout 30m > tmp/bdd-run.log 2>&1
  python3 tests/bdd-cli/report/bdd_report.py

Usage:
  python3 tests/bdd-cli/report/bdd_report.py
      [--session tmp/test_run/<dir>]   default: newest session
      [--gotest  tmp/bdd-run.log]      default: that path under the repo root
      [--out     tmp/bdd-report.html]

Paths default to the repo root (resolved from this file's location), so
the script runs the same from any working directory. Relative arguments
are resolved against the repo root too, not against cwd.

Requires the engine telemetry that `router.go` and `claude_provider.go`
emit — per-turn `duration_ms` / `role` and the `AI turn usage` record.
Against a log without those, durations and costs come out empty.
"""

import argparse
import html
import json
import re
from datetime import datetime
from pathlib import Path

# tests/bdd-cli/report/bdd_report.py -> repo root. Every default path
# hangs off this rather than off cwd, so the script behaves the same
# whether it is run from the repo root, from its own directory, or by an
# editor with some third working directory.
REPO_ROOT = Path(__file__).resolve().parents[3]


def at_root(path):
    """Resolve a path argument against the repo root, not against cwd."""
    candidate = Path(path)
    return candidate if candidate.is_absolute() else REPO_ROOT / candidate

# Role colors: validated colorblind-safe triple. Every role is also
# labeled in text, so color is never load-bearing.
ROLE_COLORS = {
    "prompt": "#2563eb",
    "fix": "#ea580c",
    "apply": "#059669",
    "judge": "#7c3aed",
}
ROLE_ORDER = ["prompt", "fix", "apply", "judge"]

DETERMINISTIC_COLOR = "#94a3b8"
BOOT_COLOR = "#64748b"

TOKEN_KEYS = [
    "input_tokens",
    "output_tokens",
    "cache_read_input_tokens",
    "cache_creation_input_tokens",
]

# Artifacts a framework's runner writes when it finishes. Used only to
# date the engine's discovery run, which leaves no log record of its own.
RUNNER_ARTIFACTS = [
    ("playwright", "**/test-results/.last-run.json"),
    ("jest", "**/coverage/coverage-final.json"),
]


# ---------------------------------------------------------------- parsing


def parse_ts(raw):
    """slog writes RFC3339 with nanoseconds; datetime wants <=6 digits.

    Normalised to naive LOCAL time so log records, artifact mtimes and the
    `go test` stderr stamps — three sources with three different tz
    conventions — sit on one axis.
    """
    if not raw:
        return None
    try:
        parsed = datetime.fromisoformat(re.sub(r"(\.\d{6})\d+", r"\1", raw))
    except ValueError:
        return None
    if parsed.tzinfo is not None:
        parsed = parsed.astimezone().replace(tzinfo=None)
    return parsed


def load_engine_log(path):
    out = []
    with open(path, encoding="utf-8", errors="replace") as handle:
        for line in handle:
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            out.append((parse_ts(rec.get("time")), rec))
    return out


def collect_turns(records):
    """Fold the per-turn records into one dict per turn.

    Also captures the two intra-turn boundaries the claude provider's
    message stream exposes: the first AssistantMessage (the CLI has
    finished booting and the model has started producing) and the
    ResultMessage. That splits a turn into CLI boot / generation /
    teardown. crush and codex stream nothing, so their turns stay opaque.
    """
    turns, order, open_turn, last_ts = {}, [], None, None

    for ts, rec in records:
        if ts is not None:
            last_ts = ts
        msg = rec.get("msg")

        if msg == "Dispatching AI turn":
            num = rec.get("turn")
            turn = {
                "turn": num,
                "role": rec.get("role"),
                "cli": rec.get("cli"),
                "model": rec.get("model"),
                "started": ts,
                "ended": None,
                "first_output": None,
                "result_at": None,
                "duration_s": None,
                "status": "open",
                "error": None,
                "cost_usd": None,
                "tokens": {},
            }
            turns[num] = turn
            order.append(num)
            open_turn = turn

        elif open_turn is None:
            continue

        elif msg == "AssistantMessage received":
            if open_turn["first_output"] is None:
                open_turn["first_output"] = ts

        elif msg in ("ResultMessage received", "CLI transcript saved"):
            if open_turn["result_at"] is None:
                open_turn["result_at"] = ts

        elif msg == "AI turn usage":
            if rec.get("cost_usd") is not None:
                open_turn["cost_usd"] = float(rec["cost_usd"])
            for key in TOKEN_KEYS:
                if rec.get(key) is not None:
                    open_turn["tokens"][key] = int(rec[key])

        elif msg in ("AI turn returned", "AI turn failed"):
            turn = turns.get(rec.get("turn"), open_turn)
            if rec.get("duration_ms") is not None:
                turn["duration_s"] = float(rec["duration_ms"]) / 1000.0
            turn["ended"] = ts
            turn["status"] = "ok" if msg == "AI turn returned" else "failed"
            turn["error"] = rec.get("error")
            open_turn = None

    # A turn still open at EOF has no duration record: either the process
    # was killed mid-turn, or it is still running. Which one is not
    # knowable from the log, so both are reported as "no completion
    # record" with a lower-bound elapsed.
    for num in order:
        turn = turns[num]
        if turn["status"] == "open" and turn["started"] and last_ts:
            turn["duration_s"] = max(
                0.0, (last_ts - turn["started"]).total_seconds()
            )
            turn["duration_is_floor"] = True
            turn["ended"] = last_ts

    return [turns[num] for num in order], last_ts


GOTEST_VERDICT_RE = re.compile(
    r"^\s*--- (PASS|FAIL|SKIP): TestBDDFixtures/(\S+) \(([\d.]+)s\)"
)
# The judge runs in the test process, where slog is unconfigured and
# writes its default text format to stderr.
GOTEST_USAGE_RE = re.compile(
    r"^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) INFO AI turn usage (.+)$"
)


def parse_gotest(path):
    """Per-fixture wall clock + verdict, and the harness judge's usage."""
    verdicts, judges = {}, []
    if not path or not Path(path).exists():
        return verdicts, judges

    for line in Path(path).read_text(encoding="utf-8", errors="replace").splitlines():
        match = GOTEST_VERDICT_RE.match(line)
        if match:
            verdict, name, secs = match.groups()
            verdicts[name] = {"verdict": verdict, "wall_s": float(secs)}
            continue

        match = GOTEST_USAGE_RE.match(line)
        if match:
            stamp, rest = match.groups()
            fields = dict(
                pair.split("=", 1) for pair in rest.split() if "=" in pair
            )
            judges.append(
                {
                    "at": datetime.strptime(stamp, "%Y/%m/%d %H:%M:%S"),
                    "cost_usd": float(fields.get("cost_usd", 0) or 0),
                    "tokens": sum(
                        int(fields.get(key, 0) or 0) for key in TOKEN_KEYS
                    ),
                }
            )
    return verdicts, judges


# The record the engine writes immediately AFTER `runner.Discover`
# returns. Discovery itself logs nothing, so this is what closes its
# span. Log-derived on purpose: artifact mtimes in a preserved tmpdir can
# be overwritten by anyone who later re-runs the suite by hand, and a
# measurement that quietly changes under inspection is worse than none.
DISCOVERY_END_MARKERS = ("Loading full checklist", "Loaded prompts")

# The synthetic subject the playwright runner emits when the suite could
# not start (playwright_runner.go: playwrightStartupMarker).
STARTUP_MARKER = "<startup>"


def find_discovery(tmpdir, records, window_start, window_end):
    """Bound the engine's test-run span, and say whether tests ran.

    Duration comes from the log; the verdict comes from the prompt
    artifacts the engine wrote during the run. A subject carrying the
    runner's startup marker means Discover got zero real failures back
    and synthesized a placeholder — i.e. the suite never started, so the
    span is startup cost, not test-execution cost.
    """
    end = None
    for ts, rec in records:
        if rec.get("msg") in DISCOVERY_END_MARKERS and ts and ts <= window_end:
            end = ts
            break
    if end is None or end <= window_start:
        return None, None, None, None

    framework, outcome = None, None
    for path in sorted(Path(tmpdir).glob("tmp/*/*user.txt")):
        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        if STARTUP_MARKER in text:
            framework = "playwright" if "playwright" in text.lower() else "runner"
            outcome = (
                "the engine synthesized its startup marker subject, so the "
                "runner returned zero real test failures — the suite never "
                "started, making this startup cost, not test-execution cost"
            )
            break
    if framework is None:
        framework = "test runner"

    return framework, end, None, outcome


EMPTY_FAILURE_RE = re.compile(
    r"Last Failure Output\s*\n+```[a-z]*\s*\n\s*```", re.IGNORECASE
)


def find_empty_failure_prompts(tmpdir):
    """Prompts that describe a failing test but carry no failure text.

    Playwright reports a webServer startup failure in its JSON report's
    top-level `errors[]`, not on stderr — so a synthesized startup
    subject can reach the model with an empty failure block. That is
    invisible in timings but explains a lot about what the fix turn had
    to work with.
    """
    hits = []
    for path in sorted(Path(tmpdir).glob("tmp/*/*user.txt")):
        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        if EMPTY_FAILURE_RE.search(text):
            hits.append(path.name)
    return hits


def read_runner_outcome(path):
    """Summarise a runner artifact: did any test actually report?"""
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None
    if not isinstance(data, dict) or "status" not in data:
        return None
    results = data.get("failedTests")
    if data.get("status") == "failed" and isinstance(results, list) and not results:
        return (
            "status \"failed\" with zero test results — the suite never "
            "started, so this is startup cost, not test-execution cost"
        )
    return f"runner status \"{data.get('status')}\""


# ------------------------------------------------------------ phase model


def build_phases(fixture):
    """Gap-free ordered slices of one fixture's wall clock.

    Everything is measured except the leading harness block, which is a
    residual: `go test` reports the subtest's total but never stamps when
    it began, so prep is whatever the other measured slices do not claim.
    """
    turns = fixture["turns"]
    first_ts, last_ts = fixture["first_ts"], fixture["last_ts"]
    wall = fixture["wall_s"]
    phases = []

    # --- trailing harness block, measured from the judge's own record.
    post_s = None
    if fixture["judge"] and last_ts:
        post_s = max(0.0, (fixture["judge"]["at"] - last_ts).total_seconds())

    # --- leading harness block: the residual.
    if wall and first_ts and last_ts:
        engine_span = (last_ts - first_ts).total_seconds()
        pre_s = wall - engine_span - (post_s or 0.0)
        phases.append(
            {
                "label": "Fixture prep",
                "detail": "repo-layer copy, input overlay, prep commands "
                          "(npm / playwright install), pre-run snapshot",
                "kind": "deterministic",
                "owner": "harness",
                "seconds": max(0.0, pre_s),
                "measured": False,
            }
        )

    # --- engine work before the first model call.
    first_dispatch = turns[0]["started"] if turns else last_ts
    if first_ts and first_dispatch:
        framework, disco_end, _artifact, outcome = fixture["discovery"]
        if disco_end:
            detail = (f"engine spawned the {framework} runner to find failing "
                      f"tests · bounded by the next engine log record")
            if outcome:
                detail += f" · {outcome}"
            phases.append(
                {
                    "label": f"Test run ({framework})",
                    "detail": detail,
                    "kind": "deterministic",
                    "owner": "tests",
                    "seconds": (disco_end - first_ts).total_seconds(),
                    "measured": True,
                }
            )
            phases.append(
                {
                    "label": "Checklist load + prompt render",
                    "detail": "checklist parse, template render, prompt "
                              "artifacts written to tmp/",
                    "kind": "deterministic",
                    "owner": "engine",
                    "seconds": (first_dispatch - disco_end).total_seconds(),
                    "measured": True,
                }
            )
        else:
            phases.append(
                {
                    "label": "Engine start-up",
                    "detail": "architecture load, test discovery, checklist "
                              "parse, template render",
                    "kind": "deterministic",
                    "owner": "engine",
                    "seconds": (first_dispatch - first_ts).total_seconds(),
                    "measured": True,
                }
            )

    # --- the turns, with the engine's own work between them.
    prev_end = None
    for turn in turns:
        if prev_end and turn["started"]:
            gap = (turn["started"] - prev_end).total_seconds()
            phases.append(
                {
                    "label": "Engine between turns",
                    "detail": "parse the model's result file, write "
                              "artifacts, build the next prompt",
                    "kind": "deterministic",
                    "owner": "engine",
                    "seconds": max(0.0, gap),
                    "measured": True,
                }
            )
        phases.append(
            {
                "label": f"Turn #{turn['turn']} — {turn['role']}",
                "detail": f"{turn['cli']} · {turn['model']}",
                "kind": "model",
                "role": turn["role"],
                "seconds": turn["duration_s"] or 0.0,
                "measured": True,
                "turn": turn,
            }
        )
        prev_end = turn["ended"]

    # --- engine work after the last turn.
    if prev_end and last_ts:
        tail = (last_ts - prev_end).total_seconds()
        if tail > 0.0005:
            phases.append(
                {
                    "label": "Engine shutdown",
                    "detail": "final verdict, error wrapping, log flush",
                    "kind": "deterministic",
                    "owner": "engine",
                    "seconds": tail,
                    "measured": True,
                }
            )

    # --- trailing harness block.
    if post_s is not None:
        judge = fixture["judge"]
        phases.append(
            {
                "label": "Post-run + judge",
                "detail": "post-run snapshot and diff, fixture teardown "
                          "(docker compose down), then the harness judge "
                          "call on claude · sonnet",
                "kind": "mixed",
                "owner": "harness",
                "role": "judge",
                "seconds": post_s,
                "measured": True,
                "cost_usd": judge["cost_usd"],
                "tokens": judge["tokens"],
            }
        )

    # Position every slice on the wall-clock axis for the gantt.
    offset = 0.0
    for phase in phases:
        phase["offset"] = offset
        offset += phase["seconds"]
    fixture["phase_total"] = offset

    return phases


def read_cmd(fixture_name):
    path = REPO_ROOT / "tests/bdd-cli/fixtures" / fixture_name / "fixture.yaml"
    if not path.exists():
        return ""
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.startswith("cmd:"):
            return line.split(":", 1)[1].strip()
    return ""


def load_fixtures(session_dir, verdicts, judges):
    fixtures = []
    for child in sorted(Path(session_dir).iterdir()):
        if not child.is_dir():
            continue
        log_path = child / "tmp" / "true-bdd.log.json"
        if not log_path.exists():
            continue

        records = load_engine_log(log_path)
        turns, last_ts = collect_turns(records)
        first_ts = next((ts for ts, _ in records if ts), None)
        meta = verdicts.get(child.name, {})

        fixture = {
            "name": child.name,
            "cmd": read_cmd(child.name),
            "turns": turns,
            "first_ts": first_ts,
            "last_ts": last_ts,
            "wall_s": meta.get("wall_s"),
            "verdict": meta.get("verdict"),
            "judge": judges[len(fixtures)] if len(fixtures) < len(judges) else None,
            "discovery": find_discovery(child, records, first_ts, turns[0]["started"])
            if turns and first_ts
            else (None, None, None, None),
            "empty_failure_prompts": find_empty_failure_prompts(child),
            "model_s": sum(t["duration_s"] or 0.0 for t in turns),
            "cost": sum(t["cost_usd"] or 0.0 for t in turns),
            "tokens": sum(sum(t["tokens"].values()) for t in turns),
        }
        fixture["phases"] = build_phases(fixture)
        fixtures.append(fixture)
    return fixtures


# ------------------------------------------------------------- formatting


def fmt_dur(seconds):
    if seconds is None:
        return "—"
    if seconds < 1:
        return f"{seconds * 1000:.0f}ms"
    if seconds < 60:
        return f"{seconds:.1f}s"
    minutes = int(seconds // 60)
    return f"{minutes}m {seconds - minutes * 60:04.1f}s"


def fmt_money(value):
    if not value:
        return "—"
    return f"${value:,.4f}"


def fmt_int(value):
    if not value:
        return "—"
    return f"{value:,}"


def esc(value):
    return html.escape(str(value if value is not None else ""))


def bar(pct, color, height=8):
    pct = max(0.0, min(100.0, pct))
    return (
        f'<div class="bar" style="height:{height}px">'
        f'<span style="width:{pct:.2f}%;background:{color}"></span></div>'
    )


def boot_total_for_findings(fixtures):
    """Seconds spent booting CLI sessions, across every splittable turn."""
    total = 0.0
    for fixture in fixtures:
        for turn in fixture["turns"]:
            if turn["first_output"] and turn["started"]:
                total += (turn["first_output"] - turn["started"]).total_seconds()
    return total


def phase_color(phase):
    if phase["kind"] == "deterministic":
        return DETERMINISTIC_COLOR
    return ROLE_COLORS.get(phase.get("role"), "#64748b")


def role_dot(role):
    color = ROLE_COLORS.get(role, "#64748b")
    return f'<span class="dot" style="background:{color}"></span>{esc(role or "—")}'


def verdict_chip(verdict):
    if verdict == "PASS":
        return '<span class="chip pass">PASS</span>'
    if verdict == "FAIL":
        return '<span class="chip fail">FAIL</span>'
    return '<span class="chip none">—</span>'


# ---------------------------------------------------------------- render


def render(fixtures, session_name, out_path):
    total_wall = sum(f["wall_s"] or 0.0 for f in fixtures)
    all_phases = [(f, p) for f in fixtures for p in f["phases"]]
    det_s = sum(p["seconds"] for _, p in all_phases if p["kind"] == "deterministic")
    model_s = sum(p["seconds"] for _, p in all_phases if p["kind"] != "deterministic")
    engine_cost = sum(f["cost"] for f in fixtures)
    judge_cost = sum(
        f["judge"]["cost_usd"] for f in fixtures if f["judge"]
    )
    total_tokens = sum(f["tokens"] for f in fixtures) + sum(
        f["judge"]["tokens"] for f in fixtures if f["judge"]
    )
    all_turns = [(f, t) for f in fixtures for t in f["turns"]]
    max_turn = max((t["duration_s"] or 0.0 for _, t in all_turns), default=1.0) or 1.0
    passed = sum(1 for f in fixtures if f["verdict"] == "PASS")

    parts = [HEAD]
    parts.append(f"""
<header>
  <h1>TrueBDD fixture suite — run report</h1>
  <p class="sub">Session <code>{esc(session_name)}</code> ·
     {len(fixtures)} fixture(s) with engine logs · {passed}/{len(fixtures)} passed</p>
</header>
""")

    det_pct = (det_s / total_wall * 100) if total_wall else 0.0
    tiles = [
        ("Wall clock", fmt_dur(total_wall) if total_wall else "—",
         "measured by go test"),
        ("Non-deterministic", fmt_dur(model_s),
         f"{100 - det_pct:.1f}% — models deciding"),
        ("Deterministic", fmt_dur(det_s),
         f"{det_pct:.1f}% — engine + harness code"),
        ("Cost", fmt_money(engine_cost + judge_cost),
         f"{fmt_money(engine_cost)} engine + {fmt_money(judge_cost)} judge"),
        ("Tokens", fmt_int(total_tokens), "in + out + cache"),
    ]
    parts.append('<div class="tiles">')
    for label, value, note in tiles:
        parts.append(
            f'<div class="tile"><div class="k">{esc(label)}</div>'
            f'<div class="v">{esc(value)}</div>'
            f'<div class="n">{esc(note)}</div></div>'
        )
    parts.append("</div>")

    # ---- findings the timeline itself implies. Each one is emitted only
    # when the underlying data supports it, so an uneventful run gets no
    # section at all rather than a reassuring empty one.
    findings = []
    for fixture in fixtures:
        framework, disco_end, _artifact, outcome = fixture["discovery"]
        if outcome and "never started" in outcome:
            span = (
                (disco_end - fixture["first_ts"]).total_seconds()
                if disco_end and fixture["first_ts"]
                else None
            )
            findings.append(
                ("The suite never ran — zero tests executed",
                 f"<code>{esc(fixture['name'])}</code>'s {fmt_dur(span)} "
                 f"{esc(framework)} slice is a <strong>startup failure</strong>, "
                 f"not a test run: the engine fell back to its "
                 f"<code>&lt;startup&gt;</code> marker subject, which it only "
                 f"emits when the runner exits non-zero having reported no test "
                 f"results at all. Everything downstream — every turn, every "
                 f"dollar — is driven by that placeholder, not by a real failing "
                 f"test.")
            )
        if fixture["empty_failure_prompts"]:
            findings.append(
                ("The fix turn was handed an empty failure",
                 f"{len(fixture['empty_failure_prompts'])} prompt(s) in "
                 f"<code>{esc(fixture['name'])}</code> contain a "
                 f"<em>Last Failure Output</em> block with nothing in it. The "
                 f"runner's startup subject fills that field from the child "
                 f"process's <strong>stderr</strong>, but Playwright reports a "
                 f"webServer startup failure in its JSON report's top-level "
                 f"<code>errors[]</code> on stdout — which the parsed report "
                 f"struct does not keep. The model had to infer the failure it "
                 f"was asked to fix.")
            )
    if boot_deficit := (boot_total_for_findings(fixtures) - det_s):
        if boot_deficit > 0:
            findings.append(
                ("CLI boot outweighs all engine logic",
                 f"{fmt_dur(boot_total_for_findings(fixtures))} went to starting "
                 f"CLI sessions before a single token was generated — more than "
                 f"the {fmt_dur(det_s)} of deterministic work in the whole run. "
                 f"Every turn opens a fresh session, so the boot is paid per "
                 f"turn, not per run.")
            )
    if findings:
        parts.append('<section><h2>What the timeline says</h2>')
        parts.append('<div class="findings">')
        for title, body in findings:
            parts.append(
                f'<div class="finding"><div class="ft">{title}</div>'
                f"<div class=\"fb\">{body}</div></div>"
            )
        parts.append("</div></section>")

    # ---- the headline split
    parts.append('<section><h2>Deterministic vs non-deterministic</h2>')
    parts.append(
        '<p class="lede">Every slice of the fixture\'s wall clock, in order, '
        "tagged by what governs its duration. <strong>Deterministic</strong> "
        "is Go code — engine start-up, test discovery, prompt rendering, "
        "snapshots, installs — whose cost is a property of the machine. "
        "<strong>Non-deterministic</strong> is a model deciding how long to "
        "take. The slices are contiguous and sum to the wall clock, so "
        "nothing hides in a residual.</p>"
    )
    parts.append(
        '<div class="legend">'
        f'<span class="sw" style="background:{DETERMINISTIC_COLOR}"></span>deterministic'
        f'<span class="sw" style="background:{ROLE_COLORS["prompt"]}"></span>prompt'
        f'<span class="sw" style="background:{ROLE_COLORS["fix"]}"></span>fix'
        f'<span class="sw" style="background:{ROLE_COLORS["apply"]}"></span>apply'
        f'<span class="sw" style="background:{ROLE_COLORS["judge"]}"></span>judge'
        "</div>"
    )

    for fixture in fixtures:
        wall = fixture["wall_s"] or fixture["phase_total"] or 1.0
        parts.append(
            '<div class="trace"><div class="trace-head">'
            f'<span class="name">{esc(fixture["name"])}</span>'
            f'<span class="meta">{fmt_dur(fixture["wall_s"])} wall · '
            f'accounted {fmt_dur(fixture["phase_total"])}</span></div>'
        )
        parts.append(
            '<table class="phases"><thead><tr><th>Phase</th><th>Kind</th>'
            '<th class="num">Duration</th><th class="num">% wall</th>'
            "<th>Position in the run</th></tr></thead><tbody>"
        )
        for phase in fixture["phases"]:
            pct = phase["seconds"] / wall * 100
            left = phase["offset"] / wall * 100
            color = phase_color(phase)
            kind = phase["kind"]
            kind_html = {
                "deterministic": '<span class="tag det">deterministic</span>',
                "model": '<span class="tag mod">non-deterministic</span>',
                "mixed": '<span class="tag mix">mixed</span>',
            }[kind]
            note = "" if phase["measured"] else ' <span class="approx">residual</span>'
            parts.append(
                f'<tr><td><div class="name">{esc(phase["label"])}{note}</div>'
                f'<div class="detail">{esc(phase["detail"])}</div></td>'
                f"<td>{kind_html}</td>"
                f'<td class="num">{fmt_dur(phase["seconds"])}</td>'
                f'<td class="num">{pct:.1f}%</td>'
                f'<td class="wide"><div class="gantt">'
                f'<span style="margin-left:{left:.2f}%;width:{max(pct, 0.4):.2f}%;'
                f'background:{color}"></span></div></td></tr>'
            )
        parts.append("</tbody></table></div>")
    parts.append("</section>")

    # ---- deterministic detail, split by who owns the time
    parts.append('<section><h2>Where the Go-side time goes</h2>')
    parts.append(
        '<p class="lede">Not all deterministic time belongs to the engine. '
        "Three different things run Go-or-shell code here, and only one of "
        "them is code true-bdd could make faster.</p>"
    )

    owners = {
        "engine": {
            "title": "Engine logic",
            "note": "config, checklist parse, template render, result "
                    "parsing between turns — true-bdd's own code",
        },
        "tests": {
            "title": "Test subprocess",
            "note": "the framework runner the engine spawns (go test / "
                    "jest / npx playwright) — the project's tests, not "
                    "the engine",
        },
        "harness": {
            "title": "BDD harness",
            "note": "fixture scaffolding that exists only because this is "
                    "a test: tmpdir overlay, npm / playwright install, "
                    "snapshots, diff, teardown, judge",
        },
    }
    for key, meta in owners.items():
        meta["seconds"] = sum(
            p["seconds"] for _, p in all_phases if p.get("owner") == key
        )
    owner_total = sum(m["seconds"] for m in owners.values()) or 1.0

    # The harness's total is not purely Go: the post-run block it owns is
    # measured as one span covering the snapshot, the diff, teardown AND
    # the judge's model call. Saying so on the tile keeps the number from
    # reading as "7.7s of Go code".
    mixed_s = sum(
        p["seconds"] for _, p in all_phases
        if p["kind"] == "mixed" and p.get("owner") == "harness"
    )
    parts.append('<div class="tiles">')
    for key, meta in owners.items():
        note = f"{meta['seconds'] / (total_wall or 1) * 100:.1f}% of wall clock"
        if key == "harness" and mixed_s:
            note = (f"incl. {fmt_dur(mixed_s)} post-run block that also "
                    f"contains the judge call")
        parts.append(
            f'<div class="tile"><div class="k">{esc(meta["title"])}</div>'
            f'<div class="v">{fmt_dur(meta["seconds"])}</div>'
            f'<div class="n">{esc(note)}</div></div>'
        )
    parts.append("</div>")
    parts.append('<table><thead><tr><th>Owner</th>'
                 '<th class="num">Total</th><th class="num">Share</th>'
                 "<th>Relative</th></tr></thead><tbody>")
    for key, meta in sorted(owners.items(), key=lambda kv: -kv[1]["seconds"]):
        parts.append(
            f'<tr><td><div class="name">{esc(meta["title"])}</div>'
            f'<div class="detail">{esc(meta["note"])}</div></td>'
            f'<td class="num">{fmt_dur(meta["seconds"])}</td>'
            f'<td class="num">{meta["seconds"] / owner_total * 100:.1f}%</td>'
            f'<td class="wide">'
            f'{bar(meta["seconds"] / owner_total * 100, DETERMINISTIC_COLOR)}</td>'
            "</tr>"
        )
    parts.append("</tbody></table>")

    parts.append(
        '<p class="lede">Itemised, every deterministic slice — plus the '
        "mixed post-run block, which the harness owns:</p>"
    )
    det_rows = {}
    for _, phase in all_phases:
        if phase["kind"] == "model":
            continue
        entry = det_rows.setdefault(
            phase["label"],
            {"s": 0.0, "n": 0, "detail": phase["detail"],
             "owner": phase.get("owner", "—")},
        )
        entry["s"] += phase["seconds"]
        entry["n"] += 1
    max_det = max((e["s"] for e in det_rows.values()), default=1.0) or 1.0
    parts.append('<table><thead><tr><th>Step</th><th>Owner</th>'
                 '<th class="num">Count</th><th class="num">Total</th>'
                 "<th>Relative</th></tr></thead><tbody>")
    for label, entry in sorted(det_rows.items(), key=lambda kv: -kv[1]["s"]):
        parts.append(
            f'<tr><td><div class="name">{esc(label)}</div>'
            f'<div class="detail">{esc(entry["detail"])}</div></td>'
            f'<td><span class="tag det">{esc(owners.get(entry["owner"], {}).get("title", entry["owner"]))}</span></td>'
            f'<td class="num">{entry["n"]}</td>'
            f'<td class="num">{fmt_dur(entry["s"])}</td>'
            f'<td class="wide">{bar(entry["s"] / max_det * 100, DETERMINISTIC_COLOR)}</td>'
            "</tr>"
        )
    parts.append("</tbody></table></section>")

    # ---- inside the model turns
    parts.append('<section><h2>Inside the model turns</h2>')
    parts.append(
        '<p class="lede">A model turn is not all model. The claude provider '
        "streams its session messages, so each turn splits into <strong>CLI "
        "boot</strong> (process spawn, SessionStart hooks, MCP server and "
        "plugin init — paid per turn because every turn is a fresh session), "
        "<strong>generation</strong>, and result teardown. crush and codex "
        "stream nothing, so their turns cannot be split.</p>"
    )
    parts.append('<table><thead><tr><th>Turn</th><th>Runs on</th>'
                 '<th class="num">CLI boot</th><th class="num">Generation</th>'
                 '<th class="num">Teardown</th><th class="num">Total</th>'
                 "<th>Split</th></tr></thead><tbody>")
    boot_total = 0.0
    for fixture, turn in all_turns:
        total = turn["duration_s"] or 0.0
        if turn["first_output"] and turn["started"]:
            boot = (turn["first_output"] - turn["started"]).total_seconds()
            gen = (
                (turn["result_at"] - turn["first_output"]).total_seconds()
                if turn["result_at"]
                else None
            )
            tail = (
                (turn["ended"] - turn["result_at"]).total_seconds()
                if turn["result_at"] and turn["ended"]
                else None
            )
            boot_total += boot
            segs = [
                (boot, BOOT_COLOR),
                (gen or 0.0, ROLE_COLORS.get(turn["role"], "#64748b")),
                (tail or 0.0, "#cbd5e1"),
            ]
            split = '<div class="split">' + "".join(
                f'<span style="width:{(s / total * 100) if total else 0:.2f}%;'
                f'background:{c}"></span>'
                for s, c in segs
            ) + "</div>"
        else:
            boot = gen = tail = None
            split = '<div class="opaque">no stream — CLI is opaque</div>'
        parts.append(
            f'<tr><td>#{esc(turn["turn"])} {role_dot(turn["role"])}</td>'
            f'<td><code>{esc(turn["cli"])}</code></td>'
            f'<td class="num">{fmt_dur(boot)}</td>'
            f'<td class="num">{fmt_dur(gen)}</td>'
            f'<td class="num">{fmt_dur(tail)}</td>'
            f'<td class="num">{fmt_dur(total)}</td>'
            f'<td class="wide">{split}</td></tr>'
        )
    parts.append("</tbody></table>")
    if boot_total:
        parts.append(
            f'<p class="lede">Boot tax across the suite: '
            f"<strong>{fmt_dur(boot_total)}</strong> spent starting CLI "
            "sessions before any token was generated.</p>"
        )
    parts.append("</section>")

    # ---- cost by role
    parts.append('<section><h2>Cost and time by role</h2>')
    parts.append('<table><thead><tr><th>Role</th><th>Runs on</th>'
                 '<th class="num">Turns</th><th class="num">Time</th>'
                 '<th class="num">Tokens</th><th class="num">Cost</th>'
                 "<th>Share of model time</th></tr></thead><tbody>")
    by_role = {}
    for _, turn in all_turns:
        entry = by_role.setdefault(
            turn["role"] or "—",
            {"turns": 0, "time": 0.0, "cost": 0.0, "tokens": 0, "clis": set()},
        )
        entry["turns"] += 1
        entry["time"] += turn["duration_s"] or 0.0
        entry["cost"] += turn["cost_usd"] or 0.0
        entry["tokens"] += sum(turn["tokens"].values())
        if turn["cli"]:
            entry["clis"].add(turn["cli"])
    for fixture in fixtures:
        if not fixture["judge"]:
            continue
        entry = by_role.setdefault(
            "judge",
            {"turns": 0, "time": 0.0, "cost": 0.0, "tokens": 0, "clis": {"claude"}},
        )
        entry["turns"] += 1
        entry["cost"] += fixture["judge"]["cost_usd"]
        entry["tokens"] += fixture["judge"]["tokens"]
    ordered = [r for r in ROLE_ORDER if r in by_role] + [
        r for r in by_role if r not in ROLE_ORDER
    ]
    for role in ordered:
        entry = by_role[role]
        pct = (entry["time"] / model_s * 100) if model_s else 0.0
        time_cell = fmt_dur(entry["time"]) if entry["time"] else "—"
        parts.append(
            f"<tr><td>{role_dot(role)}</td>"
            f"<td><code>{esc(', '.join(sorted(entry['clis'])))}</code></td>"
            f'<td class="num">{entry["turns"]}</td>'
            f'<td class="num">{time_cell}</td>'
            f'<td class="num">{fmt_int(entry["tokens"])}</td>'
            f'<td class="num">{fmt_money(entry["cost"])}</td>'
            f'<td class="wide">{bar(pct, ROLE_COLORS.get(role, "#64748b"))}</td></tr>'
        )
    parts.append("</tbody></table>")
    parts.append(
        '<p class="lede">The judge has no duration of its own here: it runs '
        "inside the post-run block, whose measured span covers the snapshot, "
        "the diff, teardown and the call together.</p>"
    )
    parts.append("</section>")

    # ---- by cli/model
    parts.append('<section><h2>By CLI and model</h2>')
    by_cli = {}
    for _, turn in all_turns:
        entry = by_cli.setdefault(
            (turn["cli"], turn["model"]),
            {"turns": 0, "time": 0.0, "cost": 0.0, "tokens": 0, "reported": 0},
        )
        entry["turns"] += 1
        entry["time"] += turn["duration_s"] or 0.0
        entry["cost"] += turn["cost_usd"] or 0.0
        entry["tokens"] += sum(turn["tokens"].values())
        if turn["cost_usd"] is not None or turn["tokens"]:
            entry["reported"] += 1
    parts.append('<table><thead><tr><th>CLI · model</th>'
                 '<th class="num">Turns</th><th class="num">Time</th>'
                 '<th class="num">Tokens</th><th class="num">Cost</th>'
                 "<th>Usage reported</th></tr></thead><tbody>")
    for (cli, model), entry in sorted(by_cli.items(), key=lambda kv: str(kv[0])):
        if entry["reported"] == 0:
            reported = '<span class="bad">none — CLI reports no usage</span>'
        elif entry["reported"] < entry["turns"]:
            reported = (
                f'<span class="warn">{entry["reported"]}/{entry["turns"]} turns</span>'
            )
        else:
            reported = f'{entry["reported"]}/{entry["turns"]} turns'
        parts.append(
            f"<tr><td><code>{esc(cli)}</code> · <code>{esc(model)}</code></td>"
            f'<td class="num">{entry["turns"]}</td>'
            f'<td class="num">{fmt_dur(entry["time"])}</td>'
            f'<td class="num">{fmt_int(entry["tokens"])}</td>'
            f'<td class="num">{fmt_money(entry["cost"])}</td>'
            f"<td>{reported}</td></tr>"
        )
    parts.append("</tbody></table></section>")

    # ---- turn trace
    parts.append('<section><h2>Turn-by-turn trace</h2>')
    for fixture in fixtures:
        parts.append(
            '<div class="trace"><div class="trace-head">'
            f'<span class="name">{esc(fixture["name"])}</span>'
            f'<span class="meta">{len(fixture["turns"])} turns · '
            f'{fmt_dur(fixture["model_s"])} model time · '
            f'{fmt_money(fixture["cost"])}</span></div>'
        )
        if not fixture["turns"]:
            parts.append('<div class="empty">no AI turns recorded</div>')
        for turn in fixture["turns"]:
            secs = turn["duration_s"] or 0.0
            pct = secs / max_turn * 100
            color = ROLE_COLORS.get(turn["role"], "#64748b")
            flag = ""
            if turn["status"] == "open":
                flag = '<span class="bad">no completion record</span>'
            elif turn["status"] == "failed":
                flag = '<span class="bad">failed</span>'
            floor = "≥ " if turn.get("duration_is_floor") else ""
            parts.append(
                '<div class="turn">'
                f'<span class="tn">#{esc(turn["turn"])}</span>'
                f'<span class="tr">{role_dot(turn["role"])}</span>'
                f'<span class="tc"><code>{esc(turn["cli"])}</code></span>'
                f'<span class="tb">{bar(pct, color, 10)}</span>'
                f'<span class="td">{floor}{fmt_dur(secs)}</span>'
                f'<span class="tk">{fmt_int(sum(turn["tokens"].values()))}</span>'
                f'<span class="tp">{fmt_money(turn["cost_usd"])}</span>'
                f'<span class="tf">{flag}</span>'
                "</div>"
            )
        parts.append("</div>")
    parts.append("</section>")

    # ---- caveats
    parts.append("""
<section><h2>What these numbers do and don't cover</h2>
<ul class="caveats">
  <li><strong>One block is a residual, not a measurement.</strong>
      <em>Fixture prep</em> is wall clock minus every measured slice, because
      <code>go test</code> reports a subtest's total but never stamps when it
      began. Everything else on the timeline is measured from a log record or
      an artifact mtime.</li>
  <li><strong>crush and codex report no usage.</strong> The <code>claude</code>
      CLI returns a usage block per turn; the others return bare text, so their
      turns show duration but no tokens or price. Any fixture whose apply turn
      runs on <code>coder</code> has real spend missing from the cost column.</li>
  <li><strong>The test-run slice is an upper bound.</strong> The engine logs
      nothing around its runner invocation, so the slice runs from the
      engine's first record to the next record it writes afterwards
      ("Loading full checklist"). Any engine work in between is counted as
      test time, which for this run overstates it by about 12ms. Deliberately
      log-derived rather than taken from the runner artifact's mtime: a
      preserved tmpdir can be re-run by hand later, and a measurement that
      changes when someone inspects it is worse than none.</li>
  <li><strong>The post-run block is mixed.</strong> Its span is measured
      (engine's last log record → the judge's usage record), but it contains
      the snapshot, the diff, fixture teardown and the judge call together;
      only the judge's own cost is separable.</li>
  <li><strong>Turn cost is attributed by sequence.</strong> The usage record
      carries no turn id, so it is paired with the turn that was open when it
      was written. The engine walks cells sequentially, so this is exact
      here.</li>
</ul>
</section>
""")

    parts.append("</main></body>")
    Path(out_path).write_text("".join(parts), encoding="utf-8")
    return out_path


HEAD = """<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>TrueBDD run report</title>
<style>
:root{
  --bg:#fff; --fg:#0f172a; --muted:#64748b; --line:#e2e8f0;
  --card:#fff; --track:#e2e8f0; --code:#f1f5f9;
}
@media (prefers-color-scheme:dark){
  :root{
    --bg:#0b1120; --fg:#e2e8f0; --muted:#94a3b8; --line:#1e293b;
    --card:#111827; --track:#1e293b; --code:#111827;
  }
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);
  font:15px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Inter,sans-serif;
  -webkit-font-smoothing:antialiased}
main,header{max-width:1180px;margin:0 auto;padding:0 32px}
header{padding-top:48px;padding-bottom:8px}
h1{font-size:28px;letter-spacing:-.02em;margin:0 0 6px}
h2{font-size:17px;margin:0 0 4px;padding-bottom:10px;
  border-bottom:1px solid var(--line)}
.sub{color:var(--muted);margin:0;font-size:14px}
section{margin:44px auto}
.lede{color:var(--fg);opacity:.85;margin:14px 0;max-width:78ch;font-size:14px}
code{font:13px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;
  background:var(--code);padding:1px 5px;border-radius:4px}
.tiles{display:grid;grid-template-columns:repeat(auto-fit,minmax(190px,1fr));
  gap:12px;margin:24px 0}
.tile{border:1px solid var(--line);border-radius:10px;padding:16px 18px;
  background:var(--card)}
.tile .k{font-size:11px;letter-spacing:.08em;text-transform:uppercase;
  color:var(--muted)}
.tile .v{font-size:26px;letter-spacing:-.02em;margin:6px 0 2px}
.tile .n{font-size:12px;color:var(--muted)}
table{width:100%;border-collapse:collapse;margin-top:12px;font-size:14px;
  display:block;overflow-x:auto}
thead th{font-size:11px;letter-spacing:.06em;text-transform:uppercase;
  color:var(--muted);font-weight:500;text-align:left;padding:10px 12px;
  border-bottom:1px solid var(--line);white-space:nowrap}
tbody td{padding:11px 12px;border-bottom:1px solid var(--line);
  vertical-align:middle}
table.phases{margin:0}
table.phases thead th{padding-left:18px}
table.phases tbody td:first-child{padding-left:18px}
.num{text-align:right;font-variant-numeric:tabular-nums;white-space:nowrap}
th.num{text-align:right}
.wide{width:32%;min-width:190px}
.name{font-weight:500}
.detail{margin-top:3px;font-size:12px;color:var(--muted);max-width:52ch}
.bar{background:var(--track);border-radius:99px;overflow:hidden;width:100%}
.bar span{display:block;height:100%;border-radius:99px}
.split{display:flex;height:9px;border-radius:99px;overflow:hidden;width:100%;
  background:var(--track)}
.split span{display:block;height:100%}
.gantt{background:var(--track);border-radius:99px;height:9px;width:100%}
.gantt span{display:block;height:100%;border-radius:99px}
.opaque{font-size:12px;color:var(--muted);font-style:italic}
.legend{display:flex;align-items:center;gap:8px;font-size:12px;
  color:var(--muted);margin:16px 0 4px;flex-wrap:wrap}
.sw{width:10px;height:10px;border-radius:3px;display:inline-block;
  margin-left:14px}
.legend .sw:first-child{margin-left:0}
.dot{width:8px;height:8px;border-radius:99px;display:inline-block;
  margin-right:7px}
.tag{font-size:11px;padding:3px 8px;border-radius:99px;white-space:nowrap;
  border:1px solid var(--line)}
.tag.det{color:#475569;background:#94a3b81f;border-color:#94a3b855}
.tag.mod{color:#b45309;background:#ea580c14;border-color:#ea580c44}
.tag.mix{color:#6d28d9;background:#7c3aed14;border-color:#7c3aed44}
@media (prefers-color-scheme:dark){
  .tag.det{color:#cbd5e1}
  .tag.mod{color:#fdba74}
  .tag.mix{color:#c4b5fd}
}
.approx{font-size:11px;color:var(--muted);font-weight:400}
.chip{font-size:11px;padding:3px 9px;border-radius:99px;
  border:1px solid var(--line);color:var(--muted)}
.chip.pass{color:#059669;border-color:#05966955;background:#05966912}
.chip.fail{color:#dc2626;border-color:#dc262655;background:#dc262612}
.bad{color:#dc2626}
.warn{color:#ea580c}
.trace{border:1px solid var(--line);border-radius:10px;margin-top:16px;
  background:var(--card);overflow:hidden}
.trace-head{display:flex;justify-content:space-between;align-items:center;
  padding:14px 18px;border-bottom:1px solid var(--line);gap:16px;
  flex-wrap:wrap}
.trace-head .meta{color:var(--muted);font-size:13px;
  font-variant-numeric:tabular-nums}
.turn{display:grid;
  grid-template-columns:44px 96px 72px 1fr 86px 82px 78px auto;
  gap:12px;align-items:center;padding:10px 18px;font-size:13px;
  border-bottom:1px solid var(--line)}
.turn:last-child{border-bottom:0}
.turn .tn{color:var(--muted);font-variant-numeric:tabular-nums}
.turn .td,.turn .tk,.turn .tp{text-align:right;
  font-variant-numeric:tabular-nums;white-space:nowrap}
.turn .tk,.turn .tp{color:var(--muted)}
.empty{padding:14px 18px;color:var(--muted);font-size:13px}
.findings{display:grid;gap:12px;margin-top:18px}
.finding{border:1px solid var(--line);border-left:3px solid #ea580c;
  border-radius:8px;padding:14px 18px;background:var(--card)}
.finding .ft{font-weight:600;font-size:14px;margin-bottom:5px}
.finding .fb{font-size:14px;opacity:.9;max-width:82ch}
.caveats{margin:16px 0 0;padding-left:20px;max-width:82ch}
.caveats li{margin-bottom:10px;font-size:14px}
@media (max-width:760px){
  main,header{padding:0 18px}
  .turn{grid-template-columns:36px 88px 1fr 74px;row-gap:6px}
  .turn .tc,.turn .tk,.turn .tp{display:none}
}
</style></head><body><main>
"""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--session")
    ap.add_argument("--gotest", default="tmp/bdd-run.log")
    ap.add_argument("--out", default="tmp/bdd-report.html")
    args = ap.parse_args()

    if args.session:
        session = at_root(args.session)
    else:
        runs_dir = REPO_ROOT / "tmp/test_run"
        if not runs_dir.is_dir():
            raise SystemExit(f"no run sessions: {runs_dir} does not exist")
        runs = sorted(
            (p for p in runs_dir.iterdir() if p.is_dir()), key=lambda p: p.name
        )
        if not runs:
            raise SystemExit(f"no sessions under {runs_dir}")
        session = runs[-1]

    verdicts, judges = parse_gotest(at_root(args.gotest))
    fixtures = load_fixtures(session, verdicts, judges)
    if not fixtures:
        raise SystemExit(f"no fixture engine logs under {session}")

    out = render(fixtures, Path(session).name, at_root(args.out))
    for fixture in fixtures:
        wall = fixture["wall_s"]
        if wall:
            drift = abs(wall - fixture["phase_total"])
            print(f"  {fixture['name']}: wall {wall:.2f}s, "
                  f"phases {fixture['phase_total']:.2f}s, drift {drift:.3f}s")
    print(f"wrote {out}  ({len(fixtures)} fixture(s), "
          f"{sum(len(f['turns']) for f in fixtures)} turns)")


if __name__ == "__main__":
    main()
