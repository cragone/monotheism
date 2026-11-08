import { Pressable, Text, View } from 'react-native';
import { useRouter } from 'expo-router';
import { ago, type Thread } from '../lib/api';
import { useTheme } from '../theme';
import { LikeButton } from './LikeButton';
import { Card, Muted } from './ui';

export function ThreadCard({ thread }: { thread: Thread }) {
  const t = useTheme();
  const router = useRouter();
  const open = () => router.push({ pathname: '/t/[id]', params: { id: String(thread.id) } });
  return (
    <Card>
      <Pressable accessibilityRole="link" onPress={open}>
        <Text style={{ color: t.ink, fontSize: 17, fontWeight: '700' }}>{thread.title}</Text>
        {!!thread.body && (
          <Text style={{ color: t.ink, marginTop: 4, lineHeight: 22 }}>
            {thread.body}
            {thread.truncated ? '…' : ''}
          </Text>
        )}
      </Pressable>
      <View style={{ flexDirection: 'row', gap: 14, alignItems: 'center', marginTop: 8 }}>
        <Muted>@{thread.author}</Muted>
        <Muted>{ago(thread.created_at)}</Muted>
        <LikeButton kind="threads" id={thread.id} liked={thread.liked} likes={thread.likes} />
        <Pressable onPress={open} hitSlop={8}>
          <Muted>
            {thread.reply_count} {thread.reply_count === 1 ? 'reply' : 'replies'}
          </Muted>
        </Pressable>
      </View>
    </Card>
  );
}
