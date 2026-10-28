package architecture

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ondatra-ai/true-bdd/services/true-bdd/internal/infrastructure/disk"
)

func parseHTTP(data []byte, path, root string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve OpenAPI document: %w", err)
	}

	hostRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve Host project root: %w", err)
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = localHTTPDocument(hostRoot)

	document, err := loader.LoadFromDataWithPath(data, &url.URL{Path: absolute})
	if err != nil {
		return fmt.Errorf("OpenAPI 3: %w", err)
	}

	if !strings.HasPrefix(document.OpenAPI, "3.") {
		return fmt.Errorf("%w: expected OpenAPI 3", ErrInterfaceDocument)
	}

	err = document.Validate(context.Background())
	if err != nil {
		return fmt.Errorf("OpenAPI 3: %w", err)
	}

	return nil
}

func localHTTPDocument(root string) openapi3.ReadFromURIFunc {
	return func(_ *openapi3.Loader, location *url.URL) ([]byte, error) {
		if location.Host != "" || (location.Scheme != "" && location.Scheme != "file") {
			return nil, fmt.Errorf("%s: %w: OpenAPI references must name local Host project documents",
 location, ErrInterfaceDocument)
		}

		relative, err := filepath.Rel(root, location.Path)
		if err != nil {
			return nil, fmt.Errorf("resolve OpenAPI reference %s: %w", location, err)
		}

		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("%s: %w: OpenAPI reference is outside the Host project", location, ErrInterfaceDocument)
		}

		data, err := disk.Read(location.Path)
		if err != nil {
			return nil, fmt.Errorf("read OpenAPI reference %s: %w", location, err)
		}

		err = validateInterfaceYAML(data)
		if err != nil {
			return nil, fmt.Errorf("parse OpenAPI reference %s: %w", location, err)
		}

		return data, nil
	}
}
