package cmd_test

import (
	"slices"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/cmd"
)

func TestBuildTestsTakesNoModelOverride(t *testing.T) {
	command, _, err := cmd.NewBuildCommand(nil).Find([]string{"tests"})
	if err != nil {
		t.Fatal(err)
	}

	if command.Flags().Lookup("model") != nil {
		t.Fatal("build tests dispatches no AI turn, so it must not offer --model")
	}
}

func TestEveryAITurnCommandAcceptsRepeatedModelOverrides(t *testing.T) {
	for _, group := range []struct {
		name     string
		commands []string
	}{
		{"build", []string{"steps", "code"}}, {"us", []string{"refine", "apply"}}, {"scen", []string{"check"}},
	} {
		for _, name := range group.commands {
			t.Run(group.name+" "+name, func(t *testing.T) {
				root := cmd.NewBuildCommand(nil)
				if group.name == "us" {
					root = cmd.NewUSCommand(nil)
				}

				if group.name == "scen" {
					root = cmd.NewScenCommand(nil)
				}

				command, _, err := root.Find([]string{name})
				if err != nil {
					t.Fatal(err)
				}

				values := []string{"coder=claude:one", "quick=claude:two,three"}

				err = command.ParseFlags([]string{"--model", values[0], "--model", values[1]})
				if err != nil {
					t.Fatal(err)
				}

				got, err := command.Flags().GetStringArray("model")
				if err != nil || !slices.Equal(got, values) {
					t.Fatalf("overrides = %v, %v", got, err)
				}
			})
		}
	}
}
