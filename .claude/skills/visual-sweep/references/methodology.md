# visual-sweep exploration methodology (generic, normative)

How a sweep round explores a live web UI. Nothing here is project-specific:
every element, region, and route is discovered from the DOM at runtime. The
project-specific half (how to launch the app, where findings get pinned) lives
in the target adapter, not here.

## Discovery heuristics

- **Interactive elements** = semantic interactives
  (`a, button, [role=button|tab|menuitem|link|option], [tabindex], input,
  select, textarea, summary, label`) ∪ **pointer roots** (visible elements whose
  computed `cursor` is `pointer` and whose parent's isn't — cursor inherits, so
  descendants would explode the list) ∪ **hover-rule targets** (elements
  matching the base selector of any `:hover` rule in same-origin stylesheets —
  hover-styled list rows and cards react to the mouse even at `cursor: auto`).
  Filter to visible, on-screen, ≥8×8px; dedupe by signature + position; cap 200
  (the probe logs when capped).
- **Regions** (transit-sweep targets) = nav-like chrome detected purely by
  geometry: tall/narrow containers (≥50% viewport height, ≤25% width) hugging a
  vertical edge — rails, sidebars — or wide/short ones (≥50% width, ≤15%
  height) hugging a horizontal edge — toolbars, headers. Innermost of nested
  matches wins.
- **Routes** = same-origin links harvested from `a[href]` in the DOM, deduped
  by pathname, cap 8.
- **Overlays** = visible positioned roots (`fixed`/`absolute`, `z-index > 0`,
  ≥40×40px, no qualifying positioned ancestor) — panels, flyouts, tooltips.

## Gestures (what a round exercises)

1. **Hover enumeration** — every discovered interactive element: rest → hover →
   away, measuring own box + parent box + `fontWeight`/`letterSpacing` at each
   state (hover-probe).
2. **Transit sweeps** — the cursor crosses each region at 25ms steps (3
   vertical or horizontal passes + one diagonal entry from content), the way a
   user mouses PAST it, not to it (transit-sweep).
3. **Cold-cache first load** — fresh browser context, watch the first ~3s for
   font-swap and late-content reflow (walk-shifts; run it twice for a second
   cold sample).
4. **Route cycle** — every harvested route ×2 with dwell, then rapid no-dwell
   flips (walk-shifts).
5. **Random cursor walks** — 2×25 random points (walk-shifts).
6. **Judgment passes** (orchestrator, Playwright MCP) — anything the probes
   flagged plus click-where-safe, keyboard focus rings, animations, and visual
   composition. Never click elements whose text/attrs suggest destructive,
   submitting, or paying actions.

## Oracles and thresholds

| Oracle | Threshold | Symptom |
|---|---|---|
| Buffered `PerformanceObserver('layout-shift')` with per-element attribution | any non-input shift | `shift` |
| 250ms bounding-box sampler on chrome landmarks | Δ ≥ 0.5px between consecutive samples (same document) | `jiggle` |
| Rest/hover/after box + font measurement | Δ ≥ 0.5px or any font-prop change | `jiggle` / `font-pop` |
| 25ms overlay sampler during pure transit | any appearance; dwell < 150ms | `flash` |
| Scoped MutationObserver | node churn during hover | `churn` |
| Console/pageerror capture | any error | `console` |

A probe signal is a **candidate**, not automatically a finding: the
orchestrator confirms it visually (screenshot/video), judges user impact, and
triages. Expected input-driven movement (a panel the user opened, a collapse
they clicked) is not a finding.

## Finding fingerprints (the dedupe key)

`<element-signature>/<symptom>` where the signature is the element's
`data-testid` in brackets when present (`[rail-flyout]`), else
`tag#id`/`tag.class:nth(n)`, and symptom ∈
`shift | jiggle | flash | flicker | font-pop | squeeze | sliver | churn | console`.

## Probe interface

All three probes live beside this file in `scripts/` and share one env
interface (see `scripts/lib/discover.mjs`):

```bash
VS_URL=<entry url> VS_OUT=<artifact dir> node scripts/hover-probe.mjs
# optional: VS_SCOPE=<css selector>  VS_VIEWPORT=1440x900
#           VS_REQUIRE_FROM=<package.json that resolves playwright>
```

Each prints a one-line `SUMMARY …` (plus one line per JIGGLE/region/top-shift)
and writes full JSON + `.webm` video to `VS_OUT` — read the JSON only when a
summary line warrants it. The committed scripts are templates: copy one into
the run dir and extend it when a bespoke probe is needed.
