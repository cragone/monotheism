import { useEffect } from 'react';
import { ScrollView } from 'react-native';
import { useRouter } from 'expo-router';
import { api } from '../lib/api';
import { useAuth } from '../lib/auth';
import { Composer } from '../components/Composer';
import { Card, Screen } from '../components/ui';

export default function NewThread() {
  const { user, ready } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (ready && !user) router.replace('/login');
  }, [ready, user, router]);

  return (
    <ScrollView keyboardShouldPersistTaps="handled">
      <Screen>
        <Card>
          <Composer
            withTitle
            placeholder="What's on your mind? (optional)"
            submitLabel="Post"
            onSubmit={async ({ title, body }) => {
              const { id } = await api.createThread(title, body);
              router.replace({ pathname: '/t/[id]', params: { id: String(id) } });
            }}
          />
        </Card>
      </Screen>
    </ScrollView>
  );
}
