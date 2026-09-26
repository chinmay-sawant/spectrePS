#!/usr/bin/env bash
# check-traceability.sh checks the validation traceability table against the
# tests that are live in this tree.
#
# The table is sampledata/validation/traceability.tsv with four columns:
# section, case, test, and status.
#
#   live          the test column names a go test function on this tree
#   pending:X     another agent with scope X is writing that test; reported,
#                 not failed
#   proof         the proof is a command outside go test, such as a make
#                 target, a script, or a generator run
#   unowned       no test and no pending owner; reported for the integrator
#
# Exit 0 when every live row matches, 1 when a live row does not, and 2 when
# the table or the test list cannot be read.

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
table="$repo_root/sampledata/validation/traceability.tsv"

if [[ ! -f "$table" ]]; then
	echo "check-traceability: missing table: $table" >&2
	exit 2
fi

test_list=$(mktemp)
trap 'rm -f "$test_list"' EXIT

if ! output=$(cd "$repo_root" && go test -p 1 -list '.*' ./... 2>&1); then
	echo "check-traceability: go test -list failed:" >&2
	printf '%s\n' "$output" >&2
	exit 2
fi

printf '%s\n' "$output" | grep -E '^[A-Z][A-Za-z0-9_]*$' >"$test_list" || true
if [[ ! -s "$test_list" ]]; then
	echo "check-traceability: go test -list named no tests" >&2
	exit 2
fi

total=0
live_ok=0
pending=0
proofs=0
unowned=0
failures=0

while IFS=$'\t' read -r section case_id test_name status; do
	[[ -z "$section" ]] && continue
	[[ "$section" == '#'* ]] && continue
	[[ "$section" == section ]] && continue
	total=$((total + 1))
	case "$status" in
	live)
		if [[ -n "$test_name" ]] && grep -qxF "$test_name" "$test_list"; then
			printf 'ok      %s / %s -> %s\n' "$section" "$case_id" "$test_name"
			live_ok=$((live_ok + 1))
		else
			printf 'MISSING %s / %s -> %s (live row, not in go test -list)\n' \
				"$section" "$case_id" "$test_name"
			failures=$((failures + 1))
		fi
		;;
	pending:*)
		printf 'pending %s / %s -> %s (%s)\n' \
			"$section" "$case_id" "$test_name" "${status#pending:}"
		pending=$((pending + 1))
		if [[ -n "$test_name" ]] && grep -qxF "$test_name" "$test_list"; then
			printf '        note: %s is already live; change the status to live\n' "$test_name"
		fi
		;;
	proof)
		printf 'proof   %s / %s -> %s\n' "$section" "$case_id" "$test_name"
		proofs=$((proofs + 1))
		;;
	unowned)
		printf 'unowned %s / %s -> %s\n' "$section" "$case_id" "$test_name"
		unowned=$((unowned + 1))
		;;
	*)
		printf 'BAD_STATUS %s / %s: status %q is not live, pending:<scope>, proof, or unowned\n' \
			"$section" "$case_id" "$status"
		failures=$((failures + 1))
		;;
	esac
done <"$table"

printf '\n%d cases: %d live, %d pending, %d proof, %d unowned, %d failing\n' \
	"$total" "$live_ok" "$pending" "$proofs" "$unowned" "$failures"

if ((failures > 0)); then
	exit 1
fi
