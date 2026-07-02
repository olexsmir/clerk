# clerk — plain-text accounting toolchain

Go toolkit for ledger/hledger journal files: parsing, linting, formatting, LSP, tags.

## Commands
```bash
go build -ldflags="-X 'main.version=dev'" .   # build
go test ./...                                 # all tests
go test ./internal/linter/ -run TestLinter -v # linter golden suite
```

## Conventions
- **Golden tests:** `testdata/<name>.input` (fixture) + `<name>.golden` (expected), use `internal/testutil/golden.Load/Assert`
- **semantic.Build(files):** files must be in include-order (includers after includes)

## Boundaries
- Use `internal/decimal.Decimal` for monetary amounts (no float64)
