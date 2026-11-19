#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP="msqlite"
PREFIX="${PREFIX:-/usr/local}"
DEST=""
FORCE_ARCH=""
UNINSTALL=0
NO_BUILD=0

usage() {
	cat <<EOF
Установка $APP

Использование:
  sudo ./install.sh [опции]

Опции:
  --dest DIR       куда установить (по умолчанию: \$PREFIX/bin)
  --prefix DIR     префикс установки (по умолчанию: /usr/local)
  --arch ARCH      принудительно выбрать архитектуру: amd64, arm64, armv7
  --no-build       не пересобирать бинарник, даже если исходники новее
  --uninstall      удалить установленный бинарник и симлинк
  -h, --help       эта справка

Примеры:
  sudo ./install.sh
  ./install.sh --dest "\$HOME/bin"
  sudo ./install.sh --prefix /opt/msqlite
  ./install.sh --dest ./bin --arch arm64

После установки:
  <dest>/$APP-linux-<arch>   сам бинарник
  <dest>/$APP               симлинк на него
EOF
}

die() {
	echo "Ошибка: $*" >&2
	exit 1
}

while [ $# -gt 0 ]; do
	case "$1" in
	--dest)
		[ $# -ge 2 ] || die "для --dest нужен каталог"
		DEST="$2"
		shift 2
		;;
	--prefix)
		[ $# -ge 2 ] || die "для --prefix нужен каталог"
		PREFIX="$2"
		shift 2
		;;
	--arch)
		[ $# -ge 2 ] || die "для --arch нужна архитектура"
		FORCE_ARCH="$2"
		shift 2
		;;
	--no-build)
		NO_BUILD=1
		shift
		;;
	--uninstall)
		UNINSTALL=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "неизвестная опция: $1 (см. --help)"
		;;
	esac
done

BINDIR="${DEST:-$PREFIX/bin}"

[ "$(uname -s)" = "Linux" ] || die "поддерживается только Linux"

HOST_ARCH=""
case "$(uname -m)" in
aarch64 | arm64 | arm64v8)
	HOST_ARCH="arm64"
	;;
armv7l | armv7 | armv8l | armv6l)
	HOST_ARCH="armv7"
	;;
x86_64 | amd64)
	HOST_ARCH="amd64"
	;;
esac

if [ -n "$FORCE_ARCH" ]; then
	ARCH="$FORCE_ARCH"
else
	[ -n "$HOST_ARCH" ] || die "неподдерживаемая архитектура: $(uname -m) (доступны: amd64, arm64, armv7)"
	ARCH="$HOST_ARCH"
fi

TARGET="${APP}-linux-${ARCH}"
case "$ARCH" in
amd64)
	SRC="$ROOT/bin/msqlite"
	MAKE_TARGET="build-amd64"
	;;
arm64)
	SRC="$ROOT/bin/msqlite-linux-arm64"
	MAKE_TARGET="pi"
	;;
armv7)
	SRC="$ROOT/bin/msqlite-linux-armv7"
	MAKE_TARGET="pi32"
	;;
*)
	die "неизвестная архитектура: $ARCH (доступны: amd64, arm64, armv7)"
	;;
esac

pick_sudo() {
	if [ "$(id -u)" -eq 0 ]; then
		SUDO=""
		return
	fi
	if mkdir -p "$BINDIR" 2>/dev/null && [ -w "$BINDIR" ]; then
		SUDO=""
		return
	fi
	if command -v sudo >/dev/null 2>&1; then
		SUDO="sudo"
		return
	fi
	die "нет прав на запись в $BINDIR (запустите через sudo или укажите --dest)"
}

pick_sudo

if [ "$UNINSTALL" -eq 1 ]; then
	removed=""
	if [ -L "$BINDIR/$APP" ]; then
		link="$(readlink "$BINDIR/$APP" || true)"
		$SUDO rm -f "$BINDIR/$APP"
		removed="$BINDIR/$APP"
		base="$(basename "${link:-}")"
		case "$base" in
		msqlite-linux-*)
			$SUDO rm -f "$BINDIR/$base"
			removed="$removed, $BINDIR/$base"
			;;
		esac
	fi
	$SUDO rm -f "$BINDIR/$TARGET"
	removed="$removed, $BINDIR/$TARGET"
	echo "Удалено: $removed"
	exit 0
fi

needs_build() {
	[ -f "$SRC" ] || return 0
	local newer
	newer="$(find "$ROOT" -type f \
		\( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' -o -name 'VERSION' \
		-o -name 'Makefile' -o -path '*/scripts/version.sh' \) \
		-newer "$SRC" -print -quit 2>/dev/null || true)"
	[ -n "$newer" ]
}

if [ "$NO_BUILD" -eq 0 ] && needs_build; then
	echo "Исходники новее бинарника, пересобираю ($MAKE_TARGET)..."
	command -v make >/dev/null 2>&1 || die "нет make; соберите бинарники на машине разработчика или используйте --no-build"
	make -C "$ROOT" "$MAKE_TARGET"
fi

[ -f "$SRC" ] || die "не найден бинарник $SRC (соберите: make $MAKE_TARGET)"
if [ "$NO_BUILD" -eq 0 ] && needs_build; then
	die "бинарник $SRC не обновился после сборки"
fi
if [ "$NO_BUILD" -eq 1 ] && needs_build; then
	echo "Внимание: $SRC старше исходников, установка без пересборки (--no-build)." >&2
fi
[ -x "$SRC" ] || chmod +x "$SRC"
$SUDO mkdir -p "$BINDIR"

TMP="$BINDIR/.${TARGET}.tmp.$$"
cleanup() {
	$SUDO rm -f "$TMP" 2>/dev/null || true
}
trap cleanup EXIT

$SUDO install -m 0755 "$SRC" "$TMP"
$SUDO mv -f "$TMP" "$BINDIR/$TARGET"
$SUDO ln -sfn "$TARGET" "$BINDIR/$APP"

$SUDO cmp -s "$SRC" "$BINDIR/$TARGET" || die "установленный файл отличается от $SRC"
LINK="$(readlink "$BINDIR/$APP" || true)"
[ "$LINK" = "$TARGET" ] || die "симлинк $BINDIR/$APP указывает не на $TARGET"
SUM="$(sha256sum "$BINDIR/$TARGET" | awk '{print $1}')"

echo "Платформа: linux/$ARCH"
echo "Установлено: $BINDIR/$TARGET"
echo "Симлинк:    $BINDIR/$APP -> $LINK"
echo "SHA256:     $SUM"

if [ "$ARCH" = "$HOST_ARCH" ]; then
	SRC_VER="$("$SRC" -version 2>/dev/null | head -1 || true)"
	DST_VER="$("$BINDIR/$APP" -version 2>/dev/null | head -1 || true)"
	[ -n "$SRC_VER" ] && [ "$SRC_VER" = "$DST_VER" ] || die "проверка версии не прошла (src: ${SRC_VER:-?}, dst: ${DST_VER:-?})"
	echo "Проверка:   $DST_VER"
fi

case ":$PATH:" in
*":$BINDIR:"*) ;;
*) echo "Добавьте $BINDIR в PATH, если его там ещё нет." ;;
esac
