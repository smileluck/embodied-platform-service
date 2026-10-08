<!-- last-updated: 2026-10-03 -->
# 示例：前端 API 封装（types + 函数）

> 目的：展示 `web/src/api/` 的组织标准——类型唯一真源 + 请求函数封装。

## 核心原则

1. 服务端类型只在 `web/src/api/types.ts` 声明，字段 snake_case 与后端 json tag 逐字一致
2. 函数在 `web/src/api/index.ts`，动词开头、返回 `request.get<R<...>>` 泛型；禁止页面裸调
3. 统一信封 `R<T>` 与分页 `PageResult<T>` 复用既有泛型，不自造

## 示例（提取自 web/src/api/types.ts 与 web/src/api/index.ts，节选）

```ts
// types.ts —— 信封与分页（全局唯一声明）
export interface R<T = any> {
  code: number
  msg: string
  data: T
}

export interface PageResult<T> {
  list: T[]
  page: { page: number; page_size: number; total: number }
}

// index.ts —— dict 封装函数
export const listDictTypes = (params: { page: number; page_size: number; name?: string; code?: string }) =>
  request.get<R<PageResult<DictType>>>('/dict-types', { params })
export const createDictType = (data: Partial<DictType>) => request.post<R<DictType>>('/dict-types', data)
export const updateDictType = (id: number, data: Partial<DictType>) => request.put<R<null>>(`/dict-types/${id}`, data)
export const deleteDictType = (id: number) => request.delete<R<null>>(`/dict-types/${id}`)
```

## 要点

- 可选查询参数传 `undefined` 时 axios 自动省略（`name: query.name || undefined`）
- 管理端登录/刷新/登出也在本文件（`/auth/*`，经本服务代理平台）；仅租户端 app-auth 直调平台（`web/src/api/platform.ts`，fetch，token 双用）
- 401 刷新重放、错误 toast 兜底已由 `request.ts` 拦截器统一处理，函数内不重复处理

## 真实参考文件

- `web/src/api/types.ts`、`web/src/api/index.ts`
- `web/src/api/request.ts`（拦截器）、`web/src/api/platform.ts`（平台直调）
