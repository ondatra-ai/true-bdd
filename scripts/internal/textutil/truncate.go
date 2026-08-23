package textutil

// Truncate returns the first limit runes of text, or text unchanged if it is
// no longer than that.
//
// Every `body[:4000]`, `title[:200]` and `stderr[:600]` in the scripts this
// ports was a Python slice, and Python slices a string by CODE POINT. Go
// slices by byte. The difference is not cosmetic here: CodeRabbit findings
// are full of multi-byte characters — the severity labels alone carry 🟠 and
// ⚠️, and the review bodies carry ✅ and ▸ — so a byte slice would both cut
// a different amount of text and split a rune, putting U+FFFD into a prompt
// or a ticket. Counting runes keeps the ported limits meaning what they meant.
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
