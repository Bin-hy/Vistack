# UI 次世代重构 Checklist

> 每一项通过运行或观察行为来验证，聚焦系统行为。

## 实现完整性
- [ ] 两端深色 token 已定义且值一致（验证：`web/web-client/src/style.css` 与 `web/web-admin/src/style.css` 的 `:root` 变量值一致，对应 AC1/AC2）
- [ ] `web-admin/style.css` 已删除 Vite 脚手架残留（验证：无 `h1 { font-size: 3.2em }`、全局 `button`、`#app { max-width }`、`@media prefers-color-scheme`）
- [ ] 玻璃工具类 `.glass`/`.glass-strong`/`.gradient-text` 已存在并被使用

## 集成
- [ ] `BiliLayout` 导航链接全部指向真实路由，无 `to="/"` 死链（验证：grep 无 `to="/"` 指向占位，对应 AC3）
- [ ] emoji 图标（🔔📜⭐🕘）与 PNG（点赞/收藏/转发）已替换为 lucide 图标（验证：grep 无对应 emoji/PNG 引用，对应 AC3）
- [ ] 全站无原生 `alert(`/`confirm(`/`prompt(`（验证：grep 无命中，对应 AC4）
- [ ] Toast 与确认对话框已接入 Creator（登录提醒/删除确认/上传错误）并可用（对应 AC4/AC6）

## 编译与测试
- [ ] `pnpm --filter web-client build` 通过（vue-tsc + vite）
- [ ] `pnpm --filter web-admin build` 通过
- [ ] `pnpm -r build` 全量构建通过

## 一致性
- [ ] 无硬编码颜色类名（验证：grep `blue-`、`pink-`、`gray-`、`red-` 等应无命中或仅存在于被 token 替代处，对应 AC2）
- [ ] 两端视觉统一（深色玻璃、蓝紫渐变主色，无浅色残留，对应 AC1）

## 端到端场景
- [ ] 场景 1：`pnpm dev` 启动两端 → 首页深色玻璃卡片流正常渲染，骨架屏/空态正确 → 登录 → 播放页 DASH 可播放、点赞/收藏有选中态反馈（对应 AC6）
- [ ] 场景 2：创作者中心上传视频 → 进度条正常 → 转码状态展示 → 删除视频走自定义确认框（非原生 confirm）→ Toast 提示结果（对应 AC4/AC6）
- [ ] 场景 3：窄屏（移动端宽度）下导航收起为抽屉菜单，列表/播放页无溢出错位（对应 AC7）
