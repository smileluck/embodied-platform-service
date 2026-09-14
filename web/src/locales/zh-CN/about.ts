// about 模块文案（由对应页面抽取填充）
export default {
  updateLog: '更新记录',
  commitMeta: '{n} 条提交 · 构建时自动生成',
  intro: 'DDD 驱动的全栈后台管理系统：后端 Gin + GORM + Wire，前端 Vue 3 + Naive UI，支持多数据库与 RBAC 权限体系，架构预留微服务演进能力。',
  featuresTitle: '核心特性',
  features: {
    arch: 'DDD 四层架构：依赖倒置，仓储接口在领域层，Wire 注入与 Kratos 同构',
    db: '多数据库：MySQL / PostgreSQL / SQLite 配置一行切换，自动建表与种子数据',
    rbac: 'RBAC + JWT：用户-角色-权限三级模型，双令牌与按钮级权限控制',
    menu: '动态菜单路由：菜单后台配置、运行时注册，改菜单不改代码',
    token: 'Token 静默续期：401 自动刷新并重放请求，用户无感',
    i18n: '中英文国际化：界面文案与接口提示按语言自动切换',
  },
  type: {
    feat: '新增',
    fix: '修复',
    style: '样式',
    refactor: '重构',
    perf: '性能',
    docs: '文档',
    test: '测试',
    chore: '杂项',
    other: '其他',
  },
}
