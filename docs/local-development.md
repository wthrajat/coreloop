# Local development

This guide contains the technical setup that is intentionally kept out of the
main README.

## Requirements

- Node.js 24 or newer
- pnpm 11.17
- Go 1.26 or newer
- SQLite 3

The repository pins `pnpm@11.17.0` in `package.json`.

## Install

```bash
corepack enable
corepack install --global pnpm@11.17.0
pnpm install --frozen-lockfile
```

## Run the public interface

The public and signed-out screens do not need service credentials:

```bash
pnpm run dev
```

Open <http://localhost:3000>.

## Run a connected environment

Copy the environment template and add your development credentials:

```bash
cp .env.example .env.local
```

Never commit `.env.local` or put a secret in a `NEXT_PUBLIC_*` variable.

Apply all migrations:

```bash
bash -c 'set -a; source .env.local; set +a; exec pnpm run migrate'
```

Run the Go API and Next.js app in separate terminals:

```bash
bash -c 'set -a; source .env.local; set +a; exec pnpm run dev:api'
```

```bash
pnpm run dev
```

Next.js runs on `http://localhost:3000` and proxies `/api/*` to the Go API on
`http://localhost:8080`.

## Useful commands

```bash
pnpm run migrate
pnpm run admin invite --ttl 24h
pnpm run admin telegram-webhook
```

The administration commands use the active environment and may change external
state. Use development credentials unless you intentionally mean to update
production.

## Release checks

Run these before deploying:

```bash
pnpm run check
pnpm run format:check
GOCACHE=$PWD/.cache/go-build GOENV=off go vet ./api/... ./backend/... ./cmd/... ./migrations/...
pnpm run build
pnpm audit --prod --audit-level high
```

The audit needs network access to the npm registry. Do not claim it passed when
the registry is unavailable.

## Technical references

- The browser talks to the Go application API; product logic stays out of
  Next.js pages.
- Turso is the source of truth for jobs, leases, deliveries, progress, and
  configuration.
- QStash wakes durable jobs; it is not the queue database.
- Telegram is the lesson and news reading surface. The web app is the control
  surface.
