# MVP roadmap

Last updated: 2026-08-03

## Development rule

Build in small weekday sessions. Every milestone should produce something usable
and teach one transferable engineering concept. Do not reserve a large weekend
for a rewrite.

## Phase 0: prove dynamic generation and delivery

Build one narrow technical spike that:

1. Sends a provider-independent lesson request through the Groq adapter.
2. Produces the required structured lesson sections from the shared prompt.
3. Retries once with correction feedback when validation fails.
4. Splits a substantial lesson on semantic boundaries below Telegram's message
   limit.
5. Sends the complete ordered bundle to one private test chat.
6. Stores the lesson version, parts, provider usage, validation result, delivery
   IDs, and errors in Turso.

The first usable product must generate lessons dynamically. Manually seeded
lessons can be test fixtures, not the primary MVP content source.

## Phase 1: invited user and generated curriculum

Build the thinnest multi-user end-to-end product:

1. Create one Vercel project with a responsive Next.js UI and modular Go API
   functions.
2. Add single-use invite links and Telegram OIDC login with bot-message access;
   do not add email or password signup.
3. Add Turso/libSQL with shared tables and strict `user_id` ownership.
4. Create onboarding for topics, current level, duration, depth, schedule, time
   zone, weekend preference, and Telegram bundle preference.
5. Let the planner choose and persist the active topic and coherent theme.
6. Generate the complete lesson dynamically, then pre-render ordered Telegram
   parts.
7. Deliver the bundle and record `read`, `save`, `skip`, `already_know`, and
   `next` actions.
8. Show only the current theme, next delivery, settings, and progress in the
   simple responsive web app.

Learning outcomes: frontend/backend contracts, authentication, authorization,
relational modelling, profiles, product onboarding, and deployment.

## Phase 2: scheduler, progress, and recall

1. Model topics, prerequisites, learning paths, and theme blocks.
2. Use QStash to wake signed job endpoints and Turso as the durable job source of
   truth.
3. Record completion and relevance feedback.
4. Generate one recall question per completed lesson.
5. Put one due recall question at the beginning of a later scheduled lesson.
6. Continue new deliveries when older lessons remain unread and keep unread
   assignments in backlog.
7. Add progress showing themes learned and due reviews.

Learning outcomes: relational modelling, queries, state machines, product
metrics, scheduling, time zones, retries, idempotency, and evaluation.

## Phase 3: reliable Telegram delivery and free-mode controls

1. Generate and validate every bundle part before its scheduled send.
2. Add bundle-level and part-level idempotency, serial delivery, bounded retry,
   and recovery from a partially delivered bundle.
3. Record provider message IDs and errors without storing more destination data
   than necessary.
4. Support automatic full-bundle delivery and introduction-then-`continue` as a
   per-user choice.
5. Route Groq free quota first and Gemini free quota second.
6. When both fail, persist `blocked_quota`, notify the learner, and send the
   owner a content-free operational alert.
7. Never invoke OpenAI automatically; require an explicit owner enable action.
8. Recover quota-blocked jobs chronologically and deliver the entire backlog.
9. Verify that no normal delivery path can silently invoke a billable fallback.

Learning outcomes: webhooks, external API constraints, rate limits, idempotency,
delivery observability, cost controls, caching, and graceful degradation.

## Phase 4: trusted current-information pipeline

1. Add five to ten authoritative sources only.
2. Fetch and normalize their feeds or release APIs.
3. Store canonical URLs, content hashes, dates, and evidence passages.
4. Deduplicate obvious repeats.
5. Rank candidates using explicit rules.
6. Deliver every candidate that clears the relevance threshold; do not impose a
   separate numerical daily cap.
7. Generate a detailed briefing for each delivered candidate, outside the three
   curriculum lesson slots.

Learning outcomes: ingestion, parsing, polling, rate limits, data provenance,
ranking, and operational failure handling.

## Phase 5: grounded multi-provider AI content

1. Define a provider-independent lesson request and structured result schema.
2. Implement Groq first, Gemini second, and the manual-only OpenAI adapter last,
   all against the same conformance tests.
3. Supply selected source evidence, profile, active theme, duration, depth, and
   curriculum context.
4. Require usefulness, prior approaches, mechanics, production cases,
   trade-offs, future direction, uncertainty labels, and primary links.
5. Validate structured output, semantic requirements, and reading-time bounds.
6. Create an evaluation set covering accuracy, relevance, depth, language,
   citation support, usefulness, and theme continuity.
7. Log provider, model, prompt version, usage, latency, validation, and cost.
8. Compare providers only in an owner-approved evaluation mode. Automatic
   routing remains Groq, then Gemini, then a blocked job; OpenAI is manual-only.
9. Add `deeper`, `example`, and `quiz` actions.

Learning outcomes: tokens, context, structured outputs, grounding, evaluations,
prompt injection, observability, latency, and cost.

## Phase 6: product polish

Complete the responsive web experience:

- Telegram connection and recovery
- Saved-topic metadata and search
- Curriculum progress
- Sources and corrections
- Schedule and topic settings
- A small operations page for failed jobs
- Clear onboarding presets with progressively disclosed advanced settings
- Accessible loading, empty, validation, error, paused, and disconnected-channel
  states

Learning outcomes: product scope, information architecture, authentication,
frontend/backend contracts, and user feedback.

## Phase 7: later experiments

Run one controlled experiment at a time:

- One, two, or three daily lessons and alternate delivery times
- 15-minute versus 30-minute textual lessons
- Alternate radar ranking thresholds
- Written versus audio delivery
- Recall question immediately versus the next day
- Product scenario once per week
- Automatic full bundle versus introduction-then-`continue`

## What not to build initially

- Organizations, teams, billing, or public user discovery
- Social feeds, likes, followers, or public profiles
- Native mobile applications
- Autonomous agents browsing arbitrary sites
- Broad crawling of the entire web
- A vector database before search requirements justify it
- Microservices, Kafka, Kubernetes, or a service mesh
- An elaborate machine-learning recommender
- Gamification that rewards opening without understanding
- A separate database for every user before isolation needs justify it
- A settings page that exposes internal prompt, scheduler, and provider details
- Three model calls for every lesson without an evaluation or fallback reason
- WhatsApp or email delivery before Telegram usage proves another channel is
  necessary

## Suggested portfolio evidence

Preserve the following artifacts as the system develops:

- The original problem statement and rejected alternatives
- Architecture and product decision records
- A data model and migration history
- Reliability tests for retry and duplicate delivery
- A ranking evaluation with false-positive examples
- An AI evaluation dataset and regression results
- Cost, latency, and relevance measurements
- A short postmortem for a real failure
- Evidence of a product change made from usage data

These artifacts demonstrate more than the ability to generate code. They show
problem selection, product judgment, system design, evaluation, and operational
ownership.

## Definition of a successful first version

The MVP is operationally complete when:

- The primary user and at least one friend maintain separate profiles and plans.
- Each user's complete lesson bundles arrive in the connected private Telegram
  chat within the configured best-effort delivery windows.
- Theme lessons remain coherent across morning, noon, night, and following days.
- Most delivered topics are rated relevant.
- Current briefings cite authoritative sources when verification succeeds and
  show the mandatory warning when it does not.
- The service prevents cross-user data access and duplicate messages.
- The service operates without regular manual repair or unapproved recurring
  spend.
- The planner continues selecting useful themes without a fixed course end or
  frequent accidental repetition.

If these conditions are not met, improve the habit, cadence, or content before
adding more infrastructure.
