import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    environment: 'node',
    env: {
      NODE_ENV: 'test',
      TARO_APP_API_BASE_URL: 'https://test.example.com/api/miniapp',
      TARO_APP_ENV: 'test',
    },
  },
})
