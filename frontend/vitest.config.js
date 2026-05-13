import { defineConfig } from "vitest/config";

/**
 * Vitest 测试配置。
 */
export default defineConfig({
  /**
   * 测试环境配置。
   */
  test: {
    /**
     * 使用浏览器环境。
     */
    environment: "jsdom",
    /**
     * 初始化脚本。
     */
    setupFiles: ["./src/test/setup.js"]
  }
});
