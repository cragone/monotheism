export type DeviceType = "light" | "sound_machine" | "garage" | "temp";

export const DEVICE_TYPES: { value: DeviceType; label: string; emoji: string }[] = [
  { value: "light", label: "Light", emoji: "💡" },
  { value: "sound_machine", label: "Sound Machine", emoji: "🔊" },
  { value: "garage", label: "Garage", emoji: "🚪" },
  { value: "temp", label: "Temperature", emoji: "🌡️" },
];

export function emojiFor(type: string): string {
  return DEVICE_TYPES.find((t) => t.value === type)?.emoji ?? "❓";
}
