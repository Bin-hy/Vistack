# UI 次世代重构 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `web/web-client/src/style.css` | 深色 token + 玻璃工具类 |
| 修改 | `web/web-client/tailwind.config.js` | glass 色 + 动效 |
| 修改 | `web/web-client/src/App.vue` | 挂载 Toast/Confirm 宿主 |
| 修改 | `web/web-client/src/layouts/BiliLayout.vue` | 玻璃导航 + 图标 |
| 修改 | `web/web-client/src/components/ui/button.vue`、`card.vue`、`input.vue` | 玻璃/渐变 |
| 新增 | `web/web-client/src/components/ui/icon.vue` | lucide 统一封装 |
| 新增 | `web/web-client/src/components/ui/toast/{useToast.ts,ToastViewport.vue}` | Toast |
| 新增 | `web/web-client/src/components/ui/dialog/{useConfirm.ts,ConfirmDialog.vue}` | 确认框 |
| 修改 | `web/web-client/src/components/ui/index.ts` | 导出新组件 |
| 修改 | `web/web-client/src/components/video/VideoCard.vue`、`creator/VideoCard.vue` | 玻璃卡 |
| 修改 | `web/web-client/src/views/{Index,VideoPlayer,Creator,AuthLogin,AuthRegister,Profile}/index.vue` | 套用主题 |
| 修改 | `web/web-admin/src/style.css` | 深色 token + 删 Vite 残留 |
| 修改 | `web/web-admin/tailwind.config.js` | 同 web-client |
| 修改 | `web/web-admin/src/App.vue`、`layouts/BiliLayout.vue` | 主题 + Toast |
| 修改 | `web/web-admin/src/components/ui/{button.vue,index.ts}` | 玻璃/渐变 |
| 修改 | `web/web-admin/src/views/{Index,AuthLogin,AuthRegister}/index.vue` | 套用主题 |
| 修改 | `package.json` | 新增 lucide-vue-next |

## T1: web-client 设计 token

**文件：** `web/web-client/src/style.css`、`web/web-client/tailwind.config.js`
**依赖：** 无
**步骤：**
1. `style.css` 重写 `:root` 为深色 token（background/foreground/primary/gradient-from/to/secondary/accent/muted/border/input/ring/destructive/glass），加 `color-scheme: dark`
2. 新增 `@layer components` 的 `.glass`/`.glass-strong`/`.gradient-text`
3. `body` 加深色背景与 indigo/violet 柔光渐变
4. `tailwind.config.js` 的 `theme.extend` 加 `colors.glass`、`keyframes`、`animation`

**验证：** `pnpm --filter web-client build` 通过

## T2: web-admin 设计 token + 清理残留

**文件：** `web/web-admin/src/style.css`、`web/web-admin/tailwind.config.js`
**依赖：** T1（复用同一 token 值）
**步骤：**
1. `style.css` 重写为与 web-client 相同的深色 token + 玻璃工具类
2. 删除 Vite 脚手架残留：`h1`、全局 `button`、`.card`、`#app`（max-width/居中）、`@media prefers-color-scheme`
3. `tailwind.config.js` 同 T1

**验证：** `pnpm --filter web-admin build` 通过

## T3: 安装 lucide-vue-next

**文件：** `package.json`（根）
**依赖：** 无
**步骤：**
1. 根 `package.json` 的 `dependencies` 新增 `lucide-vue-next`
2. `pnpm install`

**验证：** `pnpm install` 成功；`node -e "require('lucide-vue-next')"` 或 `pnpm ls lucide-vue-next` 可见

## T4: 基础组件改造（web-client）

**文件：** `web/web-client/src/components/ui/button.vue`、`card.vue`、`input.vue`、`index.ts`
**依赖：** T1、T3
**步骤：**
1. `button.vue`：`default` 变体改为 `bg-gradient-to-r from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))] text-white shadow hover:opacity-90`；`outline`/`ghost` 改玻璃描边；focus ring 用 `--ring`
2. `card.vue`：改用 `.glass` 类 + 圆角 + 阴影
3. `input.vue`：深色底 + 玻璃描边 + focus ring
4. `index.ts` 导出保持不变

**验证：** `pnpm --filter web-client build` 通过

## T5: 基础组件改造（web-admin）

**文件：** `web/web-admin/src/components/ui/button.vue`、`index.ts`
**依赖：** T2、T3
**步骤：** 同 T4 的 button 改造；`index.ts` 导出保持一致

**验证：** `pnpm --filter web-admin build` 通过

## T6: 图标封装 icon.vue

**文件：** `web/web-client/src/components/ui/icon.vue`、`index.ts`
**依赖：** T3
**步骤：**
1. `icon.vue` 接收 `name` prop，映射到 lucide 图标（Bell/Newspaper/Star/History/Menu/Search/ThumbsUp/Bookmark/Share2/Settings 等），透传 `class`/`size`
2. `index.ts` 导出 `UiIcon`

**验证：** `pnpm --filter web-client build` 通过

## T7: Toast 系统

**文件：** `web/web-client/src/components/ui/toast/useToast.ts`、`toast/ToastViewport.vue`
**依赖：** T1
**步骤：**
1. `useToast.ts`：模块级单例 store，`toast({title, description, type})` 推入队列，定时自动移除
2. `ToastViewport.vue`：固定右下角渲染 toast，玻璃样式，success/error/info 三态

**验证：** `pnpm --filter web-client build` 通过

## T8: 确认对话框

**文件：** `web/web-client/src/components/ui/dialog/useConfirm.ts`、`dialog/ConfirmDialog.vue`
**依赖：** T1
**步骤：**
1. `useConfirm.ts`：Promise 化 `confirm(options)`，返回 boolean
2. `ConfirmDialog.vue`：玻璃模态，danger 态红色确认按钮，Esc/遮罩取消

**验证：** `pnpm --filter web-client build` 通过

## T9: web-client App + BiliLayout

**文件：** `web/web-client/src/App.vue`、`layouts/BiliLayout.vue`
**依赖：** T1、T4、T6、T7、T8
**步骤：**
1. `App.vue` 渲染 `<router-view/>` 同时挂 `<ToastViewport/>` 与确认对话框宿主
2. `BiliLayout.vue`：导航玻璃化（`.glass-strong` sticky），Logo 用 `.gradient-text`，导航链接指向真实路由，emoji 图标换 `UiIcon`，搜索框深色，头像菜单玻璃化

**验证：** `pnpm --filter web-client build` 通过

## T10: web-admin App + BiliLayout

**文件：** `web/web-admin/src/App.vue`、`layouts/BiliLayout.vue`
**依赖：** T2、T5、T7（复用思路，admin 侧简化）
**步骤：** 同 T9 的玻璃导航（admin 无复杂菜单，仅 Logo + 登录/注册按钮）

**验证：** `pnpm --filter web-admin build` 通过

## T11: 首页（web-client）

**文件：** `web/web-client/src/views/Index/index.vue`、`components/video/VideoCard.vue`
**依赖：** T9
**步骤：**
1. `VideoCard.vue`：玻璃卡片 + 封面加载占位 + hover 抬升/光晕
2. `Index/index.vue`：骨架屏深色化、空态优化、卡片网格

**验证：** `pnpm --filter web-client build` 通过

## T12: 播放页（VideoPlayer）

**文件：** `web/web-client/src/views/VideoPlayer/index.vue`
**依赖：** T9、T6
**步骤：**
1. 点赞/收藏/转发改 `UiIcon` 按钮，选中态（`fill` + 主色），带点击反馈
2. 播放器外壳、信息卡、UP 主卡、推荐位玻璃化

**验证：** `pnpm --filter web-client build` 通过

## T13: 创作者中心（Creator）

**文件：** `web/web-client/src/views/Creator/index.vue`、`components/creator/VideoCard.vue`
**依赖：** T9、T7、T8
**步骤：**
1. `alert`/`confirm` 替换为 `toast`/`confirm`（登录提醒、删除确认、上传错误等）
2. 投稿表单、进度条、封面裁剪、列表、编辑/删除对话框玻璃化组件化
3. 清理硬编码颜色类名

**验证：** `pnpm --filter web-client build` 通过

## T14: 登录/注册/个人中心（web-client）

**文件：** `web/web-client/src/views/AuthLogin/index.vue`、`AuthRegister/index.vue`、`Profile/index.vue`
**依赖：** T9
**步骤：**
1. 三个页面统一玻璃卡片 + 表单深色化 + 错误态
2. 登录/注册页去原生 `alert`（若有）改为 `toast`

**验证：** `pnpm --filter web-client build` 通过

## T15: web-admin 页面

**文件：** `web/web-admin/src/views/Index/index.vue`、`AuthLogin/index.vue`、`AuthRegister/index.vue`
**依赖：** T10
**步骤：** 套用同一 token/组件风格，登录/注册表单深色化

**验证：** `pnpm --filter web-admin build` 通过

## T16: 收尾验证

**文件：** 全部
**依赖：** T1–T15
**步骤：**
1. `pnpm -r build` 全量构建
2. 检查无硬编码 `blue-*`/`pink-*`（grep）
3. 检查无原生 `alert(`/`confirm(`（grep）

**验证：** `pnpm -r build` 通过；两个 grep 无命中

## 执行顺序

```
T1 ──> T4 ──> T9 ──> T11/T12/T13/T14
T2 ──> T5 ──> T10 ──> T15
T3 ──> T6 ─┘
        └──> T7 ──> T9
        └──> T8 ──> T9
T16（依赖全部）
```
