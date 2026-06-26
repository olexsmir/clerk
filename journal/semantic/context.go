package semantic

import (
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
)

// Context holds workspace-level semantic data built from parsed journal files.
type Context struct {
	// Files in dependency order (includes before includers).
	Files []*journal.ParsedFile

	Accounts    map[string]*AccountInfo
	Commodities map[string]*CommodityInfo
}

type AccountInfo struct {
	// len == 0 =  account is used but never declared (undeclared)
	// len  > 1 = account is declared more than once (duplicated)
	// len  > 0 = account appears in a posting
	Directives []*ast.AccountDirective
	Usages     []AccountUsage
}

// AccountUsage is a single posting that references an account.
type AccountUsage struct {
	FileIndex int
	Posting   *ast.Posting
}

// CommodityInfo tracks all declarations and usages for one commodity.
type CommodityInfo struct {
	Directives []*ast.CommodityDirective
	Usages     []CommodityUsage
}

// CommodityUsage is a single amount that references a commodity.
type CommodityUsage struct {
	FileIndex int
	Amount    *ast.Amount
}
