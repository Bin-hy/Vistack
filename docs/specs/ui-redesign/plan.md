# UI 次世代重构（深色玻璃拟态）Plan

## 架构概览

纯前端改造，范围限定 `web-client` 与 `web-admin`。核心思路：**先立 token，再改组件，最后套页面**。

1. **设计 token 层**：重定义两端 `style.css` 的 CSS 变量为深色玻璃值；两端值完全一致。共享 tailwind preset（`web/ui/tailwind.config.js`）已把 `primary/background/border` 等颜色接到 `hsl(var(--x))`，因此只需改变量值即可整体换肤。
2. **工具与动效层**：在两端 `tailwind.config.js` 的 `theme.extend` 中新增 `glass` 色（支持 alpha 透明度修饰符）、`keyframes`/`animation`（光晕、淡入、缩放）。
3. **基础组件层**：统一两端 `components/ui`（button/card/input），新增 Toast 与确认对话框，引入 `lucide-vue-next` 图标。
4. **布局与页面层**：重构 `BiliLayout`，逐页套用玻璃卡片与图标，替换原生弹窗。

## 核心数据结构

### 设计 Token（`style.css :root`，两端一致）

```css
:root {
  color-scheme: dark;
  /* 基底 */
  --background: 240 16% 4%;      /* 近黑带蓝紫倾向 */
  --foreground: 220 20% 94%;

  /* 主色：indigo→violet 渐变 */
  --primary: 243 75% 60%;        /* indigo #6366f1 */
  --primary-foreground: 0 0% 100%;
  --gradient-from: 243 75% 60%;
  --gradient-to: 258 90% 66%;    /* violet #8b5cf6 */

  /* 表面 */
  --secondary: 240 12% 12%;
  --secondary-foreground: 220 20% 88%;
  --accent: 243 30% 18%;
  --accent-foreground: 220 20% 94%;
  --muted: 240 12% 10%;
  --muted-foreground: 220 12% 60%;

  /* 线条与控件 */
  --border: 240 12% 16%;
  --input: 240 12% 14%;
  --ring: 243 75% 60%;
  --destructive: 0 72% 55%;
  --destructive-foreground: 0 0% 100%;

  /* 玻璃（配合 alpha 修饰符使用） */
  --glass: 0 0% 100%;
  --glass-border: 0 0% 100%;
}
```

### 玻璃工具类（`style.css @layer components`）

```css
@layer components {
  .glass {
    background-color: hsl(var(--glass) / 0.04);
    backdrop-filter: blur(16px) saturate(140%);
    -webkit-backdrop-filter: blur(16px) saturate(140%);
    border: 1px solid hsl(var(--glass-border) / 0.08);
  }
  .glass-strong {
    background-color: hsl(var(--glass) / 0.08);
    backdrop-filter: blur(24px) saturate(160%);
    -webkit-backdrop-filter: blur(24px) saturate(160%);
    border: 1px solid hsl(var(--glass-border) / 0.12);
  }
  .gradient-text {
    background: linear-gradient(90deg, hsl(var(--gradient-from)), hsl(var(--gradient-to)));
    -webkit-background-clip: text;
    background-clip: text;
    color: transparent;
  }
}
```

### Tailwind 扩展（两端 `tailwind.config.js`）

```js
theme: {
  extend: {
    colors: {
      glass: 'hsl(var(--glass) / <alpha-value>)',
    },
    keyframes: {
      glow: { '0%,100%': { opacity: '0.5' }, '50%': { opacity: '1' } },
      'fade-in': { from: { opacity: '0' }, to: { opacity: '1' } },
      'scale-in': { from: { opacity: '0', transform: 'scale(0.96)' }, to: { opacity: '1', transform: 'scale(1)' } },
    },
    animation: {
      glow: 'glow 8s ease-in-out infinite',
      'fade-in': 'fade-in 0.2s ease-out',
      'scale-in': 'scale-in 0.15s ease-out',
    },
  },
}
```

### Toast 组合式函数（`components/ui/toast/useToast.ts`）

```ts
type ToastType = 'success' | 'error' | 'info'
interface ToastOptions { title: string; description?: string; type?: ToastType; duration?: number }
// useToast() 返回 { toast(options) }；全局单例，跨组件调用
```

### 确认对话框组合式函数（`components/ui/dialog/useConfirm.ts`）

```ts
interface ConfirmOptions { title: string; description?: string; danger?: boolean; confirmText?: string; cancelText?: string }
// useConfirm() 返回 { confirm(options): Promise<boolean> }；基于 Promise 的确认对话框
```

## 模块设计

### 设计 token（`web-client/src/style.css`、`web-admin/src/style.css`）
**职责：** 定义全部 CSS 变量、`.glass`/`.glass-strong`/`.gradient-text` 工具类、全局 `body` 深色背景与柔光渐变。
**依赖：** tailwind preset（`web/ui/tailwind.config.js`，已就绪）。
**注意：** `web-admin/style.css` 需删除 Vite 脚手架残留（`h1`、全局 `button`、`.card`、`#app`、`@media prefers-color-scheme`）。

### 图标系统（`lucide-vue-next`）
**职责：** 统一图标来源，替换 emoji（🔔📜⭐🕘）、PNG（点赞/收藏/转发）、内联 SVG（菜单）。
**依赖：** 新增 `lucide-vue-next` 到根 `package.json` 依赖（两端共用）。
**映射：** Bell / Newspaper / Star / History / Menu / Search / ThumbsUp / Bookmark / Share2 / Play / Pause / Volume2 / Maximize / Settings 等。

### 基础组件（`web-client/src/components/ui/`、`web-admin/src/components/ui/`）
- `button.vue`：保留 variant/size，`default` 改为蓝紫渐变（`bg-gradient-to-r from-indigo to-violet`），`outline`/`ghost` 改为玻璃描边，focus 用 `--ring`。
- `card.vue`：改用 `.glass` + 圆角 + 阴影。
- `input.vue`：深色底 + 玻璃描边 + focus ring。
- 新增 `toast/`（ToastViewport.vue + useToast.ts）、`dialog/`（ConfirmDialog.vue + useConfirm.ts）、`icon.vue`（统一 lucide 封装，按名渲染）。

### 布局（`BiliLayout.vue`，两端）
**职责：** 玻璃化吸顶导航：Logo（渐变文字）、真实路由链接、搜索、图标按钮、头像下拉菜单；移动端菜单抽屉化。
**依赖：** 图标系统、button/input。

### 页面（web-client）
- `Index`：玻璃卡片视频流 + 骨架屏（`animate-pulse` 深色版）+ 空态。
- `VideoPlayer`：播放器外壳 `.glass`；点赞/收藏/转发换 lucide 图标（选中态 `fill` + 主色）；UP 主卡与推荐位玻璃化。
- `Creator`：投稿/进度/裁剪/列表/编辑/删除全部玻璃化组件化；`alert`/`confirm` 替换为 `toast`/`confirm`。
- `AuthLogin`/`AuthRegister`/`Profile`：统一玻璃卡片 + 表单错误态。

### 页面（web-admin）
- `BiliLayout`、`Index`、`AuthLogin`、`AuthRegister`：套用同一 token 与组件风格，清理残留样式。

## 模块交互

```
style.css(token) ──> tailwind preset ──> components/ui(button/card/input/icon)
                                              │
BiliLayout ──> 各页面 ──> toast/useConfirm 组合式函数 ──> ToastViewport/ConfirmDialog
```

全局挂载：`App.vue` 中引入 `<ToastViewport />` 与 `<ConfirmDialogHost />`，`useToast`/`useConfirm` 通过模块级单例在任意组件调用，无需层层传参。

## 文件组织

```
web/web-client/src/
├── style.css                         修改：深色 token + 玻璃工具类
├── tailwind.config.js                修改：glass 色 + 动效
├── App.vue                           修改：挂载 ToastViewport/ConfirmDialogHost
├── layouts/BiliLayout.vue            修改：玻璃导航 + 图标 + 真实链接
├── components/ui/
│   ├── button.vue / card.vue / input.vue   修改：玻璃/渐变
│   ├── icon.vue                      新增：lucide 统一封装
│   ├── toast/{useToast.ts, ToastViewport.vue}     新增
│   └── dialog/{useConfirm.ts, ConfirmDialog.vue}  新增
├── components/video/VideoCard.vue    修改：玻璃卡片 + 骨架屏
├── components/creator/VideoCard.vue  修改：玻璃化
├── views/Index/index.vue             修改
├── views/VideoPlayer/index.vue       修改：图标按钮 + 玻璃卡
├── views/Creator/index.vue           修改：去原生弹窗 + 玻璃化
├── views/AuthLogin/index.vue         修改
├── views/AuthRegister/index.vue      修改
└── views/Profile/index.vue           修改

web/web-admin/src/
├── style.css                         修改：深色 token + 删除 Vite 残留
├── tailwind.config.js                修改：同 web-client
├── App.vue                           修改：挂载 Toast
├── layouts/BiliLayout.vue            修改
├── components/ui/{button.vue,index.ts}  修改/新增
└── views/{Index,AuthLogin,AuthRegister}/index.vue   修改

package.json（根）                    修改：新增 lucide-vue-next
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 主题模式 | 仅深色（不做明暗切换） | 与 spec「深色沉浸」一致，避免双主题维护成本 |
| 图标 | lucide-vue-next | 树摇、体积小、风格统一、Vue3 原生支持 |
| Toast/对话框 | 自研轻量组合式函数（`useToast`/`useConfirm`） | 无重依赖，Promise 化替代 `alert`/`confirm`，两端行为一致 |
| 玻璃实现 | CSS `backdrop-filter` + `@layer components` 工具类 | 标准玻璃拟态，无需额外依赖 |
| 渐变主色 | `bg-gradient-to-r from-[hsl(var(--gradient-from))] to-[hsl(var(--gradient-to))]` | 复用 token，避免硬编码 hex |
| token 复用 | 两端 `style.css` 各写一份同值变量，共享 preset 不改 | 共享 preset 已在 `web/ui`，保持 scope 内；两端变量值保持一致 |
| 依赖安装 | `lucide-vue-next` 加入根 `package.json` | 与 vue/vue-router/pinia 同层，两端共享 |
