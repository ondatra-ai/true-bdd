One-line: the brand's content module — a big two-line headline over a small paragraph, generous inner padding; use it in a CardRow, never alone in a floating grid.

```jsx
<CardRow>
  <Card headline="Lorem ipsum is placeholder text" body="Lorem ipsum is placeholder text commonly used in the graphic, print, and publishing industries." />
  <Card headline="Lorem ipsum is placeholder text" body="…" />
  <Card headline="Lorem ipsum is placeholder text" body="…" />
</CardRow>
```

- Headline is hard-clamped to 2 lines — write copy that fits; do not raise the clamp.
- No radius, no shadow. The border and the neighbour's border are the separation.
- `tone="dark"` for cards inside black bands; `padding="var(--pad-card-lg)"` for hero-scale rows.
