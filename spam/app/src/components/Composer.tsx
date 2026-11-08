import { useState } from 'react';
import { View } from 'react-native';
import { Button, ErrorText, Field } from './ui';

type Props = {
  placeholder: string;
  submitLabel: string;
  withTitle?: boolean;
  onSubmit: (v: { title: string; body: string }) => Promise<void>;
  onCancel?: () => void;
};

export function Composer({ placeholder, submitLabel, withTitle, onSubmit, onCancel }: Props) {
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await onSubmit({ title, body });
      setTitle('');
      setBody('');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <View style={{ gap: 8 }}>
      {withTitle && <Field placeholder="Title" value={title} onChangeText={setTitle} maxLength={200} />}
      <Field placeholder={placeholder} value={body} onChangeText={setBody} multiline maxLength={withTitle ? 10000 : 5000} />
      <ErrorText message={error} />
      <View style={{ flexDirection: 'row', justifyContent: 'flex-end', gap: 8 }}>
        {onCancel && <Button label="Cancel" onPress={onCancel} />}
        <Button label={submitLabel} primary onPress={submit} disabled={busy || (withTitle ? !title.trim() : !body.trim())} />
      </View>
    </View>
  );
}
