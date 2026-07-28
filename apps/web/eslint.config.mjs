import antfu from '@antfu/eslint-config'
import reactHooks from 'eslint-plugin-react-hooks'

export default antfu(
  {
    formatters: {
      css: true,
      html: true,
    },
    ignores: [
      '.react-router/**',
      'app/services/generated/**',
      'build/**',
      'node_modules/**',
    ],
    markdown: false,
    react: true,
    stylistic: {
      indent: 2,
      quotes: 'single',
      semi: false,
    },
    toml: false,
    typescript: true,
    yaml: false,
  },
  {
    files: ['**/*.{js,jsx,ts,tsx}'],
    name: 'admin-multi-tenant/react-hooks',
    plugins: {
      'react-hooks': reactHooks,
    },
    rules: {
      'react-hooks/exhaustive-deps': 'warn',
      'react-hooks/rules-of-hooks': 'error',
      'react/exhaustive-deps': 'off',
      'react/set-state-in-effect': 'off',
      'react-refresh/only-export-components': 'off',
    },
  },
  {
    files: ['*.config.{js,mjs,ts}', 'config/**/*.{js,mjs,ts}'],
    name: 'admin-multi-tenant/node-config',
    rules: {
      'node/prefer-global/process': 'off',
    },
  },
)
