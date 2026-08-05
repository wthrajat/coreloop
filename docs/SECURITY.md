# Security policy

Coreloop is an invite-only private application. Report a vulnerability through
GitHub private vulnerability reporting when it is available for the repository.
Otherwise, contact the maintainer through a private channel listed on their
GitHub profile. Do not include provider keys, Telegram tokens, session cookies,
user profiles, lesson bodies, or database exports in a public issue.

## Security boundaries

- Telegram login uses Authorization Code Flow with PKCE, state, nonce, RS256
  signature verification, issuer validation, and audience validation.
- Invitations and sessions are random opaque values. Only keyed hashes are
  stored. Sessions use `Secure`, `HttpOnly`, `SameSite=Lax` cookies in
  production; mutations also require a double-submit CSRF token and exact
  origin.
- New profiles require a single-use invite. Returning identities may sign in
  without consuming another invite.
- QStash workers verify the current or next HS256 signing key, issuer, exact
  destination, time claims, and raw-body SHA-256 hash.
- Telegram webhooks require the configured secret header and use constant-time
  comparison.
- Every user-owned store query receives the authenticated user ID. Owner routes
  additionally compare the verified Telegram subject to the owner allowlist.
- Scheduled jobs can call Groq and Gemini only. Paid OpenAI use exists behind a
  separate owner-only, CSRF-protected action for one blocked job.
- Account deletion hard-deletes the user row and relies on foreign-key cascades
  for private profile, schedule, progress, queue, and delivery data. Shared
  lessons contain no user identity.

## Secret handling

Use Vercel encrypted environment variables. Never prefix a secret with
`NEXT_PUBLIC_`. Rotate Telegram, Turso, QStash, AI, session, and SMTP
credentials after suspected disclosure. Rotation invalidates no database
content, although rotating `SESSION_SECRET` signs every user out and invalidates
unused invite links.
