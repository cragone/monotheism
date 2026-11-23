import { useEffect, useState } from "react";
import { subscribeHearEvents, type HeardEvent } from "./api/hearEvents";
import Waveform from "./Waveform";

export default function ListenPanel() {
  const [latest, setLatest] = useState<HeardEvent | null>(null);

  useEffect(() => {
    return subscribeHearEvents(setLatest);
  }, []);

  return (
    <div className="flex flex-col gap-3">
      <h2 className="text-lg font-semibold">Listening</h2>
      {latest ? (
        <>
          <Waveform points={latest.waveform} />
          <p className="text-sm">
            {latest.transcript || <span className="opacity-60">(no speech detected)</span>}
          </p>
        </>
      ) : (
        <div className="opacity-60 text-sm">Waiting for the mic…</div>
      )}
    </div>
  );
}
