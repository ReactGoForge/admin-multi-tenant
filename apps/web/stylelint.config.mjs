const tailwindAtRules = [
  'apply',
  'config',
  'custom-variant',
  'plugin',
  'reference',
  'source',
  'theme',
  'utility',
  'variant',
]

/** @type {import('stylelint').Config} */
export default {
  extends: [
    'stylelint-config-standard-scss',
    'stylelint-config-recess-order',
  ],
  rules: {
    'at-rule-no-unknown': [true, { ignoreAtRules: tailwindAtRules }],
    'import-notation': null,
    'no-descending-specificity': null,
    'no-empty-source': null,
    'selector-class-pattern': null,
    'selector-pseudo-class-no-unknown': [true, { ignorePseudoClasses: ['global'] }],
    'scss/at-rule-no-unknown': [true, { ignoreAtRules: tailwindAtRules }],
  },
}
