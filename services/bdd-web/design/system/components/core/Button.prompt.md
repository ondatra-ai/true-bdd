One-line: the brand's only button — a large sharp-cornered rectangle with bold uppercase type; use it for every call to action.

```jsx
<Button variant="primary" size="lg">See How We Build for Change</Button>
```

- Variants: `primary` (black fill), `secondary` (white fill, black rule), `ghost` (no chrome), `inverse` (for use on black or gradient bands).
- Sizes: `sm` 48px, `md` 68px (default), `lg` 88px tall.
- Hover **inverts** fill and ink — never lighten, darken, or round.
- Label copy is sentence-cased in source and uppercased by CSS; keep it short (2–6 words).
- Two or more buttons always go inside `<ButtonRow>` so they share edges with zero gap.
