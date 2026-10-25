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
	"sync"
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
	trailer   string
	failure   error
	failureMu sync.Mutex
	answers   sync.WaitGroup
}

func (p *promptWriter) Write(data []byte) (int, error) {
	count, err := p.write(data)
	if err != nil {
		p.fail(err)
	}

	return count, err
}

func (p *promptWriter) write(data []byte) (int, error) {
	err := disk.AppendBytes(p.log, data, disk.Private)
	if err != nil {
		return 0, err
	}

	p.pending += string(data)
	for !p.skipTrailer() {
		consumed, err := p.consumePrompt()
		if err != nil {
			return 0, err
		}

		if !consumed {
			break
		}
	}

	return len(data), nil
}

func (p *promptWriter) consumePrompt() (bool, error) {
	const (
		menuForm = `(?m)^[ \t]*\[1\] Apply this fix to working copy[ \t]*\r?\n` +
			`[ \t]*\[2\] Refine the fix prompt(?: \(provide feedback\))?[ \t]*\r?\n` +
			`[ \t]*\[3\] Exit without applying[ \t\r\n]*\z`
		clarificationForm = `(?m)^[ \t]*1\) .+\n(?:[ \t]*[1-9][0-9]*\) .+\n)*` +
			`[ \t]*0\) Other(?: \(type custom answer\))?[ \t\r\n]*\z`
	)

	menu := strings.Index(p.pending, "Your choice (1/2/3): ")

	clarify := strings.Index(p.pending, "Your answer: ")
	switch {
	case menu >= 0 && (clarify < 0 || menu < clarify):
		return true, p.applyMenu(menu+len("Your choice (1/2/3): "), false)
	case clarify >= 0:
		return true, p.pauseQuestion(clarify, clarify+len("Your answer: "), false)
	case regexp.MustCompile(menuForm).MatchString(p.pending):
		return true, p.applyMenu(len(p.pending), true)
	case regexp.MustCompile(clarificationForm).MatchString(p.pending):
		return true, p.pauseQuestion(len(p.pending), len(p.pending), true)
	default:
		return false, nil
	}
}

func (p *promptWriter) applyMenu(end int, form bool) error {
	p.pending = p.pending[end:]
	if form {
		p.trailer = "Your choice (1/2/3): "
	}

	_, err := io.WriteString(p.input, "1\n")
	if err != nil {
		return fmt.Errorf("answering menu: %w", err)
	}

	return nil
}

func (p *promptWriter) pauseQuestion(start, end int, form bool) error {
	prompt := p.pending[:start]

	p.pending = p.pending[end:]
	if form {
		p.trailer = "Your answer: "
	}

	return p.clarify(prompt)
}

func (p *promptWriter) skipTrailer() bool {
	if p.trailer == "" {
		return false
	}

	p.pending = strings.TrimLeft(p.pending, " \t\r\n=")
	if p.trailer == "Your answer: " && strings.HasPrefix(p.pending, "(") {
		const otherLabel = "(type custom answer)"
		if strings.HasPrefix(otherLabel, p.pending) {
			return true
		}

		p.pending = strings.TrimPrefix(p.pending, otherLabel)
		p.pending = strings.TrimLeft(p.pending, " \t\r\n")
	}

	if strings.HasPrefix(p.trailer, p.pending) {
		return true
	}

	p.pending = strings.TrimPrefix(p.pending, p.trailer)

	p.trailer = ""

	return false
}

func (p *promptWriter) clarify(prompt string) error {
	match := regexp.MustCompile(`(?m)^\[\d+/\d+\]`).FindAllStringIndex(prompt, -1)
	if len(match) > 0 {
		prompt = prompt[match[len(match)-1][0]:]
	}

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
		Options: regexp.MustCompile(`(?m)^[ \t]*\d+\) .+$`).FindAllString(prompt,
			-1)}

	err = changeRun(p.directory,
		p.tag,
		func(run *Attempt) {
			run.Status = "paused"
			run.Questions = append(run.Questions,
				question)
		})
	if err != nil {
		return err
	}

	p.answers.Go(func() {
		err := p.answerQuestion(index, question)
		if err != nil {
			p.fail(err)
		}
	})

	return nil
}

func (p *promptWriter) fail(err error) {
	p.failureMu.Lock()
	p.failure = errors.Join(p.failure, err)
	p.failureMu.Unlock()
	p.cancel()
}

func (p *promptWriter) answerQuestion(index int, question Question) error {
	answer, err := p.waitAnswer(question.AnswerFile)
	if err != nil {
		return err
	}

	err = changeRun(p.directory, p.tag, func(run *Attempt) {
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
