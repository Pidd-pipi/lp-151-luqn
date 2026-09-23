# 匿名树洞社区（gbtreehole）

一个全栈匿名树洞社区平台。用户无需注册即可获得随机昵称与唯一身份密钥，在不暴露真实身份的前提下倾诉吐槽、互动交流；系统提供话题标签、点赞热度排行、每日精选、敏感词过滤与后台审核等能力。

## 功能列表

- 匿名发帖与评论：随机昵称 + 唯一身份密钥，凭密钥管理自己的内容，图文帖 + 匿名评论
- 话题标签分类：按标签浏览，用户发帖可选已有标签或创建新标签，管理员维护标签库
- 点赞与热度排行：帖子/评论匿名点赞，首页支持「最新帖子」与「热度排行」双视图
- 每日精选推送：自动按点赞和互动量筛选，首页置顶展示，管理员可手动推荐
- 敏感词过滤与审核：内置敏感词库，命中内容进入审核队列，管理员可放行或永久屏蔽
- 匿名身份管理：可创建多个匿名身份，本地切换使用

## 技术栈

| 层次 | 技术 |
| ---- | ---- |
| 后端 | Go 1.22 + Gin + GORM |
| 数据库 | MySQL 8.0 |
| 缓存 | Redis（热门帖子 / 会话） |
| 前端 | React 18 + TypeScript + Ant Design 5 + Vite |
| 认证 | 身份密钥 + JWT（golang-jwt/v5） |
| 部署 | Docker Compose + Nginx |

## 目录结构

```
.
├── backend/                # Go 后端
│   ├── cmd/server/         # 程序入口
│   ├── internal/
│   │   ├── config/         # 环境变量配置
│   │   ├── model/          # GORM 模型
│   │   ├── repository/     # 数据访问层
│   │   ├── service/        # 业务逻辑层
│   │   ├── handler/        # HTTP 处理层
│   │   ├── router/         # 路由注册
│   │   ├── middleware/     # 中间件
│   │   ├── dto/            # 请求/响应结构体
│   │   └── constants/      # 错误码/状态/Redis key
│   ├── migrations/         # 迁移脚本
│   ├── api/openapi.yaml    # OpenAPI 文档
│   ├── deploy/             # 部署说明
│   └── Dockerfile
├── frontend/               # React 前端
├── database/init.sql       # MySQL 初始化数据
├── docker-compose.yml
└── .env.example
```

## 快速启动（推荐 Docker Compose）

```bash
cp .env.example .env
# 按需修改 .env 中的端口和密钥
docker compose up -d --build
```

启动后：

- 前端：http://localhost:18401
- 后端 API：http://localhost:19401
- 健康检查：http://localhost:19401/healthz
- OpenAPI：http://localhost:19401/api/openapi.yaml

## 本地开发

后端：

```bash
cd backend
go mod tidy
go run ./cmd/server
go build ./...
go test ./...
```

后端依赖本地 MySQL / Redis，可用环境变量覆盖连接信息：

```bash
MYSQL_HOST=127.0.0.1 MYSQL_PORT=3306 MYSQL_USER=root MYSQL_PASSWORD=... \
MYSQL_DATABASE=gbtreehole REDIS_ADDR=127.0.0.1:6379 \
JWT_SECRET=dev-secret go run ./cmd/server
```

前端：

```bash
cd frontend
npm install
npm run dev
```

开发环境前端 `/api` 会代理到 `http://localhost:19401`。

## 环境变量

| 变量 | 默认值 | 说明 |
| ---- | ------ | ---- |
| COMPOSE_PROJECT_NAME | gbtreehole | Compose 项目名 |
| FRONTEND_PORT | 18401 | 前端对外端口 |
| BACKEND_PORT | 19401 | 后端对外端口 |
| MYSQL_PORT | 57401 | MySQL 对外端口 |
| REDIS_PORT | 57411 | Redis 对外端口 |
| MYSQL_ROOT_PASSWORD | root123456 | MySQL root 密码 |
| MYSQL_DATABASE | gbtreehole | 数据库名 |
| MYSQL_USER | gbtreehole | 数据库用户 |
| MYSQL_PASSWORD | gbtreehole123 | 数据库密码 |
| JWT_SECRET | dev secret | JWT 签名密钥，生产必须修改 |
| JWT_EXPIRE_MIN | 10080 | token 有效期（分钟） |

## 核心 API 清单

统一前缀 `/api/v1`，响应格式 `{ "code": 0, "message": "ok", "data": ... }`。

| 方法 | 路径 | 说明 |
| ---- | ---- | ---- |
| POST | /auth/identities | 创建匿名身份，返回身份 + JWT |
| POST | /auth/login | 使用身份密钥登录换 JWT |
| GET | /posts | 帖子列表（page/page_size/tag_id/featured） |
| POST | /posts | 发布帖子（需登录） |
| GET | /posts/hot | 热门帖子 |
| GET | /posts/featured | 每日精选 |
| GET | /posts/{id} | 帖子详情 |
| GET | /posts/{id}/comments | 帖子评论 |
| POST | /comments | 发表评论（需登录） |
| GET | /tags | 标签列表 |
| POST | /tags | 创建标签（需登录） |
| POST | /likes/toggle | 点赞/取消点赞（需登录） |
| GET | /admin/reviews | 审核队列 |
| POST | /admin/reviews/action | 审核放行/屏蔽 |
| GET | /admin/sensitive-words | 敏感词列表 |
| POST | /admin/sensitive-words | 新增敏感词 |
| DELETE | /admin/sensitive-words/{id} | 删除敏感词 |
| POST | /admin/feature | 手动推荐/取消推荐 |
| GET | /admin/tags | 标签维护 |

详细 OpenAPI 文档见 `backend/api/openapi.yaml`。
