"""Deterministic gzip storage of large committed JSONL outputs (storage format ``sela-jsonl-gzip-v1``).

The v3 gap-search candidates (``data/research/v3_gap_search/<run>/candidates.jsonl``, 7 to 44 MB of
metadata) are committed compressed, as ``candidates.jsonl.gz``. The compressed bytes depend only on
the content: one gzip member (RFC 1952) with a fixed 10-byte header (no file name, comment, extra
field or header CRC; MTIME 0; XFL 2; OS 255), a raw deflate stream at level 9 (window 15, memLevel 8,
default strategy), then the CRC-32 and the size. The header is written here, not by the ``gzip``
module, because what that module writes has changed between Python versions.

Hashes. A recorded SHA-256 of a ``.jsonl`` name that is stored compressed is the SHA-256 of the
UNCOMPRESSED content, as it was before the files were compressed, so every recorded value keeps its
meaning; the SHA-256 of the ``.gz`` bytes is recorded next to it where the file is an output.

``write`` keeps an existing ``.gz`` whose content is already the new content, so a rebuild never
changes committed bytes, even where the local deflate (for example zlib-ng) would compress the same
content differently. Stdlib only. See docs/research/V3_GAP_SEARCH.md ("Storage of the candidates").
"""
from __future__ import annotations

import gzip
import hashlib
import os
import struct
import zlib
from pathlib import Path

STORAGE_VERSION = "sela-jsonl-gzip-v1"
SUFFIX = ".gz"
LEVEL = 9
# ID1 ID2, CM 8 (deflate), FLG 0 (no FNAME/FCOMMENT/FEXTRA/FHCRC), MTIME 0, XFL 2 (level 9), OS 255 (unknown)
HEADER = b"\x1f\x8b\x08\x00\x00\x00\x00\x00\x02\xff"
FORMAT = ("gzip (RFC 1952), one member; header without file name, comment or extra field, MTIME 0, XFL 2, "
          "OS 255; raw deflate level 9 (window 15, memLevel 8, default strategy); CRC-32 and size trailer")


def gzip_bytes(data: bytes) -> bytes:
    """The deterministic gzip encoding of ``data``."""
    c = zlib.compressobj(LEVEL, zlib.DEFLATED, -zlib.MAX_WBITS, zlib.DEF_MEM_LEVEL, zlib.Z_DEFAULT_STRATEGY)
    body = c.compress(data) + c.flush()
    return HEADER + body + struct.pack("<II", zlib.crc32(data) & 0xFFFFFFFF, len(data) & 0xFFFFFFFF)


def decode(raw: bytes):
    """(content, problems) of a stored file. ``problems`` names every departure from the format above
    (header fields, more than one member, trailer); content is None when the stream cannot be read."""
    problems = []
    if raw[:2] != HEADER[:2] or raw[2:3] != HEADER[2:3]:
        return None, ["not a gzip deflate stream"]
    if raw[:10] != HEADER:
        problems.append("gzip header is not the deterministic one (flags 0: no file name; MTIME 0; XFL 2; OS 255)")
    try:
        if raw[3] != 0:  # optional fields present: let the gzip module parse the header
            return gzip.decompress(raw), problems
        d = zlib.decompressobj(-zlib.MAX_WBITS)
        content = d.decompress(raw[10:]) + d.flush()
    except (OSError, EOFError, zlib.error) as exc:
        return None, problems + [f"unreadable gzip stream ({exc})"]
    if not d.eof:
        return None, problems + ["truncated gzip stream"]
    trailer = d.unused_data
    if len(trailer) < 8:
        return None, problems + ["truncated gzip trailer"]
    crc, size = struct.unpack("<II", trailer[:8])
    if crc != zlib.crc32(content) & 0xFFFFFFFF or size != len(content) & 0xFFFFFFFF:
        problems.append("gzip trailer (CRC-32 or size) does not match the content")
    if trailer[8:]:
        problems.append("more than one gzip member (or trailing bytes)")
    return content, problems


def gz_path(path) -> Path:
    """``x.jsonl`` -> ``x.jsonl.gz`` (a ``.gz`` path is returned unchanged)."""
    p = Path(path)
    return p if p.name.endswith(SUFFIX) else p.with_name(p.name + SUFFIX)


def logical_path(path) -> Path:
    """``x.jsonl.gz`` -> ``x.jsonl``: the name under which hashes of the content are recorded."""
    p = Path(path)
    return p.with_name(p.name[:-len(SUFFIX)]) if p.name.endswith(SUFFIX) else p


def stored_path(path) -> Path:
    """The file that holds ``path``'s content: ``path`` itself when it names a ``.gz`` file, else its
    ``.gz`` sibling when that exists, else ``path``."""
    p = Path(path)
    if p.name.endswith(SUFFIX):
        return p
    g = gz_path(p)
    return g if g.exists() else p


def exists(path) -> bool:
    return stored_path(path).exists()


def read_bytes(path) -> bytes:
    """Uncompressed content of ``path`` (a ``.gz`` path, or a plain name whose ``.gz`` sibling holds it)."""
    p = stored_path(path)
    raw = p.read_bytes()
    return gzip.decompress(raw) if p.name.endswith(SUFFIX) else raw


def read_text(path) -> str:
    return read_bytes(path).decode("utf-8")


def content_sha256(path) -> str:
    """SHA-256 of the uncompressed content: the value recorded for the ``.jsonl`` name."""
    return hashlib.sha256(read_bytes(path)).hexdigest()


def write(path, data: bytes) -> bytes:
    """Store ``data`` at ``gz_path(path)`` (atomically) and return the bytes on disk. An existing file in
    the deterministic format with the same content is kept byte for byte."""
    g = gz_path(path)
    if g.exists():
        old = g.read_bytes()
        content, problems = decode(old)
        if content == data and not problems:
            return old
    new = gzip_bytes(data)
    g.parent.mkdir(parents=True, exist_ok=True)
    tmp = g.with_name(g.name + ".tmp")
    tmp.write_bytes(new)
    os.replace(tmp, g)
    return new
