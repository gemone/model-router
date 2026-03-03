import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import { resolve } from "path";

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./src/test/setup.js"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      exclude: [
        "node_modules/",
        "dist/",
        "src/main.js",
        "src/router/",
        "src/i18n/index.js",
        "src/test/",
        "**/*.config.js",
        "**/*.test.js",
        "**/*.spec.js",
      ],
    },
    include: ["src/**/*.test.js", "src/**/*.spec.js"],
  },
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
    },
  },
});
