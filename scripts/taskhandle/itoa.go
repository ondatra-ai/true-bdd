package taskhandle

import "strconv"

// itoa is strconv.Itoa under a name short enough to read inside a checklist
// note built by concatenation.
func itoa(value int) string { return strconv.Itoa(value) }
