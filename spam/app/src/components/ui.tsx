import { type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View, type TextInputProps, type ViewStyle } from 'react-native';
import { MAX_WIDTH, useTheme } from '../theme';

export function Screen({ children }: { children: ReactNode }) {
  return <View style={styles.screen}>{children}</View>;
}

export function Card({ children, style }: { children: ReactNode; style?: ViewStyle }) {
  const t = useTheme();
  return <View style={[styles.card, { backgroundColor: t.card, borderColor: t.line }, style]}>{children}</View>;
}

export function Button({
  label, onPress, primary, disabled,
}: { label: string; onPress: () => void; primary?: boolean; disabled?: boolean }) {
  const t = useTheme();
  return (
    <Pressable
      accessibilityRole="button"
      onPress={onPress}
      disabled={disabled}
      style={[
        styles.btn,
        { borderColor: primary ? t.accent : t.line, backgroundColor: primary ? t.accent : t.card },
        disabled && { opacity: 0.5 },
      ]}>
      <Text style={{ color: primary ? '#fff' : t.ink, fontWeight: primary ? '700' : '500' }}>{label}</Text>
    </Pressable>
  );
}

export function LinkText({ label, onPress, color, bold }: { label: string; onPress: () => void; color?: string; bold?: boolean }) {
  const t = useTheme();
  return (
    <Pressable accessibilityRole="link" onPress={onPress} hitSlop={8}>
      <Text style={{ color: color ?? t.muted, fontSize: 14, fontWeight: bold ? '600' : '400' }}>{label}</Text>
    </Pressable>
  );
}

export function Field(props: TextInputProps) {
  const t = useTheme();
  return (
    <TextInput
      placeholderTextColor={t.muted}
      {...props}
      style={[styles.input, { color: t.ink, backgroundColor: t.card, borderColor: t.line }, props.multiline && styles.multiline, props.style]}
    />
  );
}

export function Muted({ children }: { children: ReactNode }) {
  const t = useTheme();
  return <Text style={{ color: t.muted, fontSize: 13 }}>{children}</Text>;
}

export function ErrorText({ message }: { message: string }) {
  const t = useTheme();
  return message ? <Text style={{ color: t.danger, fontSize: 14 }}>{message}</Text> : null;
}

const styles = StyleSheet.create({
  screen: { width: '100%', maxWidth: MAX_WIDTH, alignSelf: 'center', padding: 16 },
  card: { borderWidth: 1, borderRadius: 14, padding: 14, marginBottom: 12 },
  btn: { borderWidth: 1, borderRadius: 999, paddingVertical: 7, paddingHorizontal: 16, alignItems: 'center' },
  input: { borderWidth: 1, borderRadius: 10, paddingHorizontal: 12, paddingVertical: 9, fontSize: 16 },
  multiline: { minHeight: 90, textAlignVertical: 'top' },
});
