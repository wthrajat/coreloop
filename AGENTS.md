# Coreloop agent context

## Scope

These instructions apply to the entire repository. Treat this directory as the
project root in every session:

```text
/Users/rajat/dev/plans/coreloop
```

Do not initialize the application in a parent or child directory. Keep
`AGENTS.md` as the only Markdown file at the repository root. Put every other
Markdown document in `docs/` and use paths relative to `docs/` when linking
between documents there.

## Start every new session here

1. Read this file completely.
2. Inspect `git status --short`; the repository may contain user changes.
3. Read only the documents relevant to the task from the documentation map
   below. Product decisions in the numbered documents are intentional.
4. Inspect the implementation before proposing architecture changes.
5. Run the narrowest relevant checks while working, then the full release gates
   before claiming completion.

Never put credentials, Telegram identifiers, invite tokens, database dumps, or
real lesson/user data in source control, tests, screenshots, or logs.

## Product in one paragraph

Coreloop is the final product name, chosen on 2026-08-04, for a private,
invite-only, Telegram-first learning system for working software engineers. Each
person configures topics and delivery settings in a small responsive web app.
The backend continuously plans a coherent curriculum, generates detailed
technical lessons, splits the full text across Telegram messages, sends ranked
official engineering news outside lesson windows, and records lightweight
Read/Skip feedback. The goal is steady daily career development in backend,
cloud, applied AI, product engineering, communication, sales, security,
reliability, and optional Bitcoin topics.

## User context and non-negotiable product decisions

- The owner is an India-based engineer with substantial Bitcoin/Web3 experience
  who wants broader, durable engineering and product skills.
- Available study time is fragmented. Defaults are three detailed 15-minute
  lessons at 08:30, 13:00, and 20:30 in `Asia/Kolkata`, with weekends disabled.
- Profiles can choose 15- or 30-minute lessons, one to six lessons per day,
  exact delivery times, weekend delivery, topics, level, depth, radar, and
  recall settings.
- Friends receive a private invite link, sign in with Telegram, and configure
  independent profiles. There is no email/password signup and no public signup.
- Lessons are English-only, mostly theoretical, technically precise, simple to
  read, and detailed even at 15 minutes. Avoid childish analogies.
- Every lesson should emphasize why the topic exists, the problem it solves,
  prior approaches, real uses, trade-offs, operation, failure modes, practical
  decisions, and likely future direction.
- A theme continues across multiple lessons. The planner stores selections and
  avoids needless topic repetition. Learning runs indefinitely; there is no week
  or course-length cap.
- The complete lesson is delivered through Telegram. The web app is a control
  surface and progress view, not the primary lesson reader.
- Current-tech radar items are separate from lesson windows, ranked, sourced
  from official feeds, and have no user-facing daily cap. Skipping an item
  reduces similar future rankings.
- Read and Skip buttons are deliberately lightweight. Unread backlog never
  blocks future scheduled lessons.
- Jobs are durable, chronological, and first-come-first-served. Do not introduce
  user priority rules without a new explicit product decision.
- Groq is the first automatic AI provider, Gemini is the automatic fallback, and
  OpenAI is never an automatic fallback. Only the owner may explicitly use
  OpenAI for one selected quota-blocked job.
- If generated content fails validation, issue one corrective request. If the
  second result is usable but not fully verified, deliver it with an explicit
  warning; information takes precedence over cosmetic structure.
- Optimize AI use with stable compact prompts, small dynamic payloads,
  deterministic rendering, content caching, and no unnecessary profile or
  conversation history in model calls.
- The operating target is zero mandatory spend: Vercel Hobby, Turso Free, QStash
  Free, Telegram Bot API, and free Groq/Gemini quotas. Quota exhaustion is an
  error/blocked state, not authorization to buy capacity.

## Current implementation status

The end-to-end repository implementation was completed locally on 2026-08-03.
The application is deploy-ready, but live acceptance with the owner's real
Turso, Telegram, QStash, Vercel, Groq, Gemini, and optional OpenAI credentials
has not been performed. Do not claim that external integrations are verified
until those checks have actually run.

Implemented areas include:

- responsive public, onboarding, overview, settings, progress, profile, and
  owner-operations interfaces;
- Telegram OIDC Authorization Code Flow with PKCE, state, nonce, RS256/JWKS,
  opaque sessions, CSRF protection, and single-use invites;
- a pure-Go Turso SQL-over-HTTP `database/sql` driver, ordered migrations, and a
  seeded topic/source catalogue;
- curriculum planning, prompt compilation, structured lesson generation,
  semantic validation, correction retry, caches, and provider run records;
- durable job expansion, QStash wake-ups, leases, retry/recovery, quota states,
  and idempotent Telegram bundle delivery;
- official RSS/Atom source ingestion, HTTP cache validators, deduplication,
  ranking, provenance, and per-profile feedback;
- privacy export, destructive profile deletion, operational metrics, optional
  SMTP alerts, health/readiness endpoints, CI, and security documentation.

The exact evidence and implementation boundaries are in
`docs/012-implementation-log.md`.

## Architecture

| Concern        | Implementation                                  |
| -------------- | ----------------------------------------------- |
| Web UI         | Next.js App Router, React, TypeScript           |
| Backend        | Go packages exposed through Vercel Go Functions |
| Vercel entries | `api/app`, `api/jobs`, `api/telegram`           |
| Database       | Turso/libSQL through `/v2/pipeline`             |
| Authentication | Telegram OIDC plus application-owned sessions   |
| Delivery       | Telegram Bot API messages and callback buttons  |
| Scheduler      | One QStash schedule calling `/api/jobs/tick`    |
| Durable queue  | Turso job rows; QStash only wakes workers       |
| AI routing     | Groq, then Gemini; manual owner-only OpenAI     |
| Time zone      | `Asia/Kolkata`                                  |
| Hosting        | One Vercel Hobby project                        |

Important boundaries:

- Keep product/business logic out of Next.js pages. The browser talks to the Go
  application API.
- Keep the three public Go function entry points small; ordinary packages under
  `backend/internal/` own behavior.
- Turso is the source of truth for jobs, leases, provider state, deliveries,
  user configuration, and curriculum history. Never rely on an in-memory queue.
- QStash and Telegram requests must be authenticated before processing.
- Provider output remains untrusted even with JSON-schema mode; application
  validation is mandatory.
- Shared cached lessons must not contain user identity or private profile data.
- Account deletion hard-deletes the user and cascades private records. Do not
  weaken this behavior accidentally.

## Repository map

```text
app/                         Next.js routes
components/                  shared UI and product shell
lib/                         browser-safe API/types/defaults
api/app/index.go             web API Vercel function
api/jobs/index.go            signed QStash Vercel function
api/telegram/index.go        Telegram webhook Vercel function
backend/app/                 composition root used by entry points
backend/internal/auth/       invites, OIDC, sessions
backend/internal/config/     runtime configuration and production validation
backend/internal/content/    lesson schema, prompts, validation, rendering
backend/internal/database/   Turso driver and migration runner
backend/internal/jobs/       tick, dispatch, generation, delivery, radar
backend/internal/providers/  Groq, Gemini, and OpenAI adapters/router
backend/internal/qstash/     publishing and signature verification
backend/internal/radar/      official feed parsing and safe fetching
backend/internal/store/      SQL persistence methods
backend/internal/telegram/   Bot API, chunking, and delivery
backend/cmd/migrate/         production migration CLI
backend/cmd/admin/           invite and webhook administration CLI
cmd/api/                     local combined Go server
migrations/                  embedded ordered SQL migrations
tests/                       frontend/default behavior tests
docs/                        all project/product/operations Markdown
```

## Documentation map

- `docs/README.md`: product overview and local development.
- `docs/deployment.md`: complete beginner-oriented credential and deployment
  procedure. Keep this accurate when environment variables or services change.
- `docs/runbook.md`: failure recovery and credential rotation.
- `docs/SECURITY.md`: trust boundaries and secret handling.
- `docs/external-contracts.md`: primary upstream API references and review date.
- `docs/001-context-and-goals.md`: owner career and learning context.
- `docs/003-product-requirements.md`: intended user behavior and requirements.
- `docs/004-content-and-curriculum.md`: learning lanes and content quality.
- `docs/005-system-design.md`: detailed system/data design.
- `docs/010-grilling-decisions.md`: final decisions from product questioning;
  this supersedes conflicting earlier ideas.
- `docs/011-implementation-plan.md`: selected implementation plan.
- `docs/012-implementation-log.md`: what exists and what has been verified.
- `docs/DESIGN.md` and `docs/PRODUCT.md`: UI and product principles.
- `docs/theme.md`: the original flat, color-block theme input. The product-ready
  adaptation and responsive rules are recorded in `docs/DESIGN.md`.

When documents conflict, prefer the newest explicit decision, then the running
implementation. Preserve historical documents unless the user asks to rewrite
history; add a supersession note instead.

## Runtime configuration

`.env.example` is the canonical variable inventory. Production requires:

- application: `APP_ENV`, `APP_ORIGIN`, `NEXT_PUBLIC_APP_ORIGIN`,
  `APP_TIME_ZONE`, `SESSION_SECRET`;
- Turso: `TURSO_DATABASE_URL`, `TURSO_AUTH_TOKEN`;
- Telegram: `TELEGRAM_CLIENT_ID`, `TELEGRAM_CLIENT_SECRET`,
  `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `OWNER_TELEGRAM_SUBJECT`;
- QStash: `QSTASH_CURRENT_SIGNING_KEY`, `QSTASH_NEXT_SIGNING_KEY`,
  `QSTASH_TOKEN`;
- at least one free provider: `GROQ_API_KEY` or `GEMINI_API_KEY`.

The three model variables have reviewed defaults. OpenAI and SMTP variables are
optional. Never add a secret to a `NEXT_PUBLIC_*` variable.

## Commands

Requirements: Node.js 24+, pnpm 11.17+, Go 1.26+, and SQLite 3.

The repository pins `pnpm@11.17.0` in `package.json` and commits
`pnpm-lock.yaml`. Vercel must have `ENABLE_EXPERIMENTAL_COREPACK=1` so its build
uses the pinned pnpm version.

```bash
pnpm install --frozen-lockfile
pnpm run dev             # Next.js on :3000
pnpm run dev:api         # local Go API on :8080, separate terminal
pnpm run migrate         # apply embedded migrations to configured Turso
pnpm run admin invite --ttl 24h
pnpm run admin telegram-webhook
```

Full release gates:

```bash
pnpm run check
pnpm run format:check
GOCACHE=$PWD/.cache/go-build GOENV=off go vet ./api/... ./backend/... ./cmd/... ./migrations/...
pnpm run build
```

Run `pnpm audit --prod --audit-level high` in a networked environment. CI also
runs it. If registry access is unavailable, report that limitation instead of
claiming the audit passed.

## Implementation rules

- Preserve existing user changes and keep diffs focused.
- Prefer clear, short, single-purpose Go functions and small React components.
- Reuse the existing store, error, response, session, and provider patterns.
- Use parameterized SQL and maintain transaction/idempotency boundaries.
- Add schema changes as a new ordered migration; never edit an already-applied
  migration unless the user explicitly confirms no environment has used it.
- Add or update tests for changed behavior. Test failure, retry, quota, and
  idempotency paths when touching external integrations.
- Keep source fetching restricted to intended official HTTPS sources and retain
  SSRF/redirect protections.
- Keep Telegram messages HTML-safe and below the platform limit. Persist each
  delivered part before advancing.
- Keep mutations protected by session authentication, exact-origin checks, and
  CSRF validation. Owner endpoints also require the configured Telegram subject.
- Never make OpenAI part of scheduled automatic failover.
- Do not introduce billing, paid infrastructure, public signup, email auth,
  gamification, childish teaching language, or a separate lesson web reader
  without a new explicit user decision.

## Deployment and continuation handoff

Use `docs/deployment.md` for the actual deployment. Deployment changes external
state and uses private credentials, so do not deploy, create accounts, register
webhooks, create schedules, migrate a live database, or create invitations
unless the user explicitly asks in that session.

At the end of future substantial work, update `docs/012-implementation-log.md`
with the outcome and actual evidence. If a product or architecture decision
changes, update the relevant numbered document and this file so a new session
does not resurrect superseded assumptions.
