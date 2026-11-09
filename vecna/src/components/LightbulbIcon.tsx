export default function LightbulbIcon({ on, className }: { on: boolean; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      className={className}
      fill="none"
      stroke="currentColor"
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path
        d="M9 21h6M10 23h4M8.5 17.5v-1.84c0-.53-.24-1.03-.68-1.33A6.5 6.5 0 1 1 12 2.5a6.5 6.5 0 0 1 4.18 11.33c-.44.3-.68.8-.68 1.33v1.84a1 1 0 0 1-1 1h-5a1 1 0 0 1-1-1Z"
        fill={on ? "currentColor" : "none"}
        fillOpacity={on ? 0.25 : 0}
      />
      <path d="M12 2.5a6.5 6.5 0 0 1 4.18 11.33c-.44.3-.68.8-.68 1.33v1.84a1 1 0 0 1-1 1h-5a1 1 0 0 1-1-1v-1.84c0-.53-.24-1.03-.68-1.33A6.5 6.5 0 0 1 12 2.5Z" />
      <path d="M9.5 17.5h5" />
      {on && (
        <>
          <path d="M12 6.5a3.5 3.5 0 0 0-3.5 3.5" strokeWidth={1.25} opacity={0.6} />
        </>
      )}
    </svg>
  );
}
