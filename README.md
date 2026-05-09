# Evalux Server
author: jiaqiangguo@foxmail.com

面向移动应用的用户体验智能评估系统 — 后端服务

## 技术栈

- **语言**: Go 1.25
- **Web 框架**: Gin
- **ORM**: ent (entgo.io)
- **数据库**: PostgreSQL
- **认证**: JWT (golang-jwt)
- **对象存储**: Cloudreve

## 项目结构

```
evalux-server/
├── cmd/server/         # 应用入口
│   └── main.go
├── ent/                # ORM 层
│   ├── schema/         # 数据模型定义 (手写)
│   └── ...             # 自动生成的 CRUD 代码
├── internal/
│   ├── config/         # 配置加载
│   ├── db/             # 数据库连接与种子数据
│   ├── handler/        # HTTP 请求处理器
│   ├── llm/            # 大模型 API 客户端
│   ├── middleware/     # 中间件 (JWT, CORS)
│   ├── model/          # DTO 数据传输对象
│   ├── repo/           # 数据访问层
│   ├── response/       # 统一响应封装
│   ├── router/         # 路由注册与依赖注入
│   ├── service/        # 业务逻辑层
│   └── storage/        # 对象存储客户端
└── go.mod
```

## 开发

### 环境要求

- Go >= 1.25
- PostgreSQL >= 14

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 服务监听端口 |
| `DATABASE_URL` | `postgres://postgres:password@127.0.0.1:5432/evalux?sslmode=disable` | 数据库连接串 |
| `JWT_SECRET` | `evalux-dev-secret-key-change-in-production` | JWT 签名密钥 |
| `ADMIN_ACCOUNT` | `admin` | 默认管理员账号 |
| `ADMIN_PASSWORD` | `admin123456` | 默认管理员密码 |
| `CLOUDREVE_BASE_URL` | `http://127.0.0.1:5212` | Cloudreve 地址 |

### 运行

```bash
# 生成 ent 代码 (修改 schema 后)
go generate ./ent

# 启动服务
go run ./cmd/server
```

### 构建

```bash
go build -o evalux-server ./cmd/server
```

## 许可证

本项目基于 [GNU General Public License v2.0](./LICENSE) 许可证发布。
