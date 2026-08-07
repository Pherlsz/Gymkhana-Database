import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
    }),
    react(),
  ],
  server: {
    port: 5173,
    proxy: {
      "/health": "http://localhost:8080",
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["@testing-library/jest-dom"],
  },
});
