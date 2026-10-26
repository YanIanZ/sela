# shellcheck shell=bash
# Sourced by the Sela ops scripts (backup.sh, restore.sh, restore-drill.sh).
#
# sela_pg_url URL sets PG_URL to URL without its password and, when the URL
# carries one, writes the password to a private temporary password file
# (PGPASSFILE, mode 0600 in a 0700 directory). pg_dump, pg_restore and psql
# then get only PG_URL as an argument: a process's arguments are visible to
# every local account (ps, /proc/*/cmdline), its environment and a 0600
# file are not. Non-URL connection strings and URLs without a password
# (a libpq service file, an existing PGPASSFILE) pass through unchanged.
# The password is never printed.

sela_pg_dir=""

sela_pg_cleanup() {
  if [ -n "$sela_pg_dir" ]; then rm -rf "$sela_pg_dir"; fi
}

sela_pg_url() {
  local url="$1" scheme rest authority tail userinfo hostpart user pw
  case "$url" in
    postgres://*|postgresql://*) ;;
    *) PG_URL="$url"; return 0 ;;
  esac
  case "$url" in
    *[?\&]password=*)
      echo "${0##*/}: put the database password in the URL's user part or a PGPASSFILE, not in a password= parameter" >&2
      return 2 ;;
  esac
  scheme="${url%%://*}://"
  rest="${url#*://}"
  authority="${rest%%[/?]*}"
  tail="${rest#"$authority"}"
  if [[ "$authority" != *@* ]]; then PG_URL="$url"; return 0; fi
  userinfo="${authority%%@*}"
  hostpart="${authority#*@}"
  if [[ "$userinfo" != *:* ]]; then PG_URL="$url"; return 0; fi
  user="${userinfo%%:*}"
  pw="${userinfo#*:}"
  # Percent-decode the password (as libpq does), keeping literal backslashes.
  pw="${pw//\\/\\\\}"
  printf -v pw '%b' "${pw//%/\\x}"
  if [ -z "$sela_pg_dir" ]; then
    sela_pg_dir="$(umask 077 && mktemp -d)"
  fi
  # .pgpass format: host:port:database:user:password, with \ and : escaped.
  pw="${pw//\\/\\\\}"
  pw="${pw//:/\\:}"
  (umask 077 && printf '*:*:*:*:%s\n' "$pw" > "$sela_pg_dir/pgpass")
  export PGPASSFILE="$sela_pg_dir/pgpass"
  if [ -n "$user" ]; then
    PG_URL="$scheme$user@$hostpart$tail"
  else
    PG_URL="$scheme$hostpart$tail"
  fi
}

# sela_warn_argv_password NAME URL warns when a URL given as a command-line
# argument carries a password (the script's own arguments are visible too).
sela_warn_argv_password() {
  case "$2" in
    postgres://*:*@*|postgresql://*:*@*)
      local auth="${2#*://}"; auth="${auth%%[/?]*}"
      if [[ "${auth%%@*}" == *:* ]]; then
        echo "${0##*/}: warning: $1 carries a password on the command line, where other local accounts can read it; set it in the environment variable instead" >&2
      fi ;;
  esac
}
