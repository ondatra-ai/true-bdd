One-line: lays Cards out as connected equal-width columns with zero gap — the guideline's card behaviour.

```jsx
<CardRow columns={3}>{cards}</CardRow>
```

- Two to four columns reads best; beyond four the two-line headline clamp starts truncating.
- Never introduce `gap` between cards.
