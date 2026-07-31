package lsp

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestConfigMerge(t *testing.T) {
	tests := map[string]struct {
		cfg  Config
		in   string
		want Config
	}{
		"if not set, uses default":    {Config{}, `{"semanticHighlighting": true}`, Config{SemanticHighlighting: true}},
		"empty object keeps defaults": {DefaultConfig, `{}`, Config{SemanticHighlighting: true}},
		"nil settings keep defaults":  {DefaultConfig, "", Config{SemanticHighlighting: true}},

		"disables provided option": {DefaultConfig, `{"semanticHighlighting": false}`, Config{SemanticHighlighting: false}},
		"snake_case key":           {DefaultConfig, `{"semantic_highlighting": false}`, Config{SemanticHighlighting: false}},
		"kebab-case key":           {DefaultConfig, `{"semantic-highlighting": false}`, Config{SemanticHighlighting: false}},

		"malformed settings ignored":  {Config{SemanticHighlighting: true}, `{`, Config{SemanticHighlighting: true}},
		"non-object settings ignored": {Config{SemanticHighlighting: true}, `"clerk"`, Config{SemanticHighlighting: true}},
		"unknown fields ignored":      {Config{SemanticHighlighting: true}, `{"lint": true}`, Config{SemanticHighlighting: true}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tt.cfg.merge(protocol.LSPAny(tt.in))
			if tt.cfg != tt.want {
				t.Errorf("merge(%q) = %+v, want %+v", tt.in, tt.cfg, tt.want)
			}
		})
	}
}

func TestInitialize_AppliesInitializationOptions(t *testing.T) {
	srv := NewServer("test")
	res, err := srv.server.Initialize(t.Context(), &protocol.InitializeParams{
		InitializationOptions: protocol.LSPAny(`{"semanticHighlighting": false}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if srv.server.semanticHighlightingEnabled() {
		t.Error("semanticHighlighting=false in initializationOptions not applied")
	}
	if res.Capabilities.SemanticTokensProvider == nil {
		t.Error("SemanticTokensProvider must always be advertised")
	}
}
