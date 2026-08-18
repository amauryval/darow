import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config'

// jsdom for every test: a composable is exercised inside a mounted
// component (onMounted, watchers, teardown), which needs a document.
export default mergeConfig(
    viteConfig,
    defineConfig({
        test: {
            environment: 'jsdom',
            restoreMocks: true
        }
    })
)
