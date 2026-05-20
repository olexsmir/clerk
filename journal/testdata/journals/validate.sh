#!/bin/bash

cd "$(dirname "$0")/../.."

has_ledger=0
has_hledger=0
command -v ledger &>/dev/null && has_ledger=1
command -v hledger &>/dev/null && has_hledger=1

[[ $has_ledger -eq 0 && $has_hledger -eq 0 ]] && {
	echo "Neither ledger nor hledger installed"
	exit 1
}

passed=0
failed=0

for f in tests/journal/*; do
	[[ -f "$f" ]] || continue
	name=$(basename "$f")

	[[ "$name" == broken-* ]] && continue
	[[ "$name" == "actual-1ktxns-100accts.journal" ]] && continue
	[[ "$name" == *.sh ]] && continue

	if [[ $has_ledger -eq 1 ]]; then
		if timeout 2 ledger -f "$f" print >/dev/null 2>&1; then
			echo "LEDGER OK   $name"
			passed=$((passed + 1))
		else
			echo "LEDGER FAIL $name"
			failed=$((failed + 1))
		fi
	fi

	# hledger only for .dat files
	if [[ $has_hledger -eq 1 && "$name" == *.dat ]]; then
		if timeout 2 hledger -f "$f" print >/dev/null 2>&1; then
			echo "HLEDGER OK  $name"
		else
			echo "HLEDGER FAIL $name"
		fi
	fi
done

echo ""
echo "Summary: $passed passed, $failed failed"
[[ $failed -eq 0 ]] && exit 0 || exit 1
