# S&F Design System

A design system reverse-engineered from a single source document: **`uploads/S&F Guideline_Exp 11.pdf`**
(also saved as `uploads/SF-Guideline-Exp11.pdf` — the original filename contains characters the
tooling can't read). No codebase, no Figma file, and no website were supplied.

## Sources given

| Source | What it contained |
|---|---|
| `uploads/S&F Guideline_Exp 11.pdf` | One-page brand guideline: `01—Typography`, `02—Grid`, `03—Colors`, `04—Elements` (buttons, cards, partner-logo composition), `05—Aesthetic that we love` |
| `uploads/PoppinsBold.ttf`, `uploads/PoppinsMedium.ttf` | The two shipped webfont weights, copied to `assets/fonts/` |
| Two embedded raster gradients inside the PDF | Extracted losslessly to `assets/gradients/` — these are the real brand artwork, not approximations |

Reference sites the guideline names as "aesthetic that we love":
`awwwards.com`, `lowesinnovationlabs.com`, `fourmula.ai`.

## Company / product context

S&F is a **consultancy-style practice** rather than a software product. Everything in the guideline
points at editorial surfaces: presentations, marketing pages, one-pagers, reports, social posts.
The two real strings in the document — *"See How We Build for Change"* and *"start the conversation"* —
are both agency-website CTAs.

The intellectual centre of the brand is explicit and unusual: the colour system is derived from
**Spiral Dynamics** (Clare Graves / Don Beck), the developmental-stage model from adult psychology and
organisational theory. Blue → Orange → Green → Yellow → Teal are named *stages of organisational
maturity*, not decorative hues. That is the brand's argument, and it is why the palette is otherwise
strictly monochrome: the colour is the idea, so it is rationed.

**There is exactly one product surface**: a marketing website (`ui_kits/website/`). No app,
dashboard, or docs site is implied by the source.

### No logo exists in the sources
The PDF's own partner-lockup example sets the mark as the literal text `S&F`. No logo file, SVG, or
mark artwork was supplied. `components/brand/Wordmark.jsx` therefore renders the brand name in
Poppins Bold type, and nothing in this system draws a mark. **Do not invent one.**

---

## Content fundamentals

**Register.** Declarative and structural. Sentences state a mechanism, not a feeling. The brand
writes about systems, stages, structure, and order — abstract nouns used concretely.

**Person.** Mostly **we** (the practice) speaking to an implied **you** (the organisation). Never "I".
Never a first-person founder voice. CTAs use the imperative: *"Start the conversation."*

**Casing.** Sentence case in source for everything — headlines, body, button labels. Uppercase is a
*rendering* decision applied by CSS to buttons, tags, and section labels; never type in caps.
The guideline's own section headers use a numbered em-dash form: `01—Typography`, `04—Elements`.
Reuse that pattern (`SectionLabel`).

**Length.** Headlines are short and hard-clamped to two lines — the card component enforces this.
Paragraphs run ~44 characters wide and 2–4 sentences long. The rhythm is: one oversized assertion,
one small paragraph that qualifies it. Never three paragraphs in a column.

**Emoji: never.** Not in UI, not in copy, not in social. The system has no emoji affordance at all.

**Punctuation.** Em-dashes as structural joints (in section labels and in prose). The multiplication
sign `×` is reserved for the partner lockup — never the letter "x", never a plus.

**Vibe.** Swiss, editorial, slightly severe — a research practice that happens to design well.
The one warm note in the entire system is the blurred gradient; everything else is black type on
white paper.

Examples in the brand's voice (as written in `ui_kits/website/`):
- *"Organisations evolve in stages"* — headline
- *"Structure is the only lever that outlives the people who pull it"* — statement
- *"Read the stage before you change the structure"* — section headline
- *"We map where a system actually is, then build the structure it needs to reach the next stage."* — body
- *"Tell us where your system is stuck"* — form headline

Copy note: apart from the two CTA strings, the PDF contained only lorem ipsum. All copy in the UI kit
and templates is representative placeholder written in the extracted voice — replace it with real copy.

---

## Visual foundations

**Colour.** Monochromatic by default: `white-999 #FFFFFF`, `gray-100 #DEDFE4`, `gray-600 #828388`,
`gray-999 #232429`, `black-999 #040406`. Three semantic colours (`red-500`, `green-500`, `yellow-500`)
exist for status only. Six gradient stops carry the Spiral Dynamics meaning. Two background colours
per page, maximum — usually white and black.

**Gradient logic.** The guideline is unusually specific: gradients "behave more like blurred colour
fields than directional blends." Transitions are **radial and organic, with no visible edges or
linear flow**. They read light, desaturated, calm; depth comes from a slightly more saturated focal
area, not from contrast. Use as a *touch-up accent* and for *selective typographic colouring* only.
Prefer the real artwork (`--gradient-image-radial`, `--gradient-image-soft`) over the CSS fallbacks.
A left-to-right `linear-gradient` between two saturated stops is a violation.

**Type.** Poppins only, two weights. Medium 500 for body, Bold 700 for every heading. Four sizes:
20 / 32 / 52 / 84. Line height is a flat **120% at every size** — this is what makes the blocks feel
dense and Swiss. Headings carry `-0.02em` tracking; uppercase runs carry positive tracking
(`0.06em` buttons, `0.08em` labels). There is no Regular, Light, Semibold, or Italic file — do not
request weights that don't exist.

**Grid.** Swiss, 12 columns, **margin 0**, gutter 20. Margin zero is the defining move: content runs
to the page edge, and full-bleed bands are the norm rather than the exception. Common spans: 12,
7/5 editorial split, 4×3 module row, 6/6.

**Whitespace strategy.** Whitespace is *inside* modules (40–60px inner padding), not *between* them.
Cards and buttons butt directly against each other with zero gap and share a single hairline border.
Vertical rhythm between sections is large and consistent: 120px standard, 80px for tight utility bands.

**Backgrounds.** Flat white or flat black, plus one full-bleed blurred gradient band per page.
No photography direction is given in the source. No patterns, no textures, no hand-drawn
illustration, no grain. If imagery is added later it should read cool and desaturated to sit beside
the gradient — but that is an inference, not a rule from the source.

**Corners.** `0px`, everywhere, no exceptions. `--radius-none` exists so consumers can see it is zero.

**Borders.** 1px hairline, `black-999` on light grounds, `gray-999` on dark. The border is the only
chrome a component gets. Adjacent modules overlap borders by `-1px` so seams stay single-weight.

**Shadows.** **None.** There is no elevation system. Depth is value contrast (black band beside white
band) and the gradient field's focal area. No inner shadows, no protection gradients, no scrim
capsules — text sits on the light part of a gradient, or the gradient's intensity is lowered.

**Cards.** Square, hairline-bordered, unfilled, no shadow, 40px inner padding, headline clamped to
two lines, paragraph left-aligned beneath. Always in a connected row.

**Transparency & blur.** Only inside the gradient artwork itself. No frosted glass, no backdrop
filters, no translucent overlays. `--blur-field: 80px` documents the softness of the field, not a
UI effect.

**Motion.** Minimal and mechanical: 120ms, `cubic-bezier(.2,0,.2,1)`. Fades and colour swaps only.
No bounce, no spring, no scale-in, no parallax, no scroll-triggered reveals.

**Hover.** Buttons **invert** — fill and ink swap outright. Never lighten, darken, or tint. Links
shift from `black-999` to `gray-600`. Nav items move between 55% and 100% opacity.

**Press.** A 1px downward nudge (`translateY(1px)`). No scale, no colour change beyond the hover state.

**Disabled.** 35% opacity, `cursor: not-allowed`. No grey fill substitution.

**Focus.** The border darkens to `black-999` (light) or `white-999` (dark). No glow, no ring colour,
no offset outline.

**Fixed elements.** The nav is sticky at 80px with a hairline bottom border. Nothing else is fixed.

---

## Iconography

**The source defines no iconography at all.** There is no icon font, no SVG sprite, no icon set, no
icon usage example anywhere in the guideline. This is a genuine gap, not an omission on our part.

What the system does use in place of icons:
- **The `×` multiplication sign (U+00D7)** — the one sanctioned glyph, reserved exclusively for the
  partner lockup.
- **The em-dash `—`** in numbered section labels (`01—Typography`).
- **Solid colour squares** (20×20, no radius) as the stage-model markers in `StageList` — a swatch,
  not an icon.
- **Type and rules** for everything else. Nav, lists, and CTAs carry no glyphs.

**No emoji, ever.** No unicode symbols beyond `×` and `—`.

We deliberately did **not** substitute a CDN icon set (Lucide, Heroicons, etc.). Introducing one
would invent a visual language the brand has not chosen, and the geometric-Swiss direction makes the
choice consequential. **If icons are needed, this is the highest-priority decision to make** — see
the ask at the end.

---

## Components

Authored primitives, grouped by concern. The source defines **Button**, **Card**, and the
**partner-logo composition**; everything else is a supporting primitive or a flagged addition.

`components/core/`
- **Button** — large sharp rectangle, bold uppercase, 4 variants × 3 sizes, hover inverts
- **ButtonRow** — flush horizontal row, zero gap, shared borders
- **Card** — editorial module: two-line clamped headline + small paragraph
- **CardRow** — equal-width connected columns, no gutter
- **Tag** — small uppercase metadata marker (*intentional addition*)

`components/brand/`
- **Wordmark** — the brand name set in type; stands in for the absent logo
- **PartnerLockup** — `S&F × Partner` at equal weight
- **GradientField** — full-bleed blurred colour band using the real artwork
- **GradientText** — selective typographic gradient colouring

`components/layout/`
- **Grid** / **GridItem** — Swiss 12-column, margin 0, gutter 20
- **Section** — full-bleed tonal band with the 120px rhythm
- **SectionLabel** — the guideline's own `01—Typography` numbered label

`components/forms/`
- **Input** — square text field, border-only chrome (*intentional addition*)

### Intentional additions
- **Tag** — the guideline defines semantic red/green/yellow but no component that uses them; Tag is
  the smallest honest home for status colour.
- **Input** — the guideline's CTA is *"start the conversation"*, which requires contact capture;
  no form controls are specified, so Input follows the button's rules (square, border-only, no fill).
- **SectionLabel / Grid / Section** — codifications of `02—Grid` and the guideline's own layout
  behaviour rather than new inventions.

Not built, because the source does not define them: Select, Checkbox, Radio, Switch, Dialog, Toast,
Tooltip, Tabs, Avatar, Table.

---

## Index

| Path | What it is |
|---|---|
| `styles.css` | Global entry point — `@import` list only |
| `tokens/fonts.css` | `@font-face` for Poppins Medium & Bold |
| `tokens/colors.css` | Palette, gradient stops, semantic aliases |
| `tokens/typography.css` | Scale, weights, tracking, composite type roles |
| `tokens/spacing.css` | 20px-based scale + semantic padding/rhythm |
| `tokens/layout.css` | Grid tokens, `.sf-grid`, `.sf-modules` |
| `tokens/effects.css` | Radius (0), borders, gradients, transitions |
| `tokens/base.css` | Element resets, link colours, selection |
| `guidelines/*.html` | 23 foundation specimen cards (Type, Colors, Spacing, Brand) |
| `components/*/` | The primitives above, each with `.jsx` + `.d.ts` + `.prompt.md` + a card |
| `ui_kits/website/` | Marketing-site recreation — `README.md`, `index.html`, 8 screen files |
| `templates/presentation/` | 6-slide 16:9 deck template (`Presentation.dc.html`) |
| `templates/one-pager/` | Printable single-sheet brief (`OnePager.dc.html`) |
| `assets/fonts/` | `Poppins-Bold.ttf`, `Poppins-Medium.ttf` |
| `assets/gradients/` | `gradient-field-radial.png`, `gradient-field-soft.png` — extracted from the PDF |
| `thumbnail.html` | Homepage tile |
| `SKILL.md` | Agent-Skills front matter for use outside this project |
