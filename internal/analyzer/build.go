package analyzer

import (
	"sort"
	"strings"

	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
)

// Build constructs [Analysis] from a flat resolved journal view.
func Build(rj *journal.ResolvedJournal) *Analysis {
	fileIndex := make(map[*journal.ParsedFile]int)
	for i, pf := range rj.Occurrences {
		fileIndex[pf] = i
	}

	a := &Analysis{
		Files:                   rj.Occurrences,
		Accounts:                make(map[string]*AccountInfo),
		AccountAliases:          make(map[string]string),
		Commodities:             make(map[string]*CommodityInfo),
		Payees:                  make(map[string]*PayeeInfo),
		Tags:                    make(map[string]*TagInfo),
		AccountsByPrefix:        make(map[string][]string),
		TransactionsByKey:       make(map[string][]*ast.Transaction),
		TransactionsCountByDate: make(map[string]int),
	}
	for _, item := range rj.Items {
		if item.IsInclude {
			continue
		}
		idx := fileIndex[item.Occurrence]
		a.addEntry(idx, item.Occurrence.Ast.Entries[item.EntryIndex])
	}
	a.buildPrefixIndex()
	a.sortAccountNames()
	a.collectPayeeNames()
	a.collectDates()
	a.collectTags()
	return a
}

func txDuplicateKey(tx *ast.Transaction, names []string) string {
	var b strings.Builder
	b.WriteString(tx.Date.String())
	b.WriteByte('|')
	if tx.Payee != nil {
		b.WriteString(tx.Payee.Name)
	}
	b.WriteByte('|')
	for i := range tx.Postings {
		b.WriteString(names[i])
		b.WriteByte(',')
	}
	return b.String()
}

// PayeeTemplates returns  the last Transactions's postings per payee name.
func (a *Analysis) PayeeTemplates() map[string][]PostingTemplate {
	templates := make(map[string][]PostingTemplate)
	for _, tx := range a.Transactions {
		payee := ""
		if tx.Payee != nil {
			payee = tx.Payee.Name
		}
		if payee == "" {
			continue
		}

		t := make([]PostingTemplate, len(tx.Postings))
		for i, p := range tx.Postings {
			t[i] = PostingTemplate{
				Account:    p.Account.String(),
				IsInferred: p.Amount == nil,
			}
			if p.Amount != nil {
				t[i].Amount = p.Amount.Quantity.String()
				t[i].Commodity = p.Amount.Commodity
			}
		}
		templates[payee] = t
	}
	return templates
}

func (a *Analysis) addEntry(fileIndex int, entry ast.Entry) {
	switch e := entry.(type) {
	case *ast.AccountDirective:
		a.addAccountDirective(e)
	case *ast.CommodityDirective:
		a.addCommodityDirective(e)
	case *ast.AliasDirective:
		a.addAliasDirective(e)
	case *ast.PayeeDirective:
		a.addPayeeDirective(e)
	case *ast.TagDirective:
		a.addTagDirective(e)
	case *ast.Comment:
		a.addCommentTags(fileIndex, nil, e)
	case *ast.Transaction:
		a.addPayee(fileIndex, e.Payee)
		a.Transactions = append(a.Transactions, e)
		a.TransactionsCountByDate[e.Date.String()]++

		names := make([]string, len(e.Postings))
		for i, p := range e.Postings {
			names[i] = p.Account.String()
		}
		a.addPostings(fileIndex, e.Postings, names, &e.Date)

		a.addCommentTags(fileIndex, &e.Date, e.Comment)
		for _, c := range e.HeaderComments {
			a.addCommentTags(fileIndex, &e.Date, c)
		}
		key := txDuplicateKey(e, names)
		a.TransactionsByKey[key] = append(a.TransactionsByKey[key], e)
	case *ast.PeriodicTransaction:
		a.PeriodicTransactions = append(a.PeriodicTransactions, e)
		a.addPostings(fileIndex, e.Postings, nil, nil)
		a.addCommentTags(fileIndex, nil, e.Comment)
		for _, c := range e.HeaderComments {
			a.addCommentTags(fileIndex, nil, c)
		}
	case *ast.AutomatedTransaction:
		a.AutomatedTransactions = append(a.AutomatedTransactions, e)
		a.addPostings(fileIndex, e.Postings, nil, nil)
		a.addCommentTags(fileIndex, nil, e.Comment)
		for _, c := range e.HeaderComments {
			a.addCommentTags(fileIndex, nil, c)
		}
	}

	// Every directive-like entry goes into Directives.
	switch entry.(type) {
	case *ast.AccountDirective, *ast.CommodityDirective, *ast.PayeeDirective, *ast.TagDirective, *ast.IncludeDirective,
		*ast.AliasDirective, *ast.YearDirective, *ast.DecimalMarkDirective, *ast.DefaultCommodityDirective, *ast.MarketPriceDirective,
		*ast.ConversionDirective, *ast.ApplyDirective, *ast.EndDirective, *ast.CommentBlockDirective, *ast.IgnoredDirective:
		a.Directives = append(a.Directives, entry)
	}
}

func (a *Analysis) addAccountDirective(ad *ast.AccountDirective) {
	aname := ad.Account.String()
	info, ok := a.Accounts[aname]
	if !ok {
		info = &AccountInfo{}
		a.Accounts[aname] = info
	}
	info.Directives = append(info.Directives, ad)
	for _, sd := range ad.Subdirectives {
		if sd.Kind == ast.SubdirectiveAlias {
			a.AccountAliases[sd.Value] = aname
		}
	}
}

// addAliasDirective records a top-level "alias A = B" directive; the alias
// source name resolves to the target account.
func (a *Analysis) addAliasDirective(ad *ast.AliasDirective) {
	a.AccountAliases[ad.From.String()] = ad.To.String()
	a.AliasDirectives = append(a.AliasDirectives, ad)
}

func (a *Analysis) addPayeeDirective(pd *ast.PayeeDirective) {
	if pd.Name == nil {
		return
	}
	info, ok := a.Payees[pd.Name.Name]
	if !ok {
		info = &PayeeInfo{}
		a.Payees[pd.Name.Name] = info
	}
	info.Directives = append(info.Directives, pd)
}

func (a *Analysis) addTagDirective(td *ast.TagDirective) {
	if td.Name == "" {
		return
	}
	info, ok := a.Tags[td.Name]
	if !ok {
		info = &TagInfo{}
		a.Tags[td.Name] = info
	}
	info.Directives = append(info.Directives, td)
}

func (a *Analysis) addCommentTags(fileIndex int, date *ast.Date, c *ast.Comment) {
	if c == nil {
		return
	}
	for i := range c.Tags {
		t := &c.Tags[i]
		info, ok := a.Tags[t.Key]
		if !ok {
			info = &TagInfo{}
			a.Tags[t.Key] = info
		}
		info.Usage = append(info.Usage, TagUsage{FileIndex: fileIndex, Tag: t})
		info.UsedCount++
		if date != nil {
			info.LastUsed = maxDate(info.LastUsed, *date)
		}
	}
}

func (a *Analysis) addCommodityDirective(cd *ast.CommodityDirective) {
	info, ok := a.Commodities[cd.Commodity]
	if !ok {
		info = &CommodityInfo{}
		a.Commodities[cd.Commodity] = info
	}
	info.Directives = append(info.Directives, cd)
}

func (a *Analysis) addPayee(fileIndex int, payee *ast.Payee) {
	if payee == nil {
		return
	}
	info, ok := a.Payees[payee.Name]
	if !ok {
		info = &PayeeInfo{}
		a.Payees[payee.Name] = info
	}
	info.Usage = append(info.Usage, PayeeUsage{
		FileIndex: fileIndex,
		Payee:     payee,
	})
	info.UsedCount++
}

func (a *Analysis) addPostings(fileIndex int, postings []ast.Posting, names []string, date *ast.Date) {
	if names == nil {
		names = make([]string, len(postings))
		for i, p := range postings {
			names[i] = p.Account.String()
		}
	}
	for i, posting := range postings {
		aname := names[i]
		info, ok := a.Accounts[aname]
		if !ok {
			info = &AccountInfo{}
			a.Accounts[aname] = info
		}
		info.Usages = append(info.Usages, AccountUsage{
			FileIndex: fileIndex,
			Posting:   &postings[i],
		})
		info.UsedCount++
		if date != nil {
			info.LastUsed = maxDate(info.LastUsed, *date)
		}

		a.addCommodityUsage(fileIndex, posting.Amount, date)
		if posting.Cost != nil {
			a.addCommodityUsage(fileIndex, &posting.Cost.Amount, date)
		}
		if posting.Balance != nil {
			a.addCommodityUsage(fileIndex, &posting.Balance.Amount, date)
			if posting.Balance.Cost != nil {
				a.addCommodityUsage(fileIndex, &posting.Balance.Cost.Amount, date)
			}
		}

		a.addCommentTags(fileIndex, date, posting.Comment)
		for i := range posting.Comments {
			a.addCommentTags(fileIndex, date, &posting.Comments[i])
		}
	}
}

func (a *Analysis) collectDates() {
	seen := make(map[string]bool)
	for _, tx := range a.Transactions {
		s := tx.Date.String()
		if s != "" && !seen[s] {
			seen[s] = true
			a.Dates = append(a.Dates, tx.Date)
		}
	}
	sort.Slice(a.Dates, func(i, j int) bool {
		return a.Dates[i].Compare(a.Dates[j]) < 0
	})
	a.DateStrings = make([]string, len(a.Dates))
	for i, d := range a.Dates {
		a.DateStrings[i] = d.String()
	}
}

func (a *Analysis) collectTags() {
	names := make([]string, 0, len(a.Tags))
	values := make(map[string]bool)
	for name, info := range a.Tags {
		names = append(names, name)
		seen := make(map[string]bool)
		for _, u := range info.Usage {
			if u.Tag.Value == "" {
				continue
			}
			seen[u.Tag.Value] = true
			values[u.Tag.Value] = true
		}
		if len(seen) > 0 {
			info.Values = sortedKeys(seen)
		}
	}
	sort.Strings(names)
	a.TagNames = names
	a.TagValues = sortedKeys(values)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CountTransactionsOnDate returns the number of transactions on d.
func (a *Analysis) CountTransactionsOnDate(d ast.Date) int {
	return a.TransactionsCountByDate[d.String()]
}

func (a *Analysis) addCommodityUsage(fileIndex int, am *ast.Amount, date *ast.Date) {
	if am == nil || am.Commodity == "" {
		return
	}
	info, ok := a.Commodities[am.Commodity]
	if !ok {
		info = &CommodityInfo{}
		a.Commodities[am.Commodity] = info
	}
	info.Usages = append(info.Usages, CommodityUsage{
		FileIndex: fileIndex,
		Amount:    am,
	})
	info.UsedCount++
	if date != nil {
		info.LastUsed = maxDate(info.LastUsed, *date)
	}
}

func (a *Analysis) buildPrefixIndex() {
	for name := range a.Accounts {
		parts := strings.Split(name, ":")
		for i := 1; i < len(parts); i++ {
			prefix := strings.Join(parts[:i], ":") + ":"
			a.AccountsByPrefix[prefix] = append(a.AccountsByPrefix[prefix], name)
		}
	}
}

func (a *Analysis) collectPayeeNames() {
	names := make([]string, 0, len(a.Payees))
	for name := range a.Payees {
		names = append(names, name)
	}
	sort.Strings(names)
	a.PayeeNames = names
}

func (a *Analysis) sortAccountNames() {
	names := make([]string, 0, len(a.Accounts))
	for name := range a.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	a.AccountNames = names
}

func maxDate(a, b ast.Date) ast.Date {
	if a.Year > b.Year ||
		(a.Year == b.Year && (a.Month > b.Month ||
			(a.Year == b.Year && a.Month == b.Month && a.Day > b.Day))) {
		return a
	}
	return b
}
