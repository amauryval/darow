import vue from 'eslint-plugin-vue'
import tsPlugin from '@typescript-eslint/eslint-plugin'
import tsParser from '@typescript-eslint/parser'
import vueParser from 'vue-eslint-parser'
import unusedImports from 'eslint-plugin-unused-imports'
import prettierConfig from 'eslint-config-prettier'
import globals from 'globals'
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
            'unused-imports/no-unused-vars': 'warn'
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
            'unused-imports/no-unused-vars': 'warn'
        }
    },
    {
        ignores: ['**/dist/**', '**/dist-ssr/**', '**/coverage/**']
    },
    prettierConfig
]
