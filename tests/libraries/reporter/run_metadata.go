package reporter

// RunMetadata is what the engine reported about its own setup and its
// interactive decisions — the context a timeline slice needs to be
// readable as something other than a duration.
type RunMetadata struct {
	ArchitectureFile string
	Services         int

	ChecklistCommand string
	ChecklistPath    string
	Items            int
	Prompts          int
	MaxApplyAttempts int

	// ItemsFile is where the walked items came from (the story file, for
	// `us apply`'s ACs). TargetFile is the file fixes actually mutate —
	// for `us apply` the scratch registry, nameable only at seed time.
	ItemsFile  string
	TargetFile string
	// CommittedFile is the canonical file the scratch is renamed over
	// once the walk converges. A run that converged therefore has NO
	// scratch left on disk — the fixes are only findable here.
	CommittedFile string

	// TestRun is the framework runner the engine spawned during
	// discovery, when it logged one.
	TestRun Invocation

	Decisions []Decision
}

// Decision is one branch point in the fix loop: what the engine asked
// for, what it was told, and what came of it.
type Decision struct {
	Kind      string
	Subject   string
	Section   string
	Iteration int
	Detail    string
}

// Metadata extracts the run's setup and decisions from the log.
func (l *EngineLog) Metadata() RunMetadata {
	var meta RunMetadata

	for index := range l.Records {
		record := &l.Records[index]
		meta.consume(record)
	}

	return meta
}

// consume folds one record into the metadata.
func (m *RunMetadata) consume(record *LogRecord) {
	switch record.Msg {
	case msgArchitecture:
		m.ArchitectureFile = record.File
		m.Services = intOr(record.Services)
	case msgChecklistLoad:
		m.ChecklistCommand = record.Command
		m.ChecklistPath = record.Path
	case msgStoryLoaded, msgStoryScenarios:
		m.ItemsFile = record.File
	case msgScratchSeeded:
		m.TargetFile = record.To
		m.CommittedFile = record.From
	case msgPromptsLoaded:
		m.Items = intOr(record.Items)
		m.Prompts = intOr(record.Prompts)
		m.MaxApplyAttempts = intOr(record.MaxApplyAttempts)
	case msgSpawnRunner:
		m.TestRun = Invocation{
			Binary: record.Binary, Args: record.Args, Dir: record.Dir, Known: true,
		}
	case msgFixGenerating, msgFixChoice, msgFixApplying, msgFixNotApplied:
		m.Decisions = append(m.Decisions, decisionFrom(record))
	}
}

// decisionFrom renders one fix-loop record as a decision line.
func decisionFrom(record *LogRecord) Decision {
	decision := Decision{
		Kind:      record.Msg,
		Subject:   record.SubjectID,
		Section:   record.Section,
		Iteration: intOr(record.Iteration),
	}

	switch record.Msg {
	case msgFixChoice:
		decision.Detail = "answered " + quoteArg(record.RawInput) +
			" — interpreted as " + record.Action
	case msgFixNotApplied:
		decision.Detail = record.Error
	}

	return decision
}

// intOr dereferences an optional count, treating an absent one as zero.
func intOr(value *int) int {
	if value == nil {
		return 0
	}

	return *value
}
