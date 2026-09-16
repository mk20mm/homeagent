export default {
  extends: ['stylelint-config-standard'],
  rules: {
    // CSS Modules 用驼峰类名，关闭 kebab-case 检查
    'selector-class-pattern': null,
    'no-descending-specificity': null,
  },
}
