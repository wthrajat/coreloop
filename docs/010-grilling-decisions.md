# Grilling decisions

Last updated: 2026-08-03

## Status

Accepted product and implementation direction. These decisions were produced by
the `grill-me` interview and supersede conflicting assumptions in documents
`001` through `009`.

## Product duration

Coreloop is not a ten-week course and has no curriculum end date or lesson
cap. It continuously chooses the next useful theme, persists topic and lesson
history, suppresses accidental repetition, and continues while the profile is
active.

## Accounts and invitations

- The application is shared only with close friends.
- The owner generates a single-use invite link for each friend.
- Telegram is the only identity provider. Use Telegram OIDC Authorization Code
  Flow with PKCE and the `openid`, `profile`, and `telegram:bot_access` scopes.
- There is no email signup, password, magic-link email, or separate Telegram
  connection code.
- After authentication, the user creates a profile and configures topics,
  current level, lesson duration, depth, schedule, weekends, and delivery mode.
- The first release is English-only.

## Learning and completion

- Default curriculum delivery remains three 15-minute weekday lessons at 09:00,
  14:00, and 21:00 in `Asia/Kolkata`; every user can configure it.
- Lessons remain on a coherent theme and are simple, technically precise, and
  detailed even at the 15-minute preset.
- Lessons are primarily theoretical. Coding, architecture, debugging, and
  product exercises are optional actions rather than mandatory homework.
- The system chooses the next topic and stores the selection, objectives,
  lesson version, and content fingerprint so it does not repeat material often.
- A repeat is allowed only when labelled as deeper continuation, spaced recall,
  failed-recall remediation, or a material technology update.
- The final Telegram part has a `read` button. Its callback records completion
  in the database. Delivery alone never marks a lesson read.
- Unread lessons remain in backlog and never block later scheduled delivery.
- Every lesson ends with one optional immediate recall question. At most one due
  recall question appears inside a later scheduled lesson; recall creates no
  separate notification stream.

## Dynamic generation and validation

- Dynamically generated lessons are mandatory from the first usable release.
- There is no human preview or approval step before delivery.
- Shared, versioned instructions and one normalized lesson schema apply to all
  providers.
- Generated text is personalized only by topic, current level, duration, and
  depth. Compatible users can reuse the same lesson version; assignments and
  progress remain per-user.
- On missing information or invalid structure, the system sends one corrective
  request to the same provider.
- Information quality outranks presentation. A second structurally imperfect
  response may ship.
- When factual claims or current-source support remain unverifiable, the lesson
  still ships with a prominent warning explaining the verification failure and
  separate verified facts, model interpretation, and unverified claims.

## Provider routing and money

The original references to xAI/Grok were a naming mistake. The intended free
provider is Groq.

Automatic generation order is:

1. Groq free quota
2. Gemini free quota
3. Stop and record `blocked_quota`

OpenAI uses the owner's existing paid credits but is never an automatic
fallback. It is invoked only through an explicit owner action outside the
normal scheduler.

All friends share the owner's server-side provider accounts. Work is processed
first-come, first-served with no user-priority or reserved-quota system. Provider
keys never reach a browser or user profile.

If Groq and Gemini are both unavailable:

- The learner receives a Telegram quota-error message.
- The owner receives a content-free operational alert.
- The generation job remains stored in the database.
- When free quota becomes available, all blocked work is processed
  chronologically and the recovered lessons are sent, even if this creates a
  large batch.

## AI efficiency

- A normal lesson uses one model call; validation permits at most one corrective
  call. Do not call several providers speculatively or ask one model to judge
  another by default.
- Generation is stateless. Never resend a user's complete chat history, profile,
  curriculum, or earlier lesson bodies.
- Build a compact request containing only topic, current level, duration, depth,
  active-theme position, unmet objectives, one due recall item, and selected
  source evidence.
- Keep one stable, versioned instruction prefix and one small structured dynamic
  payload. State every rule once. Compression means removing repetition and
  irrelevant context, not abbreviating away product requirements.
- Put stable content first so provider-native prompt caching can work where the
  selected model and free tier support it. Measure cache reads and writes rather
  than assuming they save quota or money.
- Cache verified lessons at the application layer using a provider-independent
  lesson-specification hash. Compatible learners reuse the content while keeping
  assignments and progress private.
- Cache normalized sources, content hashes, rankings, compiled prompt inputs, and
  rendered Telegram parts. Do not use an LLM for deterministic filtering,
  deduplication, scheduling, chunking, or state transitions.
- Apply explicit input and output token budgets. Select the most relevant source
  passages deterministically instead of sending entire documents or truncating
  blindly.
- Log tokens, provider cache usage, application cache hits, retries, latency, and
  validation results. A prompt becomes “more efficient” only when it still passes
  the representative quality evaluation set.

## Current-tech radar

- Radar messages are outside the three configured curriculum lesson slots.
- There is no fixed daily radar cap.
- A transparent rule-based relevance score is the spam control. It considers
  selected-topic match, career usefulness, technical impact, actionability,
  source authority, novelty, duplication, and hype.
- Low-scoring candidates are not delivered. `skip` becomes negative relevance
  feedback.
- Every delivered current item is detailed rather than a headline summary and
  may use several Telegram parts.
- Users configure topics, not arbitrary sources. The application owns a curated
  source allowlist.

## Hosting and implementation

- No payment card, billing account, or automatic paid upgrade is allowed for
  hosting infrastructure.
- One Vercel Hobby project hosts the simple Next.js UI and Go API functions.
- Turso's no-card free plan stores relational state.
- Upstash QStash's free plan wakes and queues serverless work. A scheduler tick
  around every ten minutes is sufficient; exact delivery time is not important.
- Service and AI quota exhaustion must result in a visible error or delayed
  backlog, never a charge.

## Privacy and administration

- The product has no administrator screen for reading friends' profiles,
  lessons, feedback, or recall answers.
- Operational alerts contain only opaque identifiers, provider/quota category,
  status, and timestamps.
- A database owner can inherently access the database using infrastructure
  credentials; the MVP provides application-level privacy, not encryption that
  hides data from the infrastructure owner.
- The administrator alert address is a deployment secret such as
  `ADMIN_ALERT_EMAIL`; the personal address is not committed to this repository.

## Superseded decisions

- The ten-week acceleration path and any fixed curriculum endpoint
- xAI/Grok as a provider
- A separately hosted always-on backend server
- Email or password authentication
- A separate `/start <token>` Telegram-account linking step
- Manual or statically seeded lessons as the first usable product
- A one-item-per-day radar cap
- Automatically invoking OpenAI after free quota exhaustion
