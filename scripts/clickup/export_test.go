package clickup

import "encoding/json"

// The field plan is unexported and reaches the filing turn as JSON, so the
// test asserts against those bytes through this export_test.go seam, which
// the compiler drops from any non-test build.

// PlanFieldsForTest is the plan the filing prompt carries, as it is embedded.
func PlanFieldsForTest(queue []Finding) []byte {
	encoded, err := json.MarshalIndent(planFields(queue), "", "  ")
	if err != nil {
		panic(err)
	}

	return encoded
}
