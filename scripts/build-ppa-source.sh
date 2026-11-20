#!/usr/bin/env bash
#
# Подготовка source-пакета для PPA (Launchpad).
#
# Использование:
#   ./scripts/build-ppa-source.sh [серия]
#
# По умолчанию серия берётся из /etc/os-release (например, noble, resolute).
# Go-зависимости вендорятся в orig.tar.xz, чтобы сборка на Launchpad шла
# без сети (GOPROXY=off, -mod=vendor).
#
# Результат: build/ppa/src/msqlite_<версия>-<ревизия>~<серия>1_source.changes
#
# Подпись и загрузка:
#   GPG_KEY=<key-id> ./scripts/build-ppa-source.sh noble
#   dput ppa:dgaenko/msqlite build/ppa/src/msqlite_..._source.changes
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

APP=msqlite
SERIES="${1:-$(. /etc/os-release && printf '%s' "${VERSION_CODENAME:-}")}"
REV="${REV:-1}"
BASE_VERSION="$(tr -d '[:space:]' <VERSION)"
DEB_VERSION="${BASE_VERSION}-${REV}~${SERIES}1"

if [ -z "$SERIES" ]; then
	echo "не удалось определить серию Ubuntu; укажите её аргументом" >&2
	exit 2
fi

if ! command -v go >/dev/null 2>&1; then
	echo "не найден go (нужен для 'go mod vendor')" >&2
	exit 2
fi

BUILD="$ROOT/build/ppa"
UP="$BUILD/${APP}-${BASE_VERSION}"
ORIG="$BUILD/${APP}_${BASE_VERSION}.orig.tar.xz"
SRCROOT="$BUILD/src"
SRC="$SRCROOT/${APP}-${BASE_VERSION}"

rm -rf "$BUILD"
mkdir -p "$UP" "$SRCROOT"

echo "Подготовка upstream-дерева ${BASE_VERSION}…"
git archive --format=tar HEAD | tar -x -C "$UP"
rm -rf "$UP/debian"

echo "Вендоринг Go-зависимостей…"
(cd "$UP" && go mod vendor)

echo "Создание ${ORIG##*/}…"
tar -C "$BUILD" -cJf "$SRCROOT/${APP}_${BASE_VERSION}.orig.tar.xz" "${APP}-${BASE_VERSION}"
tar -C "$SRCROOT" -xf "$SRCROOT/${APP}_${BASE_VERSION}.orig.tar.xz"
cp -a "$ROOT/debian" "$SRC/debian"

cat >"$SRC/debian/changelog" <<EOF
${APP} (${DEB_VERSION}) ${SERIES}; urgency=medium

  * Release ${BASE_VERSION}.

 -- Dmitry Gaenko <d.gaenko@gmail.com>  $(date -R)

EOF

SIGN_ARGS=(-us -uc)
if [ -n "${GPG_KEY:-}" ]; then
	SIGN_ARGS=(-k"$GPG_KEY")
fi

echo "Сборка source-пакета ${DEB_VERSION}…"
(cd "$SRC" && dpkg-buildpackage -S -d "${SIGN_ARGS[@]}")

echo
echo "Готово: $SRCROOT/${APP}_${DEB_VERSION}_source.changes"
echo "Загрузка: dput ppa:dgaenko/msqlite $SRCROOT/${APP}_${DEB_VERSION}_source.changes"
