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
            <small>Learning and news loop</small>
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
        <span className="eyebrow">Telegram-first personal learning</span>
        <h1>Turn what you want to learn into a loop that keeps moving.</h1>
        <p>
          Choose topics, lesson depth, delivery times, and trusted news sources.
          Coreloop plans connected lessons and ranks fresh updates, then sends
          both to Telegram while the web app stays a quiet control surface.
        </p>
        <div className="button-row">
          <a
            className="button button-primary"
            href="/api/app/auth/start?return=/overview"
          >
            Member sign in
          </a>
          <Link className="text-link" href="/access-required">
            How access works
          </Link>
        </div>
      </section>
      <section className="landing-principles" aria-label="Product principles">
        <article>
          <span>01</span>
          <h2>Learn in a sequence</h2>
          <p>
            Lessons continue a subject across multiple deliveries instead of
            dropping disconnected explanations into your day.
          </p>
        </article>
        <article>
          <span>02</span>
          <h2>Fit your actual day</h2>
          <p>
            Choose detailed fifteen or thirty-minute lessons, exact delivery
            times, weekends, and lightweight recall.
          </p>
        </article>
        <article>
          <span>03</span>
          <h2>Stay current without a feed</h2>
          <p>
            Source-backed updates are ranked, balanced across trusted sources,
            and delivered separately from lessons.
          </p>
        </article>
      </section>
    </main>
  );
}
