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

## Boundaries
- Use `internal/decimal.Decimal` for monetary amounts (no float64)
