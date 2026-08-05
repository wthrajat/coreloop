import Link from "next/link";

export default function NotFoundPage() {
  return (
    <main className="centered-state">
      <span className="eyebrow">Not found</span>
      <h1>This Coreloop page does not exist.</h1>
      <p>
        The address may be incomplete. If this was a private invitation, it may
        have expired or already been used.
      </p>
      <Link className="button button-primary" href="/">
        Return to Coreloop
      </Link>
    </main>
  );
}
