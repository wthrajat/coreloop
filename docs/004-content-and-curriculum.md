# Content and curriculum system

Last updated: 2026-08-03

## Two coordinated queues

The application should maintain two separate queues and combine them only at
delivery time.

### Curriculum queue

This queue contains durable topics arranged by prerequisites. Its purpose is to
build a coherent technical model over months.

### Radar queue

This queue contains recent developments collected from trusted sources. Its
purpose is awareness and contextual understanding, not chronological coverage
of everything published.

Keeping the queues separate prevents a busy news week from displacing database,
networking, or systems fundamentals.

## Theme blocks and sequencing

Curriculum is delivered as coherent theme blocks rather than random topics. A
theme block has:

- A clear outcome and motivation
- Prerequisite topics
- A planned progression of lessons
- An estimated number of sessions at each duration and depth
- Completion and recall criteria
- A rule for what theme can follow it

All scheduled curriculum lessons on a given day continue the active theme. A
short theme may take two or three days; a foundational or deep theme may take a
week or longer. The planner may extend a theme when recall is weak, but it should
show the user why the schedule changed.

The recommended progression within a theme is:

1. Usefulness, motivation, and the original problem
2. Previous solutions and their limitations
3. Core model and vocabulary
4. Internal mechanics and implementation
5. Production use, failure modes, and trade-offs
6. Alternatives and selection criteria
7. Future direction, interview explanation, and application exercise

Three lessons in one day are three connected parts of that progression, not
three unrelated subjects.

## Curriculum lanes

### 1. Computer systems

- Processes, threads, scheduling, and concurrency
- Memory, files, system calls, and I/O
- TCP/IP, DNS, HTTP, TLS, proxies, and load balancing
- Data structures, complexity, and practical algorithms
- Basic cryptography and security models

### 2. Backend and distributed systems

- API contracts and versioning
- Relational modelling and SQL
- Indexes, query planning, transactions, and isolation
- Caching and invalidation
- Queues, streams, and background processing
- Retries, timeouts, idempotency, and backpressure
- Replication, partitioning, consistency, and reconciliation
- Testing, profiling, migrations, and failure recovery

### 3. Cloud and production engineering

- Linux and containers
- Cloud compute, networks, storage, and managed databases
- IAM, secrets, and least privilege
- Infrastructure as code and Terraform
- CI/CD and safe rollout strategies
- Logs, metrics, traces, SLOs, and incident response
- Capacity, performance, and cost
- Kubernetes after the underlying deployment concepts are understood

### 4. Applied AI

- Tokens, tokenization, context windows, and inference
- Sampling, structured outputs, and tool calling
- Embeddings and retrieval
- Prompt and context design
- Evaluation datasets, metrics, and regression tests
- Hallucination, prompt injection, and data leakage
- Model selection, latency, caching, and cost
- Agent loops, permissions, and human approval
- Fine-tuning and model serving only after the application layer is understood

### 5. Product engineering

- Discovering and defining a real user problem
- Separating user requests from underlying needs
- Customer interviews, observation, and jobs-to-be-done reasoning
- Writing falsifiable problem and value hypotheses
- Scoping a narrow first version
- Writing requirements and acceptance criteria
- Story mapping and identifying the thinnest end-to-end slice
- Selecting outcome metrics and guardrail metrics
- Instrumentation, feedback, and experiments
- Prioritization based on impact, evidence, effort, and risk
- Release planning, migrations, rollbacks, and support
- Balancing correctness, speed, usability, reliability, and cost
- Writing decision records and learning from incidents
- Positioning, demos, objections, distribution, and basic pricing reasoning

### 6. Effective AI leverage

- Turning an ambiguous goal into a specification and verification plan
- Supplying the right context without flooding the model
- Choosing between direct generation, tools, retrieval, and deterministic code
- Designing evaluation cases before trusting output
- Using agents for bounded workflows with permissions and stopping conditions
- Reviewing model output for correctness, security, maintainability, and intent
- Measuring tokens, latency, caching, retries, quality, and cost
- Deciding what should remain a human judgment or accountable decision

### 7. Communication and practical sales

- Clear status updates, technical explanations, and decision summaries
- Design documents, proposals, and asynchronous collaboration
- Asking precise questions and uncovering hidden requirements
- Communicating uncertainty, risk, disagreement, and bad news
- Explaining a product's value to technical and non-technical audiences
- User discovery, value propositions, demos, and objection handling
- Interview communication and evidence-backed storytelling

### 8. Career translation

- System-design and debugging interviews
- Explaining trade-offs without jargon
- Turning Bitcoin work into transferable systems evidence
- Writing achievement-oriented project descriptions
- Reading a job description and identifying real skill gaps

## Initial topic sequence

The first sequence should support building Coreloop itself:

1. HTTP requests, APIs, and webhooks
2. Scheduled jobs and time zones
3. Idempotency and retry behaviour
4. Relational schemas and state transitions
5. Indexes and query plans
6. Authentication, tokens, and secret storage
7. Containers and process lifecycle
8. Cloud deployment and IAM
9. Logs, metrics, traces, and alerting
10. RSS/Atom, polling, and change detection
11. Deduplication and ranking heuristics
12. LLM tokens and context construction
13. Structured model outputs
14. Grounding, citations, and provenance
15. AI evaluation and regression testing
16. Product metrics and feedback experiments

This makes each lesson immediately applicable to the product being built.

## Ongoing curriculum loop

The curriculum has no fixed number of weeks and no terminal lesson cap. The
planner repeatedly:

1. Chooses the next useful theme from the learner's topics and current level.
2. Expands it into a coherent sequence at the selected duration and depth.
3. Stores every topic, objective, lesson version, and assignment before
   delivery.
4. Advances after the learner marks a lesson as read, while continuing scheduled
   delivery even when older lessons remain unread.
5. Suppresses accidental repetition using topic IDs, objectives, content
   fingerprints, and delivery history.
6. Permits an explicit repeat only for deeper continuation, spaced recall,
   failed-recall remediation, or a material change in the technology.
7. Selects the next theme and continues indefinitely.

A friend's independent profile can follow a different path, including a
cryptocurrency path, without affecting another learner's curriculum.

## Source hierarchy for current information

### Tier 1: authoritative sources

- Official documentation and changelogs
- Standards bodies, specifications, RFCs, and security advisories
- Official project repositories and release pages
- Vendor engineering and security announcements
- Original research papers and accompanying project pages

### Tier 2: strong context sources

- Engineering blogs describing real production systems
- Technical conference material from the people who built the system
- High-quality independent analysis that links to its evidence

### Tier 3: discovery-only sources

- Aggregators, social media, discussion sites, and newsletters

Tier 3 can identify a candidate development, but a card should link back to the
original source before presenting a factual claim as confirmed.

Suggested source groups include official feeds or changelogs from major model
providers, cloud providers, CNCF projects, PostgreSQL, GitHub releases, language
runtimes, security databases, standards bodies, and arXiv categories relevant to
production AI. Start with a small curated list and measure usefulness before
adding broad feeds.

## Current-signal ranking

Each candidate can receive explicit scores from 0 to 5:

- **Career relevance:** relation to backend, cloud, AI, product, or security
- **Authority:** quality and proximity of the source
- **Novelty:** whether it adds information not already delivered
- **Practical impact:** likelihood that it changes a design or working practice
- **Educational value:** whether it teaches a reusable concept
- **Recency:** time since publication, with topic-dependent decay
- **Hype penalty:** unsupported claims, promotional language, or weak evidence

The reason for selection should be stored with the score. Begin with transparent
weights and adjust them from feedback; do not start with an opaque recommender.

## Content production pipeline

1. Fetch and timestamp the source.
2. Extract title, author, publication date, update date, and canonical URL.
3. Preserve the relevant source passages separately from generated text.
4. Detect duplicates and cluster multiple reports about the same development.
5. Apply source-quality and relevance rules.
6. Generate a structured draft grounded only in the selected evidence.
7. Check that citations resolve and that important factual statements have
   support.
8. If information or required sections fail validation, retry once with explicit
   correction feedback.
9. If the second result is still structurally imperfect, deliver it. If facts or
   citations remain unverifiable, place a prominent warning first and separate
   verified facts, model interpretation, and unverified claims.
10. Fit the content into the appropriate lesson format.
11. Store the exact delivered version for later correction and evaluation.

## Quality rubric

A lesson is ready only if it is:

- Technically correct at the promised level
- Convincing about why the topic exists and when it is useful
- Honest when real usefulness is narrow, contested, or unproven
- Connected to prerequisites and real use
- Explicit about trade-offs and failure modes
- Written in plain but professional language
- Useful for an interview or engineering decision
- Sized for the available attention window
- Cited when it depends on current or external facts

## Retention loop

Every lesson ends with one optional immediate recall question. At most one due
spaced-recall question appears at the beginning of a later scheduled lesson and
counts inside that lesson's reading-time budget. Recall never creates a separate
notification stream and never blocks later delivery.

Questions should require retrieval or application rather than recognition. For
example, prefer “Why can retrying this request create two payments?” over “Is
idempotency useful?”

## Ranking current information

- Radar delivery can be disabled per user, but it has no fixed daily cap when
  enabled and does not consume curriculum lesson slots.
- Deliver only candidates that clear the transparent relevance threshold; the
  ranker, rather than a numerical quota, is the spam control.
- Group several announcements about the same change into one card.
- Treat `skip` as negative relevance feedback without stopping future radar
  delivery.
- Render every delivered radar item as a detailed, plain-language technical
  briefing rather than a headline-only card.
- State “no meaningful update today” internally rather than inventing content.
- Never confuse popularity with relevance.
