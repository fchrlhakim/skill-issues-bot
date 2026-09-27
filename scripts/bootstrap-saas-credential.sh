#!/usr/bin/env bash
#
# Bootstrap the SaaS credential behind skill-issues-bot's /revenue command.
#
# /revenue calls GET /api/v1/admin/reports/finance-overview on skill-issues-saas
# and authenticates with a refresh token that it rotates itself (see
# modules/discordbot/saas_token.go). SaaS revokes the ENTIRE token family when an
# already-rotated token is presented again (ErrRefreshTokenReuse), so a lost,
# replayed, or raced chain cannot be repaired by retrying: the family is
# permanently dead and the only fix is a fresh bootstrap. This script does that
# bootstrap.
#
# It mints a refresh JWT with the SaaS signing secret, registers the matching
# auth_sessions and refresh_tokens rows, installs the JWT as SAAS_REFRESH_TOKEN,
# and deletes the bot's persisted chain so the bot cannot replay the dead one.
#
# Run it ON THE APPLICATION HOST: it needs docker, sudo, and both
# .env.production files. It never prints a secret.
#
#   scripts/bootstrap-saas-credential.sh --dry-run
#   scripts/bootstrap-saas-credential.sh
#   scripts/bootstrap-saas-credential.sh --admin-user-id <uuid>
#
# Both expiry clocks are aligned deliberately. The JWT `exp` claim and the
# auth_sessions row must BOTH be live for a token to work, and whichever expires
# first is the real limit. This script derives both from the SaaS
# JWT_REFRESH_TTL so they cannot silently disagree.
set -euo pipefail

SAAS_ENV=${SAAS_ENV:-/home/skill-issues/app/skill-issues-saas/.env.production}
BOT_ENV=${BOT_ENV:-/home/skill-issues/app/skill-issues-bot/.env.production}
BOT_VOLUME_FILE=${BOT_VOLUME_FILE:-/var/lib/docker/volumes/skill-issues-bot_storage/_data/saas-token.json}
PG_CONTAINER=${PG_CONTAINER:-skill-issues-saas-pg}
PG_USER=${PG_USER:-starter}
PG_DB=${PG_DB:-starter}
DEVICE_LABEL=${DEVICE_LABEL:-discord-bot}
SUDO=${SUDO:-sudo}

DRY_RUN=0
ADMIN_USER_ID=""

usage() { sed -n '3,26p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-0}"; }
log()  { printf '  %s\n' "$*"; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run)       DRY_RUN=1 ;;
    --admin-user-id) ADMIN_USER_ID=${2:?--admin-user-id needs a value}; shift ;;
    -h|--help)       usage 0 ;;
    *)               die "unknown argument: $1 (try --help)" ;;
  esac
  shift
done

# psql with the password never entering argv.
psql_q() {
  $SUDO docker exec -i "$PG_CONTAINER" \
    psql -U "$PG_USER" -d "$PG_DB" -tAq -v ON_ERROR_STOP=1 -c "$1"
}

env_value() { $SUDO sed -n "s/^$2=//p" "$1" | tail -n1; }

command -v python3 >/dev/null || die "python3 is required to mint the JWT"
[ -r "$SAAS_ENV" ] || $SUDO test -r "$SAAS_ENV" || die "cannot read $SAAS_ENV"
$SUDO docker inspect "$PG_CONTAINER" >/dev/null 2>&1 \
  || die "container $PG_CONTAINER not found; set PG_CONTAINER"

JWT_SECRET=$(env_value "$SAAS_ENV" JWT_SECRET)
[ -n "$JWT_SECRET" ] || die "JWT_SECRET is empty in $SAAS_ENV"
REFRESH_TTL=$(env_value "$SAAS_ENV" JWT_REFRESH_TTL)
[ -n "$REFRESH_TTL" ] || REFRESH_TTL=720h

# Turn 720h / 30d / 60m into whole days for the SQL interval.
TTL_DAYS=$(python3 - "$REFRESH_TTL" <<'PY'
import re, sys
raw = sys.argv[1].strip()
m = re.fullmatch(r'(\d+)([smhd])', raw)
if not m:
    sys.exit(f"unparsable JWT_REFRESH_TTL: {raw!r}")
n, unit = int(m.group(1)), m.group(2)
secs = {'s': 1, 'm': 60, 'h': 3600, 'd': 86400}[unit] * n
print(max(1, secs // 86400))
PY
) || die "could not parse JWT_REFRESH_TTL=$REFRESH_TTL"

if [ -z "$ADMIN_USER_ID" ]; then
  ADMIN_USER_ID=$(psql_q "select user_id from auth_sessions where device_label='$DEVICE_LABEL' order by created_at desc limit 1")
  [ -n "$ADMIN_USER_ID" ] \
    || die "no existing '$DEVICE_LABEL' session to copy the admin from; pass --admin-user-id"
  log "admin user taken from the newest '$DEVICE_LABEL' session"
fi

psql_q "select 1 from users where id='$ADMIN_USER_ID'" | grep -q 1 \
  || die "user $ADMIN_USER_ID does not exist"

echo "Bootstrap plan"
log "saas env        : $SAAS_ENV"
log "bot env         : $BOT_ENV"
log "admin user      : $ADMIN_USER_ID"
log "device label    : $DEVICE_LABEL"
log "both clocks     : now + $TTL_DAYS days (from JWT_REFRESH_TTL=$REFRESH_TTL)"
log "persisted chain : $BOT_VOLUME_FILE (will be deleted)"

if [ "$DRY_RUN" = 1 ]; then
  echo "dry run: nothing changed"
  exit 0
fi

# Mint. The secret goes through the environment, never argv, so it cannot be
# read out of the process table.
SID=$(python3 -c 'import uuid;print(uuid.uuid4())')
FAM=$(python3 -c 'import uuid;print(uuid.uuid4())')
TOKEN=$(JWT_SECRET="$JWT_SECRET" SID="$SID" UID_="$ADMIN_USER_ID" DAYS="$TTL_DAYS" python3 - <<'PY'
import base64, hashlib, hmac, json, os, time, uuid

secret = os.environ["JWT_SECRET"].encode()

def b64(raw):
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()

now = int(time.time())
header = b64(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
payload = b64(json.dumps({
    "user_id": os.environ["UID_"],
    "session_id": os.environ["SID"],
    "sub": "refresh",
    "exp": now + int(os.environ["DAYS"]) * 86400,
    "iat": now,
    "jti": str(uuid.uuid4()),
}, separators=(",", ":")).encode())
signature = b64(hmac.new(secret, f"{header}.{payload}".encode(), hashlib.sha256).digest())
print(f"{header}.{payload}.{signature}")
PY
)
TOKEN_HASH=$(printf '%s' "$TOKEN" | sha256sum | cut -d' ' -f1)
[ -n "$TOKEN" ] || die "minting produced an empty token"

# Supersede any live session for this label before adding the new one, so an
# orphaned chain cannot linger and be replayed.
psql_q "update auth_sessions set revoked_at=now(), revocation_reason='superseded_by_bootstrap', updated_at=now()
        where device_label='$DEVICE_LABEL' and revoked_at is null" >/dev/null

psql_q "insert into auth_sessions (id, user_id, device_label, last_seen_at, expires_at)
        values ('$SID', '$ADMIN_USER_ID', '$DEVICE_LABEL', now(), now() + interval '$TTL_DAYS days')" >/dev/null

psql_q "insert into refresh_tokens (id, user_id, session_id, family_id, token_hash, expires_at)
        values (gen_random_uuid(), '$ADMIN_USER_ID', '$SID', '$FAM', '$TOKEN_HASH', now() + interval '$TTL_DAYS days')" >/dev/null

# Install into the bot env. The file is root-owned, so rewrite through python.
$SUDO python3 - "$BOT_ENV" "$TOKEN" <<'PY'
import pathlib, re, sys

path, token = pathlib.Path(sys.argv[1]), sys.argv[2]
text = path.read_text()
line = f"SAAS_REFRESH_TOKEN={token}"
if re.search(r"^SAAS_REFRESH_TOKEN=.*$", text, re.M):
    text = re.sub(r"^SAAS_REFRESH_TOKEN=.*$", line, text, flags=re.M)
else:
    text = text.rstrip("\n") + "\n" + line + "\n"
path.write_text(text)
PY
$SUDO chmod 600 "$BOT_ENV"

# A stale chain on the volume is fatal: the bot prefers the file over the env and
# would replay the token the SaaS just revoked.
$SUDO rm -f "$BOT_VOLUME_FILE"

echo "Bootstrapped"
log "session         : ${SID:0:8}"
log "family          : ${FAM:0:8}"
log "SAAS_REFRESH_TOKEN written to $BOT_ENV"
log "persisted chain cleared"
echo
echo "Restart the bot to pick it up:"
echo "  cd $(dirname "$BOT_ENV") && sudo docker compose -f docker-compose.prod.yml --env-file .env.production up -d bot-be"
echo "Then confirm no 'refresh rejected' warning appears in:"
echo "  sudo docker logs --since 2m skill-issues-bot-be"
