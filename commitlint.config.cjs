// Conventional Commits 规则（配合 .github/workflows/ci.yml 的 commitlint 任务）。
// 仓库提交信息可用中文正文，类型用英文：feat/fix/docs/refactor/test/chore/...
module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // 中文正文较长，放宽首行长度上限。
    'header-max-length': [2, 'always', 120],
    // 正文为中文时不强制 subject 大小写。
    'subject-case': [0],
  },
}
