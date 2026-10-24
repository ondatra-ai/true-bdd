package testrunner

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var errReplayGrant = errors.New("replay command cannot be granted")

// CanonicalReplayCommand removes machine output and quotes argv for crush's shell.
func CanonicalReplayCommand(framework, command string) (string, string, error) {
	argv, err := SplitCommand(command)
	if err != nil {
		return "", "", err
	}

	flag, ok := machineReadableFlag(framework)
	if !ok {
		return "", "", fmt.Errorf("%w: unknown framework %s", errReplayGrant, framework)
	}

	boundary := "--"
	if framework == FrameworkGoTest {
		boundary = "-args"
	}

	argv = withoutOutputFlag(argv, flag, boundary)
	safe := regexp.MustCompile(`^[A-Za-z0-9_./:=,@%+\-]+$`)

	for idx, arg := range argv {
		if strings.ContainsAny(arg, "'\n\x00") {
			return "", "", fmt.Errorf("%w: argument contains a single quote, newline or NUL", errReplayGrant)
		}

		if !safe.MatchString(arg) {
			argv[idx] = "'" + arg + "'"
		}
	}

	selectors := map[string]string{
		FrameworkGoTest:     "-run",
		FrameworkJest:       "--testNamePattern",
		FrameworkPlaywright: "--grep",
	}

	return strings.Join(argv, " "), selectors[framework], nil
}

func withoutOutputFlag(argv []string, flag outputFlag, boundary string) []string {
	result := make([]string, 0, len(argv))
	for idx := 0; idx < len(argv); idx++ {
		arg := argv[idx]
		if arg == boundary {
			return append(result, argv[idx:]...)
		}

		name, value, equals := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") || name != flag.name {
			result = append(result, arg)

			continue
		}

		if flag.value == "" {
			continue
		}

		if !equals && idx+1 < len(argv) {
			idx++
			value = argv[idx]
		}

		result = append(result, remainingReporters(arg, value, flag.value, equals)...)
	}

	return result
}

// remainingReporters preserves explicitly requested non-JSON reporters.
func remainingReporters(arg, value, removed string, equals bool) []string {
	remaining := []string{}

	for _, reporter := range strings.Split(value, ",") {
		if reporter != removed {
			remaining = append(remaining, reporter)
		}
	}

	if len(remaining) == 0 {
		return nil
	}

	if equals {
		name, _, _ := strings.Cut(arg, "=")

		return []string{name + "=" + strings.Join(remaining, ",")}
	}

	return []string{arg, strings.Join(remaining, ",")}
}
