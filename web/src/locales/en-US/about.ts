// about 模块文案（由对应页面抽取填充）
export default {
  updateLog: 'Changelog',
  commitMeta: '{n} commits · auto-generated at build time',
  intro: 'A DDD-driven full-stack admin system: Gin + GORM + Wire on the backend and Vue 3 + Naive UI on the frontend, with multi-database support and RBAC, designed to evolve toward microservices.',
  featuresTitle: 'Key Features',
  features: {
    arch: 'DDD 4-layer architecture: dependency inversion, repository interfaces in the domain layer, Wire DI isomorphic to Kratos',
    db: 'Multi-database: switch MySQL / PostgreSQL / SQLite with one config line, auto migration and seed data',
    rbac: 'RBAC + JWT: user-role-permission model with dual tokens and button-level access control',
    menu: 'Dynamic menu routes: menus configured in admin and registered at runtime — no code changes',
    token: 'Silent token renewal: 401 auto-refresh with request replay, seamless to users',
    i18n: 'Chinese/English i18n: UI text and API messages switch with the language',
  },
  type: {
    feat: 'Feature',
    fix: 'Fix',
    style: 'Style',
    refactor: 'Refactor',
    perf: 'Perf',
    docs: 'Docs',
    test: 'Test',
    chore: 'Chore',
    other: 'Other',
  },
}
