#!/bin/sh
#
# Собирает текст лицензий вендоренных Go-зависимостей в один файл.
#
# Использование: collect-licenses.sh <output-file>
set -eu

OUT="${1:?usage: $0 <output-file>}"
: >"$OUT"

find vendor -type f \( -iname 'LICENSE*' -o -iname 'COPYING*' -o -iname 'NOTICE*' \) |
	sort |
	while IFS= read -r file; do
		printf '=== %s ===\n\n' "$file" >>"$OUT"
		cat "$file" >>"$OUT"
		printf '\n' >>"$OUT"
	done
