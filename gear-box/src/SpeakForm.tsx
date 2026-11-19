import { useState } from "react";
import { speak } from "./api/speak";

export default function SpeakForm() {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = text.trim();
    if (!trimmed || busy) return;

    setBusy(true);
    setError(null);
    try {
      await speak(trimmed);
      setText("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "failed to speak");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-3">
      <textarea
        className="textarea textarea-bordered w-full"
        placeholder="What should the house say?"
        value={text}
        maxLength={200}
        rows={3}
        onChange={(e) => setText(e.target.value)}
        disabled={busy}
      />
      <div className="flex items-center justify-between">
        <span className="text-sm opacity-60">{text.length}/200</span>
        <button type="submit" className="btn btn-primary" disabled={busy || !text.trim()}>
          {busy ? "Speaking…" : "Speak"}
        </button>
      </div>
      {error && <div className="alert alert-error text-sm">{error}</div>}
    </form>
  );
}
