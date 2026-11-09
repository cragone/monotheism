import { useRef, useState } from "react";
import type { Device } from "../api/devices";
import { renameDevice, setBrightness, setPower } from "../api/devices";
import ConnectionBadge from "./ConnectionBadge";
import LightbulbIcon from "./LightbulbIcon";

const BRIGHTNESS_DEBOUNCE_MS = 150;

export default function DeviceCard({
  device,
  now,
  onChanged,
}: {
  device: Device;
  now: number;
  onChanged: () => void;
}) {
  const [pending, setPending] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [editingName, setEditingName] = useState(false);
  const [nameDraft, setNameDraft] = useState(device.name);

  function startEditingName() {
    setNameDraft(device.name);
    setEditingName(true);
  }

  async function commitName() {
    const trimmed = nameDraft.trim();
    setEditingName(false);
    if (!trimmed || trimmed === device.name) {
      setNameDraft(device.name);
      return;
    }
    try {
      await renameDevice(device.id, trimmed);
      onChanged();
    } catch (err) {
      console.error(err);
      setNameDraft(device.name);
    }
  }

  // Local override while the user is actively dragging the slider, so the
  // ~2s poll doesn't yank the thumb back before the debounced command (and
  // the resulting state message) lands. Reset once that round-trip settles.
  const [localBrightness, setLocalBrightness] = useState<number | null>(null);
  const brightness = localBrightness ?? device.lastBrightness;

  const isOn = device.lastPowerState === "ON";

  async function handleToggle() {
    setPending(true);
    try {
      await setPower(device.id, !isOn);
      onChanged();
    } catch (err) {
      console.error(err);
    } finally {
      setPending(false);
    }
  }

  function handleBrightnessChange(level: number) {
    setLocalBrightness(level);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(async () => {
      try {
        await setBrightness(device.id, level);
        onChanged();
      } catch (err) {
        console.error(err);
      } finally {
        setLocalBrightness(null);
      }
    }, BRIGHTNESS_DEBOUNCE_MS);
  }

  return (
    <div className="card bg-base-200 shadow-sm">
      <div className="card-body gap-3">
        <div className="flex items-center justify-between gap-2">
          {editingName ? (
            <input
              autoFocus
              className="input input-sm input-bordered flex-1 min-w-0"
              value={nameDraft}
              onChange={(e) => setNameDraft(e.target.value)}
              onBlur={commitName}
              onKeyDown={(e) => {
                if (e.key === "Enter") commitName();
                if (e.key === "Escape") {
                  setNameDraft(device.name);
                  setEditingName(false);
                }
              }}
            />
          ) : (
            <h2
              className="card-title text-base truncate cursor-text"
              title="Click to rename"
              onClick={startEditingName}
            >
              {device.name || "Unnamed device"}
            </h2>
          )}
          <ConnectionBadge lastSeenAt={device.lastSeenAt} now={now} />
        </div>

        <div className="flex items-center justify-between gap-4">
          <button
            type="button"
            onClick={handleToggle}
            disabled={pending}
            aria-pressed={isOn}
            aria-label={isOn ? "Turn off" : "Turn on"}
            className={`btn btn-circle btn-ghost ${
              isOn ? "text-warning drop-shadow-[0_0_6px_var(--color-warning)]" : "text-base-content/40"
            }`}
          >
            <LightbulbIcon on={isOn} className="size-7" />
          </button>
          <span className="text-sm opacity-70">{isOn ? "On" : "Off"}</span>
        </div>

        {device.supportsBrightness && (
          <label className="flex flex-col gap-1">
            <span className="text-sm opacity-70">Brightness</span>
            <input
              type="range"
              min={0}
              max={255}
              value={brightness}
              disabled={!isOn}
              className="range range-primary range-sm"
              onChange={(e) => handleBrightnessChange(Number(e.target.value))}
            />
          </label>
        )}
      </div>
    </div>
  );
}
