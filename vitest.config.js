import { defineConfig } from "vitest/config";

// 单元测试配置：jsdom 提供 window/DOM，globals 让测试文件可以直接用 describe/it/expect
export default defineConfig({
  test: {
    environment: "jsdom",
    globals: true,
  },
});
