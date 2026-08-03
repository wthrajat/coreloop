# Implementation plan

Last updated: 2026-08-03

## Status

The complete production implementation was finished locally on 2026-08-03.
All repository release gates pass. Live acceptance remains credential-gated:
provision Turso, Vercel, QStash, Telegram, and provider secrets, then exercise
their hosted contracts using `deployment.md`. Concrete files and validation
evidence are recorded in `012-implementation-log.md`.

## Selected stack

| Concern | Selection |
| --- | --- |
| Web UI | Next.js with TypeScript |
| Backend | Go Vercel Functions |
| Database | Turso/libSQL free plan |
| Authentication | Telegram OIDC with PKCE |
| Delivery | Telegram Bot API webhooks and inline buttons |
| Scheduling and queue wake-ups | Upstash QStash free plan |
| AI routing | Groq, then Gemini; OpenAI manual-only |
| Hosting | One Vercel Hobby project |
| Language | English-only |

The frontend and backend deploy together for zero-cost operations but remain
separate modules and contracts. Next.js does not contain business logic, and Go
does not render the product UI.

## Repository structure

```text
coreloop/
├── app/                         # Next.js routes and layouts
├── components/                  # small reusable UI components
├── lib/                         # browser-safe frontend helpers
├── api/
│   ├── app/index.go             # authenticated web API entry point
│   ├── telegram/index.go        # Telegram webhook entry point
│   └── jobs/index.go            # signed QStash job entry point
├── backend/
│   ├── app/                     # public facade used by function entry points
│   └── internal/
│       ├── auth/                # invites, Telegram OIDC, sessions
│       ├── profiles/            # learner settings and ownership
│       ├── curriculum/          # theme and next-topic planner
│       ├── content/             # prompt compiler, contract, validation
│       ├── providers/           # Groq, Gemini, OpenAI adapters
│       ├── cache/               # application cache keys and policy
│       ├── radar/               # ingestion, ranking, provenance
│       ├── jobs/                # durable jobs, leases, recovery
│       ├── telegram/            # rendering, chunking, delivery
│       └── store/               # Turso queries and transactions
├── migrations/                  # ordered SQL migrations
├── testdata/                    # prompts, sources, evaluation cases
├── package.json
├── go.mod
├── vercel.json
└── 001-*.md ... 011-*.md        # product and architecture history
```

Keep three Go entry points so Vercel functions remain below the Hobby direct
function limit. Route internally with ordinary Go packages rather than creating
one function for every endpoint.

## Web experience

The UI uses a restrained product interface with familiar controls, one type
family, clear focus/error/loading states, and structural mobile layouts. It does
not need elaborate cards, animation, or a lesson reader.

Initial routes:

- `/invite/[token]`: validates the invite and starts Telegram login
- `/auth/callback`: completes OIDC and creates the application session
- `/onboarding`: topics, level, duration, depth, schedule, weekends, bundle mode
- `/overview`: active theme, next delivery, connection, and quota failure state
- `/progress`: read/unread assignments, themes, saves, and recall state
- `/settings`: learning and delivery configuration, export, and delete

Use a short onboarding preset first and reveal advanced timing controls only
when requested.

## Authentication and sessions

1. The owner creates a random, single-use invite through an owner-only CLI or
   endpoint.
2. Store only its hash, expiry, and consumption state.
3. The friend opens the invite and starts Telegram OIDC Authorization Code Flow
   with PKCE, state, and nonce.
4. Request `openid`, `profile`, and `telegram:bot_access`; do not request a phone
   number.
5. The Go callback validates signature, issuer, audience, expiry, state, nonce,
   and PKCE before trusting the Telegram subject.
6. Consume the invite and create the user transactionally.
7. Issue an opaque, rotating session stored as a secure, `HttpOnly`, `SameSite`
   cookie; store only the session-token hash in Turso.
8. Send a Telegram welcome message to prove direct-message access.

The owner identity is an allowlisted Telegram subject stored as a Vercel secret.
No public signup endpoint exists.

## Core data model

The first migrations should create:

- `users`, `sessions`, and `invites`
- `learning_profiles`, `learning_preferences`, and `delivery_schedules`
- `user_topic_preferences`, `topics`, `learning_paths`, and `theme_blocks`
- `sources`, `source_items`, and `radar_candidates`
- `lessons`, `lesson_parts`, and `lesson_assignments`
- `deliveries`, `delivery_parts`, and `interactions`
- `reviews`, `job_queue`, and `provider_runs`
- `prompt_versions` and `cache_entries`

Every user-owned table includes `user_id`; every query that reads or mutates it
receives the authenticated user ID from server-side session context. Shared
lesson rows never contain a learner's name or private profile text.

Use explicit enums or checked strings for job, lesson, assignment, delivery,
verification, and provider states. Important unique constraints include:

- Telegram OIDC subject
- Invite-token hash
- Lesson cache key and version
- Assignment position within a theme
- Bundle idempotency key
- Delivery-part idempotency key
- Provider request idempotency key where supported

## Durable job flow

QStash wakes the application; Turso owns job truth.

1. One signed QStash schedule invokes `/api/jobs/tick` about every ten minutes.
2. The tick transaction leases a bounded number of due jobs using lease expiry
   and first-come sequence.
3. It publishes individual signed QStash calls for generation, delivery, source
   ingestion, or recovery.
4. A worker renews or completes its database lease and records every transition.
5. A repeated QStash call observes the completed idempotency key and becomes a
   no-op.
6. Expired leases become eligible for recovery.

Do not perform unbounded work inside the tick request. QStash free usage must be
monitored because the ten-minute tick alone consumes roughly 144 messages per
day before work-item and retry messages.

## Curriculum planning

The deterministic planner receives topic preferences, current level, theme
history, delivered objectives, content fingerprints, recall evidence, duration,
and depth. It either continues the active theme or creates a new theme plan.

The planner stores its decision before generation. It rejects a proposed topic
when substantially equivalent objectives were recently delivered, unless the
assignment is explicitly classified as continuation, review, remediation, or
material update. There is no course completion state; finishing one theme causes
selection of another.

Unread assignments do not affect scheduling. The `read` callback advances
progress and can create a due recall item.

## Lesson-generation contract

`GenerateLesson(request) -> LessonDraft` is owned by the application. The request
contains:

- Contract, prompt, and schema versions
- Topic, objectives, prerequisites, and theme position
- Current level, target duration, and requested depth
- Selected source evidence with canonical URLs and dates
- Required sections and validation rules

The result separates:

- Metadata and estimated reading time
- Motivation, prior approaches, definition, and mechanics
- Production example, trade-offs, failure modes, and alternatives
- Security, reliability, performance, and cost considerations
- Present maturity, future direction, and interview explanation
- Recall question
- Claims, citations, interpretation, and uncertainty

Provider adapters translate the contract but return one normalized Go type.
Application validation remains mandatory even when a model offers strict JSON
schema output.

## Efficient context and prompt compiler

Do not build prompts by concatenating whole database records. Create a typed
`LessonContext` and compile it deterministically.

### Stable instruction prefix

The stable prefix contains, once:

- The teaching objective and plain-but-technical writing policy
- The usefulness-first lesson order
- Factual, citation, uncertainty, and unverified-warning rules
- The distinction between theory and optional exercises
- The normalized output contract and refusal behaviour

Give the prefix a version and checksum. Keep wording and ordering stable between
requests so provider-native prefix caching can work. Do not repeat the same rule
in both system and user messages. Use provider-native structured-output
parameters for the schema instead of spelling the full schema repeatedly in
prose when the provider supports them.

### Compact dynamic payload

The per-lesson payload contains only:

- Topic ID and title
- Current level, duration preset, and depth preset
- Active theme ID, lesson position, and unmet objective IDs
- Small prerequisite and already-covered objective lists
- At most one due recall item
- Deduplicated evidence passages with source IDs, canonical URLs, and dates

Do not send names, full profiles, unrelated goals, full topic history, previous
lesson bodies, Telegram messages, or raw source pages. Curriculum history is
reduced deterministically to identifiers and a small theme-state summary.

“Compressed” must remain readable and lossless at the requirements level. Use
short field names only when the mapping is typed and tested; do not create a
cryptic minified prompt that models interpret inconsistently.

### Token-budget policy

Before every provider call:

1. Estimate or count prompt tokens using the selected provider's tokenizer when
   available.
2. Reserve the calibrated output budget required for the configured 15- or
   30-minute lesson.
3. Allocate the remaining input budget among instructions, theme state, and
   evidence.
4. Rank evidence passages by objective coverage, authority, and recency.
5. Remove duplicate or low-value passages before removing any required rule.
6. Fail with a typed context-budget error if required evidence cannot fit; never
   truncate silently in the middle of a claim.

The selected limits are configuration owned by the provider adapter and model
registry, not scattered prompt constants.

## Cache hierarchy

Use several small explicit caches instead of introducing a vector database:

1. **Source HTTP cache:** ETag, Last-Modified, canonical URL, retrieval time, and
   content hash prevent unchanged pages from being downloaded and processed.
2. **Normalized-source cache:** parsed headings and useful passages are stored by
   content hash.
3. **Radar cache:** deduplication and ranking results are stored by source-set and
   ranker version.
4. **Lesson cache:** the key hashes topic, objectives, level, duration, depth,
   source versions, prompt version, and schema version. It deliberately excludes
   user identity and provider name.
5. **Prompt-input cache:** compiled stable prefix and selected evidence can be
   reused across a fallback without rebuilding or re-reading sources.
6. **Render cache:** Telegram parts are stored by lesson version and renderer
   version and never regenerated during delivery retries.
7. **Interactive cache:** deterministic `sources` responses and previously
   generated compatible examples can be reused; personalized recall answers are
   never shared.

Cache only verified immutable outputs. A correction creates a new lesson
version. Cache invalidation is driven by versioned keys rather than broad delete
operations.

Provider-native prompt caching is an optimization below the application cache.
Keep stable prompt material first, record provider-reported cached-token usage,
and enable explicit caching only when the selected provider/model/free plan
supports it and measurements show a benefit. Application correctness must not
depend on a provider cache hit.

## Avoiding unnecessary AI work

- Use SQL and Go for scheduling, topic repetition checks, source filtering,
  ranking arithmetic, deduplication, hashes, dates, URLs, retries, chunking, and
  Telegram state.
- Do not ask an LLM to select sources from the entire ingestion database. Code
  supplies a small evidence set.
- Do not maintain a conversational AI thread per learner. Each generation is an
  independent request with compact state.
- Do not generate all future lessons. Pre-generate only the next small delivery
  horizon so changed settings and new evidence do not invalidate a large cache.
- Do not run three-provider comparisons in production.
- Do not add embeddings or a vector database until exact topic/objective IDs,
  SQL search, and content hashes fail on observed cases.
- Use one corrective generation at most. Subsequent formatting fixes are
  deterministic; unresolved information receives the warning instead of an
  unbounded retry loop.

## Provider state machine

1. Check the lesson cache.
2. Call Groq.
3. Respect a short `Retry-After` for transient rate limits; classify daily quota
   exhaustion separately.
4. On a quota-class or unavailable-provider failure, call Gemini.
5. If both free providers fail, set the job to `blocked_quota`, notify the user,
   and emit a content-free owner alert.
6. Never route a scheduled job to OpenAI.
7. An owner-only manual command may run a specific blocked job with OpenAI; this
   is a separate path with an explicit audit record.

For invalid content, send one corrective request to the same provider containing
the validation failures. After the second response:

- Structural or formatting defects do not block delivery if the content remains
  usable.
- Unverified factual material receives the mandatory warning and labelled
  separation before delivery.

When free quota recovers, lease `blocked_quota` jobs in original creation order
and deliver all recovered lessons.

## Telegram rendering and delivery

- Render immutable parts from semantic lesson sections.
- Target about 3,500 characters per `sendMessage` part.
- Label every part `Part i/N` and include theme, lesson, and reading time in the
  first part.
- Put sources, optional recall, and inline actions in the final part.
- Generate and store the whole bundle before sending part one.
- Send parts serially and persist each Telegram message ID.
- Resume from the first unconfirmed part after a retry.
- Default to complete-bundle delivery; retain introduction-then-`continue` as a
  profile setting.
- Record `read`, `save`, `skip`, `already_know`, `next`, `deeper`, `example`, and
  `quiz` callbacks idempotently.

## Current-tech radar

Start with a small owner-managed allowlist of official documentation,
changelogs, release feeds, repositories, standards, security advisories, and
primary research pages. Users select topics but cannot add URLs.

Fetch and store provenance deterministically. Score topic match, career utility,
technical impact, actionability, source authority, novelty, duplication, and
hype. Deliver every candidate above the configured threshold; there is no daily
cap. Generate the same detailed structure and verification warning used by
curriculum lessons, and do not consume a curriculum slot.

## Privacy and operational visibility

- The application has no admin content browser.
- Logs omit prompts, lesson bodies, profile text, recall answers, Telegram names,
  and message contents.
- Alerts contain opaque job/user IDs, provider category, error class, and time.
- Learners see their own quota failure directly in Telegram.
- Store `ADMIN_ALERT_EMAIL` and all provider credentials only as Vercel secrets;
  never commit the personal address.
- Prefer a Telegram owner alert first because the delivery integration already
  exists. Add a free transactional-email adapter only if the owner still needs
  email after the core flow works.

## Delivery sequence

### Milestone 1: skeleton and persistence

- [x] Initialize Next.js, Go, formatting, tests, and Vercel configuration.
- [x] Create the initial Turso migration and typed store boundaries.
- [x] Add health checks and structured error types.

### Milestone 2: generated lesson spike

- Implement the compact prompt compiler, token budgets, lesson contract, Groq
  adapter, validation, one correction retry, semantic chunking, and delivery to
  the owner's test chat.
- Store request field versions and hashes, normalized output, parts, and provider
  result without logging or duplicating the raw compiled prompt.

### Milestone 3: invited multi-user slice

- Add invite creation, Telegram OIDC, sessions, onboarding, profiles, and strict
  ownership tests.
- Generate and deliver one profile-specific theme end to end.

### Milestone 4: scheduling and recovery

- Add QStash verification, durable job leases, schedule expansion, idempotency,
  unread backlog, `read`, and partial-bundle recovery.

### Milestone 5: free-provider routing

- Add Gemini fallback, quota classification, blocked backlog, learner error
  messages, owner alerts, recovery, and manual-only OpenAI adapter.

### Milestone 6: ongoing planner and recall

- Add theme continuity, stored topic selection, repetition suppression, embedded
  recall, shared lesson cache, and indefinite next-theme selection.

### Milestone 7: efficiency hardening

- Add source, lesson, prompt-input, and render caches.
- Record prompt input/output tokens, provider cache fields, application cache hit
  types, retry causes, latency, and quota headers.
- Run the same lesson evaluation set before and after prompt compression. Remove
  repeated instructions one group at a time and keep the shorter prompt only if
  required-section coverage, correctness, usefulness, and citation support do
  not regress.

### Milestone 8: current-tech radar

- Add the curated source allowlist, polling, provenance, deduplication, ranking,
  detailed radar generation, and skip feedback.

### Milestone 9: hardening

- Add export/delete, correction handling, accessible UI states, operational
  views without private content, quota dashboards, and deployment documentation.

## Required verification

Before inviting friends, verify with automated tests:

- OIDC state, nonce, issuer, audience, expiry, PKCE, invite reuse, and session
  rotation
- Cross-user access rejection on every user-owned resource
- Topic repetition classification and ongoing theme selection
- Provider routing never invokes OpenAI from a scheduled job
- One corrective generation attempt and mandatory unverified warning
- Telegram 4,096-character safety after formatting
- Bundle and part idempotency under repeated callbacks
- Recovery after failure between two Telegram parts
- QStash signature rejection, durable leases, expiry, and duplicate invocation
- First-come processing and chronological quota-backlog recovery
- Unread lessons do not pause later delivery
- Logs and alerts contain no private lesson or profile content
- Prompt compilation never includes full profile rows, prior lesson bodies, or
  unrelated sources
- Lesson-cache keys prevent cross-level, cross-depth, stale-source, and
  stale-prompt reuse
- Prompt compression preserves every required lesson section on the evaluation
  set while reducing median input tokens

Run the narrow Go and frontend tests first, then the full suite, migration test,
and one deployed end-to-end test using a private Telegram account.

## Free-tier constraints verified on 2026-08-03

- Vercel Hobby is free and supports Go Functions, but its native cron is limited
  to once daily with imprecise timing.
- Turso advertises a no-credit-card free plan.
- QStash advertises a $0 plan with scheduled messages and 1,000 messages per day;
  keep the account on the free plan without a payment method.
- Groq publishes organization-level free-plan rate limits. Exact model limits
  must be read from the owner's account before selecting the production model.

References:

- [Vercel Hobby](https://vercel.com/docs/plans/hobby)
- [Vercel Go runtime](https://vercel.com/docs/functions/runtimes/go)
- [Vercel Hobby cron limits](https://vercel.com/docs/cron-jobs/usage-and-pricing)
- [Turso pricing](https://turso.tech/pricing)
- [QStash pricing](https://upstash.com/pricing/qstash)
- [QStash schedules](https://upstash.com/docs/qstash/features/schedules)
- [Groq rate limits](https://console.groq.com/docs/rate-limits)
- [OpenAI model guidance: lean prompts and cache measurement](https://developers.openai.com/api/docs/guides/latest-model)
- [Telegram Login](https://core.telegram.org/bots/telegram-login)
- [Telegram Bot API](https://core.telegram.org/bots/api)
