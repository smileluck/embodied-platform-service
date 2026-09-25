// 标签栏持久化 key：AdminLayout 读写（上限 12，刷新恢复）；
// Login 登录成功后清空——新会话不继承旧标签（含换账号场景，避免恢复出他人权限下的页面）
export const TABS_STORAGE_KEY = 'sx-tabs'
