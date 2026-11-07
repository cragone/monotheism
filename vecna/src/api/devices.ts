export interface Device {
  id: number;
  uniqueId: string;
  name: string;
  supportsBrightness: boolean;
  lastPowerState: string;
  lastBrightness: number;
  lastSeenAt: string | null;
  createdAt: string;
  updatedAt: string;
}

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `request failed with status ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export function listDevices(): Promise<Device[]> {
  return fetch("/api/devices").then((res) => handle<Device[]>(res));
}

export function getDevice(id: number): Promise<Device> {
  return fetch(`/api/devices/${id}`).then((res) => handle<Device>(res));
}

export function setPower(id: number, on: boolean): Promise<Device> {
  return fetch(`/api/devices/${id}/power`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ on }),
  }).then((res) => handle<Device>(res));
}

export function setBrightness(id: number, level: number): Promise<Device> {
  return fetch(`/api/devices/${id}/brightness`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ level }),
  }).then((res) => handle<Device>(res));
}
