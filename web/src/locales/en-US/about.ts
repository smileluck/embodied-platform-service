// about 模块文案（由对应页面抽取填充）
export default {
  updateLog: 'Changelog',
  commitMeta: '{n} commits · auto-generated at build time',
  intro: 'The business console of embodied-platform (embodied-intelligence device infrastructure): devices, models, tenants and file storage are all delegated to the platform with a unified account system, so the console focuses on merchant-side operations and access control. Backend Gin + GORM + Wire, frontend Vue 3 + Naive UI.',
  featuresTitle: 'Key Features',
  features: {
    identity: 'Unified identity: log in with the platform token, profile introspection + local admission projection — no local passwords or sessions',
    openapi: 'Platform open API: devices/tenants/models/thing-models via one merchant HMAC credential with fine-grained scopes',
    tenant: 'Strongly consistent tenants: platform first, local follows, rollback on failure; app users as a separate system',
    storage: 'Files via platform storage-gateway: presigned upload / 302 download, automatic local-disk fallback when unconfigured',
    rbac: 'Local RBAC: roles/permissions/dynamic menus/audit logs/IP blacklist/export kept on the console side',
    arch: 'DDD 4-layer architecture: dependency inversion, repository interfaces in the domain layer, Wire DI isomorphic to Kratos',
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
