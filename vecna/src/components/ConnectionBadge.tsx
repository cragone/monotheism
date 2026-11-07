const STALE_AFTER_MS = 30_000;

export default function ConnectionBadge({
  lastSeenAt,
  now,
}: {
  lastSeenAt: string | null;
  now: number;
}) {
  if (!lastSeenAt) {
    return <span className="badge badge-ghost badge-sm">never seen</span>;
  }

  const ageMs = now - new Date(lastSeenAt).getTime();
  const isStale = ageMs > STALE_AFTER_MS;

  return (
    <span className={`badge badge-sm ${isStale ? "badge-ghost" : "badge-success"}`}>
      {isStale ? "stale" : "online"}
    </span>
  );
}
