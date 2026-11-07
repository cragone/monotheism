import { useEffect, useRef, useState } from "react";
import { listDevices, type Device } from "../api/devices";

const POLL_INTERVAL_MS = 2000;

export function useDevices() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [now, setNow] = useState(0);
  const [generation, setGeneration] = useState(0);
  const mountedRef = useRef(true);

  function refresh() {
    setGeneration((g) => g + 1);
  }

  useEffect(() => {
    mountedRef.current = true;

    async function load() {
      try {
        const next = await listDevices();
        if (!mountedRef.current) return;
        setDevices(next);
        setError(null);
      } catch (err) {
        if (!mountedRef.current) return;
        setError(err instanceof Error ? err.message : "failed to load devices");
      } finally {
        if (mountedRef.current) {
          setLoading(false);
          setNow(Date.now());
        }
      }
    }

    void load();
    const interval = setInterval(load, POLL_INTERVAL_MS);
    return () => {
      mountedRef.current = false;
      clearInterval(interval);
    };
  }, [generation]);

  return { devices, error, loading, now, refresh };
}
