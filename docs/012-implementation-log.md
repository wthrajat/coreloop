# Implementation log

Last updated: 2026-08-04

## Milestone 1: skeleton and persistence

Status: complete

The repository is now initialized at `/Users/rajat/dev/plans/coreloop`
and must continue to be treated as the project root.

### Implemented

- Next.js App Router with TypeScript, ESLint, Prettier, and pinned package
  versions
- Responsive product shell with overview, progress, and settings routes
- Honest pre-connection and empty states rather than invented learner data
- Central OKLCH design tokens, product context, design context, keyboard focus,
  reduced-motion handling, and structural mobile navigation
- Three Go Vercel Function entry points for the application API, Telegram
  webhook, and QStash jobs
- Public Go application facade with internal HTTP, error, and store packages
- Structured JSON errors and an application health endpoint at `/api/app`
- Turso-compatible initial migration with foreign keys, constraints, indexes,
  durable job state, bundle/part idempotency, provider accounting, and cache
  metadata
- Unit tests for the accepted default schedule, Go HTTP behavior, structured
  errors, and migration table coverage
- Local Go API command, environment template, Vercel configuration, README, and
  Git repository initialization

### Verified

- `pnpm run lint`
- `pnpm run typecheck`
- `pnpm test`
- `pnpm run build`
- `go test ./api/... ./backend/... ./cmd/...`
- Initial migration execution and `PRAGMA foreign_key_check` in SQLite 3.51
- Impeccable static design detector: no findings in `app/` or `components/`

The sandbox did not permit binding a local port, so browser-level inspection was
not possible in this implementation session. The production build generated all
four current application routes successfully.

### Design-source note

`theme.md` was an empty zero-byte file at implementation time. The UI therefore
uses a temporary restrained baseline documented in `DESIGN.md`, with all visual
choices centralized in CSS tokens so a populated theme can replace them without
changing the information architecture.

## Production implementation pass

Status: repository complete; live acceptance credential-gated

The full deployable application is now implemented locally:

- Telegram PKCE/OIDC, invite-only onboarding, opaque sessions, and CSRF
- Turso HTTP driver, migrations, topic/source catalog, profile, privacy,
  curriculum, queue, lesson, delivery, radar, and operations stores
- Groq/Gemini automatic routing, owner-only OpenAI, compact prompts, strict
  output validation, one correction, verification warnings, and shared caches
- QStash-signed chronological jobs with leases, recovery, quota blocking, and
  lesson/delivery/radar work
- Telegram full-lesson chunks, read/skip callbacks, ranked radar, welcome, and
  quota messages
- Authenticated responsive control surface and owner-only operations UI
- Production validation, migration/admin commands, CI, security policy,
  deployment guide, and runbook

Final local evidence: Go tests, TypeScript, ESLint, Node tests, all three SQL
migrations, and the optimized Next.js build pass. Live Turso, Telegram, QStash,
Groq, Gemini, SMTP, OpenAI, and Vercel verification remains credential-gated
and is intentionally part of deployment acceptance testing.

## Documentation and package-manager pass

Status: complete on 2026-08-04

- Moved all Markdown documents into `docs/`, leaving the durable project context
  in the root `AGENTS.md`.
- Expanded `docs/deployment.md` into the complete credential, deployment,
  owner-bootstrap, acceptance, and recovery procedure.
- Migrated local commands, CI, Vercel detection, and the lockfile from npm to
  pnpm 11.17.0.
- Explicitly allowed only the `sharp` and `unrs-resolver` install scripts. Pinned
  patched transitive `sharp` 0.35.3 and `postcss` 8.5.25 versions after the pnpm
  production audit identified advisories in Next.js's original resolutions.
- Verified the frozen install, lint, TypeScript, Node and Go tests, all SQL
  migrations, formatting, Go vet, optimized Next.js build, Markdown links, and
  `pnpm audit --prod --audit-level high`; the final audit reports no known
  vulnerabilities.

## Coreloop brand migration

Status: complete on 2026-08-04

- Adopted Coreloop as the final product name, replacing the original working
  name throughout user-facing copy and durable documentation.
- Renamed the package, Go module/import paths, service identity, local API
  environment variable, cookies, export filenames, Telegram messages, SMTP
  subjects, HTTP user agent, and deployment examples to the `coreloop` slug.
- Renamed the repository directory to `/Users/rajat/dev/plans/coreloop`.
- Retained Radar as the distinct name of the ranked current-information feature;
  it is no longer the product brand.
- Verified a frozen pnpm install, lint, TypeScript, Node and Go tests, SQL
  migrations, formatting, Go vet, a clean production audit, the optimized
  Next.js build, Markdown links, and a zero-result stale-brand scan from the new
  repository path.
