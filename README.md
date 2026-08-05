# Coreloop

Coreloop is a private, Telegram-first learning system for working software
engineers. Choose what you want to learn and when you have time. Coreloop builds
a connected curriculum, generates detailed lessons, and delivers the complete
material to Telegram.

The web app stays focused on configuration and progress. It is not another feed
to keep open.

## What Coreloop does

- Sends clear, detailed 15- or 30-minute engineering lessons on your schedule.
- Continues a topic across lessons instead of jumping between random subjects.
- Explains why a concept exists, how it works, where it fails, and when to use
  an alternative.
- Delivers ranked technology news from official engineering sources, Hacker
  News, trusted technical blogs, and Bitcoin/Lightning communities.
- Keeps Radar news flowing from RSS and public feeds even when AI is
  unavailable.
- Sends one sourced news item per Telegram message and filters weak, stale, or
  repetitive updates.
- Lets each learner configure topics, depth, delivery times, weekends, spaced
  recall, and Radar frequency independently.
- Uses private invitations and Telegram sign-in—there is no public signup or
  password database.

## How it feels to use

1. Configure your learning goals and delivery rhythm in the web app.
2. Receive complete lessons and current-tech Radar updates in Telegram.
3. Tap **Read** or **Skip** to shape future delivery without blocking the queue.

The owner also has an Operations page for queue health, private invitations, and
immediate Telegram acceptance tests for lessons and Radar.

## Screenshots

### Learning overview

![Learning overview and delivery state](assets/images/dashboard.png)

## Documentation

- [Local development](docs/local-development.md)
- [Production deployment](docs/deployment.md)
- [Operations runbook](docs/runbook.md)
- [Security policy](docs/SECURITY.md)
- [External API contracts](docs/external-contracts.md)
- [Contributing](docs/contributing.md)

Coreloop is designed for self-hosting on free service tiers. Scheduled work uses
Groq first and Gemini second. OpenAI is never an automatic fallback and can only
be used through an explicit owner action.
