package steps

import (
	"errors"
	"fmt"
)

var errValidationOrder = errors.New("validation does not follow the written checklist order")

type validationObservation struct {
	active  map[string]int
	next    map[string]int
	last    map[string]int
	count   int
	exited  bool
	recheck string
	overlap bool
	menus   []string
}

func newValidationObservation(count int) *validationObservation {
	return &validationObservation{active: map[string]int{}, next: map[string]int{}, last: map[string]int{}, count: count}
}

func (o *validationObservation) observe(record engineLogRecord) error {
	switch record.Msg {
	case "User chose fix action":
		o.exited = record.Action == "exit"
	case "Check answered":
		return o.question(record)
	case "Cell validation started":
		err := o.question(record)
		if err != nil {
			return err
		}

		o.active[record.SubjectID] = record.PromptIndex
		o.overlap = o.overlap || len(o.active) > 1
	case "Cell validation finished":
		return o.finish(record)
	case "Applying fix prompt":
		return o.apply(record)
	}

	return nil
}

func (o *validationObservation) question(record engineLogRecord) error {
	subject, index := record.SubjectID, record.PromptIndex
	if o.active[subject] != 0 {
		return fmt.Errorf("%w: parallel Q turns within %s", errValidationOrder, subject)
	}

	expected := o.next[subject]
	if expected == 0 || expected > o.count {
		expected = 1
	}

	if o.recheck != "" {
		if subject != o.recheck {
			return fmt.Errorf("%w: %s before %s recheck", errValidationOrder, subject, o.recheck)
		}

		expected = o.last[subject]
	}

	if index != expected || index < 1 || index > o.count {
		return fmt.Errorf("%w: %s Q index %d, want %d", errValidationOrder, subject, index, expected)
	}

	if o.recheck != "" {
		o.recheck = ""
		o.next[subject] = 1
	} else {
		o.next[subject] = index + 1
	}

	o.last[subject] = index

	return nil
}

func (o *validationObservation) complete() error {
	if len(o.active) > 0 || o.recheck != "" || len(o.next) == 0 {
		return fmt.Errorf("%w: unfinished validation or recheck", errValidationOrder)
	}

	for subject, next := range o.next {
		if next != o.count+1 && !o.exited {
			return fmt.Errorf("%w: incomplete final checklist for %s", errValidationOrder, subject)
		}
	}

	return nil
}

func (o *validationObservation) finish(record engineLogRecord) error {
	if o.active[record.SubjectID] != record.PromptIndex || record.PromptIndex == 0 {
		return fmt.Errorf("%w: unmatched model finish for %s at %d",
			errValidationOrder, record.SubjectID, record.PromptIndex)
	}

	delete(o.active, record.SubjectID)

	return nil
}

func (o *validationObservation) apply(record engineLogRecord) error {
	if len(o.active) > 0 || o.recheck != "" || o.last[record.SubjectID] == 0 {
		return fmt.Errorf("%w: apply before validation completed", errValidationOrder)
	}

	o.menus = append(o.menus, record.SubjectID)
	o.next = map[string]int{}
	o.recheck = record.SubjectID

	return nil
}
