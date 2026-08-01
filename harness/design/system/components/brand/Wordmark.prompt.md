One-line: renders the brand name as type wherever a logo would go — there is no supplied logo asset, so this is the mark.

```jsx
<Wordmark size={32} />
<Wordmark size={120} gradient />
```

- Poppins Bold, tight tracking. Never letterspace it, outline it, or set it in another face.
- `tone="light"` on black or gradient backgrounds.
- If a real logo file arrives, swap this component's internals — do not draw one.
