Name:           niraluli
Version:        0.81
Release:        1%{?dist}
Summary:        Niraluli programming language with bundled Go toolchain
License:        Apache-2.0
URL:            https://github.com/mathimaaran/niraluli
Source0:        niraluli-%{version}.tar.gz
BuildArch:      x86_64
# Bundled Go; do not pull host golang Requires from ELF scanning.
AutoReq:        no
AutoProv:       no

%description
Niraluli (நிரலுளி) is a Go-inspired programming language with semantic
Tamil keywords and a C/Linux backend.

This package installs the compiler tree, stdlib, docs, corpus samples, a
portable Go 1.22 toolchain, and a prebuilt /opt/niraluli/bin/uli under
/opt/niraluli. The /usr/bin/uli wrapper sets NIRALULI_ROOT and GOROOT.

A C compiler (gcc or clang) is still required to compile and run programs
via `uli run` / `uli build`. Optional SQL backends need libsqlite3 and/or
libpq at runtime.

%prep
%setup -q

%build
export PATH="$PWD/.tools/go/bin:$PATH"
export GOROOT="$PWD/.tools/go"
mkdir -p bin
go build -o bin/uli ./cmd/uli

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/opt/niraluli
# Copy staged tree; exclude VCS noise if present.
tar -C . -cf - \
  --exclude='.git' \
  --exclude='dist' \
  --exclude='demo.txt' \
  --exclude='demo2.txt' \
  . | tar -C %{buildroot}/opt/niraluli -xf -
install -D -m 755 packaging/uli.wrapper %{buildroot}/usr/bin/uli

%files
%dir /opt/niraluli
/opt/niraluli/
/usr/bin/uli

%changelog
* Sun Sep 06 2026 Niraluli Maintainers <noreply@niraluli.dev> - 0.81-1
- Tamil-0.81 release: file/time/HTTP polish, Postgres, docs, bundled Go.
