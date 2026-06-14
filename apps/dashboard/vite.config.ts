import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";

// https://vite.dev/config/
export default defineConfig({
  // Read .env from the repo root so the whole monorepo shares one env file.
  // Only VITE_-prefixed vars are exposed to the client bundle.
  envDir: "../../",
  plugins: [
    // Generates src/routeTree.gen.ts from the files in src/routes.
    TanStackRouterVite({ target: "react", autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ],
});
