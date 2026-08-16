package linter

import (
	"fmt"
	"strings"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/journal/ast"
)

// InvalidTypeTag flags account type declarations (the type: tag and the type
// subdirective) whose value is not a valid account type code.
type InvalidTypeTag struct{}

func (InvalidTypeTag) ID() RuleID         { return "invalid-type-tag" }
func (InvalidTypeTag) Severity() Severity { return SeverityError }
func (i *InvalidTypeTag) CheckJournal(an *analyzer.Analysis) []Find {
	var finds []Find
	for _, entry := range an.Directives {
		ad, ok := entry.(*ast.AccountDirective)
		if !ok {
			continue
		}
		if ad.Comment != nil {
			for _, tag := range ad.Comment.Tags {
				if tag.Key != "type" {
					continue
				}
				if err := i.parseAccountTypeCode(tag.Value); err != nil {
					finds = append(finds, Find{
						Code:     i.ID(),
						Severity: i.Severity(),
						Span:     tag.Span,
						Message:  fmt.Sprintf("invalid type: tag value %q", tag.Value),
					})
				}
			}
		}
		for _, sd := range ad.Subdirectives {
			if sd.Kind != ast.SubdirectiveType || sd.Value == "" {
				// empty values are flagged by the parser
				continue
			}
			if err := i.parseAccountTypeCode(sd.Value); err != nil {
				finds = append(finds, Find{
					Code:     i.ID(),
					Severity: i.Severity(),
					Span:     sd.ValueSpan,
					Message:  fmt.Sprintf("invalid type subdirective value %q", sd.Value),
				})
			}
		}
	}
	return finds
}

// parseAccountTypeCode validates account type code.
// Keep in sync with: https://github.com/simonmichael/hledger/blob/b589824e713eb53a1f91ae74b3038e75fb38ea6b/hledger-lib/Hledger/Read/JournalReader.hs#L550
func (i *InvalidTypeTag) parseAccountTypeCode(s string) error {
	switch strings.ToLower(s) {
	case "a", "asset":
	case "l", "liability":
	case "e", "equity":
	case "r", "revenue":
	case "x", "expense":
	case "c", "cash":
	case "v", "conversion":
	case "g", "gains":
	case "u", "unrealised", "unrealised-gain", "unrealised-gains", "unrealized", "unrealized-gain", "unrealized-gains":
	default:
		return fmt.Errorf("invalid account type code %q, should be one of "+
			"A, L, E, R, X, C, V, G, U, Asset, Liability, Equity, Revenue, "+
			"Expense, Cash, Conversion, Gains, UnrealisedGain", s)
	}
	return nil
}
