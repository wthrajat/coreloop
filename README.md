# Coreloop

Coreloop is a private, Telegram-first technical curriculum for working
engineers. The web app configures each learner's topics, level, lesson duration,
daily rhythm, weekends, radar, and recall. Complete detailed lessons arrive as
ordered Telegram messages; ranked official engineering updates arrive outside
lesson windows.

## What is implemented

- Responsive Next.js control surface with invite onboarding, overview, settings,
  progress, privacy export/deletion, and owner operations
- Go Vercel Functions behind three entry points: application API, signed jobs,
  and Telegram callbacks
- Telegram OIDC Authorization Code Flow with PKCE, state, nonce, RS256/JWKS
  verification, bot-access scope, opaque sessions, and CSRF protection
- Turso SQL-over-HTTP `database/sql` driver, transactional migrations, seeded
  curriculum catalog, user-owned persistence, and hard-delete cascades
- India-time scheduling with configurable 15/30-minute detailed lessons, one to
  six daily times, weekday/weekend control, and uninterrupted backlog
- Durable chronological jobs, leases, recovery, QStash signature verification,
  and idempotent lesson/part delivery
- Compact versioned lesson contract, strict structured output, semantic
  validation, one corrective request, explicit unverified warning, lesson and
  render caches, and 4,096-character-safe Telegram chunks
- Automatic Groq → Gemini routing. OpenAI is inaccessible to scheduled work and
  can be invoked only by the owner for one explicitly selected blocked job
- Official-feed ingestion with ETag/Last-Modified, deduplication, per-profile
  ranking, skip feedback, provenance, and no daily radar cap
- Content-free quota alerts through Telegram and optional free SMTP, readiness
  checks, owner metrics, CI, security policy, deployment guide, and runbook

## Local verification

Requirements: Node.js 24+, pnpm 11.17+, Go 1.26+, and SQLite 3.

```bash
pnpm install --frozen-lockfile
pnpm run check
pnpm run build
```

The UI can be inspected without integrations with `pnpm run dev`. Authenticated
product routes show the intended unavailable/sign-in state until the database
and Telegram variables are configured.

For a fully connected local API, copy `.env.example` to `.env.local`, fill Turso
and integration values, export it for the Go process, then run:

```bash
set -a; source .env.local; set +a
pnpm run migrate
pnpm run dev:api
# In a second terminal:
pnpm run dev
```

The standalone Go server listens on `http://localhost:8080`; the Next.js app
listens on `http://localhost:3000`. Production serves both from one Vercel
origin. During `next dev`, `/api/*` is proxied to the local Go server.

## Production

Follow [deployment.md](deployment.md). Operational recovery and credential
rotation are documented in [runbook.md](runbook.md), and trust boundaries are
recorded in [SECURITY.md](SECURITY.md). The reviewed upstream API contracts are
linked in [external-contracts.md](external-contracts.md).

No deployment, webhook registration, schedule creation, migration, or provider
call is performed automatically from this repository. Those steps require the
owner's credentials and are deliberately explicit.
