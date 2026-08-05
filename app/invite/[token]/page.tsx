import type { Metadata } from "next";
import { notFound } from "next/navigation";

export const metadata: Metadata = {
  title: "Accept invitation",
  robots: { index: false, follow: false, noarchive: true, nocache: true },
};

export default async function InvitePage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  if (!token || token.length < 20) notFound();
  const start = `/api/app/auth/start?invite=${encodeURIComponent(token)}&return=/onboarding`;
  return (
    <main className="centered-state">
      <div className="brand-mark">
        <span />
      </div>
      <span className="eyebrow">Private invitation</span>
      <h1>Create your Coreloop profile.</h1>
      <p>
        Telegram will confirm your identity and connect private lesson delivery.
        The invitation is single-use and expires; Coreloop never asks for your
        phone number.
      </p>
      <a className="button button-primary" href={start}>
        Continue with Telegram
      </a>
      <p className="fine-print">
        By continuing, you allow this Coreloop bot to message your Telegram
        account.
      </p>
    </main>
  );
}
