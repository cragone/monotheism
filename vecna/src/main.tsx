import { createRoot } from "react-dom/client";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import "./index.css";
import Layout from "./Layout";
import DeviceControlPage from "./pages/DeviceControlPage";
import DeviceSettingsPage from "./pages/DeviceSettingsPage";

createRoot(document.getElementById("root")!).render(
  <BrowserRouter>
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<DeviceControlPage />} />
        <Route path="/settings" element={<DeviceSettingsPage />} />
      </Route>
    </Routes>
  </BrowserRouter>,
);
