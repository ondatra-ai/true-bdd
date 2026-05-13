package story

type ChangeLogEntry struct {
	Date        string `json:"date"        yaml:"date"`
	Version     string `json:"version"     yaml:"version"`
	Description string `json:"description" yaml:"description"`
	Author      string `json:"author"      yaml:"author"`
}
