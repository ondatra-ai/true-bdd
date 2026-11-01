package validate

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
)

var errSharedVerdictLost = errors.New("shared concept verdict lost: the walk restarted")

// conceptSharing is one walk's shared concept verdicts (#264): each key is judged once, by the first cell needing it.
type conceptSharing struct {
	mutex   sync.Mutex
	started bool
	entries map[string]*sharedVerdict
	links   map[string][]linkedStep
}

type sharedVerdict struct {
	done   chan struct{}
	answer string
	err    error
}

// linkedStep is one step a passing link gave a concept, for the walk's shared binding judgements.
type linkedStep struct {
	service string
	key     string
	step    conceptStep
}

func newConceptSharing() *conceptSharing {
	return &conceptSharing{}
}

func (s *conceptSharing) start() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.started = true
	s.entries = make(map[string]*sharedVerdict)
	s.links = make(map[string][]linkedStep)
}

func (s *conceptSharing) active() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.started
}

// resolve answers every key, judging each one no cell has claimed before it waits for the others' owners.
func (s *conceptSharing) resolve(ctx context.Context, keys []string,
	judge func(key string) (string, error),
) (map[string]string, error) {
	for key, entry := s.claim(keys); entry != nil; key, entry = s.claim(keys) {
		entry.answer, entry.err = judge(key)
		close(entry.done)
	}

	answers := make(map[string]string, len(keys))

	for _, key := range keys {
		entry := s.entry(key)
		if entry == nil {
			return nil, errSharedVerdictLost
		}

		select {
		case <-entry.done:
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for shared concept verdict: %w", ctx.Err())
		}

		if entry.err != nil {
			return nil, entry.err
		}

		answers[key] = entry.answer
	}

	return answers, nil
}

func (s *conceptSharing) claim(keys []string) (string, *sharedVerdict) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, key := range keys {
		if _, claimed := s.entries[key]; !claimed {
			entry := &sharedVerdict{done: make(chan struct{})}
			s.entries[key] = entry

			return key, entry
		}
	}

	return "", nil
}

func (s *conceptSharing) entry(key string) *sharedVerdict {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.entries[key]
}

// publish keeps a Scenario's passing link for the walk's shared binding judgements; a failed link keeps none.
func (s *conceptSharing) publish(subjectID string, steps []linkedStep) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.started {
		return
	}

	if len(steps) == 0 {
		delete(s.links, subjectID)

		return
	}

	s.links[subjectID] = steps
}

// stepsLinking are the walk's distinct steps whose passing link reached the concept key for the service.
func (s *conceptSharing) stepsLinking(key, service string) []conceptStep {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	unique := make(map[string]conceptStep)

	for _, steps := range s.links {
		for _, linked := range steps {
			if linked.key == key && linked.service == service {
				unique[linked.step.identity()] = linked.step
			}
		}
	}

	return rekeyedSteps(unique)
}

// rekeyedSteps orders distinct steps by block and text and keys them from 1.
func rekeyedSteps(unique map[string]conceptStep) []conceptStep {
	ordered := make([]conceptStep, 0, len(unique))
	for index, identity := range slices.Sorted(maps.Keys(unique)) {
		step := unique[identity]
		ordered = append(ordered, conceptStep{Key: strconv.Itoa(index + 1), Block: step.Block, Text: step.Text})
	}

	return ordered
}
