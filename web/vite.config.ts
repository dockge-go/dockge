import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  server: {
    port: 5174, strictPort: true,
    /* dev 态把 /v1/* 转发到后端单二进制；生产同源 5001 无需配置，两态同代码（D-04） */
    proxy: { '/v1': { target: 'http://127.0.0.1:5001', changeOrigin: true } },
  },
  build: { outDir: "dist", emptyOutDir: true },
});
