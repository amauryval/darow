import vue from 'eslint-plugin-vue'
import tsPlugin from '@typescript-eslint/eslint-plugin'
import tsParser from '@typescript-eslint/parser'
import vueParser from 'vue-eslint-parser'
import unusedImports from 'eslint-plugin-unused-imports'
import prettierConfig from 'eslint-config-prettier'
import globals from 'globals'
import vueI18n from '@intlify/eslint-plugin-vue-i18n'
import { readdirSync } from 'node:fs'

// Every directory of src but common is a module
const modules = readdirSync(new URL('./src', import.meta.url), { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && entry.name !== 'common')
    .map((entry) => entry.name)

// Configuration flat pour ESLint v10+
export default [
    // Configuration TypeScript
    {
        plugins: {
            vue,
            '@typescript-eslint': tsPlugin,
            'unused-imports': unusedImports
        },
        languageOptions: {
            globals: {
                ...globals.browser,
                ...globals.node,
                defineProps: 'readonly',
                defineEmits: 'readonly',
                defineExpose: 'readonly',
                withDefaults: 'readonly'
            }
        },
        rules: {
            ...tsPlugin.configs.recommended.rules,
            'no-console': 'off',
            'no-unused-vars': 'off',
            '@typescript-eslint/no-unused-vars': 'off',
            'unused-imports/no-unused-imports': 'error',
            'unused-imports/no-unused-vars': ['warn', { ignoreRestSiblings: true }]
        }
    },
    // Configuration Vue
    {
        files: ['**/*.vue'],
        languageOptions: {
            parser: vueParser,
            parserOptions: {
                parser: tsParser,
                sourceType: 'module',
                ecmaVersion: 'latest',
                vueFeatures: {
                    filter: true,
                    interpolationAsNonHTMLString: false
                }
            }
        },
        rules: {
            ...vue.configs['flat/recommended'].rules,
            'vue/multi-word-component-names': 'off',
            'vue/max-attributes-per-line': 'off'
        }
    },
    // No text written in a template: every text is a message of a catalog
    {
        files: ['src/**/*.vue'],
        plugins: { '@intlify/vue-i18n': vueI18n },
        rules: {
            '@intlify/vue-i18n/no-raw-text': [
                'error',
                {
                    ignorePattern: '^[\\s\\d.,:;!?()\\[\\]{}/\\\\|·•…—–+×✕©~%#*&<>=-]*$',
                    attributes: {
                        '/.+/': ['title', 'aria-label', 'placeholder', 'alt', 'label', 'message']
                    }
                }
            ]
        }
    },
    // Configuration TypeScript files
    {
        files: ['**/*.{ts,tsx}'],
        languageOptions: {
            parser: tsParser,
            parserOptions: {
                sourceType: 'module',
                ecmaVersion: 'latest'
            }
        },
        rules: {
            ...tsPlugin.configs.recommended.rules,
            'no-console': 'off',
            'no-unused-vars': 'off',
            '@typescript-eslint/no-unused-vars': 'off',
            'unused-imports/no-unused-imports': 'error',
            'unused-imports/no-unused-vars': ['warn', { ignoreRestSiblings: true }]
        }
    },
    // Common imports the platform (@myapplibs/web-platform) and itself, never a module
    {
        files: ['src/common/**/*.{ts,vue}'],
        rules: {
            'no-restricted-imports': [
                'error',
                {
                    patterns: [
                        {
                            group: ['@/*', '!@/common', '!@/common/*'],
                            message: 'src/common never imports a module'
                        }
                    ]
                }
            ]
        }
    },
    // A module imports the platform (@myapplibs/web-platform), common and itself, never another module
    ...modules.map((name) => ({
        files: [`src/${name}/**/*.{ts,vue}`],
        rules: {
            'no-restricted-imports': [
                'error',
                {
                    patterns: [
                        {
                            group: [
                                '@/*',
                                '!@/common',
                                '!@/common/*',
                                `!@/${name}`,
                                `!@/${name}/*`
                            ],
                            message: 'a module never imports another'
                        }
                    ]
                }
            ]
        }
    })),
    {
        ignores: ['**/dist/**', '**/dist-ssr/**', '**/coverage/**', '**/*.gen.ts']
    },
    prettierConfig
]
