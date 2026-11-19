import type { HistoryEntry } from "./api/speak";

export default function History({ entries }: { entries: HistoryEntry[] }) {
  if (entries.length === 0) {
    return <p className="text-sm opacity-60">Nothing spoken yet.</p>;
  }

  return (
    <ul className="flex flex-col gap-2">
      {entries.map((entry, i) => (
        <li key={i} className="rounded-box bg-base-200 px-3 py-2">
          <p>{entry.text}</p>
          <p className="text-xs opacity-50">{new Date(entry.at).toLocaleTimeString()}</p>
        </li>
      ))}
    </ul>
  );
}
