#!/usr/bin/env bash
# Build Niraluli release artifacts: source.zip (with Go), x86_64 RPM, and amd64 deb.
# Usage (from repo root): ./tools/package-release.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.81}"
RELEASE="${RELEASE:-1}"
GO_VERSION="${GO_VERSION:-1.22.12}"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"

DIST="$ROOT/dist"
STAGE="$DIST/stage/niraluli-${VERSION}"
BUILDROOT="$DIST/pkg-buildroot"
DEBROOT="$DIST/deb-root"
ZIP_NAME="niraluli-${VERSION}-source.zip"
RPM_NAME="niraluli-${VERSION}-${RELEASE}.x86_64.rpm"
DEB_NAME="niraluli_${VERSION}-${RELEASE}_amd64.deb"

echo "==> Niraluli ${VERSION} release packaging"

rm -rf "$DIST/stage" "$BUILDROOT" "$DEBROOT"
mkdir -p "$STAGE" "$DIST"

# --- Portable Go -----------------------------------------------------------
ensure_go() {
  if [[ -x "$ROOT/.tools/go/bin/go" ]]; then
    local v
    v="$("$ROOT/.tools/go/bin/go" version 2>/dev/null || true)"
    echo "using existing .tools/go ($v)"
    return 0
  fi
  echo "downloading ${GO_TARBALL}"
  mkdir -p "$ROOT/.tools"
  local tmp
  tmp="$(mktemp -d)"
  curl -fsSL "$GO_URL" -o "$tmp/$GO_TARBALL"
  tar -C "$tmp" -xzf "$tmp/$GO_TARBALL"
  rm -rf "$ROOT/.tools/go"
  mv "$tmp/go" "$ROOT/.tools/go"
  rm -rf "$tmp"
  echo "installed .tools/go ($("$ROOT/.tools/go/bin/go" version))"
}

ensure_go
export PATH="$ROOT/.tools/go/bin:$PATH"
export GOROOT="$ROOT/.tools/go"

# --- Clean source tree via git archive -------------------------------------
echo "==> staging source tree"
git archive --format=tar HEAD | tar -C "$STAGE" -xf -

# Overlay portable Go (gitignored; not in archive)
mkdir -p "$STAGE/.tools"
rm -rf "$STAGE/.tools/go"
cp -a "$ROOT/.tools/go" "$STAGE/.tools/go"

# Drop scratch demos if they ever appear in the tree
rm -f "$STAGE/demo.txt" "$STAGE/demo2.txt"

# --- Source zip ------------------------------------------------------------
echo "==> writing $DIST/$ZIP_NAME"
rm -f "$DIST/$ZIP_NAME"
(
  cd "$DIST/stage"
  zip -qr "$DIST/$ZIP_NAME" "niraluli-${VERSION}"
)

# --- Build uli into the staged tree ----------------------------------------
echo "==> building bin/uli"
mkdir -p "$STAGE/bin"
(
  cd "$STAGE"
  export PATH="$STAGE/.tools/go/bin:$PATH"
  export GOROOT="$STAGE/.tools/go"
  go build -o bin/uli ./cmd/uli
)

# --- Shared install layout -------------------------------------------------
echo "==> assembling package layout"
mkdir -p "$BUILDROOT/opt" "$BUILDROOT/usr/bin"
cp -a "$STAGE" "$BUILDROOT/opt/niraluli"
install -m 755 "$ROOT/packaging/uli.wrapper" "$BUILDROOT/usr/bin/uli"

SUMMARY="Niraluli programming language with bundled Go toolchain"
DESCRIPTION="Niraluli (நிரலுளி) is a Go-inspired programming language with semantic Tamil keywords and a C/Linux backend. This package installs the compiler tree, stdlib, docs, corpus samples, a portable Go 1.22 toolchain, and a prebuilt /opt/niraluli/bin/uli under /opt/niraluli. A C compiler (gcc or clang) is still required for uli run / uli build."

# --- Debian package --------------------------------------------------------
echo "==> building $DEB_NAME"
mkdir -p "$DEBROOT/DEBIAN"
cp -a "$BUILDROOT/opt" "$BUILDROOT/usr" "$DEBROOT/"
cp "$ROOT/packaging/debian/control" "$DEBROOT/DEBIAN/control"
# Keep Version in control in sync with VERSION/RELEASE when overridden.
sed -i "s/^Version:.*/Version: ${VERSION}-${RELEASE}/" "$DEBROOT/DEBIAN/control"
# Installed-Size in KiB
installed_kb="$(du -sk "$DEBROOT" | awk '{print $1}')"
if grep -q '^Installed-Size:' "$DEBROOT/DEBIAN/control"; then
  sed -i "s/^Installed-Size:.*/Installed-Size: ${installed_kb}/" "$DEBROOT/DEBIAN/control"
else
  sed -i "/^Architecture:/a Installed-Size: ${installed_kb}" "$DEBROOT/DEBIAN/control"
fi
chmod 755 "$DEBROOT/DEBIAN"
find "$DEBROOT/opt" "$DEBROOT/usr" -type d -exec chmod 755 {} +
fakeroot dpkg-deb --build "$DEBROOT" "$DIST/$DEB_NAME"

# --- RPM -------------------------------------------------------------------
if command -v rpmbuild >/dev/null 2>&1; then
  echo "==> rpmbuild available; using packaging/niraluli.spec"
  RPM_TOP="$DIST/rpmbuild"
  rm -rf "$RPM_TOP"
  mkdir -p "$RPM_TOP"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
  tar -C "$DIST/stage" -czf "$RPM_TOP/SOURCES/niraluli-${VERSION}.tar.gz" "niraluli-${VERSION}"
  cp "$ROOT/packaging/niraluli.spec" "$RPM_TOP/SPECS/niraluli.spec"
  sed -i \
    -e "s/^Version:.*/Version:        ${VERSION}/" \
    -e "s/^Release:.*/Release:        ${RELEASE}%{?dist}/" \
    "$RPM_TOP/SPECS/niraluli.spec"
  rpmbuild -bb \
    --define "_topdir ${RPM_TOP}" \
    --define "debug_package %{nil}" \
    "$RPM_TOP/SPECS/niraluli.spec"
  shopt -s nullglob
  built=( "$RPM_TOP"/RPMS/x86_64/niraluli-"${VERSION}"-*.x86_64.rpm )
  if [[ ${#built[@]} -eq 0 ]]; then
    echo "error: rpm not produced under $RPM_TOP/RPMS/x86_64" >&2
    exit 1
  fi
  cp -f "${built[0]}" "$DIST/$RPM_NAME"
else
  echo "==> rpmbuild not found; using tools/mkbinaryrpm.py"
  python3 "$ROOT/tools/mkbinaryrpm.py" \
    --name niraluli \
    --version "$VERSION" \
    --release "$RELEASE" \
    --arch x86_64 \
    --summary "$SUMMARY" \
    --description "$DESCRIPTION" \
    --license Apache-2.0 \
    --buildroot "$BUILDROOT" \
    --output "$DIST/$RPM_NAME"
fi

echo
echo "Artifacts:"
ls -lh "$DIST/$ZIP_NAME" "$DIST/$RPM_NAME" "$DIST/$DEB_NAME"
echo "done."
