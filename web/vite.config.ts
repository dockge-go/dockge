import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  // 前端版本：构建时由 VERSION 注入（上游 FRONTEND_VERSION 同款），用于 About 页展示
  define: {
    FRONTEND_VERSION: JSON.stringify(process.env.VERSION ?? "dev"),
  },
  server: {
    port: 5173,
    strictPort: true,
    host: "0.0.0.0",
    proxy: {
      "/v1": { target: "http://127.0.0.1:5001", changeOrigin: true, ws: true },
    },
  },
  build: {
    // 兼容基线：ES2020 + 对应浏览器（自托管面板的常见下限）
    target: "es2020",
    outDir: "dist",
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
  },
});
