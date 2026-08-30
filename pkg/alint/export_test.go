package alint

// The request builder and the gate call answer without exiting the process;
// the tests reach them through this seam.

func Request(args []string) AlintLintParams { return request(args) }

func Answer(args []string, gate Gate) error { return answer(args, gate) }
