import { useState } from 'react';
import { ScrollView, View } from 'react-native';
import { useRouter } from 'expo-router';
import { useAuth } from '../lib/auth';
import { Button, Card, ErrorText, Field, LinkText, Screen } from './ui';

export function AuthForm({ mode }: { mode: 'login' | 'register' }) {
  const { signIn } = useAuth();
  const router = useRouter();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const login = mode === 'login';

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await signIn(mode, username.trim(), password);
      if (router.canGoBack()) router.back();
      else router.replace('/');
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  }

  return (
    <ScrollView keyboardShouldPersistTaps="handled">
      <Screen>
        <Card>
          <View style={{ gap: 8 }}>
            <Field
              placeholder="Username"
              value={username}
              onChangeText={setUsername}
              autoCapitalize="none"
              autoCorrect={false}
              autoComplete="username"
              maxLength={24}
            />
            <Field
              placeholder={login ? 'Password' : 'Password (8+ characters)'}
              value={password}
              onChangeText={setPassword}
              secureTextEntry
              autoComplete={login ? 'current-password' : 'new-password'}
              onSubmitEditing={submit}
            />
            <ErrorText message={error} />
            <Button label={login ? 'Log in' : 'Create account'} primary onPress={submit} disabled={busy || !username || !password} />
          </View>
        </Card>
        <LinkText
          label={login ? 'New here? Sign up' : 'Have an account? Log in'}
          onPress={() => router.replace(login ? '/register' : '/login')}
        />
      </Screen>
    </ScrollView>
  );
}
