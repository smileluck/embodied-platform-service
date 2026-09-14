import request from './request'
import type { AppUser, BlacklistItem, CaptchaInfo, ExportRecord, FileInfo, LoginLogInfo, MenuHit, MenuNode, Merchant, MerchantAPILog, OnlineSession, OperationLogInfo, PageResult, Permission, R, Role, Tenant, TokenPair, UserInfo, LogPageResult } from './types'

// ---- 认证 ----
export const getCaptcha = () => request.get<R<CaptchaInfo>>('/auth/captcha')
export const login = (data: { username: string; password: string; captcha_id: string; captcha_code: string; device_type?: string }) =>
  request.post<R<TokenPair>>('/auth/login', data)
export const refreshToken = (refresh_token: string) =>
  request.post<R<TokenPair>>('/auth/refresh', { refresh_token })
export const logout = () => request.post<R<null>>('/auth/logout')
export const getProfile = () =>
  request.get<R<{ user: UserInfo; permissions: Permission[] }>>('/auth/profile')
export const getMenus = () => request.get<R<MenuNode[]>>('/menus')
// 菜单搜索（顶栏命令面板）：后端在当前用户可见菜单内模糊匹配，空关键词返回空
export const searchMenus = (kw: string) =>
  request.get<R<MenuHit[]>>('/menus/search', { params: { kw } })
// 本人更新昵称/邮箱，返回更新后的完整个人信息
export const updateProfile = (data: { nickname?: string; email?: string }) =>
  request.put<R<{ user: UserInfo; permissions: Permission[] }>>('/auth/profile', data)
// 本人修改密码（校验旧密码）；成功后需重新登录
export const changePassword = (data: { old_password: string; new_password: string }) =>
  request.put<R<null>>('/auth/password', data)

// ---- 用户 ----
export const listUsers = (params: { page: number; page_size: number; username?: string; status?: number }) =>
  request.get<R<PageResult<UserInfo>>>('/users', { params })
export const createUser = (data: Partial<UserInfo> & { password: string }) =>
  request.post<R<UserInfo>>('/users', data)
export const updateUser = (id: number, data: Partial<UserInfo>) =>
  request.put<R<null>>(`/users/${id}`, data)
export const deleteUser = (id: number) => request.delete<R<null>>(`/users/${id}`)
export const setUserRoles = (id: number, role_ids: number[]) =>
  request.put<R<null>>(`/users/${id}/roles`, { role_ids })
export const resetPassword = (id: number, password: string) =>
  request.put<R<null>>(`/users/${id}/password`, { password })

// ---- 角色 ----
export const listRoles = (params: { page: number; page_size: number; name?: string }) =>
  request.get<R<PageResult<Role>>>('/roles', { params })
export const createRole = (data: Partial<Role>) => request.post<R<Role>>('/roles', data)
export const getRole = (id: number) => request.get<R<Role>>(`/roles/${id}`)
export const updateRole = (id: number, data: Partial<Role>) =>
  request.put<R<null>>(`/roles/${id}`, data)
export const deleteRole = (id: number) => request.delete<R<null>>(`/roles/${id}`)
export const setRolePermissions = (id: number, permission_ids: number[]) =>
  request.put<R<null>>(`/roles/${id}/permissions`, { permission_ids })

// ---- 权限 / 菜单 ----
export const listPermissions = (params: { page: number; page_size: number; type?: string }) =>
  request.get<R<PageResult<Permission>>>('/permissions', { params })
// 全量权限点（page_size=0 不分页）：菜单管理树 / 角色分配权限树需整表构建，分页会静默截断
export const listAllPermissions = (params?: { type?: string }) =>
  request.get<R<PageResult<Permission>>>('/permissions', { params: { page_size: 0, ...params } })
export const createPermission = (data: Partial<Permission>) =>
  request.post<R<Permission>>('/permissions', data)
export const updatePermission = (id: number, data: Partial<Permission>) =>
  request.put<R<null>>(`/permissions/${id}`, data)
export const deletePermission = (id: number) => request.delete<R<null>>(`/permissions/${id}`)

// ---- 在线用户 ----
export const listOnlineUsers = (params: { page: number; page_size: number; username?: string; device?: string }) =>
  request.get<R<PageResult<OnlineSession>>>('/online-users', { params })
// 踢单个会话下线（某端）
export const kickOnlineSession = (sid: string) => request.delete<R<null>>(`/online-users/${sid}`)
// 踢某用户全部端下线
export const kickUserSessions = (userId: number) => request.delete<R<null>>(`/users/${userId}/sessions`)

// ---- 日志 ----
// start/end 为 unix 秒级时间戳
export const listLoginLogs = (params: { page: number; page_size: number; username?: string; ip?: string; status?: number; start?: number; end?: number }) =>
  request.get<R<LogPageResult<LoginLogInfo>>>('/login-logs', { params })
export const clearLoginLogs = () => request.delete<R<{ deleted: number }>>('/login-logs')
export const listOperationLogs = (params: { page: number; page_size: number; username?: string; method?: string; kw?: string; start?: number; end?: number }) =>
  request.get<R<LogPageResult<OperationLogInfo>>>('/operation-logs', { params })
export const clearOperationLogs = () => request.delete<R<{ deleted: number }>>('/operation-logs')

// ---- 商户（开放平台）----
export const listMerchants = (params: { page: number; page_size: number; name?: string; code?: string; app_key?: string; status?: number }) =>
  request.get<R<PageResult<Merchant>>>('/merchants', { params })
// 创建成功时 app_secret 仅此一次明文返回，前端需弹窗展示并提示保存
export const createMerchant = (data: { name: string; code: string; contact_name?: string; contact_phone?: string; contact_email?: string; remark?: string }) =>
  request.post<R<Merchant & { app_secret: string }>>('/merchants', data)
export const getMerchant = (id: number) => request.get<R<Merchant>>(`/merchants/${id}`)
// 联系人字段留空表示不修改（列表/详情返回的均为脱敏值，不可原样回传）
export const updateMerchant = (id: number, data: { name: string; contact_name?: string; contact_phone?: string; contact_email?: string; remark?: string }) =>
  request.put<R<null>>(`/merchants/${id}`, data)
export const deleteMerchant = (id: number) => request.delete<R<null>>(`/merchants/${id}`)
// 重置密钥：新 app_secret 仅此一次明文返回
export const resetMerchantSecret = (id: number) =>
  request.put<R<{ app_key: string; app_secret: string }>>(`/merchants/${id}/secret`)
export const setMerchantStatus = (id: number, status: number) =>
  request.put<R<null>>(`/merchants/${id}/status`, { status })
// start/end 为 unix 秒级时间戳
export const listMerchantAPILogs = (params: { page: number; page_size: number; app_key?: string; path?: string; status_code?: number; start?: number; end?: number }) =>
  request.get<R<PageResult<MerchantAPILog>>>('/merchant-api-logs', { params })

// ---- 租户 ----
export const listTenants = (params: { page: number; page_size: number; name?: string; code?: string; status?: number }) =>
  request.get<R<PageResult<Tenant>>>('/tenants', { params })
export const createTenant = (data: { name: string; code: string; contact_name?: string; contact_phone?: string; remark?: string }) =>
  request.post<R<Tenant>>('/tenants', data)
export const getTenant = (id: number) => request.get<R<Tenant>>(`/tenants/${id}`)
// code 创建后不可修改，更新入参不含 code
export const updateTenant = (id: number, data: { name: string; contact_name?: string; contact_phone?: string; remark?: string }) =>
  request.put<R<null>>(`/tenants/${id}`, data)
export const deleteTenant = (id: number) => request.delete<R<null>>(`/tenants/${id}`)
export const setTenantStatus = (id: number, status: number) =>
  request.put<R<null>>(`/tenants/${id}/status`, { status })

// ---- 应用用户 ----
// kw 模糊匹配用户名/昵称，phone 精确匹配，tenant_id 按租户筛选
export const listAppUsers = (params: { page: number; page_size: number; kw?: string; phone?: string; status?: number; tenant_id?: number }) =>
  request.get<R<PageResult<AppUser>>>('/app-users', { params })
export const createAppUser = (data: { username: string; password: string; nickname?: string; phone?: string; email?: string; tenant_ids?: number[] }) =>
  request.post<R<AppUser>>('/app-users', data)
export const getAppUser = (id: number) => request.get<R<AppUser>>(`/app-users/${id}`)
// username 创建后不可修改；tenant_ids 全量替换；status 可选（状态切换也走此接口）
export const updateAppUser = (id: number, data: { nickname?: string; phone?: string; email?: string; status?: number; tenant_ids?: number[] }) =>
  request.put<R<null>>(`/app-users/${id}`, data)
export const deleteAppUser = (id: number) => request.delete<R<null>>(`/app-users/${id}`)
export const setAppUserStatus = (id: number, status: number) =>
  request.put<R<null>>(`/app-users/${id}`, { status })
// 重置密码（新密码由管理员指定，旧密码立即失效）
export const resetAppUserPassword = (id: number, password: string) =>
  request.put<R<null>>(`/app-users/${id}/password`, { password })

// ---- IP 黑名单 ----
export const listBlacklist = (params: { page: number; page_size: number; ip?: string }) =>
  request.get<R<PageResult<BlacklistItem>>>('/ip-blacklist', { params })
export const createBlacklist = (data: { ip: string; reason?: string; expire_at?: number | null }) =>
  request.post<R<BlacklistItem>>('/ip-blacklist', data)
export const deleteBlacklist = (id: number) => request.delete<R<null>>(`/ip-blacklist/${id}`)

// ---- 文件 ----
export const listFiles = (params: { page: number; page_size: number; name?: string }) =>
  request.get<R<PageResult<FileInfo>>>('/files', { params })
// 大文件上传：覆写默认 15s 超时
export const uploadFile = (file: File) => {
  const fd = new FormData()
  fd.append('file', file)
  return request.post<R<FileInfo>>('/files', fd, { timeout: 0 })
}
export const deleteFile = (id: number) => request.delete<R<null>>(`/files/${id}`)
// 下载/预览均走鉴权接口：云存储会 302 到预签名 URL（axios 自动跟随），统一按 blob 取回
export const getFileBlob = (id: number, download = false) =>
  request.get<Blob>(`/files/${id}/raw`, { responseType: 'blob', timeout: 0, params: download ? { download: 1 } : {} })

// ---- 异步导出 ----
// 提交导出任务：params 为当前列表过滤条件（剔除 page/page_size 与空值）；429 表示队列满
export const createExport = (biz: 'users' | 'login-logs' | 'operation-logs', params: Record<string, any>) => {
  const query: Record<string, any> = {}
  for (const [k, v] of Object.entries(params)) {
    if (k === 'page' || k === 'page_size') continue
    if (v === undefined || v === null || v === '') continue
    query[k] = v
  }
  return request.post<R<ExportRecord>>(`/${biz}/export`, null, { params: query })
}
// 近期 5 条导出记录（顶栏悬浮框）
export const listRecentExports = () => request.get<R<ExportRecord[]>>('/exports', { params: { recent: 1 } })
// 本人导出记录分页
export const listExportRecords = (params: { page: number; page_size: number }) =>
  request.get<R<PageResult<ExportRecord>>>('/exports', { params })
// 导出文件下载：大文件覆写默认 15s 超时；409 未完成 / 403 非本人
export const getExportBlob = (id: number) =>
  request.get<Blob>(`/exports/${id}/download`, { responseType: 'blob', timeout: 0 })
export const deleteExport = (id: number) => request.delete<R<null>>(`/exports/${id}`)
