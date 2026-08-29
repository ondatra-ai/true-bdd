package taskhandle

import "strconv"

// retries is the ONE cap the two recoveries share — red gates at step 5 and
// review findings at step 6. The sixth is not a retry.
const retries = 5

type budget struct{ left int }

func newBudget() *budget { return &budget{left: retries} }

// spend takes one unit and reports which attempt this is, or declines. The
// step text wins over the Halting section, which calls the sixth a halt:
// Declining names "gates red after five retries" among its own cases.
func (b *budget) spend(reason string) (int, error) {
	if b.left <= 0 {
		return 0, decline(reason + " after " + strconv.Itoa(retries) + " retries")
	}

	b.left--

	return retries - b.left, nil
}

// spent is how many units have gone, for the checklist note.
func (b *budget) spent() int { return retries - b.left }
