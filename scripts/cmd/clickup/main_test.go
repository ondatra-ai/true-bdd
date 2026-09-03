package main_test

import (
	"testing"

	main "github.com/ondatra-ai/true-bdd/scripts/cmd/clickup"
)

// `triage` carries two forms on one word. One bare number advances the
// backlog; anything else is the ticket a person named.
func TestSweepCountSeparatesTheTwoForms(t *testing.T) {
	t.Parallel()

	const ticket = "86cb9feh1"

	cases := []struct {
		args     []string
		count    int
		sweeping bool
	}{
		{[]string{"10"}, 10, true},
		{[]string{"1"}, 1, true},
		{[]string{ticket}, 0, false},
		{[]string{"https://app.clickup.com/t/90151491867/" + ticket}, 0, false},
		{[]string{ticket, "86cb8vktq"}, 0, false},
		{[]string{"10", ticket}, 0, false},
		{nil, 0, false},
	}

	for _, test := range cases {
		count, sweeping := main.SweepCountForTest(test.args)
		if count != test.count || sweeping != test.sweeping {
			t.Errorf("sweepCount(%q) = %d, %v; want %d, %v",
				test.args, count, sweeping, test.count, test.sweeping)
		}
	}
}
