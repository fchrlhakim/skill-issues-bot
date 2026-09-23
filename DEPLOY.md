# Skill-Issues-Bot — Deployment

Discord marketplace bot + Postgres. Same CD shape as saas/proxy: GHCR image,
Trivy CRITICAL gate, SSH `workflow_dispatch` only (push to `main` does **not**
auto-deploy).

```
deploy/be.Dockerfile      Go API + golang-migrate
deploy/be-entrypoint.sh   migrations, then start
docker-compose.prod.yml   postgres + bot-be
.github/workflows/ci.yml
.github/workflows/cd.yml
```

Project/container names use `skill-issues-bot-*` so this stack never collides
with `skill-issues-saas` or `skill-issues-core` on a shared VPS.

## 1. Host secrets

On the VPS:

```sh
mkdir -p ~/app/skill-issues-bot
cd ~/app/skill-issues-bot
# copy .env.example from the repo, then:
cp .env.example .env.production
chmod 600 .env.production
```

Fill:

| Key | Notes |
|---|---|
| `DB_PASSWORD` | URL-safe (no `@ : / # ? &`) |
| `JWT_SECRET` | ≥ 32 chars |
| `DISCORD_BOT_TOKEN` | Developer Portal → Bot → Reset Token |
| `DISCORD_GUILD_ID` | server ID |
| `GHCR_PULL_TOKEN` | optional; CD falls back to the run token |

Enable **Server Members Intent** on the Discord application.

## 2. GitHub secrets (Actions)

Same set as the other stacks:

- `SSH_HOST`, `SSH_USER`, `SSH_KEY`
- optional `SSH_PORT`, `SSH_PASSPHRASE`, `DEPLOY_PATH` (default `~/app/skill-issues-bot`)
- environment `production` for the deploy job

GHCR packages must be readable by the deploy job (`packages: read`).

## 3. Release

Actions → **CD - Build & Deploy** → Run workflow → `production`.

The host publishes HTTP only on loopback:

| Host | Container | Why |
|---|---|---|
| `127.0.0.1:18102` | `skill-issues-bot-be:8080` | health / debug |
| (not published) | postgres | internal |

Discord traffic is outbound from `bot-be`. No NPM / public hostname required.

## 4. Manual / rollback

```sh
cd ~/app/skill-issues-bot
docker compose -f docker-compose.prod.yml --env-file .env.production ps
curl -sS http://127.0.0.1:18102/api/v1/health/live

# rollback to the previous image recorded by CD:
IMAGE_BACKEND='ghcr.io/fchrlhakim/skill-issues-bot@sha256:…' \
  docker compose -f docker-compose.prod.yml --env-file .env.production up -d
```
