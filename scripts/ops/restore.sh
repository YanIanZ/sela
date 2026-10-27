#!/usr/bin/env bash
# Sela restore (ADR-0002 §11) into an ISOLATED, EMPTY scratch database —
# never into staging or demo, never over a live database. Restores the
# content dump, then the NEWEST key dump, each in one transaction. Restore
# reconcile removes the accounts whose key row is destroyed in the key
# table it finds, so the key dump decides which deletions are honoured:
# use the newest one there is (taken after the content dump; if the
# damaged database is still reachable, take a fresh one first with
# backup.sh --keys-only). A key dump older than the content dump is
# refused. Accounts deleted after the key dump was taken are not seen.
# Afterwards run:
#   sela migrate status            (must exit 0)
#   sela restore reconcile --dry-run, then sela restore reconcile
#   sela ops canary check
#
# Usage: scripts/ops/restore.sh --content FILE --keys FILE|DIR [--database-url URL] [--dry-run]
#   --keys DIR uses the newest sela-keys-*.dump in DIR. The target URL
#   defaults to SELA_RESTORE_DATABASE_URL (prefer it: arguments are visible
#   to other local accounts); its password reaches psql and pg_restore
#   through a private temporary PGPASSFILE. --dry-run verifies both
#   archives and the target, prints what would be restored and changes
#   nothing. The URL is never printed.
set -euo pipefail
umask 077
export TZ=UTC
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=pgauth.sh
. "$here/pgauth.sh"
trap sela_pg_cleanup EXIT

url="${SELA_RESTORE_DATABASE_URL:-}" content="" keys="" dry=0
while [ $# -gt 0 ]; do
  case "$1" in
    --database-url) url="$2"; sela_warn_argv_password --database-url "$url"; shift 2 ;;
    --content) content="$2"; shift 2 ;;
    --keys) keys="$2"; shift 2 ;;
    --dry-run) dry=1; shift ;;
    -h|--help) sed -n '2,23p' "$0"; exit 0 ;;
    *) echo "restore.sh: unknown argument $1" >&2; exit 2 ;;
  esac
done
[ -n "$url" ] && [ -n "$content" ] && [ -n "$keys" ] || {
  echo "restore.sh: --content, --keys and SELA_RESTORE_DATABASE_URL (or --database-url) are required" >&2; exit 2; }
for bin in pg_restore psql; do
  command -v "$bin" >/dev/null || { echo "restore.sh: $bin not found" >&2; exit 2; }
done
if [ -d "$keys" ]; then
  dir="$keys"
  keys="$(find "$dir" -maxdepth 1 -name 'sela-keys-*.dump' -type f | sort | tail -n 1)"
  [ -n "$keys" ] || { echo "restore.sh: no sela-keys-*.dump in $dir" >&2; exit 1; }
else
  # A key dump file with a newer one next to it (the hourly dumps share the
  # directory) would bring back the accounts deleted in between.
  newest="$(find "$(dirname "$keys")" -maxdepth 1 -name 'sela-keys-*.dump' -type f | sort | tail -n 1)"
  if [ -n "$newest" ] && [[ "$(basename "$newest")" > "$(basename "$keys")" ]]; then
    echo "restore.sh: a newer key dump exists next to $(basename "$keys"): $(basename "$newest"); pass the directory (or that file); refusing" >&2
    exit 1
  fi
fi
sela_pg_url "$url"

content_toc="$(pg_restore --list "$content")"
keys_toc="$(pg_restore --list "$keys")"
if grep -Eq ' TABLE (DATA )?[^ ]+ user_data_keys ' <<<"$content_toc"; then
  echo "restore.sh: the content dump contains user_data_keys; refusing" >&2; exit 1
fi
if ! grep -Eq ' TABLE [^ ]+ user_data_keys ' <<<"$keys_toc"; then
  echo "restore.sh: the key dump has no user_data_keys table; refusing" >&2; exit 1
fi
if grep -Eq ' TABLE (DATA )?[^ ]+ ' <<<"$(grep -Ev ' user_data_keys ' <<<"$keys_toc")"; then
  echo "restore.sh: the key dump holds tables other than user_data_keys; refusing" >&2; exit 1
fi
created() { sed -n '/^;[[:space:]]*Archive created at /{s/^;[[:space:]]*Archive created at \([0-9-]* [0-9:]*\).*/\1/p;q;}' <<<"$1"; }
content_at="$(created "$content_toc")" keys_at="$(created "$keys_toc")"
[ -n "$content_at" ] && [ -n "$keys_at" ] || { echo "restore.sh: an archive has no creation time; refusing" >&2; exit 1; }
if [[ "$keys_at" < "$content_at" ]]; then
  echo "restore.sh: the key dump (taken $keys_at) is older than the content dump (taken $content_at); accounts deleted in between would come back. Use the newest key dump; refusing" >&2
  exit 1
fi
tables="$(psql "$PG_URL" -XAtqc "SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema')")"
if [ "$tables" != "0" ]; then
  echo "restore.sh: the target database is not empty ($tables tables); restore only into a fresh scratch database" >&2; exit 1
fi
echo "content dump: $(grep -c ' TABLE DATA ' <<<"$content_toc") table data entries, taken $content_at"
echo "key dump:     $(grep -c ' TABLE DATA ' <<<"$keys_toc") table data entry, taken $keys_at ($(basename "$keys"))"
echo "reconcile will honour deletions up to $keys_at; an account deleted after that is not seen"
if [ "$dry" = 1 ]; then
  echo "dry run: archives readable, target empty; nothing restored"
  exit 0
fi
start=$(date +%s)
pg_restore --no-owner --exit-on-error --single-transaction --dbname="$PG_URL" "$content"
pg_restore --no-owner --exit-on-error --single-transaction --dbname="$PG_URL" "$keys"
echo "restored in $(( $(date +%s) - start ))s; next: sela migrate status, sela restore reconcile --dry-run, sela restore reconcile, sela ops canary check"
