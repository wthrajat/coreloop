# Recommended product requirements

Last updated: 2026-08-03

## Product name

Coreloop

Coreloop is the final brand name selected on 2026-08-04. It refers to the
continuous loop of building understanding across computer science and software
systems, not specifically to employment, interviews, or career progression.

## Product objective

Deliver trustworthy, technically substantial, profile-specific learning in
configurable daily windows while maintaining awareness of important current
developments.

## Default daily experience

The product default is:

- Time zone: `Asia/Kolkata`
- Lesson duration: 15 minutes of textual reading per lesson
- Lesson frequency: three lessons per day
- Delivery windows: morning, noon, and night
- Weekend delivery: disabled
- Theme behavior: all three lessons continue the same active theme
- Local delivery times: 09:00, 14:00, and 21:00

The 15-minute duration applies to each lesson, not the whole day's reading
budget. All defaults initialize a profile and remain configurable. Users can
request more with `next`. Missed lessons return to the queue without breaking a
streak or creating guilt messages.

## Configurable learning profile

The settings model should support these per-user choices:

| Setting | Initial choices |
| --- | --- |
| Time zone | IANA time zone; `Asia/Kolkata` default |
| Lesson duration | 15 or 30 minutes |
| Explanation depth | Foundation, standard, or deep |
| Lessons per day | 1-6 within bounded delivery windows |
| Delivery days | Individual weekdays plus weekend toggle |
| Delivery times | Explicit local-time slots |
| Learning tracks | One or more selected tracks with priorities |
| Current-tech radar | Enabled or disabled; no daily cap when enabled |
| Recall frequency | Off, light, or standard |
| Telegram delivery | Connected, paused, or disconnected |
| Bundle delivery mode | Send the complete bundle automatically, or wait for `continue` after the introduction |
| Quiet periods | Pause-until date and recurring quiet hours |

Duration and depth are separate. A 15-minute deep lesson does not compress an
entire deep topic into one message; it divides that topic across more connected
sessions. The UI should offer sensible presets and reveal advanced controls only
when requested.

## Theme continuity

The scheduler must select an active theme block before selecting an individual
lesson. A theme may run for two or three days, a week, or longer depending on
its dependency graph and the chosen depth.

For example, a Terraform theme could progress as follows:

1. Motivation, prior approaches, and the problem with unmanaged infrastructure
2. Desired state, providers, resources, and dependency planning
3. State files, locking, drift, and collaboration
4. Modules, environments, secrets, and deployment workflow
5. Failure modes, alternatives, future direction, and interview explanation

Morning, noon, and night sessions continue this sequence. The current-tech radar
is labelled separately so that a news item is not mistaken for a curriculum
jump.

## Lesson types

### Foundation lesson

A curriculum topic such as database isolation, DNS resolution, Terraform state,
tokens, embeddings, or idempotency.

### Current signal

A meaningful release, deprecation, standard, security event, paper, or
engineering development. It must say what changed, why it matters, and whether
the user needs to act. It is a detailed briefing rather than a short headline,
does not consume a curriculum slot, and may span several Telegram parts.

### Product decision

A short scenario involving scope, user need, metrics, rollout, pricing,
reliability, or a technical/product trade-off.

### Production scenario

A realistic system failure or design decision followed by a debrief.

### Recall lesson

A question or compact exercise derived from earlier material.

## Structure of a substantial lesson

A complete topic can span several connected sessions. Each individual lesson is
delivered as a Telegram message bundle and should use as many of these sections
as the subject requires:

1. Topic and expected reading time
2. Why the topic is worth learning
3. The real problem and historical motivation
4. What people used before it and why that was insufficient
5. Prerequisites and connection to prior lessons
6. Precise definition
7. Internal mechanics and data flow
8. A realistic production example and demonstrated use case
9. Trade-offs, failure modes, and when not to use it
10. Alternatives and how to choose among them
11. Security, reliability, performance, and cost implications
12. Present maturity and plausible future direction
13. Relationship to the user's target career
14. A concise interview-ready explanation
15. One or two recall or application questions
16. Primary sources with publication or update dates

Plain language must not remove important technical terms. Define a term before
using it repeatedly. Prefer real system comparisons to toy metaphors.

## Interaction controls

Telegram inline buttons should offer actions for:

- `deeper`: continue the current subject at greater depth
- `example`: show code, data flow, or a production case
- `quiz`: test recall or apply the idea to a scenario
- `save`: add the lesson to the personal knowledge archive
- `skip`: deprioritize this lesson without marking the whole subject irrelevant
- `not_relevant`: lower the topic or source weight
- `already_know`: move to a harder lesson or assessment
- `next`: request another lesson without waiting for the schedule
- `sources`: show primary links and dates
- `pause`: silence scheduled delivery for a chosen period
- `settings`: open the web settings surface
- `read`: mark the assignment completed and advance topic progress

Only the explicit `read` callback counts as completion. Delivery does not imply
that a lesson was read. An unread lesson remains in backlog, but it never blocks
later scheduled lessons or triggers guilt-based reminders.

## Accounts and personalization

The initial profile should record:

- Current experience and target roles
- Familiar, weak, and unknown areas
- Preferred lesson depth
- Available weekdays and quiet hours
- Current technologies used at work
- Target technologies and cloud provider
- Saved, completed, skipped, and failed-recall topics
- Source and topic relevance feedback
- Lesson duration, depth, frequency, days, times, and Telegram connection state
- Active learning path and theme block

Personalization should initially be rule-based and visible. A user should be able
to understand why a topic was selected. Machine-learned ranking is unnecessary
for a small multi-user MVP. Every read and write must be authorized to the
current user; a person's profile, progress, and destinations are private.

Generated wording is personalized only by selected topic, current level,
duration, and depth. Compatible users may share the same immutable lesson
version, while assignments, delivery, progress, feedback, and recall remain
private. Names and unrelated profile details are not inserted into model prompts.

## Web application requirements

The web interface should be simple, responsive, and task-focused. It is a
control surface, not a parallel lesson reader. Initial authenticated surfaces
are:

- **Overview:** active theme, next delivery, Telegram connection, and due recall
- **Learning plan:** selected tracks, theme sequence, and progress
- **Progress:** completed lessons, recall evidence, saved-topic metadata, and
  weak areas
- **Profile:** goals, current level, target roles, and learning interests
- **Settings:** schedule, duration, depth, Telegram delivery, quiet hours, and
  data controls

Use familiar forms, one restrained component vocabulary, clear focus and error
states, and structural mobile layouts. Configuration should not appear as one
enormous form. Onboarding establishes a goal and a preset; advanced options use
progressive disclosure. Full lesson text is stored by the backend for delivery,
auditing, and recovery but does not require an MVP web reading view.

## Success measures

The main product metrics should be:

- Completed learning sessions per week
- Correct recall after approximately 7 and 30 days
- Percentage of lessons rated relevant
- Number of topics progressed from unfamiliar to explainable
- Number of product or engineering ideas applied in real work or the project

Secondary operational metrics include delivery success, generation failures,
source freshness, citation coverage, latency, and cost per completed lesson.

The following are not primary success metrics:

- Total messages sent
- Total time in the application
- Number of articles ingested
- A streak without evidence of recall

## Trust requirements

- Every current-event card must link to at least one primary source when a
  primary source exists.
- The card must show when the source was published or updated and when the
  system retrieved it.
- Generated content must not invent quotes, dates, benchmark numbers, versions,
  or source claims.
- Interpretation must be labelled separately from sourced fact.
- Conflicting sources should be shown as unresolved instead of silently merged.
- When important information remains unverifiable after one corrective
  regeneration, the system may deliver it only with a prominent warning that
  explains the failure and visibly separates verified facts, interpretation,
  and unverified claims.

## Telegram lesson-delivery decision

Telegram is the primary and only MVP delivery channel. The complete lesson is
sent to the learner's private bot chat as an ordered bundle, so reading does not
depend on opening the web app.

The account and delivery flow is:

1. The owner sends a close friend a single-use application invite link.
2. The friend authenticates with Telegram OIDC using `openid`, `profile`, and
   `telegram:bot_access`; no email address, password, or separate signup exists.
3. The backend validates the ID token, consumes the invite, creates the profile,
   and confirms that the bot can send direct messages.
4. Before the scheduled time, the system generates, verifies, chunks, numbers,
   and stores every part of the lesson.
5. At the scheduled time, it sends `Part 1/N` through `Part N/N` serially and
   records the Telegram message ID and state of every part.
6. The final message contains recall and lesson actions. `settings` opens the
   small web control surface.

Chunk on semantic section boundaries rather than splitting arbitrary character
positions. Telegram `sendMessage` supports at most 4,096 text characters after
entity parsing, so the application should target approximately 3,500 characters
per part to leave room for headings, numbering, links, and formatting. Generate
and validate the entire bundle before sending its first part. Give both the
bundle and each part an idempotency key so a retry resumes safely without
duplicating already delivered text.

Automatically sending the complete bundle is the default. A user who dislikes
several consecutive notifications can select an alternate mode in which the
introduction arrives on schedule and a `continue` button releases the remaining
parts. The content is identical in both modes.

WhatsApp and email are outside the MVP. Removing them avoids per-message
WhatsApp costs and keeps the first release focused; it does not by itself make
model APIs, hosting, or storage free.

## Generation behaviour

- Dynamically generated lessons are required from the first usable release;
  manually seeded content alone is not an acceptable MVP.
- There is no human preview or approval queue before normal delivery.
- The application validates the output and makes one corrective regeneration
  request when required information or structure is missing.
- Information quality outranks formatting. A second structurally imperfect
  response can ship; an unverifiable response ships only with the warning
  defined in the trust requirements.
- Lessons are mainly theoretical: conceptual models, mechanisms, production
  cases, trade-offs, failure modes, and interview language. Coding and design
  exercises are optional actions rather than required homework.
- The product is English-only in the first release.
- The curriculum has no fixed duration or terminal topic count. It continues
  selecting new coherent themes while suppressing accidental repetition.

References:

- [Telegram bot introduction](https://core.telegram.org/bots)
- [Telegram Bot API](https://core.telegram.org/bots/api)
- [Log In With Telegram](https://core.telegram.org/bots/telegram-login)
