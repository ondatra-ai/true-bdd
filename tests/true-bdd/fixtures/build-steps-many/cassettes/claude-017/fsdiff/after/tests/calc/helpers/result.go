package helpers

import (
	"fmt"
	"strconv"
	"strings"
)

// ExpectedNumber reads the whole number a step wrote — an optional minus sign
// followed by digits — as an integer. The value comes only from the step text;
// nothing is derived from the program's output here.
func ExpectedNumber(expected string) (int, error) {
	number, err := strconv.Atoi(strings.TrimSpace(expected))
	if err != nil {
		return 0, fmt.Errorf("read the expected whole number %q the step wrote: %w", expected, err)
	}
	return number, nil
}

// CheckDoubled compares the integer the kept run of the calculator's double
// command printed on stdout with the whole number the step expects. The
// observed number is only what the program printed, and the expected number is
// only what the step wrote: no doubling is performed here. A missing run, an
// unreadable expected number, or a difference between the two is reported as an
// error.
func CheckDoubled(doubled *Doubled, expected string) error {
	if doubled == nil {
		return fmt.Errorf("observe what %s %s printed on stdout: no run was kept; the doubling When must run first", CalculatorName, DoubleCommand)
	}
	want, err := ExpectedNumber(expected)
	if err != nil {
		return err
	}
	if doubled.Value != want {
		return fmt.Errorf("%s %s printed %d on stdout (%q), want %d", CalculatorName, DoubleCommand, doubled.Value, doubled.Stdout, want)
	}
	return nil
}
