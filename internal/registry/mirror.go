package registry

import (
	"net/url"
	"strings"
	"sync/atomic"
)

type MirrorSelector struct {
	mirrors []string
	next    atomic.Uint64
}

func NewMirrorSelector(mirrors []string) *MirrorSelector {
	normalized := make([]string, 0, len(mirrors))
	for _, mirror := range mirrors {
		if mirror == "" {
			continue
		}
		normalized = append(normalized, normalizeBaseURL(mirror))
	}
	return &MirrorSelector{mirrors: normalized}
}

func (m *MirrorSelector) Candidates(registryHost string) []string {
	source := normalizeBaseURL("https://" + registryHost)
	if len(m.mirrors) == 0 {
		return []string{source}
	}
	start := int(m.next.Add(1)-1) % len(m.mirrors)
	out := make([]string, 0, len(m.mirrors)+1)
	for i := 0; i < len(m.mirrors); i++ {
		out = append(out, m.mirrors[(start+i)%len(m.mirrors)])
	}
	out = append(out, source)
	return out
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}
