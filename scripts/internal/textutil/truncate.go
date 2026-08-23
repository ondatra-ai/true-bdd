package textutil

// Truncate returns the first limit runes of text, unchanged if shorter.
// Not a byte slice: CodeRabbit findings carry multi-byte emoji (🟠 ⚠️ ✅ ▸),
// and text[:limit] would split one, putting U+FFFD into a prompt or ticket.
func Truncate(text string, limit int) string {
	if limit <= 0 {
		return ""
	}

	count := 0
	for index := range text {
		if count == limit {
			return text[:index]
		}

		count++
	}

	return text
}
