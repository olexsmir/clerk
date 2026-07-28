package analyzer

import (
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
)

type Analysis struct {
	Files []*journal.ParsedFile

	Transactions          []*ast.Transaction
	PeriodicTransactions  []*ast.PeriodicTransaction
	AutomatedTransactions []*ast.AutomatedTransaction
	Directives            []ast.Entry // account, commodity, payee, etc

	Accounts    map[string]*AccountInfo
	Commodities map[string]*CommodityInfo
	Payees      map[string]*PayeeInfo

	AccountNames     []string            // sorted for binary search
	PayeeNames       []string            // sorted, all payee names from directives + usage
	AccountsByPrefix map[string][]string // "expenses:" -> ["expenses:food", "expenses:taxi"]

	// PayeeTemplates holds the last transaction's postings per payee name.
	PayeeTemplates map[string][]PostingTemplate

	TagNames []string // unique, from tag directives

	Dates       []ast.Date // unique transaction dats in sorted order
	DateStrings []string   // same order as Dates, "YYYY-MM-DD"

	// CommodityDecimalMarks decimal mark per commodity, detected from D/amounts.
	CommodityDecimalMarks map[string]byte

	// TransactionsByKey groups transactions by [TxDuplicateKey] signature.
	TransactionsByKey map[string][]*ast.Transaction
}

type AccountInfo struct {
	Directives []*ast.AccountDirective
	Usages     []AccountUsage
	UsedCount  int
	LastUsed   ast.Date
}

type AccountUsage struct {
	FileIndex int
	Posting   *ast.Posting
}

type CommodityInfo struct {
	Directives []*ast.CommodityDirective
	Usages     []CommodityUsage
	UsedCount  int
	LastUsed   ast.Date
}

type CommodityUsage struct {
	FileIndex int
	Amount    *ast.Amount
}

type PayeeInfo struct {
	Directives []*ast.Payee
	Usage      []PayeeUsage
	UsedCount  int
	LastUsed   ast.Date
}

type PayeeUsage struct {
	FileIndex int
	Payee     *ast.Payee
}

type PostingTemplate struct {
	Account    string
	Amount     string
	Commodity  string
	IsInferred bool // true if the amount was inferred (auto-balanced)
}
