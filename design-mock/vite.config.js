import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  // Home Assistant ingress serves app UIs under a Supervisor-owned path.
  // Relative asset URLs keep the built bundle portable when it is copied into
  // the real app and served below that ingress prefix.
  base: "./",
  plugins: [react()],
});
