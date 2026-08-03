# Decisions and next questions

Last updated: 2026-08-03

## Active decisions

1. Build one ongoing Coreloop combining a coherent curriculum with a
   separately ranked current-tech radar.
2. Deliver complete, detailed textual lessons through Telegram message bundles.
3. Keep the responsive web UI small: invite/login, onboarding, profile,
   schedule, topics, overview, progress, settings, export, and deletion.
4. Support close friends through single-use invites and private per-user data.
5. Use Telegram OIDC with PKCE as the only authentication and bot-access flow;
   do not implement email or password signup.
6. Default to three 15-minute weekday lessons at 09:00, 14:00, and 21:00 in
   `Asia/Kolkata`, weekends off, with all settings configurable per profile.
7. Let the system choose and persist coherent themes and topic history. Continue
   indefinitely with no ten-week endpoint or curriculum cap.
8. Count completion only after the learner presses `read`. Unread lessons remain
   queued but never block later delivery.
9. Keep recall inside curriculum bundles and current-tech radar outside the
   curriculum slots.
10. Apply no numerical radar cap. Use a transparent relevance threshold and
    `skip` feedback as the spam control.
11. Make curriculum and radar lessons simple, technical, detailed, mostly
    theoretical, and English-only.
12. Dynamically generate content from the first usable version without a human
    approval queue.
13. Retry invalid model output once. Send a second structurally imperfect result
    when informative; label and separate any still-unverified information.
14. Personalize text only by topic, current level, duration, and depth so
    compatible users can reuse an immutable verified lesson.
15. Use Groq free quota first and Gemini free quota second. The earlier xAI/Grok
    references were incorrect.
16. Never route scheduled work automatically to OpenAI. Keep OpenAI as an
    explicit owner-only manual path.
17. Share the owner's server-side provider quotas first-come, first-served.
18. Persist quota-blocked work and deliver all of it chronologically when free
    quota returns.
19. Expose no administrator product view of friends' learning content or
    answers; keep operational alerts content-free.
20. Deploy one Vercel Hobby project containing Next.js and Go Functions, with
    Turso and QStash free plans and no payment method.
21. Treat exact delivery time as best-effort. Ten-minute scheduling precision is
    sufficient; idempotency and recovery matter more.
22. Keep prompts lean and lossless: compact typed context, stable versioned
    instruction prefixes, selected evidence only, hard token budgets, and no
    full chat/profile/history replay.
23. Cache source fetches, normalized evidence, rankings, verified compatible
    lessons, compiled prompt inputs, and Telegram parts with versioned hashes.
24. Use deterministic code instead of AI for scheduling, filtering, ranking
    arithmetic, deduplication, state, validation, and chunking.

## Remaining non-blocking choices

The implementation plan can select these from evidence without another product
interview:

- The initial owner-curated source allowlist
- The exact Groq and Gemini model IDs available on the owner's free accounts
- The initial radar score weights and threshold
- The exact retention periods for operational logs and inactive sessions
- Whether an email alert adapter adds enough value beyond owner Telegram alerts

None changes the accepted product shape. Model IDs, free quotas, and external
limits must be verified again immediately before implementation because they can
change.

## Current implementation document

Use `011-implementation-plan.md` for the selected stack, boundaries, data model,
job flow, prompt and cache design, milestones, and verification requirements.

## Context-maintenance rule

Add future decisions as new numbered Markdown documents. Do not erase historical
reasoning. Mark older decisions as superseded and keep the current implementation
document consistent with the newest accepted decision record.
