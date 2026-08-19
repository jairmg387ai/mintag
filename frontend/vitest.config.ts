import { defineConfig } from 'vitest/config'

// Node-only unit test config — no jsdom/DOM dependency. Covers pure logic
// modules only (e.g. activityAutofill.ts). Component/integration testing
// (jsdom + Testing Library) is out of scope for this config; add a separate
// environment if that ever becomes necessary.
export default defineConfig({
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
