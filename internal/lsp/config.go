package lsp

import (
	"github.com/go-json-experiment/json"
	"go.lsp.dev/protocol"
)

type Config struct {
	SemanticHighlighting bool

	// TODO: Formatter
	// TODO: Linter
}

var DefaultConfig = Config{
	SemanticHighlighting: true,
}

func (c *Config) merge(v protocol.LSPAny) {
	var patch struct {
		SemanticHighlighting *bool `json:"semantic_highlighting,case:ignore"`
	}

	if err := json.Unmarshal(v, &patch); err != nil {
		return
	}
	if patch.SemanticHighlighting != nil {
		c.SemanticHighlighting = *patch.SemanticHighlighting
	}
}

func (s *server) semanticHighlightingEnabled() bool {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.config.SemanticHighlighting
}
