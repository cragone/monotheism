import { StyleSheet, useColorScheme } from 'react-native';

const light = { bg: '#fafaf7', card: '#ffffff', ink: '#1c1b1a', muted: '#6f6b66', line: '#e7e4de', accent: '#ff5a36', like: '#e0245e', danger: '#d33' };
const dark = { bg: '#131211', card: '#1c1b1a', ink: '#f1eeea', muted: '#9a958e', line: '#2d2b29', accent: '#ff5a36', like: '#ff6b93', danger: '#ff6b6b' };

export type Theme = typeof light;
export const useTheme = (): Theme => (useColorScheme() === 'dark' ? dark : light);

export const MAX_WIDTH = 680;
export const shared = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 14 },
});
