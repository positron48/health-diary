import js from '@eslint/js'
import vue from 'eslint-plugin-vue'
import tseslint from 'typescript-eslint'
export default tseslint.config(
  { ignores: ['dist/**','coverage/**','public/sw.js'] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs['flat/recommended'],
  { files: ['**/*.vue'], languageOptions: { parserOptions: { parser: tseslint.parser, extraFileExtensions: ['.vue'] } } },
  { languageOptions: { globals: { window:'readonly',document:'readonly',navigator:'readonly',location:'readonly',Element:'readonly',HTMLElement:'readonly',KeyboardEvent:'readonly',Response:'readonly',fetch:'readonly',setTimeout:'readonly',setInterval:'readonly',clearInterval:'readonly' } }, rules: { 'vue/multi-word-component-names': 'off', 'vue/max-attributes-per-line': 'off', 'vue/singleline-html-element-content-newline': 'off', 'vue/html-self-closing': 'off', 'vue/mustache-interpolation-spacing':'off', 'vue/html-closing-bracket-spacing':'off', '@typescript-eslint/no-explicit-any': 'off' } },
)
