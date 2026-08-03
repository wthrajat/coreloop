# Product concepts and recommendation

Last updated: 2026-08-03

## The actual problem

The problem is not a shortage of educational material. The internet already has
more courses, articles, papers, release notes, and videos than one person can
consume.

The problem has five parts:

1. Deciding what is worth learning
2. Fitting it into fragmented weekday time
3. Explaining it at the correct technical depth
4. Separating important developments from hype
5. Retaining and applying what was read

A cron job solves delivery. It does not by itself solve selection, sequencing,
trust, retention, or application.

## Concept A: scheduled topic messenger

The service chooses a topic and sends an explanation every few hours.

Strengths:

- Small and quick to build
- Meets the user in an existing messaging application
- Removes the decision about what to study
- Teaches scheduling, delivery APIs, deployment, and retries

Weaknesses:

- Frequent pushes can become background noise
- Random topics do not create a coherent mental model
- Reading without recall produces weak retention
- The content can become shallow or repetitive

Verdict: useful as a delivery mechanism, not a complete product.

## Concept B: personal technology radar

The service monitors trusted sources, clusters related announcements, and sends
only developments relevant to the user's career goals.

Strengths:

- Directly addresses the fear of discovering changes too late
- Can prioritize official release notes, standards, repositories, and research
- Teaches ingestion pipelines, ranking, deduplication, and source verification

Weaknesses:

- News creates awareness more readily than durable understanding
- Recency can crowd out important fundamentals
- A relevance-ranking system needs feedback before it becomes trustworthy

Verdict: an important layer, but it should not control the whole curriculum.

## Concept C: interactive curriculum coach

The service maintains a dependency-aware curriculum, sends a lesson, asks a
small recall question, and schedules future review based on the response.

Strengths:

- Creates progression instead of random exposure
- Recall and spaced review improve the chance of retention
- Can adapt depth based on demonstrated understanding
- Produces useful interview practice over time

Weaknesses:

- A static curriculum can become stale
- It does not independently solve current-technology awareness
- Fully AI-generated sequencing can be inconsistent without explicit rules

Verdict: the correct learning core, but it needs a current-information layer.

## Concept D: production engineering simulator

The service sends small scenarios such as an overloaded database, a duplicate
payment, an unsafe model action, or a failed rollout. The user chooses a response
and receives a technical debrief.

Strengths:

- Develops judgment rather than vocabulary
- Connects fundamentals, operations, security, and product consequences
- Generates strong interview discussion material

Weaknesses:

- High-quality scenarios take more work to produce and validate
- It is not the fastest initial version to ship
- It is most valuable after some foundational concepts are established

Verdict: an excellent second-stage feature.

## Concept E: a combined Coreloop

This combines the strongest parts of the preceding concepts:

- A structured, dependency-aware curriculum
- A filtered current-technology radar
- Interactive recall and spaced review
- Production and product-engineering scenarios
- A hosted web application with individual profiles and learning plans
- Telegram-first delivery of the complete textual lesson as an ordered message
  bundle
- A deliberately small responsive web control surface rather than a second
  lesson-reading experience

This is the recommended product.

## Multi-user scope

Sharing the application with friends changes the design in a useful way. The
system is no longer a personal script with one deployment-wide topic list. Each
person needs an account and a profile containing:

- Desired subjects and target outcomes
- Existing knowledge and preferred starting level
- Lesson duration and explanation depth
- Lessons per day and delivery windows
- Weekend and time-zone settings
- Delivery channel and verified destination
- Theme progress, recall results, saves, and feedback

A profile interested in backend and AI must be able to coexist with a profile
interested in cryptocurrency without either person's content affecting the
other. Shared source ingestion and reusable lesson material can reduce cost, but
selection and progress remain per user.

## Why the combined product is the best choice

It serves two goals at once. The user receives a sustainable learning system,
and the act of building that system creates relevant portfolio evidence in
backend, cloud, AI, and product engineering.

The product itself becomes a practical syllabus:

| Product capability | Engineering lesson |
| --- | --- |
| Next.js frontend and Go API | Contracts, authentication, authorization |
| Profile-based configuration | Multi-tenancy, validation, settings UX |
| Telegram bot and message bundles | Webhooks, API limits, retries, idempotency |
| Scheduled delivery | Jobs, time zones, retries, idempotency |
| Curriculum state | Relational modelling and state transitions |
| Source ingestion | Fetching, parsing, rate limits, change detection |
| Deduplication and ranking | Search, similarity, heuristics, evaluation |
| AI-generated explanations | Structured outputs, grounding, prompt design |
| Citation verification | Data provenance and failure handling |
| Feedback buttons | Product telemetry and preference learning |
| Cloud deployment | Containers, IAM, secrets, observability, cost |
| Relevance evaluation | Product metrics and experiment design |

## Product-engineering value

Product engineering is not simply writing product code. It is owning the path
from an observed human problem to a measurable improvement. For Coreloop,
that means testing whether three notifications create learning rather than
assuming that the requested frequency is correct, measuring recall rather than
opens, and changing scope when a channel constraint makes the experience worse.

The project should deliberately practise:

- User interviews and problem statements
- Hypotheses and smallest useful experiments
- Prioritization and explicit non-goals
- Metrics, instrumentation, and feedback interpretation
- Technical, product, reliability, security, and cost trade-offs
- Rollout, support, correction, and communication
- Explaining and positioning the product so another person wants to use it

## Additional products that can grow from the same foundation

These should be later modes, not separate projects initially:

1. **Interview memory bank:** converts saved lessons into interview questions
   and schedules recall.
2. **Technical change tracker:** watches a selected technology's documentation,
   releases, and deprecations and explains what changed.
3. **Production scenario gym:** sends one realistic engineering decision each
   week and evaluates the reasoning.
4. **Personal project mentor:** turns a product idea into small implementation
   decisions and reviews the resulting trade-offs.
5. **Commute audio mode:** produces a concise spoken version while retaining the
   sources and detailed written explanation.
6. **Distraction replacement surface:** exposes the current lesson through a
   phone home-screen shortcut, a small PWA, or a desktop new-tab page. It should
   not attempt fragile automation against Instagram itself.

## Product principle

Do not optimize for consuming more information. Optimize for repeatedly turning
relevant information into understood, retained, and usable knowledge.
