# Coreloop

<p align="center">
  <img
    src="assets/images/telegram-loop.png"
    alt="Coreloop delivering source-backed news in Telegram"
    width="380"
  />
</p>

<p align="center">
  <sub>A learning and news loop delivered where you already pay attention.</sub>
</p>

Coreloop is a self-hosted, Telegram-first system for turning subjects you care
about into a steady learning loop. Choose what to learn, when lessons should
arrive, and which sources to follow. Coreloop plans connected lessons, ranks
fresh updates, and delivers both to Telegram while the web app remains a quiet
control surface.

The included starter configuration focuses on software engineering, AI, cloud,
security, product, and Bitcoin. Its topic and source catalogues are data-driven,
so a deployment can adapt the same loop to a different field without replacing
the delivery system.

## What it does

- **Builds a learning loop.** Schedule detailed 15- or 30-minute lessons,
  continue a subject across multiple deliveries, and use simple Read or Skip
  feedback.
- **Runs a source-backed news loop.** Follow RSS and Atom feeds, Hacker News,
  GitHub Releases, public listing pages, sitemaps, and selected social feeds.
- **Keeps delivery useful.** Rank recent items, balance source families, reject
  stale or placeholder links, and send each update with its original source.
- **Works when AI does not.** Deterministic feed ingestion and news delivery
  continue when model quotas are exhausted. AI explanations are optional.
- **Survives restarts.** Lessons, jobs, retries, provider state, and Telegram
  deliveries live in a durable Turso queue instead of process memory.
- **Stays private by default.** Access is invite-only, authentication uses
  Telegram, and secrets remain on the server.

## Included loop

Coreloop ships with a curated technical starter set rather than an empty
dashboard:

- learning topics across backend systems, cloud, applied AI, security,
  reliability, product, communication, sales, and Bitcoin;
- a broad technology-news catalogue covering official engineering blogs, Hacker
  News, Stacker News, research, releases, and community feeds;
- configurable lesson depth, delivery times, weekends, recall, and daily news
  cadence;
- Groq-first lesson generation with Gemini fallback, plus deterministic news
  delivery when no AI provider is available.

## How it fits together

| Part             | Role                                                             |
| ---------------- | ---------------------------------------------------------------- |
| Next.js          | Responsive setup, progress, privacy, and owner operations UI     |
| Go               | Authentication, planning, ingestion, ranking, jobs, and delivery |
| Telegram Bot API | Primary lesson and news destination                              |
| Turso/libSQL     | Durable profiles, content, schedules, jobs, and delivery state   |
| QStash           | Scheduled wake-ups for the durable queue                         |
| Groq and Gemini  | Free-tier automatic lesson generation                            |

OpenAI is never an automatic fallback. An owner can use it only for an explicit
one-off retry of a blocked job.

## Run it yourself

See the [local development guide](docs/local-development.md) for prerequisites,
environment variables, migrations, and verification commands. The repository is
designed to run on free service tiers, but each provider controls its own limits
and availability.

Before deploying, copy `.env.example` to an untracked environment file and keep
all credentials in your hosting provider's secret store. Never expose a secret
through a `NEXT_PUBLIC_*` variable.
