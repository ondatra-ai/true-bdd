package architecture

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Library names code supplied as an exact-version Go module.
type Library struct {
	Module  string `yaml:"module"`
	Version string `yaml:"version"`
}

// ErrInvalidLibrary refuses ambiguous versions and unsupported library managers.
var ErrInvalidLibrary = errors.New("libraries require Go module paths and exact versions")

func (a *Architecture) validateLibraries() error {
	libraries := append([]Library(nil), a.Testing.Libraries...)
	if len(a.Testing.Libraries) > 0 && a.Testing.Framework != "go-test" {
		return ErrInvalidLibrary
	}

	for _, service := range a.Services {
		if len(service.Libraries) > 0 && service.Language != "go" {
			return ErrInvalidLibrary
		}

		libraries = append(libraries, service.Libraries...)
	}

	return validateLibraryVersions(libraries)
}

var (
	libraryModulePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/+\-]*$`)
	libraryVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+incompatible)?$`)
)

func validateLibraryVersions(libraries []Library) error {
	versions := make(map[string]string)

	for _, library := range libraries {
		if !libraryModulePattern.MatchString(library.Module) ||
			!libraryVersionPattern.MatchString(library.Version) {
			return fmt.Errorf("%s@%s: %w", library.Module, library.Version, ErrInvalidLibrary)
		}

		if previous := versions[library.Module]; previous != "" && previous != library.Version {
			return fmt.Errorf("conflicting versions for %s (%s, %s): %w", library.Module, previous, library.Version,
				ErrInvalidLibrary)
		}

		versions[library.Module] = library.Version
	}

	return nil
}

func (a *Architecture) libraries() []Library {
	libraries := append([]Library(nil), a.Testing.Libraries...)
	for _, service := range a.Services {
		libraries = append(libraries, service.Libraries...)
	}

	return libraries
}

func libraryContains(module, imported string) bool {
	return imported == module || strings.HasPrefix(imported, module+"/")
}
