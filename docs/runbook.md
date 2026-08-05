# Operations runbook

## Health and readiness

`/api/app/health` proves the function is running. `/api/app/ready` also checks
Turso and requires every migration through version 7. Neither response exposes
credentials.

The owner Operations page shows queued, leased, failed, and quota-blocked jobs,
active users, sources, chronological blocked job IDs, and explicit one-job
OpenAI controls.

## AI quota exhaustion

Scheduled generation tries Groq, then Gemini. If both are unavailable or out of
quota, the job enters `blocked_quota` with a one-hour retry time. The affected
user receives one Telegram notice per day; the owner receives a content-free
SMTP alert when configured. No paid provider is selected automatically.

The next tick returns due blocked jobs to the same first-come queue. To spend
OpenAI credits for one job, use the owner Operations page and confirm the
browser prompt. If the OpenAI account has no balance, the request fails without
changing the free-provider boundary.

## Stalled or duplicate delivery

Ticks recover leases after four minutes. Durable job, bundle, and part keys make
database work safe to retry. Telegram itself has no message-idempotency key: a
process crash after Telegram accepts a message but before Coreloop records the
returned message ID can produce one duplicate. Retrying a normal partial bundle
starts with the first part not recorded as delivered.

If Telegram rejects delivery, confirm the destination is still connected and the
bot has permission to message it. Do not manually edit delivery rows unless you
have first exported the affected records.

## Source failures

Feeds, sitemaps, GitHub releases, Hacker News, and curated Bluesky accounts use
source-specific adapters. Conditional requests retain `ETag` and
`Last-Modified`; unchanged sources do not create ranking work. A failed poll
increments only that source's `consecutive_failures` and is retried by its
normal interval. The curated catalog is changed through an ordered migration.

Radar selection, source attribution, neutral rendering, chunking, and Telegram
delivery do not require AI. Groq or Gemini may optionally simplify one briefing,
but quota exhaustion, invalid output, timeouts, and missing provider
configuration are logged as enrichment misses and immediately use the
deterministic source text. Do not requeue or edit a Radar job solely because an
AI enrichment was unavailable.

If Radar stops while lessons continue, inspect `ingest_source`, `rank_radar`,
and `deliver_radar` job states in chronological order. Confirm every migration
through version 7 is applied, the destination is connected, and Radar is
enabled. The default is eight updates spread across the user's local day,
including weekends; a value of zero means unlimited. A finite target can
legitimately leave fresh candidates pending until the next interval.

## Credential rotation

- QStash: set the new key as `NEXT`, deploy, rotate upstream, then promote it to
  `CURRENT` and deploy again.
- Session secret: rotate and expect every session and outstanding invite to
  become invalid.
- Telegram webhook secret: update Vercel, deploy, then rerun the webhook admin
  command.
- Database and provider tokens: rotate upstream, update Vercel, and redeploy.

## Backups and deletion

Use Turso's free-plan backup/export facilities before schema changes. Profile
deletion is intentionally destructive: the user row is hard-deleted and private
rows cascade. The browser asks for confirmation, but there is no application
undo.
