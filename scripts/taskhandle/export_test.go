package taskhandle

// The step logic is unexported; the tests reach it through this seam, which
// the compiler drops from any non-test build.

type Checklist = checklist

func NewChecklist() *Checklist { return newChecklist() }

func (c *checklist) Done(step Step, note string) { c.mark(step, markDone, note) }
func (c *checklist) Warn(step Step, note string) { c.mark(step, markWarn, note) }
func (c *checklist) Fail(step Step, note string) { c.mark(step, markFail, note) }
func (c *checklist) Skip(step Step, note string) { c.mark(step, markNone, note) }

type Budget = budget

func NewBudget() *Budget { return newBudget() }

func (b *budget) Spend(reason string) (int, error) { return b.spend(reason) }
func (b *budget) Spent() int                       { return b.spent() }

// Verify names what a Ticket is missing.
func Verify(detail Detail, headings []string) []string { return verify(detail, headings) }

// ParseGlobs and OutOfScope are the scope check.
func ParseGlobs(field string) []string              { return parseGlobs(field) }
func OutOfScope(changed, globs []string) []string   { return outOfScope(changed, globs) }

// Classify splits review findings the way step 6 does.
func Classify(findings []Finding) ([]Finding, []Finding) { return classify(findings) }

// Allowlists is every tool allowlist this package hands a turn, by name.
func Allowlists() map[string]string { return allowlists() }

// IsDecline and IsHalt read the terminal shapes.
func IsDecline(err error) bool { return isDecline(err) }
func IsHalt(err error) bool    { return isHalt(err) }
