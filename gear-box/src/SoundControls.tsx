import { useEffect, useState } from "react";
import {
  getSoundStatus,
  listModes,
  setSoundMode,
  setSoundPower,
  setSoundVolume,
  type SoundStatus,
} from "./api/sound";

const MODE_LABELS: Record<string, string> = {
  white: "White noise",
  pink: "Pink noise",
  brown: "Brown noise",
  rain: "Rain",
  ocean: "Ocean",
  fan: "Fan",
  tone: "Tone",
};

export default function SoundControls() {
  const [status, setStatus] = useState<SoundStatus | null>(null);
  const [modes, setModes] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    Promise.all([getSoundStatus(), listModes()])
      .then(([s, m]) => {
        setStatus(s);
        setModes(m);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "failed to reach sound machine"));
  }, []);

  async function run(action: () => Promise<SoundStatus>) {
    setBusy(true);
    setError(null);
    try {
      setStatus(await action());
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to reach sound machine");
    } finally {
      setBusy(false);
    }
  }

  if (!status) {
    return error ? (
      <div className="alert alert-error text-sm">Sound machine unreachable: {error}</div>
    ) : (
      <p className="text-sm opacity-60">Loading sound machine…</p>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-medium">Ambient sound</h2>
        <input
          type="checkbox"
          className="toggle toggle-primary"
          checked={status.on}
          disabled={busy}
          onChange={(e) => run(() => setSoundPower(e.target.checked))}
        />
      </div>

      <div className="flex flex-wrap gap-2">
        {modes.map((m) => (
          <button
            key={m}
            type="button"
            className={`btn btn-sm ${status.mode === m ? "btn-primary" : "btn-outline"}`}
            disabled={busy}
            onClick={() => run(() => setSoundMode(m))}
          >
            {MODE_LABELS[m] ?? m}
          </button>
        ))}
      </div>

      <label className="flex flex-col gap-1">
        <span className="text-sm opacity-70">Volume</span>
        <input
          type="range"
          min={0}
          max={1}
          step={0.05}
          className="range range-primary"
          value={status.volume}
          disabled={busy}
          onChange={(e) => setStatus({ ...status, volume: Number(e.target.value) })}
          onMouseUp={(e) => run(() => setSoundVolume(Number((e.target as HTMLInputElement).value)))}
          onTouchEnd={(e) => run(() => setSoundVolume(Number((e.target as HTMLInputElement).value)))}
          onKeyUp={(e) => run(() => setSoundVolume(Number((e.target as HTMLInputElement).value)))}
        />
      </label>

      {error && <div className="alert alert-error text-sm">{error}</div>}
    </div>
  );
}
