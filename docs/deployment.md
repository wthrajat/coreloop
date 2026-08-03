# Production deployment

Last verified against provider documentation: 2026-08-04.

This guide assumes you know the Vercel Hobby dashboard but have not previously
configured Turso, Telegram Login, QStash, Groq, or Gemini. Follow the sections
in order because several later credentials depend on the permanent Vercel URL.

The intended deployment has no mandatory billable service:

- one personal Vercel Hobby project;
- one Turso Free database;
- one Upstash QStash Free account and schedule;
- one Telegram bot with Telegram Login;
- Groq Free first and Gemini Free second for automatic lesson generation;
- optional OpenAI credits, usable only after an explicit owner action;
- optional Gmail SMTP alerts.

Free-plan terms and quotas can change. Confirm the plan shown by each provider
before accepting any billing prompt. The application has no billing integration
and never upgrades a provider account.

## Deployment order

```text
local checks
  → GitHub repository
  → initial Vercel deployment and stable URL
  → Turso + Telegram + AI + QStash credentials
  → Vercel production environment variables
  → Turso migrations and Vercel redeploy
  → Telegram webhook and QStash schedule
  → first invited owner login
  → replace temporary owner subject and redeploy
  → end-to-end acceptance checks
```

## 1. Understand the credentials

The following table is the complete production inventory. Values marked
"generated" are created by you, not copied from a provider.

| Variable                       | Where it comes from                | Required        | Secret  |
| ------------------------------ | ---------------------------------- | --------------- | ------- |
| `APP_ENV`                      | set to `production`                | yes             | no      |
| `APP_ORIGIN`                   | permanent Vercel production origin | yes             | no      |
| `NEXT_PUBLIC_APP_ORIGIN`       | same origin                        | yes             | no      |
| `APP_TIME_ZONE`                | set to `Asia/Kolkata`              | yes             | no      |
| `ENABLE_EXPERIMENTAL_COREPACK` | set in Vercel to `1`               | yes             | no      |
| `SESSION_SECRET`               | generated locally                  | yes             | yes     |
| `TURSO_DATABASE_URL`           | Turso CLI                          | yes             | no      |
| `TURSO_AUTH_TOKEN`             | Turso CLI                          | yes             | yes     |
| `TELEGRAM_BOT_TOKEN`           | Telegram `@BotFather`              | yes             | yes     |
| `TELEGRAM_CLIENT_ID`           | BotFather Web Login                | yes             | no      |
| `TELEGRAM_CLIENT_SECRET`       | BotFather Web Login                | yes             | yes     |
| `TELEGRAM_WEBHOOK_SECRET`      | generated locally                  | yes             | yes     |
| `OWNER_TELEGRAM_SUBJECT`       | first verified login row in Turso  | yes             | private |
| `QSTASH_TOKEN`                 | Upstash QStash console             | yes             | yes     |
| `QSTASH_CURRENT_SIGNING_KEY`   | Upstash QStash console             | yes             | yes     |
| `QSTASH_NEXT_SIGNING_KEY`      | Upstash QStash console             | yes             | yes     |
| `GROQ_API_KEY`                 | GroqCloud console                  | one free AI key | yes     |
| `GROQ_MODEL`                   | use reviewed default               | no              | no      |
| `GEMINI_API_KEY`               | Google AI Studio                   | one free AI key | yes     |
| `GEMINI_MODEL`                 | use reviewed default               | no              | no      |
| `OPENAI_API_KEY`               | OpenAI project                     | no              | yes     |
| `OPENAI_MODEL`                 | use reviewed default               | no              | no      |
| `ADMIN_ALERT_EMAIL`            | your alert inbox                   | no              | private |
| `SMTP_*`                       | Gmail/App Password                 | no              | partly  |

Never prefix a secret with `NEXT_PUBLIC_`; Next.js exposes such values to the
browser. Never paste secrets into GitHub files, Telegram messages, screenshots,
issue descriptions, or Vercel build logs.

## 2. Prepare and verify the repository locally

### 2.1 Install prerequisites

The project expects:

- Node.js 24 or newer;
- pnpm 11.17 or newer;
- Go 1.26 or newer;
- SQLite 3;
- Git;
- a personal GitHub account;
- Homebrew on macOS for the easiest Turso CLI installation.

Enable the repository-pinned pnpm version through Corepack:

```bash
corepack enable
corepack install --global pnpm@11.17.0
```

Check the installed versions:

```bash
node --version
pnpm --version
go version
sqlite3 --version
git --version
```

### 2.2 Run the release gates

From the project root:

```bash
cd /Users/rajat/dev/plans/coreloop
pnpm install --frozen-lockfile
pnpm run check
pnpm run format:check
GOCACHE=$PWD/.cache/go-build GOENV=off go vet ./api/... ./backend/... ./cmd/... ./migrations/...
pnpm run build
pnpm audit --prod --audit-level high
```

Do not deploy if a test, build, vet, or high-severity production audit fails.
`pnpm audit` requires access to the npm registry.

## 3. Put the project in a personal GitHub repository

Vercel can deploy from its CLI without Git, but a connected GitHub repository is
simpler for normal updates. Vercel automatically redeploys the production branch
after a push.

This project currently has no configured remote. Create a **private repository
under your personal GitHub account**, not a private repository owned by a GitHub
organization. Vercel's Hobby restrictions can block organization-owned private
repositories.

### Option A: GitHub website

1. Open <https://github.com/new>.
2. Use a name such as `coreloop`.
3. Select **Private**.
4. Do not initialize it with a README, `.gitignore`, or license; those files
   already exist locally.
5. Create the repository and copy its HTTPS remote URL.
6. From the project root, inspect exactly what will be committed:

```bash
git status --short
git diff -- . ':!pnpm-lock.yaml'
```

7. Confirm no `.env`, token, key, database, or private dump is listed.
8. Create the initial commit and push it:

```bash
git add .
git commit -m "Build Coreloop"
git remote add origin https://github.com/YOUR_GITHUB_USERNAME/coreloop.git
git push -u origin main
```

### Option B: GitHub CLI

After `gh auth login`, inspect the worktree as above and run:

```bash
git add .
git commit -m "Build Coreloop"
gh repo create coreloop --private --source=. --remote=origin --push
```

## 4. Create the initial Vercel Hobby project

This first deployment establishes the permanent `*.vercel.app` origin needed by
Telegram. It may report the API as not configured until all variables are added.

1. Open <https://vercel.com/new> while signed into your personal Hobby account.
2. Under **Import Git Repository**, connect GitHub if needed and select the
   personal `coreloop` repository.
3. Use these project settings:
   - Framework Preset: **Next.js**;
   - Root Directory: `.` (the repository root);
   - Build Command: leave the detected default;
   - Install Command: leave the detected pnpm default;
   - Output Directory: leave the detected default.
4. Expand **Environment Variables** and add `ENABLE_EXPERIMENTAL_COREPACK=1`.
   The repository pins pnpm 11.17.0 in `package.json`, and this tells Vercel to
   honor that Corepack version.
5. Click **Deploy**. The frontend and the three Go functions are built in the
   same Vercel project.
6. Open **Project → Settings → Domains** after deployment.
7. Choose the stable production domain, for example:

```text
https://coreloop-yourname.vercel.app
```

Use the exact origin Vercel shows, without a trailing slash. Refer to it as
`https://YOUR_PRODUCTION_DOMAIN` in the remaining guide. Do not use a preview
deployment URL because preview URLs change and Telegram only redirects to
pre-registered URLs.

If you later add a custom domain, repeat every origin-dependent step in the
"Changing the production domain" section.

## 5. Generate the two application secrets

Generate independent hexadecimal secrets. Hex characters satisfy Telegram's
webhook-secret character restrictions and are safe to paste into environment
variables.

```bash
openssl rand -hex 32
openssl rand -hex 32
```

Store the first output as `SESSION_SECRET` and the second as
`TELEGRAM_WEBHOOK_SECRET` in a password manager. Each output is 64 characters.
Do not reuse either value for another application.

Changing `SESSION_SECRET` later signs every user out and invalidates outstanding
invites. Changing `TELEGRAM_WEBHOOK_SECRET` requires registering the webhook
again.

## 6. Create the Turso Free database

Turso stores profiles, curriculum history, cached lessons, radar items, jobs,
leases, interactions, and delivery state. The serverless functions cannot use a
local SQLite file.

### 6.1 Install and authenticate the Turso CLI

On macOS:

```bash
brew install tursodatabase/tap/turso
turso auth signup
```

If you already have an account, use:

```bash
turso auth login
```

The CLI opens a browser for authentication. The official setup is documented in
the [Turso CLI quickstart](https://docs.turso.tech/quickstart).

### 6.2 Create the database

```bash
turso db create coreloop
turso db show coreloop
```

If the name already exists in your Turso organization, choose a unique name and
use that name in every later Turso command.

### 6.3 Obtain the URL and database token

```bash
turso db show coreloop --http-url
turso db tokens create coreloop
```

Copy the first command's HTTPS URL to `TURSO_DATABASE_URL` and the second
command's token to `TURSO_AUTH_TOKEN`. A database-scoped token is preferable to
a broad platform API token. The token must allow both reads and writes.

The application uses Turso's documented SQL-over-HTTP `/v2/pipeline` endpoint.
Do not put the Turso token in a browser-visible variable.

## 7. Create the Telegram bot and OIDC credentials

Telegram supplies three distinct values: a Bot API token, an OIDC client ID, and
an OIDC client secret.

### 7.1 Create the bot

1. Open Telegram and start the verified `@BotFather` account.
2. Send `/newbot`.
3. Choose a display name, for example **Coreloop**.
4. Choose an available username ending in `bot`, for example
   `rajat_coreloop_bot`.
5. BotFather returns a token resembling `123456789:...`. Save it as
   `TELEGRAM_BOT_TOKEN`.
6. Optionally configure `/setdescription`, `/setabouttext`, and `/setuserpic` so
   friends can recognize the bot before granting access.

Treat the bot token as a password. Anyone holding it can act as the bot.

### 7.2 Configure Telegram Web Login

Telegram's current OIDC setup is inside the BotFather mini app:

1. Open `@BotFather`.
2. Open **Bot Settings** for the new bot.
3. Open **Web Login**.
4. Register both of these Allowed URLs, replacing the domain exactly:

```text
https://YOUR_PRODUCTION_DOMAIN
https://YOUR_PRODUCTION_DOMAIN/api/app/auth/callback
```

5. Copy the displayed **Client ID** to `TELEGRAM_CLIENT_ID`.
6. Copy the displayed **Client Secret** to `TELEGRAM_CLIENT_SECRET`.
7. Leave the ID-token signing algorithm at **RS256**. The backend intentionally
   rejects another algorithm.

The app requests `openid`, `profile`, and `telegram:bot_access`. It does not
request a phone number. `telegram:bot_access` lets the bot send the lessons the
person has requested. The exact flow and BotFather screens are documented in
[Telegram Login](https://core.telegram.org/bots/telegram-login).

### 7.3 Do not guess the owner subject

`OWNER_TELEGRAM_SUBJECT` is the verified OIDC `sub` claim, not a bot token or
username. It is easiest and safest to discover it after the first invited login.
Use the temporary numeric value `0` for the first deployment. Section 15
replaces it with the real subject before the application is considered ready.

## 8. Create the Groq Free API key

Groq is the first automatic generation provider.

1. Sign in at <https://console.groq.com/>.
2. Stay on the Free plan; do not add a payment method for this deployment.
3. Open the [API Keys page](https://console.groq.com/keys).
4. Click **Create API Key**.
5. Name it `coreloop-production` and select the project/organization you intend
   to use.
6. Copy it immediately to `GROQ_API_KEY`; store it in a password manager.
7. Leave `GROQ_MODEL=openai/gpt-oss-20b`.

The selected Groq model currently supports strict JSON Schema output, which the
lesson contract uses. Free-tier limits are organization-wide and may change; the
current values are visible in Groq's **Settings → Limits**. A 429 or quota error
causes the router to try Gemini. It never causes a paid upgrade.

## 9. Create the Gemini Free API key

Gemini is the second and final automatic provider.

1. Sign in at [Google AI Studio](https://aistudio.google.com/apikey) with your
   Google account.
2. Accept the Gemini API terms if prompted.
3. Open **API Keys**.
4. Create or select a Google Cloud project dedicated to Coreloop.
5. Click **Create API key**.
6. Confirm the Key Type is **Auth**, then copy it to `GEMINI_API_KEY`.
7. Leave `GEMINI_MODEL=gemini-3.6-flash`.

As of 2026-08-04, Google says newly created AI Studio keys are Auth keys and
standard keys must be migrated before September 2026. The current procedure is
in [Gemini API key setup](https://ai.google.dev/gemini-api/docs/api-key).

Keep the project on the Gemini free tier and do not enable Cloud Billing if your
goal is a hard zero-spend setup. Free-tier availability can depend on country
and model. If the free tier is unavailable, leave the key configured and accept
the application's quota/unavailable message; do not enable billing merely to
make fallback work.

## 10. Optional: create an OpenAI project key

OpenAI is not needed for normal scheduled operation. The code cannot select it
automatically. It is available only when the authenticated owner selects one
quota-blocked job in Operations and confirms the action.

If you want that escape hatch:

1. Open the [OpenAI API key page](https://platform.openai.com/api-keys).
2. Create or select a project dedicated to Coreloop.
3. Create a **project API key**, not a shared personal/team key.
4. Copy the key immediately to `OPENAI_API_KEY`.
5. Leave `OPENAI_MODEL=gpt-5.6-terra` unless the implementation is reviewed for
   another model.
6. Review the OpenAI project limits and billing page. Keep auto-recharge off if
   you do not want automatic credit purchases.

OpenAI API usage is not free simply because a key exists. This app can consume
existing credits only after the owner's explicit one-job action. Leaving
`OPENAI_API_KEY` empty completely disables that path without affecting Groq or
Gemini.

OpenAI documents key creation and environment-variable storage in its
[developer quickstart](https://platform.openai.com/docs/quickstart/make-your-first-api-request).

## 11. Create the Upstash QStash Free credentials

QStash wakes the application every ten minutes and dispatches individual jobs.
Turso remains the queue's source of truth, so a failed QStash call does not
erase queued work.

1. Sign in at <https://console.upstash.com/>.
2. Select **QStash** from the product navigation.
3. Stay on the **Free** plan. No Redis database is needed for this application.
4. Open the QStash **Details**, **Settings**, or credential panel.
5. Copy the REST/API token to `QSTASH_TOKEN`.
6. Copy the current signing key to `QSTASH_CURRENT_SIGNING_KEY`.
7. Copy the next signing key to `QSTASH_NEXT_SIGNING_KEY`.

The token authorizes this backend to publish `/api/jobs/run` messages. The two
signing keys authenticate messages arriving from QStash and support safe key
rotation. Never substitute a Redis REST token for the QStash token.

The QStash Free plan currently advertises 1,000 message attempts per day. The
ten-minute tick consumes 144 normal attempts per day; dispatched jobs and
retries consume additional attempts. Check the
[current QStash pricing page](https://upstash.com/pricing/qstash) before
inviting many users.

## 12. Optional: configure Gmail alerts

This integration sends only owner quota/failure alerts. Users still see provider
quota state in Telegram and the web UI without SMTP.

To skip email alerts, leave `ADMIN_ALERT_EMAIL` and every `SMTP_*` variable
empty.

To use Gmail:

1. Enable 2-Step Verification for the sending Google account.
2. Open <https://myaccount.google.com/apppasswords>.
3. Create an app password named `Coreloop`.
4. Copy the 16-character value; Google shows it once.
5. Configure:

```text
ADMIN_ALERT_EMAIL=your-alert-address@example.com
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-sending-gmail-address@gmail.com
SMTP_PASSWORD=the-16-character-app-password
SMTP_FROM=your-sending-gmail-address@gmail.com
```

Use an app password, never the Google account password. Google requires 2-Step
Verification and may hide App Passwords for managed, security-key-only, or
Advanced Protection accounts. See
[Google's App Password guide](https://support.google.com/accounts/answer/185833).

## 13. Assemble a private local production environment file

Create `.env.production.local` from `.env.example`. This filename is already
ignored by Git. Fill it locally; do not commit it.

```text
APP_ENV=production
APP_ORIGIN=https://YOUR_PRODUCTION_DOMAIN
NEXT_PUBLIC_APP_ORIGIN=https://YOUR_PRODUCTION_DOMAIN
APP_TIME_ZONE=Asia/Kolkata
SESSION_SECRET=YOUR_FIRST_OPENSSL_VALUE

TURSO_DATABASE_URL=YOUR_TURSO_HTTPS_URL
TURSO_AUTH_TOKEN=YOUR_TURSO_DATABASE_TOKEN

TELEGRAM_CLIENT_ID=YOUR_TELEGRAM_CLIENT_ID
TELEGRAM_CLIENT_SECRET=YOUR_TELEGRAM_CLIENT_SECRET
TELEGRAM_BOT_TOKEN=YOUR_TELEGRAM_BOT_TOKEN
TELEGRAM_WEBHOOK_SECRET=YOUR_SECOND_OPENSSL_VALUE
OWNER_TELEGRAM_SUBJECT=0

QSTASH_CURRENT_SIGNING_KEY=YOUR_CURRENT_SIGNING_KEY
QSTASH_NEXT_SIGNING_KEY=YOUR_NEXT_SIGNING_KEY
QSTASH_TOKEN=YOUR_QSTASH_TOKEN

GROQ_API_KEY=YOUR_GROQ_KEY
GROQ_MODEL=openai/gpt-oss-20b
GEMINI_API_KEY=YOUR_GEMINI_KEY
GEMINI_MODEL=gemini-3.6-flash

# Optional; leave the value empty to disable manual OpenAI.
OPENAI_API_KEY=
OPENAI_MODEL=gpt-5.6-terra

# Optional; leave all empty to disable SMTP.
ADMIN_ALERT_EMAIL=
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=
```

Before sourcing it, check that it is ignored:

```bash
git check-ignore .env.production.local
```

The command should print `.env.production.local`. If it prints nothing, stop and
fix `.gitignore` before continuing.

## 14. Add production environment variables to Vercel

1. Open the Vercel project.
2. Go to **Settings → Environment Variables**.
3. Add every non-comment variable from `.env.production.local`.
4. Confirm `ENABLE_EXPERIMENTAL_COREPACK=1` is also set for Production. It is a
   Vercel build setting, so it is not part of `.env.production.local`.
5. Select **Production** for each value. The Telegram configuration is tied to
   one stable origin, so previews are intentionally not fully connected.
6. Mark secrets as sensitive when Vercel offers that choice.
7. Confirm:
   - `APP_ENV` is exactly `production`;
   - both origin variables use `https://` and have no trailing slash;
   - `OWNER_TELEGRAM_SUBJECT` is temporarily `0`;
   - the two generated secrets are different and at least 32 characters;
   - no secret was placed in a `NEXT_PUBLIC_*` value.

Environment changes apply only to new deployments. Do not redeploy yet; apply
the database schema first.

## 15. Apply the database migrations

From the project root, load the local production file and run the migration
tool:

```bash
set -a
source .env.production.local
set +a
pnpm run migrate
```

Expected output:

```text
applied 3 migrations
```

The runner records versions in `schema_migrations` and is safe to run again.
Confirm the versions directly:

```bash
turso db shell coreloop \
  "SELECT version, name, applied_at FROM schema_migrations ORDER BY version;"
```

You should see versions 1, 2, and 3. Replace `coreloop` with your actual
database name if different.

## 16. Redeploy with the complete environment

1. In Vercel, open **Deployments**.
2. Open the latest production deployment's menu.
3. Click **Redeploy** and confirm **Production**.
4. Wait for the Next.js build and all three Go functions to finish.
5. Open the deployment logs if Vercel reports an error.

Verify the public endpoints:

```bash
curl -i https://YOUR_PRODUCTION_DOMAIN/api/app/health
curl -i https://YOUR_PRODUCTION_DOMAIN/api/app/ready
```

Expected results:

- `/health`: HTTP 200;
- `/ready`: HTTP 200 with `"ready":true` and database schema version 3.

If readiness is 503, inspect the JSON response and Vercel Function logs before
continuing. The most common causes are an incorrect Turso URL/token or a missing
production variable.

## 17. Register the Telegram webhook

The OIDC callback handles login. A separate Bot API webhook handles Read, Skip,
and radar button presses.

With `.env.production.local` still loaded:

```bash
pnpm run admin telegram-webhook
```

Expected output:

```text
Telegram webhook configured: https://YOUR_PRODUCTION_DOMAIN/api/telegram/webhook
```

Verify Telegram's stored webhook without printing the bot token itself:

```bash
curl --silent \
  "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo"
```

Confirm that `url` matches the production webhook exactly and that
`last_error_message` is empty. Do not paste this command's expanded URL into a
ticket or chat because it contains the bot token.

## 18. Create the QStash schedule

1. Open **Upstash Console → QStash → Schedules**.
2. Click **Create Schedule**.
3. Set Destination URL to:

```text
https://YOUR_PRODUCTION_DOMAIN/api/jobs/tick
```

4. Set method to **POST**.
5. Set the cron expression to:

```text
*/10 * * * *
```

6. Leave the request body empty.
7. Do not add a custom Authorization header; QStash adds its signed
   `Upstash-Signature` header automatically.
8. Save the schedule.

QStash evaluates ordinary cron expressions in UTC, but this schedule runs every
ten minutes so timezone does not affect it. Per-user delivery times are expanded
inside the application using `Asia/Kolkata`.

Use QStash's **Run now** or equivalent console action once. The delivery should
return HTTP 200 with `{"status":"accepted"}`. A direct browser or curl POST is
expected to return 403 because it lacks QStash's signature.

Official schedule behavior is documented in
[QStash Schedules](https://upstash.com/docs/qstash/features/schedules), and the
receiver verifies the contract described in
[QStash signature verification](https://upstash.com/docs/qstash/howto/signature).

## 19. Bootstrap the first owner safely

The temporary owner subject `0` allowed production configuration to start, but
no real Telegram user has owner rights yet.

### 19.1 Create the first single-use invite

With the same production environment loaded locally:

```bash
pnpm run admin invite --ttl 24h
```

The command prints an expiry time and one URL. Open that URL yourself. Do not
send the first invite to a friend.

### 19.2 Sign in with Telegram

1. Open the invite in a normal browser.
2. Continue to Telegram Login.
3. Confirm the expected bot and approve profile plus bot messaging access.
4. Complete onboarding. A welcome message should arrive from the bot.

At this point your account exists, but the Operations route remains unavailable
because the temporary owner subject is still `0`.

### 19.3 Read the verified OIDC subject from Turso

```bash
turso db shell coreloop \
  "SELECT telegram_subject, display_name, username, created_at FROM users ORDER BY created_at;"
```

If this is the only account, copy its numeric `telegram_subject`. If multiple
rows exist, identify yours using `display_name` and `username`. This database
value came from Telegram's verified ID token; do not substitute a username, bot
ID, phone number, or an ID from an untrusted Telegram helper bot.

### 19.4 Promote that subject in Vercel

1. Replace `OWNER_TELEGRAM_SUBJECT=0` in `.env.production.local` with the copied
   value.
2. Open **Vercel → Settings → Environment Variables**.
3. Edit the production `OWNER_TELEGRAM_SUBJECT` to the same value.
4. Redeploy the latest production deployment.
5. Sign out and back in if the browser session does not refresh owner controls.
6. Open `/operations`; it should now show system metrics and invite controls.

The owner may create friend invites from Operations. Each friend gets an
independent profile and Telegram destination.

## 20. End-to-end acceptance test

Complete these checks before relying on scheduled delivery:

### Application and authentication

- `/api/app/health` and `/api/app/ready` return HTTP 200.
- The landing page works on desktop and mobile width.
- An invalid or consumed invite cannot create a new profile.
- The owner can sign in again without another invite.
- `/operations` is available only to the configured owner.

### Profile and lesson scheduling

- Save at least one topic.
- Set 15 minutes, three lessons, 08:30/13:00/20:30, and weekdays only.
- Temporarily move one lesson time near the current India time if you want an
  immediate generation test.
- Run the QStash tick from its console.
- Watch **Vercel → Logs** and **QStash → Logs** for `/api/jobs/tick` followed by
  `/api/jobs/run` calls.
- Confirm the full detailed lesson arrives as ordered Telegram parts.
- Confirm later parts are not sent twice after a QStash retry.

### Interaction and progress

- Press **Read** on the final lesson part.
- Confirm Progress changes and a recall item is eventually due.
- Press **Skip** on a radar item and confirm the button is acknowledged.
- Confirm radar announcements do not replace the configured lesson messages.

### Provider fallback and zero-spend boundary

- Inspect provider runs in Operations or Turso and confirm Groq is attempted
  before Gemini.
- Temporarily replace the Groq key with an invalid value, redeploy, and confirm
  Gemini is attempted.
- Restore the real Groq key immediately after the test.
- Temporarily remove both free keys, redeploy, and confirm the job becomes quota
  blocked, remains in Turso, and the user sees the quota message.
- Confirm OpenAI is not called automatically.
- Restore at least one free key and let a later tick recover the blocked work in
  chronological order.
- If testing manual OpenAI, select one blocked job in Operations, read the
  confirmation, and verify only that job is attempted.

### Privacy

- Create a separate test profile through a second invite.
- Export its data.
- Delete that profile and confirm it can no longer authenticate.
- Do not use the owner account for the destructive deletion test.

## 21. Normal deployments after the first setup

For ordinary code changes:

```bash
pnpm install --frozen-lockfile
pnpm run check
pnpm run format:check
pnpm run build
git add .
git commit -m "Describe the change"
git push
```

A push to `main` creates a production deployment. Other branches create preview
deployments, but previews will not have functional Telegram login unless you
explicitly configure separate allowed URLs and environment values.

For a new migration:

1. deploy neither code nor schema blindly;
2. back up or export Turso;
3. load the production environment locally;
4. run `pnpm run migrate`;
5. verify `schema_migrations` and readiness;
6. deploy the compatible application version.

For a changed Vercel environment variable, redeploy; editing a variable does not
retroactively change an existing deployment.

## 22. Changing the production domain

If you replace the `*.vercel.app` domain with a custom domain:

1. add and verify the custom domain in Vercel;
2. update `APP_ORIGIN` and `NEXT_PUBLIC_APP_ORIGIN` in Vercel and locally;
3. add the new origin and callback URL in BotFather Web Login;
4. redeploy Vercel;
5. rerun `pnpm run admin telegram-webhook`;
6. replace the QStash schedule destination;
7. verify health, readiness, login, webhook information, and one signed tick;
8. only then remove the old Telegram URLs or old domain.

The QStash signature includes the destination URL. `APP_ORIGIN`, the QStash
destination, and the actual requested origin must match exactly.

## 23. Common deployment failures

### Vercel deployment builds but every API returns 503

Check that all required variables are attached to **Production**, not merely
Preview or Development. Then redeploy. The production config requires both
QStash signing keys and at least one of Groq/Gemini.

### `/ready` reports a database failure

- Confirm `TURSO_DATABASE_URL` is the HTTP URL for the correct database.
- Create a new database token if the old one is expired or revoked.
- Run `pnpm run migrate` again and inspect `schema_migrations`.
- Confirm the database token is read-write.

### Telegram says the redirect URL is invalid

- Compare the callback character by character with BotFather Web Login.
- Use `https`, the production domain, and `/api/app/auth/callback`.
- Remove trailing slashes.
- Confirm the deployed `APP_ORIGIN` matches.

### Telegram login succeeds but no lesson can be delivered

- Confirm the user approved bot messaging access during login.
- Confirm the bot has not been blocked in Telegram.
- Check `getWebhookInfo` for errors.
- Confirm `TELEGRAM_BOT_TOKEN` belongs to the same bot as the OIDC client.

### Telegram callbacks return 403

The deployed webhook secret and the secret registered through `setWebhook` do
not match. Update Vercel, redeploy, load the same value locally, and rerun:

```bash
pnpm run admin telegram-webhook
```

### QStash tick returns 403

- Call it from QStash, not curl.
- Confirm both signing keys are copied from the same QStash account as the
  schedule.
- Confirm the schedule destination and `APP_ORIGIN` match exactly.
- Redeploy after changing signing keys.

### Lessons show quota exceeded

Check Groq and Google AI Studio quota/limit pages. The job remains durable and
will retry. Restore or rotate a free-provider key. Do not expect automatic
OpenAI fallback.

### Owner Operations remains forbidden

Query the stored `telegram_subject` again, compare it with the exact Vercel
Production value, and redeploy. A Telegram username is not accepted.

## 24. Backups, rotation, and ongoing operations

Before migrations or risky operational work, create a Turso dump:

```bash
turso db shell coreloop .dump > coreloop-backup.sql
```

The dump contains private profile and lesson data. Store it securely outside the
repository and delete it when no longer needed.

Review these regularly:

- Vercel function errors and usage;
- QStash daily message use and failed deliveries;
- Turso rows read/written and storage;
- Groq and Gemini quota state;
- Telegram webhook errors;
- blocked/failed jobs in `/operations`;
- GitHub Actions release checks and dependency audit.

Credential rotation and recovery procedures are in [runbook.md](runbook.md).
External contracts and their primary documentation are collected in
[external-contracts.md](external-contracts.md).
