# Coreloop

Coreloop is a private, self-hosted learning system that delivers technical
lessons through Telegram. Each learner chooses what to study and when to receive
it from a small web app; Coreloop plans a coherent curriculum, generates the
lessons, and sends the complete material as Telegram messages.

It is invite-only, supports independent profiles, and is designed to run on free
service tiers without maintaining a traditional server.

#### Overview

![Coreloop overview](assets/images/dashboard.png)

#### Onboarding and Configs

![Coreloop onboarding and learning configuration](assets/images/onboarding.png)

## What it does

- Delivers detailed 15- or 30-minute lessons on a configurable daily schedule.
- Builds multi-lesson themes and remembers recent coverage to avoid needless
  repetition.
- Lets each learner choose topics, level, explanation depth, recall settings,
  weekends, and exact delivery times.
- Sends ranked updates from official engineering feeds outside lesson windows.
- Records lightweight **Read** and **Skip** feedback without blocking future
  lessons when a backlog exists.
- Provides progress, profile, privacy, and owner-operations views in a
  responsive web interface.
- Uses Telegram OpenID Connect for authentication; there is no password database
  or public signup.

## How it is built

| Area                        | Technology                                       |
| --------------------------- | ------------------------------------------------ |
| Web interface               | Next.js App Router, React, and TypeScript        |
| API and workers             | Go functions deployed with the web app on Vercel |
| Database and durable queue  | Turso/libSQL                                     |
| Authentication and delivery | Telegram Login and Bot API                       |
| Scheduling                  | Upstash QStash                                   |
| Lesson generation           | Groq first, Gemini fallback                      |
| Optional manual fallback    | OpenAI, for one owner-selected blocked job only  |

The frontend and Go backend deploy as one Vercel project. You do **not** need to
deploy the Go API separately. Vercel builds the functions under `api/`, while
`cmd/api` provides a combined API server for local development.

## Local development

You need `Node.js 24+`, `pnpm 11.17+`, `Go 1.26+`, and `SQLite 3`.

```bash
corepack enable
corepack install --global pnpm@11.17.0
pnpm install --frozen-lockfile
```

To inspect the public interface and signed-out states:

```bash
pnpm run dev
```

For a connected local environment, copy the environment template and fill in
your service credentials:

```bash
cp .env.example .env.local
bash -c 'set -a; source .env.local; set +a; exec pnpm run migrate'
```

Run the API and web app in separate terminals:

```bash
bash -c 'set -a; source .env.local; set +a; exec pnpm run dev:api'
```

```bash
pnpm run dev
```

The web app runs on `http://localhost:3000` and proxies `/api/*` to the Go API
on `http://localhost:8080` during development.

## Self-hosting

Coreloop's reference deployment uses one Vercel project, one Turso database, one
Telegram bot, one QStash schedule, and at least one supported AI provider. The
steps below use Vercel, but the application can be adapted to another host that
can serve Next.js and the three Go HTTP entry points.

### 1. Create the service accounts

Prepare the following:

- a GitHub repository and Vercel project;
- a Turso database with a read-write database token;
- a Telegram bot with Web Login enabled;
- an Upstash QStash account;
- a Groq or Gemini API key (both are recommended for automatic fallback).

OpenAI and SMTP are optional. Scheduled work never falls back to OpenAI
automatically.

### 2. Establish the production URL

Import the repository into Vercel as a Next.js project and add this build
environment variable:

```text
ENABLE_EXPERIMENTAL_COREPACK=1
```

Deploy once to establish a stable production URL, such as
`https://your-coreloop.vercel.app`. Use this exact origin everywhere below,
without a trailing slash. Preview URLs are unsuitable for the production
Telegram configuration because they change between deployments.

### 3. Configure Turso

Create a database from the Turso dashboard. From its **Connect** panel, copy:

- the HTTPS database URL to `TURSO_DATABASE_URL`;
- a read-write database token to `TURSO_AUTH_TOKEN`.

The application runs migrations through Turso's HTTP API, so installing the
Turso CLI is optional.

### 4. Configure Telegram

Create a bot with the verified `@BotFather`, then open its **Web Login**
settings. Configure:

```text
Trusted origin: https://your-coreloop.vercel.app
Redirect URI:   https://your-coreloop.vercel.app/api/app/auth/callback
```

Keep the signing algorithm set to **RS256**. Save the Bot API token, Web Login
client ID, and Web Login client secret for the environment configuration.

Generate independent secrets for application sessions and Telegram webhook
verification:

```bash
openssl rand -hex 32
openssl rand -hex 32
```

### 5. Configure QStash and AI providers

From the QStash console, copy the API token plus its current and next signing
keys. Create a Groq API key, a Gemini API key, or both. The model names in
`.env.example` are the application's reviewed defaults and normally should not
be changed.

### 6. Set the production environment

Copy `.env.example` to an ignored local production file:

```bash
cp .env.example .env.production.local
git check-ignore .env.production.local
```

Set the following required values locally and in Vercel's **Production**
environment:

| Variable                           | Value                            |
| ---------------------------------- | -------------------------------- |
| `APP_ENV`                          | `production`                     |
| `APP_ORIGIN`                       | Exact production HTTPS origin    |
| `NEXT_PUBLIC_APP_ORIGIN`           | Same production origin           |
| `APP_TIME_ZONE`                    | `Asia/Kolkata`                   |
| `SESSION_SECRET`                   | First generated secret           |
| `TURSO_DATABASE_URL`               | Turso HTTPS URL                  |
| `TURSO_AUTH_TOKEN`                 | Turso read-write token           |
| `TELEGRAM_CLIENT_ID`               | Telegram Web Login client ID     |
| `TELEGRAM_CLIENT_SECRET`           | Telegram Web Login client secret |
| `TELEGRAM_BOT_TOKEN`               | BotFather Bot API token          |
| `TELEGRAM_WEBHOOK_SECRET`          | Second generated secret          |
| `OWNER_TELEGRAM_SUBJECT`           | `0` during the first deployment  |
| `QSTASH_TOKEN`                     | QStash API token                 |
| `QSTASH_CURRENT_SIGNING_KEY`       | QStash current signing key       |
| `QSTASH_NEXT_SIGNING_KEY`          | QStash next signing key          |
| `GROQ_API_KEY` or `GEMINI_API_KEY` | At least one provider key        |

Keep the default model variables from `.env.example`. The OpenAI and SMTP
variables may remain empty. Never place a secret in a `NEXT_PUBLIC_*` variable
or commit an environment file.

### 7. Apply migrations and deploy

Load the local production environment and apply every pending migration:

```bash
bash -c 'set -a; source .env.production.local; set +a; exec pnpm run migrate'
```

Redeploy the Vercel production deployment after all environment variables are
saved. Check both API probes:

```bash
curl https://your-coreloop.vercel.app/api/app/health
curl https://your-coreloop.vercel.app/api/app/ready
```

Readiness should report `"ready":true`. A database error usually means the Turso
URL, token, or migrations are incorrect.

### 8. Register delivery and scheduling

Register the Telegram webhook using the same production environment:

```bash
bash -c 'set -a; source .env.production.local; set +a; exec pnpm run admin telegram-webhook'
```

In **QStash → Schedules**, create a schedule with:

```text
Method:      POST
Destination: https://your-coreloop.vercel.app/api/jobs/tick
Cron:        */10 * * * *
Body:        empty
```

Do not add a custom authorization header. QStash signs the request and Coreloop
verifies that signature. Use QStash's **Run now** action once and confirm the
request returns HTTP 200.

### 9. Bootstrap the owner

Generate the first single-use invitation:

```bash
bash -c 'set -a; source .env.production.local; set +a; exec pnpm run admin invite --ttl 24h'
```

Open the generated production URL yourself, sign in with Telegram, and complete
onboarding. Then open the Turso database's table browser or SQL editor and run:

```sql
SELECT telegram_subject, display_name, username, created_at
FROM users
ORDER BY created_at;
```

Copy your numeric `telegram_subject` into `OWNER_TELEGRAM_SUBJECT` locally and
in Vercel, replacing the temporary `0`. Redeploy production once more. Your
account can now open `/operations` and create invitations for other learners.

Treat invitation URLs and Telegram subjects as private values.

### 10. Verify the deployment

Before relying on scheduled delivery, confirm that:

- `/api/app/health` and `/api/app/ready` return HTTP 200;
- Telegram login works from a fresh single-use invite;
- the owner can access `/operations` and ordinary users cannot;
- a QStash **Run now** call reaches `/api/jobs/tick` successfully;
- a scheduled lesson arrives in Telegram and its **Read** or **Skip** button is
  acknowledged;
- Vercel, QStash, Turso, Groq, and Gemini remain within the limits you expect.

Changing the production domain later requires updating both origin variables,
Telegram's trusted origin and redirect URI, the Telegram webhook, and the QStash
schedule destination.

## Project structure

```text
app/                 Next.js routes
components/          Shared product UI
lib/                 Browser-safe API types and defaults
api/                  Vercel Go function entry points
backend/              Authentication, jobs, content, providers, and persistence
cmd/api/              Combined local Go server
migrations/           Ordered database migrations and seed data
tests/                Frontend and behavior tests
```

## Verification

Run the release checks before deploying a change:

```bash
pnpm run check
pnpm run format:check
GOCACHE=$PWD/.cache/go-build GOENV=off go vet ./api/... ./backend/... ./cmd/... ./migrations/...
pnpm run build
```

The project deliberately performs no deployment, migration, webhook
registration, provider call, or purchase automatically. Those operations remain
explicit and use credentials supplied by the person hosting the instance.
