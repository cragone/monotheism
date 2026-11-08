import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, View } from 'react-native';
import { useRouter } from 'expo-router';
import { api, type Sort, type Thread } from '../lib/api';
import { useAuth } from '../lib/auth';
import { ThreadCard } from '../components/ThreadCard';
import { Button, ErrorText, Muted } from '../components/ui';

const SORTS: { key: Sort; label: string }[] = [
  { key: 'active', label: 'Active' },
  { key: 'new', label: 'New' },
  { key: 'top', label: 'Top' },
];

export default function Feed() {
  const { user, ready } = useAuth();
  const router = useRouter();
  const [sort, setSort] = useState<Sort>('active');
  const [threads, setThreads] = useState<Thread[]>([]);
  const [page, setPage] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = useCallback(async (s: Sort, p: number) => {
    setLoading(true);
    setError('');
    try {
      const r = await api.threads(s, p);
      setThreads((prev) => (p === 0 ? r.threads : [...prev, ...r.threads]));
      setHasMore(r.has_more);
      setPage(p);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  // Reload when the sort changes, and again once we know who is signed in (for "liked" state).
  useEffect(() => {
    if (ready) load(sort, 0);
  }, [sort, ready, user?.id, load]);

  return (
    <FlatList
      data={threads}
      keyExtractor={(t) => String(t.id)}
      renderItem={({ item }) => <ThreadCard thread={item} />}
      refreshing={loading && page === 0}
      onRefresh={() => load(sort, 0)}
      ListHeaderComponent={
        <View style={{ flexDirection: 'row', gap: 6, marginBottom: 12 }}>
          {SORTS.map((s) => (
            <Button key={s.key} label={s.label} primary={sort === s.key} onPress={() => setSort(s.key)} />
          ))}
        </View>
      }
      ListEmptyComponent={
        loading ? (
          <ActivityIndicator style={{ marginTop: 40 }} />
        ) : error ? (
          <ErrorText message={error} />
        ) : (
          <View style={{ alignItems: 'center', gap: 12, paddingVertical: 40 }}>
            <Muted>No threads yet.</Muted>
            <Button label="Start the first one" primary onPress={() => router.push(user ? '/new' : '/register')} />
          </View>
        )
      }
      ListFooterComponent={hasMore ? <Button label="Load more" onPress={() => load(sort, page + 1)} disabled={loading} /> : undefined}
      contentContainerStyle={{ width: '100%', maxWidth: 680, alignSelf: 'center', padding: 16 }}
    />
  );
}
