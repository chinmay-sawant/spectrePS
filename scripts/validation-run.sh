#!/usr/bin/env bash
# Repo-local acceptance run over sampledata/validation for v0.0.4.
#
# It reads manifest.tsv, checks every present file's SHA-256 and byte count,
# runs each committed row with the command class its path and expect column
# select, and prints one aligned table plus the pass, fail, and skip counts.
# The two external rows are skipped when their file is absent and nothing is
# ever downloaded. A digest mismatch fails the row, names the file, and does
# not run it. The script never runs under make test or make lint.
#
# Usage: bash scripts/validation-run.sh
set -uo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
cd "$repo_root"

corpus_dir=sampledata/validation
manifest=$corpus_dir/manifest.tsv
bin_abs=$repo_root/bin/spectreps

if [ ! -f "$manifest" ]; then
	echo "validation-run: missing $manifest" >&2
	exit 1
fi

if [ ! -x "$bin_abs" ]; then
	echo "validation-run: $bin_abs is missing, building it"
	if ! go build -trimpath -o bin/spectreps ./cmd/spectreps; then
		echo "validation-run: go build failed" >&2
		exit 1
	fi
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/spectreps-validation.XXXXXX") || {
	echo "validation-run: cannot create a temporary directory" >&2
	exit 1
}
row_dir=$work/row
trap 'rm -rf "$work"' EXIT

# first_line prints the first non-blank line of a file, capped at 96 columns.
first_line() {
	local line
	line=$(grep -m1 -v '^[[:space:]]*$' "$1" 2>/dev/null || true)
	line=${line%$'\r'}
	printf '%s' "${line:0:96}"
}

# wrote_output reports whether the last run left a non-empty artifact.
wrote_output() {
	local file
	for file in "$row_dir"/out-*.ppm "$row_dir"/out.pdf; do
		if [ -f "$file" ] && [ -s "$file" ]; then
			return 0
		fi
	done
	return 1
}

# class_for prints the command class for one row: raster, text, gs, rewrite,
# or info. paint and refuse rows use the paint class; struct rows use info,
# rewrite, or text.
class_for() {
	local path=$1 expect=$2
	if [ "$expect" = struct ]; then
		case "$path" in
		rewrite/*)
			printf '%s' rewrite
			return
			;;
		text/*)
			printf '%s' text
			return
			;;
		*)
			printf '%s' info
			return
			;;
		esac
	fi
	case "$path" in
	text/*)
		printf '%s' text
		;;
	gs-argv/*.pdf)
		printf '%s' gs
		;;
	*)
		printf '%s' raster
		;;
	esac
}

# run_row runs one row's command and sets rc. stdout and stderr land in the
# row directory.
run_row() {
	local path=$1 class=$2
	case "$class" in
	raster)
		"$bin_abs" raster -o "$row_dir/out-%d.ppm" "$corpus_dir/$path" \
			>"$row_dir/stdout" 2>"$row_dir/stderr"
		;;
	text)
		"$bin_abs" text "$corpus_dir/$path" \
			>"$row_dir/stdout" 2>"$row_dir/stderr"
		;;
	gs)
		"$bin_abs" gs -sDEVICE=ppmraw -sOutputFile="$row_dir/out-%d.ppm" "$corpus_dir/$path" \
			>"$row_dir/stdout" 2>"$row_dir/stderr"
		;;
	rewrite)
		"$bin_abs" rewrite -level 2 -o "$row_dir/out.pdf" "$corpus_dir/$path" \
			>"$row_dir/stdout" 2>"$row_dir/stderr"
		;;
	info)
		"$bin_abs" info "$corpus_dir/$path" \
			>"$row_dir/stdout" 2>"$row_dir/stderr"
		;;
	esac
	rc=$?
}

pass=0
fail=0
skip=0
failed_list=""

# report prints one aligned row and counts the verdict.
report() {
	printf '%-48s %-32s %s\n' "$1" "$2" "$3"
	case "$4" in
	pass)
		pass=$((pass + 1))
		;;
	fail)
		fail=$((fail + 1))
		failed_list="$failed_list$1: $3
"
		;;
	skip)
		skip=$((skip + 1))
		;;
	esac
}

printf '%-48s %-32s %s\n' row expected measured
line=0
while IFS=$'\t' read -r path sha bytes source license feature expect || [ -n "${path:-}" ]; do
	line=$((line + 1))
	if [ "$line" -eq 1 ]; then
		if [ "$path" != path ] || [ "$expect" != expect ]; then
			echo "validation-run: $manifest: unexpected header" >&2
			exit 1
		fi
		continue
	fi
	expect=${expect%$'\r'}
	if [ -z "${path:-}" ] || [ -z "${sha:-}" ] || [ -z "${bytes:-}" ] || [ -z "${expect:-}" ]; then
		echo "validation-run: $manifest line $line: short row" >&2
		exit 1
	fi

	full=$corpus_dir/$path
	if [ ! -f "$full" ]; then
		if [ "${path#external/}" != "$path" ]; then
			report "$path" "$expect" 'skipped (external tier absent)' skip
		else
			report "$path" "$expect" 'missing file' fail
			echo "validation-run: $path: committed file is missing" >&2
		fi
		continue
	fi

	got_sha=$(sha256sum -- "$full" 2>/dev/null | cut -d' ' -f1)
	got_bytes=$(wc -c < "$full")
	got_bytes=${got_bytes// /}
	status=ok
	if [ "$got_sha" != "$sha" ]; then
		status="sha256 $got_sha (want $sha)"
	fi
	if [ "$got_bytes" != "$bytes" ]; then
		if [ "$status" = ok ]; then
			status="$got_bytes bytes (want $bytes)"
		else
			status="$status, $got_bytes bytes (want $bytes)"
		fi
	fi
	if [ "$status" != ok ]; then
		report "$path" "$expect" "$status" fail
		echo "validation-run: $path: digest mismatch: $status" >&2
		continue
	fi

	class=$(class_for "$path" "$expect")
	rm -rf "$row_dir"
	mkdir -p "$row_dir"
	run_row "$path" "$class"

	case "$expect" in
	paint | struct)
		if [ "$rc" -ne 0 ]; then
			report "$path" "$expect" "exit $rc: $(first_line "$row_dir/stderr")" fail
			continue
		fi
		case "$class" in
		raster | gs | rewrite)
			if wrote_output; then
				report "$path" "$expect" ok pass
			else
				report "$path" "$expect" 'exit 0 but no output' fail
			fi
			;;
		*)
			report "$path" "$expect" ok pass
			;;
		esac
		;;
	refuse:*)
		want=${expect#refuse:}
		if [ "$rc" -eq 0 ]; then
			report "$path" "$expect" 'exit 0 (blank success)' fail
		elif grep -qF -- "$want" "$row_dir/stderr"; then
			report "$path" "$expect" ok pass
		else
			report "$path" "$expect" "exit $rc: $(first_line "$row_dir/stderr")" fail
		fi
		;;
	*)
		report "$path" "$expect" 'unknown expect' fail
		;;
	esac
done < "$manifest"

rm -rf "$row_dir"
if [ "$fail" -gt 0 ]; then
	echo 'validation-run: failed rows:' >&2
	printf '%s' "$failed_list" | while IFS= read -r entry; do
		[ -n "$entry" ] && echo "  $entry" >&2
	done
fi
printf '%d pass, %d fail, %d skipped\n' "$pass" "$fail" "$skip"
if [ "$fail" -ne 0 ]; then
	exit 1
fi
exit 0
