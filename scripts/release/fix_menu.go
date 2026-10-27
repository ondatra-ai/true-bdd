package release

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ondatra-ai/true-bdd/pkg/cli"
	"github.com/ondatra-ai/true-bdd/pkg/cli/truebdd"
)

type fixMenu struct {
	output  bytes.Buffer
	pending string
	input   io.WriteCloser
}

func (menu *fixMenu) Write(data []byte) (int, error) {
	const (
		choice        = "Your choice (1/2/3): "
		clarification = "CLARIFICATION NEEDED"
	)

	menu.output.Write(data)

	menu.pending += string(data)
	if strings.Contains(menu.pending, clarification) {
		_ = menu.input.Close()
	} else if strings.Contains(menu.pending, choice) {
		menu.pending = ""

		_, err := io.WriteString(menu.input, "1\n")
		if err != nil {
			return 0, fmt.Errorf("answering rehearsal fix menu: %w", err)
		}
	}

	const retainedBytes = 128
	if len(menu.pending) > retainedBytes {
		menu.pending = menu.pending[len(menu.pending)-retainedBytes:]
	}

	return len(data), nil
}

func rehearseCode(ctx context.Context, binary string, live bool, opt cli.Options) (cli.Result, error) {
	if !live {
		return truebdd.RehearseCode(ctx, binary, opt)
	}

	input, answer, err := os.Pipe()
	if err != nil {
		return cli.Result{}, fmt.Errorf("opening rehearsal input: %w", err)
	}
	defer func() { _ = input.Close() }()
	defer func() { _ = answer.Close() }()

	menu := &fixMenu{input: answer}

	var stderr bytes.Buffer

	opt.Stdin = input
	opt.Output = cli.Streams(menu, &stderr)
	result, err := truebdd.RehearseCode(ctx, binary, opt)
	result.Stdout, result.Stderr = menu.output.String(), stderr.String()

	return result, err
}
