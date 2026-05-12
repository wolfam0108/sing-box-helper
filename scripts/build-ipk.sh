#!/bin/sh
# Build a singbox-helper .ipk from a pre-built binary.
#
# The .ipk format used here is the "tar.gz of three files" variant
# (debian-binary + control.tar.gz + data.tar.gz) which Entware's opkg
# accepts everywhere - no `ar` required, so this script runs in
# git-bash on Windows, plain bash, dash, busybox sh and Linux CI.
#
# Required environment variables:
#   BINARY  : path to the compiled singbox-helper binary
#   PKGVER  : package version, e.g. 1.0.0 (no leading 'v')
#   PKGARCH : opkg architecture string, e.g. mipsel-3.4 / aarch64-3.10
#
# Optional:
#   OUTDIR  : where to put the .ipk (default: ./dist)
#
# Example:
#   BINARY=./singbox-helper-mipsle PKGVER=1.0.0 PKGARCH=mipsel-3.4 \
#     ./scripts/build-ipk.sh

set -eu

: "${BINARY:?BINARY (path to compiled singbox-helper) is required}"
: "${PKGVER:?PKGVER (package version, e.g. 1.0.0) is required}"
: "${PKGARCH:?PKGARCH (e.g. mipsel-3.4) is required}"
OUTDIR=${OUTDIR:-./dist}

if [ ! -f "$BINARY" ]; then
    echo "build-ipk: BINARY '$BINARY' not found" >&2
    exit 1
fi

# Repository-relative paths to package metadata.
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
PKG_DIR=$SCRIPT_DIR/../packages/keenetic
CONTROL_TEMPLATE=$PKG_DIR/control.template
POSTINST=$PKG_DIR/postinst
PRERM=$PKG_DIR/prerm
INIT_SCRIPT=$PKG_DIR/files/opt/etc/init.d/S98singbox-helper

for f in "$CONTROL_TEMPLATE" "$POSTINST" "$PRERM" "$INIT_SCRIPT"; do
    [ -f "$f" ] || { echo "build-ipk: missing $f" >&2; exit 1; }
done

# All work happens in a tempdir; layout matches the on-disk tree.
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT INT TERM

mkdir -p "$WORK/data/opt/bin"
mkdir -p "$WORK/data/opt/etc/init.d"
mkdir -p "$WORK/data/opt/etc/singbox-helper"
mkdir -p "$WORK/control"

cp "$BINARY"      "$WORK/data/opt/bin/singbox-helper"
cp "$INIT_SCRIPT" "$WORK/data/opt/etc/init.d/S98singbox-helper"
chmod 0755 "$WORK/data/opt/bin/singbox-helper"
chmod 0755 "$WORK/data/opt/etc/init.d/S98singbox-helper"

# Substitute __PKGVER__ / __PKGARCH__ in the control template.
sed -e "s/__PKGVER__/$PKGVER/g" -e "s/__PKGARCH__/$PKGARCH/g" \
    "$CONTROL_TEMPLATE" > "$WORK/control/control"
cp "$POSTINST" "$WORK/control/postinst"
cp "$PRERM"    "$WORK/control/prerm"
chmod 0755 "$WORK/control/postinst" "$WORK/control/prerm"

# Inner tarballs. --owner/--group=0 so the .ipk is reproducible across
# users and avoids leaking developer UID into the package. --sort=name
# would help reproducibility further but isn't in busybox tar, skipping.
(cd "$WORK/data"    && tar --owner=0 --group=0 -czf "$WORK/data.tar.gz"    .)
(cd "$WORK/control" && tar --owner=0 --group=0 -czf "$WORK/control.tar.gz" .)
printf '2.0\n' > "$WORK/debian-binary"

# Outer .ipk = tar.gz of the three files. Entware's opkg supports this
# in addition to the `ar` variant, so we use it for portability.
OUT_NAME="singbox-helper_${PKGVER}_${PKGARCH}.ipk"
mkdir -p "$OUTDIR"
OUT_PATH=$(cd "$OUTDIR" && pwd)/$OUT_NAME
(cd "$WORK" && tar --owner=0 --group=0 -czf "$OUT_PATH" \
    debian-binary control.tar.gz data.tar.gz)

echo "Built $OUT_PATH"
ls -lh "$OUT_PATH"
