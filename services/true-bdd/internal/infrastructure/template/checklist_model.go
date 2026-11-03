package template

// ChecklistModelSubject exposes only the fields this Model hands its templates.
func (data *StoryRefineData) ChecklistModelSubject() any { return data.Story }

// ChecklistModelSubject excludes fix documents and current-walk evidence.
func (data *ScenarioApplyData) ChecklistModelSubject() any {
	return map[string]any{
		"StoryID": data.StoryID, "StoryPath": data.StoryPath, "ACID": data.ACID,
		"LineageScenarioID": data.LineageScenarioID, "Description": data.Description,
		"RequirementsScratchPath": data.RequirementsScratchPath, "Steps": modelSteps(data.Steps),
	}
}

// ChecklistModelSubject excludes the Registry, fix documents and current-walk evidence.
func (data *ScenarioCheckData) ChecklistModelSubject() any {
	lineage := make([]map[string]string, 0, len(data.UserStories))
	for _, ref := range data.UserStories {
		lineage = append(lineage, map[string]string{"Story": ref.Story, "ScenarioID": ref.ScenarioID})
	}

	return map[string]any{
		"ID": data.ID, "Description": data.Description, "Service": data.Service, "Path": data.Path,
		"Steps": modelSteps(data.Steps), "UserStories": lineage,
	}
}

func modelSteps(steps MergedSteps) map[string][]string {
	return map[string][]string{"Given": steps.Given, "When": steps.When, "Then": steps.Then}
}
