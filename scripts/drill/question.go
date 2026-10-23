package drill

import "time"

// Answer is an operator's single-line answer and its Specification citation.
type Answer struct {
	Answer   string `json:"answer"`
	Document string `json:"document"`
	Passage  string `json:"passage"`
}

// Question retains the entire printed question, its options and its answer file.
type Question struct {
	Prompt     string    `json:"prompt"`
	Options    []string  `json:"options"`
	AnswerFile string    `json:"answer_file"`
	Answer     string    `json:"answer"`
	Document   string    `json:"document"`
	Passage    string    `json:"passage"`
	AnsweredAt time.Time `json:"answered_at"`
}
