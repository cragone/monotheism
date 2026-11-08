import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Alert, ScrollView, Text, View } from 'react-native';
import { Stack, useLocalSearchParams, useRouter } from 'expo-router';
import { ago, api, type Reply, type Thread } from '../../lib/api';
import { useAuth } from '../../lib/auth';
import { useTheme } from '../../theme';
import { Composer } from '../../components/Composer';
import { LikeButton } from '../../components/LikeButton';
import { confirmDelete, ReplyTree } from '../../components/ReplyTree';
import { Card, ErrorText, LinkText, Muted, Screen } from '../../components/ui';

export default function ThreadScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const t = useTheme();
  const { user, ready } = useAuth();
  const router = useRouter();
  const [data, setData] = useState<{ thread: Thread; replies: Reply[] } | null>(null);
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      setData(await api.thread(id));
      setError('');
    } catch (e) {
      setError((e as Error).message);
    }
  }, [id]);

  useEffect(() => {
    if (ready) load();
  }, [ready, user?.id, load]);

  if (error) return <Screen><ErrorText message={error} /></Screen>;
  if (!data) return <ActivityIndicator style={{ marginTop: 40 }} />;
  const { thread, replies } = data;

  return (
    <ScrollView keyboardShouldPersistTaps="handled">
      <Stack.Screen options={{ title: thread.title.slice(0, 40) }} />
      <Screen>
        <Card>
          <Text style={{ color: t.ink, fontSize: 19, fontWeight: '700' }}>{thread.title}</Text>
          {thread.deleted ? (
            <Text style={{ color: t.muted, fontStyle: 'italic', marginTop: 4 }}>This thread was deleted.</Text>
          ) : (
            <>
              {!!thread.body && <Text style={{ color: t.ink, marginTop: 6, lineHeight: 22 }}>{thread.body}</Text>}
              <View style={{ flexDirection: 'row', gap: 14, alignItems: 'center', marginTop: 10 }}>
                <Muted>@{thread.author}</Muted>
                <Muted>{ago(thread.created_at)}</Muted>
                <LikeButton kind="threads" id={thread.id} liked={thread.liked} likes={thread.likes} />
                {thread.mine && (
                  <LinkText
                    label="Delete"
                    onPress={async () => {
                      if (!(await confirmDelete('thread'))) return;
                      try {
                        await api.deleteThread(thread.id);
                        router.replace('/');
                      } catch (e) {
                        Alert.alert('Error', (e as Error).message);
                      }
                    }}
                  />
                )}
              </View>
            </>
          )}
        </Card>

        {!thread.deleted && (
          <Card>
            {user ? (
              <Composer
                placeholder="Add a reply…"
                submitLabel="Reply"
                onSubmit={async ({ body }) => {
                  await api.reply(id, body);
                  await load();
                }}
              />
            ) : (
              <LinkText label="Log in to reply" onPress={() => router.push('/login')} bold />
            )}
          </Card>
        )}

        <ReplyTree threadId={id} replies={replies} onChanged={load} />
      </Screen>
    </ScrollView>
  );
}
