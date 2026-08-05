# Security policy

Coreloop is an invite-only private application. Report a vulnerability through
GitHub private vulnerability reporting when it is available for the repository.
Otherwise, contact the maintainer through a private channel listed on their
GitHub profile. Do not include provider keys, Telegram tokens, session cookies,
user profiles, lesson bodies, or database exports in a public issue.

## Security boundaries

- Telegram login uses Authorization Code Flow with PKCE, state, nonce, RS256
  signature verification, issuer, audience, authorized-party, issue-time, and
  expiry validation. A short-lived, host-only cookie also binds the callback to
  the browser that started the flow.
- Invitations and sessions are random opaque values. Only keyed hashes are
  stored. Sessions use `Secure`, `HttpOnly`, `SameSite=Lax` cookies in
  production; mutations also require a double-submit CSRF token and exact
  origin.
- New profiles require a single-use invite. Returning identities may sign in
  without consuming another invite.
- QStash workers require the HS256 JWT header and verify the current or next
  signing key, issuer, exact destination, time claims, and raw-body SHA-256
  hash.
- Telegram webhooks require the configured secret header and use constant-time
  comparison. Missing webhook configuration fails closed.
- Every user-owned store query receives the authenticated user ID. Owner routes
  additionally compare the verified Telegram subject to the owner allowlist.
- Scheduled jobs can call Groq and Gemini only. Paid OpenAI use exists behind a
  separate owner-only, CSRF-protected action for one blocked job.
- Account deletion hard-deletes the user row and relies on foreign-key cascades
  for private profile, schedule, progress, queue, and delivery data. Consumed
  invitation history is deleted first so it cannot block the cascade. Shared
  lessons contain no user identity.
- Feed fetches validate DNS results and redirects at connection time. Links
  delivered to Telegram must use a public HTTPS host and the standard HTTPS
  port.
- Login endpoints are database-backed rate-limited. Deep readiness checks are
  coalesced and cached briefly to avoid turning public probes into database
  amplification.

## Secret handling

Use Vercel encrypted environment variables. Never prefix a secret with
`NEXT_PUBLIC_`. Rotate Telegram, Turso, QStash, AI, session, and SMTP
credentials after suspected disclosure. Rotation invalidates no database
content, although rotating `SESSION_SECRET` signs every user out and invalidates
unused invite links.

External-service error bodies are untrusted. Provider-generated text and
Telegram token-bearing transport URLs must never be copied into application
logs. Keep production logs private and use identifiers such as job IDs and
provider request IDs for investigation.

## Browser and deployment hardening

- Private product and invitation pages are marked `noindex` and `noarchive`.
- Responses disable framing, MIME sniffing, cross-domain policy files, and
  cross-origin opener/resource sharing. API responses are non-cacheable and
  non-indexable.
- The static Next.js build currently needs inline framework script blocks, so
  `script-src` retains `unsafe-inline`. Inline event-handler attributes remain
  blocked with `script-src-attr 'none'`. Moving to nonce-based CSP requires
  dynamic rendering and must be evaluated as an explicit performance and hosting
  trade-off; do not remove the current directive without a production browser
  test.
- GitHub Actions are pinned to reviewed full commit SHAs, checkout does not
  persist credentials, and workflow permissions are read-only. Keep the
  repository's Actions allowlist, secret scanning, private vulnerability
  reporting, and branch protection enabled where the GitHub plan supports them.

## Audit record

The 2026-08-05 release-blocking review covered authentication, authorization,
sessions, CSRF, owner controls, OIDC, QStash, Telegram, queue idempotency,
database deletion, SQL construction, SSRF, untrusted model output, browser
sinks, response headers, secrets, dependencies, and CI supply chain.

The review fixed browser-unbound login callbacks, a normal-user deletion
failure, Telegram token disclosure through transport errors, a fail-open empty
webhook secret, replayed Radar feedback, misleading logout success, unsafe
external links, provider-output logging, trusted-proxy ambiguity, public
readiness amplification, weak signing-key validation, and mutable CI action
references. No verified SQL injection, IDOR, CSRF bypass, OIDC signature bypass,
QStash signature bypass, server-side request forgery, or browser XSS path was
found.

Dependency advisories still require the networked CI `pnpm audit` gate. The
local audit must not be described as clean when the npm registry is unreachable.
