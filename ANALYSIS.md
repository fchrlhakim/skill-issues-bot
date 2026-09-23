# skill-issues-bot — deep dive (2026-09-23)

The module lives at the working copy root.
Source: `github.com/fchrlhakim/skill-issues-bot`, branch `main`, pushed 2026-09-23T07:00Z,
111 files, ~310 KB, primary language Go.

## What it actually is

Not a helper script — a **third, standalone Discord product**, unrelated to the two
Discord integrations already in the SaaS repo.

| | SaaS `sellerkyc` | SaaS `identity` | **this bot** |
|---|---|---|---|
| Role | gate seller onboarding | link buyer account | **whole Discord marketplace** |
| Transport | REST (bot token) | OAuth/OIDC | **Discord gateway (WebSocket)** |
| Needs intents | no | no | **yes — Server Members Intent** |
| Storage | SaaS Postgres | SaaS Postgres | **its own Postgres** |
| Auth | SaaS JWT | SaaS JWT | **its own JWT** |
| HTTP | SaaS API | SaaS API | **loopback `127.0.0.1:18102`** |

## Its integration with saas / proxy is one optional read

Originally this section claimed "ZERO integration"; that is no longer true, and
this paragraph is the correction. The sweep for `skill-issues.dev`, `saas`,
`proxy`, `core.skill`, `GATEWAY`, `API_KEY` used to hit only prose in `DEPLOY.md`
and `docker-compose.prod.yml`. The bot now also calls the SaaS admin finance
overview from `/revenue` (`SAAS_REVENUE_URL`, `SAAS_REVENUE_TOKEN`): a read-only
`GET` that writes nothing. The rest still holds — no shared database, no client
to skill-issues-proxy, and no shared secret beyond that bearer token. With either
variable empty, the command degrades to `SaaS revenue: unavailable`.

## Business rules (Go owns them; a Node project did before)

- **Exclusive tiers**: `user` → `buyer` (via Verify) → `seller` (admin `/seller-approve`).
  `ExclusiveGrant` grants one and removes the others; `NeverSelfAssignable` blocks
  self-promotion to staff roles.
- **17 ticket types, 5 workflows** (`legacy`, `withdrawal-v1`, `transaction-v1`,
  `review-v1`, `support-v1`). `GatedTypes` = withdraw, seller-verification, campaign, affiliate.
- **Max 3 open tickets** per member; `TicketTextLimit` 900 chars.
- **Sensitive-input screen** (`safety.go`): regex screen for private keys, card numbers
  (Luhn-ish digit run), labelled secrets — rejects the ticket if found.
- **Withdrawal is a manual ledger, never a transfer.** `WithdrawalNotice` says it outright.
  `/withdraw-paid` records one *already completed* external payment.
  `ApproveWithdrawal` requires `IsWithdrawal(type, workflow)`, status `open`,
  no prior approval, and an actor that is **not the opener** (`actorTimeOK`), with an
  RFC3339-millisecond timestamp. `RecordWithdrawalPayment` additionally cross-checks
  `others []Record` for a duplicate reference — so the same sale cannot be paid twice.
- Money uses **minor units** (`amountMinor int64`), with `ZeroDecimalCurrencies`
  for JPY/KRW/VND. No floats.

## The duplication problem — the real decision you face

Two independent systems both answer "is this Discord user a guild member?":

1. **SaaS `sellerkyc`** — REST check via bot token, writes `seller_kyc_profiles`.
   Gates whether a seller may list on the SaaS marketplace.
2. **This bot** — gateway events, writes `guild_members.tier`. Gates Discord-side
   roles and tickets.

They share no data. A seller could be `approved` in SaaS and still be `user` tier in
the bot, or vice versa. **Nothing reconciles them.** Decide which is the source of
truth before running both — otherwise you have two answers to one question, which is
the same defect class that already bit this codebase elsewhere.

## Deploy shape

- Containers/`skill-issues-bot-*`; HTTP loopback-only on `127.0.0.1:18102`; Postgres not published.
- Discord traffic is **outbound only** — no NPM hostname needed, no public port.
- CD is `workflow_dispatch` only; push to `main` does not deploy.
- Needs `DB_PASSWORD`, `JWT_SECRET` (≥32), `DISCORD_BOT_TOKEN`, `DISCORD_GUILD_ID`,
  and optional `GHCR_PULL_TOKEN`.
- Migrations `000001_create_users`, `000002_discord_marketplace` (guild_members,
  tickets, ticket_counters). Postgres + MySQL variants both shipped.

## Discord portal requirements — differs from what I told you earlier

Earlier I said "privileged intents not needed". That was correct **for the SaaS KYC
check** (it calls REST directly). It is **wrong for this bot**:

| Application setting | SaaS KYC | Buyer identity | This bot |
|---|---|---|---|
| Redirect URI | yes | yes | **not needed** |
| Bot user | yes | no | **yes** |
| Privileged intents | no | no | **Server Members Intent: ON** |

So: one Discord application can serve all three, but it must have
**Server Members Intent enabled** (Bot → Privileged Gateway Intents → Server Members Intent),
otherwise `onJoin`/`onLeave`/member lookups silently fail.

## Notes / caveats

- `README.md` is the product overview for this Discord marketplace.
- `AGENTS.md` keeps the architecture rules and names `modules/ticket`, `modules/membership`, and `modules/discordbot`.
- `modules/upload` is present, wired in `boot/boot.go` and `router/router.go`, and is starter storage rather than marketplace domain.
- Vendored business rules came from a Node project (`skillissue-discord`); the Go port
  is now the authority.
