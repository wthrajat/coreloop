# Coreloop

Coreloop is a private, invite-only learning system for working software
engineers. It builds a connected curriculum, generates clear and detailed
lessons, and delivers everything through Telegram. The web app is only for
configuration, progress, and operations—not another feed to keep open.

## Lessons

- Choose 15- or 30-minute lessons, delivery times, weekends, topics, level, and
  depth.
- Continue a theme across lessons instead of jumping between unrelated topics.
- Learn why something exists, how it works, where it fails, and when to choose
  an alternative.
- Receive the complete lesson in Telegram and use lightweight **Read** and
  **Skip** feedback.

## Current-tech Radar

- Ranks recent developer news from official engineering feeds, Hacker News,
  trusted technical blogs, and Bitcoin/Lightning communities.
- Sends one news item per Telegram message with its real source.
- Prioritizes useful releases, incidents, security developments, research, and
  engineering ideas over routine marketing.
- Keeps sending from RSS and public feeds when AI providers are unavailable.
- Lets every learner choose their own delivery frequency; the owner can trigger
  an immediate Radar delivery from Operations.

## Screenshots

### Learning overview

![Learning overview and delivery state](assets/images/dashboard.png)

### Personal setup

![Lesson and delivery configuration](assets/images/onboarding.png)

## Run locally

See the [local development guide](docs/local-development.md).

Coreloop is designed to self-host on free service tiers using Next.js, Go,
Turso, QStash, Telegram, Groq, and Gemini. OpenAI is never an automatic
fallback; only the owner can explicitly use it for one blocked job.
