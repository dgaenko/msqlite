#!/usr/bin/env bash
#
# Публикация текущей ветки на GitHub:
#  - ветка main очищается от AGENTS.md и scripts/publish-adeploy.sh;
#  - бинарники прикладываются к GitHub Release (в git не коммитятся);
#  - автор/коммиттер публичной истории обезличиваются.
#
# Требуется токен: GITHUB_TOKEN или $HOME/.config/.github.txt.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [ -n "$(git status --porcelain)" ]; then
	echo "sync-github: рабочее дерево грязное — сначала закоммитьте изменения" >&2
	exit 1
fi

REPO="${GITHUB_REPO:-dgaenko/msqlite}"
API="https://api.github.com/repos/${REPO}"
UPLOADS="https://uploads.github.com/repos/${REPO}"
PUBLIC_IDENT_NAME="${PUBLIC_IDENT_NAME:-Гаенко Дмитрий}"
PUBLIC_IDENT_EMAIL="${PUBLIC_IDENT_EMAIL:-noreply@msqlite.invalid}"

TOKEN="${GITHUB_TOKEN:-}"
if [ -z "$TOKEN" ] && [ -r "${HOME}/.config/.github.txt" ]; then
	TOKEN="$(tr -d '[:space:]' <"${HOME}/.config/.github.txt")"
fi
[ -n "$TOKEN" ] || {
	echo "sync-github: нет GITHUB_TOKEN и $HOME/.config/.github.txt" >&2
	exit 1
}

VERSION="$("${ROOT}/scripts/version.sh" version)"
TAG="v${VERSION//+/-}"

AMD64="${ROOT}/bin/msqlite"
ARM64="${ROOT}/bin/msqlite-linux-arm64"
ARMV7="${ROOT}/bin/msqlite-linux-armv7"
for f in "$AMD64" "$ARM64" "$ARMV7"; do
	[ -f "$f" ] || {
		echo "sync-github: нет ${f} (соберите: make build pi pi32)" >&2
		exit 1
	}
done

TMP="$(mktemp -d)"
IDX="$(mktemp)"
trap 'rm -rf "$TMP" "$IDX"' EXIT

cp "$AMD64" "$TMP/msqlite-linux-amd64"
cp "$ARM64" "$TMP/msqlite-linux-arm64"
cp "$ARMV7" "$TMP/msqlite-linux-armv7"
(
	cd "$TMP"
	sha256sum msqlite-linux-amd64 msqlite-linux-arm64 msqlite-linux-armv7 >SHA256SUMS
)

# Дерево публичной ветки: HEAD без внутренних файлов.
GIT_INDEX_FILE="$IDX" git read-tree HEAD
GIT_INDEX_FILE="$IDX" git rm --cached -q --ignore-unmatch AGENTS.md scripts/publish-adeploy.sh
TREE="$(GIT_INDEX_FILE="$IDX" git write-tree)"

PARENT="$(git rev-parse -q --verify refs/heads/public || true)"
if [ -n "$PARENT" ] && git log --format= --name-only "$PARENT" 2>/dev/null | grep -x 'AGENTS.md' >/dev/null; then
	# В старой публичной истории был AGENTS.md — начинаем чистую историю.
	PARENT=""
fi

export GIT_AUTHOR_NAME="$PUBLIC_IDENT_NAME"
export GIT_AUTHOR_EMAIL="$PUBLIC_IDENT_EMAIL"
export GIT_COMMITTER_NAME="$PUBLIC_IDENT_NAME"
export GIT_COMMITTER_EMAIL="$PUBLIC_IDENT_EMAIL"
if [ -n "$PARENT" ] && [ "$(git rev-parse "${PARENT}^{tree}")" = "$TREE" ]; then
	COMMIT="$PARENT"
else
	if [ -n "$PARENT" ]; then
		COMMIT="$(printf 'msqlite %s\n' "$VERSION" | git commit-tree "$TREE" -p "$PARENT")"
	else
		COMMIT="$(printf 'msqlite %s\n' "$VERSION" | git commit-tree "$TREE")"
	fi
fi
git update-ref refs/heads/public "$COMMIT"

echo "sync-github: ${VERSION} -> ${REPO}:main"
git push -f github refs/heads/public:refs/heads/main
git tag -f "$TAG" "$COMMIT" >/dev/null
git push -f github "refs/tags/${TAG}"

AUTH=(-H "Authorization: Bearer ${TOKEN}" -H "Accept: application/vnd.github+json")
CODE="$(curl -sS -o "${TMP}/release.json" -w '%{http_code}' "${AUTH[@]}" "${API}/releases/tags/${TAG}")"
REL_ID=""
case "$CODE" in
200)
	REL_ID="$(jq -r '.id' "${TMP}/release.json")"
	# Перезапись ассетов при повторной публикации той же версии.
	while read -r aid; do
		[ -n "$aid" ] || continue
		curl -fsS -X DELETE "${AUTH[@]}" "${API}/releases/assets/${aid}" >/dev/null
	done <<EOF
$(curl -fsS "${AUTH[@]}" "${API}/releases/${REL_ID}/assets?per_page=100" |
	jq -r '.[] | select(.name | test("^(msqlite-linux-(amd64|arm64|armv7)|SHA256SUMS)$")) | .id')
EOF
	;;
404)
	PAYLOAD="$(jq -n --arg tag "$TAG" --arg name "msqlite ${VERSION}" \
		--arg body "Сборка ${VERSION}. Бинарники: Linux amd64, arm64, armv7." \
		'{tag_name: $tag, name: $name, body: $body, draft: false, prerelease: false}')"
	REL_ID="$(curl -fsS -X POST "${AUTH[@]}" -H 'Content-Type: application/json' \
		-d "$PAYLOAD" "${API}/releases" | jq -r '.id')"
	;;
*)
	echo "sync-github: не удалось получить релиз ${TAG} (HTTP ${CODE})" >&2
	exit 1
	;;
esac
[ -n "$REL_ID" ] && [ "$REL_ID" != "null" ] || {
	echo "sync-github: не удалось создать релиз ${TAG}" >&2
	exit 1
}

for f in msqlite-linux-amd64 msqlite-linux-arm64 msqlite-linux-armv7 SHA256SUMS; do
	curl -fsS -X POST "${AUTH[@]}" -H 'Content-Type: application/octet-stream' \
		--data-binary "@${TMP}/${f}" "${UPLOADS}/releases/${REL_ID}/assets?name=${f}" >/dev/null
done
echo "sync-github: релиз ${TAG} опубликован (4 файла)"
