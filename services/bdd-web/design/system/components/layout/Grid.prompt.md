One-line: the brand's Swiss 12-column grid — margin 0 (content runs to the page edge), gutter 20.

```jsx
<Grid>
  <GridItem span={7}><h1>Headline</h1></GridItem>
  <GridItem span={4} start={9}><p>Supporting copy.</p></GridItem>
</Grid>
```

- Margin is genuinely zero — do not add page padding to "fix" it.
- Common spans: 12 (full bleed), 7/5 (editorial split), 4×3 (module row), 6/6.
