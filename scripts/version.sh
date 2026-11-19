#!/usr/bin/env bash
#
# Единственный источник версии msqlite.
#
# Версия всегда корректна по semver 2.0.0:
#   - точный чистый Git-тег v1.2.3          -> 1.2.3
#   - коммиты после тега                    -> 1.2.3+git.N.gHASH
#   - незакоммиченные изменения             -> ...+HASH.dirty или ....dirty
#   - без тегов                             -> VERSION+git.N.gHASH
#   - без Git                               -> VERSION+build.HASH
#
# Идентификаторы в build-metadata состоят из [0-9A-Za-z-], поэтому
# полученная строка остаётся валидным semver (в отличие от прежних
# вариантов вида 1.0.0.dirty или 1.0.0-2-gabc).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_FILE="$ROOT/VERSION"

# semver 2.0.0: MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
SEMVER_RE='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
CORE_RE='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

FIELD="${1:-full}"
COMMIT="none"
TAG=""
VERSION=""

# Убирает префикс v и build-metadata, оставляя core + prerelease.
normalize() {
	local v="${1#v}"
	printf '%s' "${v%%+*}"
}

BASE="$(tr -d '[:space:]' <"$BASE_FILE" 2>/dev/null || true)"
BASE="$(normalize "${BASE:-0.0.0}")"
if ! [[ "$BASE" =~ $CORE_RE ]]; then
	echo "version.sh: VERSION не является корректным semver: ${BASE}" >&2
	BASE="0.0.0"
fi

if command -v git >/dev/null 2>&1 &&
	git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1 &&
	git -C "$ROOT" rev-parse --verify HEAD >/dev/null 2>&1; then
	COMMIT="$(git -C "$ROOT" rev-parse --short HEAD)"

	META_DIRTY=""
	if [ -n "$(git -C "$ROOT" status --porcelain 2>/dev/null)" ]; then
		META_DIRTY=".dirty"
	fi

	TAG="$(git -C "$ROOT" describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
	TAG_BASE="$(normalize "${TAG:-}")"
	if [ -n "$TAG" ] && [[ "$TAG_BASE" =~ $CORE_RE ]]; then
		DISTANCE="$(git -C "$ROOT" rev-list --count "${TAG}..HEAD" 2>/dev/null || echo 0)"
		if [ "$DISTANCE" -eq 0 ]; then
			if [ -n "$META_DIRTY" ]; then
				VERSION="${TAG_BASE}+${COMMIT}${META_DIRTY}"
			else
				VERSION="$TAG_BASE"
			fi
		else
			VERSION="${TAG_BASE}+git.${DISTANCE}.g${COMMIT}${META_DIRTY}"
		fi
	fi

	if [ -z "$VERSION" ]; then
		COUNT="$(git -C "$ROOT" rev-list --count HEAD 2>/dev/null || echo 0)"
		VERSION="${BASE}+git.${COUNT}.g${COMMIT}${META_DIRTY}"
	fi
else
	TAG=""
	SOURCE_HASH="$(
		cd "$ROOT"
		if command -v sha256sum >/dev/null 2>&1; then
			find . -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) \
				-not -path './bin/*' -not -path './.git/*' -print0 |
				sort -z | xargs -0 sha256sum | sha256sum | cut -c1-8
		else
			date +%s | cksum | awk '{print $1}'
		fi
	)"
	VERSION="${BASE}+build.${SOURCE_HASH}"
fi

if [ -n "${MSQLITE_VERSION:-}" ]; then
	if [[ "$MSQLITE_VERSION" =~ $SEMVER_RE ]]; then
		VERSION="$MSQLITE_VERSION"
	else
		echo "version.sh: MSQLITE_VERSION не является корректным semver: ${MSQLITE_VERSION}" >&2
		exit 2
	fi
fi

if ! [[ "$VERSION" =~ $SEMVER_RE ]]; then
	echo "version.sh: вычислена некорректная semver-версия: ${VERSION}" >&2
	exit 2
fi

case "$FIELD" in
version)
	echo "$VERSION"
	;;
commit)
	echo "$COMMIT"
	;;
date)
	date -u +%Y-%m-%dT%H:%M:%SZ
	;;
tag)
	echo "$TAG"
	;;
full)
	echo "$VERSION"
	echo "commit: $COMMIT"
	echo "built:  $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	echo "base:   $BASE"
	;;
*)
	echo "usage: $0 [version|commit|date|tag|full]" >&2
	exit 2
	;;
esac
