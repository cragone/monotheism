import SpeakForm from "./SpeakForm";

export default function App() {
  return (
    <div className="min-h-screen bg-base-100 flex justify-center p-6">
      <div className="w-full max-w-md flex flex-col gap-6">
        <h1 className="text-2xl font-semibold">gear-box</h1>
        <SpeakForm />
      </div>
    </div>
  );
}
