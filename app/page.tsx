import Link from "next/link";

export default function HomePage() {
  return (
    <main className="landing-page">
      <nav className="landing-nav">
        <span className="brand-lockup">
          <span aria-hidden="true" className="brand-mark">
            <span />
          </span>
          <span>
            <strong>Coreloop</strong>
            <small>Private learning system</small>
          </span>
        </span>
        <a
          className="button button-secondary"
          href="/api/app/auth/start?return=/overview"
        >
          Member sign in
        </a>
      </nav>
      <section className="landing-hero">
        <span className="eyebrow">Telegram-first technical curriculum</span>
        <h1>
          Useful engineering knowledge, delivered at the pace of a working life.
        </h1>
        <p>
          Detailed lessons build coherent themes. Ranked official updates arrive
          separately when they matter. The web app stays a quiet control
          surface—not another feed.
        </p>
        <div className="button-row">
          <a
            className="button button-primary"
            href="/api/app/auth/start?return=/overview"
          >
            Reconnect with Telegram
          </a>
          <Link className="text-link" href="/access-required">
            How access works
          </Link>
        </div>
      </section>
      <section className="landing-principles" aria-label="Product principles">
        <article>
          <span>01</span>
          <h2>Usefulness first</h2>
          <p>
            Every lesson begins with why a concept exists, what came before it,
            and where it earns its complexity.
          </p>
        </article>
        <article>
          <span>02</span>
          <h2>Depth in small windows</h2>
          <p>
            Fifteen and thirty-minute presets remain technically detailed,
            readable, and interview-ready.
          </p>
        </article>
        <article>
          <span>03</span>
          <h2>Current, not noisy</h2>
          <p>
            Official engineering announcements are ranked against your topics
            and kept outside lesson windows.
          </p>
        </article>
      </section>
    </main>
  );
}
