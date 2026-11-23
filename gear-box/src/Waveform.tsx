const WIDTH = 300;
const HEIGHT = 60;

export default function Waveform({ points }: { points: number[] }) {
  if (points.length === 0) {
    return <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} className="w-full h-16" />;
  }

  const peak = Math.max(1, ...points.map((p) => Math.abs(p)));
  const mid = HEIGHT / 2;
  const step = WIDTH / points.length;

  const bars = points.map((p, i) => {
    const barHeight = Math.max(1, (Math.abs(p) / peak) * mid);
    return (
      <rect
        key={i}
        x={i * step}
        y={mid - barHeight}
        width={Math.max(1, step - 1)}
        height={barHeight * 2}
        className="fill-primary"
      />
    );
  });

  return (
    <svg viewBox={`0 0 ${WIDTH} ${HEIGHT}`} className="w-full h-16">
      {bars}
    </svg>
  );
}
