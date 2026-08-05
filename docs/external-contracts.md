# External API contracts

Reviewed 2026-08-05. Provider models remain environment-configurable because
free-plan availability and model aliases can change independently of this app.

- Telegram login follows the official OIDC Authorization Code Flow with PKCE,
  `openid profile telegram:bot_access`, the published token/JWKS endpoints, and
  default RS256 signing. The stable `sub` claim identifies the application
  account, while the separate numeric `id` claim is the Bot API private-chat
  identifier: <https://core.telegram.org/bots/telegram-login>
- Telegram delivery and webhook secret headers follow the Bot API:
  <https://core.telegram.org/bots/api>
- Turso persistence uses the documented `/v2/pipeline` request, typed bound
  arguments, baton transactions, and response envelope:
  <https://docs.turso.tech/sdk/http/reference>
- QStash verification checks its HS256 JWT signature, issuer, exact subject,
  time claims, and raw-body hash:
  <https://upstash.com/docs/qstash/howto/signature>
- Groq lesson generation uses JSON Object Mode with the complete application
  schema included in the system instruction, followed by application validation
  and one corrective request. Production showed `openai/gpt-oss-20b` returning
  HTTP 400 `failed_generation` for the large strict lesson schema even though
  the model is documented for strict mode. JSON Object Mode is the documented
  fallback when strict structured output is unsuitable:
  <https://console.groq.com/docs/structured-outputs>
- Gemini structured JSON uses the official response-format schema, including the
  `APPLICATION_JSON` enum required by `responseFormat.text.mimeType`:
  <https://ai.google.dev/gemini-api/docs/structured-output>
- OpenAI is manual-only. Its default is the current balanced GPT-5.6 Terra model
  and may be overridden with `OPENAI_MODEL`:
  <https://developers.openai.com/api/docs/guides/latest-model>
- Vercel Go function duration is configured in `vercel.json` at the non-Fluid
  Hobby limit of 60 seconds. Go is not currently one of Vercel's supported Fluid
  Compute runtimes:
  <https://vercel.com/docs/functions/configuring-functions/duration>,
  <https://vercel.com/docs/fluid-compute>
- Hacker News discovery uses the official Firebase API's best-story and item
  endpoints. Community score and comment counts are ranking signals; linked
  original URLs remain the source of record: <https://github.com/HackerNews/API>
- Curated project releases use GitHub's public Releases API with conditional
  requests and a polling budget below the unauthenticated rate limit:
  <https://docs.github.com/en/rest/releases/releases>,
  <https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api>
- Curated Bluesky accounts use the public AppView author-feed endpoint without
  authentication. Embedded external URLs remain the source of record:
  <https://docs.bsky.app/docs/api/app-bsky-feed-get-author-feed>
- Stacker News is consumed through its public RSS endpoints rather than its
  undocumented GraphQL API. RSS/Atom and official sitemaps are fetched with
  conditional headers, strict HTTPS validation, response-size limits, and
  source-local failure isolation.

Application validation is mandatory even when a provider claims strict schema
support. External correctness never depends on a provider cache hit or on an LLM
obeying prose alone.
