import DeviceList from "./components/DeviceList";
import { useDevices } from "./hooks/useDevices";

export default function App() {
  const { devices, error, loading, now, refresh } = useDevices();

  return (
    <div className="min-h-screen bg-base-100">
      <div className="navbar bg-base-200 shadow-sm px-4">
        <span className="text-lg font-semibold">Vecna</span>
        <span className="ml-2 text-sm opacity-60">house control</span>
      </div>

      <main className="max-w-5xl mx-auto p-4 space-y-4">
        {error && (
          <div className="alert alert-error">
            <span>{error}</span>
          </div>
        )}
        {loading ? (
          <div className="flex justify-center p-8">
            <span className="loading loading-spinner loading-lg" />
          </div>
        ) : (
          <DeviceList devices={devices} now={now} onChanged={refresh} />
        )}
      </main>
    </div>
  );
}
