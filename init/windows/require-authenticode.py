#!/usr/bin/env python3
"""Fail if a PE has no Authenticode certificate table (IMAGE_DIRECTORY_ENTRY_SECURITY)."""

from __future__ import annotations

import struct
import sys
from pathlib import Path


def cert_size(path: Path) -> int:
    data = path.read_bytes()
    if data[:2] != b"MZ":
        raise SystemExit(f"{path}: not a PE")
    e_lfanew = struct.unpack_from("<I", data, 0x3C)[0]
    if data[e_lfanew : e_lfanew + 4] != b"PE\0\0":
        raise SystemExit(f"{path}: not a PE")
    magic = struct.unpack_from("<H", data, e_lfanew + 24)[0]
    if magic == 0x10B:
        dd = e_lfanew + 24 + 96
    elif magic == 0x20B:
        dd = e_lfanew + 24 + 112
    else:
        raise SystemExit(f"{path}: unknown optional-header magic {magic:#x}")
    _, size = struct.unpack_from("<II", data, dd + 4 * 8)
    return size


def main() -> None:
    if len(sys.argv) < 2:
        raise SystemExit("usage: require-authenticode.py FILE [FILE...]")
    failed = False
    for raw in sys.argv[1:]:
        path = Path(raw)
        size = cert_size(path)
        if size <= 0:
            print(f"{path}: no Authenticode certificate table", file=sys.stderr)
            failed = True
            continue
        print(f"{path}: Authenticode present ({size} bytes)")
    if failed:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
