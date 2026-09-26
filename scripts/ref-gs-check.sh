#!/usr/bin/env bash
# Ghostscript reference proofs for the v0.0.4 validation phase 11.
#
# A proof tool, never a build or run-time dependency. It is not called by
# make test or make lint. The script resolves gs at ./ghostscript/bin/gs and
# then on PATH, normalizes both PPMs to their pixel bodies, requires exact
# equality for the axis-aligned cases, and prints the recorded byte-diff count
# for the diagonal and curve cases. It also reopens a level 2 rewrite with gs
# and runs qpdf or pdfcpu when one is installed.
#
# Usage: bash scripts/ref-gs-check.sh
set -euo pipefail

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
cd "$repo_root"

gs_bin=""
if [ -x ./ghostscript/bin/gs ]; then
	gs_bin=./ghostscript/bin/gs
elif command -v gs >/dev/null 2>&1; then
	gs_bin=$(command -v gs)
fi
if [ -z "$gs_bin" ]; then
	echo 'refs-gs-check: gs not installed, skipping'
	exit 0
fi

bin=bin/spectreps
if [ ! -x "$bin" ]; then
	go build -trimpath -o "$bin" ./cmd/spectreps
fi

out_dir=references
mkdir -p "$out_dir"

# body_of writes the pixel body of one PPM after the `255` max-value line.
# gs adds a comment line the Spectre writer does not, so both sides normalize
# through the same rule.
body_of() {
	python3 - "$1" "$2" <<'PY'
import sys

data = open(sys.argv[1], "rb").read()
marker = b"\n255\n"
at = data.find(marker)
if at < 0:
    sys.exit("no maxval line in " + sys.argv[1])
open(sys.argv[2], "wb").write(data[at + len(marker):])
PY
}

# diff_count prints the number of differing body bytes and their byte length.
diff_count() {
	python3 - "$1" "$2" <<'PY'
import sys

a = open(sys.argv[1], "rb").read()
b = open(sys.argv[2], "rb").read()
if len(a) != len(b):
    print(f"length {len(a)} against {len(b)}")
    sys.exit(0)
diffs = sum(1 for x, y in zip(a, b) if x != y)
print(f"{diffs} of {len(a)}")
PY
}

render_spectre() {
	"$bin" raster -w 612 -h 792 -r 72 -o "$2" "$1" >/dev/null
}

# render_gs_pdf lets the PDF /MediaBox set the geometry, the phase 11 rule.
render_gs_pdf() {
	"$gs_bin" -q -dNOPAUSE -dBATCH -dSAFER -sDEVICE=ppmraw -sOutputFile="$2" "$1" >/dev/null
}

# render_gs_ps pins the letter geometry, because a PostScript program with no
# setpagedevice gets the gs default page (A4 on this build) while Spectre
# defaults to 612 by 792.
render_gs_ps() {
	"$gs_bin" -q -dNOPAUSE -dBATCH -dSAFER -sDEVICE=ppmraw -g612x792 -sOutputFile="$2" "$1" >/dev/null
}

echo "refs-gs-check: gs is $("$gs_bin" --version)"
printf '%-22s %s\n' 'case' 'verdict'
failed=0
for name in line rect diag curve; do
	prog="sampledata/validation/refs/$name.ps"
	spectre_ppm="$out_dir/$name.spectre.ppm"
	gs_ppm="$out_dir/$name.gs.ppm"
	render_spectre "$prog" "$spectre_ppm"
	render_gs_ps "$prog" "$gs_ppm"
	body_of "$spectre_ppm" "$out_dir/$name.spectre.body"
	body_of "$gs_ppm" "$out_dir/$name.gs.body"
	case $name in
	line | rect)
		if cmp -s "$out_dir/$name.spectre.body" "$out_dir/$name.gs.body"; then
			printf '%-22s %s\n' "$name.ps" 'exact'
		else
			printf '%-22s %s\n' "$name.ps" "MISMATCH $(diff_count "$out_dir/$name.spectre.body" "$out_dir/$name.gs.body")"
			failed=1
		fi
		;;
	*)
		printf '%-22s %s\n' "$name.ps" "diff $(diff_count "$out_dir/$name.spectre.body" "$out_dir/$name.gs.body")"
		;;
	esac
done

echo 'refs-gs-check: rewrite cross-check'
for src in sampledata/compress/path.pdf sampledata/validation/paths/path.pdf; do
	base=$(basename "$(dirname "$src")")-$(basename "$src" .pdf)
	rewrite="$out_dir/$base.level2.pdf"
	"$bin" rewrite -level 2 -o "$rewrite" "$src" >/dev/null
	src_ppm="$out_dir/$base.src.ppm"
	rw_ppm="$out_dir/$base.l2.ppm"
	render_gs_pdf "$src" "$src_ppm"
	render_gs_pdf "$rewrite" "$rw_ppm"
	body_of "$src_ppm" "$out_dir/$base.src.body"
	body_of "$rw_ppm" "$out_dir/$base.l2.body"
	printf '%-22s %s\n' "$base" "diff $(diff_count "$out_dir/$base.src.body" "$out_dir/$base.l2.body")"
done

if command -v qpdf >/dev/null 2>&1; then
	qpdf --check "$out_dir/compress-path.level2.pdf"
elif command -v pdfcpu >/dev/null 2>&1; then
	pdfcpu validate "$out_dir/compress-path.level2.pdf"
else
	echo 'refs-gs-check: qpdf and pdfcpu not installed, skipping the validation step'
fi

if [ "$failed" -ne 0 ]; then
	echo 'refs-gs-check: an axis-aligned case did not match' >&2
	exit 1
fi
echo 'refs-gs-check: axis-aligned cases are exact; diffs recorded under references/'
