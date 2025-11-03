<h1 align="center">
  <img src="./web/public/logo.png" alt="Vistack" width="200">
  <br>Vistack<br>
</h1>

# 🎥 Vistack

**Vistack** 是一个高性能、分布式的视频平台，支持 **实时直播（Live Streaming）** 和 **视频点播（VOD）**。  
项目专注于 **高并发、高可扩展性** 的设计实践，采用现代云原生技术栈构建。

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

## 🔹 核心流程

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