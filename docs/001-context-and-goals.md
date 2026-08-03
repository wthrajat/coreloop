# Context and goals

Last updated: 2026-08-03

## Why this document exists

This file preserves the personal and career context behind the product. Future
design and implementation decisions should be checked against it instead of
optimizing for a generic learning application.

## Career context

- The user is 24 years old and has worked in Bitcoin, cryptocurrency, and Web3
  engineering since approximately 2021.
- The user has roughly five years of professional software experience.
- The undergraduate degree is in mechanical engineering rather than computer
  science. Most software knowledge was learned through implementation and
  self-study.
- The user does not feel strongly attached to the broader cryptocurrency
  industry and wants more career options outside it.
- The desired transition is toward durable software-engineering work, including
  backend systems, cloud infrastructure, applied AI, and product engineering.
- The user should not be treated as a beginner developer or asked to restart as
  a graduate engineer. The missing piece is broader and more legible experience,
  not the absence of professional experience.

## Time and attention constraints

- Weekdays already contain a meaningful full-time workload.
- Saturdays and Sundays are reserved for personal life for the primary user,
  although every profile must be able to enable or disable weekend delivery.
- Large study blocks and conventional weekend courses are therefore unlikely
  to be sustainable.
- Short weekday windows of approximately 15 minutes are realistic. A user may
  alternatively select 30-minute textual lessons.
- Some of that time currently disappears into Instagram and short-form content.
  The product should make a valuable activity easier to start in that same kind
  of fragmented window.
- The system must not create guilt or become another demanding inbox.

## Learning goals

The learning programme should gradually build practical understanding in eight
areas:

1. Computer-science and systems fundamentals
2. Backend and distributed systems
3. Cloud, infrastructure, reliability, and security
4. Applied AI and production AI systems
5. Product engineering from discovery through production operation
6. Effective use of AI for specification, execution, evaluation, and judgment
7. Communication, product positioning, and practical sales skills
8. Important and current changes in the technology industry

The aim is not to read every technology announcement. It is to stop important
concepts and developments from remaining alien, then progressively deepen the
topics most relevant to the target career.

The primary user wants visible improvement every day. Coreloop is an
ongoing service rather than a fixed-duration course: it should keep selecting,
sequencing, and delivering useful material for as long as the profile remains
active.

## Learning motivation

Usefulness is the main driver of interest and retention. A lesson should answer
these questions before demanding attention for implementation detail:

1. Why is this worth learning?
2. What real problem does it solve?
3. Why did this technology or concept have to exist?
4. What solutions existed before it?
5. Why is the newer approach better in some situations?
6. Where is it used in real production systems?
7. When is it unnecessary or the wrong choice?
8. What is likely to happen to it in the future?

The lesson can then progress from motivation to mechanism, implementation,
trade-offs, and deeper technical detail.

## Content preferences

- Explanations should use plain, precise language.
- Technical analogies are welcome when they preserve the real mechanism.
- Childish analogies involving toys or oversimplified metaphors should be
  avoided.
- A substantial topic should not be reduced to a five-line definition.
- A topic that cannot be explained properly in 15 minutes should become a
  connected series, not an inaccurate summary.
- Lessons should remain on one coherent theme for several sessions or days. A
  morning lesson, noon lesson, and evening lesson should not jump among
  unrelated technologies.
- Content should include mechanics, production use, trade-offs, failure modes,
  and interview-ready language.
- Current-event claims should carry primary-source links and publication or
  update dates.
- The system should distinguish confirmed facts, interpretation, and
  speculation.

## Preferred experience

- The product should be a hosted, responsive web application with a clear
  frontend/backend boundary and a database. For the no-billing MVP, Next.js and
  Go API functions deploy together in one Vercel Hobby project.
- It will be shared with friends, so accounts, profiles, authentication, and
  strict per-user ownership are product requirements rather than later
  commercial features.
- Every user's topics, learning plan, lesson duration, explanation depth,
  frequency, delivery times, weekend preference, time zone, progress, and
  delivery channel must live in the database. These are not deployment
  environment variables.
- The default profile uses `Asia/Kolkata`, three 15-minute textual lessons per
  weekday, morning/noon/night delivery, and no weekend delivery. Initial default
  times are 09:00, 14:00, and 21:00 local time; every profile can change them.
- Telegram is the primary and only MVP delivery channel. A complete lesson is
  delivered as an ordered bundle of messages in the user's private bot chat;
  the learner should not have to open the web app to read it.
- The web app is a small, responsive control surface for onboarding, profiles,
  learning tracks, schedules, saved-state metadata, and progress. It is not the
  primary lesson renderer.
- WhatsApp and email delivery are outside the MVP. They can be reconsidered only
  after the Telegram learning workflow is proven and a real need justifies the
  additional cost or complexity.
- Messages should support actions such as `deeper`, `example`, `quiz`, `save`,
  `skip`, and `next`.
- Configuration should be broad but bounded. The interface should make common
  choices easy and keep invalid or self-defeating combinations unavailable.
- The application should integrate Groq, Gemini, and OpenAI through a common
  internal content contract with provider-specific adapters. Prompts and output
  schemas should be shared and versioned; credentials remain server secrets.
- Automatic routing uses Groq's free quota first and Gemini's free quota second.
  OpenAI is never invoked automatically; it requires an explicit manual enable
  action from the owner.
- The initial deployment must target zero recurring cash spend. Free quotas,
  cached or reusable lessons, and hard provider budgets are acceptable; silently
  falling back to a billable model call is not.

## Long-term outcome

The user should become a backend and distributed-systems engineer who can build,
evaluate, deploy, and operate AI-enabled products. Bitcoin experience remains a
useful specialization, but it should not be the only career identity.

The product succeeds if it continuously produces useful knowledge, stronger
engineering judgment, and demonstrable product-building experience without
requiring weekend study blocks or imposing an artificial graduation date.

Different users may have different outcomes. For example, the primary user may
select backend, cloud, applied AI, product engineering, communication, and
sales, while a friend may deliberately select a cryptocurrency learning track.

## Explicit non-goals

- Maximizing notifications or time spent inside the application
- Building an infinite news feed
- Replacing primary technical sources with unverified AI summaries
- Chasing every new framework, model, or social-media trend
- Treating streaks as evidence of understanding
- Building organization administration, payments, public social features, or a
  large-scale commercial platform before the small multi-user workflow is proven
- Exposing every internal implementation option as a user-facing setting
