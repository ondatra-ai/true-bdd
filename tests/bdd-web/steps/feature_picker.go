package steps

import "fmt"

const (
	// pickerToggle opens the picker, pickerInput filters it and pickerOption is
	// one feature it offers, keyed by the feature it stands for.
	pickerToggle = "feature-picker-toggle"
	pickerInput  = "feature-picker-input"
	pickerOption = "feature-picker-option"
)

// pickFeature chooses one declared feature through the picker a container
// carries: filtered by the id, then taken from the options the filter left.
func pickFeature(state *State, container, feature string) error {
	err := openFeaturePicker(state, container)
	if err != nil {
		return err
	}

	err = fillElement(state, []string{"", childOf(container, pickerInput), feature})
	if err != nil {
		return err
	}

	return clickElement(state, []string{"",
		fmt.Sprintf("%s > %s[feature=%s]", container, pickerOption, feature)})
}

// coinFeature names a feature the product does not declare yet: typed into the
// picker's own input and confirmed with Enter, because the registry names no
// separate control for coining one.
func coinFeature(state *State, container, feature string) error {
	err := openFeaturePicker(state, container)
	if err != nil {
		return err
	}

	err = fillElement(state, []string{"", childOf(container, pickerInput), feature})
	if err != nil {
		return err
	}

	return pressTimes(state, "Enter", 1)
}

// openFeaturePicker clicks the toggle only when the input is not already
// showing: a second click on an open picker shuts it.
func openFeaturePicker(state *State, container string) error {
	shown, err := elementShown(state, childOf(container, pickerInput))
	if err != nil {
		return err
	}

	if shown {
		return nil
	}

	return clickElement(state, []string{"", childOf(container, pickerToggle)})
}

// childOf renders one element under another in the grammar a step's selector is
// written in.
func childOf(container, name string) string {
	return container + " > " + name
}
