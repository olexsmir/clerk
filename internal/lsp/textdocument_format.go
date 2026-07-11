package lsp

import (
	"context"
	"fmt"
	"strings"

	"go.lsp.dev/protocol"
	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/parser"
)

func (s *server) Formatting(ctx context.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	text, ok := s.getDocText(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	lex := lexer.New(params.TextDocument.URI.Path(), []byte(text))
	jnrl := parser.New(lex).ParseJournal()
	if len(jnrl.Errors) > 0 {
		return nil, fmt.Errorf("can't format file with errors: %v", jnrl.Errors[0].Message) // TODO: report all errors
	}

	var buf strings.Builder
	if err := s.printer.Fprint(&buf, jnrl); err != nil {
		return nil, fmt.Errorf("format: %w", err)
	}

	newText := buf.String()
	if newText == text {
		return nil, nil
	}

	endLine, endChar := getEndLineAndChar(text)
	return []protocol.TextEdit{{
		NewText: newText,
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      0,
				Character: 0,
			},
			End: protocol.Position{
				Line:      uint32(endLine),
				Character: uint32(endChar),
			},
		},
	}}, nil
}

func getEndLineAndChar(txt string) (int, int) {
	endLine := 0
	lastNewline := -1
	for i, ch := range txt {
		if ch == '\n' {
			endLine++
			lastNewline = i
		}
	}
	endChar := 0
	if lastNewline >= 0 {
		if lastNewline == len(txt)-1 {
			endLine++
			endChar = 0
		} else {
			endChar = len(txt) - lastNewline - 1
		}
	} else {
		endChar = len(txt)
	}
	return endLine, endChar
}
