export interface SoundStatus {
  on: boolean;
  mode: string;
  volume: number;
}

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `request failed with status ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export function getSoundStatus(): Promise<SoundStatus> {
  return fetch("/api/sound/status").then((res) => handle<SoundStatus>(res));
}

export function listModes(): Promise<string[]> {
  return fetch("/api/sound/modes")
    .then((res) => handle<{ modes: string[] }>(res))
    .then((body) => body.modes);
}

export function setSoundMode(name: string): Promise<SoundStatus> {
  return fetch("/api/sound/mode", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  }).then((res) => handle<SoundStatus>(res));
}

export function setSoundVolume(level: number): Promise<SoundStatus> {
  return fetch("/api/sound/volume", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ level }),
  }).then((res) => handle<SoundStatus>(res));
}

export function setSoundPower(on: boolean): Promise<SoundStatus> {
  return fetch("/api/sound/power", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ on }),
  }).then((res) => handle<SoundStatus>(res));
}
