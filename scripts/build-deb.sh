#!/usr/bin/env bash
#
# Сборка .deb-пакета msqlite для текущей архитектуры (или указанной).
#
# Использование:
#   ./scripts/build-deb.sh [amd64|arm64|armv7]
#
# Результат: bin/msqlite_<версия>_<debian-арх>.deb
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

APP=msqlite
ARCH="${1:-amd64}"

case "$ARCH" in
amd64)
	GOARCH=amd64
	DEBARCH=amd64
	;;
arm64)
	GOARCH=arm64
	DEBARCH=arm64
	;;
armv7 | armhf)
	GOARCH=arm
	DEBARCH=armhf
	;;
*)
	echo "usage: $0 [amd64|arm64|armv7]" >&2
	exit 2
	;;
esac

VERSION="$(./scripts/version.sh version)"
COMMIT="$(./scripts/version.sh commit)"
BUILD_DATE="$(./scripts/version.sh date)"

OUTDIR="${OUTDIR:-$ROOT/bin}"
STAGE="$ROOT/build/deb/${APP}_${VERSION}_${DEBARCH}"

rm -rf "$STAGE"
mkdir -p "$STAGE/DEBIAN" "$STAGE/usr/bin" "$STAGE/usr/share/doc/$APP"

echo "Сборка $APP $VERSION для linux/$ARCH ($DEBARCH)…"
BUILD_ENV=(CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH")
if [ "$GOARCH" = arm ]; then
	BUILD_ENV+=(GOARM=7)
fi
env "${BUILD_ENV[@]}" go build -buildvcs=false -trimpath \
	-ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$BUILD_DATE" \
	-o "$STAGE/usr/bin/$APP" .

chmod 0755 "$STAGE/usr/bin/$APP"
install -m 0644 LICENSE "$STAGE/usr/share/doc/$APP/copyright"
gzip -9n -c README.md >"$STAGE/usr/share/doc/$APP/README.md.gz"
chmod 0644 "$STAGE/usr/share/doc/$APP/README.md.gz"

INSTALLED_SIZE="$(du -sk "$STAGE/usr" | cut -f1)"
cat >"$STAGE/DEBIAN/control" <<EOF
Package: $APP
Version: $VERSION
Architecture: $DEBARCH
Maintainer: Dmitry Gaenko <d.gaenko@gmail.com>
Installed-Size: $INSTALLED_SIZE
Section: utils
Priority: optional
Homepage: https://github.com/dgaenko/msqlite
Description: TUI SQLite manager in Midnight Commander / Far style
 msqlite is a terminal user interface for browsing and editing SQLite
 databases. It detects databases by their header, shows tables, views,
 schemas and data, supports editing rows and running SQL queries, and can
 import and export CSV and JSON.
 .
 The binary is static and has no runtime dependencies.
EOF
chmod 0644 "$STAGE/DEBIAN/control"

(
	cd "$STAGE"
	find usr -type f -printf '%p\0' | sort -z | xargs -0 md5sum >DEBIAN/md5sums
)
chmod 0644 "$STAGE/DEBIAN/md5sums"

mkdir -p "$OUTDIR"
DEB="$OUTDIR/${APP}_${VERSION}_${DEBARCH}.deb"
dpkg-deb --build --root-owner-group "$STAGE" "$DEB" >/dev/null

echo "Готово: $DEB"
dpkg-deb -f "$DEB" Package Version Architecture
