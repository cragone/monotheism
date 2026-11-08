import { useMemo, useState } from 'react';
import { Alert, Platform, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { ago, api, type Reply } from '../lib/api';
import { useAuth } from '../lib/auth';
import { useTheme } from '../theme';
import { Composer } from './Composer';
import { LikeButton } from './LikeButton';
import { LinkText, Muted } from './ui';

// Replies nested deeper than this stay at the same indent so phone screens don't run out of width.
const MAX_DEPTH = 5;

export async function confirmDelete(what: string): Promise<boolean> {
  if (Platform.OS === 'web') return globalThis.confirm?.(`Delete this ${what}?`) ?? false;
  return new Promise((resolve) =>
    Alert.alert(`Delete this ${what}?`, undefined, [
      { text: 'Cancel', style: 'cancel', onPress: () => resolve(false) },
      { text: 'Delete', style: 'destructive', onPress: () => resolve(true) },
    ]),
  );
}

type Props = { threadId: string; replies: Reply[]; onChanged: () => void };

export function ReplyTree({ threadId, replies, onChanged }: Props) {
  const byParent = useMemo(() => {
    const m = new Map<number, Reply[]>();
    for (const r of replies) {
      const k = r.parent_id ?? 0;
      m.set(k, [...(m.get(k) ?? []), r]);
    }
    return m;
  }, [replies]);

  const level = (parent: number, depth: number) =>
    (byParent.get(parent) ?? []).map((r) => (
      <Node key={r.id} reply={r} depth={depth} threadId={threadId} onChanged={onChanged}>
        {level(r.id, Math.min(depth + 1, MAX_DEPTH))}
      </Node>
    ));

  return <View>{level(0, 0)}</View>;
}

function Node({ reply, depth, threadId, onChanged, children }: {
  reply: Reply; depth: number; threadId: string; onChanged: () => void; children: React.ReactNode;
}) {
  const t = useTheme();
  const { user } = useAuth();
  const router = useRouter();
  const [replying, setReplying] = useState(false);

  return (
    <View style={{ borderLeftWidth: 2, borderLeftColor: depth >= MAX_DEPTH ? t.accent : t.line, paddingLeft: 12, marginTop: 12 }}>
      {reply.deleted ? (
        <Text style={{ color: t.muted, fontStyle: 'italic' }}>[deleted]</Text>
      ) : (
        <>
          <Muted>
            @{reply.author} · {ago(reply.created_at)}
          </Muted>
          <Text style={{ color: t.ink, marginVertical: 4, lineHeight: 22 }}>{reply.body}</Text>
          <View style={{ flexDirection: 'row', gap: 14, alignItems: 'center' }}>
            <LikeButton kind="replies" id={reply.id} liked={reply.liked} likes={reply.likes} />
            <LinkText label="Reply" onPress={() => (user ? setReplying((v) => !v) : router.push('/login'))} />
            {reply.mine && (
              <LinkText
                label="Delete"
                onPress={async () => {
                  if (await confirmDelete('reply')) {
                    await api.deleteReply(reply.id).catch((e) => Alert.alert('Error', e.message));
                    onChanged();
                  }
                }}
              />
            )}
          </View>
          {replying && (
            <View style={{ marginTop: 8 }}>
              <Composer
                placeholder={`Reply to @${reply.author}`}
                submitLabel="Reply"
                onCancel={() => setReplying(false)}
                onSubmit={async ({ body }) => {
                  await api.reply(threadId, body, reply.id);
                  setReplying(false);
                  onChanged();
                }}
              />
            </View>
          )}
        </>
      )}
      {children}
    </View>
  );
}
