package semantic

import (
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
)

// Build constructs [Context] from a list of parsed files.
// Files should be in dependency order(includes before includers).
func Build(files []*journal.ParsedFile) *Context {
	c := &Context{
		Files:       files,
		Accounts:    make(map[string]*AccountInfo),
		Commodities: make(map[string]*CommodityInfo),
	}
	for i, pf := range files {
		for _, entry := range pf.Ast.Entries {
			switch e := entry.(type) {
			case *ast.AccountDirective:
				c.addAccountDirective(e)
			case *ast.CommodityDirective:
				c.addCommodityDirective(e)
			case *ast.Transaction:
				c.addPostings(i, e.Postings)
			case *ast.PeriodicTransaction:
				c.addPostings(i, e.Postings)
			case *ast.AutomatedTransaction:
				c.addPostings(i, e.Postings)
			}
		}
	}
	return c
}

func (c *Context) addAccountDirective(ad *ast.AccountDirective) {
	aname := ad.Account.String()
	info, ok := c.Accounts[aname]
	if !ok {
		info = &AccountInfo{}
		c.Accounts[aname] = info
	}
	info.Directives = append(info.Directives, ad)
}

func (c *Context) addCommodityDirective(cd *ast.CommodityDirective) {
	info, ok := c.Commodities[cd.Commodity]
	if !ok {
		info = &CommodityInfo{}
		c.Commodities[cd.Commodity] = info
	}
	info.Directives = append(info.Directives, cd)
}

func (c *Context) addPostings(fileIndex int, postings []*ast.Posting) {
	for _, posting := range postings {
		// Account tracking
		aname := posting.Account.String()
		info, ok := c.Accounts[aname]
		if !ok {
			info = &AccountInfo{}
			c.Accounts[aname] = info
		}
		info.Usages = append(info.Usages, AccountUsage{
			FileIndex: fileIndex,
			Posting:   posting,
		})

		// Commodity tracking
		c.addCommodityUsage(fileIndex, posting.Amount)
		if posting.Cost != nil {
			c.addCommodityUsage(fileIndex, &posting.Cost.Amount)
		}
		if posting.Balance != nil {
			c.addCommodityUsage(fileIndex, &posting.Balance.Amount)
			if posting.Balance.Cost != nil {
				c.addCommodityUsage(fileIndex, &posting.Balance.Cost.Amount)
			}
		}
	}
}

func (c *Context) addCommodityUsage(fileIndex int, am *ast.Amount) {
	if am == nil || am.Commodity == "" {
		return
	}
	info, ok := c.Commodities[am.Commodity]
	if !ok {
		info = &CommodityInfo{}
		c.Commodities[am.Commodity] = info
	}
	info.Usages = append(info.Usages, CommodityUsage{
		FileIndex: fileIndex,
		Amount:    am,
	})
}
