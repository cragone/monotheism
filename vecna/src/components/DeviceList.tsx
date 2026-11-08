import type { Device } from "../api/devices";
import DeviceCard from "./DeviceCard";

export default function DeviceList({
  devices,
  now,
  onChanged,
}: {
  devices: Device[];
  now: number;
  onChanged: () => void;
}) {
  if (devices.length === 0) {
    return (
      <div className="alert alert-info">
        <span>
          No devices yet — check that SHELLY_HOST points at the plug and
          it's reachable on the LAN. See VECNA.md for setup steps.
        </span>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      {devices.map((device) => (
        <DeviceCard key={device.id} device={device} now={now} onChanged={onChanged} />
      ))}
    </div>
  );
}
