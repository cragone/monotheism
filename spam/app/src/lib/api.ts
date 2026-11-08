import { Platform } from 'react-native';
import * as SecureStore from 'expo-secure-store';

// On web the app is served by the same Go server as the API, so use relative URLs.
// Native builds talk to production unless EXPO_PUBLIC_API_URL points elsewhere (e.g. a LAN dev box).
const BASE =
  Platform.OS === 'web' ? '' : (process.env.EXPO_PUBLIC_API_URL ?? 'https://spam.charlesragone.com');

const TOKEN_KEY = 'spam_token';

export async function getToken(): Promise<string | null> {
  try {
    if (Platform.OS === 'web') return globalThis.localStorage?.getItem(TOKEN_KEY) ?? null;
    return await SecureStore.getItemAsync(TOKEN_KEY);
  } catch {
    return null;
  }
}

export async function setToken(token: string | null): Promise<void> {
  try {
    if (Platform.OS === 'web') {
      if (token) globalThis.localStorage?.setItem(TOKEN_KEY, token);
      else globalThis.localStorage?.removeItem(TOKEN_KEY);
    } else if (token) await SecureStore.setItemAsync(TOKEN_KEY, token);
    else await SecureStore.deleteItemAsync(TOKEN_KEY);
  } catch {
    // storage unavailable (private mode etc.): the session just won't persist
  }
}

export type User = { id: number; username: string };

export type Thread = {
  id: number;
  title: string;
  body: string;
  truncated: boolean;
  author: string | null;
  mine: boolean;
  created_at: number;
  last_activity: number;
  likes: number;
  reply_count: number;
  liked: boolean;
  deleted: boolean;
};

export type Reply = {
  id: number;
  parent_id: number | null;
  body: string;
  author: string | null;
  mine: boolean;
  created_at: number;
  likes: number;
  liked: boolean;
  deleted: boolean;
};

export type Sort = 'active' | 'new' | 'top';

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = await getToken();
  const res = await fetch(BASE + path, {
    method,
    headers: {
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
  return data as T;
}

type AuthResult = { token: string; user: User };

export const api = {
  me: () => request<{ user: User | null }>('GET', '/api/me'),
  register: (username: string, password: string) =>
    request<AuthResult>('POST', '/api/register', { username, password }),
  login: (username: string, password: string) =>
    request<AuthResult>('POST', '/api/login', { username, password }),
  logout: () => request<{ ok: true }>('POST', '/api/logout', {}),
  threads: (sort: Sort, page: number) =>
    request<{ threads: Thread[]; has_more: boolean }>('GET', `/api/threads?sort=${sort}&page=${page}`),
  thread: (id: number | string) =>
    request<{ thread: Thread; replies: Reply[] }>('GET', `/api/threads/${id}`),
  createThread: (title: string, body: string) =>
    request<{ id: number }>('POST', '/api/threads', { title, body }),
  deleteThread: (id: number) => request<{ ok: true }>('DELETE', `/api/threads/${id}`),
  reply: (threadId: number | string, body: string, parentId?: number) =>
    request<{ id: number }>('POST', `/api/threads/${threadId}/replies`, { body, parent_id: parentId }),
  deleteReply: (id: number) => request<{ ok: true }>('DELETE', `/api/replies/${id}`),
  like: (kind: 'threads' | 'replies', id: number) =>
    request<{ liked: boolean; likes: number }>('POST', `/api/${kind}/${id}/like`),
};

export function ago(ts: number): string {
  let n = Math.max(1, Math.round((Date.now() - ts) / 1000));
  for (const [div, label] of [[60, 's'], [60, 'm'], [24, 'h'], [30, 'd'], [12, 'mo']] as const) {
    if (n < div) return `${n}${label}`;
    n = Math.floor(n / div);
  }
  return `${n}y`;
}
