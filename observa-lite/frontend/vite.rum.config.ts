import { defineConfig } from "vite";
export default defineConfig({
  build: {
    outDir: "dist/rum",
    emptyOutDir: true,
    lib: {
      entry: "src/rum/sdk.ts",
      name: "VetraBrowserSDK",
      formats: ["iife"],
      fileName: () => "vetra.js",
    },
  },
});
