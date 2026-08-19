package lsp

import (
	"github.com/go-json-experiment/json"
	"go.lsp.dev/protocol"
)

type Config struct {
	SemanticHighlighting bool

	// LatinToCyrillicCompletion matches Latin input against Cyrillic labels.
	LatinToCyrillicCompletion bool

	// TODO: Formatter
	// TODO: Linter
}

var DefaultConfig = Config{
	SemanticHighlighting:      true,
	LatinToCyrillicCompletion: false,
}

func (c *Config) merge(v protocol.LSPAny) error {
	if len(v) == 0 { // no settings provided
		return nil
	}

	var patch struct {
		SemanticHighlighting      *bool `json:"semantic_highlighting,case:ignore"`
		LatinToCyrillicCompletion *bool `json:"latin_to_cyrillic_completion,case:ignore"`
	}

	if err := json.Unmarshal(v, &patch); err != nil {
		return err
	}
	if patch.SemanticHighlighting != nil {
		c.SemanticHighlighting = *patch.SemanticHighlighting
	}
	if patch.LatinToCyrillicCompletion != nil {
		c.LatinToCyrillicCompletion = *patch.LatinToCyrillicCompletion
	}
	return nil
}

func (s *server) semanticHighlightingEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.SemanticHighlighting
}

func (s *server) latinToCyrillicCompletionEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.LatinToCyrillicCompletion
}
