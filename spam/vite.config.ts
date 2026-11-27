import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    allowedHosts: ["spam.charlesragone.com", "localhost"],
    proxy: { "/api": { target: "http://localhost:8080", changeOrigin: false } },
  },
});
