<h1 align="center">
  <img src="./web/public/logo.png" alt="Vistack" width="200">
  <br>Vistack<br>
</h1>

# 🎥 Vistack

**Vistack** 是一个高性能、分布式的视频平台，支持 **实时直播（Live Streaming）** 和 **视频点播（VOD）**。  
项目专注于 **高并发、高可扩展性** 的设计实践，采用现代云原生技术栈构建。

在线体验：<https://vistack.huai-xhy.site>

---

## 🚀 项目特点

- **直播模块**  
  - 使用 **OBS** 作为客户端采集与编码（H.264/AAC）  
  - **live777 (Rust SFU)** 作为流分发服务器，支持多用户拉流观看  
  - 服务器不做转码，轻量高效，高并发能力强  

- **视频点播模块**  
  - 使用 **DASH**（`.mpd + .m4s`）作为视频分片方案，支持自适应码率播放  
  - **Go 后端** 管理视频元信息、生成上传和播放签名 URL  
  - **MinIO** 存储原视频和转码后的 DASH 分片  
  - 前端使用 **dash.js / Video.js** 播放 DASH 视频  

- **分布式与可扩展**  
  - 异步转码服务（Go Worker + FFmpeg）  
  - 消息队列（NATS / RabbitMQ）分发转码任务  
  - 支持水平扩展、容器化部署（Docker / Kubernetes）  

- **安全与防盗链**  
  - 直播推流密钥验证  
  - DASH 播放使用预签名 URL 或 JWT 鉴权  

---

## 🧩 系统架构概览


---

## ⚙️ 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | Vue3 + Pinia + dash.js / Video.js |
| 后端 API | Go + Gin/Fiber + PostgreSQL + Redis |
| 直播服务 | live777 (Rust SFU) |
| 对象存储 | MinIO (S3 兼容) |
| 转码 | FFmpeg + Go Worker |
| 消息队列 | NATS / RabbitMQ |
| 代理 & CDN | Nginx / 可选 CDN |
| 部署 | Docker Compose / Kubernetes |
| 监控 | Prometheus + Grafana |

---

## � 界面展示

- 首页：展示推荐内容，提供一键进入创作中心的入口。
- 创作中心：参考 Bilibili 风格的视频投稿界面，支持分片上传、进度展示、创作空间视频列表。
- 播放页：自适应码率 DASH 播放器，支持清晰度切换、弹幕、UP 主信息展示，预留点赞 / 收藏 / 转发等交互按钮。
- 个人中心：展示用户基础信息（昵称、头像）及相关视频信息。

展示网站：<https://vistack.huai-xhy.site>

---

## �🔹 核心流程

### 直播

1. OBS 客户端采集 + 转码 → 推流到 live777  
2. live777 负责流转发 → WebRTC 前端观看  
3. 可选同步录制 → 存储到 MinIO，支持回放  

### 点播 (VOD)

1. 用户上传视频 → Go API 生成 MinIO 上传签名 URL  
2. 转码服务使用 FFmpeg 生成 DASH 分片 → 上传到 MinIO  
3. 前端通过 dash.js / Video.js 播放 DASH 视频  
4. 可生成 HLS 备份兼容 iOS Safari  

---

## 🧭 项目目标

- 掌握 **Go 分布式架构设计** 和 **高并发任务调度**  
- 学习 **Rust SFU (live777) 实时流媒体分发**  
- 实践 **云原生部署与可观测性**（Docker + Prometheus + Grafana）  
- 实现 **直播 + 点播一体化视频平台**  

---

# 快速启动

## docker 启动

```bash
cp .env .env.local
docker-compose up -d
```

## server

- `conf/app.toml` 是 `server` 配置文件，

```bash
go mod tidy
go run .
```

## web

- `web/` 是 `web` 前端项目目录，

```bash
pnpm install
pnpm run dev
```


# 数据库表结构

| 模块                | 表名                 | 说明                                    |
| ----------------- | ------------------ | ------------------------------------- |
| 🧍‍♂️ 用户系统 & 权限管理 | `users`            | 用户基本信息，包含登录凭证、邮箱、状态、关联角色等             |
|                   | `user_profiles`    | 用户扩展资料（昵称、头像等）                         |
|                   | `roles`            | 角色表，用于定义系统角色（如管理员、普通用户、审核员等）          |
|                   | `authorities`      | 权限表，存储资源方法（GET/POST 等）与资源 URI（REST 接口路径） |
|                   | `role_authority`   | 角色与权限的多对多关联，实现 RBAC（基于角色的访问控制）        |
|                   | `user_authority`   | 用户级别的特定权限授权/禁用，实现更细粒度的权限控制            |
| 📁 文件管理           | `files`            | 通用文件表，存储 MinIO 桶名、对象 Key、引用类型（头像/视频/封面等）、大小等 |
| 🎬 视频管理           | `videos`           | 视频主表，存储视频标题、描述、封面文件 ID、时长、状态、可见性等      |
|                   | `video_sources`    | 原始视频上传记录（关联 videos 与 files）              |
|                   | `video_transcodes` | 转码任务与状态，记录转码进度、分辨率、编码格式、清单文件 ID        |
|                   | `video_manifest`   | 播放清单表，记录 DASH/HLS 清单文件、协议类型、码率档位 profiles 等 |
| 🏷️ 标签系统          | `tags`             | 标签表，用于管理视频分类、话题等标签                    |
|                   | `video_tags`       | 视频与标签的关联关系，多对多映射                      |
| 💬 社交互动           | `video_comments`   | 视频评论表，支持父子评论结构                         |
|                   | `video_likes`      | 视频点赞关系表（视频 ID + 用户 ID 复合主键）          |
|                   | `video_favorites`  | 视频收藏关系表（视频 ID + 用户 ID 复合主键）          |
| 📈 播放统计           | `video_play_logs`  | 视频播放日志，记录播放用户、时间、IP、User-Agent 等信息    |
