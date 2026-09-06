package steps

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/testkit/bddgo"
)

// registerRedisSteps binds the clauses that read the relay's coordination
// state directly: a relay that FORWARDS a reply rather than storing it, and
// one that reclaims an orphan on a TTL, are only observable at the backend.
func registerRedisSteps(suite *bddgo.Suite[State]) {
	suite.Step(`^the Redis key "([^"]+)" is gone$`, assertRedisKeyGone)
	suite.Step(`^the Redis key "([^"]+)" does not hold the late reply$`, assertNoLateReply)
	suite.Step(`^work "([^"]+)" is in state "([^"]+)"$`, assertWorkState)
	suite.Step(`^work "([^"]+)" has a positive TTL of at most (\d+) seconds?$`, assertWorkTTL)
}

// assertRedisKeyGone watches the key until it is absent: a reply key goes on
// the answer, a work record on its TTL, and both are "gone" to the scenario.
func assertRedisKeyGone(state *State, args []string) error {
	named := args[0]

	client, key, err := openKey(state, named)
	if err != nil {
		return err
	}

	defer client.close()

	deadline := time.Now().Add(redisGoneTimeout)

	for {
		present, existsErr := redisExists(state, client, key)
		if existsErr != nil {
			return existsErr
		}

		if !present {
			return nil
		}

		if !time.Now().Before(deadline) {
			held, _ := redisGet(state, client, key)

			return state.fail("the Redis key %q (%s) is still held after %s: %s",
				named, key, redisGoneTimeout, held)
		}

		time.Sleep(redisPollInterval)
	}
}

// assertNoLateReply holds the key to not carrying the bytes the late reply
// sent: a relay that stored it would serve the caller a body nobody is
// waiting for. An absent key satisfies the clause outright.
func assertNoLateReply(state *State, args []string) error {
	named := args[0]

	_, label, _ := strings.Cut(named, ":")

	marker, sent := state.LateReplies[label]
	if !sent {
		return state.fail("%w: work %q", ErrNoLateReply, label)
	}

	client, key, err := openKey(state, named)
	if err != nil {
		return err
	}

	defer client.close()

	held, err := redisGet(state, client, key)
	if err != nil {
		return err
	}

	if strings.Contains(held, marker) {
		return state.fail("the Redis key %q (%s) holds the late reply %q: %s",
			named, key, marker, held)
	}

	return nil
}

// assertWorkState waits for the relay's record of the claimed work to reach
// the state the step names — orphaned is what a claim nobody is waiting for
// becomes, and it is a fact about the record rather than about a response.
func assertWorkState(state *State, args []string) error {
	label, want := args[0], args[1]

	item, err := lookupWork(state, label)
	if err != nil {
		return err
	}

	client, err := dialRedis(state)
	if err != nil {
		return err
	}

	defer client.close()

	key := workRecordKey(state, item)
	deadline := time.Now().Add(workStateTimeout)

	for {
		held, getErr := redisGet(state, client, key)
		if getErr != nil {
			return getErr
		}

		if recordState(held) == want {
			return nil
		}

		if !time.Now().Before(deadline) {
			return state.fail("work %q (%s) is in state %q after %s, want %q: %s",
				label, key, recordState(held), workStateTimeout, want, held)
		}

		time.Sleep(redisPollInterval)
	}
}

// recordState reads the state off a work record, calling an absent or
// unreadable one what it is rather than failing the poll on it.
func recordState(held string) string {
	if held == "" {
		return "(no record)"
	}

	var record struct {
		State string `json:"state"`
	}

	err := json.Unmarshal([]byte(held), &record)
	if err != nil {
		return "(unreadable)"
	}

	return record.State
}

// assertWorkTTL holds the record to expiring, and to expiring within the
// window the step names: no TTL at all is a record that outlives the relay.
func assertWorkTTL(state *State, args []string) error {
	label := args[0]

	limit, err := strconv.Atoi(args[1])
	if err != nil {
		return state.fail("the step names %q seconds, which is not a number: %w", args[1], err)
	}

	item, err := lookupWork(state, label)
	if err != nil {
		return err
	}

	client, err := dialRedis(state)
	if err != nil {
		return err
	}

	defer client.close()

	key := workRecordKey(state, item)

	seconds, err := redisTTL(state, client, key)
	if err != nil {
		return err
	}

	if seconds <= 0 || seconds > limit {
		return state.fail("work %q's record (%s) has TTL %ds "+
			"(-1 is no expiry, -2 is gone), want a positive TTL of at most %ds",
			label, key, seconds, limit)
	}

	return nil
}
