# Multi-user product decision

Last updated: 2026-08-03

## Status

Partially superseded by `009-telegram-full-lesson-decision.md` and
`010-grilling-decisions.md`. The multi-user and per-profile decisions remain
accepted. The WhatsApp-first channel, web-rendered lesson, xAI provider, and
separately hosted backend decisions no longer apply.

## What this decision supersedes

This document supersedes the earlier assumptions that Coreloop would be a
single-user private bot, that Telegram would necessarily be the first user-facing
channel, that the default cadence would be two weekday messages, and that
PostgreSQL was the selected database.

## Product shape

Coreloop is a hosted, multi-user learning application with:

- A responsive web frontend
- A separately deployed modular backend API
- Authentication and private profiles
- A Turso/libSQL database candidate
- Per-user learning tracks, theme plans, schedules, progress, and destinations
- WhatsApp-first 1:1 link notifications when officially feasible
- Telegram and email channel adapters as fallbacks
- OpenAI, Gemini, and xAI provider adapters behind one content contract

## Tentative defaults

- Time zone: `Asia/Kolkata`
- Textual lesson duration: 15 minutes per lesson, pending confirmation
- Curriculum lessons: three per day
- Windows: morning, noon, and night, with exact times pending confirmation
- Weekend delivery: off
- Explanation depth: standard
- Theme continuity: required across the day's lessons and subsequent days
- Current-tech radar: separate from curriculum, exact quota pending confirmation

Defaults initialize a profile; they are not deployment-wide restrictions. Every
person can change the supported settings without affecting another user.

## Why groups and broadcasts are not the core

A WhatsApp group or broadcast cannot express individual topic selection,
frequency, timing, progress, pause state, or privacy cleanly. The official
Business Platform is better treated as an opted-in 1:1 notification system. A
short approved template opens the authenticated web lesson where dynamic content
and interactions belong.

Current WhatsApp requirements include explicit opt-in, approved templates for
business-initiated messages outside the 24-hour window, opt-out handling, quality
enforcement, and per-delivered-message pricing. These constraints require a
feasibility spike before WhatsApp is promised as the only channel.

Cryptocurrency education also needs careful content policy review because
WhatsApp restricts promotion or facilitation of virtual-currency commerce.
Neutral technical education should not be described as investment advice or a
way to buy, sell, or promote currency.

References:

- [WhatsApp Business policy](https://whatsappbusiness.com/policy/)
- [WhatsApp Business Platform features](https://whatsappbusiness.com/products/business-platform-features/)
- [WhatsApp Business Platform pricing](https://whatsappbusiness.com/products/platform-pricing/)

## Configuration principle

Make learning choices configurable, not every implementation detail. Users
should control goals, topics, difficulty, duration, depth, frequency, schedule,
weekends, radar, recall, channels, and pauses. Administrators control provider
credentials, model routing, source allowlists, system prompts, hard safety
constraints, and operational limits.

This separation keeps the product flexible without turning onboarding into a
control panel for internals.

## Multi-provider principle

The application owns one versioned lesson specification and JSON schema. Each
provider adapter translates it to the provider's API and normalizes the result.
All adapters pass the same conformance and evaluation suite.

Do not assume that identical prompts produce equivalent lessons. Compare the
providers on a representative set of themes and measure:

- Technical correctness
- Usefulness and motivation
- Depth and reading-time fit
- Source and citation support
- Theme continuity
- Interview usefulness
- Latency, token usage, and cost

Normal production generation uses a selected primary provider and a fallback.
Three-provider generation is an evaluation mode unless future evidence justifies
the extra cost.

## Product-engineering principle

Configuration requests are hypotheses about what helps learning. The product
should implement them faithfully and then measure outcomes. For example, three
daily lessons remain the tentative default, but recall and completion data should
show whether three improves learning or merely increases unopened messages.

This is product engineering: translate a request into the underlying need,
deliver the smallest complete workflow, measure the intended outcome, observe
failure modes, and revise the product rather than defending the first design.
