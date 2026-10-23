package drill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ondatra-ai/true-bdd/pkg/disk"
)

type promptWriter struct {
	cancel    context.CancelFunc
	ctx       context.Context
	runner    Runner
	directory string
	tag       string
	tree      string
	log       string
	input     io.Writer
	pending   string
	failure   error
}

func (p *promptWriter) Write(data []byte) (int, error) {
	count, err := p.write(data)
	if err != nil {
		p.failure = err
		p.cancel()
	}

	return count, err
}

func (p *promptWriter) write(data []byte) (int, error) {
	err := disk.AppendBytes(p.log, data, disk.Private)
	if err != nil {
		return 0, err
	}

	p.pending += string(data)
	for {
		menu := strings.Index(p.pending, "Your choice (1/2/3): ")

		clarify := strings.Index(p.pending, "Your answer: ")
		switch {
		case menu >= 0 && (clarify < 0 || menu < clarify):
			p.pending = p.pending[menu+len("Your choice (1/2/3): "):]

			_, err = io.WriteString(p.input, "1\n")
			if err != nil {
				return 0, fmt.Errorf("answering menu: %w", err)
			}
		case clarify >= 0:
			prompt := p.pending[:clarify]

			p.pending = p.pending[clarify+len("Your answer: "):]

			err := p.clarify(prompt)
			if err != nil {
				return 0, err
			}
		default:
			return len(data), nil
		}
	}
}

func (p *promptWriter) clarify(prompt string) error {
	match := regexp.MustCompile(`(?m)^\[\d+/\d+\]`).FindAllStringIndex(prompt, -1)
	if len(match) == 0 {
		return fmt.Errorf("%w: clarification has no question", errRefused)
	}

	prompt = prompt[match[len(match)-1][0]:]

	record, err := loadUnchecked(p.directory)
	if err != nil {
		return err
	}

	index := len(record.Runs[p.tag].Questions)

	question := Question{Prompt: prompt,
		AnswerFile: filepath.Join(p.directory,
			"runs",
			p.tag,
			fmt.Sprintf("answer-%d.json",
				index+1)),
		Options: regexp.MustCompile(`(?m)^\s+\d+\) .+$`).FindAllString(prompt,
			-1)}

	err = changeRun(p.directory,
		p.tag,
		func(run *Run) {
			run.Status = "paused"
			run.Questions = append(run.Questions,
				question)
		})
	if err != nil {
		return err
	}

	answer, err := p.waitAnswer(question.AnswerFile)
	if err != nil {
		return err
	}

	err = changeRun(p.directory, p.tag, func(run *Run) {
		run.Status = "running"
		run.Questions[index].Answer = answer.Answer
		run.Questions[index].Document = answer.Document
		run.Questions[index].Passage = answer.Passage
		run.Questions[index].AnsweredAt = time.Now().UTC()
	})
	if err != nil {
		return err
	}

	_, err = io.WriteString(p.input, answer.Answer+"\n")
	if err != nil {
		return fmt.Errorf("answering clarification: %w", err)
	}

	return nil
}

func (p *promptWriter) waitAnswer(path string) (Answer, error) {
	ticker := time.NewTicker(p.runner.Poll)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return Answer{}, fmt.Errorf("waiting for answer: %w", p.ctx.Err())
		case <-ticker.C:
			data, err := disk.Read(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			if err != nil {
				return Answer{}, err
			}

			var answer Answer

			err = json.Unmarshal(data, &answer)
			if err != nil {
				return answer, fmt.Errorf("decoding operator answer: %w", err)
			}

			return answer, p.validateAnswer(answer)
		}
	}
}

func (p *promptWriter) validateAnswer(answer Answer) error {
	if strings.TrimSpace(answer.Answer) == "" || strings.ContainsAny(answer.Answer, "\r\n") || answer.Passage == "" {
		return fmt.Errorf("%w: answer needs one line and a Specification passage", errRefused)
	}

	document := filepath.ToSlash(filepath.Clean(answer.Document))
	if !specificationDocument(document) {
		return fmt.Errorf("%w: answer source is outside Specification", errRefused)
	}

	resolved := resolvePath(filepath.Join(p.tree, document))

	relative, err := filepath.Rel(resolvePath(p.tree), resolved)
	if err != nil || !specificationDocument(filepath.ToSlash(relative)) {
		return fmt.Errorf("%w: answer source resolves outside Specification",
			errRefused)
	}

	data, err := disk.Read(resolved)
	if err != nil {
		return err
	}

	if !strings.Contains(string(data), answer.Passage) {
		return fmt.Errorf("%w: cited passage does not occur in document", errRefused)
	}

	return nil
}

func specificationDocument(path string) bool {
	return strings.HasPrefix(path,
		"docs/product/") || path == "docs/architecture/architecture.yaml" || path == "docs/scenarios.yaml"
}
