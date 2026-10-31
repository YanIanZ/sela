# Sela runbook (v2 demo and v3 foundation)

Authority: [SPEC](../../SPEC.md) I.ops, C5, V9, V25; v3 persistence, accounts and encryption: [ADR-0002](../adr/ADR-0002-persistence-accounts-and-encryption.md), [API_SPEC_V3](../api/API_SPEC_V3.md). Commands below were run in this repository on 2026-10-26 unless marked **not run** (they need credentials or a budget that this environment does not have; see [blockers](../status/BLOCKERS.md)).

## 1. Run locally from a clean checkout

Requirements: Go 1.26.10 (the `toolchain` line of `go.mod`; with `GOTOOLCHAIN=auto` the go command fetches it) and Node.js 22+ (npm 10+). The v2 demo needs no database, Redis, Docker or API key. `/api/v3` needs PostgreSQL 16 (§7); without `SELA_DATABASE_URL` in development every `/api/v3` route except `GET /api/v3/policies` and `GET /api/v3/auth/options` answers `503 store_unavailable`.

```sh
git clone https://github.com/YanIanZ/sela.git && cd sela
(cd web && npm ci && npm run build)          # builds web/dist
go build -o build/sela ./cmd/sela
go build -o build/fakellm ./cmd/fakellm      # TEST DOUBLE, never Meta

# A) AI disabled (manual path only) — the honest default without Modal:
SELA_ENV=development SELA_SECURE_COOKIES=false ./build/sela
# open http://localhost:8080

# B) Development with the labelled test double for AI steps:
./build/fakellm -addr 127.0.0.1:8090 &
SELA_ENV=development SELA_SECURE_COOKIES=false \
SELA_LLM_MODE=test_double SELA_ALLOW_TEST_DOUBLE=true SELA_LLM_BASE_URL=http://127.0.0.1:8090 \
./build/sela
```

In mode B every AI result is labelled **TEST DOUBLE — bukan Meta / not Meta**. The test double is refused when `SELA_ENV=production`.

Frontend development with hot reload: run the Go server on :8080, then `cd web && npm run dev` (Vite on :5173 proxies `/api`).

## 2. Checks

```sh
gofmt -l cmd internal                      # must print nothing
go vet ./...
go test -race -count=1 ./...                 # PostgreSQL tests skip without SELA_TEST_DATABASE_URL
export SELA_TEST_DATABASE_URL='postgres://sela:sela-dev-only@127.0.0.1:5432/sela_test?sslmode=disable'
SELA_REQUIRE_DB_TESTS=1 go test -race -count=1 ./...   # what CI runs: database tests required
SELA_TIMING_TESTS=1 SELA_TEST_DATABASE_URL=… go test -count=1 \
  -run 'TestUnknownUsernameCost|TestV3UnknownUsernameCost' ./internal/account ./internal/httpapi   # optional latency assertions (never with -race)
SELA_DRILL_DATABASE_URL="$SELA_TEST_DATABASE_URL" scripts/ops/restore-drill.sh   # the URL in the environment, not --database-url (§8)
(cd web && npm run typecheck && npm test && npm run build)
python3 -m unittest discover -s tests/research -p 'test_*.py'
python3 -m unittest discover -s tests/modal -p 'test_*.py'
python3 scripts/research/corpus_pipeline.py verify
E2E_BASE_URL=http://127.0.0.1:8080 node e2e/v3-frontend.e2e.mjs   # app + database running, Playwright + Chromium
go run ./cmd/sela-eval -mode offline       # deterministic evaluation vs eval/thresholds.json
go run ./cmd/sela-eval -mode test_double   # + pipeline and fault-injection runs (TEST DOUBLE)
```

Latency comparisons (known against unknown usernames) only log their measurements in the default suite: under `-race` and with packages in parallel single logins vary by several times, so a fixed band failed about one run in ten without any defect. The property they illustrate is asserted deterministically (which argon2id parameters every verification ran with). Run them with `SELA_TIMING_TESTS=1` on a quiet machine, without `-race`, when changing the hashing or the failure path.

The v1 Next.js app and its root `npm` checks were retired on 2026-10-29; see [development](DEVELOPMENT.md) for the current Go + `web/` commands.

## 3. Configuration (environment)

| Variable | Default | Meaning |
| --- | --- | --- |
| `SELA_ENV` | `production` | `production`, `staging`, `demo`, `development` or `test` (guards per environment: ADR-0002 §12 and §7 below). Production, staging and demo forbid the test double and require Secure cookies, an https `SELA_PUBLIC_ORIGIN`, `SELA_DATABASE_URL` and a KEK |
| `SELA_ADDR` | `:8080` | Listen address |
| `SELA_WEB_DIR` | `web/dist` | Built frontend |
| `SELA_PUBLIC_ORIGIN` | (request host) | Exact origin accepted for mutations, e.g. `https://sela.example` |
| `SELA_SECURE_COOKIES` | `true` in production | Set `false` only for plain-HTTP local runs |
| `SELA_TRUST_PROXY` | `false` | Use the last `X-Forwarded-For` element (across all header lines) for rate limiting; only behind a proxy that appends it |
| `SELA_MAX_SESSIONS` | `500` | Memory bound. When full, an ordinary session idle for 10 minutes is evicted first; a session with storage consent or an active share only after 30 idle minutes; otherwise new visitors get 503 `capacity` |
| `SELA_MAX_SESSIONS_PER_CLIENT` | `50` | Live sessions per client address (IPv6 /64); more → 429. Raise it if many demo visitors share one NAT |
| `SELA_METRICS_TOKEN` | — | Optional operator bearer token (≥16 characters, no spaces) for `GET /api/v2/metrics`; unset means the route does not exist |
| `SELA_REDIS_URL` | — | Optional `redis://`/`rediss://` public cache and control plane. Unset (or unavailable) keeps Sela fully functional on the authoritative embedded/PostgreSQL path; Redis never holds personal content and never decides consent or safety |
| `SELA_ADMIN_MFA_KEY` | — | Standard base64 of exactly 32 bytes; seals operator TOTP secrets. Unset disables `/api/admin/v1` (503 `admin_unavailable`). Rotating it invalidates every enrolled TOTP secret, so re-provision staff after a rotation |
| `SELA_AI_MONTHLY_BUDGET_USD` | `0` | Durable monthly cap on estimated API spend, enforced in PostgreSQL before every paid call. `0` disables the gate. Requires `SELA_DATABASE_URL`; the server refuses to start unmetered when a positive cap is set |
| `SELA_AI_RESERVE_USD` | `0.25` | Worst-case reservation per AI operation (generator + verifier + regeneration). Must be positive when the cap is enabled |
| `SELA_RESEARCH_KB` | `data/research/corpus/knowledge_base.jsonl` | Reviewer corpus index |
| `SELA_LLM_MODE` | `disabled` | `disabled`, `modal` (self-hosted vLLM on Modal), `meta_api` (Meta Model API; development/test/demo only) or `test_double` |
| `SELA_LLM_BASE_URL` | — | Modal server URL (https) or test double URL |
| `SELA_LLM_API_KEY` | — | vLLM API key (secret) |
| `SELA_MODAL_KEY`, `SELA_MODAL_SECRET` | — | Modal proxy-auth token pair (secret) |
| `SELA_LLM_SERVED_MODEL` | — | `sela-glimmer-cec94fc` |
| `SELA_LLM_MODEL_ID` | — | `meta-models/Muse-Glimmer-30B` |
| `SELA_LLM_MODEL_REVISION` | — | `cec94fcfd6512f2cfe708138c32533b346ea4f69` |
| `SELA_LLM_CONCURRENCY` | `2` | Concurrent model calls (admission) |
| `SELA_LLM_OP_TIMEOUT` | `6m` | Whole AI operation budget, including a cold start (maximum 15m; warm the model with the smoke test before a demo) |
| `SELA_LLM_WARM_WINDOW` | `5m` | Assumed warm time after a live call (= Modal scale-down) |
| `SELA_LLM_COLD_START_ESTIMATE` | `4m` | Shown to users while the model starts |
| `SELA_LLM_REASONING_EFFORT` | `minimal` | `meta_api` only: `minimal`, `low`, `medium`, `high` or `xhigh`. Muse Spark rejects `none` |
| `SELA_MAX_AI_OPS_PER_SESSION` | `8` | Per-session AI operations per 10 minutes |
| `SELA_STORE` | `memory` (development, test, demo), `disabled` (staging, production) | v2 in-memory journey store; `memory` is refused in staging and production, where v2 journey routes are not served. Alias: `SELA_V2_JOURNEY=on\|off` |
| `SELA_DATABASE_URL` | — | `postgres://` URL of the v3 store (secret; never logged or printed). Staging and production require `sslmode=verify-full` unless the host is a unix socket or loopback |
| `SELA_DB_AUTO_MIGRATE` | `false` | `true` applies pending **schema** migrations at start; development and test only |
| `SELA_DB_MAX_CONNS`, `SELA_DB_STATEMENT_TIMEOUT` | `10`, `30s` | Pool size (1–200) and per-statement timeout (1s–10m). Every size works: a request holds one connection at a time, and the guest purge and the retention job run on the one connection that holds their advisory lock; at 1, requests wait while a job runs |
| `SELA_DATA_KEK`, `SELA_DATA_KEK_ID` | — | Current key-encryption key: standard base64 of exactly 32 bytes, and its id `^(dev\|test\|demo\|staging\|prod)-[a-z0-9-]{1,40}$` whose prefix must match `SELA_ENV` (secret manager only; only the id is ever printed) |
| `SELA_DATA_KEK_PREVIOUS` | — | `id:base64,…` keys being retired; used only to unwrap (§9) |
| `SELA_GUEST_INACTIVE_TTL` (alias `SELA_GUEST_TTL`), `SELA_GUEST_MAX_AGE` | `168h`, `720h` | Guest deletion after inactivity (1h–30d) and at the latest after creation (1d–30d) |
| `SELA_MAX_GUEST_ACCOUNTS` | `10000` | Live guests; beyond it guest start answers `503 server_busy` |
| `SELA_ALLOWED_AGE_BANDS` | `18_plus` in production, else `15_17,18_plus,unknown` | Bands that may sign up (D-031: minors in production only after legal review) |
| `SELA_GUARDIAN_CONSENT_REQUIRED` | `true` | Whether `15_17` users must hold an active guardian consent record (enforced by feature milestones) |
| `SELA_DATA_REGION_LABEL` | — | Hosting region shown in the guest notice and profile once B-15 is decided (with the host, B-3) |
| `SELA_ARGON2_MEMORY_KIB`, `SELA_ARGON2_ITERATIONS`, `SELA_ARGON2_PARALLELISM` | `65536`, `2`, `1` | argon2id parameters; floor 19456 KiB and 2 passes (also a database CHECK). A stored hash whose parameters differ, lower or higher, is re-hashed on the account's next successful login. Unknown usernames are verified against dummy hashes that follow the parameters the stored credentials use (read at start and every 10 minutes); each unknown username keeps the same tuple from one refresh to the next unless the boundary between two tuples passes it, which happens to about as many unknown usernames as the share of a tuple moves (one in N for each account re-hashed on sign-in or registered among N). So, after a change, a failed login costs the same for an existing and an unknown username, and repeating it repeats the cost for both. **Residual:** for up to 10 minutes after a restart with new parameters, and for someone who knows a particular account has not signed in since the change, the cost of a failed login still reflects that; an unknown username near a boundary changes cost once when the boundary passes it (an existing account changes cost when its owner signs in); a KEK rotation re-maps every unknown username at once (§9). Change the parameters rarely and expect the stored mix to converge only as users sign in. Give every instance the same parameters (finish a rolling deploy of a change promptly): while they differ, each sign-in re-hashes to its instance's tuple, and a correct sign-in or code regeneration that overlaps another sign-in of the same account re-hashing it is refused once (`401 invalid_credentials` / `403 current_password_incorrect`, an `auth.login_failed` in the owner's trail, no counter raised; ADR-0002 §9) |
| `SELA_AUTH_HASH_CONCURRENCY` | `2` | Concurrent argon2id computations; a request waiting more than 5 s gets `503 server_busy` |
| `SELA_SESSION_IDLE`, `SELA_SESSION_ABSOLUTE` | `2h`, `12h` | Account session timeouts (untrusted device) |
| `SELA_SESSION_TRUSTED_IDLE`, `SELA_SESSION_TRUSTED_ABSOLUTE` | `168h`, `720h` | Trusted-device ("remember this device") timeouts, at most 30 days |
| `SELA_REAUTH_WINDOW` | `10m` | Password proof window for export and account deletion |

Secrets come only from the environment of the host or a secret manager; never from files in the repository. The binary never logs them.

## 4. Container image

```sh
docker build -f deploy/v2/Dockerfile --build-arg VERSION=$(git rev-parse --short HEAD) -t sela:v2 .
docker run --rm -p 8080:8080 -e SELA_SECURE_COOKIES=false sela:v2
```

Deploy as **one instance** (sessions live in process memory; several instances would split them). Put HTTPS in front, set `SELA_PUBLIC_ORIGIN`, keep `SELA_SECURE_COOKIES=true`, and set `SELA_TRUST_PROXY=true` only behind a proxy that overwrites `X-Forwarded-For`. A restart erases all sessions, which the UI explains. On SIGTERM/SIGINT the server ends open event streams with a final content-free event and exits cleanly; a second signal forces exit. **Not run:** no hosting account was available (B-3).

## 5. Meta model on Modal

Follow [modal/README.md](../../modal/README.md): budget approval, `modal token new`, environments (`dev`, `staging`), secrets, `modal deploy -e dev modal/sela_llm.py`, proxy token, smoke test, then set the `SELA_LLM_*` variables above and restart the Go app. Verify with `go run ./cmd/sela-eval -mode live`. **Not run** (B-1).

Cost controls (D-022): scale to zero after 5 minutes, at most one GPU container, proxy auth rejects anonymous requests before a container starts, and Sela's own per-session and per-address limits. To stop all spend immediately: `modal app stop sela-llm -e <env>` and set `SELA_LLM_MODE=disabled`.

Rollback: Go app — redeploy the previous image tag. Model server — on Modal Team plans `modal app rollback sela-llm -e <env>`; on Starter, check out the previous commit and `modal deploy` again. Switching AI off never affects the manual path.

## 6. Demo script (about 3 minutes)

Everything shown is synthetic; say so at the start. Use a fresh browser profile, plus a second profile/window for the fictional recipient.

1. **Start (0:00–0:20).** Welcome page: what Sela is and is not. *Mulai pelan-pelan* → boundaries and privacy (memory only, Modal processing, no real services) → anonymous → adult attestation.
2. **Permissions (0:20–0:35).** Allow AI processing; keep session-only retention. Point out they are separate and neither means sharing.
3. **Story (0:35–0:45).** Choose *Nadia* (Indonesian). The story contains a negation ("Aku tidak susah tidur"), a friend's opinion, uncertainty and a worry.
4. **Story Ledger (0:45–1:25).** *Bantu susun catatan dengan AI*: show the warming/running status, validated previews, provenance badge (`live_meta` only when the Modal endpoint is configured; otherwise say that this recording uses the test double or the manual path). Open an item: *Kata-katamu* vs *Rumusan Sela*; qualifier chips (negation, other person, past, uncertain); a "wording reverted" note if any. Confirm each item; approve version *n*. Edit one item to show approval is cleared and the version increments; approve again.
5. **Next Step (1:25–1:45).** Preferences: Indonesian, online, free or low cost, weekday evening, barrier *cost*. Show reasons (✓/✗), near matches and non-handoff steps. Choose a fictional slot.
6. **Opening Practice (optional, 1:45–2:00).** Draft lines, then type a practice attempt and show "mentioned / not yet mentioned" (no score).
7. **Share (2:00–2:20).** Select fields; the preview is exactly what the recipient will see; explicit consent sentence names version, recipient and duration; share. Receipt: link + separate access code; "sent ≠ care received".
8. **Recipient (2:20–2:40).** Second window: open the link, try a wrong code (rejected), enter the code, see only the shared fields, acknowledge, complete the scripted consultation with the fixture plan.
9. **Follow-up and revoke (2:40–3:00).** Owner *Perjalananku*: completed (simulated), fixture plan, check-in. Revoke; the recipient's next action fails with "access not available".

Supplementary clip (30 s): the crisis fixture stops the journey before any AI call and shows verified Indonesian help resources with sources; the no-AI path reaches a handoff; with `SELA_LLM_MODE=disabled` the AI button explains it is unavailable.

Record the commit, fixture version, provider mode and model revision with any recording. Never present a test-double or recorded run as live Meta inference.

## 7. Database and migrations (v3)

PostgreSQL 16 is the system of record for `/api/v3` (D-027). The schema is created only by the embedded migrations in `internal/db/migrations/` (`NNNN_name.up.sql`, `.down.sql`, `.validate.sql`; reserved ranges per milestone, ADR-0002 §4). Every command reads `SELA_ENV` and `SELA_DATABASE_URL` and takes a schema-keyed advisory lock, so two runners never interleave. **Every operator command refuses a database whose environment marker (`instance_meta.environment`) names another environment than `SELA_ENV`** — `migrate up`, `dry-run`, `down-last`, `keys status`, `keys rewrap`, `restore reconcile`, `ops canary write|check` exit 4 and change nothing; `migrate status` prints the marker and exits 4. In staging and production the TLS rule (`sslmode=verify-full` for any non-local host) is checked on the effective connection settings, so `PGHOST`, `PGSSLMODE` and libpq service files are covered too.

Database server settings (all environments): keep the server log free of row values with `log_error_verbosity = terse` (drops the `DETAIL` lines of errors, which can quote row values) and do not raise `log_statement` above `ddl` or set `log_min_duration_statement` with parameters (`log_parameter_max_length = 0`). The application already avoids expected errors: a taken username is detected without a failing statement.

Local development (the role `sela` may create schemas and databases):

```sh
export SELA_ENV=development SELA_SECURE_COOKIES=false
export SELA_DATABASE_URL='postgres://sela:sela-dev-only@127.0.0.1:5432/sela_dev?sslmode=disable'
eval "$(./build/sela keys generate-kek --env development | grep '^SELA_')"   # dev key, never reuse elsewhere
export SELA_DATA_KEK SELA_DATA_KEK_ID
./build/sela migrate up            # or SELA_DB_AUTO_MIGRATE=true for schema migrations at start
./build/sela                       # serves /api/v2 and /api/v3
```

| Command | Effect | Exit |
| --- | --- | --- |
| `sela migrate status` | Every embedded and applied version: `applied`, `pending`, `out_of_order`, `checksum_mismatch`, `unknown_in_database`; the `instance_meta` environment marker | 0 current, 3 pending, 4 mismatch, unknown or another environment's database |
| `sela migrate up [--to NNNN] [--backup-ref REF]` | Applies pending migrations in version order, each in its own transaction with its `schema_migrations` row (version, name, kind, checksum, applied_seq, duration, backup_ref, binary version). The first run writes `instance_meta.environment` | 0, 4 on mismatch or another environment's database, 1 otherwise |
| `sela migrate dry-run` | Applies every pending migration and its validate file in **one** transaction, prints durations and validation row counts, rolls back. `no-transaction` migrations cannot be dry-run | 0, 1 when a step fails |
| `sela migrate down-last --confirm NNNN [--backup-ref REF]` | Reverts the most recently applied migration (by `applied_seq`, not the highest number); refuses `irreversible` ones, another environment's database, and a down file that differs from the one recorded when the migration was applied (`schema_migrations.down_checksum`; `status` shows "the down file changed after it was applied" on that row). Then either deploy the binary whose down file was applied, or roll back by restoring the backup; never edit `down_checksum` by hand | 0, 4 on another environment's database or a changed down file, 1 otherwise |

Server start: a checksum mismatch, a version unknown to the binary or a database marked for another environment always stops the server. Pending migrations stop it in demo, staging and production (run `sela migrate up` first); in development and test `SELA_DB_AUTO_MIGRATE=true` applies pending schema migrations. **Data migrations never run at start.** `/api/v2/ready` reports `database` and `schema` and answers 503 when demo, staging or production cannot reach a current database. While the database cannot be reached at run time (refused or lost connections, timeouts, a server restarting or out of connections), every store-backed `/api/v3` route answers `503 store_unavailable` with `Retry-After: 5` and logs `v3.store_unavailable` with the error class only; a rising count of those lines, not of `v3.error`, is the database alarm.

Migration files must not control the transaction: `BEGIN`, `COMMIT`, `END`, `ROLLBACK`, `SAVEPOINT`, `RELEASE`, `START TRANSACTION` and `PREPARE TRANSACTION` are refused in the files of every migration that runs in the runner's transaction (the runner provides the transaction), and a validate file must be exactly one query. The check follows the server's own statement grammar, so a function named `begin`, `end`/`case` used as column labels and SQL-standard (`BEGIN ATOMIC … END`) bodies cannot hide a statement, transaction control inside such a body is refused, and a file that ends inside a body or an open parenthesis is refused. Should a file still end the transaction, or end it and open a new one (`COMMIT; BEGIN;`, caught by comparing the transaction id before and after each file), the runner stops with "the migration ended the runner's transaction" instead of reporting a rollback: compare the database with the backup before retrying.

**Data migration procedure** (`-- sela:kind data`; also any migration that moves user rows): (1) `scripts/ops/backup.sh` and keep the printed backup reference; (2) restore that pair into an isolated scratch database (`scripts/ops/restore-drill.sh --content … --keys …`, never staging or demo) and run `sela migrate dry-run` there, recording the output in the PR; (3) the migration's mandatory validate file must return zero rows (rows whose owner changed or vanished, moved ciphertext, per-owner counts that differ); any row aborts and rolls back; (4) `sela migrate up --backup-ref <reference>`; (5) rollback is the reviewed down file (`down-last --confirm NNNN --backup-ref …`) or, for `irreversible`, a restore of the step-1 backup.

## 8. Backup and restore

**Schedule.** Daily, a full run: `scripts/ops/backup.sh --out-dir DIR` writes, in this order, a **content dump** without `user_data_keys` and a **key dump** with only `user_data_keys` (`pg_dump -Fc`), verifies both with `pg_restore --list`, writes a SHA-256 file and prints the backup reference. **Hourly, a key-only run:** `scripts/ops/backup.sh --keys-only --out-dir DIR` writes a key dump alone (small); it is what lets a restore honour deletions made after the last content dump. Example crontab of the backup job (UTC):

```cron
17 2 * * *  SELA_DATABASE_URL=... /opt/sela/scripts/ops/backup.sh --out-dir /backups/sela --sela /opt/sela/sela
7  * * * *  SELA_DATABASE_URL=... /opt/sela/scripts/ops/backup.sh --out-dir /backups/sela --keys-only
```

`sela backup check --content F --keys F` verifies the split of an existing pair and that the key dump is not older than the content dump. Retention: content dumps 30 days, key dumps 7 days, encrypted at rest by the backup storage in the hosting region (storage target pending, B-3). Meaning for a deleted account: gone from the live database at once, no usable key in any backup after 7 days, ciphertext gone from backups after 30 days; after a total loss of the live database, see "What reconcile cannot see" below.

**Files and secrets.** The scripts run with `umask 077`: the output directory is created `0700`, every dump and checksum file is `0600`, written under a hidden `.partial` name and renamed only once verified, and an existing file is never overwritten (a warning is printed if an existing directory is readable by group or others). Pass the database URL in the environment — `SELA_DATABASE_URL` for `backup.sh`, `SELA_RESTORE_DATABASE_URL` for `restore.sh`, `SELA_DRILL_DATABASE_URL` for `restore-drill.sh` — not as `--database-url` (a process's arguments are visible to every local account; the scripts warn when a password is given that way). Inside, the password is removed from the URL and handed to `pg_dump`, `pg_restore`, `psql` and `sela` through a private temporary `PGPASSFILE` that is deleted on every exit (the cleanup is armed before the file is written, so an early failure such as an output directory that cannot be created or a failed `go build` in the drill removes it too); a libpq service file or your own `PGPASSFILE` works as well (then the URL carries no password). A `password=` URL parameter is refused.

**Restore** only into a fresh scratch database, with the content dump and the **newest key dump** — pass the key dump directory and `restore.sh` picks the newest `sela-keys-*.dump`; if the damaged database is still reachable, first take a fresh key dump from it (`backup.sh --keys-only`):

```sh
export SELA_RESTORE_DATABASE_URL='postgres://…/sela_restore_scratch?sslmode=verify-full'
scripts/ops/restore.sh --content /backups/sela/sela-content-<ts>.dump --keys /backups/sela --dry-run
scripts/ops/restore.sh --content /backups/sela/sela-content-<ts>.dump --keys /backups/sela
```

`--dry-run` checks both archives, that the key dump is not older than the content dump and that no newer key dump lies next to the one given (either is refused: accounts deleted in between would come back), and that the target is empty; without it, the script restores content then keys, each in one transaction, and prints when the key dump was taken. Then, with `SELA_ENV`, `SELA_DATA_KEK`/`_ID` of that environment and `SELA_DATABASE_URL` pointing at the scratch database: `sela migrate status` (must be 0; if it reports pending migrations, run `sela migrate up` first), `sela restore reconcile --dry-run`, `sela restore reconcile`, and `sela ops canary check`. The dry run prints four counts: users whose key is destroyed or missing (a large "missing" count means the key dump is wrong — stop), **keys without a user** (accounts created after the content dump; a large count means the content dump is wrong — stop) and **sessions not ended**. The run then (1) deletes the users with a destroyed or missing key through the deletion path, (2) destroys the keys without a user (the canary's key is kept), so the ciphertext of an account that no longer exists cannot be opened from any copy, and (3) **revokes every session** (reason `restored`): the content dump cannot tell which sessions were revoked after it was taken (logout, logout everywhere, revoke, password change, recovery), so all of them end.

**After a restore, tell users** (in the app notice and the status page, ID and EN; the in-app notice and the status-page post are planned, backlog S-262 criterion 3, and the status page itself is part of S-263, blocked externally (B-3, B-15); both are a gate for "ready for controlled pilot"; until they exist, no restore of a pilot database may happen without a direct notice to every affected user through the app, [D-056](../decisions/DECISION_LOG.md#d-056)): everyone with an account must sign in again; **guests cannot get back in** — a guest's cookie was its only key and its session has ended, so guest data can no longer be opened, and the guest purge deletes it on schedule (at most `SELA_GUEST_INACTIVE_TTL`, 7 days by default, after the last activity in the content dump, or at the guest's maximum age), including guest data deleted in the hour before the failure that the restore brought back (the guest notice `guest-storage-2026-11-01.2` says this; added in the integration pass, confirmed by the PM, [D-056](../decisions/DECISION_LOG.md#d-056); the backend owner may still refine the wording); **password changes, recoveries and recovery-code regenerations made after `<content dump time>` were lost** — the previous password and the previous codes work again — so anyone who changed their password or regenerated codes after that time (for example because a device was lost or a cookie stolen) must do it again now; accounts created after that time no longer exist and must be created again; and, when the live database was lost entirely, an account deleted in the hour before the failure (after the newest key dump) may have come back, so anyone who deleted their account then should sign in and delete it again (below; the privacy text `terms-privacy-2026-11-01.2` promises this announcement). Record the content dump time and the key dump time with the restore.

**What reconcile cannot see.** Reconcile honours the deletions recorded in the key dump it is given. An account deleted after the newest key dump was taken — at most one key-dump interval (one hour) before the failure when the live database is lost entirely — comes back with the restore. Sela keeps no record of deletions outside the key table (deletion leaves no audit event and logs hold no user ids, by design), so such an account cannot be found afterwards: the exposure is bounded by the key-dump interval, and taking key dumps more often shrinks it. A guest deleted in that window comes back too, but nobody can open it (every session was revoked and a guest has no other credential); the guest purge removes it. The privacy text `terms-privacy-2026-11-01.2` states this window as one hour, so while that version is current the key-dump interval must stay at one hour or shorter; a longer interval needs a new policy version first (backlog S-177 criterion 2). This residual window is accepted for a controlled pilot and revisited before a public release; a deletion journal shipped to the backup storage would close it once B-3 decides that storage (D-055). With the key dump of the content dump's own run (not the newest), every deletion since that daily backup would come back; this is why the newest key dump is required.

**Restore drill** (monthly and before every data migration): `SELA_DRILL_DATABASE_URL=<admin URL> scripts/ops/restore-drill.sh` creates a synthetic source database with one user deleted before the backup, one deleted **after** it, one created **after** it and one session revoked **after** it, takes a stale key dump, the daily pair and a later key-only dump, checks that `restore.sh` refuses the stale key dump, restores the content dump with the newest key dump into a scratch database, runs status, reconcile dry-run (must find both deleted users, one key without a user and one session not ended), reconcile (must delete both users, destroy that key and revoke that session) and the canary check, prints the time to restore and drops both databases. Record the date, time to restore and outcome below. Each environment's operator writes the canary once with `sela ops canary write` (a synthetic record sealed under a key row that belongs to no user).

| Date | Environment | Time to restore | Outcome |
| --- | --- | --- | --- |
| 2026-10-26 | local (synthetic drill, PostgreSQL 16.15) | 1 s | ok: status current, reconcile dry-run found the 1 synthetic deleted user, canary ok |
| 2026-10-26 | local (synthetic drill after review round 1, PostgreSQL 16.15) | 1 s | ok: stale key dump refused, newest key dump used, reconcile found and deleted both deleted users (one deleted after the backup), canary ok |
| 2026-10-26 | local (synthetic drill after review round 2, PostgreSQL 16.15) | 0 s | ok: stale key dump refused, newest key dump used, reconcile deleted both deleted users, destroyed the key of the user created after the backup and revoked the restored session, canary ok |
| 2026-10-26 | local (synthetic drill at the foundation commit `0ff3341`, after review round 3, PostgreSQL 16.15) | 1 s | ok: stale key dump refused, newest key dump used, reconcile deleted the 2 deleted users through the deletion path, destroyed 1 key without a user and revoked 1 session, canary ok (run twice on `git archive` exports of the commit, by the documentation critic at about 21:00 UTC and the documentation integrator at about 21:10 UTC, both agents, with the same result; [implementation status §1](../status/IMPLEMENTATION_STATUS.md#1-where-v3-stands)) |

## 9. Data key rotation (KEK)

Each user's data key (DEK) is wrapped by the KEK; the DEK itself never changes, so rotation only re-wraps:

1. `sela keys generate-kek --env <env>` and store the value in the secret manager under a new id (`prod-YYYYMMDD`).
2. Set `SELA_DATA_KEK`/`SELA_DATA_KEK_ID` to the new key and move the old one to `SELA_DATA_KEK_PREVIOUS=<old id>:<old base64>`; restart. The login-throttle secret is derived from the current KEK, so this restart also resets every login backoff and the per-username buckets, and it re-maps every unknown username to a new dummy argon2id tuple: while more than one parameter tuple is stored (after an argon2id change, until accounts re-hash), the failed-login cost of an unknown username may change across the rotation where an existing account's does not. Prefer rotating while one tuple is in use (`SELECT argon2_memory_kib, argon2_iterations, count(*) FROM credentials GROUP BY 1, 2`), unless the rotation is urgent.
3. `sela keys rewrap [--batch 500]` (idempotent and resumable; batches commit separately).
4. `sela keys status` must report `data keys under other KEKs: 0`; `sela ops canary check` must pass.
5. Remove `SELA_DATA_KEK_PREVIOUS` and restart. Keep the old KEK in the secret manager until the key-backup retention (7 days) has passed, then destroy it.

A lost KEK (with its secret-manager copy) makes all content unreadable; KEK custody and its backup are owner responsibilities. A suspected DEK exposure (key table and KEK both leaked) needs a re-encryption tool that does not exist yet (ADR-0002 Consequences).

**Container and deployment note:** since the v3 foundation, `SELA_ENV=production` (the image default) refuses to start without `SELA_DATABASE_URL`, a `prod-…` KEK, Secure cookies and an https `SELA_PUBLIC_ORIGIN`. `deploy/v2/Dockerfile` builds with `golang:1.26.10-alpine` and copies `go.mod` and `go.sum` (D-054); the CI container job checks that the production default refuses to start, then runs the image with `SELA_ENV=development` against a PostgreSQL 16 service and checks health, readiness and the public `/api/v3` routes.

## 10. Operator console, public cache and AI budget

**Separate trust domain.** Operator accounts are provisioned out of band and never through `/api/v3`; an anonymous user cookie cannot authorize `/api/admin/v1`. Staff sign in with a password plus TOTP, and the operator cookie (`__Host-sela_admin`, SameSite=Strict, HttpOnly) is distinct from the user cookie. Roles are deny-by-default; every mutation is appended to PostgreSQL `admin_audit` without personal content.

**Console UI.** Open `/operator` (for example `http://localhost:8081/operator`). The page is mounted outside the consumer shell and shows the overview, AI spend, cache health/invalidation and the audit trail allowed by the signed-in role. It never renders user stories, journals, prompts or per-user data. Owners and security admins also see the staff list and can revoke an operator; the last active owner and the signed-in account cannot be revoked from the UI. Role editing, directory, research and budget-editing screens are still pending.

```sh
# Provision a staff identity; the TOTP secret is printed once.
printf '%s\n' "$OPERATOR_PASSWORD" | sela admin create --username ops.one --role ops
sela admin revoke --username ops.one --reason "left the organisation"

# Sign in (the response sets the operator cookie).
curl -sS -X POST https://sela.example/api/admin/v1/sessions \
  -H 'Content-Type: application/json' -H 'X-Sela-Request: 1' \
  -H 'Origin: https://sela.example' \
  -d '{"username":"ops.one","password":"…","totp":"123456"}'

curl -sS https://sela.example/api/admin/v1/overview -b 'sela_admin=…'
curl -sS -X POST https://sela.example/api/admin/v1/cache/invalidate \
  -H 'X-Sela-Request: 1' -H 'Origin: https://sela.example' -b 'sela_admin=…'
```

Browser acceptance (synthetic staff only; screenshots and axe land in `e2e/results/operator/`):

```sh
E2E_BASE_URL=http://127.0.0.1:8081 \
OPERATOR_USERNAME=ops.ui OPERATOR_PASSWORD='…' OPERATOR_TOTP_SECRET='…' \
node e2e/operator.e2e.mjs
```

**Redis.** The Compose stack runs a disposable, non-persistent Redis (`redis:7-alpine`, no volume, `--save ""`, 64 MB LRU) and points the app at `redis://redis:6379/0`. It caches only the revisioned public help-content catalogue. A miss, an outage or a malformed value falls back to the embedded fixtures; writes are best-effort. Operator cache invalidation deletes only the public help key and writes an audit event. Never put user text, session tokens, consent grants or safety decisions in Redis.

**Planned support demo.** Seed three clearly synthetic directory entries and two fictional sandbox requests (development, test or demo only):

```sh
docker compose exec -T app /sela care seed-demo --admin ops.local
```

Open `/support/directory` for the public sourced search (no login, no location, no booking). The operator console shows the editorial queue and the sandbox for owners, directory editors/reviewers and ops; every sandbox row is labelled **Simulasi**. The directory shows `Synthetic` rows only in development/test/demo. Production and staging refuse the seed command.

**AI budget.** When `SELA_AI_MONTHLY_BUDGET_USD` is positive, every generative listening operation reserves `SELA_AI_RESERVE_USD` in `ai_spend` inside a transaction serialized by an advisory lock, then settles the measured estimate from provenance. A failed operation settles at the reservation, because provider work may already have been billed. Admission failures fail closed: the operation returns `aiState: "budget_limited"` and makes no provider call. Redis is not consulted, so an outage or restart can neither reset nor bypass the cap. The ledger stores task, provider, token counts and cost estimates only — no user id, session id, prompt or reply. The operator overview reports month-to-date committed/actual spend, open reservations and the top tasks; there is no prompt or per-user view by design.
