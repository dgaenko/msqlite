#!/usr/bin/env bash
#
# Полный цикл выпуска:
#   1. коммит всех изменений и push в GitLab;
#   2. сборка amd64/arm64/armv7;
#   3. выкладка amd64/arm64 в пакет обновления adeploy;
#   4. публикация кода на GitHub (без AGENTS.md и publish-adeploy.sh)
#      и бинарников в GitHub Release.
#
# Сообщение коммита: MSG="..." make ship
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || {
	echo "ship: не Git-репозиторий" >&2
	exit 1
}

git add -A
if ! git diff --cached --quiet; then
	MSG="${MSG:-msqlite: update $(date -u +%Y-%m-%dT%H:%M:%SZ)}"
	git commit -m "$MSG"
fi
git push origin HEAD

make dist

"${ROOT}/scripts/sync-github.sh"
