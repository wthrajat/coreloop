import Link from "next/link";

export const metadata = { title: "Private access" };

export default function AccessRequiredPage() {
  return (
    <main className="centered-state">
      <span className="eyebrow">Invite-only</span>
      <h1>Your learning profile starts from a private link.</h1>
      <p>
        A friend who already owns this Coreloop instance can create a single-use
        link. Telegram confirms your identity and grants the bot permission to
        deliver private messages; no email account or phone-number form is used.
      </p>
      <a
        className="button button-primary"
        href="/api/app/auth/start?return=/overview"
      >
        Returning member sign in
      </a>
      <Link className="text-link" href="/">
        Back home
      </Link>
    </main>
  );
}
