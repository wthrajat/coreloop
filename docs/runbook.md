# Operations runbook

## Health and readiness

`/api/app/health` proves the function is running. `/api/app/ready` also checks
Turso and requires migration version 3. Neither response exposes credentials.

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

Ticks recover leases after four minutes. Job, bundle, and part idempotency keys
make repeated QStash calls safe. A Telegram part is persisted after its message
ID returns. Retrying a partial bundle starts with the first undelivered part.

If Telegram rejects delivery, confirm the destination is still connected and the
bot has permission to message it. Do not manually edit delivery rows unless you
have first exported the affected records.

## Source failures

Official feeds use `ETag` and `Last-Modified`; unchanged feeds return 304 and do
not create radar work. A failed poll increments `consecutive_failures` and is
retried by its normal interval. The current source catalog is deliberately small
and can be changed through a migration.

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
