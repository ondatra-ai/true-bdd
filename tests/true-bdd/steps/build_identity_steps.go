package steps

import "regexp"

func assertBuildIdentityLogged(state *State, _ []string) error {
	records, err := readEngineLog(state)
	if err != nil {
		return err
	}

	if len(records) == 0 || records[0].Msg != "Engine started" || countEngineLog(records, "Engine started") != 1 {
		return state.fail("expected one Engine started record at the start of the engine log")
	}

	if records[0].Level != "INFO" {
		return state.fail("Engine started level = %q, want INFO", records[0].Level)
	}

	field := regexp.MustCompile(`^\S+$`)
	if !field.MatchString(records[0].Version) || !field.MatchString(records[0].Commit) {
		return state.fail("start record has invalid build identity %q/%q", records[0].Version, records[0].Commit)
	}

	return nil
}
