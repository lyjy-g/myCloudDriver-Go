import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

/**
 * Vite 构建配置。
 */
export default defineConfig({
  /**
   * 插件列表。
   */
  plugins: [react()],
  /**
   * 开发服务器配置。
   */
  server: {
    /**
     * 开发端口。
     */
    port: 5173
  }
});
