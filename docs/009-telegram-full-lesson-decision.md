# Telegram full-lesson decision

Last updated: 2026-08-03

## Status

Partially superseded by `010-grilling-decisions.md`. Telegram full-lesson
delivery and the message-bundle contract remain accepted. Telegram OIDC replaces
the separate connection-token flow, Groq replaces xAI, and the curriculum no
longer has a fixed acceleration period.

## Decision

Telegram is the primary and only delivery channel for the first version. Every
scheduled lesson is read completely inside the user's private bot chat as an
ordered set of textual messages. The learner does not need to open the web app
to continue reading.

The web application remains, but only as a simple responsive control surface
for:

- Account onboarding and private profiles
- Learning tracks, level, and goals
- Lesson duration, depth, daily count, days, times, and quiet periods
- Telegram connection and bundle-delivery mode
- Active theme, upcoming schedule, progress, and recall state
- Account export and deletion

The MVP does not need a full web lesson reader, elaborate dashboard, WhatsApp,
or email delivery.

## What this supersedes

This decision supersedes only the channel and lesson-rendering parts of
`008-multi-user-product-decision.md`:

- WhatsApp is no longer the preferred first channel.
- Telegram is no longer a fallback.
- A WhatsApp link to a web-rendered lesson is no longer the primary flow.
- Email is not an MVP archive or fallback.

The multi-user product, profile configuration, coherent theme sequencing, and
Turso/libSQL decisions remain. Provider routing is now Groq, Gemini, and
manual-only OpenAI as defined in `010-grilling-decisions.md`.

## Accepted defaults

- Time zone: `Asia/Kolkata`
- Lesson duration: 15 minutes per lesson
- Lessons per day: three
- Delivery times: 09:00, 14:00, and 21:00 local time
- Weekend delivery: off
- Explanation depth: standard
- Delivery mode: automatically send the complete message bundle
- Theme continuity: all daily lessons continue one active theme

These are profile defaults, not global restrictions. Each user can select 15 or
30 minutes, change the lesson count and times, enable weekends, choose depth and
topics, pause delivery, or require a `continue` action after the introduction.

## Message-bundle contract

A lesson is one logical bundle containing multiple immutable parts:

1. The first part identifies the theme and lesson, shows `Part 1/N`, states the
   total expected reading time, and begins with usefulness and motivation.
2. Middle parts use `Part i/N` and keep related sections together. A section is
   not cut merely to fill the maximum message size.
3. The last part closes the lesson with the interview-ready explanation, recall
   or application question, sources, and inline actions.

Telegram permits up to 4,096 text characters in `sendMessage` after entity
parsing. The renderer therefore targets approximately 3,500 characters per part
to leave room for numbering, links, and formatting. This is a technical target,
not a content-length target: a 15-minute or 30-minute lesson uses as many parts
as its validated reading time and structure require.

The content pipeline must generate, verify, render, and persist the complete
bundle before the scheduler sends `Part 1/N`. Each bundle and part receives a
stable idempotency key. Delivery is serial and records every Telegram message ID,
allowing a retry to resume at the first unconfirmed part instead of repeating
the lesson.

## Telegram account connection

The bot cannot initiate a conversation with a person who has never started it.
The web app therefore creates a short-lived, single-use token and opens a link
equivalent to `t.me/<bot>?start=<token>`. The backend consumes that token,
associates the Telegram chat ID with the authenticated profile, and reports the
connection in both Telegram and the web settings page.

A user can disconnect, pause, or replace the chat destination. Telegram chat IDs
and connection tokens are sensitive application data and must not appear in
client logs or analytics payloads.

## Zero-spend constraint

Telegram removes the WhatsApp per-message cost, but Telegram alone does not make
hosting, storage, or AI generation free. The first deployment targets zero
recurring cash spend through free quotas and explicit guardrails:

- Use only services whose available free quota is verified during implementation.
- Configure a zero-spend production mode that never invokes a billable fallback.
- Pre-generate lessons instead of making a model call at delivery time.
- Cache reviewed lessons by topic, objectives, duration, depth, source version,
  and prompt version so compatible users can reuse them.
- Keep personalization, assignments, progress, and recall per-user even when the
  underlying lesson is reused.
- Pause generation or use cached and manually reviewed material when a free
  quota is exhausted.
- Require an explicit owner action and budget before any paid provider is
  enabled for evaluation.

Groq, Gemini, and OpenAI adapters can all exist without all three being active in
automatic production. Provider support is an engineering capability; it is not
permission to incur cost.

## Why this is simpler

- The scheduled notification and reading surface are the same place.
- There is no authenticated deep-link handoff in the normal learning flow.
- Telegram supplies private chats, buttons, bot webhooks, and delivery message
  IDs through one interface.
- The web app can focus on configuration and progress rather than duplicating a
  rich long-form reader.
- Removing WhatsApp avoids template approval, policy, and per-message billing
  work before the learning behavior is validated.

## Main trade-offs

- One lesson can create several notifications. The optional
  introduction-then-`continue` mode limits that interruption without moving the
  content to the web.
- Telegram formatting is less capable than a custom web reader.
- A partially delivered bundle requires part-level persistence and recovery.
- Searching or revisiting old lessons is initially constrained by Telegram chat
  history; the web app stores progress and saved-topic metadata, not a duplicate
  reader.
- Editing an already delivered lesson can create inconsistency, so published
  lesson versions are immutable and corrections are delivered explicitly.

## References

- [Telegram bot introduction](https://core.telegram.org/bots)
- [Telegram Bot API](https://core.telegram.org/bots/api)
