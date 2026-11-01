package steps

type validationObservation struct {
	active  map[string]bool
	gates   map[string]bool
	overlap bool
	menus   []string
}

func newValidationObservation() *validationObservation {
	return &validationObservation{active: map[string]bool{}, gates: map[string]bool{}}
}

func (o *validationObservation) observe(state *State, record engineLogRecord) error {
	switch record.Msg {
	case "Check answered":
		o.gates[record.SubjectID] = true
	case "Cell validation started":
		return o.start(state, record.SubjectID)
	case "Cell validation finished":
		delete(o.active, record.SubjectID)
	case "Applying fix prompt":
		if len(o.active) > 0 {
			return state.fail("fix before validation completed")
		}

		o.menus = append(o.menus, record.SubjectID)
		o.gates = map[string]bool{}
	}

	return nil
}

func (o *validationObservation) start(state *State, subject string) error {
	if !o.gates[subject] || subject == "gamma" {
		return state.fail("Q before D or after failed D for %s", subject)
	}

	if o.active[subject] {
		return state.fail("parallel Q turns within %s", subject)
	}

	o.active[subject] = true
	o.overlap = o.overlap || len(o.active) > 1

	return nil
}
