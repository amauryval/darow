import eslint from "@eslint/js";
import eslintPluginVue from "eslint-plugin-vue";
import globals from "globals";
import tseslint from "typescript-eslint";

// Correctness rules only ("essential") — no Vue formatting/style rules,
// since this project has no Prettier and doesn't format templates that way.
export default tseslint.config(
  { ignores: ["dist", "node_modules"] },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  ...eslintPluginVue.configs["flat/essential"],
  {
    languageOptions: {
      globals: { ...globals.browser, ...globals.node },
    },
  },
  {
    files: ["**/*.vue"],
    languageOptions: {
      parserOptions: { parser: tseslint.parser },
    },
  },
  {
    rules: {
      // TypeScript already reports references to undeclared identifiers;
      // no-undef only adds noise for DOM globals (HTMLDivElement, etc.).
      "no-undef": "off",
      "vue/multi-word-component-names": "off",
    },
  },
);
