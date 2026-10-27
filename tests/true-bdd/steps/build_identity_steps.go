package steps

import "regexp"

func assertBuildIdentityLogged(state *State, _ []string) error {
	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	answer := regexp.MustCompile(`(?m)^true-bdd version (\S+) commit (\S+)$`).FindStringSubmatch(state.Result.Stdout)
	if len(answer) == 0 {
		return state.fail("version answer has no build identity: %s", state.Result.Stdout)
	}

	if len(records) == 0 || records[0].Msg != "Engine started" || countEngineLog(records, "Engine started") != 1 {
		return state.fail("expected one Engine started record at the start of the engine log")
	}

	if records[0].Level != "INFO" {
		return state.fail("Engine started level = %q, want INFO", records[0].Level)
	}

	if records[0].Version != answer[1] || records[0].Commit != answer[2] {
		return state.fail("start record build identity %q/%q differs from version answer %q/%q",
			records[0].Version, records[0].Commit, answer[1], answer[2])
	}

	return nil
}
