'use strict';

// BDD run report UI.
//
// Five views behind hash routes, so every page deep-links and survives a
// reload. State polls /api/state; the current view re-renders only when
// the server's version actually moves, which is what lets a long BDD run
// stream into an open page without throwing away scroll position or
// closing expanders.

const POLL_MS = 5000;

const view = document.getElementById('view');
const crumb = document.getElementById('crumb');
const statusEl = document.getElementById('status');

let version = -1;
let compareSelection = [];

// ---------------------------------------------------------------- utils

const h = (tag, attrs = {}, ...kids) => {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === false || v === null || v === undefined) continue;
    if (k === 'class') el.className = v;
    else if (k === 'html') el.innerHTML = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v);
  }
  for (const kid of kids.flat()) {
    if (kid === null || kid === undefined || kid === false) continue;
    el.append(kid.nodeType ? kid : document.createTextNode(String(kid)));
  }
  return el;
};

const api = async (path) => {
  const res = await fetch(path);
  if (!res.ok) {
    let detail = res.statusText;
    try { detail = (await res.json()).error || detail; } catch { /* not json */ }
    throw new Error(`${res.status} ${detail}`);
  }
  return res.json();
};

const apiText = async (path) => {
  const res = await fetch(path);
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.text();
};

// secs renders a duration the way the old report did: milliseconds under
// a second (an engine step is meaningless as "0.0s"), then seconds, then
// minutes once the number stops being scannable.
const secs = (value, known = true) => {
  if (!known || value === null || value === undefined) return '—';
  if (value === 0) return '0ms';
  const abs = Math.abs(value);
  if (abs < 1) return `${Math.round(value * 1000)}ms`;
  if (abs < 60) return `${value.toFixed(1)}s`;
  const mins = Math.trunc(value / 60);
  return `${mins}m ${Math.abs(value - mins * 60).toFixed(1)}s`;
};

// money and count render absent as an em dash: every zero in this report
// means "nothing was reported", not "this was free".
const money = (value) => (!value ? '—' : `$${value.toFixed(4)}`);
const count = (value) => (!value ? '—' : value.toLocaleString());

const verdictChip = (verdict) => {
  const cls = { PASS: 'pass', FAIL: 'fail', SKIP: 'skip' }[verdict] || 'none';
  return h('span', { class: `chip ${cls}` }, verdict || '—');
};

// How the run reached the models, per caller. It belongs beside every
// verdict: the same green verdict means "the models did this again" in
// live and "the engine still does this to a recording" in replay.
// Unknown is its own state — a session predating the record is not live.
const MODE_TITLES = {
  live: 'live — real AI CLIs, every turn billed',
  record: 'record — real AI CLIs, cassettes written for a passing run',
  replay: 'replay — turns served from cassettes, no AI CLI spawned',
};

// A mode is per CALLER now: `target:replay,tests:live` runs the engine
// from cassettes while a real judge grades it. Single-word values are
// sessions recorded before the split and mean both callers at once.
const CALLERS = ['target', 'tests'];

const parseMode = (mode) => {
  const raw = (mode || '').trim();
  if (!raw) return {};
  if (!raw.includes(':')) return { target: raw, tests: raw };

  const out = {};
  raw.split(',').forEach((part) => {
    const [caller, value] = part.split(':').map((s) => s.trim());
    if (CALLERS.includes(caller)) out[caller] = value;
  });

  return out;
};

// True only when NOTHING reached a model, which is what lets a page say
// the numbers on it were remembered rather than spent.
const isReplayed = (mode, caller) => {
  const parsed = parseMode(mode);
  if (caller) return parsed[caller] === 'replay';

  return CALLERS.every((name) => parsed[name] === 'replay');
};

// The matrix header has room for a word, not a spec: one letter per
// caller, in target-then-tests order.
const shortMode = (mode) => {
  const parsed = parseMode(mode);
  const letters = CALLERS.map((caller) => (parsed[caller] || '?')[0]);

  return letters.every((l) => l === letters[0]) ? (parsed.target || '?') : letters.join('/');
};

// One chip per caller, each coloured by its own mode: a run that replays
// the engine and bills a live judge is two different claims, and a single
// chip could only tell one of them.
const modeChip = (mode) => {
  const parsed = parseMode(mode);
  if (!parsed.target && !parsed.tests) {
    return h('span', {
      class: 'chip mode unknown',
      title: 'mode unknown — this session predates the record',
    }, 'unknown');
  }

  return h('span', { class: 'modepair' }, ...CALLERS.map((caller) => h('span', {
    class: `chip mode ${parsed[caller] || 'unknown'}`,
    title: `${caller}: ${MODE_TITLES[parsed[caller]] || 'mode unknown'}`,
  }, `${caller}:${parsed[caller] || '?'}`)));
};

// On a run or test page the mode is not a detail among others: it
// decides what the whole page MEANS. The same green verdict is a claim
// about the models in live and a claim about the engine in replay, and
// the numbers below it are money spent in one and money remembered in
// the other. So it gets a banner, not a tile someone has to go looking
// for.
const MODE_BANNERS = {
  replay: 'Every AI turn was served from a cassette — no model ran. '
    + 'Cost and token counts below belong to the recording, not to this run. '
    + 'Graded by comparing the recorded outcome, not by the judge.',
  record: 'Real AI CLIs ran and were billed. Cassettes and the recorded outcome '
    + 'are published to the fixture only if it passed.',
  live: 'Real AI CLIs ran and were billed, and a claude judge graded the result '
    + 'against the fixture rubric.',
};

// The banner takes its colour from the LOUDEST claim on the page: any
// real model call makes the numbers money, and record is louder still
// because it also rewrites what the fixtures replay from.
const bannerMode = (mode) => {
  const parsed = parseMode(mode);
  const values = CALLERS.map((caller) => parsed[caller]).filter(Boolean);
  if (!values.length) return 'unknown';
  if (values.includes('record')) return 'record';
  if (values.includes('live')) return 'live';

  return 'replay';
};

const modeBanner = (mode) => {
  const parsed = parseMode(mode);
  const loudest = bannerMode(mode);
  const label = CALLERS.every((c) => parsed[c] === loudest)
    ? loudest.toUpperCase()
    : CALLERS.map((c) => `${c}:${parsed[c] || '?'}`).join(' ');

  return h('div', { class: `banner mode-${loudest}` },
    h('strong', {}, label),
    ' — ',
    MODE_BANNERS[loudest]
      || 'This session was recorded before the harness wrote down which mode it ran in, '
        + 'so how it reached the models is unknown.');
};

// Money a replay run appears to have spent was spent by the RECORDING:
// the engine reads cost and tokens out of each CLI response, and in
// replay those responses come from cassettes. Rendering it as an
// ordinary charge would bill a free run for work someone already paid
// for, so it is marked as recorded and never adds up to a claim about
// this run.
const cost = (value, mode, caller) => (!isReplayed(mode, caller)
  ? money(value)
  : h('span', {
    class: 'recorded',
    title: 'recorded cost — replayed from cassettes, this run spent nothing',
  }, money(value)));

// The run's score as three numbers: passed, failed, and how many
// fixtures the invocation set out to run. The last is the point — a
// denominator counting only what has finished can never say how much of
// the run is left.
const outcomeTriple = (run) => h('span', {
  class: 'triple',
  title: `${run.passed} passed / ${run.failed} failed / ${run.planned} planned`,
}, `${run.passed} / ${run.failed} / ${run.planned}`);

const delta = (value, render = secs) => {
  if (!value) return h('span', { class: 'delta zero' }, '—');
  const cls = value > 0 ? 'up' : 'down';
  return h('span', { class: `delta ${cls}` }, `${value > 0 ? '+' : '−'}${render(Math.abs(value))}`);
};

const tile = (k, v) => h('div', { class: 'tile' }, h('div', { class: 'k' }, k), h('div', { class: 'v' }, v));

const wrap = (...kids) => h('div', { class: 'wrap' }, ...kids);

// copyBtn writes text to the clipboard and confirms in place. `build` is
// a thunk — possibly async — so a payload that has to fetch file bodies
// is only assembled when someone actually asks for it.
const copyBtn = (label, build, title = '') => h('button', {
  class: 'copy',
  title,
  onclick: async (ev) => {
    // Inside a <summary>, a click would otherwise open the expander.
    ev.preventDefault();
    ev.stopPropagation();

    const btn = ev.currentTarget;
    const original = btn.textContent;
    try {
      await navigator.clipboard.writeText(await build());
      btn.textContent = 'copied';
    } catch {
      btn.textContent = 'copy failed';
    }
    setTimeout(() => { btn.textContent = original; }, 1400);
  },
}, label);

const table = (headers, rows) =>
  h('table', {},
    h('thead', {}, h('tr', {}, headers.map((col) =>
      h('th', { class: col.num ? 'num' : null }, col.label ?? col)))),
    h('tbody', {}, rows));

const fail = (err) => h('p', { class: 'err' }, String(err.message || err));

const shortID = (id) => id.replace('_', ' ').replace(/-/g, ':').replace(/^(\d+):(\d+):(\d+) /, '$1-$2-$3 ');

// ------------------------------------------------------------- polling

async function poll() {
  try {
    const state = await api('/api/state');
    statusEl.className = 'status live';
    statusEl.textContent = `${state.runs.length} run(s) · scanned ${new Date(state.scanned_at).toLocaleTimeString()}`;
    if (state.version !== version) {
      version = state.version;
      await render();
    }
  } catch (err) {
    statusEl.className = 'status';
    statusEl.textContent = `offline — ${err.message}`;
  }
}

// --------------------------------------------------------------- views

async function renderRunList() {
  crumb.textContent = '';
  const { runs } = await api('/api/runs');
  if (!runs.length) return h('p', { class: 'empty' }, 'No runs yet. Run the BDD suite.');

  const rows = runs.map((run) => h('tr', {},
    h('td', {}, h('label', { class: 'pick' }, h('input', {
      type: 'checkbox',
      'data-run': run.id,
      checked: compareSelection.includes(run.id),
      onchange: (ev) => toggleCompare(run.id, ev.target.checked),
    }))),
    h('td', {}, h('a', { href: `#/run/${run.id}` }, shortID(run.id)),
      run.complete ? null : h('span', { class: 'tag warn', style: 'margin-left:6px' }, 'in progress')),
    h('td', {}, modeChip(run.mode)),
    h('td', {}, verdictChip(run.failed ? 'FAIL' : 'PASS'), ' ', outcomeTriple(run),
      run.skipped ? h('span', { class: 'tag', style: 'margin-left:6px' }, `${run.skipped} skipped`) : null),
    h('td', { class: 'num' }, secs(run.wall_seconds)),
    h('td', { class: 'num' }, run.turns),
    h('td', { class: 'num' }, cost(run.cost_usd + run.judge_cost_usd, run.mode)),
  ));

  const compareBtn = h('button', {
    class: 'primary',
    disabled: compareSelection.length !== 2,
    onclick: () => { location.hash = `#/cmp/${compareSelection[0]}/${compareSelection[1]}`; },
  }, compareSelection.length === 2 ? 'Compare selected' : 'Select two runs to compare');

  return h('div', {},
    h('div', { style: 'display:flex;align-items:center;gap:10px;margin-bottom:12px' }, compareBtn),
    wrap(table(
      ['', 'Run', 'Mode', { label: 'Outcome — passed / failed / planned' },
        { label: 'Wall', num: true }, { label: 'Turns', num: true }, { label: 'Cost', num: true }],
      rows)),
    await renderMatrix());
}

// ---------------------------------------------------------------- matrix

// Every test against every run on one grid. Green or red carries the
// outcome and the reason is spelled out in words beneath the duration —
// the two together are what make a failure legible, since a CLI error at
// 3s is a startup problem and the same error at 300s is not.
//
// How many runs the carousel shows at once. Full timestamps are wide, so
// a window keeps the grid readable however much history accumulates.
const MATRIX_WINDOW = 5;

// matrixOffset is the index of the leftmost visible run. Kept at module
// scope so paging does not lose its place when the poller re-renders.
let matrixOffset = null;

async function renderMatrix() {
  const m = await api('/api/matrix');
  if (!m.tests.length) return h('div', {});

  const total = m.runs.length;
  const window = Math.min(MATRIX_WINDOW, total);
  const maxOffset = Math.max(0, total - window);

  // Default to the newest runs: the right-hand edge is what you came to
  // look at. Clamped on every render so a run appearing or disappearing
  // cannot leave the window out of range.
  if (matrixOffset === null) matrixOffset = maxOffset;
  matrixOffset = Math.max(0, Math.min(matrixOffset, maxOffset));

  const from = matrixOffset;
  const to = from + window;
  const visible = m.runs.slice(from, to);

  const page = (delta) => {
    matrixOffset = Math.max(0, Math.min(matrixOffset + delta, maxOffset));
    render();
  };

  const controls = h('div', { class: 'carousel' },
    h('button', { disabled: from === 0, title: 'older runs', onclick: () => page(-1) }, '‹'),
    h('span', { class: 'crumb' },
      total <= window
        ? `${total} run${total === 1 ? '' : 's'}`
        : `runs ${from + 1}–${to} of ${total} · oldest ← → newest`),
    h('button', { disabled: to >= total, title: 'newer runs', onclick: () => page(1) }, '›'),
    h('span', { class: 'spacer' }));

  const head = h('tr', {},
    h('th', { class: 'name' }, 'Test'),
    visible.map((r) => h('th', {
      class: r.partial ? 'col-partial' : null,
      title: `${r.label} — ${r.fixtures} fixture(s), ${r.mode || 'mode unknown'}`
        + `${r.partial ? ', partial run — only some fixtures ran' : ''}`
        + `${r.complete ? '' : ', still running'}`,
    }, r.label, h('span', { class: 'colmode' }, shortMode(r.mode)))),
    h('th', { class: 'num' }, 'passed'));

  const body = m.tests.map((row) => matrixRow(row, from, to));

  return h('div', {},
    controls,
    h('div', { class: 'matrix-wrap' },
      h('table', { class: 'matrix' }, h('thead', {}, head), h('tbody', {}, body))));
}

function matrixRow(row, from, to) {
  const cells = row.cells.slice(from, to).map((c) => {
    const absent = c.outcome === 'absent';
    const cls = absent ? 'gone' : (c.failed ? 'fail' : 'pass');
    return h('td', {
      class: `cell ${cls}`,
      title: absent
        ? `${row.name} did not run in ${c.run}`
        : `${row.name} @ ${c.run}\n${c.label}${c.detail ? '\n\n' + c.detail : ''}`,
      onclick: absent ? null : () => {
        location.hash = `#/run/${c.run}/test/${encodeURIComponent(row.name)}`;
      },
    }, absent
      ? '·'
      : [h('span', { class: 'dur' }, secs(c.wall_seconds, c.has_wall)),
         c.reason ? h('span', { class: 'why' }, c.reason) : null]);
  });

  // The tally counts every run the test was observed in, not just the
  // visible window — paging the carousel must not appear to change a
  // test's history.
  return h('tr', {},
    h('td', { class: 'name', title: row.name }, row.name),
    cells,
    h('td', { class: 'num', title: reasonSummary(row) },
      `${row.passed}/${row.observed}`));
}

const reasonSummary = (row) => {
  const parts = Object.entries(row.reasons).map(([k, n]) => `${n}× ${k}`);
  return parts.length
    ? `${row.passed} passed, ${row.failed} failed — ${parts.join(', ')}`
    : `${row.observed} run(s), no failure observed`;
};

function toggleCompare(id, on) {
  compareSelection = on
    ? [...compareSelection, id].slice(-2)
    : compareSelection.filter((x) => x !== id);
  render();
}

async function renderRun(runID) {
  const data = await api(`/api/runs/${encodeURIComponent(runID)}`);
  crumb.textContent = shortID(runID);

  const nav = h('div', { style: 'display:flex;gap:8px;align-items:center;margin-bottom:12px' },
    h('button', {
      disabled: !data.prev,
      onclick: () => { location.hash = `#/run/${data.prev}`; },
    }, '← older'),
    h('button', {
      disabled: !data.next,
      onclick: () => { location.hash = `#/run/${data.next}`; },
    }, 'newer →'),
    h('span', { class: 'crumb' }, '[ and ] navigate'),
  );

  const t = data.run;
  const tiles = h('div', { class: 'tiles' },
    tile('Passed / failed / planned', outcomeTriple(t)),
    tile('Wall clock', secs(t.wall_seconds)),
    tile('AI turns', t.turns),
    tile('Engine cost', cost(t.cost_usd, t.mode, 'target')),
    tile('Judge cost', cost(t.judge_cost_usd, t.mode, 'tests')),
  );

  const rows = data.tests.map((test) => h('tr', {},
    h('td', {}, h('a', { href: `#/run/${runID}/test/${encodeURIComponent(test.name)}` }, test.name)),
    h('td', {}, verdictChip(test.verdict)),
    h('td', { class: 'num' }, secs(test.wall_seconds, test.has_wall)),
    h('td', { class: 'num' }, test.turns || '—'),
    h('td', { class: 'num' }, cost(test.cost_usd, t.mode, 'target')),
    h('td', { class: 'num' }, count(test.tokens)),
    h('td', { class: 'num' }, test.exit_code === null ? '—' : test.exit_code),
    h('td', { class: 'num' }, test.diff_count || '—'),
    // Drift is the timeline's self-check: phases that do not sum to the
    // measured wall clock mean the phase model has a hole.
    h('td', { class: 'num' }, test.has_wall ? secs(test.drift_seconds) : '—'),
  ));

  return h('div', {}, nav, modeBanner(t.mode), tiles,
    h('h2', {}, 'Fixtures'),
    wrap(table(
      ['Fixture', 'Verdict', { label: 'Wall', num: true }, { label: 'Turns', num: true },
        { label: 'Cost', num: true }, { label: 'Tokens', num: true },
        { label: 'Exit', num: true }, { label: 'Files', num: true }, { label: 'Drift', num: true }],
      rows)));
}

async function renderTest(runID, name) {
  const d = await api(`/api/runs/${encodeURIComponent(runID)}/tests/${encodeURIComponent(name)}`);
  crumb.innerHTML = '';
  crumb.append(h('a', { href: `#/run/${runID}` }, shortID(runID)), ' / ', name);

  return h('div', {},
    h('div', { class: 'pathbar' },
      h('code', {}, d.run_dir || ''),
      copyBtn('copy path', () => d.run_dir,
        'the fixture\u2019s working directory, relative to the repo root'),
      copyBtn('copy prompt', () => testPrompt(d),
        'a ready-to-paste description of this run: what it was, what it did, and where its files are')),
    modeBanner(d.summary.mode),
    h('div', { class: 'tiles' },
      tile('Verdict', verdictChip(d.summary.verdict)),
      tile('Wall clock', secs(d.summary.wall_seconds, d.summary.has_wall)),
      tile('Turns', d.summary.turns),
      tile('Model time', secs(d.summary.model_seconds)),
      tile('Cost', cost(d.summary.cost_usd, d.summary.mode, 'target')),
      tile('Files changed', d.files.length)),
    expectedVsActual(d),
    turnList(runID, name, d),
    fileList(d),
    evidence(runID, name, d));
}

// testPrompt assembles everything someone would otherwise have to type
// before asking about this run: which fixture, in which session, what it
// was told to do, how it ended, and the repo-relative paths of the files
// that explain it. Paths are repo-relative on purpose — absolute ones are
// machine-specific and tmpdir-relative ones locate nothing.
function testPrompt(d) {
  const lines = [];
  const s = d.summary;

  lines.push(`BDD fixture \`${s.name}\` from run \`${d.run}\` of the true-bdd suite.`);
  lines.push('');
  lines.push(`Outcome: ${s.verdict}` +
    (s.exit_code === null ? ' (never exited)' : ` (exit ${s.exit_code})`) +
    `, ${secs(s.wall_seconds, s.has_wall)} wall, ${s.turns} AI turn(s), ${money(s.cost_usd)}.`);
  // Without this a reader cannot tell a run that exercised the models
  // from one that replayed a recording of them.
  lines.push(`AI mode: ${s.mode || 'unknown (session predates the record)'}`);
  lines.push(`Command: true-bdd ${d.expected.command || s.command}`);

  if (d.expected.exit_code !== undefined) {
    lines.push(`Expected exit code: ${d.expected.exit_code}`);
  }

  if (s.failures?.length) {
    lines.push('', 'Failures:');
    for (const f of s.failures) lines.push(`- ${f}`);
  }

  lines.push('', `Run directory (relative to repo root): ${d.run_dir}`);

  const evidence = (d.artifacts || []).filter((a) => a.bytes);
  if (evidence.length) {
    lines.push('', 'Evidence:');
    for (const a of evidence) {
      lines.push(`- ${a.kind}: ${a.repo_path || `<${a.ref}>`}`);
    }
  }

  // Split the diff: everything under the run's own tmp/ is per-prompt
  // scratch the engine writes on every run, and listing forty of them
  // buries the handful of files the run was actually judged on.
  const files = d.files || [];
  const scratch = files.filter((f) => f.path.startsWith('tmp/'));
  const output = files.filter((f) => !f.path.startsWith('tmp/'));

  if (output.length) {
    lines.push('', `Files the run produced (${output.length}):`);
    for (const f of output) lines.push(`- ${f.kind} ${f.repo_path || f.path}`);
  } else if (files.length) {
    lines.push('', 'The run produced no files outside its own scratch directory.');
  }

  if (scratch.length) {
    lines.push('', `Plus ${scratch.length} scratch artifact(s) under ${d.run_dir}/tmp/ ` +
      '(per-prompt system/user/response/result files).');
  }

  if (d.expected.judge_spec) {
    lines.push('', 'The judge graded it against this rubric:', '', d.expected.judge_spec.trim());
  }

  return lines.join('\n');
}

// expectedVsActual pairs what the fixture declared against what the run
// produced, per assertion, so a reader sees the two side by side rather
// than inferring the expectation from the failure text.
// A failure line from the deep check that replaces the judge in replay:
// the golden comparison, or the cassette census.
const gradedFailure = (line) => line.startsWith('golden:') || line.startsWith('cassettes:');

function expectedVsActual(d) {
  const e = d.expected;
  const a = d.actual;
  const replayed = d.summary.mode === 'replay';

  const badge = {
    snapshot: h('span', { class: 'tag' }, 'this run’s manifest'),
    repo: h('span', { class: 'tag warn' }, 'source tree as of now — may have drifted'),
    absent: h('span', { class: 'tag warn' }, 'no manifest found'),
  }[e.source];

  const exitOK = a.exit_code !== null && a.exit_code === e.exit_code;
  const rows = [
    row('exit code', e.exit_code, a.exit_code === null ? 'never exited' : a.exit_code, exitOK),
    ...(e.stdout_checks.length
      ? e.stdout_checks.map((rx) => row('stdout matches', rx,
        a.failures.some((f) => f.startsWith('stdout:') && f.includes(rx)) ? 'not found' : 'matched',
        !a.failures.some((f) => f.startsWith('stdout:') && f.includes(rx))))
      : [row('stdout checks', 'none declared', '—', true)]),
    // What actually graded the deep check. In replay the judge was
    // never asked, so claiming its rubric passed would credit a check
    // that did not run.
    ...(replayed
      ? [row('recorded outcome',
        'byte-identical to the recording, every cassette consumed',
        a.failures.some(gradedFailure) ? 'differs' : 'matched',
        !a.failures.some(gradedFailure))]
      : [row('judge rubric', e.judge_spec ? `${e.judge_spec.split('\n').length} line rubric` : 'none',
        judgeVerdictText(a), !a.failures.some((f) => f.startsWith('judge:')))]),
  ];

  function row(label, expected, actual, ok) {
    return h('tr', { class: ok ? null : 'changed' },
      h('td', {}, label),
      h('td', { class: 'mono' }, String(expected)),
      h('td', { class: 'mono' }, String(actual)),
      h('td', {}, ok ? '✓' : '✗'));
  }

  const rubric = e.judge_spec
    ? h('details', {}, h('summary', {},
      replayed
        ? 'Judge rubric (declared by the fixture — NOT used in this run; replay grades against the recording)'
        : 'Judge rubric (what the run was graded against)'),
      h('div', { class: 'body' }, h('pre', { class: 'text' }, e.judge_spec)))
    : null;

  return h('div', {},
    h('h2', {}, 'Expected vs actual ', badge),
    wrap(table(['Assertion', 'Expected', 'Actual', ''], rows)),
    a.failures.length
      ? h('div', { style: 'margin-top:8px' },
        wrap(table(['Failure'], a.failures.map((f) => h('tr', { class: 'op-delete' }, h('td', { class: 'mono' }, f))))))
      : null,
    rubric ? h('div', { style: 'margin-top:8px' }, rubric) : null);
}

const judgeVerdictText = (actual) => {
  const judge = actual.failures.find((f) => f.startsWith('judge:'));
  return judge ? judge.replace(/^judge:\s*/, 'FAIL: ') : 'PASS';
};

// turnList is the run's timeline AND its turn list — they were two
// sections showing the same spans, one as bars and one as detail, and
// the reader had to hold a turn number in their head to cross them.
//
// Rows are every phase in chronological order — EVERY phase, however
// short. A phase backed by a model call expands into that turn's detail;
// a deterministic slice stays one thin line, because "parse the result
// file, write artifacts, build the next prompt" is the same sentence
// every time and only its DURATION ever says anything.
//
// The sub-second slices were once folded into a single trailing expander
// on the grounds that a 2ms row is unactionable. That was wrong twice
// over: it broke the chronology (the fold landed at the end, so the rows
// no longer read top-to-bottom as the run happened), and a slice being
// small is a FINDING — "the engine spent 2ms here" is the evidence that
// the wall clock belongs to the models, and it can only be read if the
// row is there. The timeline accounts for the whole run or it accounts
// for nothing.
function turnList(runID, name, d) {
  if (!d.phases.length && !d.turns.length) return null;

  // Denominator for the bars: the measured wall clock when there is one,
  // else the accounted total, so a run without a harness record still
  // shows proportions rather than dividing by zero.
  const hasWall = d.summary.has_wall && d.summary.wall_seconds > 0;
  const total = hasWall ? d.summary.wall_seconds : (d.summary.phase_total_seconds || 1);
  // What the shares are a share OF. Naming the fallback matters: on a
  // run with no harness record the denominator is the phases' own sum,
  // and calling that "wall" would claim a measurement that never existed.
  const denominator = hasWall ? 'wall' : 'accounted phase time';

  // Attempts within a cell come from the server: the same numbers decide
  // whether a turn is named "Validate" or "Re-validate", and counting
  // them twice in two languages is how those two disagree.
  const rows = [];

  for (const p of d.phases) {
    const bar = ganttBar(p, total);
    const isTurn = p.turn_index !== null && p.turn_index !== undefined;
    const t = isTurn ? d.turns[p.turn_index] : null;

    if (!t) {
      rows.push(phaseRow(runID, name, d, p, bar, total, denominator));
      continue;
    }

    rows.push(turnRow(runID, name, d, t, bar, t.attempt, t.attempt_total, p.offset_seconds));
  }

  const turnCount = d.turns.length;

  return h('div', {},
    h('h2', {}, `Timeline — ${turnCount} turn${turnCount === 1 ? '' : 's'}`,
      d.summary.has_wall
        ? h('span', { class: 'crumb', style: 'text-transform:none;margin-left:8px' },
            `over ${secs(d.summary.wall_seconds)}`)
        : null),
    ...rows);
}

// ganttBar places a slice on the run's wall clock: it starts where the
// slice started and is as wide as it lasted. The offset is what makes it
// a timeline rather than a bar chart — it answers "what came after what".
function ganttBar(p, total) {
  const width = Math.max(0.4, Math.min(100, (p.seconds / total) * 100));
  const offset = Math.max(0, Math.min(100, (p.offset_seconds / total) * 100));
  const cls = p.kind === 'deterministic' ? 'role-det' : `role-${p.role || 'prompt'}`;

  return h('div', { class: 'gantt' },
    h('span', { class: cls, style: `margin-left:${offset.toFixed(2)}%;width:${width.toFixed(2)}%` }));
}

// phaseRow is one non-model slice — and it opens, exactly like a turn.
//
// A deterministic slice is not "the boring one". It is where the fixture
// declares itself: which files were overlaid, which checklist was
// walked and how wide, what the runner subprocess printed, what the
// judge was told to assert. All of that is in the API and none of it is
// anywhere else on this page, so a collapsed row that only showed a
// label and a duration threw the answers away and kept the arithmetic.
//
// The owner tag stays on the summary line — "engine" / "tests" /
// "harness" is the one thing a reader cannot infer from the label, and
// it is what says whether a slow slice is true-bdd's to fix.
function phaseRow(runID, name, d, p, bar, total, denominator) {
  const share = total > 0 ? (p.seconds / total) * 100 : 0;

  // Label plus offset: a phase has no id of its own, and its duration
  // grows while the run does.
  return h('details', { class: 'det', 'data-key': `phase:${p.label}:${p.offset_seconds}` },
    h('summary', {},
      h('span', { class: 'turn-head' }, p.label,
        p.owner ? h('span', { class: 'tag', style: 'margin-left:6px' }, p.owner) : null,
        p.measured ? null : h('span', { class: 'tag', style: 'margin-left:6px' }, 'residual')),
      h('span', { class: 'phase-at', title: 'time since the run began' }, secs(p.offset_seconds)),
      h('span', { class: 'phase-time' }, secs(p.seconds)),
      bar,
      h('span', { class: 'row-meta' },
        // "0.0%" of a 2ms slice reads as "this took no time", which is
        // the opposite of the finding. Below the resolution of one
        // decimal the share is stated as a bound instead.
        h('span', { class: 'turn-cost' },
          share > 0 && share < 0.05
            ? `<0.1% of ${denominator}`
            : `${share.toFixed(1)}% of ${denominator}`))),
    h('div', { class: 'body' },
      // The slice's own prose. It used to live only in a tooltip, which
      // is the same as not being there for anyone reading down the page.
      p.detail ? h('p', { class: 'crumb' }, p.detail) : null,
      phaseBody(runID, name, d, p)));
}

// phaseBody dispatches on the slice's label to whatever the report can
// actually say about that kind of work. The default is honest rather
// than empty: a between-turns gap really has nothing in it but Go.
function phaseBody(runID, name, d, p) {
  const label = p.label || '';

  if (label === 'Fixture prep') return prepBody(d);
  if (label.startsWith('Test run')) return testRunBody(runID, name, d, p);

  // "Engine start-up" is the merged slice a run gets when there was no
  // test discovery to split it on — for `us apply` it IS the checklist
  // load, so it gets the checklist body rather than the generic note.
  if (label === 'Checklist load + prompt render' || label === 'Engine start-up') {
    return checklistBody(d);
  }

  if (label === 'Post-run + judge') return judgeBody(d);

  return h('p', { class: 'crumb' },
    'Pure Go: no subprocess, no model. The artifacts this step produced ' +
    'are listed on the turns either side of it.');
}

// facts is the two-column table every phase body uses.
const facts = (rows) => wrap(table(['', ''], rows));

// fkey labels a block inside a phase body.
const fkey = (text) => h('div', { class: 'fkey' }, text);

// commandList renders shell commands as what they are — one line each,
// monospaced, in the order they run.
function commandList(title, cmds) {
  return h('div', {}, fkey(title),
    h('pre', { class: 'text' }, cmds.join('\n')));
}

// collapsible is a nested expander for a body too long to sit open.
const collapsible = (title, text) => h('details', {},
  h('summary', {}, h('span', { class: 'turn-head' }, title)),
  h('div', { class: 'body' }, h('pre', { class: 'text' }, text)));

// prepBody is the scaffolding the harness laid down before the CLI ran:
// what was copied in, what was overlaid on top, and which commands ran
// between the two. This slice is the report's one residual, so saying
// what is inside it is the only check a reader has on the number.
function prepBody(d) {
  const e = d.expected;

  const source = {
    snapshot: 'this run’s own manifest.json',
    repo: 'the source tree as of now — may have drifted',
    absent: 'no manifest found',
  }[e.source] || e.source;

  return h('div', {},
    facts([
      kv('repo layer (pre-copied)', 'true-bdd/, templates/'),
      kv('input overlay', e.input_path || '—'),
      kv('command', `true-bdd ${e.command || '—'}`),
      kv('stdin piped', e.answers ? `${e.answers.split('\n').filter(Boolean).length} line(s)` : 'none'),
      kv('manifest read from', source),
    ]),
    e.answers ? collapsible('answers piped to stdin', e.answers) : null,
    e.prep?.length
      ? commandList('prep commands', e.prep)
      : h('p', { class: 'crumb' },
          'No prep: commands — this slice is the repo-layer copy, the input ' +
          'overlay and the pre-run snapshot alone.'));
}

// checklistBody is what the engine loaded and how wide the walk is.
// items × prompts is the shape of the whole run: it says how many model
// calls the walk can cost before a single fix is applied.
function checklistBody(d) {
  const m = d.meta || {};

  const walk = `${m.items || 0} item(s) × ${m.prompts || 0} prompt(s)` +
    (m.max_apply_attempts ? ` · max ${m.max_apply_attempts} apply attempt(s)` : '');

  const rows = [
    kv('command', m.checklist_command || '—'),
    kv('checklist file', m.checklist_path || '—'),
    kv('architecture', m.architecture_file || '—'),
    kv('walk', walk),
  ];

  if (m.decisions) rows.push(kv('decisions', count(m.decisions)));
  if (m.services) rows.push(kv('services', count(m.services)));

  // Cells the run actually reached, in order. A walk that died early
  // shows fewer than the checklist declares, and that gap is the point.
  const cells = [];
  const seen = new Set();
  for (const t of d.turns) {
    const label = t.cell.label;
    if (!label || seen.has(label)) continue;
    seen.add(label);
    cells.push(label);
  }

  return h('div', {}, facts(rows),
    cells.length
      ? h('div', {}, fkey(`cells walked (${cells.length})`),
          h('pre', { class: 'text' }, cells.join('\n')))
      : null);
}

// testRunBody is the framework runner the engine spawned. Both streams
// render rather than get summarised: when a suite never starts, the
// reason is in that output and nowhere else.
//
// Which subprocesses belong to this slice comes from the phase, never
// from its label: a fix loop reruns the same test repeatedly, so
// several slices carry the identical label "Test run (playwright ·
// rerun)" and matching on it would put every rerun's output under every
// rerun's row. A session recorded before the index existed has no
// association at all — that falls back to listing them, which is the
// old behaviour and still better than showing nothing.
function testRunBody(runID, name, d, p) {
  const parts = [];
  const m = d.meta || {};

  const all = d.test_runs || [];
  const mine = p.test_runs
    ? p.test_runs.map((i) => all[i]).filter(Boolean)
    : all;

  if (m.test_run_command) parts.push(facts([kv('test runner', m.test_run_command)]));

  if (!mine.length) {
    parts.push(facts([
      kv('framework', d.discovery.framework || '—'),
      kv('outcome', d.discovery.outcome || '—'),
    ]));
    parts.push(h('p', { class: 'crumb' },
      'No runner output was captured for this span — only the time between ' +
      'engine log records is known.'));

    return h('div', {}, ...parts);
  }

  for (const run of mine) {
    parts.push(fkey(run.label));
    parts.push(facts([
      kv('outcome', run.outcome || '—'),
      kv('duration', secs(run.duration_seconds)),
      kv('exit', run.has_exit ? String(run.exit_code) : '—'),
    ]));
    parts.push(streamBlock(runID, name, 'stdout', run.stdout, run.framework));
    parts.push(streamBlock(runID, name, 'stderr', run.stderr, run.framework));
  }

  return h('div', {}, ...parts);
}

// streamBlock renders one captured stream. An empty-but-captured stream
// says so in its own words — that is a finding about the framework, not
// a gap in the report.
function streamBlock(runID, name, label, ref, framework) {
  if (!ref || !ref.ref) return null;

  if (ref.missing) {
    return h('p', { class: 'crumb' },
      `${label} was captured to ${ref.repo_path || ref.name}, which is no longer on disk.`);
  }

  if (!ref.bytes) {
    return h('p', { class: 'crumb' }, `${framework || 'the runner'} wrote nothing to ${label}.`);
  }

  return h('details', {},
    h('summary', {}, h('span', { class: 'turn-head' }, `${label} · ${count(ref.bytes)} bytes`)),
    h('div', { class: 'body' }, lazyText(runID, name, ref.ref)));
}

// judgeBody is the harness's verdict machinery: who graded it, what it
// cost, what it was asked to assert, and what teardown ran afterwards.
// The rubric and the file diff have their own sections on this page, so
// this block points at them rather than printing them twice.
function judgeBody(d) {
  const j = d.judge || {};
  const e = d.expected;
  const a = d.actual;

  const rows = [
    kv('judge model', j.ran ? `${j.cli} · ${j.model}` : 'did not run'),
    kv('judge cost', money(j.cost_usd)),
    kv('judge tokens', count(j.tokens)),
    kv('verdict', a.verdict || '—'),
    kv('expected exit', String(e.exit_code)),
    kv('stdout checks', e.stdout_checks.length ? e.stdout_checks.join(' · ') : 'none declared'),
  ];

  return h('div', {}, facts(rows),
    a.failures.length
      ? h('div', {}, fkey(`failed checks (${a.failures.length})`),
          h('pre', { class: 'text' }, a.failures.join('\n\n')))
      : h('p', { class: 'crumb' }, 'Every declared assertion held.'),
    e.teardown?.length ? commandList('teardown commands', e.teardown) : null,
    h('p', { class: 'crumb' },
      `This span also covers the post-run snapshot and diff — ${d.files.length} ` +
      'changed file(s), listed below — and the deep check shown under ' +
      '“Expected vs actual”.'));
}

// turnRow is a model call: the same shape as an overhead row, plus an
// expander and the copy button.
function turnRow(runID, name, d, t, bar, nth, total, offset) {
  // Keyed on the turn number, which is the engine's own stable id for
  // this call. The summary text is not usable as a key: it carries the
  // duration, cost and token count, all of which move on every rescan
  // while the turn is still running — the exact case the expander state
  // exists to survive.
  return h('details', { 'data-key': `turn:${t.number}` },
    h('summary', {},
      h('span', { class: 'turn-head' },
        `#${t.number} · ${t.operation.label || t.cell.label || t.role}`,
        ` · ${t.model}`),
      h('span', { class: 'phase-at', title: 'time since the run began' }, secs(offset)),
      h('span', { class: 'phase-time' }, secs(t.duration_seconds)),
      bar,
      h('span', { class: 'row-meta' },
        h('span', { class: 'turn-cost' }, `${money(t.cost_usd)} · ${count(t.tokens.total)} tok`),
        t.status !== 'ok' ? h('span', { class: 'tag warn' }, t.status) : null,
        copyBtn('copy', () => turnPrompt(runID, d, t),
          `the whole turn: facts, command, file paths AND the system and user prompt text (each artifact capped at ${count(MAX_EMBED)} chars)`))),
    h('div', { class: 'body' },
      wrap(table(['', ''], [
        kv('cli / model', `${t.cli} / ${t.model}`),
        t.operation.why ? kv('why it ran', t.operation.why) : null,
        total > 1 ? kv('attempt', `${nth} of ${total} on this check`) : null,
        kv('duration', secs(t.duration_seconds) + (t.duration_is_floor ? ' (lower bound)' : '')),
        kv('cost', money(t.cost_usd)),
        kv('tokens', `${count(t.tokens.total)} — in ${count(t.tokens.input)}, out ${count(t.tokens.output)}, cache r ${count(t.tokens.cache_read)} / w ${count(t.tokens.cache_creation)}`),
        kv('prompt size', `system ${count(t.system_length)} · user ${count(t.user_length)}`),
        ...provenance(t).map(([label, value]) => kv(label, value)),
        kv('command', t.command || '—'),
        kv('tools', t.tool_calls.length ? toolSummary(t.tool_calls) : '—'),
      ])),
      // The two sides of the check, openable in place: the file the
      // subject came from and the checklist that posed the question. A
      // path alone makes the reader go and look; this is the looking.
      d.provenance?.length ? h('div', {}, h('div', { class: 'fkey' }, 'checked against'),
        artifactList(runID, name, d.provenance)) : null,
      artifactList(runID, name, [...t.inputs, ...t.outputs], { d, turn: t })));
}

// provenance names the files behind a turn: where its subject came from,
// which checklist posed the question, what the fix mutates, and every
// document the prompt resolved. The row header says WHAT was compared
// and WHY; this says against exactly which files. Rows with nothing to
// say are dropped rather than printed as "—", so a command that declares
// no docs: keys does not grow an empty line.
function provenance(t) {
  const o = t.operation || {};
  const rows = [];
  // The files themselves are listed under "checked against", openable;
  // these are the per-turn facts about them that a shared file list
  // cannot carry — which section of the checklist this turn's question
  // belongs to, and which documents this turn in particular resolved.
  if (o.section_name) rows.push(['section', `${o.section} — “${o.section_name}”`]);
  else if (o.section) rows.push(['section', o.section]);
  if (o.docs?.length) rows.push(['docs read', o.docs.join(' · ')]);
  return rows;
}

// turnFacts is the turn's fact table as text — the same rows the page
// shows, in the same order. Every turn-scoped copy embeds this, so what
// lands in the clipboard is what is on screen.
function turnFacts(t) {
  return [
    `cli / model   ${t.cli} / ${t.model}`,
    ...(t.operation.why ? [`why it ran    ${t.operation.why}`] : []),
    ...(t.attempt_total > 1 ? [`attempt       ${t.attempt} of ${t.attempt_total} on this check`] : []),
    `duration      ${secs(t.duration_seconds)}` +
      (t.duration_is_floor ? ' (lower bound)' : '') + ` · status ${t.status}` +
      (t.error ? ` · ${t.error}` : ''),
    `cost          ${money(t.cost_usd)}`,
    `tokens        ${count(t.tokens.total)} — in ${count(t.tokens.input)}, ` +
      `out ${count(t.tokens.output)}, cache r ${count(t.tokens.cache_read)} / ` +
      `w ${count(t.tokens.cache_creation)}`,
    `prompt size   system ${count(t.system_length)} · user ${count(t.user_length)}`,
    ...provenance(t).map(([label, value]) => `${label.padEnd(13)} ${value}`),
    `command       ${t.command || '—'}`,
    `tools         ${t.tool_calls.length ? toolSummary(t.tool_calls) : '—'}`,
  ].join('\n');
}

// fetchArtifact pulls one artifact body, capped so a runaway prompt
// cannot produce an unpasteable clipboard.
const MAX_EMBED = 24000;

async function fetchArtifact(runID, name, ref) {
  try {
    const body = await apiText(
      `/api/runs/${encodeURIComponent(runID)}/tests/${encodeURIComponent(name)}/text?ref=${encodeURIComponent(ref)}`);
    return body.length > MAX_EMBED
      ? body.slice(0, MAX_EMBED) + `\n…(truncated, ${body.length - MAX_EMBED} more chars)…`
      : body;
  } catch (err) {
    return `(could not read: ${err.message})`;
  }
}

// turnPrompt is testPrompt narrowed to one model call: everything needed
// to ask about THIS turn without describing it first. The artifact paths
// matter most — the system and user prompts are what the turn actually
// saw, and they are files on disk rather than anything on this page.
async function turnPrompt(runID, d, t, only = null) {
  const lines = [];

  lines.push(`AI turn #${t.number} of BDD fixture \`${d.summary.name}\`, ` +
    `run \`${d.run}\` of the true-bdd suite.`);
  lines.push(`Operation: ${t.operation.label || t.cell.label || '(none)'}` +
    (t.operation.why ? ` — ${t.operation.why}` : '') + ` · role: ${t.role}`);
  lines.push('', turnFacts(t));

  lines.push('', `Run directory: ${d.run_dir}`);
  lines.push(`Fixture overall: ${d.summary.verdict}` +
    (d.summary.failures?.length ? ` — ${d.summary.failures[0]}` : '') + '.');

  // The bodies. This is the part that makes the paste self-contained:
  // the fact table says how the turn went, the prompts say what it was
  // actually asked, and without them the reader can only guess.
  //
  // A missing artifact is named rather than dropped. Silently omitting
  // it makes the paste claim the turn had no such file, when what
  // actually happened is that the tmpdir was cleaned — and someone
  // reading the paste would go looking for a prompt that was never
  // reported gone.
  const scoped = [...t.inputs, ...t.outputs].filter((r) => !only || r.ref === only.ref);
  const files = scoped.filter((r) => !r.missing);
  const gone = scoped.filter((r) => r.missing);

  for (const r of files) {
    const body = await fetchArtifact(runID, d.summary.name, r.ref);
    // Body verbatim: this claims to be the file's full text, so trailing
    // whitespace and the final newline are part of it.
    lines.push('', `───── ${r.kind} · ${r.repo_path || r.name} ─────`, '', body);
  }

  for (const r of gone) {
    lines.push('', `───── ${r.kind} · ${r.repo_path || r.name} ─────`, '',
      '(recorded by the engine but no longer on disk)');
  }

  return lines.join('\n');
}

const kv = (k, v) => h('tr', {}, h('td', { style: 'width:160px;color:var(--muted)' }, k), h('td', { class: 'mono' }, v));

const toolSummary = (calls) => {
  const counts = {};
  for (const c of calls) counts[c.name] = (counts[c.name] || 0) + 1;
  return Object.entries(counts).map(([n, c]) => `${n}×${c}`).join(', ');
};

function artifactList(runID, name, refs, context = null) {
  const usable = refs.filter((r) => !r.missing);
  if (!usable.length) return null;
  return h('div', { style: 'margin-top:8px' }, usable.map((r) => h('div', { class: 'artifact' },
    h('details', { 'data-key': `art:${r.ref}` },
      h('summary', {}, `${r.kind} · ${r.name} · ${count(r.bytes)} bytes`),
      h('div', { class: 'body' },
        r.repo_path
          ? h('div', { class: 'pathbar' }, h('code', {}, r.repo_path),
              copyBtn('copy path', () => r.repo_path))
          : null,
        lazyText(runID, name, r.ref))),
    // Outside the expander, so neither copying nor locating a file
    // requires rendering its body on the page first.
    context
      ? copyBtn('copy', () => turnPrompt(runID, context.d, context.turn, r),
          `this file\u2019s text (capped at ${count(MAX_EMBED)} chars), with the turn it belongs to as context`)
      : (r.repo_path ? copyBtn('copy path', () => r.repo_path, r.repo_path) : null))));
}

// lazyText fetches a body only when its expander is opened — prompts run
// to tens of kilobytes, and a test page has dozens of them.
function lazyText(runID, name, ref) {
  const box = h('pre', { class: 'text' }, 'loading…');
  const url = `/api/runs/${encodeURIComponent(runID)}/tests/${encodeURIComponent(name)}/text?ref=${encodeURIComponent(ref)}`;
  let loaded = false;
  const load = async () => {
    if (loaded) return;
    loaded = true;
    try { box.textContent = await apiText(url); } catch (err) { box.textContent = String(err); }
  };
  queueMicrotask(() => {
    const details = box.closest('details');
    if (details) details.addEventListener('toggle', () => { if (details.open) load(); }, { once: true });
    else load();
  });
  return box;
}

function fileList(d) {
  if (!d.files.length) return null;
  const rows = d.files.map((f) => h('tr', {},
    h('td', {}, h('span', { class: 'tag' }, f.kind)),
    h('td', { class: 'mono' },
      f.repo_path || f.path,
      copyBtn('copy', () => f.repo_path || f.path, 'copy this path')),
    h('td', { class: 'num' }, count(f.bytes_before)),
    h('td', { class: 'num' }, count(f.bytes_after))));
  return h('div', {}, h('h2', {}, `Files changed (${d.files.length})`),
    wrap(table(['Kind', 'Path', { label: 'Before', num: true }, { label: 'After', num: true }], rows)));
}

function evidence(runID, name, d) {
  const parts = [];

  if (!d.judge.has_transcript && d.judge.ran) {
    parts.push(h('p', { class: 'crumb' },
      'Judge transcript not captured for this run — sessions recorded before the harness persisted it.'));
  }

  if (d.artifacts.length) parts.push(artifactList(runID, name, d.artifacts));
  if (!parts.length) return null;

  return h('div', {}, h('h2', {}, 'Evidence'), ...parts);
}

// ------------------------------------------------------------- compare

async function renderCompareRuns(left, right) {
  const c = await api(`/api/compare/${encodeURIComponent(left)}/${encodeURIComponent(right)}`);
  crumb.textContent = `${shortID(left)} ↔ ${shortID(right)}`;

  const rows = c.rows.map((row) => {
    const cls = row.op === 'match' ? (row.changed ? 'changed' : null) : `op-${row.op}`;
    const link = row.op === 'match'
      ? h('a', { href: `#/cmp/${left}/${right}/test/${encodeURIComponent(row.name)}` }, row.name)
      : row.name;
    return h('tr', { class: cls },
      h('td', { class: 'mark' }, { match: '', delete: '−', insert: '+' }[row.op]),
      h('td', {}, link, row.fields.length ? h('div', { class: 'crumb' }, row.fields.join(', ')) : null),
      h('td', {}, row.left ? verdictChip(row.left.verdict) : '—'),
      h('td', {}, row.right ? verdictChip(row.right.verdict) : '—'),
      h('td', { class: 'num' }, row.left ? secs(row.left.wall_seconds, row.left.has_wall) : '—'),
      h('td', { class: 'num' }, row.right ? secs(row.right.wall_seconds, row.right.has_wall) : '—'),
      h('td', { class: 'num' }, delta(row.deltas.wall_seconds)),
      h('td', { class: 'num' }, row.left ? row.left.turns : '—'),
      h('td', { class: 'num' }, row.right ? row.right.turns : '—'),
      h('td', { class: 'num' }, delta(row.deltas.cost_usd, (v) => `$${v.toFixed(4)}`)));
  });

  return h('div', {},
    h('div', { class: 'tiles' },
      tile('Matched', c.summary.matched),
      tile('Changed', c.summary.changed),
      tile('Only left', c.summary.only_left),
      tile('Only right', c.summary.only_right)),
    h('h2', {}, `${shortID(left)}  ↔  ${shortID(right)}`),
    wrap(table(['', 'Test', 'A', 'B', { label: 'Wall A', num: true }, { label: 'Wall B', num: true },
      { label: 'Δ wall', num: true }, { label: 'Turns A', num: true }, { label: 'Turns B', num: true },
      { label: 'Δ cost', num: true }], rows)));
}

async function renderCompareTest(left, right, name) {
  const c = await api(`/api/compare/${encodeURIComponent(left)}/${encodeURIComponent(right)}/tests/${encodeURIComponent(name)}`);
  crumb.innerHTML = '';
  crumb.append(h('a', { href: `#/cmp/${left}/${right}` }, `${shortID(left)} ↔ ${shortID(right)}`), ' / ', name);

  return h('div', {},
    h('h2', {}, 'Outcome'),
    wrap(table(['Field', shortID(left), shortID(right), ''], c.actual.scalars.map(scalarRow))),
    listSection('Failures', c.actual.failures),
    expectedSection(c),
    judgeSection(left, right, name, c),
    turnSection(left, right, name, c),
    fileSection(c));
}

const scalarRow = (s) => h('tr', { class: s.changed ? 'changed' : null },
  h('td', {}, s.field),
  h('td', { class: 'mono' }, s.left || '—'),
  h('td', { class: 'mono' }, s.right || '—'),
  h('td', {}, s.changed ? '≠' : '='));

function listSection(title, edits) {
  const changed = edits.filter((e) => e.op !== 'match');
  if (!edits.length) return null;
  return h('div', {}, h('h2', {}, `${title} ${changed.length ? `(${changed.length} changed)` : '(identical)'}`),
    wrap(table(['', 'Value'], edits.map((e) => h('tr', { class: e.op === 'match' ? null : `op-${e.op}` },
      h('td', { class: 'mark' }, { match: '', delete: '−', insert: '+' }[e.op]),
      h('td', { class: 'mono' }, e.value))))));
}

function expectedSection(c) {
  const e = c.expected;
  const note = e.comparable
    ? null
    : h('p', { class: 'crumb' },
      `Only a per-run manifest snapshot makes this a real comparison. Left: ${e.source_left}, right: ${e.source_right}. ` +
      'A "repo" side is today\'s fixture.yaml, not what that run was held to.');

  return h('div', {},
    h('h2', {}, 'Expected'),
    note,
    wrap(table(['Field', 'A', 'B', ''], e.scalars.map(scalarRow))),
    listSection('Expected · stdout checks', e.stdout_checks),
    listSection('Expected · judge rubric', e.judge_spec),
    listSection('Expected · prep', e.prep),
    listSection('Expected · teardown', e.teardown));
}

// judgeSection diffs the two judge prompts — what the grader was actually
// told, which is the only way to explain a PASS that became a FAIL when
// nothing in the command changed.
function judgeSection(left, right, name, c) {
  const j = c.judge;
  const body = [wrap(table(['Field', 'A', 'B', ''], j.scalars.map(scalarRow)))];

  if (j.comparable) {
    for (const [label, ref] of [['user prompt', 'judge:user'], ['system prompt', 'judge:system'], ['response', 'judge:response']]) {
      body.push(h('details', {}, h('summary', {}, `Diff judge ${label}`),
        h('div', { class: 'body' }, lazyTextDiff(left, right, name, ref))));
    }
  } else {
    body.push(h('p', { class: 'crumb' },
      `Judge transcript missing on ${!j.left_has_transcript && !j.right_has_transcript ? 'both runs' : (!j.left_has_transcript ? 'the left run' : 'the right run')} — ` +
      'only runs recorded after the harness started persisting it can be diffed.'));
  }

  return h('div', {}, h('h2', {}, 'Judge'), ...body);
}

function turnSection(left, right, name, c) {
  if (!c.turns.length) return null;

  const changed = c.turns.filter((t) => t.op !== 'match').length;
  const rows = c.turns.map((t) => {
    const cls = t.op === 'match' ? (t.changed ? 'changed' : null) : `op-${t.op}`;
    return h('tr', { class: cls },
      h('td', { class: 'mark' }, { match: '', delete: '−', insert: '+' }[t.op]),
      h('td', {}, t.label || t.cell_key.replace(/\x1f/g, ' · ')),
      h('td', {}, (t.left || t.right).role),
      h('td', { class: 'num' }, t.left ? secs(t.left.duration_seconds) : '—'),
      h('td', { class: 'num' }, t.right ? secs(t.right.duration_seconds) : '—'),
      h('td', { class: 'num' }, delta(t.deltas.duration_seconds)),
      h('td', { class: 'num' }, t.left ? money(t.left.cost_usd) : '—'),
      h('td', { class: 'num' }, t.right ? money(t.right.cost_usd) : '—'),
      h('td', {}, t.op === 'match'
        ? h('details', {}, h('summary', {}, 'diff'),
          h('div', { class: 'body' },
            lazyTextDiff(left, right, name, `turn:${t.left.index}:in:1`, `turn:${t.right.index}:in:1`)))
        : ''));
  });

  return h('div', {},
    h('h2', {}, `Turns — ${c.turns.length} aligned, ${changed} added or removed`),
    h('p', { class: 'crumb' },
      'Aligned on checklist cell and role, not on turn number: a run that needed one extra retry ' +
      'shows a single insertion instead of shifting every later row.'),
    wrap(table(['', 'Cell', 'Role', { label: 'A', num: true }, { label: 'B', num: true },
      { label: 'Δ', num: true }, { label: 'Cost A', num: true }, { label: 'Cost B', num: true }, ''], rows)));
}

function fileSection(c) {
  if (!c.files.length) return null;
  const changed = c.files.filter((f) => f.op !== 'match' || f.changed).length;
  const rows = c.files.map((f) => h('tr', { class: f.op === 'match' ? (f.changed ? 'changed' : null) : `op-${f.op}` },
    h('td', { class: 'mark' }, { match: '', delete: '−', insert: '+' }[f.op]),
    h('td', { class: 'mono' }, f.path),
    h('td', { class: 'num' }, f.left ? count(f.left.bytes_after) : '—'),
    h('td', { class: 'num' }, f.right ? count(f.right.bytes_after) : '—')));
  return h('div', {}, h('h2', {}, `Files changed — ${changed} differ`),
    wrap(table(['', 'Path', { label: 'A bytes', num: true }, { label: 'B bytes', num: true }], rows)));
}

// lazyTextDiff fetches a line diff only when opened.
function lazyTextDiff(left, right, name, ref, rightRef = null) {
  const box = h('div', { class: 'linediff' }, 'loading…');
  const url = `/api/diff/text?lrun=${encodeURIComponent(left)}&ltest=${encodeURIComponent(name)}&lref=${encodeURIComponent(ref)}`
    + `&rrun=${encodeURIComponent(right)}&rtest=${encodeURIComponent(name)}&rref=${encodeURIComponent(rightRef || ref)}`;
  let loaded = false;
  const load = async () => {
    if (loaded) return;
    loaded = true;
    try {
      box.replaceChildren(...renderTextDiff(await api(url)));
    } catch (err) {
      box.replaceChildren(fail(err));
    }
  };
  queueMicrotask(() => {
    const details = box.closest('details');
    if (details) details.addEventListener('toggle', () => { if (details.open) load(); }, { once: true });
    else load();
  });
  return box;
}

function renderTextDiff(d) {
  if (d.identical) return [h('p', { class: 'empty' }, 'Identical.')];
  if (!d.hunks.length) return [h('p', { class: 'empty' }, 'No overlapping content to diff.')];

  const out = d.hunks.map((hunk) => h('div', { class: 'hunk' },
    h('div', { class: 'meta' }, `@@ −${hunk.left_start} +${hunk.right_start} @@`),
    hunk.lines.map((ln) => h('div', { class: `ln ${ln.op}` },
      h('span', { class: 'sig' }, { match: ' ', insert: '+', delete: '−' }[ln.op]),
      h('span', {}, ln.text)))));

  if (d.truncated) {
    out.push(h('p', { class: 'crumb', style: 'padding:6px 8px' },
      `Diff truncated — the remaining lines are not shown. Left ${d.left_lines} lines, right ${d.right_lines}.`));
  }
  return out;
}

// --------------------------------------------------------------- router

// A re-render replaces the whole view, so anything the READER did to the
// page — scrolling, opening expanders — is in the new tree's way. While a
// suite is running the version moves every rescan, which turns that into
// the page yanking itself out from under someone every few seconds.
//
// Expander state is keyed on summary text rather than position: a run in
// flight grows new turns, and an index would reopen whatever slid into
// the old slot instead of the thing that was open.
const openDetails = new Set();

// The renderer assigns data-key to everything whose summary text moves.
// Summary text remains the fallback for expanders that are stable by
// construction (a rubric, a fact block) and need no id of their own.
const detailKey = (el) => {
  if (el.dataset.key) return el.dataset.key;
  const summary = el.querySelector(':scope > summary');
  return summary ? summary.textContent.trim() : '';
};

// `toggle` does not bubble, so this listens in the capture phase.
document.addEventListener('toggle', (ev) => {
  if (ev.target.tagName !== 'DETAILS') return;
  const key = detailKey(ev.target);
  if (!key) return;
  if (ev.target.open) openDetails.add(key);
  else openDetails.delete(key);
}, true);

const restoreOpen = (root) => {
  for (const el of root.querySelectorAll('details')) {
    if (openDetails.has(detailKey(el))) el.open = true;
  }
};

let lastRoute = null;

async function render() {
  const parts = location.hash.replace(/^#\/?/, '').split('/').filter(Boolean).map(decodeURIComponent);
  // Scroll is only worth restoring when the page is the same page.
  // Navigating somewhere new should land at the top, as it always has.
  const sameRoute = location.hash === lastRoute;
  const scroll = window.scrollY;
  // Keys are only unique within a page — two runs hold the same turn
  // numbers — so leaving a route drops what was open on it.
  if (!sameRoute) openDetails.clear();
  try {
    let node;
    if (!parts.length) node = await renderRunList();
    else if (parts[0] === 'run' && parts.length === 2) node = await renderRun(parts[1]);
    else if (parts[0] === 'run' && parts[2] === 'test') node = await renderTest(parts[1], parts[3]);
    else if (parts[0] === 'cmp' && parts.length === 3) node = await renderCompareRuns(parts[1], parts[2]);
    else if (parts[0] === 'cmp' && parts[3] === 'test') node = await renderCompareTest(parts[1], parts[2], parts[4]);
    else node = h('p', { class: 'empty' }, 'Not found.');
    if (sameRoute) restoreOpen(node);
    view.replaceChildren(node);
    lastRoute = location.hash;
    if (sameRoute) window.scrollTo(0, scroll);
  } catch (err) {
    view.replaceChildren(fail(err));
  }
}

// [ and ] step between runs from anywhere a run is in scope.
document.addEventListener('keydown', async (ev) => {
  if (ev.metaKey || ev.ctrlKey || ev.altKey) return;
  if (ev.key !== '[' && ev.key !== ']') return;
  const parts = location.hash.replace(/^#\/?/, '').split('/').filter(Boolean);
  if (parts[0] !== 'run') return;
  const data = await api(`/api/runs/${encodeURIComponent(decodeURIComponent(parts[1]))}`);
  const target = ev.key === '[' ? data.prev : data.next;
  if (target) location.hash = `#/run/${target}`;
});

window.addEventListener('hashchange', render);
render();
poll();
setInterval(poll, POLL_MS);
