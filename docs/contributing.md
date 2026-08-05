# Contributing

Coreloop welcomes focused fixes and improvements that preserve its private,
Telegram-first product boundaries. Discuss large product or architecture changes
before implementation so a pull request does not accidentally create a second
delivery system, weaken privacy, or introduce paid infrastructure.

## Before opening a pull request

- Keep business logic in the Go backend rather than Next.js pages.
- Add database changes as a new ordered migration; never rewrite a migration
  that may already be deployed.
- Preserve durable-job idempotency, lease fencing, Telegram message limits,
  exact-origin CSRF checks, and user scoping on every data access.
- Do not add credentials, real user identifiers, lesson text, or production logs
  to tests, screenshots, issues, or commits.
- Add focused tests for changed behavior, including retry and failure paths for
  external integrations.

Run the release checks from [local development](local-development.md) before
requesting review. Use the private reporting path in
[the security policy](SECURITY.md) for vulnerabilities instead of a public
issue.
