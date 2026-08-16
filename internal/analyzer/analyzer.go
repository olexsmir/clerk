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

	AccountAliases  map[string]string     // maps alias names to the account they resovle to
	AliasDirectives []*ast.AliasDirective // holds every top-level "alias A = B" directive in entry order

	AccountNames     []string                     // sorted for binary search
	PayeeNames       []string                     // sorted, all payee names from directives + usage
	PayeeTemplates   map[string][]PostingTemplate // holds the last transaction's postings per payee name
	AccountsByPrefix map[string][]string          // "expenses:" -> ["expenses:food", "expenses:taxi"]

	Tags      map[string]*TagInfo
	TagNames  []string // unique tag names, sorted
	TagValues []string // unique non-empty tag values across all tags, sorted

	Dates       []ast.Date // unique transaction dates, sorted
	DateStrings []string   // same order as Dates, rendered as written

	TransactionsCountByDate map[string]int                // counts transactions per date, keyed by [Date.String]
	TransactionsByKey       map[string][]*ast.Transaction // groups transactions by [TxDuplicateKey] signature
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
	Directives []*ast.PayeeDirective
	Usage      []PayeeUsage
	UsedCount  int
	LastUsed   ast.Date
}

type PayeeUsage struct {
	FileIndex int
	Payee     *ast.Payee
}

type TagInfo struct {
	Directives []*ast.TagDirective
	Usage      []TagUsage
	UsedCount  int
	LastUsed   ast.Date
	Values     []string // unique non-empty values from comment usages, sorted
}

type TagUsage struct {
	FileIndex int
	Tag       *ast.Tag
}

type PostingTemplate struct {
	Account    string
	Amount     string
	Commodity  string
	IsInferred bool // true if the amount was inferred (auto-balanced)
}
