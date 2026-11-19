import { useCallback, useEffect, useState } from "react";
import SpeakForm from "./SpeakForm";
import History from "./History";
import { listHistory, type HistoryEntry } from "./api/speak";

export default function App() {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);

  const refresh = useCallback(() => {
    listHistory()
      .then(setEntries)
      .catch(() => {
        /* best-effort; the speak form surfaces its own errors */
      });
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="min-h-screen bg-base-100 flex justify-center p-6">
      <div className="w-full max-w-md flex flex-col gap-6">
        <h1 className="text-2xl font-semibold">gear-box</h1>
        <SpeakForm onSpoken={refresh} />
        <div>
          <h2 className="text-lg font-medium mb-2">Recently spoken</h2>
          <History entries={entries} />
        </div>
      </div>
    </div>
  );
}
