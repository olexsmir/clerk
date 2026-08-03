# clerk — plain-text accounting toolchain

Go toolkit for ledger/hledger journal files: parsing, linting, formatting, LSP, tags.

## Commands
```bash
go build .                                    # build
go test ./...                                 # all tests
go test ./internal/linter/ -run TestLinter -v # linter golden suite
```

## Conventions
- **Golden tests:** `testdata/<name>.txtar`
- **Table tests:** `map[string]struct{...}` keyed by case name, iterated with `t.Run(tname, ...)`

## Boundaries
- Use `internal/decimal.Decimal` for monetary amounts (no float64)
