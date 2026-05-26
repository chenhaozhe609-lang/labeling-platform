import { defineConfig } from 'vitest/config'
import path from 'node:path'

// vitest 配置（与 vite build 分开，避免给生产构建引入 test 依赖）。
// 用 esbuild 的 automatic JSX 运行时（react/jsx-runtime），无需在每个文件 import React，
// 也不依赖 @vitejs/plugin-react 的 fast-refresh（测试用不到）。
export default defineConfig({
  resolve: {
    alias: { '@': path.resolve(import.meta.dirname, './src') },
  },
  esbuild: { jsx: 'automatic' },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
})
