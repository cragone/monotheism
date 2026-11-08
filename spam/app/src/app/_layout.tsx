import { Stack, useRouter } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { View } from 'react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { Button } from '../components/ui';
import { AuthProvider, useAuth } from '../lib/auth';
import { useTheme } from '../theme';

function HeaderRight() {
  const { user, signOut } = useAuth();
  const router = useRouter();
  return (
    <View style={{ flexDirection: 'row', gap: 8, marginRight: 8 }}>
      {user ? (
        <>
          <Button label="New" primary onPress={() => router.push('/new')} />
          <Button label="Log out" onPress={signOut} />
        </>
      ) : (
        <>
          <Button label="Log in" onPress={() => router.push('/login')} />
          <Button label="Sign up" primary onPress={() => router.push('/register')} />
        </>
      )}
    </View>
  );
}

function Shell() {
  const t = useTheme();
  return (
    <Stack
      screenOptions={{
        headerStyle: { backgroundColor: t.bg },
        headerTintColor: t.ink,
        headerShadowVisible: false,
        contentStyle: { backgroundColor: t.bg },
        headerRight: () => <HeaderRight />,
      }}>
      <Stack.Screen name="index" options={{ title: 'spam.' }} />
      <Stack.Screen name="t/[id]" options={{ title: 'Thread' }} />
      <Stack.Screen name="new" options={{ title: 'New thread' }} />
      <Stack.Screen name="login" options={{ title: 'Log in' }} />
      <Stack.Screen name="register" options={{ title: 'Sign up' }} />
    </Stack>
  );
}

export default function RootLayout() {
  return (
    <SafeAreaProvider>
      <AuthProvider>
        <StatusBar style="auto" />
        <Shell />
      </AuthProvider>
    </SafeAreaProvider>
  );
}
