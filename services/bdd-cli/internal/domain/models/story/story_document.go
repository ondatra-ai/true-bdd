package story

// StoryDocument is the in-memory representation of a
// `docs/product/stories/<id>-*.yaml` file. Only the `story:` block is read;
// legacy sections are dropped.
type StoryDocument struct {
	Story Story `json:"story" yaml:"story"`
}
