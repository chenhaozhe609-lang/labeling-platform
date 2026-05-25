import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
    rules: {
      // 以下均为新插件版本的严格「性能/风格」建议规则（非正确性 bug），会误报合法模式：
      //  - set-state-in-effect / refs / immutability：自动保存、草稿、看板轮询里的 effect 同步与 latest-ref；
      //  - react-refresh/only-export-components：shadcn/ui 组件惯用「组件 + variants」同文件导出（仅影响 dev 热更新）。
      // 统一降为 warning：不阻断 CI，但仍提示，待 B1.8 集中清理。
      'react-hooks/set-state-in-effect': 'warn',
      'react-hooks/refs': 'warn',
      'react-hooks/immutability': 'warn',
      'react-refresh/only-export-components': 'warn',
    },
  },
])
