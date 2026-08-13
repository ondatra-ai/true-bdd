One-line: the only large-format colour in the system — a blurred radial field behind a hero or section band.

```jsx
<GradientField variant="radial" height={520}>
  <h1>Change is structural</h1>
</GradientField>
```

- Uses the real brand gradient artwork in `assets/gradients/`, not a CSS approximation.
- One gradient field per page. It is a touch-up accent, not a theme.
- Never a linear directional blend and never a hard-edged stop.
- Drop `intensity` to ~0.5 if body copy sits on top.
