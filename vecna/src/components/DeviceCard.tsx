import { useRef, useState } from "react";
import type { Device } from "../api/devices";
import { setBrightness, setPower } from "../api/devices";
import ConnectionBadge from "./ConnectionBadge";

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
        <div className="flex items-center justify-between">
          <h2 className="card-title text-base">{device.name || "Unnamed device"}</h2>
          <ConnectionBadge lastSeenAt={device.lastSeenAt} now={now} />
        </div>

        <label className="flex items-center justify-between gap-4">
          <span className="text-sm opacity-70">Power</span>
          <input
            type="checkbox"
            className="toggle toggle-primary"
            checked={isOn}
            disabled={pending}
            onChange={handleToggle}
          />
        </label>

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
