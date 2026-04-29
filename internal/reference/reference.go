package reference

import (
	"fmt"
	"strings"
)

const dockerHubRegistry = "registry-1.docker.io"

type ImageReference struct {
	Original   string
	Registry   string
	Repository string
	Reference  string
}

func Parse(raw string) (ImageReference, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ImageReference{}, fmt.Errorf("image reference is empty")
	}

	ref := "latest"
	name := trimmed
	lastSlash := strings.LastIndex(name, "/")
	lastColon := strings.LastIndex(name, ":")
	if lastColon > lastSlash {
		ref = name[lastColon+1:]
		name = name[:lastColon]
	}
	if strings.Contains(ref, "@") {
		return ImageReference{}, fmt.Errorf("unsupported image reference: %s", raw)
	}

	parts := strings.Split(name, "/")
	registry := dockerHubRegistry
	repository := name
	if len(parts) == 1 {
		repository = "library/" + parts[0]
	} else if isRegistry(parts[0]) {
		registry = parts[0]
		repository = strings.Join(parts[1:], "/")
	} else {
		repository = name
	}

	if registry == "docker.io" {
		registry = dockerHubRegistry
	}
	if registry == dockerHubRegistry && !strings.Contains(repository, "/") {
		repository = "library/" + repository
	}
	if repository == "" {
		return ImageReference{}, fmt.Errorf("invalid image repository: %s", raw)
	}
	return ImageReference{
		Original:   raw,
		Registry:   registry,
		Repository: repository,
		Reference:  ref,
	}, nil
}

func isRegistry(part string) bool {
	return strings.Contains(part, ".") || strings.Contains(part, ":") || part == "localhost"
}
