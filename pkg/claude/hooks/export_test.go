package hooks

// judge is the decode-and-ask half, which answers without touching either
// descriptor; the tests reach it through this seam.

func Judge(payload []byte, answer PostToolUseFunc) (string, bool) {
	return judge(payload, answer)
}

func Block(reason string) error { return block(reason) }
