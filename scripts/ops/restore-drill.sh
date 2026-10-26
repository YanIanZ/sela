#!/usr/bin/env bash
# Sela restore drill (ADR-0002 §11), monthly and before every data
# migration. Self-contained: it creates a synthetic source database
# (migrations, the operator canary, one user deleted before the backup,
# one deleted AFTER it, one created AFTER it, and one live session revoked
# AFTER it), backs it up with backup.sh, takes a later key-only dump,
# restores the content dump with the newest key dump into an isolated
# scratch database with restore.sh, runs `sela migrate status`, `sela
# restore reconcile --dry-run`, `sela restore reconcile` and `sela ops
# canary check`, checks that both deleted users are gone, that the key of
# the user created after the backup is destroyed, that the restored
# session is revoked, and that a key dump older than the content dump is
# refused, records the time to restore, and drops both scratch databases.
# With --content/--keys it restores a real backup instead (then SELA_ENV,
# SELA_DATA_KEK and SELA_DATA_KEK_ID of that environment must be set;
# --keys may be the directory holding the key dumps, whose newest is
# used).
#
# Usage: scripts/ops/restore-drill.sh [--database-url URL] [--sela BIN] [--content F --keys F|DIR]
#   URL (default: SELA_DRILL_DATABASE_URL) connects to an existing database
#   as a role that may CREATE DATABASE (locally:
#   postgres://sela:sela-dev-only@127.0.0.1:5432/sela_test?sslmode=disable).
#   The password reaches psql, pg_dump, pg_restore and sela only through a
#   private temporary PGPASSFILE.
set -euo pipefail
umask 077
export TZ=UTC
here="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=pgauth.sh
. "$here/pgauth.sh"

url="${SELA_DRILL_DATABASE_URL:-}" sela_bin="" content="" keys=""
while [ $# -gt 0 ]; do
  case "$1" in
    --database-url) url="$2"; shift 2 ;;
    --sela) sela_bin="$2"; shift 2 ;;
    --content) content="$2"; shift 2 ;;
    --keys) keys="$2"; shift 2 ;;
    -h|--help) sed -n '2,22p' "$0"; exit 0 ;;
    *) echo "restore-drill.sh: unknown argument $1" >&2; exit 2 ;;
  esac
done
[ -n "$url" ] || { echo "restore-drill.sh: --database-url or SELA_DRILL_DATABASE_URL is required" >&2; exit 2; }
for bin in psql pg_dump pg_restore; do
  command -v "$bin" >/dev/null || { echo "restore-drill.sh: $bin not found" >&2; exit 2; }
done
root="$(cd "$here/../.." && pwd)"
# The cleanup trap is installed before anything is created (the work
# directory, the password file of sela_pg_url, the scratch databases), so
# an early exit (a failed go build, an unreachable server) leaves nothing.
work="" admin="" src_db="" dst_db=""
cleanup() {
  if [ -n "$admin" ]; then
    if [ -n "$src_db" ]; then psql "$admin" -XAtqc "DROP DATABASE IF EXISTS $src_db" >/dev/null 2>&1 || true; fi
    if [ -n "$dst_db" ]; then psql "$admin" -XAtqc "DROP DATABASE IF EXISTS $dst_db" >/dev/null 2>&1 || true; fi
  fi
  if [ -n "$work" ]; then rm -rf "$work"; fi
  sela_pg_cleanup
}
trap cleanup EXIT
work="$(mktemp -d)"
sela_pg_url "$url"
admin="$PG_URL"
if [ -z "$sela_bin" ]; then
  (cd "$root" && go build -o "$work/sela" ./cmd/sela)
  sela_bin="$work/sela"
fi

# Scratch database URLs: same server and credentials, another database name.
base="${admin%%\?*}"; query="${admin#"$base"}"; prefix="${base%/*}"
suffix="$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')"
src_db="sela_drill_src_$suffix"; dst_db="sela_drill_dst_$suffix"
src="$prefix/$src_db$query"; dst="$prefix/$dst_db$query"
psql "$admin" -XAtqc "CREATE DATABASE $dst_db" >/dev/null

synthetic=0
if [ -z "$content" ]; then
  synthetic=1
  export SELA_ENV=test SELA_DATA_KEK_ID=test-drill
  SELA_DATA_KEK="$("$sela_bin" keys generate-kek --env test | sed -n 's/^SELA_DATA_KEK=//p')"
  export SELA_DATA_KEK
  psql "$admin" -XAtqc "CREATE DATABASE $src_db" >/dev/null
  SELA_DATABASE_URL="$src" "$sela_bin" migrate up >/dev/null
  SELA_DATABASE_URL="$src" "$sela_bin" ops canary write >/dev/null
  # d001: deleted before the backup (its key row is already a tombstone).
  # d002: alive at backup time, deleted afterwards (key row live in the
  # backup's own key dump, a tombstone only in the later key dump).
  # k001: stays; its session is live at backup time and revoked afterwards.
  psql "$src" -XAtqc "INSERT INTO users (id, kind, age_band, age_band_source, age_band_confirmed_at, locale, created_at, last_active_at)
      VALUES ('00000000-0000-4000-8000-00000000d001', 'account', '18_plus', 'self_declared', now(), 'id', now(), now()),
             ('00000000-0000-4000-8000-00000000d002', 'account', '18_plus', 'self_declared', now(), 'id', now(), now()),
             ('00000000-0000-4000-8000-00000000c001', 'account', '18_plus', 'self_declared', now(), 'id', now(), now());
    INSERT INTO user_data_keys (user_id, created_at, destroyed_at) VALUES ('00000000-0000-4000-8000-00000000d001', now(), now());
    INSERT INTO user_data_keys (user_id, wrapped_dek, kek_id, created_at)
      VALUES ('00000000-0000-4000-8000-00000000d002', '\\x00', 'test-drill', now()),
             ('00000000-0000-4000-8000-00000000c001', '\\x00', 'test-drill', now());
    INSERT INTO sessions (id, user_id, token_hash, kind, trusted_device, device_browser, device_os, created_at, last_seen_at,
        idle_timeout_seconds, absolute_expires_at, authenticated_at)
      VALUES ('00000000-0000-4000-8000-0000000051a1', '00000000-0000-4000-8000-00000000c001', sha256('drill session'::bytea), 'account',
        true, 'other', 'other', now(), now(), 604800, now() + interval '30 days', now());" >/dev/null
  SELA_DATABASE_URL="$src" "$here/backup.sh" --keys-only --out-dir "$work/stale" >/dev/null
  sleep 1
  SELA_DATABASE_URL="$src" "$here/backup.sh" --out-dir "$work/backup" --sela "$sela_bin" >/dev/null
  content="$(ls "$work"/backup/sela-content-*.dump)"
  stale_keys="$(ls "$work"/stale/sela-keys-*.dump)"
  # The deletion after the backup (as the deletion path leaves it), then
  # the next key-only dump.
  # After the backup also: k001's session is revoked (a password change)
  # and n001 signs up (its users row is in no content dump, its key in the
  # later key dump).
  psql "$src" -XAtqc "UPDATE user_data_keys SET wrapped_dek = NULL, kek_id = NULL, destroyed_at = now()
      WHERE user_id = '00000000-0000-4000-8000-00000000d002';
    DELETE FROM users WHERE id = '00000000-0000-4000-8000-00000000d002';
    UPDATE sessions SET revoked_at = now(), revoke_reason = 'password_changed' WHERE id = '00000000-0000-4000-8000-0000000051a1';
    INSERT INTO users (id, kind, age_band, age_band_source, age_band_confirmed_at, locale, created_at, last_active_at)
      VALUES ('00000000-0000-4000-8000-00000000e001', 'account', '18_plus', 'self_declared', now(), 'id', now(), now());
    INSERT INTO user_data_keys (user_id, wrapped_dek, kek_id, created_at)
      VALUES ('00000000-0000-4000-8000-00000000e001', '\\x00', 'test-drill', now());" >/dev/null
  sleep 1
  SELA_DATABASE_URL="$src" "$here/backup.sh" --keys-only --out-dir "$work/backup" >/dev/null
  keys="$work/backup"
  # A key dump older than the content dump must be refused.
  if SELA_RESTORE_DATABASE_URL="$dst" "$here/restore.sh" --content "$content" --keys "$stale_keys" --dry-run >/dev/null 2>"$work/stale.err"; then
    echo "drill: restore.sh accepted a key dump older than the content dump"; exit 1
  fi
  grep -q 'older than the content dump' "$work/stale.err" || { echo "drill: unexpected refusal"; cat "$work/stale.err"; exit 1; }
fi

start=$(date +%s)
SELA_RESTORE_DATABASE_URL="$dst" "$here/restore.sh" --content "$content" --keys "$keys" --dry-run >/dev/null
restore_out="$(SELA_RESTORE_DATABASE_URL="$dst" "$here/restore.sh" --content "$content" --keys "$keys")"
elapsed=$(( $(date +%s) - start ))

status_out="$(SELA_DATABASE_URL="$dst" "$sela_bin" migrate status)" || { echo "drill: migrate status failed"; echo "$status_out"; exit 1; }
reconcile_out="$(SELA_DATABASE_URL="$dst" "$sela_bin" restore reconcile --dry-run)"
SELA_DATABASE_URL="$dst" "$sela_bin" ops canary check >/dev/null
if [ "$synthetic" = 1 ]; then
  grep -q 'users with a destroyed key: 2' <<<"$reconcile_out" || {
    echo "drill: reconcile did not find both deleted users (one deleted after the backup)"; echo "$reconcile_out"; exit 1; }
  grep -q 'keys without a user: 1' <<<"$reconcile_out" || {
    echo "drill: reconcile did not find the key of the user created after the backup"; echo "$reconcile_out"; exit 1; }
  grep -q 'sessions not ended: 1' <<<"$reconcile_out" || {
    echo "drill: reconcile did not find the restored session"; echo "$reconcile_out"; exit 1; }
  deleted_out="$(SELA_DATABASE_URL="$dst" "$sela_bin" restore reconcile)"
  grep -q 'deleted through the deletion path: 2' <<<"$deleted_out" || { echo "drill: reconcile did not delete them"; echo "$deleted_out"; exit 1; }
  grep -q 'keys without a user destroyed: 1' <<<"$deleted_out" || { echo "drill: reconcile did not destroy the orphan key"; echo "$deleted_out"; exit 1; }
  grep -q 'sessions revoked (everyone signs in again): 1' <<<"$deleted_out" || { echo "drill: reconcile did not revoke the session"; echo "$deleted_out"; exit 1; }
  left="$(psql "$dst" -XAtqc "SELECT count(*) FROM users WHERE id IN ('00000000-0000-4000-8000-00000000d001', '00000000-0000-4000-8000-00000000d002')")"
  [ "$left" = 0 ] || { echo "drill: $left deleted users are back after reconcile"; exit 1; }
  live="$(psql "$dst" -XAtqc "SELECT count(*) FROM user_data_keys WHERE user_id = '00000000-0000-4000-8000-00000000e001' AND destroyed_at IS NULL")"
  [ "$live" = 0 ] || { echo "drill: the key of a user who no longer exists is still live after reconcile"; exit 1; }
  open="$(psql "$dst" -XAtqc "SELECT count(*) FROM sessions WHERE revoked_at IS NULL OR revoke_reason <> 'restored'")"
  [ "$open" = 0 ] || { echo "drill: $open restored sessions are not ended after reconcile"; exit 1; }
fi
echo "restore drill ok: time to restore ${elapsed}s"
grep '^key dump:' <<<"$restore_out"
echo "$status_out" | tail -1
echo "$reconcile_out"
if [ "$synthetic" = 1 ]; then echo "$deleted_out"; fi
echo "canary: ok"
