// A failure's stack keeps only the test's own frame. The engine copies the
// failure message into its prompts, and the frames below it (Jest's
// internals, Node's task queue) shift line numbers between Node versions,
// which would make the recorded prompts differ from one machine to the next.
Error.stackTraceLimit = 1;
