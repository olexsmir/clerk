package analyzer

import (
	"fmt"
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
		Files:                 rj.Occurrences,
		Accounts:              make(map[string]*AccountInfo),
		AccountAliases:        make(map[string]string),
		Commodities:           make(map[string]*CommodityInfo),
		Payees:                make(map[string]*PayeeInfo),
		Tags:                  make(map[string]*TagInfo),
		AccountsByPrefix:      make(map[string][]string),
		PayeeTemplates:        make(map[string][]PostingTemplate),
		CommodityDecimalMarks: make(map[string]byte),
		TransactionsByKey:     make(map[string][]*ast.Transaction),
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

func TxDuplicateKey(tx *ast.Transaction) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%04d-%02d-%02d|", tx.Date.Year, tx.Date.Month, tx.Date.Day)
	if tx.Payee != nil {
		b.WriteString(tx.Payee.Name)
	}
	b.WriteByte('|')
	for _, p := range tx.Postings {
		b.WriteString(p.Account.String())
		b.WriteByte(',')
	}
	return b.String()
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
		a.Transactions = append(a.Transactions, e)
		a.addPostings(fileIndex, e.Postings, &e.Date)
		a.addPayee(fileIndex, e.Payee)
		a.addPayeeTemplate(e)
		a.addCommentTags(fileIndex, &e.Date, e.Comment)
		for _, c := range e.HeaderComments {
			a.addCommentTags(fileIndex, &e.Date, c)
		}
		key := TxDuplicateKey(e)
		a.TransactionsByKey[key] = append(a.TransactionsByKey[key], e)
	case *ast.PeriodicTransaction:
		a.PeriodicTransactions = append(a.PeriodicTransactions, e)
		a.addPostings(fileIndex, e.Postings, nil)
		a.addCommentTags(fileIndex, nil, e.Comment)
		for _, c := range e.HeaderComments {
			a.addCommentTags(fileIndex, nil, c)
		}
	case *ast.AutomatedTransaction:
		a.AutomatedTransactions = append(a.AutomatedTransactions, e)
		a.addPostings(fileIndex, e.Postings, nil)
		a.addCommentTags(fileIndex, nil, e.Comment)
		for _, c := range e.HeaderComments {
			a.addCommentTags(fileIndex, nil, c)
		}
	case *ast.DefaultCommodityDirective:
		if e.Amount.Commodity != "" {
			mark := e.Amount.QuantityFmt.Decimal
			if mark != 0 {
				a.CommodityDecimalMarks[e.Amount.Commodity] = mark
			}
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
		if sd.Name == "alias" {
			a.AccountAliases[sd.Value] = aname
		}
	}
}

// addAliasDirective records a top-level "alias A = B" directive; the alias
// source name resolves to the target account.
func (a *Analysis) addAliasDirective(ad *ast.AliasDirective) {
	a.AccountAliases[ad.From.String()] = ad.To.String()
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

func (a *Analysis) addPostings(fileIndex int, postings []*ast.Posting, date *ast.Date) {
	for _, posting := range postings {
		aname := posting.Account.String()
		info, ok := a.Accounts[aname]
		if !ok {
			info = &AccountInfo{}
			a.Accounts[aname] = info
		}
		info.Usages = append(info.Usages, AccountUsage{
			FileIndex: fileIndex,
			Posting:   posting,
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

		// Collect decimal mark from amount formatting.
		if posting.Amount != nil && posting.Amount.Commodity != "" && posting.Amount.QuantityFmt.Decimal != 0 {
			if _, ok := a.CommodityDecimalMarks[posting.Amount.Commodity]; !ok {
				a.CommodityDecimalMarks[posting.Amount.Commodity] = posting.Amount.QuantityFmt.Decimal
			}
		}
	}
}

func (a *Analysis) addPayeeTemplate(tx *ast.Transaction) {
	payee := ""
	if tx.Payee != nil {
		payee = tx.Payee.Name
	}
	if payee == "" {
		return
	}

	templates := make([]PostingTemplate, len(tx.Postings))
	for i, p := range tx.Postings {
		t := PostingTemplate{
			Account:    p.Account.String(),
			IsInferred: p.Amount == nil,
		}
		if p.Amount != nil {
			t.Amount = p.Amount.Quantity.String()
			t.Commodity = p.Amount.Commodity
		}
		templates[i] = t
	}
	a.PayeeTemplates[payee] = templates
}

func (a *Analysis) collectDates() {
	seen := make(map[string]bool)
	for _, tx := range a.Transactions {
		s := formatDate(tx.Date)
		if s != "" && !seen[s] {
			seen[s] = true
			a.Dates = append(a.Dates, tx.Date)
			a.DateStrings = append(a.DateStrings, s)
		}
	}
	sort.Slice(a.Dates, func(i, j int) bool {
		return a.Dates[i].Compare(a.Dates[j]) < 0
	})
	sort.Strings(a.DateStrings)
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

func formatDate(d ast.Date) string {
	if d.Year == 0 && d.Month == 0 && d.Day == 0 {
		return ""
	}
	if d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > 31 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
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
