package commands

// ChecklistModelSubject is the body checklist's validation view, without skeletons or fix context.
func (item *BuildStepsItem) ChecklistModelSubject() any {
	var concept any
	if item.Concept != nil {
		concept = map[string]string{"Means": item.Concept.Means}
	}

	label := ""
	if item.Concept != nil {
		label = item.Label()
	}

	return map[string]any{"Suite": item.Suite, "Label": label, "Concept": concept}
}
