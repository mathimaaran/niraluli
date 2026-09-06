#!/usr/bin/env bash
# Build Niraluli release artifacts: source.zip (with Go) and x86_64 RPM.
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
RPM_TOP="$DIST/rpmbuild"
ZIP_NAME="niraluli-${VERSION}-source.zip"
TAR_NAME="niraluli-${VERSION}.tar.gz"
RPM_NAME="niraluli-${VERSION}-${RELEASE}.x86_64.rpm"

echo "==> Niraluli ${VERSION} release packaging"

rm -rf "$DIST/stage" "$RPM_TOP"
mkdir -p "$STAGE" "$DIST" "$RPM_TOP"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

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

# --- Tarball for rpmbuild --------------------------------------------------
echo "==> writing $RPM_TOP/SOURCES/$TAR_NAME"
tar -C "$DIST/stage" -czf "$RPM_TOP/SOURCES/$TAR_NAME" "niraluli-${VERSION}"
cp "$ROOT/packaging/niraluli.spec" "$RPM_TOP/SPECS/niraluli.spec"

# Patch Version/Release in a copy if env overrides differ from the checked-in spec
sed -i \
  -e "s/^Version:.*/Version:        ${VERSION}/" \
  -e "s/^Release:.*/Release:        ${RELEASE}%{?dist}/" \
  "$RPM_TOP/SPECS/niraluli.spec"

# --- rpmbuild --------------------------------------------------------------
if ! command -v rpmbuild >/dev/null 2>&1; then
  echo "rpmbuild not found; attempting: sudo apt-get install -y rpm"
  sudo apt-get install -y rpm
fi

echo "==> rpmbuild -bb"
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
# Also keep the distro-tagged name if different
cp -f "${built[0]}" "$DIST/$(basename "${built[0]}")"

echo
echo "Artifacts:"
ls -lh "$DIST/$ZIP_NAME" "$DIST/$RPM_NAME" "${built[0]}"
echo "done."
