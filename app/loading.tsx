export default function ApplicationLoading() {
  return (
    <main className="centered-state" role="status">
      <span aria-hidden="true" className="loading-line" />
      <p>Opening Coreloop…</p>
    </main>
  );
}
