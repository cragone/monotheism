import { Link } from "react-router-dom";
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
          No devices yet — add one from the{" "}
          <Link to="/settings" className="link">
            device settings page
          </Link>
          , or set SHELLY_HOST for a startup default. See VECNA.md for setup
          steps.
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
