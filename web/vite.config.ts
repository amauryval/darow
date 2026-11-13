/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

// The platform's web library ships its sources (src/), imported by its package name: the alias points
// the name at them, ahead of '@'.
const platform = path.resolve(import.meta.dirname, './node_modules/@myapplibs/web-platform/src')

// https://vite.dev/config/
export default defineConfig({
    build: {
        // Vite empties outDir before bundling — on a failed build it would leave dist/ without the
        // committed .gitkeep the Go embed needs. build-frontend clears the previous output instead.
        emptyOutDir: false,
        rolldownOptions: {
            external: ['._*'] // Ignore les fichiers ._*
        }
    },
    // Sources, not a bundle: Vite compiles them with the application's own.
    optimizeDeps: {
        exclude: ['@myapplibs/web-platform']
    },
    server: {
        proxy: {
            '/api/v1': {
                target: 'http://127.0.0.1:3000',
                changeOrigin: false,
                secure: false,
                ws: true
            }
        },
        watch: {}
    },
    define: {
        __VUE_I18N_FULL_INSTALL__: false,
        __VUE_I18N_LEGACY_API__: false,
        __INTLIFY_PROD_DEVTOOLS__: false
    },
    plugins: [vue(), tailwindcss()],
    resolve: {
        alias: {
            '@myapplibs/web-platform': platform,
            '@': path.resolve(import.meta.dirname, './src')
        }
    },
    test: {
        environment: 'jsdom',
        setupFiles: ['src/i18n.setup.ts'],
        server: {
            deps: {
                inline: ['@myapplibs/web-platform']
            }
        }
    }
})
