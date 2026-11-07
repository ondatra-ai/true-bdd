package architecture_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/architecture"
)

func TestLoadRecordCommandForms(t *testing.T) {
	t.Parallel()

	for _, declaration := range []string{
		`"record suite"`, `{command: "record suite"}`, `{command: "record suite", exclude: []}`,
		`{command: "record suite", exclude: ["cache/**", "**/private/**"]}`,
	} {
		t.Run(declaration, func(t *testing.T) {
			t.Parallel()

			commands := strings.Replace(allThreeCommands, `"go test -json ./... -mode=record"`, declaration, 1)

			arch, err := architecture.Load(writeSpec(t, specWith(commands)))
			if err != nil {
				t.Fatal(err)
			}

			if arch.Testing.Commands.Record != "record suite" {
				t.Fatalf("command lost: %+v", arch.Testing.Commands)
			}
		})
	}
}

func TestLoadRefusesMalformedRecordCommand(t *testing.T) {
	t.Parallel()

	for _, declaration := range []string{
		`[]`, `null`, `42`, `true`, `{}`, `{exclude: []}`, `{command: []}`, `{command: 42}`, `{command: null}`,
		`{command: " "}`, `{command: record, extra: value}`, `{command: record, exclude: private}`,
		`{command: record, exclude: null}`, `{command: record, exclude: [42]}`,
		`{command: record, exclude: [null]}`, `{command: record, command: other}`,
	} {
		t.Run(declaration, func(t *testing.T) {
			t.Parallel()

			commands := strings.Replace(allThreeCommands, `"go test -json ./... -mode=record"`, declaration, 1)

			_, err := architecture.Load(writeSpec(t, specWith(commands)))
			if err == nil || !strings.Contains(err.Error(), "testing.commands.record") {
				t.Fatalf("malformed record not named: %v", err)
			}
		})
	}
}

func TestLoadRecordStringHasNoExclusions(t *testing.T) {
	t.Parallel()

	arch, err := architecture.Load(writeSpec(t, specWith(allThreeCommands)))
	if err != nil {
		t.Fatal(err)
	}

	if len(arch.Testing.Commands.RecordExclude) != 0 {
		t.Fatalf("undeclared exclusions: %v", arch.Testing.Commands.RecordExclude)
	}
}

func TestLoadRecordExclusions(t *testing.T) {
	t.Parallel()

	commands := strings.Replace(allThreeCommands, `"go test -json ./... -mode=record"`,
		`{command: record, exclude: ["cache/**", "**/private/**"]}`, 1)

	arch, err := architecture.Load(writeSpec(t, specWith(commands)))
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(arch.Testing.Commands.RecordExclude, []string{"cache/**", "**/private/**"}) {
		t.Fatalf("exclusions lost: %+v", arch.Testing.Commands)
	}
}

func TestMalformedRecordHasDistinctError(t *testing.T) {
	t.Parallel()

	commands := strings.Replace(allThreeCommands, `"go test -json ./... -mode=record"`,
		`{command: record, exclude: false}`, 1)

	_, err := architecture.Load(writeSpec(t, specWith(commands)))
	if !errors.Is(err, architecture.ErrRecordCommand) {
		t.Fatalf("want ErrRecordCommand: %v", err)
	}
}
