#!/usr/bin/env bash
# Builds secnote for every supported platform in one go.
set -euo pipefail

usage() {
	cat <<'EOF'
使い方: ./build.sh [OS/ARCH ...]

ターゲットを省略すると、次のすべてをビルドします。
  linux/amd64  linux/arm64  windows/amd64  windows/arm64  darwin/amd64  darwin/arm64

例:
  ./build.sh                            すべてのターゲット
  ./build.sh windows/amd64 linux/arm64  指定したターゲットだけ
  VERSION=1.2.0 ./build.sh              バージョンを指定してビルド

環境変数:
  VERSION  バイナリに埋め込むバージョン (既定: cmd/secnote/main.go に書かれた値)
  OUT      出力先ディレクトリ (既定: dist)

指定できる OS/ARCH の一覧は `go tool dist list` で確認できます。
EOF
}

case "${1:-}" in
-h | --help)
	usage
	exit 0
	;;
esac

cd "$(dirname "$0")"

if ! command -v go >/dev/null 2>&1; then
	echo "build.sh: go コマンドが見つかりません" >&2
	exit 1
fi

if [ $# -eq 0 ]; then
	set -- linux/amd64 linux/arm64 windows/amd64 windows/arm64 darwin/amd64 darwin/arm64
fi
for target in "$@"; do
	case "$target" in
	?*/?*) ;;
	*)
		echo "build.sh: ターゲットは OS/ARCH の形で指定してください: $target" >&2
		exit 2
		;;
	esac
done

out=${OUT:-dist}
ldflags="-s -w"
if [ -n "${VERSION:-}" ]; then
	version=$VERSION
	ldflags="$ldflags -X main.version=$version"
else
	# Ask the program itself, so main.go stays the only place the version
	# is written down.
	version=$(go run ./cmd/secnote -version | awk '{print $2}')
fi

mkdir -p "$out"
built=()
for target in "$@"; do
	os=${target%/*}
	arch=${target#*/}
	name="secnote-$version-$os-$arch"
	if [ "$os" = windows ]; then
		name="$name.exe"
	fi
	printf '%-15s -> %s\n' "$target" "$out/$name"
	# cgo stays off so that every target builds from any host without a
	# cross compiler.
	GOOS=$os GOARCH=$arch CGO_ENABLED=0 \
		go build -trimpath -ldflags "$ldflags" -o "$out/$name" ./cmd/secnote
	built+=("$name")
done

# Checksums for the files built in this run, in the format `sha256sum -c`
# reads.
if command -v sha256sum >/dev/null 2>&1; then
	(cd "$out" && sha256sum "${built[@]}" >SHA256SUMS)
elif command -v shasum >/dev/null 2>&1; then
	(cd "$out" && shasum -a 256 "${built[@]}" >SHA256SUMS)
else
	echo "build.sh: sha256sum も shasum も無いため SHA256SUMS は作成しません" >&2
fi

echo "${#built[@]} 個のバイナリを $out/ に出力しました (バージョン $version)"
