# SmileX-Admin-Gin 项目初始化计划（MySQL 默认 + 超管 admin/123456）

Go Web 管理系统：Gin + GORM + google/wire，Kratos 兼容布局（biz/data/service/server），多数据库（**MySQL 默认**，可切 PostgreSQL / SQLite），RBAC + JWT + 用户管理。未来切 Kratos 时业务层零改动。

## 一、目录结构

```
SmileX-Admin-Gin/
├── cmd/server/main.go        # 入口：wire 注入，启动 HTTP
├── api/                      # proto 契约（首期手写 .proto，不生成代码）
│   ├── auth/v1/auth.proto
│   └── admin/v1/             # user/role/permission
├── configs/config.yaml       # db.driver 默认 mysql
├── internal/
│   ├── biz/                  # 领域层（纯 Go），四限界上下文
│   │   ├── auth/  user/  role/  permission/
│   │   │   ├── entity.go     # 实体 + 值对象（bcrypt 密码、状态）
│   │   │   ├── repo.go       # 仓储接口（data 层实现）
│   │   │   └── usecase.go    # Usecase
│   ├── data/
│   │   ├── data.go           # 多数据库工厂（mysql/pg/sqlite）+ wire Provider
│   │   ├── model/            # GORM PO + assembler
│   │   ├── auth/  user/  role/  permission/   # 仓储实现
│   │   └── jwt.go
│   ├── service/              # 应用层：薄用例（DTO 对应 proto 契约）
│   │   ├── auth/  user/  role/  permission/
│   ├── server/               # 传输层（唯一 Kratos 不兼容层）
│   │   ├── http.go           # Gin server + 路由
│   │   └── middleware/       # JWT、CORS、RBAC、日志
│   └── conf/config.go        # viper + wire Provider
├── pkg/
│   ├── eventbus/  response/  logger/  pagination/
├── migrations/               # MySQL/PG/SQLite 三方言建表 SQL + 种子数据
├── Makefile  go.mod  README.md（含 Kratos 升级路径）
```

依赖方向 `server → service → biz ← data`，wire 装配；上下文间禁止互 import data/service。

## 二、多数据库与种子数据

- `configs/config.yaml`：`db.driver: mysql`（默认），mysql/postgres/sqlite 各自配置段；工厂按 driver 建 `*gorm.DB`，连接池 + 慢 SQL 日志
- **种子数据：超级管理员 `admin / 123456`**（bcrypt 加密存储）+ 超管角色（拥有全部权限）+ 基础权限点；README 标注上线前修改默认密码
- PO 用 GORM 通用类型；migrations 三方言；SQLite 供本地测试

## 三、首期功能

1. 认证：登录（admin/123456 可直接登录）、JWT access+refresh、刷新、登出
2. 用户管理：CRUD、启用/禁用、分配角色、重置密码、分页
3. 角色管理：CRUD、绑定权限
4. 权限管理：API 权限 + 菜单标识 CRUD
5. 中间件：JWT 认证 → 自研轻量 RBAC（内存缓存）
6. 启动时自动执行 migrate + 种子数据（可配置关闭）

## 四、API（/api/v1）

`POST /auth/login|refresh|logout`、`GET /auth/profile`；`/users` `/roles` `/permissions` CRUD + 关联接口。

## 五、实施步骤

1. go mod init + 依赖（gin、gorm 三 driver、golang-jwt、viper、bcrypt、zap、wire）
2. conf + data 多数据库工厂 + pkg 基础工具
3. biz 四上下文 → data 仓储实现 + JWT → service 用例
4. server：Gin 路由 + 中间件，wire 装配
5. api proto 契约 + migrations + 种子数据（含 admin/123456）
6. Makefile + README
7. `go build` + MySQL 启动冒烟测试（登录 → CRUD → RBAC 拦截）

## 六、验证

`go build ./...` 通过；MySQL 启动自动建表+种子；curl 冒烟：admin/123456 登录拿 token → 创建用户/角色 → 未授权接口被拦截。