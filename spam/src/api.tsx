import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { createIdentity, forgetIdentity, importPhrase, loadIdentity, signedFetch, type Identity } from "./account";

export type Author = { pubkey: string; username: string; display_name: string };
export type Post = {
  id: number; parent_id: number | null; body: string; created_at: number;
  author: Author; likes: number; replies: number; liked: boolean;
};
export type User = { pubkey: string; username: string; display_name: string; bio: string; created_at: number };

type Ctx = {
  id: Identity | null;
  me: User | null;
  ready: boolean;
  call: <T = unknown>(method: string, path: string, body?: unknown) => Promise<T>;
  create: () => void;
  recover: (phrase: string) => void;
  logout: () => void;
  setMe: (u: User) => void;
};

const AppCtx = createContext<Ctx>(null!);
export const useApp = () => useContext(AppCtx);

export function AppProvider({ children }: { children: ReactNode }) {
  const [id, setId] = useState<Identity | null>(() => loadIdentity());
  const [me, setMe] = useState<User | null>(null);
  const [ready, setReady] = useState(false);

  const call = useCallback(
    async <T,>(method: string, path: string, body?: unknown): Promise<T> => {
      const r = await signedFetch(id, method, path, body);
      const j = await r.json().catch(() => ({}));
      if (!r.ok) throw new Error(j.message || `Request failed (${r.status})`);
      return j as T;
    },
    [id],
  );

  useEffect(() => {
    let live = true;
    setMe(null);
    if (!id) { setReady(true); return; }
    setReady(false);
    call<{ user: User | null }>("GET", "/api/me")
      .then((r) => live && setMe(r.user))
      .catch(() => {})
      .finally(() => live && setReady(true));
    return () => { live = false; };
  }, [id, call]);

  const value = useMemo<Ctx>(() => ({
    id, me, ready, call, setMe,
    create: () => setId(createIdentity()),
    recover: (p) => setId(importPhrase(p)),
    logout: () => { forgetIdentity(); setId(null); },
  }), [id, me, ready, call]);

  return <AppCtx.Provider value={value}>{children}</AppCtx.Provider>;
}

export function timeAgo(ts: number) {
  const s = Math.max(1, Math.floor(Date.now() / 1000 - ts));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}
