async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `request failed with status ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export function speak(text: string): Promise<{ ok: boolean }> {
  return fetch("/api/speak", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  }).then((res) => handle<{ ok: boolean }>(res));
}
