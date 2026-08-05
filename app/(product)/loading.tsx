export default function ProductLoading() {
  return (
    <div className="page-stack" role="status">
      <span aria-hidden="true" className="loading-line" />
      <p className="muted-copy">Opening your private workspace…</p>
    </div>
  );
}
