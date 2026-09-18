// about 模块文案（由对应页面抽取填充）
export default {
  updateLog: '更新记录',
  commitMeta: '{n} 条提交 · 构建时自动生成',
  intro: 'embodied-platform（具身智能设备基础设施平台）的业务管理端：设备、型号、租户与文件存储全部委外给平台，账号体系统一到平台，专注商户侧业务运营与权限管理。后端 Gin + GORM + Wire，前端 Vue 3 + Naive UI。',
  featuresTitle: '核心特性',
  features: {
    identity: '统一身份：平台 token 直接登录，profile 自省 + 本地准入投影，无本地密码/会话',
    openapi: '平台开放面：设备/租户/型号/物模型走商户 HMAC 单一凭证，scope 精细授权',
    tenant: '租户强一致同步：平台先行、本地跟随，失败整体回滚；应用用户为独立体系',
    storage: '文件走平台 storage-gateway：预签名上传/302 下载，平台未配置自动降级本地磁盘',
    rbac: '本地 RBAC：角色/权限/动态菜单/操作日志/IP 黑名单/导出保留在业务侧',
    arch: 'DDD 四层架构：依赖倒置，仓储接口在领域层，Wire 注入与 Kratos 同构',
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
