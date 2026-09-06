#!/usr/bin/env python3
"""Build a minimal binary RPM (gzip+cpio payload) without rpmbuild.

Usage:
  mkbinaryrpm.py --name niraluli --version 0.81 --release 1 \\
    --arch x86_64 --summary '...' --description '...' \\
    --buildroot /path/to/root --output dist/niraluli-0.81-1.x86_64.rpm
"""
from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import os
import struct
import sys
import time
from pathlib import Path


# RPM tag numbers (subset)
RPMTAG_NAME = 1000
RPMTAG_VERSION = 1001
RPMTAG_RELEASE = 1002
RPMTAG_SUMMARY = 1004
RPMTAG_DESCRIPTION = 1005
RPMTAG_BUILDTIME = 1006
RPMTAG_BUILDHOST = 1007
RPMTAG_SIZE = 1009
RPMTAG_LICENSE = 1014
RPMTAG_GROUP = 1016
RPMTAG_OS = 1021
RPMTAG_ARCH = 1022
RPMTAG_PAYLOADFORMAT = 1124
RPMTAG_PAYLOADCOMPRESSOR = 1125
RPMTAG_PAYLOADFLAGS = 1126
RPMTAG_SOURCERPM = 1044
RPMTAG_FILEDIGESTALGO = 5011
RPMTAG_DIRINDEXES = 1116
RPMTAG_BASENAMES = 1117
RPMTAG_DIRNAMES = 1118
RPMTAG_FILESIZES = 1028
RPMTAG_FILEMODES = 1030
RPMTAG_FILERDEVS = 1033
RPMTAG_FILEMTIMES = 1034
RPMTAG_FILEDIGESTS = 1035
RPMTAG_FILELINKTOS = 1036
RPMTAG_FILEFLAGS = 1037
RPMTAG_FILEUSERNAME = 1039
RPMTAG_FILEGROUPNAME = 1040
RPMTAG_FILEDEVICES = 1095
RPMTAG_FILEINODES = 1096
RPMTAG_FILELANGS = 1097
RPMTAG_PAYLOADSHA256 = 5093
RPMTAG_PAYLOADDIGEST = 5092
RPMTAG_PAYLOADDIGESTALGO = 5093  # reused carefully below

# Header types
RPM_NULL = 0
RPM_CHAR = 1
RPM_INT8 = 2
RPM_INT16 = 3
RPM_INT32 = 4
RPM_INT64 = 5
RPM_STRING = 6
RPM_BIN = 7
RPM_STRING_ARRAY = 8
RPM_I18NSTRING = 9

HEADER_MAGIC = b"\x8e\xad\xe8\x01"
LEAD_MAGIC = b"\xed\xab\xee\xdb"


def align(n: int, a: int = 8) -> int:
    return (n + a - 1) // a * a


def encode_cpio(buildroot: Path) -> bytes:
    """Newc cpio archive of files under buildroot (absolute install paths)."""
    out = io.BytesIO()
    inode = 1

    def write_header(name: str, mode: int, size: int, mtime: int, nlink: int = 1) -> None:
        nonlocal inode
        name_b = name.encode("utf-8") + b"\x00"
        hdr = (
            b"070701"
            + f"{inode:08x}".encode()
            + f"{mode:08x}".encode()
            + f"{0:08x}".encode()  # uid
            + f"{0:08x}".encode()  # gid
            + f"{nlink:08x}".encode()
            + f"{mtime:08x}".encode()
            + f"{size:08x}".encode()
            + f"{0:08x}".encode()  # major
            + f"{0:08x}".encode()  # minor
            + f"{0:08x}".encode()  # rmajor
            + f"{0:08x}".encode()  # rminor
            + f"{len(name_b):08x}".encode()
            + f"{0:08x}".encode()  # checksum
            + name_b
        )
        out.write(hdr)
        pad = align(len(hdr), 4) - len(hdr)
        out.write(b"\x00" * pad)
        inode += 1

    # Collect paths relative to buildroot
    entries: list[Path] = []
    for dirpath, dirnames, filenames in os.walk(buildroot):
        dirnames.sort()
        filenames.sort()
        rel_dir = Path(dirpath).relative_to(buildroot)
        # ensure directory itself is listed (except root)
        if rel_dir != Path("."):
            entries.append(Path(dirpath))
        for fn in filenames:
            entries.append(Path(dirpath) / fn)

    # Also include empty dirs that walk visits; already covered.
    # Always include standard install roots if present.
    seen_dirs: set[Path] = set()

    for path in entries:
        rel = path.relative_to(buildroot)
        install_name = "/" + rel.as_posix()
        st = path.lstat()
        mode = st.st_mode
        mtime = int(st.st_mtime)
        if path.is_symlink():
            target = os.readlink(path).encode("utf-8")
            write_header(install_name, mode, len(target), mtime)
            out.write(target)
            pad = align(len(target), 4) - len(target)
            out.write(b"\x00" * pad)
        elif path.is_dir():
            if path in seen_dirs:
                continue
            seen_dirs.add(path)
            write_header(install_name + "/", mode, 0, mtime, nlink=2)
        elif path.is_file():
            data = path.read_bytes()
            write_header(install_name, mode, len(data), mtime)
            out.write(data)
            pad = align(len(data), 4) - len(data)
            out.write(b"\x00" * pad)

    # TRAILER
    write_header("TRAILER!!!", 0, 0, 0)
    # pad archive to 4 bytes
    pos = out.tell()
    out.write(b"\x00" * (align(pos, 4) - pos))
    return out.getvalue()


class HeaderStore:
    def __init__(self) -> None:
        self.entries: list[tuple[int, int, object]] = []

    def add(self, tag: int, typ: int, value: object) -> None:
        self.entries.append((tag, typ, value))

    def encode(self) -> bytes:
        # Sort by tag as rpm does
        ents = sorted(self.entries, key=lambda e: e[0])
        index = io.BytesIO()
        store = io.BytesIO()
        for tag, typ, value in ents:
            offset = store.tell()
            count = 1
            if typ == RPM_STRING or typ == RPM_I18NSTRING:
                assert isinstance(value, str)
                store.write(value.encode("utf-8") + b"\x00")
            elif typ == RPM_STRING_ARRAY:
                assert isinstance(value, list)
                count = len(value)
                for s in value:
                    store.write(s.encode("utf-8") + b"\x00")
            elif typ == RPM_BIN:
                assert isinstance(value, (bytes, bytearray))
                count = len(value)
                store.write(value)
            elif typ == RPM_INT32:
                if isinstance(value, list):
                    count = len(value)
                    for v in value:
                        store.write(struct.pack("!I", v & 0xFFFFFFFF))
                else:
                    store.write(struct.pack("!I", int(value) & 0xFFFFFFFF))
            elif typ == RPM_INT16:
                # align to 2
                pad = align(store.tell(), 2) - store.tell()
                store.write(b"\x00" * pad)
                offset = store.tell()
                if isinstance(value, list):
                    count = len(value)
                    for v in value:
                        store.write(struct.pack("!H", v & 0xFFFF))
                else:
                    store.write(struct.pack("!H", int(value) & 0xFFFF))
            elif typ == RPM_INT8 or typ == RPM_CHAR:
                if isinstance(value, list):
                    count = len(value)
                    store.write(bytes(value))
                else:
                    store.write(bytes([int(value) & 0xFF]))
            else:
                raise ValueError(f"unsupported type {typ}")
            # pad store region between region? RPM pads each INT16/32 alignment before write — handled above for INT16
            # INT32 must be 4-aligned
            if typ == RPM_INT32:
                # we already wrote; for first write need align before
                pass
            index.write(struct.pack("!iiii", tag, typ, offset, count))

        # Actually INT32 alignment must happen BEFORE writing each INT32 region.
        # Rebuild carefully.
        return self._encode_aligned(ents)

    def _encode_aligned(self, ents: list[tuple[int, int, object]]) -> bytes:
        index = io.BytesIO()
        store = bytearray()

        def pad_to(a: int) -> None:
            while len(store) % a:
                store.append(0)

        for tag, typ, value in ents:
            if typ in (RPM_INT16,):
                pad_to(2)
            elif typ in (RPM_INT32, RPM_INT64):
                pad_to(4)
            offset = len(store)
            count = 1
            if typ in (RPM_STRING, RPM_I18NSTRING):
                store.extend(value.encode("utf-8") + b"\x00")  # type: ignore[union-attr]
            elif typ == RPM_STRING_ARRAY:
                assert isinstance(value, list)
                count = len(value)
                for s in value:
                    store.extend(s.encode("utf-8") + b"\x00")
            elif typ == RPM_BIN:
                assert isinstance(value, (bytes, bytearray))
                count = len(value)
                store.extend(value)
            elif typ == RPM_INT32:
                if isinstance(value, list):
                    count = len(value)
                    for v in value:
                        store.extend(struct.pack("!I", v & 0xFFFFFFFF))
                else:
                    store.extend(struct.pack("!I", int(value) & 0xFFFFFFFF))
            elif typ == RPM_INT16:
                if isinstance(value, list):
                    count = len(value)
                    for v in value:
                        store.extend(struct.pack("!H", v & 0xFFFF))
                else:
                    store.extend(struct.pack("!H", int(value) & 0xFFFF))
            elif typ in (RPM_INT8, RPM_CHAR):
                if isinstance(value, list):
                    count = len(value)
                    store.extend(bytes(int(x) & 0xFF for x in value))
                else:
                    store.append(int(value) & 0xFF)
            else:
                raise ValueError(f"unsupported type {typ}")
            index.write(struct.pack("!iiii", tag, typ, offset, count))

        idx = index.getvalue()
        store_b = bytes(store)
        # Header: magic + reserved + nindex + hsize + index + store
        hdr = HEADER_MAGIC + b"\x00\x00\x00\x00"
        hdr += struct.pack("!II", len(ents), len(store_b))
        hdr += idx + store_b
        return hdr


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def collect_files(buildroot: Path) -> list[dict]:
    files: list[dict] = []
    for dirpath, dirnames, filenames in os.walk(buildroot):
        dirnames.sort()
        filenames.sort()
        for fn in filenames:
            path = Path(dirpath) / fn
            rel = path.relative_to(buildroot)
            install = "/" + rel.as_posix()
            st = path.lstat()
            if path.is_symlink():
                digest = ""
                linkto = os.readlink(path)
                size = 0
            else:
                data = path.read_bytes()
                digest = hashlib.md5(data).hexdigest()
                linkto = ""
                size = len(data)
            files.append(
                {
                    "path": install,
                    "size": size,
                    "mode": st.st_mode,
                    "mtime": int(st.st_mtime),
                    "digest": digest,
                    "linkto": linkto,
                    "flags": 0,
                }
            )
    files.sort(key=lambda f: f["path"])
    return files


def build_rpm(
    *,
    name: str,
    version: str,
    release: str,
    arch: str,
    summary: str,
    description: str,
    license_name: str,
    buildroot: Path,
    output: Path,
) -> None:
    cpio = encode_cpio(buildroot)
    payload = gzip.compress(cpio, compresslevel=9, mtime=0)
    files = collect_files(buildroot)
    total_size = sum(f["size"] for f in files)

    # Dirname / basename split
    dirnames: list[str] = []
    dir_index: dict[str, int] = {}
    basenames: list[str] = []
    dirindexes: list[int] = []
    for f in files:
        p = f["path"]
        d = os.path.dirname(p)
        if not d.endswith("/"):
            d += "/"
        b = os.path.basename(p)
        if d not in dir_index:
            dir_index[d] = len(dirnames)
            dirnames.append(d)
        dirindexes.append(dir_index[d])
        basenames.append(b)

    hdr = HeaderStore()
    hdr.add(RPMTAG_NAME, RPM_STRING, name)
    hdr.add(RPMTAG_VERSION, RPM_STRING, version)
    hdr.add(RPMTAG_RELEASE, RPM_STRING, release)
    hdr.add(RPMTAG_SUMMARY, RPM_I18NSTRING, summary)
    hdr.add(RPMTAG_DESCRIPTION, RPM_I18NSTRING, description)
    hdr.add(RPMTAG_BUILDTIME, RPM_INT32, int(time.time()))
    hdr.add(RPMTAG_BUILDHOST, RPM_STRING, "niraluli-package-release")
    hdr.add(RPMTAG_SIZE, RPM_INT32, total_size)
    hdr.add(RPMTAG_LICENSE, RPM_STRING, license_name)
    hdr.add(RPMTAG_GROUP, RPM_STRING, "Development/Languages")
    hdr.add(RPMTAG_OS, RPM_STRING, "linux")
    hdr.add(RPMTAG_ARCH, RPM_STRING, arch)
    hdr.add(RPMTAG_PAYLOADFORMAT, RPM_STRING, "cpio")
    hdr.add(RPMTAG_PAYLOADCOMPRESSOR, RPM_STRING, "gzip")
    hdr.add(RPMTAG_PAYLOADFLAGS, RPM_STRING, "9")
    hdr.add(RPMTAG_SOURCERPM, RPM_STRING, f"{name}-{version}-{release}.src.rpm")
    hdr.add(RPMTAG_FILEDIGESTALGO, RPM_INT32, 1)  # MD5 for simplicity with older rpm
    hdr.add(RPMTAG_DIRNAMES, RPM_STRING_ARRAY, dirnames)
    hdr.add(RPMTAG_BASENAMES, RPM_STRING_ARRAY, basenames)
    hdr.add(RPMTAG_DIRINDEXES, RPM_INT32, dirindexes)
    hdr.add(RPMTAG_FILESIZES, RPM_INT32, [f["size"] for f in files])
    hdr.add(RPMTAG_FILEMODES, RPM_INT16, [f["mode"] for f in files])
    hdr.add(RPMTAG_FILERDEVS, RPM_INT16, [0] * len(files))
    hdr.add(RPMTAG_FILEMTIMES, RPM_INT32, [f["mtime"] for f in files])
    hdr.add(RPMTAG_FILEDIGESTS, RPM_STRING_ARRAY, [f["digest"] for f in files])
    hdr.add(RPMTAG_FILELINKTOS, RPM_STRING_ARRAY, [f["linkto"] for f in files])
    hdr.add(RPMTAG_FILEFLAGS, RPM_INT32, [f["flags"] for f in files])
    hdr.add(RPMTAG_FILEUSERNAME, RPM_STRING_ARRAY, ["root"] * len(files))
    hdr.add(RPMTAG_FILEGROUPNAME, RPM_STRING_ARRAY, ["root"] * len(files))
    hdr.add(RPMTAG_FILEDEVICES, RPM_INT32, [1] * len(files))
    hdr.add(RPMTAG_FILEINODES, RPM_INT32, list(range(1, len(files) + 1)))
    hdr.add(RPMTAG_FILELANGS, RPM_STRING_ARRAY, [""] * len(files))

    header = hdr.encode()

    # Immutable region signature header: only size tags needed for modern rpm.
    # Use header+payload MD5 in the signature header (legacy compatible).
    sig = HeaderStore()
    # RPMSIGTAG_SIZE = 1000 (size of header+payload)
    # RPMSIGTAG_MD5 = 1004
    # RPMSIGTAG_PAYLOADSIZE = 1007
    RPMSIGTAG_SIZE = 1000
    RPMSIGTAG_MD5 = 1004
    RPMSIGTAG_PAYLOADSIZE = 1007
    sig.add(RPMSIGTAG_SIZE, RPM_INT32, len(header) + len(payload))
    sig.add(RPMSIGTAG_PAYLOADSIZE, RPM_INT32, len(cpio))
    md5 = hashlib.md5(header + payload).digest()
    sig.add(RPMSIGTAG_MD5, RPM_BIN, md5)
    sig_hdr = sig.encode()
    # Signature header padded to 8 bytes
    pad = align(len(sig_hdr), 8) - len(sig_hdr)
    sig_hdr_padded = sig_hdr + b"\x00" * pad

    # Lead
    nevr = f"{name}-{version}-{release}".encode("ascii")[:66]
    lead = LEAD_MAGIC + struct.pack("!HBB", 3, 0, 0)  # major=3, minor=0, type=0 binary
    lead += struct.pack("!H", 1)  # archnum (i386 legacy; real arch in header)
    lead += nevr + b"\x00" * (66 - len(nevr))
    lead += struct.pack("!H", 1)  # osnum linux
    lead += struct.pack("!H", 5)  # signature type = header-style
    lead += b"\x00" * 16
    assert len(lead) == 96

    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(lead + sig_hdr_padded + header + payload)
    print(f"wrote {output} ({output.stat().st_size} bytes, {len(files)} files)")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--name", required=True)
    ap.add_argument("--version", required=True)
    ap.add_argument("--release", default="1")
    ap.add_argument("--arch", default="x86_64")
    ap.add_argument("--summary", required=True)
    ap.add_argument("--description", required=True)
    ap.add_argument("--license", default="Apache-2.0")
    ap.add_argument("--buildroot", required=True, type=Path)
    ap.add_argument("--output", required=True, type=Path)
    args = ap.parse_args()
    if not args.buildroot.is_dir():
        print(f"buildroot not a directory: {args.buildroot}", file=sys.stderr)
        return 1
    build_rpm(
        name=args.name,
        version=args.version,
        release=args.release,
        arch=args.arch,
        summary=args.summary,
        description=args.description,
        license_name=args.license,
        buildroot=args.buildroot,
        output=args.output,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
