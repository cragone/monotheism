import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { api, getToken, setToken, type User } from './api';

type AuthState = {
  user: User | null;
  ready: boolean;
  signIn: (mode: 'login' | 'register', username: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const Ctx = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        if (await getToken()) setUser((await api.me()).user);
      } catch {
        // offline or server down: treat as signed out, keep the token for next launch
      }
      setReady(true);
    })();
  }, []);

  const signIn = useCallback(async (mode: 'login' | 'register', username: string, password: string) => {
    const r = await (mode === 'login' ? api.login : api.register)(username, password);
    await setToken(r.token);
    setUser(r.user);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await api.logout();
    } catch {
      // revoke is best-effort; always drop the local token
    }
    await setToken(null);
    setUser(null);
  }, []);

  const value = useMemo(() => ({ user, ready, signIn, signOut }), [user, ready, signIn, signOut]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthState {
  const v = useContext(Ctx);
  if (!v) throw new Error('useAuth must be used inside AuthProvider');
  return v;
}
