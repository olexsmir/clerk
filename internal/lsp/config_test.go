package lsp

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestConfig_Merge(t *testing.T) {
	tests := map[string]struct {
		in        string
		cfg, want Config
	}{
		"if not set, uses default":    {`{"semanticHighlighting": true}`, Config{}, Config{SemanticHighlighting: true}},
		"empty object keeps defaults": {`{}`, DefaultConfig, Config{SemanticHighlighting: true}},
		"nil settings keep defaults":  {"", DefaultConfig, Config{SemanticHighlighting: true}},

		"disables provided option": {`{"semanticHighlighting": false}`, DefaultConfig, Config{SemanticHighlighting: false}},
		"snake_case key":           {`{"latin_to_cyrillic_completion": true}`, DefaultConfig, Config{LatinToCyrillicCompletion: true, SemanticHighlighting: true}},
		"kebab-case key":           {`{"semantic-highlighting": false}`, DefaultConfig, Config{SemanticHighlighting: false}},

		"malformed settings ignored":  {`{`, DefaultConfig, DefaultConfig},
		"non-object settings ignored": {`"clerk"`, DefaultConfig, DefaultConfig},
		"unknown fields ignored":      {`{"lint": true}`, DefaultConfig, DefaultConfig},
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

func TestServer_Intialize_config(t *testing.T) {
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
