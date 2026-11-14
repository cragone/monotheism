import DeviceList from "../components/DeviceList";
import { useDevices } from "../hooks/useDevices";

export default function DeviceControlPage() {
  const { devices, error, loading, now, refresh } = useDevices();

  return (
    <>
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
    </>
  );
}
