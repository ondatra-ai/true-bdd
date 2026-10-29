package architecture

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// cliInterface is the supported OpenCLI-like draft shape, with named extensions.
type cliInterface struct {
	OpenCLI string `yaml:"opencli"`
	Info    struct {
		Title   string `yaml:"title"`
		Version string `yaml:"version"`
	} `yaml:"info"`
	Commands   map[string]cliCommand `yaml:"commands"`
	Extensions cliExtensions         `yaml:"extensions"`
}

type cliCommand struct {
	Description string                 `yaml:"description"`
	Parameters  []cliParameter         `yaml:"parameters"`
	Responses   map[string]cliResponse `yaml:"responses"`
}

type cliParameter struct {
	Name        string `yaml:"name"`
	In          string `yaml:"in"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Schema      struct {
		Type string `yaml:"type"`
	} `yaml:"schema"`
}

type cliResponse struct {
	Description string         `yaml:"description"`
	Content     map[string]any `yaml:"content"`
}

type cliExtensions struct {
	Files         map[string]cliFile `yaml:"files"`
	LogRecords    map[string]cliLog  `yaml:"log_records"`
	OutgoingCalls map[string]cliCall `yaml:"outgoing_calls"`
}

type cliFile struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"`
	Access string `yaml:"access"`
}

type cliLog struct {
	Message string `yaml:"message"`
	Format  string `yaml:"format"`
}

type cliCall struct {
	Command   string   `yaml:"command"`
	Arguments []string `yaml:"arguments"`
}

func parseCLI(declaration *PublicInterface, data []byte) error {
	var document cliInterface

	err := yaml.Unmarshal(data, &document)
	if err != nil {
		return fmt.Errorf("OpenCLI draft: %w", err)
	}

	if !hasText(document.OpenCLI, document.Info.Title, document.Info.Version) || len(document.Commands) == 0 {
		return fmt.Errorf("%w: OpenCLI draft requires opencli, info.title, info.version and commands", ErrInterfaceDocument)
	}

	for name, command := range document.Commands {
		err := command.validate(name)
		if err != nil {
			return err
		}
	}

	err = document.Extensions.validate()
	if err != nil {
		return err
	}

	declaration.references = document.references()

	return nil
}

func (document cliInterface) references() map[string]bool {
	references := make(map[string]bool)
	for name := range document.Commands {
		references[(InterfaceReference{Command: name}).String()] = true
	}

	for name := range document.Extensions.Files {
		references[(InterfaceReference{File: name}).String()] = true
	}

	for name := range document.Extensions.LogRecords {
		references[(InterfaceReference{LogRecord: name}).String()] = true
	}

	for name := range document.Extensions.OutgoingCalls {
		references[(InterfaceReference{OutgoingCall: name}).String()] = true
	}

	return references
}

func (c cliCommand) validate(name string) error {
	if !hasText(name) || len(c.Responses) == 0 {
		return fmt.Errorf("%w: CLI command %q requires a name and responses for its exit codes", ErrInterfaceDocument, name)
	}

	for _, parameter := range c.Parameters {
		if !parameter.valid() {
			return fmt.Errorf("%w: CLI command %q parameter requires name and in (argument, flag or option)",
				ErrInterfaceDocument, name)
		}
	}

	return nil
}

func (p cliParameter) valid() bool {
	if !hasText(p.Name) {
		return false
	}

	switch p.In {
	case "", "argument", "flag", "option":
		return true
	default:
		return false
	}
}

func hasText(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}

	return true
}

func (e cliExtensions) validate() error {
	err := e.validateFiles()
	if err != nil {
		return err
	}

	err = e.validateLogs()
	if err != nil {
		return err
	}

	return e.validateCalls()
}

func (e cliExtensions) validateFiles() error {
	for name, file := range e.Files {
		if !hasText(name, file.Path, file.Format) ||
			(file.Access != "read" && file.Access != "write" && file.Access != "read-write") {
			return fmt.Errorf("%w: extension files.%s requires path, format and access (read, write or read-write)",
				ErrInterfaceDocument, name)
		}
	}

	return nil
}

func (e cliExtensions) validateLogs() error {
	for name, record := range e.LogRecords {
		if !hasText(name, record.Message, record.Format) {
			return fmt.Errorf("%w: extension log_records.%s requires message and format", ErrInterfaceDocument, name)
		}
	}

	return nil
}

func (e cliExtensions) validateCalls() error {
	for name, call := range e.OutgoingCalls {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(call.Command) == "" {
			return fmt.Errorf("%w: extension outgoing_calls.%s requires command", ErrInterfaceDocument, name)
		}
	}

	return nil
}
