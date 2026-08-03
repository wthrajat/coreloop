# External API contracts

Reviewed 2026-08-03. Provider models remain environment-configurable because
free-plan availability and model aliases can change independently of this app.

- Telegram login follows the official OIDC Authorization Code Flow with PKCE,
  `openid profile telegram:bot_access`, the published token/JWKS endpoints, and
  default RS256 signing: <https://core.telegram.org/bots/telegram-login>
- Telegram delivery and webhook secret headers follow the Bot API:
  <https://core.telegram.org/bots/api>
- Turso persistence uses the documented `/v2/pipeline` request, typed bound
  arguments, baton transactions, and response envelope:
  <https://docs.turso.tech/sdk/http/reference>
- QStash verification checks its HS256 JWT signature, issuer, exact subject,
  time claims, and raw-body hash:
  <https://upstash.com/docs/qstash/howto/signature>
- Groq structured JSON uses strict JSON Schema where the selected model supports
  it: <https://console.groq.com/docs/structured-outputs>
- Gemini structured JSON uses the official response-format schema:
  <https://ai.google.dev/gemini-api/docs/structured-output>
- OpenAI is manual-only. Its default is the current balanced GPT-5.6 Terra model
  and may be overridden with `OPENAI_MODEL`:
  <https://developers.openai.com/api/docs/guides/latest-model>
- Vercel Go function duration is configured in `vercel.json` within the Hobby
  limit: <https://vercel.com/docs/functions/configuring-functions/duration>

Application validation is mandatory even when a provider claims strict schema
support. External correctness never depends on a provider cache hit or on an LLM
obeying prose alone.
