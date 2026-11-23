export interface HeardEvent {
  transcript: string;
  waveform: number[];
  timestamp: string;
}

// Wraps a plain EventSource on /api/hear-events. Relies on EventSource's
// native auto-reconnect rather than writing manual retry logic -- a
// transient server restart self-heals without any extra client code.
export function subscribeHearEvents(onEvent: (e: HeardEvent) => void): () => void {
  const source = new EventSource("/api/hear-events");
  source.addEventListener("heard", (ev) => {
    onEvent(JSON.parse((ev as MessageEvent).data));
  });
  return () => source.close();
}
