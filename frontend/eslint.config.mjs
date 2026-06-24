// @ts-check
import withNuxt from './.nuxt/eslint.config.mjs'

export default withNuxt(
  {
    files: ['app/components/Can.vue'],
    rules: {
      'vue/multi-word-component-names': 'off'
    }
  }
)
