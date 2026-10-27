package release

// GuideCommands exposes the guide's commands to the tests that hold README.md to them.
func GuideCommands() []string { return guideCommands() }

// GuideFirstRun is the guide's first run, which the rehearsal starts through its typed wrapper.
const GuideFirstRun = guideFirstRun
