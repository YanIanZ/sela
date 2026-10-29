#!/usr/bin/env bash
# Sela backup (ADR-0002 §11). A full run (daily) writes two dumps, in order:
#   1. content dump: pg_dump -Fc without user_data_keys
#   2. key dump:     pg_dump -Fc with only user_data_keys
# so every user in the content dump has a key row in the key dump (a user
# deleted in between appears as a tombstone, the safe direction). A
# --keys-only run (hourly, or more often) writes a key dump alone: a
# restore uses the newest key dump, so restore reconcile also removes
# accounts deleted after the last content dump (RUNBOOK §8). Every dump is
# verified with pg_restore --list and checksummed. Retention (content 30
# days, keys 7 days) and encryption at rest belong to the backup storage.
#
# Usage: scripts/ops/backup.sh --out-dir DIR [--keys-only] [--database-url URL] [--schema NAME] [--sela BIN]
#   The URL defaults to SELA_DATABASE_URL; prefer that, since a URL given as
#   an argument is visible to other local accounts. The password goes to
#   pg_dump through a private temporary PGPASSFILE, never as an argument.
#   Files are created 0600 (directory 0700), written under a temporary name
#   and renamed only once verified; an existing dump is never overwritten.
set -euo pipefail
umask 077
export TZ=UTC
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=pgauth.sh
. "$here/pgauth.sh"

out_dir="" url="${SELA_DATABASE_URL:-}" schema="public" sela_bin="" keys_only=0
while [ $# -gt 0 ]; do
  case "$1" in
    --out-dir) out_dir="$2"; shift 2 ;;
    --database-url) url="$2"; sela_warn_argv_password --database-url "$url"; shift 2 ;;
    --schema) schema="$2"; shift 2 ;;
    --sela) sela_bin="$2"; shift 2 ;;
    --keys-only) keys_only=1; shift ;;
    -h|--help) sed -n '2,19p' "$0"; exit 0 ;;
    *) echo "backup.sh: unknown argument $1" >&2; exit 2 ;;
  esac
done
[ -n "$out_dir" ] || { echo "backup.sh: --out-dir is required" >&2; exit 2; }
[ -n "$url" ] || { echo "backup.sh: SELA_DATABASE_URL or --database-url is required" >&2; exit 2; }
for bin in pg_dump pg_restore sha256sum; do
  command -v "$bin" >/dev/null || { echo "backup.sh: $bin not found" >&2; exit 2; }
done
# The cleanup trap is installed before sela_pg_url writes the password
# file, so every exit from here on (a directory that cannot be created, a
# dump name that exists, a failed dump) removes it.
partial=()
cleanup() {
  if [ ${#partial[@]} -gt 0 ]; then rm -f "${partial[@]}"; fi
  sela_pg_cleanup
}
trap cleanup EXIT
sela_pg_url "$url"

if [ -d "$out_dir" ]; then
  if [ -n "$(find "$out_dir" -maxdepth 0 -perm /077 2>/dev/null)" ]; then
    echo "backup.sh: warning: $out_dir is accessible to group or others; the dumps are 0600, but use a private (0700) directory" >&2
  fi
else
  mkdir -p -m 700 "$out_dir"
fi
ts="$(date -u +%Y%m%dT%H%M%SZ)"
content="$out_dir/sela-content-$ts.dump"
keys="$out_dir/sela-keys-$ts.dump"
if [ "$keys_only" = 1 ]; then sums="$out_dir/sela-keys-$ts.sha256"; else sums="$out_dir/sela-backup-$ts.sha256"; fi
for f in "$content" "$keys" "$sums"; do
  [ ! -e "$f" ] || { echo "backup.sh: $f already exists; refusing to overwrite it" >&2; exit 1; }
done

# The public schema is the whole database (no --schema, so restoring into a
# fresh database does not collide with its own public schema).
scope=()
if [ "$schema" != "public" ]; then scope=(--schema="$schema"); fi

# dump FINAL PG_DUMP_ARGS...: write to a temporary name, verify, rename.
dump() {
  local final="$1"; shift
  local tmp
  tmp="$(dirname "$final")/.$(basename "$final").partial"
  partial+=("$tmp")
  pg_dump -Fc --no-owner "$@" -f "$tmp" --dbname="$PG_URL"
  pg_restore --list "$tmp" >/dev/null
  chmod 600 "$tmp"
  mv "$tmp" "$final"
}

# The login throttle's rows (keyed hashes of typed usernames, about an
# hour old) are not backed up: only its definition is, and a restore starts
# it empty.
if [ "$keys_only" = 0 ]; then
  dump "$content" "${scope[@]}" --exclude-table="$schema.user_data_keys" --exclude-table-data="$schema.login_throttle"
fi
dump "$keys" --table="$schema.user_data_keys"

if [ "$keys_only" = 0 ] && [ -n "$sela_bin" ]; then
  "$sela_bin" backup check --content "$content" --keys "$keys"
fi
tmp_sums="$out_dir/.$(basename "$sums").partial"
partial+=("$tmp_sums")
if [ "$keys_only" = 1 ]; then
  (cd "$out_dir" && sha256sum "$(basename "$keys")") > "$tmp_sums"
else
  (cd "$out_dir" && sha256sum "$(basename "$content")" "$(basename "$keys")") > "$tmp_sums"
fi
chmod 600 "$tmp_sums"
mv "$tmp_sums" "$sums"

if [ "$keys_only" = 1 ]; then
  echo "key dump:     $keys"
  exit 0
fi
sum="$(sha256sum "$content" | cut -d' ' -f1)"
echo "content dump: $content"
echo "key dump:     $keys"
echo "backup reference (for sela migrate up --backup-ref): sela-content-$ts.dump#sha256:$sum"
