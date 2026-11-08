import { useState } from 'react';
import { Alert, Pressable, Text } from 'react-native';
import { useRouter } from 'expo-router';
import { api } from '../lib/api';
import { useAuth } from '../lib/auth';
import { useTheme } from '../theme';

type Props = { kind: 'threads' | 'replies'; id: number; liked: boolean; likes: number };

export function LikeButton({ kind, id, liked: l0, likes: n0 }: Props) {
  const t = useTheme();
  const { user } = useAuth();
  const router = useRouter();
  const [s, setS] = useState({ liked: l0, likes: n0 });

  async function toggle() {
    if (!user) return router.push('/login');
    try {
      setS(await api.like(kind, id));
    } catch (e) {
      Alert.alert('Could not like', (e as Error).message);
    }
  }

  return (
    <Pressable accessibilityRole="button" accessibilityState={{ selected: s.liked }} onPress={toggle} hitSlop={8}>
      <Text style={{ color: s.liked ? t.like : t.muted, fontSize: 14, fontWeight: s.liked ? '700' : '400' }}>
        {s.liked ? '♥' : '♡'} {s.likes}
      </Text>
    </Pressable>
  );
}
