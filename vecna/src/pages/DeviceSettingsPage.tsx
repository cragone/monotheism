import { useEffect, useState, type FormEvent } from "react";
import {
  createDevice,
  deleteDevice,
  listDevices,
  updateDeviceType,
  type Device,
} from "../api/devices";
import { DEVICE_TYPES, emojiFor, type DeviceType } from "../deviceTypes";

const EMPTY_FORM = { host: "", name: "", deviceType: "light" as DeviceType };

export default function DeviceSettingsPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);

  async function load() {
    try {
      const next = await listDevices();
      setDevices(next);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to load devices");
    }
  }

  useEffect(() => {
    (async () => {
      await load();
    })();
  }, []);

  async function handleCreate(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await createDevice(form);
      setForm(EMPTY_FORM);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to add device");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleTypeChange(id: number, deviceType: DeviceType) {
    try {
      await updateDeviceType(id, deviceType);
      await load();
    } catch (err) {
      console.error(err);
      setError(err instanceof Error ? err.message : "failed to update device type");
    }
  }

  async function handleDelete(id: number) {
    try {
      await deleteDevice(id);
      await load();
    } catch (err) {
      console.error(err);
      setError(err instanceof Error ? err.message : "failed to delete device");
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Manage Devices</h1>

      {error && (
        <div className="alert alert-error">
          <span>{error}</span>
        </div>
      )}

      <form onSubmit={handleCreate} className="card bg-base-200 shadow-sm">
        <div className="card-body gap-3">
          <h2 className="card-title text-base">Add a device</h2>
          <input
            className="input input-sm input-bordered"
            placeholder="Host (e.g. 192.168.1.210)"
            value={form.host}
            onChange={(e) => setForm({ ...form, host: e.target.value })}
            required
          />
          <input
            className="input input-sm input-bordered"
            placeholder="Name"
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            required
          />
          <select
            className="select select-sm select-bordered"
            value={form.deviceType}
            onChange={(e) => setForm({ ...form, deviceType: e.target.value as DeviceType })}
          >
            {DEVICE_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.emoji} {t.label}
              </option>
            ))}
          </select>
          <button className="btn btn-primary btn-sm" disabled={submitting} type="submit">
            Add Device
          </button>
        </div>
      </form>

      <ul className="space-y-2">
        {devices.map((d) => (
          <li key={d.id} className="card bg-base-200 shadow-sm">
            <div className="card-body flex-row items-center justify-between gap-3 flex-wrap">
              <div className="min-w-0">
                <div className="font-medium truncate">
                  {emojiFor(d.deviceType)} {d.name}
                </div>
                <div className="text-sm opacity-60">{d.host}</div>
              </div>
              <div className="flex items-center gap-2">
                <select
                  className="select select-sm select-bordered"
                  value={d.deviceType}
                  onChange={(e) => handleTypeChange(d.id, e.target.value as DeviceType)}
                >
                  {DEVICE_TYPES.map((t) => (
                    <option key={t.value} value={t.value}>
                      {t.emoji} {t.label}
                    </option>
                  ))}
                </select>
                <button className="btn btn-error btn-sm" onClick={() => handleDelete(d.id)} type="button">
                  Delete
                </button>
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
