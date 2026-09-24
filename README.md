# Skill Issues Discord bot

This repository is a standalone Discord marketplace for one guild. Business rules are in Go (`modules/ticket`, `modules/membership`, `modules/discordbot`), ported from `skillissue-discord`. The process uses its own Postgres database and its own JWT, and serves HTTP on the process port. Production publishes that HTTP only on `127.0.0.1:18102`. Discord traffic is an outbound gateway only. There is no public hostname.

The bot makes exactly one outbound call to another Skill Issues service: an optional, read-only `GET` of the SaaS admin finance overview, used by `/revenue`. It authenticates with a refresh token (`SAAS_REFRESH_TOKEN`) that it rotates against the SaaS and persists under `storage/`, so the command keeps working past the 15-minute access-token lifetime. It writes nothing to the SaaS, and the two share no database. There is no client to skill-issues-proxy.

## What it does

- The HTTP API under `/api/v1` covers health, auth, users, uploads, audit, tickets, and membership.
- The Discord gateway starts only when both `DISCORD_BOT_TOKEN` and `DISCORD_GUILD_ID` are non-empty (`main.go`). Identify intents are `IntentsGuilds | IntentsGuildMembers` (`New` in `modules/discordbot/bot.go`). Enable **Server Members Intent** on the Discord application.

## Membership

Tiers are exclusive: `user` → `buyer` → `seller`. The Discord roles are `User`, `Buyer`, and `Seller`.

- Verify (`verify:accept` / `POST /api/v1/membership/verify`) grants Buyer and removes User. `DecideVerification` refuses `Suspended`, `Blacklisted`, `Under Review`, a nil role list, and `MemberTier` `conflict` (two of User, Buyer, and Seller, or any legacy role `New Member`, `Verified Seller`, `Provisional Seller`, `Verified Buyer`). Already Buyer or Seller returns action `already` and does not change the tier.
- Seller comes only through admin `/seller-approve` after a seller-verification ticket. `ExclusiveGrant` adds the target role and removes the other two member roles.
- The two marketplace areas are separated in the server, not only in the database: the Seller Area denies `VIEW_CHANNEL` to `@everyone` and `User`, and the Buyer Area does the same, so a Buyer cannot read the Seller channels and a Seller cannot read the Buyer channels.
- `guild_members` stores the tier. It is written on join, on Verify, and on a seller approval. `/membersync` reconciles the stored tier against the roles Discord actually holds and prunes rows for members who left; `/memberpanel` reports the counts read from Discord's roles rather than from the stored column.
- `NeverSelfAssignable` includes staff roles and User, Buyer, and Seller. Language and region roles in `PickableRoles` are the only self-serve picks (`/rolepanel`).
- `YoungAccountDays` is 7. A young account can still verify. The decision sets `FlagYoungAccount`.

## Tickets

There are 17 types and 5 workflows: `legacy`, `withdrawal-v1`, `transaction-v1`, `review-v1`, and `support-v1`.

`MaxOpenTickets` is 3. `TicketTextLimit` is 900.

`GatedTypes` are `withdraw`, `seller-verification`, `campaign`, and `affiliate`.

The following types are copied from `Types` in `modules/ticket/catalog.go`:

| Key | Opener | Workflow |
|---|---|---|
| `purchase` | buyer | `transaction-v1` |
| `sale` | seller | `transaction-v1` |
| `introduction` | any | `transaction-v1` |
| `seller-verification` | buyer | `review-v1` |
| `buyer-support` | buyer | `support-v1` |
| `delivery` | any | `transaction-v1` |
| `refund` | buyer | `review-v1` |
| `dispute` | any | `review-v1` |
| `scam` | any | `review-v1` |
| `impersonation` | any | `review-v1` |
| `regional` | any | `support-v1` |
| `translation` | any | `support-v1` |
| `partnership` | any | `support-v1` |
| `general` | any | `support-v1` |
| `withdraw` | seller | `withdrawal-v1` |
| `campaign` | any | `review-v1` |
| `affiliate` | any | `review-v1` |

Status graphs:

- `legacy` and `transaction-v1` share `legacyStatuses`: `open`, `pending-verification`, `buyer-confirmation`, `seller-confirmation`, `payment-pending`, `payment-confirmed`, `delivery-pending`, `delivery-submitted`, `buyer-review`, `dispute-opened`, `escalated`, `resolved`, `cancelled`, `closed`.
- `withdrawal-v1`: `open` → `approved`, `rejected`, or `cancelled`. `approved` → `rejected` or `cancelled`. `paid` only after `/withdraw-paid`, then `closed`.
- `review-v1` uses `under-admin-review`, `waiting-for-member`, and `escalated`.
- `support-v1` uses `in-progress` and `waiting-for-member`.
- Terminal statuses come from `terminalStatuses()`: `resolved` → `closed`, `cancelled` → `closed`, and `closed` is terminal.

The sensitive-input screen is `ticket.SensitiveFindings` (`modules/ticket/safety.go`). It rejects the ticket text. It does not store the secret.

`SafetyCopy` in `modules/discordbot/commands.go` says: "Admins never ask for passwords, OTP, PIN, CVV, full card numbers, API keys, private keys, or seed phrases. Skillissue.ai is an independent intermediary: it does not hold funds, process payments, or guarantee transactions. Every deal goes through a ticket — never DMs."

## Withdrawals

A withdrawal is a manual ledger. It is never a transfer and never a balance.

- `/withdraw` opens a seller ticket. Approval is not payment. `ApproveWithdrawal` requires `IsWithdrawal` (`type == "withdraw"` and workflow `withdrawal-v1`), no prior approval, and `actorTimeOK`: the actor id is non-empty, the actor id is not the seller id, and the timestamp is RFC3339 with milliseconds.
- `/withdraw-paid` calls `RecordWithdrawalPayment` and inserts `outgoing_mutations`. `reference` is unique, so one sale cannot be recorded twice.
- Amounts are `amountMinor int64`. `ZeroDecimalCurrencies` are JPY, KRW, and VND.
- `/mutasi` shows only that caller's admin-confirmed outgoing records.

## Discord commands

Discord visibility is not authorization. Handlers recheck admin in the `handleCommand` `adminOnly` map, which uses `IsTicketAdmin`.

`Commands` in `modules/discordbot/commands.go` are:

| Name | Admin only | Description |
|---|---|---|
| `panel` | yes | Post the ticket panel in this channel (admin only) |
| `rolepanel` | yes | Post the language and region pickers here (admin only) |
| `verifypanel` | yes | Post the marketplace access panel here (admin only) |
| `testwelcome` | yes | Preview your own welcome card and message (admin only) |
| `tickets` | yes | View the active ticket queue (admin only) |
| `ticket-status` | no | Show your ticket status, assignment, and next steps |
| `ticket-handoff` | yes | Assign this ticket to another human admin (admin only) |
| `ticket-resolution` | yes | Save the member-visible resolution note before closing (admin only) |
| `whoami` | no | Show your roles and access |
| `safety` | no | Read transaction safety rules and prohibited credentials |
| `lookup` | yes | Look up a member's standing and account age (admin only) |
| `withdraw` | no | Request a manual seller withdrawal review; no automatic transfer |
| `mutasi` | no | View only your own admin-confirmed outgoing records |
| `seller-approve` | yes | Approve this seller application and grant Seller (admin only) |
| `withdraw-paid` | yes | Record an already completed external withdrawal payment (admin only) |
| `automodsync` | yes | Create missing launch AutoMod safety rules (admin only, never edits or deletes) |
| `rolesync` | yes | Create missing launch roles (admin only, never deletes) |
| `guildsync` | yes | Create missing launch categories and channels (admin only, never deletes) |
| `memberpanel` | yes | Post the Buyer/Seller split panel with live counts here (admin only) |
| `membersync` | yes | Reconcile stored member tiers with Discord roles and prune departed members (admin only) |
| `server` | yes | Show member and ticket counts for this server (admin only) |
| `revenue` | yes | Show confirmed outgoing ledger totals (admin only) |

## Guild layout

- `/rolesync` calls `syncLaunchRoles`. It creates missing names from `launchRoles()`, never deletes, and skips an existing name and the name `Server Owner`.
- `/guildsync` calls `syncLaunchGuild`. It creates missing categories and channels from `launchLayout()` and never deletes. The six category displays are Start Here, Transaction Support, Seller Area, Buyer Area, Language Areas, and Internal Operations.
- `/automodsync` calls `syncAutoMod`. It creates missing rules from `autoModRules()` and never edits or deletes an existing rule, so manual changes survive. Staff roles are exempt so moderators can quote a scam while handling it. One rule (`Safety · Impersonation Profile`, a `MEMBER_PROFILE` trigger) is refused by Discord with `403 Missing Access` even for a bot holding `ADMINISTRATOR`; the command reports it as refused instead of claiming success. Create that one by hand in Server Settings if Discord enables it for this guild.

## Run locally

From the repository root:

1. Copy the env file: `cp .env.example .env`.
2. Set `JWT_SECRET` to at least 32 characters. To start the gateway, also set `DISCORD_BOT_TOKEN` and `DISCORD_GUILD_ID`. An empty token or an empty guild id leaves HTTP up and does not open Discord.
3. Start Postgres from `docker-compose.yml`: `docker compose --profile postgres up -d`. `make docker-up` runs `docker compose up -d`, which starts the `default` profile: Postgres and Redis. The Postgres service is also on the `postgres` profile.
4. Apply migrations with the path production uses: `migrate -path migrations/postgres -database "$DATABASE_URL" up`. `DATABASE_URL` is a postgres URL for database `starter` (matching `.env.example` `DB_NAME`). For this compose file that URL is `postgres://starter:starter@localhost:5432/starter?sslmode=disable`. Root copies of the SQL exist, but `deploy/be-entrypoint.sh` and `make migrate-up` use `migrations/postgres`.
5. Start the process: `make run` (`go run .`). It listens on `APP_PORT` (`8080` in `.env.example`).
6. Confirm `GET /api/v1/health/live` returns success (`success` is `true`). With the gateway env set, the log line is `discord gateway started`.

## HTTP API

Routes below are registered in `router/router.go` and the `Group*` methods. The prefix is `/api/v1`. Auth is public or JWT.

`operator` means the caller's user id must appear in `OPERATOR_USER_IDS`. Those routes expose every account's email address, the full audit trail, and the ticket/membership surfaces (see below), so they fail closed: with `OPERATOR_USER_IDS` unset they return 403 to everyone.

| Method | Path | Auth |
|---|---|---|
| GET | `/health/live`, `/health/ready` | public |
| GET | `/metrics` | public, and enabled by default (`METRICS_ENABLE=true`); restrict it at the network in production |
| POST | `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout` | public, `auth` limiter |
| GET | `/user` | JWT + operator (`OPERATOR_USER_IDS`) |
| GET | `/user/me` | JWT |
| POST | `/uploads` | JWT |
| GET | `/audit-logs` | JWT + operator (`OPERATOR_USER_IDS`) |
| GET | `/tickets/types` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets` | JWT + operator (`OPERATOR_USER_IDS`) |
| GET | `/tickets` | JWT + operator (`OPERATOR_USER_IDS`) |
| GET | `/tickets/mutasi` | JWT + operator (`OPERATOR_USER_IDS`) |
| GET | `/tickets/whoami` | JWT + operator (`OPERATOR_USER_IDS`) |
| GET | `/tickets/:id` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets/:id/claim` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets/:id/handoff` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets/:id/resolution` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets/:id/status` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets/:id/withdraw-approve` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets/:id/withdraw-paid` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/tickets/:id/seller-approve` | JWT + operator (`OPERATOR_USER_IDS`) |
| POST | `/membership/verify` | JWT + operator (`OPERATOR_USER_IDS`) |
| GET | `/membership/:discord_id` | JWT + operator (`OPERATOR_USER_IDS`) |

The profile route is `/user/me`.

### Why `/tickets` and `/membership` are operator-only

Both surfaces take the caller's identity from the request: `/tickets` builds its
`Actor` (id, roles, `is_admin`, `owner_id`, available admins) from the JSON body,
and `/membership` reads `discord_id` and `roles` from the body. Neither value is
cross-checked against Discord, so an ordinary JWT could assert `is_admin: true`
and drive claims, resolutions, and the withdrawal payment path. The Discord bot
does not use this API - it calls the ticket service in-process - so the whole
group is gated behind the same `OPERATOR_USER_IDS` allowlist as the user
directory and the audit trail. With the allowlist unset, nobody is an operator
and both groups return 403.

## HTTP platform

The process still includes the starter HTTP platform: Gin timeouts, JWT, bcrypt, audit logs, rate limiting, Prometheus, and local upload storage.

## Production

See [DEPLOY.md](DEPLOY.md) for the host setup.

- Container names use the `skill-issues-bot-*` prefix.
- HTTP listens on `127.0.0.1:18102` only.
- CD is `workflow_dispatch` only. The image is `ghcr.io/<owner>/skill-issues-bot`.

## Commands

From the repository root, these Makefile targets exist: `make test`, `make tidy`, `make build`, `make docker-up`, `make docker-down`, and `make migrate-up`.

`make migrate-up` prints the `migrations/postgres` command. It does not apply migrations. Start Postgres with the profile command in [Run locally](#run-locally).
