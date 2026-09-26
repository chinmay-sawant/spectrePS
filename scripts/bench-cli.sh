#!/usr/bin/env bash
# bench-cli.sh measures the built binary and writes profiles/cli.txt.
#
# The fixed list covers the application surface: version, raster of the
# checked-in path PDF at 72 and 300 dpi on the default 612 by 792 point page,
# raster of the synthetic stroke program, rewrite of the checked-in image PDF
# at levels 2 through 5, text, pdfimage, and validate.
#
# The scaling section rasterizes the stroke program and the path PDF at 72,
# 150, 300, 600, and 1200 dpi with -w 200 -h 200, so every cell has the same
# 200 by 200 point page and the 1200 dpi cell stays under the 40 million pixel
# cap. hyperfine is used when it is on PATH; otherwise GNU time reports seconds
# and peak RSS. Both write profiles/cli.txt, and profiles/ is gitignored.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

make build

out=profiles/cli
mkdir -p "$out"

stroke=$out/stroke.ps
printf '%s\n' '0 1 1999 { /y exch def 0 y moveto 200 y lineto 1 setlinewidth stroke } for' > "$stroke"
printf '%s\n' '1 2 add showpage' > "$out/small.ps"

path_pdf=sampledata/compress/path.pdf
what_pdf=sampledata/compress/whatisthis.pdf
text_pdf=sampledata/fixtures/text.pdf

labels=(
  version
  raster-path-72-default
  raster-path-300-default
  rewrite-whatisthis-l2
  rewrite-whatisthis-l3
  rewrite-whatisthis-l4
  rewrite-whatisthis-l5
  text
  pdfimage
  validate-ps
  validate-path
  raster-path-72
  raster-path-150
  raster-path-300
  raster-path-600
  raster-path-1200
  raster-stroke-72
  raster-stroke-150
  raster-stroke-300
  raster-stroke-600
  raster-stroke-1200
)

cmds=(
  "bin/spectreps version"
  "bin/spectreps raster -r 72 -o $out/path-72-default.ppm $path_pdf"
  "bin/spectreps raster -r 300 -o $out/path-300-default.ppm $path_pdf"
  "bin/spectreps rewrite -level 2 -o $out/whatisthis-l2.pdf $what_pdf"
  "bin/spectreps rewrite -level 3 -o $out/whatisthis-l3.pdf $what_pdf"
  "bin/spectreps rewrite -level 4 -o $out/whatisthis-l4.pdf $what_pdf"
  "bin/spectreps rewrite -level 5 -o $out/whatisthis-l5.pdf $what_pdf"
  "bin/spectreps text $text_pdf"
  "bin/spectreps pdfimage -w 200 -h 200 -o $out/stroke.pdf $stroke"
  "bin/spectreps validate $out/small.ps"
  "bin/spectreps validate $path_pdf"
  "bin/spectreps raster -w 200 -h 200 -r 72 -o $out/path-72.ppm $path_pdf"
  "bin/spectreps raster -w 200 -h 200 -r 150 -o $out/path-150.ppm $path_pdf"
  "bin/spectreps raster -w 200 -h 200 -r 300 -o $out/path-300.ppm $path_pdf"
  "bin/spectreps raster -w 200 -h 200 -r 600 -o $out/path-600.ppm $path_pdf"
  "bin/spectreps raster -w 200 -h 200 -r 1200 -o $out/path-1200.ppm $path_pdf"
  "bin/spectreps raster -w 200 -h 200 -r 72 -o $out/stroke-72.ppm $stroke"
  "bin/spectreps raster -w 200 -h 200 -r 150 -o $out/stroke-150.ppm $stroke"
  "bin/spectreps raster -w 200 -h 200 -r 300 -o $out/stroke-300.ppm $stroke"
  "bin/spectreps raster -w 200 -h 200 -r 600 -o $out/stroke-600.ppm $stroke"
  "bin/spectreps raster -w 200 -h 200 -r 1200 -o $out/stroke-1200.ppm $stroke"
)

report=profiles/cli.txt

if command -v hyperfine >/dev/null 2>&1; then
  {
    printf 'hyperfine %s, warmup 1, runs 3\n' "$(hyperfine --version)"
    hyperfine --warmup 1 --runs 3 --style basic "${cmds[@]}"
  } > "$report"
  printf '%s\n' "bench-cli: wrote $report with hyperfine"
  exit 0
fi

if [ ! -x /usr/bin/time ]; then
  printf '%s\n' 'bench-cli: hyperfine and /usr/bin/time are both absent, cannot measure'
  exit 1
fi

reps=10
printf 'GNU time -f %%e %%M, %s runs per command, min / median / max seconds\n' "$reps" > "$report"
for i in "${!labels[@]}"; do
  raw=$(mktemp)
  for _ in $(seq "$reps"); do
    /usr/bin/time -a -o "$raw" -f '%e %M' \
      /bin/sh -c "${cmds[$i]}" >/dev/null 2>/dev/null
  done
  stats=$(sort -n "$raw" | awk '
    NR == 1 { min = $1 }
    NR == 5 { a = $1 }
    NR == 6 { b = $1 }
    { if ($1 > max) max = $1 }
    END { printf "%.3f\t%.3f\t%.3f", min, (a + b) / 2, max }
  ' -)
  rss=$(awk '{ if ($2 > max) max = $2 } END { print max }' "$raw")
  printf '%s\t%s\t%s\n' "${labels[$i]}" "$stats" "$rss" >> "$report"
  rm -f "$raw"
done

# Render the scaling cells with their pixel counts so a reader can check
# linearity without recomputing the page geometry.
{
  printf '\nscaling table, page 200 by 200 points\n'
  printf '%-10s %5s %10s %9s %9s %9s %14s\n' input dpi pixels min-s median-s max-s "peak-rss-kb"
  for input in path stroke; do
    for dpi in 72 150 300 600 1200; do
      label=raster-$input-$dpi
      row=$(awk -F '\t' -v want="$label" '$1 == want { print $2 "\t" $3 "\t" $4 "\t" $5 }' "$report")
      low=$(printf '%s' "$row" | cut -f1)
      median=$(printf '%s' "$row" | cut -f2)
      high=$(printf '%s' "$row" | cut -f3)
      rss=$(printf '%s' "$row" | cut -f4)
      pixels=$(awk -v d="$dpi" 'BEGIN { side = 200 * d / 72; side = int(side + 0.5); print side * side }')
      printf '%-10s %5s %10s %9s %9s %9s %14s\n' \
        "$input" "$dpi" "$pixels" "$low" "$median" "$high" "$rss"
    done
  done
} >> "$report"

printf '%s\n' "bench-cli: wrote $report"
